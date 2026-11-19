//! Go `encoding/json` `json.NewDecoder(r).Decode(&map[string]string{})` (v1): syntax errors of the scanner,
//! `EOF` / `unexpected EOF` and `UnmarshalTypeError` messages. Scanner adapted from `qovery-cmd-admin`'s `gojson.rs`.

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum Step {
    BeginValueOrEmpty,
    BeginValue,
    BeginStringOrEmpty,
    BeginString,
    EndValue,
    EndTop,
    InString,
    InStringEsc,
    InStringEscU(u8),
    Neg,
    One,
    Zero,
    Dot,
    Dot0,
    E,
    ESign,
    E0,
    Lit(&'static [u8], usize, &'static str),
    Error,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Parse {
    ObjectKey,
    ObjectValue,
    ArrayValue,
}

struct Scanner<'a> {
    data: &'a [u8],
    /// index of the byte being fed
    pos: usize,
    /// index of the backslash of the current string escape
    esc_start: usize,
    step: Step,
    stack: Vec<Parse>,
    end_top: bool,
    err: Option<String>,
}

const MAX_NESTING_DEPTH: usize = 10000;

/// Go `jsonwire.QuoteRune`: the rune at the start of `b`, quoted with `strconv.QuoteRune` (`'\xff'` for an
/// invalid UTF-8 byte).
fn quote_rune(b: &[u8]) -> String {
    let Some(&first) = b.first() else {
        return "'\\x0'".to_string();
    };
    let r = (1..=b.len().min(4))
        .find_map(|n| std::str::from_utf8(&b[..n]).ok())
        .and_then(|s| s.chars().next());
    match r {
        None => format!("'\\x{first:x}'"),
        Some('\'') => "'\\''".to_string(),
        Some('"') => "'\"'".to_string(),
        // Unicode format characters (Cf) are not printable for Go (`qovery_log::gofmt::quote` prints them as is)
        Some(
            r @ ('\u{ad}'
            | '\u{600}'..='\u{605}'
            | '\u{200b}'..='\u{200f}'
            | '\u{202a}'..='\u{202e}'
            | '\u{2060}'..='\u{2064}'
            | '\u{feff}'),
        ) => {
            format!("'\\u{:04x}'", r as u32)
        }
        Some(r) => {
            let q = qovery_log::gofmt::quote(&r.to_string());
            format!("'{}'", &q[1..q.len() - 1])
        }
    }
}

/// Go `jsonwire.InvalidTextError` of an invalid escape sequence `what` (`\x`, `\u12g4`).
fn invalid_escape_sequence(what: &[u8]) -> String {
    let what = String::from_utf8_lossy(what);
    let need_escape = what
        .chars()
        .any(|r| r == '`' || r == '\u{FFFD}' || r.is_whitespace() || r.is_control());
    let what = if what.chars().count() == 1 {
        quote_rune(what.as_bytes())
    } else if need_escape {
        qovery_log::gofmt::quote(&what)
    } else {
        format!("`{what}`")
    };
    format!("invalid escape sequence {what} in string")
}

fn is_space(c: u8) -> bool {
    matches!(c, b' ' | b'\t' | b'\r' | b'\n')
}

fn is_hex(c: u8) -> bool {
    c.is_ascii_hexdigit()
}

impl<'a> Scanner<'a> {
    fn new(data: &'a [u8]) -> Self {
        Scanner {
            data,
            pos: 0,
            esc_start: 0,
            step: Step::BeginValue,
            stack: Vec::new(),
            end_top: false,
            err: None,
        }
    }

    fn error(&mut self, c: u8, context: &str) {
        self.step = Step::Error;
        let at = self
            .data
            .get(self.pos..)
            .filter(|b| !b.is_empty())
            .map(|b| b.to_vec())
            .unwrap_or_else(|| vec![c]);
        self.err = Some(format!("invalid character {} {}", quote_rune(&at), context));
    }

    fn escape_error(&mut self, len: usize) {
        self.step = Step::Error;
        let end = (self.esc_start + len).min(self.data.len());
        self.err = Some(invalid_escape_sequence(&self.data[self.esc_start..end]));
    }

    fn push(&mut self, c: u8, p: Parse) {
        self.stack.push(p);
        if self.stack.len() > MAX_NESTING_DEPTH {
            self.error(c, "exceeded max depth");
        }
    }

    fn pop(&mut self) {
        self.stack.pop();
        if self.stack.is_empty() {
            self.step = Step::EndTop;
            self.end_top = true;
        } else {
            self.step = Step::EndValue;
        }
    }

    fn begin_value(&mut self, c: u8) {
        if is_space(c) {
            return;
        }
        match c {
            b'{' => {
                self.step = Step::BeginStringOrEmpty;
                self.push(c, Parse::ObjectKey);
            }
            b'[' => {
                self.step = Step::BeginValueOrEmpty;
                self.push(c, Parse::ArrayValue);
            }
            b'"' => self.step = Step::InString,
            b'-' => self.step = Step::Neg,
            b'0' => self.step = Step::Zero,
            b't' => self.step = Step::Lit(b"rue", 0, "true"),
            b'f' => self.step = Step::Lit(b"alse", 0, "false"),
            b'n' => self.step = Step::Lit(b"ull", 0, "null"),
            b'1'..=b'9' => self.step = Step::One,
            _ => self.error(c, "looking for beginning of value"),
        }
    }

    fn end_value(&mut self, c: u8) {
        let Some(ps) = self.stack.last().copied() else {
            self.step = Step::EndTop;
            self.end_top = true;
            self.end_top_step(c);
            return;
        };
        if is_space(c) {
            self.step = Step::EndValue;
            return;
        }
        match ps {
            Parse::ObjectKey => {
                if c == b':' {
                    *self.stack.last_mut().unwrap() = Parse::ObjectValue;
                    self.step = Step::BeginValue;
                } else {
                    self.error(c, "after object key");
                }
            }
            Parse::ObjectValue => {
                if c == b',' {
                    *self.stack.last_mut().unwrap() = Parse::ObjectKey;
                    self.step = Step::BeginString;
                } else if c == b'}' {
                    self.pop();
                } else {
                    self.error(c, "after object key:value pair");
                }
            }
            Parse::ArrayValue => {
                if c == b',' {
                    self.step = Step::BeginValue;
                } else if c == b']' {
                    self.pop();
                } else {
                    self.error(c, "after array element");
                }
            }
        }
    }

    fn end_top_step(&mut self, c: u8) {
        if !is_space(c) {
            self.error(c, "after top-level value");
        }
    }

    fn feed(&mut self, c: u8) {
        match self.step {
            Step::Error => {}
            Step::BeginValueOrEmpty => {
                if is_space(c) {
                } else if c == b']' {
                    self.end_value(c);
                } else {
                    self.begin_value(c);
                }
            }
            Step::BeginValue => self.begin_value(c),
            Step::BeginStringOrEmpty => {
                if is_space(c) {
                } else if c == b'}' {
                    *self.stack.last_mut().unwrap() = Parse::ObjectValue;
                    self.end_value(c);
                } else {
                    self.begin_string(c);
                }
            }
            Step::BeginString => self.begin_string(c),
            Step::EndValue => self.end_value(c),
            Step::EndTop => self.end_top_step(c),
            Step::InString => {
                if c == b'"' {
                    self.step = Step::EndValue;
                } else if c == b'\\' {
                    self.esc_start = self.pos;
                    self.step = Step::InStringEsc;
                } else if c < 0x20 {
                    self.error(c, "in string");
                }
            }
            Step::InStringEsc => match c {
                b'b' | b'f' | b'n' | b'r' | b't' | b'\\' | b'/' | b'"' => {
                    self.step = Step::InString
                }
                b'u' => self.step = Step::InStringEscU(0),
                _ => self.escape_error(2),
            },
            Step::InStringEscU(n) => {
                if is_hex(c) {
                    self.step = if n == 3 {
                        Step::InString
                    } else {
                        Step::InStringEscU(n + 1)
                    };
                } else {
                    self.escape_error(6);
                }
            }
            Step::Neg => match c {
                b'0' => self.step = Step::Zero,
                b'1'..=b'9' => self.step = Step::One,
                _ => self.error(c, "in numeric literal"),
            },
            Step::One => {
                if c.is_ascii_digit() {
                } else {
                    self.zero(c);
                }
            }
            Step::Zero => self.zero(c),
            Step::Dot => {
                if c.is_ascii_digit() {
                    self.step = Step::Dot0;
                } else {
                    self.error(c, "in numeric literal");
                }
            }
            Step::Dot0 => {
                if c.is_ascii_digit() {
                } else if c == b'e' || c == b'E' {
                    self.step = Step::E;
                } else {
                    self.end_value(c);
                }
            }
            Step::E => {
                if c == b'+' || c == b'-' {
                    self.step = Step::ESign;
                } else {
                    self.e_sign(c);
                }
            }
            Step::ESign => self.e_sign(c),
            Step::E0 => {
                if !c.is_ascii_digit() {
                    self.end_value(c);
                }
            }
            Step::Lit(rest, i, name) => {
                if c == rest[i] {
                    self.step = if i + 1 == rest.len() {
                        Step::EndValue
                    } else {
                        Step::Lit(rest, i + 1, name)
                    };
                } else {
                    self.error(
                        c,
                        &format!("in literal {name} (expecting {})", quote_rune(&rest[i..])),
                    );
                }
            }
        }
    }

    fn begin_string(&mut self, c: u8) {
        if is_space(c) {
        } else if c == b'"' {
            self.step = Step::InString;
        } else {
            self.error(c, "looking for beginning of object key string");
        }
    }

    fn zero(&mut self, c: u8) {
        if c == b'.' {
            self.step = Step::Dot;
        } else if c == b'e' || c == b'E' {
            self.step = Step::E;
        } else {
            self.end_value(c);
        }
    }

    fn e_sign(&mut self, c: u8) {
        if c.is_ascii_digit() {
            self.step = Step::E0;
        } else {
            self.error(c, "in numeric literal");
        }
    }
}

/// `json.NewDecoder(r).Decode(&m)` with `m map[string]string`: reads the first JSON value of `data` (trailing
/// bytes are ignored like the Go decoder does). `Ok(None)` for `null` (nil map). Entries in input order, the
/// last duplicate wins.
pub fn decode_string_map(data: &[u8]) -> Result<Option<Vec<(String, String)>>, String> {
    let mut s = Scanner::new(data);
    let mut end = None;
    for (i, &c) in data.iter().enumerate() {
        s.pos = i;
        s.feed(c);
        if s.end_top {
            end = Some(i);
            break;
        }
        if let Some(e) = &s.err {
            return Err(e.clone());
        }
    }
    if end.is_none() {
        s.pos = data.len();
        s.feed(b' ');
        if !s.end_top {
            return Err(if data.iter().any(|&c| !is_space(c)) {
                "unexpected EOF"
            } else {
                "EOF"
            }
            .to_string());
        }
    }
    unmarshal(data)
}

/// Kinds of JSON values, as named in Go's `UnmarshalTypeError`.
enum Val {
    Null,
    Str(String),
    Other(&'static str),
}

struct Parser<'a> {
    d: &'a [u8],
    i: usize,
}

impl Parser<'_> {
    fn ws(&mut self) {
        while self.i < self.d.len() && is_space(self.d[self.i]) {
            self.i += 1;
        }
    }

    fn string(&mut self) -> String {
        let start = self.i;
        self.i += 1;
        while self.d[self.i] != b'"' {
            if self.d[self.i] == b'\\' {
                self.i += 1;
            }
            self.i += 1;
        }
        self.i += 1;
        unquote(&self.d[start + 1..self.i - 1])
    }

    /// Skip a (valid) value, returning its kind.
    fn value(&mut self) -> Val {
        self.ws();
        match self.d[self.i] {
            b'"' => Val::Str(self.string()),
            b'{' | b'[' => {
                let kind = if self.d[self.i] == b'{' {
                    "object"
                } else {
                    "array"
                };
                let mut depth = 0usize;
                loop {
                    match self.d[self.i] {
                        b'"' => {
                            self.string();
                            continue;
                        }
                        b'{' | b'[' => depth += 1,
                        b'}' | b']' => {
                            depth -= 1;
                            if depth == 0 {
                                self.i += 1;
                                break;
                            }
                        }
                        _ => {}
                    }
                    self.i += 1;
                }
                Val::Other(kind)
            }
            b'n' => {
                self.i += 4;
                Val::Null
            }
            b't' => {
                self.i += 4;
                Val::Other("bool")
            }
            b'f' => {
                self.i += 5;
                Val::Other("bool")
            }
            _ => {
                while self.i < self.d.len()
                    && matches!(
                        self.d[self.i],
                        b'-' | b'+' | b'.' | b'e' | b'E' | b'0'..=b'9'
                    )
                {
                    self.i += 1;
                }
                Val::Other("number")
            }
        }
    }
}

/// `jsonwire.AppendUnquote` of the (valid) content of a string: lone surrogates and invalid UTF-8 bytes become
/// U+FFFD.
fn unquote(b: &[u8]) -> String {
    let hex = |b: &[u8]| -> Option<u32> {
        let s = std::str::from_utf8(b.get(..4)?).ok()?;
        if s.bytes().all(|c| c.is_ascii_hexdigit()) {
            u32::from_str_radix(s, 16).ok()
        } else {
            None
        }
    };
    let mut out = String::new();
    let mut i = 0;
    while i < b.len() {
        if b[i] != b'\\' {
            let len =
                (1..=(b.len() - i).min(4)).find(|&n| std::str::from_utf8(&b[i..i + n]).is_ok());
            match len {
                Some(n) => {
                    out.push_str(std::str::from_utf8(&b[i..i + n]).unwrap());
                    i += n;
                }
                None => {
                    out.push('\u{FFFD}');
                    i += 1;
                }
            }
            continue;
        }
        let c = b.get(i + 1).copied().unwrap_or(b'\\');
        i += 2;
        match c {
            b'b' => out.push('\u{8}'),
            b'f' => out.push('\u{c}'),
            b'n' => out.push('\n'),
            b'r' => out.push('\r'),
            b't' => out.push('\t'),
            b'u' => {
                let v1 = hex(&b[i..]).unwrap_or(0xFFFD);
                i += 4;
                if (0xD800..0xE000).contains(&v1) {
                    let v2 = if b.get(i) == Some(&b'\\') && b.get(i + 1) == Some(&b'u') {
                        hex(&b[i + 2..])
                    } else {
                        None
                    };
                    match v2 {
                        Some(v2)
                            if (0xD800..0xDC00).contains(&v1) && (0xDC00..0xE000).contains(&v2) =>
                        {
                            out.push(
                                char::from_u32(0x10000 + ((v1 - 0xD800) << 10) + (v2 - 0xDC00))
                                    .unwrap_or('\u{FFFD}'),
                            );
                            i += 6;
                        }
                        _ => out.push('\u{FFFD}'),
                    }
                } else {
                    out.push(char::from_u32(v1).unwrap_or('\u{FFFD}'));
                }
            }
            other => out.push(char::from(other)),
        }
    }
    out
}

/// Unmarshal of a syntactically valid first value into `map[string]string` (Go keeps the first type error).
fn unmarshal(data: &[u8]) -> Result<Option<Vec<(String, String)>>, String> {
    let mut p = Parser { d: data, i: 0 };
    p.ws();
    match data[p.i] {
        b'{' => {}
        _ => {
            return match p.value() {
                Val::Null => Ok(None),
                Val::Str(_) => Err(
                    "json: cannot unmarshal string into Go value of type map[string]string"
                        .to_string(),
                ),
                Val::Other(k) => Err(format!(
                    "json: cannot unmarshal {k} into Go value of type map[string]string"
                )),
            };
        }
    }
    p.i += 1;
    let mut out: Vec<(String, String)> = Vec::new();
    let mut saved: Option<String> = None;
    loop {
        p.ws();
        if p.d[p.i] == b'}' {
            break;
        }
        let key = p.string();
        p.ws();
        p.i += 1; // ':'
        let v = match p.value() {
            Val::Str(s) => s,
            Val::Null => String::new(),
            Val::Other(k) => {
                if saved.is_none() {
                    saved = Some(format!(
                        "json: cannot unmarshal {k} into Go struct field .{key} of type string"
                    ));
                }
                String::new()
            }
        };
        match out.iter_mut().find(|(k, _)| *k == key) {
            Some(e) => e.1 = v,
            None => out.push((key, v)),
        }
        p.ws();
        if p.d[p.i] == b',' {
            p.i += 1;
        }
    }
    match saved {
        Some(e) => Err(e),
        None => Ok(Some(out)),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn err(s: &str) -> String {
        decode_string_map(s.as_bytes()).unwrap_err()
    }

    #[test]
    fn decode() {
        assert_eq!(err(""), "EOF");
        assert_eq!(err("  \n"), "EOF");
        assert_eq!(err("{\"a\":"), "unexpected EOF");
        assert_eq!(
            err("x"),
            "invalid character 'x' looking for beginning of value"
        );
        assert_eq!(err("{\"a\" 1}"), "invalid character '1' after object key");
        assert_eq!(
            err("[]"),
            "json: cannot unmarshal array into Go value of type map[string]string"
        );
        assert_eq!(
            err("{\"A\":1}"),
            "json: cannot unmarshal number into Go struct field .A of type string"
        );
        assert_eq!(
            err("{\"A\":\"\\x\"}"),
            "invalid escape sequence `\\x` in string"
        );
        assert_eq!(
            err("{\"A\":\"\\u12\"}"),
            "invalid escape sequence `\\u12\"}` in string"
        );
        assert_eq!(
            err("\u{feff}{}"),
            "invalid character '\\ufeff' looking for beginning of value"
        );
        assert_eq!(
            err("{\"A\":1.}"),
            "invalid character '}' in numeric literal"
        );
        assert_eq!(
            decode_string_map(b"{\"A\":\"\\ud800x\\ud83d\\ude00\"}").unwrap(),
            Some(vec![("A".to_string(), "\u{FFFD}x\u{1F600}".to_string())])
        );
        assert_eq!(decode_string_map(b"null").unwrap(), None);
        assert_eq!(
            decode_string_map(b"{\"b\":\"x\",\"a\":null,\"b\":\"y\"} trailing").unwrap(),
            Some(vec![
                ("b".to_string(), "y".to_string()),
                ("a".to_string(), String::new())
            ])
        );
        assert_eq!(
            err("1"),
            "json: cannot unmarshal number into Go value of type map[string]string"
        );
    }
}
