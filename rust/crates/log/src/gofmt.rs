//! Go formatting helpers, so that values print exactly like the Go implementation.

use chrono::{DateTime, FixedOffset, Offset, TimeZone, Timelike};
use std::time::Duration;

/// Go `strconv.Quote` / `fmt.Sprintf("%q", s)`.
pub fn quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\x07' => out.push_str("\\a"),
            '\x08' => out.push_str("\\b"),
            '\x0c' => out.push_str("\\f"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\x0b' => out.push_str("\\v"),
            c if (c as u32) < 0x20 || c as u32 == 0x7f => {
                out.push_str(&format!("\\x{:02x}", c as u32))
            }
            c if is_go_printable(c) => out.push(c),
            c if (c as u32) < 0x10000 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push_str(&format!("\\U{:08x}", c as u32)),
        }
    }
    out.push('"');
    out
}

fn is_go_printable(c: char) -> bool {
    // Approximation of unicode.IsPrint: letters, marks, numbers, punctuation, symbols and ASCII space.
    !(c.is_control() || (c.is_whitespace() && c != ' ') || ('\u{80}'..='\u{9f}').contains(&c))
}

/// Go `time.Time.String()`: `2006-01-02 15:04:05.999999999 -0700 MST`.
///
/// Times decoded from JSON keep their offset; a `Z` offset prints as `+0000 UTC`, any other offset
/// prints its numeric zone twice (Go has no zone name for a fixed offset parsed from RFC 3339).
pub fn time_string<Tz: TimeZone>(t: &DateTime<Tz>) -> String {
    let fixed: DateTime<FixedOffset> = t.fixed_offset();
    let offset_secs = fixed.offset().fix().local_minus_utc();
    let base = fixed.format("%Y-%m-%d %H:%M:%S").to_string();
    let nanos = fixed.nanosecond() % 1_000_000_000;
    let frac = if nanos == 0 {
        String::new()
    } else {
        let s = format!(".{nanos:09}");
        s.trim_end_matches('0').to_string()
    };
    let sign = if offset_secs < 0 { '-' } else { '+' };
    let abs = offset_secs.abs();
    let num = format!("{sign}{:02}{:02}", abs / 3600, (abs % 3600) / 60);
    let name = if offset_secs == 0 {
        "UTC".to_string()
    } else {
        num.clone()
    };
    format!("{base}{frac} {num} {name}")
}

/// Go `time.Time.String()` for a timestamp given as RFC 3339 string (as returned by the API).
/// Returns the input unchanged if it cannot be parsed.
pub fn time_string_from_rfc3339(s: &str) -> String {
    match DateTime::parse_from_rfc3339(s) {
        Ok(t) => {
            if s.ends_with('Z') || s.ends_with('z') {
                time_string(&t.with_timezone(&chrono::Utc))
            } else {
                time_string(&t)
            }
        }
        Err(_) => s.to_string(),
    }
}

fn offset_secs<Tz: TimeZone>(t: &DateTime<Tz>) -> i32 {
    t.offset().fix().local_minus_utc()
}

/// Go numeric zone: `-0700` (`colon` false) or `-07:00` (`colon` true).
fn numeric_zone(secs: i32, colon: bool) -> String {
    let sign = if secs < 0 { '-' } else { '+' };
    let abs = secs.abs();
    if colon {
        format!("{sign}{:02}:{:02}", abs / 3600, (abs % 3600) / 60)
    } else {
        format!("{sign}{:02}{:02}", abs / 3600, (abs % 3600) / 60)
    }
}

/// Go fractional seconds of `.999999999` layouts: trailing zeros (and the dot) removed.
fn trimmed_nanos<Tz: TimeZone>(t: &DateTime<Tz>) -> String {
    let nanos = t.nanosecond() % 1_000_000_000;
    if nanos == 0 {
        String::new()
    } else {
        format!(".{nanos:09}").trim_end_matches('0').to_string()
    }
}

/// Go `t.Format(time.RFC3339)`: `2006-01-02T15:04:05Z07:00` (`Z` for a zero offset).
pub fn time_rfc3339<Tz: TimeZone>(t: &DateTime<Tz>) -> String
where
    Tz::Offset: std::fmt::Display,
{
    let secs = offset_secs(t);
    let zone = if secs == 0 {
        "Z".to_string()
    } else {
        numeric_zone(secs, true)
    };
    format!("{}{zone}", t.format("%Y-%m-%dT%H:%M:%S"))
}

/// Go `t.Format(time.RFC3339Nano)`, which is also the JSON encoding of a `time.Time`.
pub fn time_rfc3339_nano<Tz: TimeZone>(t: &DateTime<Tz>) -> String
where
    Tz::Offset: std::fmt::Display,
{
    let secs = offset_secs(t);
    let zone = if secs == 0 {
        "Z".to_string()
    } else {
        numeric_zone(secs, true)
    };
    format!(
        "{}{}{zone}",
        t.format("%Y-%m-%dT%H:%M:%S"),
        trimmed_nanos(t)
    )
}

/// Go `t.Format(time.RFC1123)`: `Mon, 02 Jan 2006 15:04:05 MST`. A zero offset prints `UTC` (the API
/// timestamps are in UTC), any other fixed offset prints `-0700` (Go knows no zone name for it).
pub fn time_rfc1123<Tz: TimeZone>(t: &DateTime<Tz>) -> String
where
    Tz::Offset: std::fmt::Display,
{
    let secs = offset_secs(t);
    let zone = if secs == 0 {
        "UTC".to_string()
    } else {
        numeric_zone(secs, false)
    };
    format!("{} {zone}", t.format("%a, %d %b %Y %H:%M:%S"))
}

/// Go `t.Format(time.RFC822)`: `02 Jan 06 15:04 MST` (zone like [`time_rfc1123`]).
pub fn time_rfc822<Tz: TimeZone>(t: &DateTime<Tz>) -> String
where
    Tz::Offset: std::fmt::Display,
{
    let secs = offset_secs(t);
    let zone = if secs == 0 {
        "UTC".to_string()
    } else {
        numeric_zone(secs, false)
    };
    format!("{} {zone}", t.format("%d %b %y %H:%M"))
}

/// `utils.ToIso8601`: `t.Format("2006-01-02T15:04:05.000Z")` (wall clock of the time's own offset,
/// milliseconds truncated, literal `Z`).
pub fn time_iso8601<Tz: TimeZone>(t: &DateTime<Tz>) -> String
where
    Tz::Offset: std::fmt::Display,
{
    let millis = (t.nanosecond() % 1_000_000_000) / 1_000_000;
    format!("{}.{millis:03}Z", t.format("%Y-%m-%dT%H:%M:%S"))
}

/// Go `time.Time{}.String()` (zero value), printed when the API omits a timestamp.
pub const ZERO_TIME_STRING: &str = "0001-01-01 00:00:00 +0000 UTC";

/// Go `time.Duration.String()`.
pub fn duration_string(d: Duration) -> String {
    let nanos = d.as_nanos();
    if nanos == 0 {
        return "0s".to_string();
    }
    if nanos < 1_000 {
        return format!("{nanos}ns");
    }
    if nanos < 1_000_000 {
        return format!("{}µs", trim_float(nanos as f64 / 1_000.0));
    }
    if nanos < 1_000_000_000 {
        return format!("{}ms", trim_float(nanos as f64 / 1_000_000.0));
    }
    let total_secs = d.as_secs();
    let h = total_secs / 3600;
    let m = (total_secs % 3600) / 60;
    let s = total_secs % 60;
    let sub = d.subsec_nanos();
    let secs = if sub == 0 {
        format!("{s}")
    } else {
        let frac = format!("{sub:09}");
        format!("{s}.{}", frac.trim_end_matches('0'))
    };
    if h > 0 {
        format!("{h}h{m}m{secs}s")
    } else if m > 0 {
        format!("{m}m{secs}s")
    } else {
        format!("{secs}s")
    }
}

fn trim_float(v: f64) -> String {
    let s = format!("{v:.9}");
    let s = s.trim_end_matches('0');
    s.trim_end_matches('.').to_string()
}

/// Go `time.ParseDuration`, as signed nanoseconds (negative durations allowed). Exact port: integer
/// arithmetic, overflow (beyond ±292 years) is `time: invalid duration "<s>"`, values quoted like the
/// `time` package (non-ASCII bytes as `\xNN`).
pub fn parse_duration_nanos(s: &str) -> Result<i64, String> {
    use crate::gotime::quote as time_quote;
    const MAX: u64 = 1 << 63;
    let orig = s;
    let invalid = || format!("time: invalid duration {}", time_quote(orig));
    let mut s = s.as_bytes();
    let mut d: u64 = 0;
    let mut neg = false;
    if let Some(&c) = s.first()
        && (c == b'-' || c == b'+')
    {
        neg = c == b'-';
        s = &s[1..];
    }
    if s == b"0" {
        return Ok(0);
    }
    if s.is_empty() {
        return Err(invalid());
    }
    while !s.is_empty() {
        if !(s[0] == b'.' || s[0].is_ascii_digit()) {
            return Err(invalid());
        }
        // leadingInt
        let digits = s.iter().take_while(|c| c.is_ascii_digit()).count();
        let mut v: u64 = 0;
        for &c in &s[..digits] {
            if v > MAX / 10 {
                return Err(invalid());
            }
            v = v * 10 + u64::from(c - b'0');
            if v > MAX {
                return Err(invalid());
            }
        }
        let pre = digits > 0;
        s = &s[digits..];
        // leadingFraction
        let (mut f, mut scale, mut post) = (0u64, 1f64, false);
        if s.first() == Some(&b'.') {
            s = &s[1..];
            let digits = s.iter().take_while(|c| c.is_ascii_digit()).count();
            let mut overflow = false;
            for &c in &s[..digits] {
                if overflow {
                    continue;
                }
                if f > (MAX - 1) / 10 {
                    overflow = true;
                    continue;
                }
                let y = f * 10 + u64::from(c - b'0');
                if y > MAX {
                    overflow = true;
                    continue;
                }
                f = y;
                scale *= 10.0;
            }
            post = digits > 0;
            s = &s[digits..];
        }
        if !pre && !post {
            return Err(invalid());
        }
        let i = s
            .iter()
            .position(|&c| c == b'.' || c.is_ascii_digit())
            .unwrap_or(s.len());
        if i == 0 {
            return Err(format!(
                "time: missing unit in duration {}",
                time_quote(orig)
            ));
        }
        let u = &s[..i];
        s = &s[i..];
        let unit: u64 = match u {
            b"ns" => 1,
            b"us" => 1_000,
            _ if u == "µs".as_bytes() || u == "μs".as_bytes() => 1_000,
            b"ms" => 1_000_000,
            b"s" => 1_000_000_000,
            b"m" => 60_000_000_000,
            b"h" => 3_600_000_000_000,
            _ => {
                return Err(format!(
                    "time: unknown unit {} in duration {}",
                    time_quote(std::str::from_utf8(u).unwrap_or_default()),
                    time_quote(orig)
                ));
            }
        };
        if v > MAX / unit {
            return Err(invalid());
        }
        v *= unit;
        if f > 0 {
            // Go: `v += uint64(float64(f) * (float64(unit) / scale))` (saturating float-to-int conversion).
            v += (f as f64 * (unit as f64 / scale)) as u64;
            if v > MAX {
                return Err(invalid());
            }
        }
        d += v;
        if d > MAX {
            return Err(invalid());
        }
    }
    if neg {
        // `-Duration(d)` with d <= 1<<63.
        return Ok((d as i64).wrapping_neg());
    }
    if d > MAX - 1 {
        return Err(invalid());
    }
    Ok(d as i64)
}

/// Go `time.Duration.String()` of signed nanoseconds.
pub fn duration_string_nanos(n: i64) -> String {
    if n < 0 {
        format!(
            "-{}",
            duration_string(Duration::from_nanos(n.unsigned_abs()))
        )
    } else {
        duration_string(Duration::from_nanos(n as u64))
    }
}

/// Parse a non-negative Go duration string (`time.ParseDuration`), e.g. `15m`, `1h30m`, `500ms`, `2.5s`.
/// Negative durations are an error here (`time: invalid duration "<s>"`): see [`parse_duration_nanos`].
pub fn parse_duration(s: &str) -> Result<Duration, String> {
    match parse_duration_nanos(s)? {
        n if n < 0 => Err(format!(
            "time: invalid duration {}",
            crate::gotime::quote(s)
        )),
        n => Ok(Duration::from_nanos(n as u64)),
    }
}

/// Go `json.Marshal` of a dynamic value (`map[string]interface{}` & co): object keys sorted, HTML characters
/// escaped (`<` `>` `&` as `<` ...), U+2028/U+2029 escaped. Independent of serde_json's `preserve_order`.
pub fn go_json(v: &serde_json::Value) -> String {
    let mut out = String::new();
    write_go_json(v, &mut out, None, 0);
    out
}

/// Go `json.MarshalIndent(v, "", indent)`.
pub fn go_json_indent(v: &serde_json::Value, indent: &str) -> String {
    let mut out = String::new();
    write_go_json(v, &mut out, Some(indent), 0);
    out
}

fn go_json_string(s: &str, out: &mut String) {
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '<' => out.push_str("\\u003c"),
            '>' => out.push_str("\\u003e"),
            '&' => out.push_str("\\u0026"),
            '\u{2028}' => out.push_str("\\u2028"),
            '\u{2029}' => out.push_str("\\u2029"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
}

fn write_go_json(v: &serde_json::Value, out: &mut String, indent: Option<&str>, depth: usize) {
    use serde_json::Value;
    let nl = |out: &mut String, d: usize| {
        if let Some(i) = indent {
            out.push('\n');
            for _ in 0..d {
                out.push_str(i);
            }
        }
    };
    match v {
        Value::Null => out.push_str("null"),
        Value::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
        Value::Number(n) => out.push_str(&go_number(n)),
        Value::String(s) => go_json_string(s, out),
        Value::Array(a) => {
            if a.is_empty() {
                out.push_str("[]");
                return;
            }
            out.push('[');
            for (i, x) in a.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                nl(out, depth + 1);
                write_go_json(x, out, indent, depth + 1);
            }
            nl(out, depth);
            out.push(']');
        }
        Value::Object(m) => {
            if m.is_empty() {
                out.push_str("{}");
                return;
            }
            let mut keys: Vec<&String> = m.keys().collect();
            keys.sort();
            out.push('{');
            for (i, k) in keys.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                nl(out, depth + 1);
                go_json_string(k, out);
                out.push(':');
                if indent.is_some() {
                    out.push(' ');
                }
                write_go_json(&m[*k], out, indent, depth + 1);
            }
            nl(out, depth);
            out.push('}');
        }
    }
}

/// Go float64 JSON encoding (`strconv.AppendFloat(f, 'f' or 'e', -1, 64)`); integers unchanged.
fn go_number(n: &serde_json::Number) -> String {
    if n.is_i64() || n.is_u64() {
        return n.to_string();
    }
    go_float(n.as_f64().unwrap_or(0.0))
}

/// Go `encoding/json` float64 encoding: shortest repr, exponent form outside [1e-6, 1e21)
/// (`1e-7`, `1e+21`).
pub fn go_float(f: f64) -> String {
    let abs = f.abs();
    if abs != 0.0 && !(1e-6..1e21).contains(&abs) {
        let s = format!("{f:e}");
        let (mantissa, exp) = s.split_once('e').unwrap_or((&s, "0"));
        return match exp.strip_prefix('-') {
            Some(digits) => format!("{mantissa}e-{digits}"),
            None => format!("{mantissa}e+{exp:0>2}"),
        };
    }
    format!("{f}")
}

/// Go `json.Marshal(v)` of a struct: serde serialization (struct field order) + Go HTML escaping.
/// Use [`go_json`] for maps / `serde_json::Value` (sorted keys).
pub fn go_marshal<T: serde::Serialize + ?Sized>(v: &T) -> String {
    escape_html(&serde_json::to_string(v).unwrap_or_default())
}

/// Go `json.MarshalIndent(v, "", indent)` of a struct (see [`go_marshal`]).
pub fn go_marshal_indent<T: serde::Serialize + ?Sized>(v: &T, indent: &str) -> String {
    let mut buf = Vec::new();
    let fmt = serde_json::ser::PrettyFormatter::with_indent(indent.as_bytes());
    let mut ser = serde_json::Serializer::with_formatter(&mut buf, fmt);
    let _ = v.serialize(&mut ser);
    escape_html(&String::from_utf8_lossy(&buf))
}

fn escape_html(s: &str) -> String {
    s.replace('<', "\\u003c")
        .replace('>', "\\u003e")
        .replace('&', "\\u0026")
        .replace('\u{2028}', "\\u2028")
        .replace('\u{2029}', "\\u2029")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_time_string() {
        assert_eq!(
            time_string_from_rfc3339("2024-01-02T15:04:05.123456Z"),
            "2024-01-02 15:04:05.123456 +0000 UTC"
        );
        assert_eq!(
            time_string_from_rfc3339("2024-01-02T15:04:05Z"),
            "2024-01-02 15:04:05 +0000 UTC"
        );
        assert_eq!(
            time_string_from_rfc3339("2024-01-02T15:04:05+02:00"),
            "2024-01-02 15:04:05 +0200 +0200"
        );
    }

    #[test]
    fn go_duration_string() {
        assert_eq!(duration_string(Duration::from_secs(900)), "15m0s");
        assert_eq!(duration_string(Duration::from_secs(3661)), "1h1m1s");
        assert_eq!(duration_string(Duration::from_millis(1500)), "1.5s");
        assert_eq!(parse_duration("15m").unwrap(), Duration::from_secs(900));
        assert_eq!(
            parse_duration("abc").unwrap_err(),
            "time: invalid duration \"abc\""
        );
    }

    #[test]
    fn go_parse_duration() {
        // Values checked against Go 1.27 `time.ParseDuration`.
        let ok = |s: &str| parse_duration_nanos(s).unwrap();
        assert_eq!(ok("1h30m"), 5_400_000_000_000);
        assert_eq!(ok("-1.5h"), -5_400_000_000_000);
        assert_eq!(ok("+0"), 0);
        assert_eq!(ok("-0"), 0);
        assert_eq!(ok(".5s"), 500_000_000);
        assert_eq!(ok("1.s"), 1_000_000_000);
        assert_eq!(ok("1.0000000001h"), 3_600_000_000_360);
        assert_eq!(ok("1µs1μs1us"), 3_000);
        assert_eq!(ok("2562047h47m16.854775807s"), i64::MAX);
        assert_eq!(ok("-2562047h47m16.854775808s"), i64::MIN);
        let err = |s: &str| parse_duration_nanos(s).unwrap_err();
        // Overflow is an error, never a wrap or a panic.
        assert_eq!(
            err("2562047h47m16.854775808s"),
            "time: invalid duration \"2562047h47m16.854775808s\""
        );
        assert_eq!(err("3000000h"), "time: invalid duration \"3000000h\"");
        assert_eq!(err("9999999999h"), "time: invalid duration \"9999999999h\"");
        assert_eq!(
            err("99999999999999999999999h"),
            "time: invalid duration \"99999999999999999999999h\""
        );
        assert!(parse_duration("3000000h").is_err());
        assert_eq!(err("."), "time: invalid duration \".\"");
        assert_eq!(err("--5s"), "time: invalid duration \"--5s\"");
        assert_eq!(err("5"), "time: missing unit in duration \"5\"");
        assert_eq!(
            err("5é"),
            "time: unknown unit \"\\xc3\\xa9\" in duration \"5\\xc3\\xa9\""
        );
        assert_eq!(
            parse_duration("-5m").unwrap_err(),
            "time: invalid duration \"-5m\""
        );
        assert_eq!(parse_duration("-0").unwrap(), Duration::ZERO);
    }

    #[test]
    fn go_floats() {
        assert_eq!(go_float(1e20), "100000000000000000000");
        assert_eq!(go_float(1e21), "1e+21");
        assert_eq!(go_float(1e-7), "1e-7");
        assert_eq!(go_float(1.5), "1.5");
        assert_eq!(go_float(3.0), "3");
    }

    #[test]
    fn go_quote() {
        assert_eq!(quote("a\"b\n"), "\"a\\\"b\\n\"");
        assert_eq!(quote("été"), "\"été\"");
    }
}
