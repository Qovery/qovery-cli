//! Go `encoding/json` syntax check (`checkValid`) with the error messages of the Go 1.27 build of the CLI (the
//! legacy renderings of the jsonv2 `jsontext` errors): `invalid character 'x' looking for beginning of value`,
//! `unexpected end of JSON input`, ... Used before decoding JSON with serde wherever Go prints its
//! `json.Unmarshal` error.
//!
//! Not the same messages as go-jose's fork of encoding/json (`qovery-cmd-admin`) nor as a `json.Decoder` stream
//! (`qovery-cmd-env`).

/// jsonwire `QuoteRune`: the first rune of `b`, quoted like `strconv.QuoteRune` (`'\xNN'` when invalid).
fn quote_rune(b: &[u8]) -> String {
    let Some(&first) = b.first() else {
        return "' '".to_string();
    };
    if first < 0x80 {
        return quote_char(first);
    }
    let width = match first {
        0xC0..=0xDF => 2,
        0xE0..=0xEF => 3,
        0xF0..=0xF7 => 4,
        _ => 0,
    };
    match b
        .get(..width)
        .and_then(|w| std::str::from_utf8(w).ok())
        .and_then(|s| s.chars().next())
    {
        Some(r) if width > 0 => {
            let q = crate::gofmt::quote(&r.to_string());
            format!("'{}'", &q[1..q.len() - 1])
        }
        _ => format!("'\\x{first:x}'"),
    }
}

/// Go `encoding/json` `quoteChar`.
fn quote_char(c: u8) -> String {
    match c {
        b'\'' => "'\\''".to_string(),
        b'"' => "'\"'".to_string(),
        _ => {
            let q = crate::gofmt::quote(&char::from(c).to_string());
            format!("'{}'", &q[1..q.len() - 1])
        }
    }
}

/// jsonwire `NewInvalidEscapeSequenceError(what).Error()`.
fn invalid_escape(what: &[u8]) -> String {
    let what = String::from_utf8_lossy(what);
    let need_escape = what
        .chars()
        .any(|r| r == '`' || r == '\u{FFFD}' || r.is_whitespace() || r.is_control());
    let shown = if need_escape {
        crate::gofmt::quote(&what)
    } else {
        format!("`{what}`")
    };
    format!("invalid escape sequence {shown} in string")
}

/// Port of Go's `encoding/json` scanner (`checkValid`): the syntax error message of an invalid document.
/// The Go CLI is built with the `jsonv2` experiment: messages are the legacy renderings of `jsontext` errors.
pub fn go_json_check_valid(data: &[u8]) -> Result<(), String> {
    #[derive(Clone, Copy, PartialEq)]
    enum Parse {
        ObjectKey,
        ObjectValue,
        ArrayValue,
    }
    #[derive(Clone, Copy, PartialEq)]
    enum St {
        BeginValue,
        BeginValueOrEmpty,
        BeginStringOrEmpty,
        BeginString,
        EndValue,
        EndTop,
        InString,
        InStringEsc,
        EscU(u8),
        Neg,
        One,
        Zero,
        Dot,
        Dot0,
        E,
        ESign,
        E0,
        Lit(&'static [u8], usize, &'static str),
    }
    let is_space = |c: u8| matches!(c, b' ' | b'\t' | b'\r' | b'\n');
    let err = |pos: usize, ctx: &str| {
        format!(
            "invalid character {} {ctx}",
            quote_rune(&data[pos.min(data.len())..])
        )
    };
    let mut stack: Vec<Parse> = Vec::new();
    let mut st = St::BeginValue;
    let mut end_top = false;

    // one scanner step; Err on a syntax error
    let step = |st: &mut St,
                stack: &mut Vec<Parse>,
                end_top: &mut bool,
                i: usize,
                c: u8|
     -> Result<(), String> {
        let mut cur = *st;
        loop {
            match cur {
                St::BeginValueOrEmpty => {
                    if is_space(c) {
                        return Ok(());
                    }
                    if c == b']' {
                        cur = St::EndValue;
                        continue;
                    }
                    cur = St::BeginValue;
                }
                St::BeginValue => {
                    if is_space(c) {
                        return Ok(());
                    }
                    *st = match c {
                        b'{' => {
                            stack.push(Parse::ObjectKey);
                            St::BeginStringOrEmpty
                        }
                        b'[' => {
                            stack.push(Parse::ArrayValue);
                            St::BeginValueOrEmpty
                        }
                        b'"' => St::InString,
                        b'-' => St::Neg,
                        b'0' => St::Zero,
                        b't' => St::Lit(b"true", 1, "true"),
                        b'f' => St::Lit(b"false", 1, "false"),
                        b'n' => St::Lit(b"null", 1, "null"),
                        b'1'..=b'9' => St::One,
                        _ => return Err(err(i, "looking for beginning of value")),
                    };
                    return Ok(());
                }
                St::BeginStringOrEmpty => {
                    if is_space(c) {
                        return Ok(());
                    }
                    if c == b'}' {
                        if let Some(last) = stack.last_mut() {
                            *last = Parse::ObjectValue;
                        }
                        cur = St::EndValue;
                        continue;
                    }
                    cur = St::BeginString;
                }
                St::BeginString => {
                    if is_space(c) {
                        return Ok(());
                    }
                    if c == b'"' {
                        *st = St::InString;
                        return Ok(());
                    }
                    return Err(err(i, "looking for beginning of object key string"));
                }
                St::EndValue => {
                    let Some(ps) = stack.last().copied() else {
                        *st = St::EndTop;
                        *end_top = true;
                        cur = St::EndTop;
                        continue;
                    };
                    if is_space(c) {
                        *st = St::EndValue;
                        return Ok(());
                    }
                    let pop = |stack: &mut Vec<Parse>, st: &mut St, end_top: &mut bool| {
                        stack.pop();
                        if stack.is_empty() {
                            *st = St::EndTop;
                            *end_top = true;
                        } else {
                            *st = St::EndValue;
                        }
                    };
                    match ps {
                        Parse::ObjectKey => {
                            if c == b':' {
                                *stack.last_mut().unwrap() = Parse::ObjectValue;
                                *st = St::BeginValue;
                                return Ok(());
                            }
                            return Err(err(i, "after object key"));
                        }
                        Parse::ObjectValue => {
                            if c == b',' {
                                *stack.last_mut().unwrap() = Parse::ObjectKey;
                                *st = St::BeginString;
                                return Ok(());
                            }
                            if c == b'}' {
                                pop(stack, st, end_top);
                                return Ok(());
                            }
                            return Err(err(i, "after object key:value pair"));
                        }
                        Parse::ArrayValue => {
                            if c == b',' {
                                *st = St::BeginValue;
                                return Ok(());
                            }
                            if c == b']' {
                                pop(stack, st, end_top);
                                return Ok(());
                            }
                            return Err(err(i, "after array element"));
                        }
                    }
                }
                St::EndTop => {
                    *st = St::EndTop;
                    if !is_space(c) {
                        return Err(err(i, "after top-level value"));
                    }
                    return Ok(());
                }
                St::InString => {
                    *st = match c {
                        b'"' => St::EndValue,
                        b'\\' => St::InStringEsc,
                        c if c < 0x20 => return Err(err(i, "in string")),
                        _ => St::InString,
                    };
                    return Ok(());
                }
                St::InStringEsc => {
                    *st = match c {
                        b'b' | b'f' | b'n' | b'r' | b't' | b'\\' | b'/' | b'"' => St::InString,
                        b'u' => St::EscU(0),
                        _ => return Err(invalid_escape(&data[i - 1..(i + 1).min(data.len())])),
                    };
                    return Ok(());
                }
                St::EscU(n) => {
                    if !c.is_ascii_hexdigit() {
                        let start = i - n as usize - 2;
                        return Err(invalid_escape(&data[start..(start + 6).min(data.len())]));
                    }
                    *st = if n == 3 {
                        St::InString
                    } else {
                        St::EscU(n + 1)
                    };
                    return Ok(());
                }
                St::Neg => {
                    *st = match c {
                        b'0' => St::Zero,
                        b'1'..=b'9' => St::One,
                        _ => return Err(err(i, "in numeric literal")),
                    };
                    return Ok(());
                }
                St::One => {
                    if c.is_ascii_digit() {
                        *st = St::One;
                        return Ok(());
                    }
                    cur = St::Zero;
                }
                St::Zero => match c {
                    b'.' => {
                        *st = St::Dot;
                        return Ok(());
                    }
                    b'e' | b'E' => {
                        *st = St::E;
                        return Ok(());
                    }
                    _ => cur = St::EndValue,
                },
                St::Dot => {
                    if c.is_ascii_digit() {
                        *st = St::Dot0;
                        return Ok(());
                    }
                    return Err(err(i, "in numeric literal"));
                }
                St::Dot0 => match c {
                    b'0'..=b'9' => {
                        *st = St::Dot0;
                        return Ok(());
                    }
                    b'e' | b'E' => {
                        *st = St::E;
                        return Ok(());
                    }
                    _ => cur = St::EndValue,
                },
                St::E => {
                    if c == b'+' || c == b'-' {
                        *st = St::ESign;
                        return Ok(());
                    }
                    cur = St::ESign;
                }
                St::ESign => {
                    if c.is_ascii_digit() {
                        *st = St::E0;
                        return Ok(());
                    }
                    return Err(err(i, "in numeric literal"));
                }
                St::E0 => {
                    if c.is_ascii_digit() {
                        *st = St::E0;
                        return Ok(());
                    }
                    cur = St::EndValue;
                }
                St::Lit(word, k, name) => {
                    if c != word[k] {
                        return Err(err(
                            i,
                            &format!("in literal {name} (expecting {})", quote_char(word[k])),
                        ));
                    }
                    *st = if k + 1 == word.len() {
                        St::EndValue
                    } else {
                        St::Lit(word, k + 1, name)
                    };
                    return Ok(());
                }
            }
        }
    };

    for (i, &c) in data.iter().enumerate() {
        step(&mut st, &mut stack, &mut end_top, i, c)?;
    }
    // scanner.eof(): an incomplete token is an unexpected end
    if end_top {
        return Ok(());
    }
    if matches!(
        st,
        St::Lit(..) | St::Neg | St::Dot | St::E | St::ESign | St::InStringEsc | St::EscU(_)
    ) {
        return Err("unexpected end of JSON input".to_string());
    }
    step(&mut st, &mut stack, &mut end_top, data.len(), b' ')?;
    if end_top {
        return Ok(());
    }
    Err("unexpected end of JSON input".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_scanner_messages() {
        let cases: [(&[u8], &str); 12] = [
            (b"x", "invalid character 'x' looking for beginning of value"),
            (b"{\"a\" 1}", "invalid character '1' after object key"),
            (
                b"{\"a\": 1 2}",
                "invalid character '2' after object key:value pair",
            ),
            (b"[1 2]", "invalid character '2' after array element"),
            (
                b"{1}",
                "invalid character '1' looking for beginning of object key string",
            ),
            (b"\"a\\x\"", "invalid escape sequence `\\x` in string"),
            (
                b"\"\\u12g4\"",
                "invalid escape sequence `\\u12g4` in string",
            ),
            (b"-a", "invalid character 'a' in numeric literal"),
            (b"1.x", "invalid character 'x' in numeric literal"),
            (b"1ex", "invalid character 'x' in numeric literal"),
            (b"{} {}", "invalid character '{' after top-level value"),
            (b"\"a\nb\"", "invalid character '\\n' in string"),
        ];
        for (input, expected) in cases {
            assert_eq!(
                go_json_check_valid(input).unwrap_err(),
                expected,
                "{}",
                String::from_utf8_lossy(input)
            );
        }
        assert!(go_json_check_valid(b" {\"a\": [1, -2.5e+3, true, null, \"\\u00e9\"]} ").is_ok());
        assert!(go_json_check_valid(b"42").is_ok());
        assert_eq!(
            go_json_check_valid(b"[1,").unwrap_err(),
            "unexpected end of JSON input"
        );
    }
}
