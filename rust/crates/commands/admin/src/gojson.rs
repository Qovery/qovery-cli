//! Go `encoding/json` (as forked by go-jose) error messages: `json.Unmarshal(data, &map[string]string{})`
//! and the syntax check (`checkValid`) whose errors are printed by the Go commands.

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

struct Scanner {
    step: Step,
    stack: Vec<Parse>,
    end_top: bool,
    err: Option<String>,
}

const MAX_NESTING_DEPTH: usize = 10000;

/// Go `quoteChar`.
fn quote_char(c: u8) -> String {
    match c {
        b'\'' => "'\\''".to_string(),
        b'"' => "'\"'".to_string(),
        _ => {
            let q = qovery_log::gofmt::quote(&char::from(c).to_string());
            format!("'{}'", &q[1..q.len() - 1])
        }
    }
}

fn is_space(c: u8) -> bool {
    matches!(c, b' ' | b'\t' | b'\r' | b'\n')
}

fn is_hex(c: u8) -> bool {
    c.is_ascii_hexdigit()
}

impl Scanner {
    fn new() -> Self {
        Scanner {
            step: Step::BeginValue,
            stack: Vec::new(),
            end_top: false,
            err: None,
        }
    }

    fn error(&mut self, c: u8, context: &str) {
        self.step = Step::Error;
        self.err = Some(format!("invalid character {} {}", quote_char(c), context));
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
                    self.step = Step::InStringEsc;
                } else if c < 0x20 {
                    self.error(c, "in string literal");
                }
            }
            Step::InStringEsc => match c {
                b'b' | b'f' | b'n' | b'r' | b't' | b'\\' | b'/' | b'"' => {
                    self.step = Step::InString
                }
                b'u' => self.step = Step::InStringEscU(0),
                _ => self.error(c, "in string escape code"),
            },
            Step::InStringEscU(n) => {
                if is_hex(c) {
                    self.step = if n == 3 {
                        Step::InString
                    } else {
                        Step::InStringEscU(n + 1)
                    };
                } else {
                    self.error(c, "in \\u hexadecimal character escape");
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
                    self.error(c, "after decimal point in numeric literal");
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
                        &format!("in literal {name} (expecting {})", quote_char(rest[i])),
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
            self.error(c, "in exponent of numeric literal");
        }
    }

    fn eof(&mut self) -> Result<(), String> {
        if let Some(e) = &self.err {
            return Err(e.clone());
        }
        if self.end_top {
            return Ok(());
        }
        self.feed(b' ');
        if self.end_top && self.err.is_none() {
            return Ok(());
        }
        Err(self
            .err
            .clone()
            .unwrap_or_else(|| "unexpected end of JSON input".to_string()))
    }
}

/// Go `checkValid(data)`.
pub fn check_valid(data: &[u8]) -> Result<(), String> {
    let mut s = Scanner::new();
    for &c in data {
        s.feed(c);
        if let Some(e) = &s.err {
            return Err(e.clone());
        }
    }
    s.eof()
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
        let raw = std::str::from_utf8(&self.d[start..self.i]).unwrap_or("\"\"");
        serde_json::from_str::<String>(raw)
            .unwrap_or_else(|_| String::from_utf8_lossy(&self.d[start + 1..self.i - 1]).to_string())
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

/// go-jose `json.Unmarshal(data, &m)` with `m map[string]string`. `Ok(None)` for `null` (nil map).
/// Map entries are returned in input order (later duplicates are rejected like go-jose does).
pub fn unmarshal_string_map(data: &[u8]) -> Result<Option<Vec<(String, String)>>, String> {
    check_valid(data)?;
    let mut p = Parser { d: data, i: 0 };
    p.ws();
    let type_err =
        |kind: &str, t: &str| format!("json: cannot unmarshal {kind} into Go value of type {t}");
    match data[p.i] {
        b'n' => return Ok(None),
        b'{' => {}
        _ => {
            return match p.value() {
                Val::Str(_) => Err(type_err("string", "map[string]string")),
                Val::Other(k) => Err(type_err(k, "map[string]string")),
                Val::Null => Ok(None),
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
        if out.iter().any(|(k, _)| *k == key) {
            return Err(format!("json: duplicate key '{key}' in object"));
        }
        p.ws();
        p.i += 1; // ':'
        let v = match p.value() {
            Val::Str(s) => s,
            Val::Null => String::new(),
            // go-jose `literalStore` fails at once on a number (`d.error`), unlike the other kinds (`d.saveError`)
            Val::Other("number") => return Err(type_err("number", "string")),
            Val::Other(k) => {
                if saved.is_none() {
                    saved = Some(type_err(k, "string"));
                }
                String::new()
            }
        };
        out.push((key, v));
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

    #[test]
    fn syntax_errors() {
        let e = |s: &str| check_valid(s.as_bytes()).unwrap_err();
        assert_eq!(
            e("foo"),
            "invalid character 'o' in literal false (expecting 'a')"
        );
        assert_eq!(
            e("x"),
            "invalid character 'x' looking for beginning of value"
        );
        assert_eq!(e(""), "unexpected end of JSON input");
        assert_eq!(e("{"), "unexpected end of JSON input");
        assert_eq!(
            e("{\"a\":1,}"),
            "invalid character '}' looking for beginning of object key string"
        );
        assert_eq!(e("{\"a\" 1}"), "invalid character '1' after object key");
        assert_eq!(e("{} x"), "invalid character 'x' after top-level value");
        assert_eq!(
            e("1."),
            "invalid character ' ' after decimal point in numeric literal"
        );
        assert_eq!(
            e("[1,]"),
            "invalid character ']' looking for beginning of value"
        );
        assert_eq!(
            e("{'a':1}"),
            "invalid character '\\'' looking for beginning of object key string"
        );
        assert!(check_valid(b" {\"a\": [1, 2.5e3, true, null, {}]} ").is_ok());
    }

    #[test]
    fn string_maps() {
        assert_eq!(unmarshal_string_map(b"null").unwrap(), None);
        assert_eq!(
            unmarshal_string_map(b"{\"b\":\"x\",\"a\":null}").unwrap(),
            Some(vec![
                ("b".to_string(), "x".to_string()),
                ("a".to_string(), String::new())
            ])
        );
        assert_eq!(
            unmarshal_string_map(b"[]").unwrap_err(),
            "json: cannot unmarshal array into Go value of type map[string]string"
        );
        assert_eq!(
            unmarshal_string_map(b"{\"a\":1}").unwrap_err(),
            "json: cannot unmarshal number into Go value of type string"
        );
        assert_eq!(
            unmarshal_string_map(b"{\"a\":true,\"b\":[1]}").unwrap_err(),
            "json: cannot unmarshal bool into Go value of type string"
        );
        // go-jose stops at a number (`d.error`) but only saves the first bool/object/array error (`d.saveError`)
        assert_eq!(
            unmarshal_string_map(b"{\"a\":true,\"b\":1}").unwrap_err(),
            "json: cannot unmarshal number into Go value of type string"
        );
        assert_eq!(
            unmarshal_string_map(b"{\"a\":\"1\",\"a\":\"2\"}").unwrap_err(),
            "json: duplicate key 'a' in object"
        );
    }
}
