//! pflag-compatible flag parsing, implemented with clap.
//!
//! A flat clap command is built from the flags available on the found cobra command; clap tokenizes and
//! collects values, then values are validated/converted with pflag semantics and messages.

use crate::flag::{Flag, FlagKind};
use clap::error::{ContextKind, ContextValue, ErrorKind};
use clap::parser::ValueSource;
use clap::{Arg, ArgAction};
use qovery_log::gofmt::quote;
use std::collections::BTreeMap;

const POSITIONAL: &str = "__cobra_positional_args";

/// A parsed flag value.
#[derive(Clone, Debug)]
pub struct FlagValue {
    pub flag: Flag,
    /// raw values in command-line order (empty when not changed)
    pub raw: Vec<String>,
    pub changed: bool,
}

pub struct Parsed {
    pub flags: BTreeMap<String, FlagValue>,
    pub args: Vec<String>,
}

fn build_clap(name: &str, flags: &[Flag]) -> clap::Command {
    let mut cmd = clap::Command::new(name.to_string())
        .no_binary_name(true)
        .disable_help_flag(true)
        .disable_version_flag(true)
        .disable_help_subcommand(true)
        .disable_colored_help(true)
        .args_override_self(true)
        .dont_delimit_trailing_values(true);
    for f in flags {
        let mut arg = Arg::new(f.name.clone())
            .long(f.name.clone())
            .required(false);
        if let Some(c) = f.shorthand {
            arg = arg.short(c);
        }
        arg = match f.kind {
            FlagKind::Bool => arg
                .num_args(0..=1)
                .require_equals(true)
                .default_missing_value("true")
                .action(ArgAction::Set),
            k if k.is_multi() => arg
                .num_args(1)
                .allow_hyphen_values(true)
                .action(ArgAction::Append),
            _ => arg
                .num_args(1)
                .allow_hyphen_values(true)
                .action(ArgAction::Set),
        };
        cmd = cmd.arg(arg);
    }
    cmd.arg(
        Arg::new(POSITIONAL)
            .num_args(0..)
            .action(ArgAction::Append)
            .required(false),
    )
}

fn flag_display(f: &Flag) -> String {
    match f.shorthand {
        Some(c) => format!("-{c}, --{}", f.name),
        None => format!("--{}", f.name),
    }
}

/// Go `strconv.ParseBool`.
pub fn parse_go_bool(s: &str) -> Option<bool> {
    match s {
        "1" | "t" | "T" | "TRUE" | "true" | "True" => Some(true),
        "0" | "f" | "F" | "FALSE" | "false" | "False" => Some(false),
        _ => None,
    }
}

/// Go `strconv.ParseInt(s, 0, bits)` error-compatible parsing.
pub fn parse_go_int(s: &str, bits: u32) -> Result<i64, String> {
    let syntax = || format!("strconv.ParseInt: parsing {}: invalid syntax", quote(s));
    let range = || format!("strconv.ParseInt: parsing {}: value out of range", quote(s));
    if s.is_empty() {
        return Err(syntax());
    }
    let (neg, body) = match s.as_bytes()[0] {
        b'-' => (true, &s[1..]),
        b'+' => (false, &s[1..]),
        _ => (false, s),
    };
    let lower = body.to_ascii_lowercase();
    let (radix, digits, prefixed) = if let Some(r) = lower.strip_prefix("0x") {
        (16, r.to_string(), true)
    } else if let Some(r) = lower.strip_prefix("0b") {
        (2, r.to_string(), true)
    } else if let Some(r) = lower.strip_prefix("0o") {
        (8, r.to_string(), true)
    } else if lower.len() > 1 && lower.starts_with('0') {
        (8, lower[1..].to_string(), true)
    } else {
        (10, lower.clone(), false)
    };
    let digits = if prefixed {
        digits.replace('_', "")
    } else {
        digits
    };
    if digits.is_empty() || digits.contains('_') {
        return Err(syntax());
    }
    let v = u128::from_str_radix(&digits, radix).map_err(|e| {
        if matches!(e.kind(), std::num::IntErrorKind::PosOverflow) {
            range()
        } else {
            syntax()
        }
    })?;
    let max = 1u128 << (bits - 1);
    if (!neg && v >= max) || (neg && v > max) {
        return Err(range());
    }
    Ok(if neg { -(v as i128) as i64 } else { v as i64 })
}

/// Go `strconv.ParseUint(s, 0, 64)`.
pub fn parse_go_uint(s: &str) -> Result<u64, String> {
    if s.starts_with('-') || s.starts_with('+') {
        return Err(format!(
            "strconv.ParseUint: parsing {}: invalid syntax",
            quote(s)
        ));
    }
    parse_go_int(&format!("+{s}"), 65)
        .map(|v| v as u64)
        .map_err(|e| {
            e.replace("ParseInt", "ParseUint")
                .replace(&quote(&format!("+{s}")), &quote(s))
        })
}

/// pflag `stringToStringValue.Set`: a value with a single `=` is ONE pair (`Key=a,b` -> `Key`=`a,b`);
/// it is CSV-split only when it contains several `=`.
pub fn string_to_string_pairs(val: &str) -> Result<Vec<(String, String)>, String> {
    let ss: Vec<String> = match val.matches('=').count() {
        0 => return Err(format!("{val} must be formatted as key=value")),
        1 => vec![val.trim_matches('"').to_string()],
        _ => read_as_csv(val),
    };
    ss.iter()
        .map(|pair| match pair.split_once('=') {
            Some((k, v)) => Ok((k.to_string(), v.to_string())),
            None => Err(format!("{pair} must be formatted as key=value")),
        })
        .collect()
}

/// pflag `readAsCSV`.
pub fn read_as_csv(val: &str) -> Vec<String> {
    if val.is_empty() {
        return Vec::new();
    }
    let mut out = Vec::new();
    let mut cur = String::new();
    let mut in_quotes = false;
    let mut chars = val.chars().peekable();
    while let Some(c) = chars.next() {
        match c {
            '"' if in_quotes && chars.peek() == Some(&'"') => {
                cur.push('"');
                chars.next();
            }
            '"' if cur.is_empty() && !in_quotes => in_quotes = true,
            '"' if in_quotes => in_quotes = false,
            ',' if !in_quotes => out.push(std::mem::take(&mut cur)),
            c => cur.push(c),
        }
    }
    out.push(cur);
    out
}

fn validate_value(f: &Flag, v: &str) -> Result<(), String> {
    let invalid = |e: String| {
        format!(
            "invalid argument {} for {} flag: {}",
            quote(v),
            quote(&flag_display(f)),
            e
        )
    };
    match f.kind {
        FlagKind::Bool => parse_go_bool(v).map(|_| ()).ok_or_else(|| {
            invalid(format!(
                "strconv.ParseBool: parsing {}: invalid syntax",
                quote(v)
            ))
        }),
        FlagKind::Int | FlagKind::Int64 => parse_go_int(v, 64).map(|_| ()).map_err(invalid),
        FlagKind::Int32 => parse_go_int(v, 32).map(|_| ()).map_err(invalid),
        FlagKind::Uint => parse_go_uint(v).map(|_| ()).map_err(invalid),
        FlagKind::Float64 => v.parse::<f64>().map(|_| ()).map_err(|_| {
            invalid(format!(
                "strconv.ParseFloat: parsing {}: invalid syntax",
                quote(v)
            ))
        }),
        FlagKind::Duration => qovery_log::gofmt::parse_duration_nanos(v)
            .map(|_| ())
            .map_err(invalid),
        FlagKind::StringToString => string_to_string_pairs(v).map(|_| ()).map_err(invalid),
        FlagKind::String | FlagKind::StringSlice | FlagKind::StringArray => Ok(()),
    }
}

fn invalid_arg_from(err: &clap::Error) -> Option<String> {
    match err.get(ContextKind::InvalidArg) {
        Some(ContextValue::String(s)) => Some(s.clone()),
        Some(ContextValue::Strings(v)) => v.first().cloned(),
        _ => None,
    }
}

/// Translate a clap error into the pflag message.
fn translate_error(err: clap::Error, args: &[String], flags: &[Flag]) -> String {
    let invalid_arg = invalid_arg_from(&err).unwrap_or_default();
    // clap renders args as "--name <name>" / "-n, --name <name>": keep the flag token only
    let token = invalid_arg
        .split_whitespace()
        .next()
        .unwrap_or("")
        .trim_end_matches(',')
        .to_string();
    match err.kind() {
        ErrorKind::UnknownArgument => {
            if let Some(long) = token.strip_prefix("--") {
                let name = long.split('=').next().unwrap_or(long);
                return format!("unknown flag: --{name}");
            }
            if let Some(short) = token.strip_prefix('-') {
                let c = short.chars().next().unwrap_or('-');
                // pflag reports the remaining shorthands, starting at the unknown one: `-hx` -> `in -x`
                let original = args
                    .iter()
                    .find(|a| a.starts_with('-') && !a.starts_with("--") && a[1..].contains(c))
                    .map(|a| {
                        let body = a[1..].split('=').next().unwrap_or("");
                        let pos = body.find(c).unwrap_or(0);
                        format!("-{}", &a[1 + pos..])
                    })
                    .unwrap_or_else(|| token.clone());
                return format!("unknown shorthand flag: '{c}' in {original}");
            }
            format!("unknown flag: {token}")
        }
        ErrorKind::InvalidValue
        | ErrorKind::NoEquals
        | ErrorKind::WrongNumberOfValues
        | ErrorKind::TooFewValues => {
            // find the flag (whole tokens: `--service-id <service-id>` is not `--service`)
            let flag = flags.iter().find(|f| {
                let long = format!("--{}", f.name);
                invalid_arg.split([' ', ',', '=']).any(|t| t == long)
                    || f.shorthand.is_some_and(|c| invalid_arg == format!("-{c}"))
            });
            if let Some(f) = flag {
                if f.kind == FlagKind::Bool {
                    return format!(
                        "invalid argument \"\" for {} flag: strconv.ParseBool: parsing \"\": invalid syntax",
                        quote(&flag_display(f))
                    );
                }
                // was it given as a shorthand?
                let as_short = f.shorthand.and_then(|c| {
                    args.iter()
                        .rev()
                        .find(|a| a.starts_with('-') && !a.starts_with("--") && a.ends_with(c))
                        .map(|a| (c, a.clone()))
                });
                let as_long = args.iter().any(|a| a == &format!("--{}", f.name));
                if let (Some((c, tok)), false) = (as_short, as_long) {
                    return format!("flag needs an argument: '{c}' in {tok}");
                }
                return format!("flag needs an argument: --{}", f.name);
            }
            err.to_string()
                .lines()
                .next()
                .unwrap_or("")
                .trim_start_matches("error: ")
                .to_string()
        }
        _ => err
            .to_string()
            .lines()
            .next()
            .unwrap_or("")
            .trim_start_matches("error: ")
            .to_string(),
    }
}

/// Parse `args` against `flags` with pflag semantics.
pub fn parse(name: &str, flags: &[Flag], args: &[String]) -> Result<Parsed, String> {
    // pflag: `---x` and `--=x` are syntax errors (checked before anything else on the token)
    for a in args {
        if a == "--" {
            break;
        }
        if a.starts_with("---") || a.starts_with("--=") {
            return Err(format!("bad flag syntax: {a}"));
        }
    }
    // pflag normalize functions: rewrite `--alias[=v]` into `--name[=v]`
    let mut normalized: Vec<String> = Vec::with_capacity(args.len());
    let mut terminated = false;
    for a in args {
        let mut out = a.clone();
        if !terminated {
            if a == "--" {
                terminated = true;
            } else if let Some(long) = a.strip_prefix("--") {
                let (key, rest) = match long.split_once('=') {
                    Some((k, v)) => (k, Some(v)),
                    None => (long, None),
                };
                if let Some(f) = flags.iter().find(|f| f.aliases.iter().any(|al| al == key)) {
                    out = match rest {
                        Some(v) => format!("--{}={v}", f.name),
                        None => format!("--{}", f.name),
                    };
                }
            }
        }
        normalized.push(out);
    }
    let args = &normalized[..];
    let clap_cmd = build_clap(name, flags);
    let matches = clap_cmd
        .try_get_matches_from(args)
        .map_err(|e| translate_error(e, args, flags))?;

    // validate values in command line order (pflag stops at the first invalid one)
    let mut ordered: Vec<(usize, &Flag, String)> = Vec::new();
    let mut out = BTreeMap::new();
    for f in flags {
        let changed = matches.value_source(&f.name) == Some(ValueSource::CommandLine);
        let mut raw = Vec::new();
        if changed {
            let indices: Vec<usize> = matches
                .indices_of(&f.name)
                .map(|i| i.collect())
                .unwrap_or_default();
            let values: Vec<String> = matches
                .get_raw(&f.name)
                .map(|v| v.map(|s| s.to_string_lossy().to_string()).collect())
                .unwrap_or_default();
            for (i, v) in values.iter().enumerate() {
                ordered.push((indices.get(i).copied().unwrap_or(usize::MAX), f, v.clone()));
            }
            raw = values;
        }
        out.insert(
            f.name.clone(),
            FlagValue {
                flag: f.clone(),
                raw,
                changed,
            },
        );
    }
    ordered.sort_by_key(|(i, _, _)| *i);
    for (_, f, v) in ordered {
        validate_value(f, &v)?;
    }
    let positional: Vec<String> = matches
        .get_many::<String>(POSITIONAL)
        .map(|v| v.cloned().collect())
        .unwrap_or_default();
    Ok(Parsed {
        flags: out,
        args: positional,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn flags() -> Vec<Flag> {
        vec![
            Flag::string("application", "Application Name").short('n'),
            Flag::bool("watch", "Watch").short('w'),
            Flag::int("port", "Port").short('p'),
            Flag::string_slice("command", "cmd").default_value("[sh]"),
            Flag::bool("help", "help").short('h'),
        ]
    }

    fn a(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn basic() {
        let p = parse(
            "x",
            &flags(),
            &a(&["pos1", "-n", "app", "--watch", "pos2", "--port=3"]),
        )
        .unwrap();
        assert_eq!(p.args, a(&["pos1", "pos2"]));
        assert_eq!(p.flags["application"].raw, a(&["app"]));
        assert_eq!(p.flags["watch"].raw, a(&["true"]));
        assert_eq!(p.flags["port"].raw, a(&["3"]));
        assert!(!p.flags["command"].changed);
    }

    #[test]
    fn bool_values_and_last_wins() {
        let p = parse(
            "x",
            &flags(),
            &a(&["--watch=false", "-n", "a", "--application", "b"]),
        )
        .unwrap();
        assert_eq!(p.flags["watch"].raw, a(&["false"]));
        assert_eq!(p.flags["application"].raw.last().unwrap(), "b");
        let p = parse("x", &flags(), &a(&["-wn", "app"])).unwrap();
        assert_eq!(p.flags["application"].raw, a(&["app"]));
        assert_eq!(p.flags["watch"].raw, a(&["true"]));
        let p = parse("x", &flags(), &a(&["-napp"])).unwrap();
        assert_eq!(p.flags["application"].raw, a(&["app"]));
        let p = parse("x", &flags(), &a(&["--application", "--watch"])).unwrap();
        assert_eq!(p.flags["application"].raw, a(&["--watch"]));
    }

    #[test]
    fn errors() {
        assert_eq!(
            parse("x", &flags(), &a(&["--foo"])).err().unwrap(),
            "unknown flag: --foo"
        );
        assert_eq!(
            parse("x", &flags(), &a(&["--foo=bar"])).err().unwrap(),
            "unknown flag: --foo"
        );
        assert_eq!(
            parse("x", &flags(), &a(&["-x"])).err().unwrap(),
            "unknown shorthand flag: 'x' in -x"
        );
        assert_eq!(
            parse("x", &flags(), &a(&["--watch=maybe"])).err().unwrap(),
            "invalid argument \"maybe\" for \"-w, --watch\" flag: strconv.ParseBool: parsing \"maybe\": invalid syntax"
        );
        assert_eq!(
            parse("x", &flags(), &a(&["--port", "abc"])).err().unwrap(),
            "invalid argument \"abc\" for \"-p, --port\" flag: strconv.ParseInt: parsing \"abc\": invalid syntax"
        );
        assert_eq!(
            parse("x", &flags(), &a(&["--application"])).err().unwrap(),
            "flag needs an argument: --application"
        );
        assert_eq!(
            parse("x", &flags(), &a(&["-n"])).err().unwrap(),
            "flag needs an argument: 'n' in -n"
        );
    }

    #[test]
    fn needs_argument_names_the_whole_flag() {
        let flags = vec![
            Flag::string("service", "Service Name"),
            Flag::string("service-id", "Service ID"),
            Flag::string("application", "Application Name").short('n'),
            Flag::string("applications", "Application Names"),
        ];
        assert_eq!(
            parse("x", &flags, &a(&["--service-id"])).err().unwrap(),
            "flag needs an argument: --service-id"
        );
        assert_eq!(
            parse("x", &flags, &a(&["--service"])).err().unwrap(),
            "flag needs an argument: --service"
        );
        assert_eq!(
            parse("x", &flags, &a(&["--applications"])).err().unwrap(),
            "flag needs an argument: --applications"
        );
        assert_eq!(
            parse("x", &flags, &a(&["-n"])).err().unwrap(),
            "flag needs an argument: 'n' in -n"
        );
    }

    #[test]
    fn after_double_dash() {
        let p = parse("x", &flags(), &a(&["--", "-n", "x"])).unwrap();
        assert_eq!(p.args, a(&["-n", "x"]));
    }
}

#[cfg(test)]
mod string_to_string_tests {
    use super::*;

    #[test]
    fn pflag_semantics() {
        assert_eq!(
            string_to_string_pairs("Key=a,b").unwrap(),
            vec![("Key".into(), "a,b".into())]
        );
        assert_eq!(
            string_to_string_pairs("a=1,b=2").unwrap(),
            vec![("a".into(), "1".into()), ("b".into(), "2".into())]
        );
        assert_eq!(
            string_to_string_pairs("abc").unwrap_err(),
            "abc must be formatted as key=value"
        );
        assert_eq!(
            qovery_log::gofmt::parse_duration_nanos("-1s").unwrap(),
            -1_000_000_000
        );
    }
}
