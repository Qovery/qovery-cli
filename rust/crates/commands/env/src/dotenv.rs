//! Port of `github.com/joho/godotenv` v1.5.1 `Read` (parser.go): parse a `.env` file into a map.

use regex::Regex;
use std::collections::BTreeMap;
use std::io::Read;
use std::path::Path;
use std::sync::LazyLock;

static ESCAPE_REGEX: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"\\.").unwrap());
static EXPAND_VAR_REGEX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(\\)?(\$)(\()?\{?([A-Z0-9_]+)?\}?").unwrap());
static UNESCAPE_CHARS_REGEX: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"\\([^$])").unwrap());

/// `godotenv.Read(filename)`. Go returns a map (random iteration order); keys are sorted here.
pub fn read(filename: &str) -> Result<BTreeMap<String, String>, String> {
    let path = Path::new(filename);
    let mut file = std::fs::File::open(path)
        .map_err(|e| qovery_core::context::go_path_error("open", path, &e))?;
    let mut buf = Vec::new();
    file.read_to_end(&mut buf)
        .map_err(|e| qovery_core::context::go_path_error("read", path, &e))?;
    unmarshal_bytes(&buf)
}

/// `godotenv.UnmarshalBytes(src)`.
pub fn unmarshal_bytes(src: &[u8]) -> Result<BTreeMap<String, String>, String> {
    let mut out = BTreeMap::new();
    parse_bytes(src, &mut out)?;
    Ok(out)
}

fn parse_bytes(src: &[u8], out: &mut BTreeMap<String, String>) -> Result<(), String> {
    let src = replace_crlf(src);
    let mut cutset: &[u8] = &src;
    while let Some(start) = get_statement_start(cutset) {
        let (key, left) = locate_key_name(start)?;
        let (value, left) = extract_var_value(left, out)?;
        out.insert(key, value);
        cutset = left;
    }
    Ok(())
}

fn replace_crlf(src: &[u8]) -> Vec<u8> {
    let mut out = Vec::with_capacity(src.len());
    let mut i = 0;
    while i < src.len() {
        if src[i] == b'\r' && src.get(i + 1) == Some(&b'\n') {
            out.push(b'\n');
            i += 2;
        } else {
            out.push(src[i]);
            i += 1;
        }
    }
    out
}

/// Go `utf8.DecodeRune`: the rune at the start of `b` and its size (U+FFFD, 1 for invalid UTF-8).
fn decode_rune(b: &[u8]) -> (char, usize) {
    for len in 1..=b.len().min(4) {
        if let Ok(s) = std::str::from_utf8(&b[..len]) {
            return (s.chars().next().unwrap(), len);
        }
    }
    ('\u{FFFD}', 1)
}

/// Go `bytes.IndexFunc`.
fn index_func(b: &[u8], f: impl Fn(char) -> bool) -> Option<usize> {
    let mut i = 0;
    while i < b.len() {
        let (r, n) = decode_rune(&b[i..]);
        if f(r) {
            return Some(i);
        }
        i += n;
    }
    None
}

/// Go `bytes.TrimLeftFunc`.
fn trim_left_func(b: &[u8], f: impl Fn(char) -> bool) -> &[u8] {
    match index_func(b, |r| !f(r)) {
        Some(i) => &b[i..],
        None => &b[b.len()..],
    }
}

/// Go `unicode.IsSpace`.
fn go_is_space(r: char) -> bool {
    matches!(
        r,
        '\t' | '\n' | '\u{b}' | '\u{c}' | '\r' | ' ' | '\u{85}' | '\u{a0}'
    ) || (r as u32 > 0xff && r.is_whitespace())
}

/// godotenv `isSpace`: a space character but not a line break.
fn is_space(r: char) -> bool {
    matches!(
        r,
        '\t' | '\u{b}' | '\u{c}' | '\r' | ' ' | '\u{85}' | '\u{a0}'
    )
}

fn is_line_end(r: char) -> bool {
    r == '\n' || r == '\r'
}

/// `getStatementStart`: skip blank and comment lines.
fn get_statement_start(src: &[u8]) -> Option<&[u8]> {
    let pos = index_func(src, |r| !go_is_space(r))?;
    let src = &src[pos..];
    if src[0] != b'#' {
        return Some(src);
    }
    let pos = index_func(src, |r| r == '\n')?;
    get_statement_start(&src[pos..])
}

/// Go `unicode.IsLetter` / `unicode.IsNumber` of a byte converted to a rune (Latin-1).
fn is_letter_or_number(b: u8) -> bool {
    let c = char::from(b);
    c.is_alphabetic() || c.is_numeric()
}

/// `locateKeyName`.
fn locate_key_name(src: &[u8]) -> Result<(String, &[u8]), String> {
    let mut src = trim_left_func(src, is_space);
    if let Some(trimmed) = src.strip_prefix(b"export".as_slice())
        && index_func(trimmed, is_space) == Some(0)
    {
        src = trim_left_func(trimmed, is_space);
    }

    let mut key = String::new();
    let mut offset = 0;
    for (i, &c) in src.iter().enumerate() {
        if is_space(char::from(c)) {
            continue;
        }
        match c {
            b'=' | b':' => {
                key = String::from_utf8_lossy(&src[..i]).into_owned();
                offset = i + 1;
                break;
            }
            b'_' => {}
            _ => {
                if is_letter_or_number(c) || c == b'.' {
                    continue;
                }
                return Err(format!(
                    "unexpected character {} in variable name near {}",
                    qovery_log::gofmt::quote(&char::from(c).to_string()),
                    qovery_log::gofmt::quote(&String::from_utf8_lossy(src))
                ));
            }
        }
    }

    if src.is_empty() {
        return Err("zero length string".to_string());
    }

    let key = key.trim_end_matches(go_is_space).to_string();
    let cutset = trim_left_func(&src[offset..], is_space);
    Ok((key, cutset))
}

/// `extractVarValue`.
fn extract_var_value<'a>(
    src: &'a [u8],
    vars: &BTreeMap<String, String>,
) -> Result<(String, &'a [u8]), String> {
    let quote = match src.first() {
        Some(&q @ (b'"' | b'\'')) => q,
        _ => {
            // unquoted value - read until end of line
            let end_of_line = match index_func(src, is_line_end) {
                Some(i) => i,
                None => {
                    if src.is_empty() {
                        return Ok((String::new(), &src[src.len()..]));
                    }
                    src.len()
                }
            };
            let line: Vec<char> = String::from_utf8_lossy(&src[..end_of_line])
                .chars()
                .collect();
            let mut end_of_var = line.len();
            if end_of_var == 0 {
                return Ok((String::new(), &src[end_of_line..]));
            }
            // a comment after a whitespace ends the value
            for i in (0..end_of_var).rev() {
                if line[i] == '#' && i > 0 && is_space(line[i - 1]) {
                    end_of_var = i;
                    break;
                }
            }
            let value: String = line[..end_of_var].iter().collect();
            let trimmed = value.trim_matches(is_space);
            return Ok((expand_variables(trimmed, vars), &src[end_of_line..]));
        }
    };

    // lookup quoted string terminator
    for i in 1..src.len() {
        if src[i] != quote || src[i - 1] == b'\\' {
            continue;
        }
        let inner = &src[..i];
        let start = inner
            .iter()
            .position(|&c| c != quote)
            .unwrap_or(inner.len());
        let end = inner
            .iter()
            .rposition(|&c| c != quote)
            .map(|p| p + 1)
            .unwrap_or(0);
        let mut value = if start < end {
            String::from_utf8_lossy(&inner[start..end]).into_owned()
        } else {
            String::new()
        };
        if quote == b'"' {
            value = expand_variables(&expand_escapes(&value), vars);
        }
        return Ok((value, &src[i + 1..]));
    }

    let val_end = src.iter().position(|&c| c == b'\n').unwrap_or(src.len());
    Err(format!(
        "unterminated quoted value {}",
        String::from_utf8_lossy(&src[..val_end])
    ))
}

/// `expandEscapes`: `\n` and `\r` in double-quoted values.
fn expand_escapes(s: &str) -> String {
    let out = ESCAPE_REGEX.replace_all(s, |c: &regex::Captures| match &c[0] {
        "\\n" => "\n".to_string(),
        "\\r" => "\r".to_string(),
        m => m.to_string(),
    });
    UNESCAPE_CHARS_REGEX.replace_all(&out, "$1").into_owned()
}

/// `expandVariables`: `${VAR}` / `$VAR` from the variables parsed so far.
fn expand_variables(v: &str, m: &BTreeMap<String, String>) -> String {
    EXPAND_VAR_REGEX
        .replace_all(v, |c: &regex::Captures| {
            let whole = &c[0];
            if c.get(1).is_some() || c.get(3).is_some() {
                whole[1..].to_string()
            } else if let Some(name) = c.get(4) {
                m.get(name.as_str()).cloned().unwrap_or_default()
            } else {
                whole.to_string()
            }
        })
        .into_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(s: &str) -> BTreeMap<String, String> {
        unmarshal_bytes(s.as_bytes()).unwrap()
    }

    fn one(s: &str, key: &str, value: &str) {
        let m = parse(s);
        assert_eq!(
            m.get(key).map(String::as_str),
            Some(value),
            "{s:?} -> {m:?}"
        );
    }

    // Ported from godotenv_test.go (TestParsing / TestComments / TestExpanding).
    #[test]
    fn parsing() {
        one("FOO=bar", "FOO", "bar");
        one("FOO =bar", "FOO", "bar");
        one("FOO= bar", "FOO", "bar");
        one("FOO=\"bar\"", "FOO", "bar");
        one("FOO='bar'", "FOO", "bar");
        one("FOO=\"escaped\\\"bar\"", "FOO", "escaped\"bar");
        one("FOO=\"'d'\"", "FOO", "'d'");
        one("FOO: bar", "FOO", "bar");
        one("export OPTION_A=2", "OPTION_A", "2");
        one("OPTION_A = 2", "OPTION_A", "2");
        one("FOO=\"bar\\nbaz\"", "FOO", "bar\nbaz");
        one("FOO.BAR=foobar", "FOO.BAR", "foobar");
        one("FOO=foobar=", "FOO", "foobar=");
        one("FOO=bar ", "FOO", "bar");
        one("FOO=\"bar\" # comment", "FOO", "bar");
        one("FOO=bar # this is foo", "FOO", "bar");
        one("FOO=bar#baz", "FOO", "bar#baz");
        one("FOO='bar\\nbaz'", "FOO", "bar\\nbaz");
        one("FOO=\"bar\\\\r\\ b\"", "FOO", "bar\\r b");
        one("KEY=\"`\"", "KEY", "`");
        one("FOO=\"\"", "FOO", "");
        one("FOO=", "FOO", "");
        assert!(parse("# FOO=bar\n").is_empty());
        assert!(parse("  # comment\n\n").is_empty());
    }

    #[test]
    fn expanding() {
        one("FOO=test\nBAR=$FOO", "BAR", "test");
        one("FOO=test\nBAR=${FOO}bar", "BAR", "testbar");
        one("FOO=test\nBAR=\"quote $FOO\"", "BAR", "quote test");
        one("FOO=test\nBAR='quote $FOO'", "BAR", "quote $FOO");
        one("FOO=test\nBAR=\\$FOO", "BAR", "$FOO");
        one("BAR=$(echo)", "BAR", "(echo)");
        one("BAR=$UNKNOWN", "BAR", "");
    }

    #[test]
    fn errors() {
        assert_eq!(
            unmarshal_bytes(b"foo-bar=1\n").unwrap_err(),
            "unexpected character \"-\" in variable name near \"foo-bar=1\\n\""
        );
        assert_eq!(
            unmarshal_bytes(b"A=\"abc\nB=1").unwrap_err(),
            "unterminated quoted value \"abc"
        );
        assert_eq!(
            unmarshal_bytes(b"FOO\nBAR=1").unwrap_err(),
            "unexpected character \"\\n\" in variable name near \"FOO\\nBAR=1\""
        );
    }
}
