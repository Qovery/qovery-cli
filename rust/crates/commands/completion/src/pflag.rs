//! Minimal port of pflag v1.0.10 `FlagSet.Parse` (ContinueOnError, interspersed), used by `__complete`
//! which needs pflag's exact behaviour (positional count, `Changed`, error messages) on a partial command line.
//!
//! `qovery_cobra` parses with clap internally (its command-line parser is not reusable on a partial command line);
//! the value parsers (`strconv` / pflag semantics) are `qovery_cobra`'s.

use qovery_cobra::{Flag, FlagKind};
use qovery_log::gofmt::quote;
use std::collections::BTreeSet;

/// The state of a pflag `FlagSet` across several `Parse` calls (`Changed` is never reset by pflag).
pub struct FlagSet<'a> {
    pub flags: &'a [Flag],
    /// names of the flags set on the command line (pflag `Flag.Changed`)
    pub changed: BTreeSet<String>,
    /// positional args of the last `parse` (pflag `Args()`)
    pub args: Vec<String>,
    /// what pflag writes to the flag set output (deprecation warnings); cobra redirects it to a buffer
    pub output: String,
}

impl<'a> FlagSet<'a> {
    pub fn new(flags: &'a [Flag]) -> Self {
        FlagSet {
            flags,
            changed: BTreeSet::new(),
            args: Vec::new(),
            output: String::new(),
        }
    }

    pub fn lookup(&self, name: &str) -> Option<&'a Flag> {
        self.flags.iter().find(|f| f.name == name)
    }

    pub fn shorthand_lookup(&self, c: char) -> Option<&'a Flag> {
        self.flags.iter().find(|f| f.shorthand == Some(c))
    }

    /// pflag `Parse`.
    pub fn parse(&mut self, arguments: &[String]) -> Result<(), String> {
        self.args = Vec::new();
        if arguments.is_empty() {
            return Ok(());
        }
        self.parse_args(arguments.to_vec())
    }

    fn parse_args(&mut self, mut args: Vec<String>) -> Result<(), String> {
        while !args.is_empty() {
            let s = args.remove(0);
            if s.is_empty() || !s.starts_with('-') || s.len() == 1 {
                self.args.push(s);
                continue;
            }
            if s.as_bytes()[1] == b'-' {
                if s.len() == 2 {
                    // "--" terminates the flags
                    self.args.extend(args);
                    break;
                }
                args = self.parse_long_arg(&s, args)?;
            } else {
                args = self.parse_short_arg(&s, args)?;
            }
        }
        Ok(())
    }

    fn parse_long_arg(&mut self, s: &str, mut a: Vec<String>) -> Result<Vec<String>, String> {
        let name = &s[2..];
        if name.is_empty() || name.starts_with('-') || name.starts_with('=') {
            return Err(format!("bad flag syntax: {s}"));
        }
        let (name, split_value) = match name.split_once('=') {
            Some((n, v)) => (n, Some(v.to_string())),
            None => (name, None),
        };
        let Some(flag) = self.lookup(name) else {
            if name == "help" {
                return Err("pflag: help requested".to_string());
            }
            return Err(format!("unknown flag: --{name}"));
        };
        let value = if let Some(v) = split_value {
            v
        } else if flag.kind == FlagKind::Bool {
            "true".to_string()
        } else if !a.is_empty() {
            a.remove(0)
        } else {
            return Err(format!("flag needs an argument: --{name}"));
        };
        self.set(flag, &value)?;
        Ok(a)
    }

    fn parse_single_short_arg(
        &mut self,
        shorthands: &str,
        args: &[String],
    ) -> Result<(String, Vec<String>), String> {
        let mut out_args = args.to_vec();
        if shorthands.starts_with("test.") {
            return Ok((String::new(), out_args));
        }
        let c = shorthands.chars().next().unwrap_or('-');
        let rest = &shorthands[c.len_utf8()..];
        let mut out_shorts = rest.to_string();
        let Some(flag) = self.shorthand_lookup(c) else {
            if c == 'h' {
                return Err("pflag: help requested".to_string());
            }
            return Err(format!(
                "unknown shorthand flag: {} in -{shorthands}",
                go_quote_rune(c)
            ));
        };
        let value;
        if rest.len() > 1 && rest.starts_with('=') {
            value = rest[1..].to_string();
            out_shorts = String::new();
        } else if flag.kind == FlagKind::Bool {
            value = "true".to_string();
        } else if !rest.is_empty() {
            value = rest.to_string();
            out_shorts = String::new();
        } else if !args.is_empty() {
            value = args[0].clone();
            out_args = args[1..].to_vec();
        } else {
            return Err(format!(
                "flag needs an argument: {} in -{shorthands}",
                go_quote_rune(c)
            ));
        }
        self.set(flag, &value)?;
        Ok((out_shorts, out_args))
    }

    fn parse_short_arg(&mut self, s: &str, args: Vec<String>) -> Result<Vec<String>, String> {
        let mut a = args.clone();
        let mut shorthands = s[1..].to_string();
        // pflag passes the original `args` for every shorthand of the group
        while !shorthands.is_empty() {
            let (sh, na) = self.parse_single_short_arg(&shorthands, &args)?;
            shorthands = sh;
            a = na;
        }
        Ok(a)
    }

    /// pflag `FlagSet.Set`.
    fn set(&mut self, flag: &Flag, value: &str) -> Result<(), String> {
        if let Err(cause) = validate_value(flag, value) {
            let flag_name = match flag.shorthand {
                Some(c) => format!("-{c}, --{}", flag.name),
                None => format!("--{}", flag.name),
            };
            return Err(format!(
                "invalid argument {} for {} flag: {}",
                quote(value),
                quote(&flag_name),
                cause
            ));
        }
        self.changed.insert(flag.name.clone());
        if !flag.deprecated.is_empty() {
            self.output.push_str(&format!(
                "Flag --{} has been deprecated, {}\n",
                flag.name, flag.deprecated
            ));
        }
        Ok(())
    }
}

/// Go `%q` of a rune.
fn go_quote_rune(c: char) -> String {
    match c {
        '\'' => "'\\''".to_string(),
        '\\' => "'\\\\'".to_string(),
        c if (c as u32) < 0x20 || c as u32 == 0x7f => format!("'\\x{:02x}'", c as u32),
        c => format!("'{c}'"),
    }
}

/// pflag `Value.Set` errors for the flag types used by the CLI.
fn validate_value(f: &Flag, v: &str) -> Result<(), String> {
    match f.kind {
        FlagKind::Bool => parse_go_bool(v)
            .map(|_| ())
            .ok_or_else(|| format!("strconv.ParseBool: parsing {}: invalid syntax", quote(v))),
        FlagKind::Int | FlagKind::Int64 => parse_go_int(v, 64).map(|_| ()),
        FlagKind::Int32 => parse_go_int(v, 32).map(|_| ()),
        FlagKind::Uint => parse_go_uint(v).map(|_| ()),
        FlagKind::Float64 => v
            .parse::<f64>()
            .map(|_| ())
            .map_err(|_| format!("strconv.ParseFloat: parsing {}: invalid syntax", quote(v))),
        FlagKind::Duration => qovery_log::gofmt::parse_duration_nanos(v).map(|_| ()),
        FlagKind::StringToString => string_to_string_pairs(v).map(|_| ()),
        FlagKind::String | FlagKind::StringSlice | FlagKind::StringArray => Ok(()),
    }
}

pub use qovery_cobra::parse_go_bool;
use qovery_cobra::{parse_go_int, parse_go_uint, string_to_string_pairs};

#[cfg(test)]
mod tests {
    use super::*;

    fn flags() -> Vec<Flag> {
        vec![
            Flag::string("application", "Application Name").short('n'),
            Flag::bool("watch", "Watch").short('w'),
            Flag::int("port", "Port").short('p'),
            Flag::bool("help", "help").short('h'),
        ]
    }

    fn a(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn parse_like_pflag() {
        let f = flags();
        let mut fs = FlagSet::new(&f);
        fs.parse(&a(&["x", "-n", "app", "--watch", "y", "--", "-p"]))
            .unwrap();
        assert_eq!(fs.args, a(&["x", "y", "-p"]));
        assert!(fs.changed.contains("application") && fs.changed.contains("watch"));
        assert_eq!(
            fs.parse(&a(&["--nope"])).unwrap_err(),
            "unknown flag: --nope"
        );
        assert_eq!(
            fs.parse(&a(&["-x"])).unwrap_err(),
            "unknown shorthand flag: 'x' in -x"
        );
        assert_eq!(
            fs.parse(&a(&["-n"])).unwrap_err(),
            "flag needs an argument: 'n' in -n"
        );
        assert_eq!(
            fs.parse(&a(&["--application"])).unwrap_err(),
            "flag needs an argument: --application"
        );
        assert_eq!(
            fs.parse(&a(&["--port=x"])).unwrap_err(),
            "invalid argument \"x\" for \"-p, --port\" flag: strconv.ParseInt: parsing \"x\": invalid syntax"
        );
        let mut fs = FlagSet::new(&f);
        fs.parse(&a(&["-wnapp"])).unwrap();
        assert!(fs.changed.contains("application"));
    }
}
