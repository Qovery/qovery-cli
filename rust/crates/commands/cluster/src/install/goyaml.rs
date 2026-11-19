//! `gopkg.in/yaml.v3` compatible `Unmarshal` into `map[string]interface{}` and `Marshal` back.
//!
//! The install command rewrites Helm values files with yaml.v3: the output (key order, indentation of 4,
//! quoting styles, block scalars) must be byte-identical. The emitter is a port of the parts of the libyaml
//! emitter used by yaml.v3 for block mappings/sequences of scalars (no comments, anchors or tags).

use serde_yaml::Value as Y;
use std::sync::LazyLock;

/// A decoded YAML value, as yaml.v3 decodes into `interface{}`.
#[derive(Debug, Clone, PartialEq)]
pub enum GoYaml {
    Null,
    Bool(bool),
    Int(i64),
    Uint(u64),
    Float(f64),
    Str(String),
    Seq(Vec<GoYaml>),
    Map(Vec<(String, GoYaml)>),
    /// `map[interface{}]interface{}`: a nested mapping with a key that does not resolve to a string (`8080:`,
    /// `true:`); yaml.v3 keeps the typed keys and re-encodes them (`0x1F` -> `31`, `1e6` -> `1e+06`).
    AnyMap(Vec<(GoYaml, GoYaml)>),
}

impl GoYaml {
    pub fn get(&self, key: &str) -> Option<&GoYaml> {
        match self {
            GoYaml::Map(m) => m.iter().find(|(k, _)| k == key).map(|(_, v)| v),
            _ => None,
        }
    }

    pub fn get_mut(&mut self, key: &str) -> Option<&mut GoYaml> {
        match self {
            GoYaml::Map(m) => m.iter_mut().find(|(k, _)| k == key).map(|(_, v)| v),
            _ => None,
        }
    }

    /// `m[key] = value` on a map.
    pub fn set(&mut self, key: &str, value: GoYaml) {
        if let GoYaml::Map(m) = self {
            match m.iter_mut().find(|(k, _)| k == key) {
                Some((_, v)) => *v = value,
                None => m.push((key.to_string(), value)),
            }
        }
    }

    pub fn contains_key(&self, key: &str) -> bool {
        self.get(key).is_some()
    }

    pub fn is_map(&self) -> bool {
        matches!(self, GoYaml::Map(_))
    }

    pub fn as_str(&self) -> Option<&str> {
        match self {
            GoYaml::Str(s) => Some(s),
            _ => None,
        }
    }
}

fn scalar_key(v: &Y) -> String {
    match v {
        Y::String(s) => s.clone(),
        Y::Bool(b) => b.to_string(),
        Y::Number(n) => n.to_string(),
        Y::Null => "null".to_string(),
        Y::Tagged(t) => scalar_key(&t.value),
        other => serde_yaml::to_string(other)
            .unwrap_or_default()
            .trim_end()
            .to_string(),
    }
}

fn convert(v: Y) -> GoYaml {
    match v {
        Y::Null => GoYaml::Null,
        Y::Bool(b) => GoYaml::Bool(b),
        Y::Number(n) => {
            if let Some(i) = n.as_i64() {
                GoYaml::Int(i)
            } else if let Some(u) = n.as_u64() {
                GoYaml::Uint(u)
            } else {
                GoYaml::Float(n.as_f64().unwrap_or(0.0))
            }
        }
        Y::String(s) => GoYaml::Str(s),
        Y::Sequence(s) => GoYaml::Seq(s.into_iter().map(convert).collect()),
        Y::Mapping(m) => {
            // yaml.v3 `isStringMap`
            if m.keys().all(is_string_key) {
                GoYaml::Map(
                    m.into_iter()
                        .map(|(k, v)| (scalar_key(&k), convert(v)))
                        .collect(),
                )
            } else {
                GoYaml::AnyMap(
                    m.into_iter()
                        .map(|(k, v)| (convert(k), convert(v)))
                        .collect(),
                )
            }
        }
        Y::Tagged(t) => {
            if t.tag == "!!str" || t.tag == "str" {
                match t.value {
                    Y::String(s) => GoYaml::Str(s),
                    other => GoYaml::Str(scalar_key(&other)),
                }
            } else {
                convert(t.value)
            }
        }
    }
}

fn is_string_key(k: &Y) -> bool {
    match k {
        Y::String(_) => true,
        Y::Tagged(t) => t.tag == "!!str" || t.tag == "str",
        _ => false,
    }
}

/// The keys of the top-level mapping as written (plain scalar text), in document order.
struct RawKeys(Vec<String>);

impl<'de> serde::Deserialize<'de> for RawKeys {
    fn deserialize<D: serde::Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        struct V;
        impl<'de> serde::de::Visitor<'de> for V {
            type Value = RawKeys;
            fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
                f.write_str("a mapping")
            }
            fn visit_map<A: serde::de::MapAccess<'de>>(
                self,
                mut map: A,
            ) -> Result<RawKeys, A::Error> {
                let mut keys = Vec::new();
                while let Some((k, _)) = map.next_entry::<String, serde::de::IgnoredAny>()? {
                    keys.push(k);
                }
                Ok(RawKeys(keys))
            }
        }
        d.deserialize_map(V)
    }
}

/// `yaml.Unmarshal([]byte(s), &map[string]interface{})`.
///
/// The top-level keys decode into Go strings: yaml.v3 keeps their text as written (`1e6`, `0x1F`, `true`) and
/// skips the null ones; nested mappings decode into `interface{}` (typed keys, see [`GoYaml::AnyMap`]).
pub fn unmarshal(s: &str) -> Result<GoYaml, String> {
    let mut v: Y = serde_yaml::from_str(s).map_err(|e| format!("yaml: {e}"))?;
    v.apply_merge().map_err(|e| format!("yaml: {e}"))?;
    if let Y::Mapping(m) = v {
        let mut raw_keys: Vec<Option<String>> = serde_yaml::from_str::<RawKeys>(s)
            .map(|k| k.0.into_iter().map(Some).collect())
            .unwrap_or_default();
        let mut entries = Vec::new();
        for (k, v) in m {
            let key = match &k {
                Y::Null => continue,
                k if is_string_key(k) => scalar_key(k),
                k => raw_keys
                    .iter_mut()
                    .find(|raw| {
                        raw.as_deref()
                            .is_some_and(|r| serde_yaml::from_str::<Y>(r).ok().as_ref() == Some(k))
                    })
                    .and_then(Option::take)
                    // keys brought by a merge (`<<`) are not in the top-level text
                    .unwrap_or_else(|| scalar_key(k)),
            };
            entries.push((key, convert(v)));
        }
        return Ok(GoYaml::Map(entries));
    }
    match convert(v) {
        GoYaml::Null => Ok(GoYaml::Map(vec![])),
        m @ GoYaml::Map(_) => Ok(m),
        _ => {
            Err("yaml: unmarshal errors: cannot unmarshal into map[string]interface {}".to_string())
        }
    }
}

/// yaml.v3 `keyList.Less` for string keys (letters after other characters, digit runs compared as numbers).
pub fn key_less(a: &str, b: &str) -> bool {
    let ar: Vec<char> = a.chars().collect();
    let br: Vec<char> = b.chars().collect();
    let mut digits = false;
    let mut i = 0;
    while i < ar.len() && i < br.len() {
        if ar[i] == br[i] {
            digits = ar[i].is_numeric();
            i += 1;
            continue;
        }
        let al = ar[i].is_alphabetic();
        let bl = br[i].is_alphabetic();
        if al && bl {
            return ar[i] < br[i];
        }
        if al || bl {
            return if digits { al } else { bl };
        }
        let mut an: i64 = 0;
        let mut bn: i64 = 0;
        if ar[i] == '0' || br[i] == '0' {
            let mut j = i as isize - 1;
            while j >= 0 && ar[j as usize].is_numeric() {
                if ar[j as usize] != '0' {
                    an = 1;
                    bn = 1;
                    break;
                }
                j -= 1;
            }
        }
        let mut ai = i;
        while ai < ar.len() && ar[ai].is_ascii_digit() {
            an = an.wrapping_mul(10).wrapping_add(ar[ai] as i64 - '0' as i64);
            ai += 1;
        }
        let mut bi = i;
        while bi < br.len() && br[bi].is_ascii_digit() {
            bn = bn.wrapping_mul(10).wrapping_add(br[bi] as i64 - '0' as i64);
            bi += 1;
        }
        if an != bn {
            return an < bn;
        }
        if ai != bi {
            return ai < bi;
        }
        return ar[i] < br[i];
    }
    ar.len() < br.len()
}

/// yaml.v3 `keyList.Less` for the typed keys of a `map[interface{}]interface{}`: numbers and bools (as 1/0)
/// compared by value, then by `reflect.Kind`; other kinds by `reflect.Kind`; strings with [`key_less`].
fn any_key_less(a: &GoYaml, b: &GoYaml) -> bool {
    // reflect.Kind: Bool 1, Int 2, Uint64 11, Float64 14, Interface (nil) 20, String 24, Slice 23, Map 21
    fn kind(v: &GoYaml) -> u8 {
        match v {
            GoYaml::Bool(_) => 1,
            GoYaml::Int(_) => 2,
            GoYaml::Uint(_) => 11,
            GoYaml::Float(_) => 14,
            GoYaml::Null => 20,
            GoYaml::Map(_) | GoYaml::AnyMap(_) => 21,
            GoYaml::Seq(_) => 23,
            GoYaml::Str(_) => 24,
        }
    }
    fn key_float(v: &GoYaml) -> Option<f64> {
        match v {
            GoYaml::Bool(b) => Some(if *b { 1.0 } else { 0.0 }),
            GoYaml::Int(i) => Some(*i as f64),
            GoYaml::Uint(u) => Some(*u as f64),
            GoYaml::Float(f) => Some(*f),
            _ => None,
        }
    }
    let (ak, bk) = (kind(a), kind(b));
    if let (Some(af), Some(bf)) = (key_float(a), key_float(b)) {
        if af != bf {
            return af < bf;
        }
        if ak != bk {
            return ak < bk;
        }
        // numLess
        return match (a, b) {
            (GoYaml::Int(x), GoYaml::Int(y)) => x < y,
            (GoYaml::Uint(x), GoYaml::Uint(y)) => x < y,
            (GoYaml::Bool(x), GoYaml::Bool(y)) => !x & y,
            _ => false,
        };
    }
    match (a, b) {
        (GoYaml::Str(x), GoYaml::Str(y)) => key_less(x, y),
        _ => ak < bk,
    }
}

fn ordering(less: impl Fn() -> bool, greater: impl Fn() -> bool) -> std::cmp::Ordering {
    if less() {
        std::cmp::Ordering::Less
    } else if greater() {
        std::cmp::Ordering::Greater
    } else {
        std::cmp::Ordering::Equal
    }
}

/// Go `strconv.FormatFloat(f, 'g', -1, 64)` as used by yaml.v3.
pub fn format_float(f: f64) -> String {
    if f.is_nan() {
        return ".nan".to_string();
    }
    if f.is_infinite() {
        return if f > 0.0 {
            ".inf".to_string()
        } else {
            "-.inf".to_string()
        };
    }
    if f == 0.0 {
        return if f.is_sign_negative() {
            "-0".to_string()
        } else {
            "0".to_string()
        };
    }
    let e = format!("{:e}", f); // shortest digits: "-1.2345e6"
    let (mant, exp) = e.split_once('e').unwrap_or((&e, "0"));
    let exp: i32 = exp.parse().unwrap_or(0);
    let neg = mant.starts_with('-');
    let digits: String = mant.chars().filter(|c| c.is_ascii_digit()).collect();
    let sign = if neg { "-" } else { "" };
    // 'g' with the shortest precision: %e when exp < -4 || exp >= 6
    if !(-4..6).contains(&exp) {
        // %e with the shortest digits
        let m = if digits.len() > 1 {
            format!("{}.{}", &digits[..1], &digits[1..])
        } else {
            digits.clone()
        };
        let esign = if exp < 0 { '-' } else { '+' };
        return format!("{sign}{m}e{esign}{:02}", exp.abs());
    }
    // %f
    let dp = exp + 1; // position of the decimal point in digits
    let out = if dp <= 0 {
        format!("0.{}{}", "0".repeat((-dp) as usize), digits)
    } else if dp as usize >= digits.len() {
        format!("{}{}", digits, "0".repeat(dp as usize - digits.len()))
    } else {
        format!("{}.{}", &digits[..dp as usize], &digits[dp as usize..])
    };
    format!("{sign}{out}")
}

/// yaml.v3 `resolve("", s)` is not the string tag, or the string is an old bool / base 60 float.
fn needs_quotes(s: &str) -> bool {
    if matches!(
        s,
        "" | "~"
            | "null"
            | "Null"
            | "NULL"
            | "true"
            | "True"
            | "TRUE"
            | "false"
            | "False"
            | "FALSE"
            | ".inf"
            | ".Inf"
            | ".INF"
            | "+.inf"
            | "+.Inf"
            | "+.INF"
            | "-.inf"
            | "-.Inf"
            | "-.INF"
            | ".nan"
            | ".NaN"
            | ".NAN"
            | "<<"
    ) {
        return true;
    }
    if matches!(
        s,
        "y" | "Y"
            | "yes"
            | "Yes"
            | "YES"
            | "on"
            | "On"
            | "ON"
            | "n"
            | "N"
            | "no"
            | "No"
            | "NO"
            | "off"
            | "Off"
            | "OFF"
    ) {
        return true;
    }
    let c = s.as_bytes()[0];
    if !(c == b'+' || c == b'-' || c == b'.' || c.is_ascii_digit()) {
        return false;
    }
    // ints (with yaml.v3 underscore removal), floats, timestamps, base 60 floats
    let plain = s.replace('_', "");
    let int_like = {
        let t = plain.strip_prefix(['+', '-']).unwrap_or(&plain);
        !t.is_empty()
            && (t.chars().all(|c| c.is_ascii_digit())
                || (t.starts_with("0x")
                    && t.len() > 2
                    && t[2..].chars().all(|c| c.is_ascii_hexdigit()))
                || (t.starts_with("0o")
                    && t.len() > 2
                    && t[2..].chars().all(|c| ('0'..='7').contains(&c)))
                || (t.starts_with("0b")
                    && t.len() > 2
                    && t[2..].chars().all(|c| c == '0' || c == '1')))
    };
    if int_like {
        return true;
    }
    static FLOAT_RE: LazyLock<regex::Regex> = LazyLock::new(|| {
        regex::Regex::new(r"^[-+]?(\.[0-9]+|[0-9]+(\.[0-9]*)?)([eE][-+]?[0-9]+)?$")
            .expect("valid regex")
    });
    static TIMESTAMP_RE: LazyLock<regex::Regex> = LazyLock::new(|| {
        regex::Regex::new(r"^\d{4}-\d{1,2}-\d{1,2}([Tt ]\d{1,2}:\d{1,2}:\d{1,2}(\.\d*)?\s*(Z|[-+]\d{1,2}(:\d{2})?)?)?$")
            .expect("valid regex")
    });
    static BASE60_RE: LazyLock<regex::Regex> = LazyLock::new(|| {
        regex::Regex::new(r"^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$")
            .expect("valid regex")
    });
    if FLOAT_RE.is_match(&plain) {
        return true;
    }
    if TIMESTAMP_RE.is_match(s) {
        return true;
    }
    s.contains(':') && BASE60_RE.is_match(s)
}

#[derive(Clone, Copy, PartialEq, Debug)]
enum Style {
    Plain,
    Single,
    Double,
    Literal,
}

#[derive(Clone, Copy, PartialEq)]
enum State {
    SeqItem,
    MapKey,
    MapSimpleValue,
    Other,
}

struct Analysis {
    multiline: bool,
    block_plain_allowed: bool,
    single_quoted_allowed: bool,
    block_allowed: bool,
}

fn is_printable_char(c: char) -> bool {
    let u = c as u32;
    u == 0x0A
        || (0x20..=0x7E).contains(&u)
        || (0xA0..=0xD7FF).contains(&u)
        || ((0xE000..=0xFFFD).contains(&u) && u != 0xFEFF)
        || (0x10000..=0x10FFFF).contains(&u)
}

fn is_break_char(c: char) -> bool {
    matches!(c, '\r' | '\n' | '\u{85}' | '\u{2028}' | '\u{2029}')
}

fn analyze(value: &str) -> Analysis {
    if value.is_empty() {
        return Analysis {
            multiline: false,
            block_plain_allowed: true,
            single_quoted_allowed: true,
            block_allowed: false,
        };
    }
    let chars: Vec<char> = value.chars().collect();
    let mut block_indicators = value.starts_with("---") || value.starts_with("...");
    let mut line_breaks = false;
    let mut special_characters = false;
    let mut tab_characters = false;
    let (mut leading_space, mut leading_break, mut trailing_space, mut trailing_break) =
        (false, false, false, false);
    let (mut break_space, mut space_break) = (false, false);
    let mut preceded_by_whitespace = true;
    let (mut previous_space, mut previous_break) = (false, false);
    let n = chars.len();
    for i in 0..n {
        let c = chars[i];
        let followed_by_whitespace =
            i + 1 >= n || matches!(chars[i + 1], ' ' | '\t') || is_break_char(chars[i + 1]);
        if i == 0 {
            match c {
                '#' | ',' | '[' | ']' | '{' | '}' | '&' | '*' | '!' | '|' | '>' | '\'' | '"'
                | '%' | '@' | '`' => block_indicators = true,
                '?' | ':' => {
                    if followed_by_whitespace {
                        block_indicators = true
                    }
                }
                '-' if followed_by_whitespace => block_indicators = true,
                _ => {}
            }
        } else {
            match c {
                ':' => {
                    if followed_by_whitespace {
                        block_indicators = true
                    }
                }
                '#' if preceded_by_whitespace => block_indicators = true,
                _ => {}
            }
        }
        if c == '\t' {
            tab_characters = true;
        } else if !is_printable_char(c) {
            special_characters = true;
        }
        if c == ' ' {
            if i == 0 {
                leading_space = true;
            }
            if i + 1 == n {
                trailing_space = true;
            }
            if previous_break {
                break_space = true;
            }
            previous_space = true;
            previous_break = false;
        } else if is_break_char(c) {
            line_breaks = true;
            if i == 0 {
                leading_break = true;
            }
            if i + 1 == n {
                trailing_break = true;
            }
            if previous_space {
                space_break = true;
            }
            previous_space = false;
            previous_break = true;
        } else {
            previous_space = false;
            previous_break = false;
        }
        preceded_by_whitespace = matches!(c, ' ' | '\t') || is_break_char(c);
    }
    let mut a = Analysis {
        multiline: line_breaks,
        block_plain_allowed: true,
        single_quoted_allowed: true,
        block_allowed: true,
    };
    if leading_space || leading_break || trailing_space || trailing_break {
        a.block_plain_allowed = false;
    }
    if trailing_space {
        a.block_allowed = false;
    }
    if break_space {
        a.block_plain_allowed = false;
        a.single_quoted_allowed = false;
    }
    if space_break || tab_characters || special_characters {
        a.block_plain_allowed = false;
        a.single_quoted_allowed = false;
    }
    if space_break || special_characters {
        a.block_allowed = false;
    }
    if line_breaks {
        a.block_plain_allowed = false;
    }
    if block_indicators {
        a.block_plain_allowed = false;
    }
    a
}

struct Emitter {
    out: String,
    column: i32,
    whitespace: bool,
    indention: bool,
    indent: i32,
    indents: Vec<i32>,
    states: Vec<State>,
}

const BEST_INDENT: i32 = 4;

impl Emitter {
    fn put(&mut self, c: char) {
        self.out.push(c);
        self.column += 1;
    }

    fn put_break(&mut self) {
        self.out.push('\n');
        self.column = 0;
        self.indention = true;
    }

    fn write_str(&mut self, s: &str) {
        for c in s.chars() {
            self.put(c);
        }
    }

    fn increase_indent(&mut self, flow: bool) {
        self.indents.push(self.indent);
        if self.indent < 0 {
            self.indent = if flow { BEST_INDENT } else { 0 };
        } else if self.states.last() == Some(&State::SeqItem) {
            self.indent += 2;
        } else {
            self.indent = BEST_INDENT * ((self.indent + BEST_INDENT) / BEST_INDENT);
        }
    }

    fn pop_indent(&mut self) {
        self.indent = self.indents.pop().unwrap_or(-1);
    }

    fn write_indent(&mut self) {
        let indent = self.indent.max(0);
        if !self.indention || self.column > indent || (self.column == indent && !self.whitespace) {
            self.put_break();
        }
        while self.column < indent {
            self.put(' ');
        }
        self.whitespace = true;
    }

    fn write_indicator(
        &mut self,
        indicator: &str,
        need_whitespace: bool,
        is_whitespace: bool,
        is_indention: bool,
    ) {
        if need_whitespace && !self.whitespace {
            self.put(' ');
        }
        self.write_str(indicator);
        self.whitespace = is_whitespace;
        self.indention = self.indention && is_indention;
    }

    fn emit(&mut self, v: &GoYaml, simple_key: bool) {
        match v {
            GoYaml::Null => self.emit_scalar("null", Style::Plain, simple_key),
            GoYaml::Bool(b) => {
                self.emit_scalar(if *b { "true" } else { "false" }, Style::Plain, simple_key)
            }
            GoYaml::Int(i) => self.emit_scalar(&i.to_string(), Style::Plain, simple_key),
            GoYaml::Uint(u) => self.emit_scalar(&u.to_string(), Style::Plain, simple_key),
            GoYaml::Float(f) => self.emit_scalar(&format_float(*f), Style::Plain, simple_key),
            GoYaml::Str(s) => {
                let style = if s.contains('\n') {
                    Style::Literal
                } else if !needs_quotes(s) {
                    Style::Plain
                } else {
                    Style::Double
                };
                self.emit_scalar(s, style, simple_key)
            }
            GoYaml::Seq(items) => {
                if items.is_empty() {
                    self.write_indicator("[", true, true, false);
                    self.write_indicator("]", false, false, false);
                    self.states.pop();
                    return;
                }
                self.increase_indent(false);
                for item in items {
                    self.write_indent();
                    self.write_indicator("-", true, false, true);
                    self.states.push(State::SeqItem);
                    self.emit(item, false);
                }
                self.pop_indent();
                self.states.pop();
            }
            GoYaml::Map(entries) => {
                if entries.is_empty() {
                    self.write_indicator("{", true, true, false);
                    self.write_indicator("}", false, false, false);
                    self.states.pop();
                    return;
                }
                let mut sorted: Vec<(GoYaml, &GoYaml)> = entries
                    .iter()
                    .map(|(k, v)| (GoYaml::Str(k.clone()), v))
                    .collect();
                sorted.sort_by(|a, b| {
                    ordering(|| any_key_less(&a.0, &b.0), || any_key_less(&b.0, &a.0))
                });
                self.emit_mapping(&sorted);
            }
            GoYaml::AnyMap(entries) => {
                if entries.is_empty() {
                    self.write_indicator("{", true, true, false);
                    self.write_indicator("}", false, false, false);
                    self.states.pop();
                    return;
                }
                let mut sorted: Vec<(GoYaml, &GoYaml)> =
                    entries.iter().map(|(k, v)| (k.clone(), v)).collect();
                sorted.sort_by(|a, b| {
                    ordering(|| any_key_less(&a.0, &b.0), || any_key_less(&b.0, &a.0))
                });
                self.emit_mapping(&sorted);
            }
        }
    }

    /// A non-empty block mapping whose entries are already sorted.
    fn emit_mapping(&mut self, sorted: &[(GoYaml, &GoYaml)]) {
        self.increase_indent(false);
        for (k, val) in sorted {
            self.write_indent();
            self.states.push(State::MapSimpleValue);
            self.emit(k, true);
            self.write_indicator(":", false, false, false);
            self.states.push(State::MapKey);
            self.emit(val, false);
        }
        self.pop_indent();
        self.states.pop();
    }

    fn emit_scalar(&mut self, value: &str, requested: Style, simple_key: bool) {
        let a = analyze(value);
        let mut style = requested;
        if simple_key && a.multiline {
            style = Style::Double;
        }
        if style == Style::Plain {
            if !a.block_plain_allowed {
                style = Style::Single;
            }
            if value.is_empty() && simple_key {
                style = Style::Single;
            }
        }
        if style == Style::Single && !a.single_quoted_allowed {
            style = Style::Double;
        }
        if style == Style::Literal && (!a.block_allowed || simple_key) {
            style = Style::Double;
        }
        self.increase_indent(true);
        match style {
            Style::Plain => {
                if !value.is_empty() && !self.whitespace {
                    self.put(' ');
                }
                self.write_str(value);
                if !value.is_empty() {
                    self.whitespace = false;
                }
                self.indention = false;
            }
            Style::Single => {
                self.write_indicator("'", true, false, false);
                for c in value.chars() {
                    if c == '\'' {
                        self.put('\'');
                    }
                    self.put(c);
                }
                self.write_indicator("'", false, false, false);
                self.whitespace = false;
                self.indention = false;
            }
            Style::Double => {
                self.write_indicator("\"", true, false, false);
                for c in value.chars() {
                    if !is_printable_char(c)
                        || c == '\u{FEFF}'
                        || is_break_char(c)
                        || c == '"'
                        || c == '\\'
                    {
                        self.put('\\');
                        let u = c as u32;
                        match u {
                            0x00 => self.put('0'),
                            0x07 => self.put('a'),
                            0x08 => self.put('b'),
                            0x09 => self.put('t'),
                            0x0A => self.put('n'),
                            0x0B => self.put('v'),
                            0x0C => self.put('f'),
                            0x0D => self.put('r'),
                            0x1B => self.put('e'),
                            0x22 => self.put('"'),
                            0x5C => self.put('\\'),
                            0x85 => self.put('N'),
                            0xA0 => self.put('_'),
                            0x2028 => self.put('L'),
                            0x2029 => self.put('P'),
                            _ => {
                                let s = if u <= 0xFF {
                                    format!("x{u:02X}")
                                } else if u <= 0xFFFF {
                                    format!("u{u:04X}")
                                } else {
                                    format!("U{u:08X}")
                                };
                                self.write_str(&s);
                            }
                        }
                    } else {
                        self.put(c);
                    }
                }
                self.write_indicator("\"", false, false, false);
                self.whitespace = false;
                self.indention = false;
            }
            Style::Literal => {
                self.write_indicator("|", true, false, false);
                let chars: Vec<char> = value.chars().collect();
                if chars
                    .first()
                    .is_some_and(|c| *c == ' ' || is_break_char(*c))
                {
                    self.write_indicator(&BEST_INDENT.to_string(), false, false, false);
                }
                let n = chars.len();
                let chomp = if n == 0 || !is_break_char(chars[n - 1]) {
                    Some("-")
                } else if n == 1 || is_break_char(chars[n - 2]) {
                    Some("+")
                } else {
                    None
                };
                if let Some(c) = chomp {
                    self.write_indicator(c, false, false, false);
                }
                self.whitespace = true;
                let mut breaks = true;
                for c in chars {
                    if is_break_char(c) {
                        if c == '\n' {
                            self.put_break();
                        } else {
                            self.out.push(c);
                            self.column = 0;
                            self.indention = true;
                        }
                        breaks = true;
                    } else {
                        if breaks {
                            self.write_indent();
                        }
                        self.put(c);
                        self.indention = false;
                        breaks = false;
                    }
                }
            }
        }
        self.pop_indent();
        self.states.pop();
    }
}

/// `yaml.Marshal(map[string]interface{})`.
pub fn marshal(v: &GoYaml) -> String {
    let mut e = Emitter {
        out: String::new(),
        column: 0,
        whitespace: true,
        indention: true,
        indent: -1,
        indents: vec![],
        states: vec![],
    };
    e.states.push(State::Other);
    e.emit(v, false);
    // document end
    e.write_indent();
    if !e.out.ends_with('\n') {
        e.out.push('\n');
    }
    e.out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn roundtrip(s: &str) -> String {
        marshal(&unmarshal(s).unwrap())
    }

    // Expected outputs produced by Go yaml.v3 (Unmarshal into map[string]interface{} then Marshal).
    #[test]
    fn matches_yaml_v3() {
        let input = r##"a:
  - x
  - y: 1
    z: [1, 2]
    w: {}
  - - nested
    - seq
  - []
b: "true"
c: ""
d: "yes"
e: 1.0
f: 1.5
g: 1e6
h: "multi\nline\n"
i: "multi\nline"
j: "trailing\n\n"
k: " leading space"
l: "*star"
m: "a: b"
n: "#hash"
o: null
p: ~
r: "123"
s: '0x1F'
t: "- dash"
u: "it's"
v: "tab\there"
w2: "é"
x10: 1
x9: 2
X: 3
_u: 4
key with space: v
"needs: quote": v
y: "  \nfoo"
z: !!str 12
aa: 100000000000000000000.0
ab: 0.0000001
ac: -1
ad: "@at"
ae: "%pct"
af: "a #b"
ag: "a:"
ah: "line1\nline2 "
ai: "\u0001ctl"
"##;
        let expected = r##"_u: 4
X: 3
a:
    - x
    - w: {}
      "y": 1
      z:
        - 1
        - 2
    - - nested
      - seq
    - []
aa: 1e+20
ab: 1e-07
ac: -1
ad: '@at'
ae: '%pct'
af: 'a #b'
ag: 'a:'
ah: "line1\nline2 "
ai: "\x01ctl"
b: "true"
c: ""
d: "yes"
e: 1
f: 1.5
g: 1e+06
h: |
    multi
    line
i: |-
    multi
    line
j: |+
    trailing

k: ' leading space'
key with space: v
l: '*star'
m: 'a: b'
"n": '#hash'
'needs: quote': v
o: null
p: null
r: "123"
s: "0x1F"
t: '- dash'
u: it's
v: "tab\there"
w2: é
x9: 2
x10: 1
"y": "  \nfoo"
z: "12"
"##;
        assert_eq!(roundtrip(input), expected);
    }

    #[test]
    fn matches_yaml_v3_nested() {
        assert_eq!(
            roundtrip("b: 1\na: \"x\\n\\n\"\n"),
            "a: |+\n    x\n\nb: 1\n"
        );
        assert_eq!(
            roundtrip("a:\n  b: \"x\\n\\n\"\nc: 1\n"),
            "a:\n    b: |+\n        x\n\nc: 1\n"
        );
        assert_eq!(
            roundtrip("a: [{}]\nb: [[]]\nc:\n  - \"multi\\nline\"\n  - k: \"m\\nl\"\n"),
            "a:\n    - {}\nb:\n    - []\nc:\n    - |-\n      multi\n      line\n    - k: |-\n        m\n        l\n"
        );
        assert_eq!(
            roundtrip("base: &b\n  x: 1\nd:\n  <<: *b\n  y: 2\n"),
            "base:\n    x: 1\nd:\n    x: 1\n    \"y\": 2\n"
        );
        assert_eq!(
            roundtrip(
                "k: \"  x\"\nl: \"x  \"\nm: \"\\ttab\"\nn: \"a\\u00a0b\"\no: \"---\"\np: \"a\\rb\"\n"
            ),
            "k: '  x'\nl: 'x  '\nm: \"\\ttab\"\n\"n\": a\u{a0}b\no: '---'\np: \"a\\rb\"\n"
        );
        assert_eq!(roundtrip(""), "{}\n");
    }

    #[test]
    fn floats_like_go() {
        assert_eq!(format_float(1.0), "1");
        assert_eq!(format_float(1.5), "1.5");
        assert_eq!(format_float(1e6), "1e+06");
        assert_eq!(format_float(100000.0), "100000");
        assert_eq!(format_float(123456789.0), "1.23456789e+08");
        assert_eq!(format_float(0.0001), "0.0001");
        assert_eq!(format_float(0.00001), "1e-05");
        assert_eq!(format_float(-2.5), "-2.5");
    }

    // Expected outputs produced by Go yaml.v3 (Unmarshal into map[string]interface{} then Marshal).
    #[test]
    fn non_string_keys_like_yaml_v3() {
        assert_eq!(
            roundtrip(
                "1e6: a\n0x1F: b\n007: c\n1.50: d\n.inf: e\ntrue: f\n12: i\n-0: j\n+5: k\n0o17: l\n"
            ),
            "\"+5\": k\n\"-0\": j\n\".inf\": e\n\"0o17\": l\n\"0x1F\": b\n\"1e6\": a\n\"1.50\": d\n\"007\": c\n\"12\": i\n\"true\": f\n"
        );
        assert_eq!(roundtrip("null: g\nx: 1\n"), "x: 1\n");
        assert_eq!(
            roundtrip("top:\n  1e6: a\n  0x1F: b\n  x: c\n"),
            "top:\n    31: b\n    1e+06: a\n    x: c\n"
        );
        assert_eq!(
            roundtrip("top:\n  12: a\n  x: c\n"),
            "top:\n    12: a\n    x: c\n"
        );
        assert_eq!(roundtrip("top:\n  true: a\n"), "top:\n    true: a\n");
        assert_eq!(
            roundtrip("list:\n- 0x1F: a\n  y: b\n"),
            "list:\n    - 31: a\n      \"y\": b\n"
        );
    }

    #[test]
    fn key_order() {
        let mut keys = vec!["x10", "x9", "X", "_u", "a", "a-b", "ab", "A"];
        keys.sort_by(|a, b| {
            if key_less(a, b) {
                std::cmp::Ordering::Less
            } else if key_less(b, a) {
                std::cmp::Ordering::Greater
            } else {
                std::cmp::Ordering::Equal
            }
        });
        assert_eq!(keys, vec!["_u", "A", "X", "a", "a-b", "ab", "x9", "x10"]);
    }
}
