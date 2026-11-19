//! Port of Go's `time.Parse(time.RFC3339, value)` (generic `parse` path, which gives the error messages)
//! returning the Unix timestamp of the parsed time. It is also what `time.Time.UnmarshalJSON` accepts
//! (Go 1.27: `parseStrictRFC3339` falls back to `Parse(RFC3339, ...)`, its extra strict checks are disabled).

const LAYOUT: &str = "2006-01-02T15:04:05Z07:00";

/// `time` package's private `quote`: `"` and `\` escaped, bytes of non-ASCII or control characters as `\xNN`.
pub(crate) fn quote(s: &str) -> String {
    let b = s.as_bytes();
    let mut out = String::from("\"");
    let mut i = 0;
    while i < b.len() {
        let c = b[i];
        if !(b' '..0x80).contains(&c) {
            let ch = s[i..].chars().next().unwrap_or('\u{FFFD}');
            let width = if ch == '\u{FFFD}' {
                if i + 2 < b.len() && &s[i..i + 3] == "\u{FFFD}" {
                    3
                } else {
                    1
                }
            } else {
                ch.len_utf8()
            };
            for j in 0..width {
                out.push_str(&format!("\\x{:02x}", b[i + j]));
            }
            i += width;
        } else {
            if c == b'"' || c == b'\\' {
                out.push('\\');
            }
            out.push(c as char);
            i += 1;
        }
    }
    out.push('"');
    out
}

fn parse_error(value: &str, layout_elem: &str, value_elem: &str) -> String {
    format!(
        "parsing time {} as {}: cannot parse {} as {}",
        quote(value),
        quote(LAYOUT),
        quote(value_elem),
        quote(layout_elem)
    )
}

fn message_error(value: &str, message: &str) -> String {
    format!("parsing time {}{}", quote(value), message)
}

fn is_digit(s: &[u8], i: usize) -> bool {
    i < s.len() && s[i].is_ascii_digit()
}

/// `getnum(s, fixed)`.
fn getnum(s: &[u8], fixed: bool) -> Result<(i64, &[u8]), ()> {
    if !is_digit(s, 0) {
        return Err(());
    }
    if !is_digit(s, 1) {
        if fixed {
            return Err(());
        }
        return Ok(((s[0] - b'0') as i64, &s[1..]));
    }
    Ok((((s[0] - b'0') as i64) * 10 + (s[1] - b'0') as i64, &s[2..]))
}

fn is_leap(year: i64) -> bool {
    year % 4 == 0 && (year % 100 != 0 || year % 400 == 0)
}

fn days_in(month: i64, year: i64) -> i64 {
    match month {
        2 if is_leap(year) => 29,
        2 => 28,
        4 | 6 | 9 | 11 => 30,
        _ => 31,
    }
}

/// Days since 1970-01-01 of a proleptic Gregorian date.
fn days_from_civil(y: i64, m: i64, d: i64) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = if y >= 0 { y } else { y - 399 } / 400;
    let yoe = y - era * 400;
    let mp = (m + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

enum Std {
    LongYear,
    ZeroMonth,
    ZeroDay,
    Hour,
    ZeroMinute,
    ZeroSecond,
    Iso8601ColonTz,
}

/// `time.Parse(time.RFC3339, value).Unix()`, with Go's error messages.
pub fn parse_rfc3339_unix(avalue: &str) -> Result<i64, String> {
    parse_rfc3339(avalue).map(|(secs, _)| secs)
}

/// `time.Parse(time.RFC3339, value)` as `(Unix(), Nanosecond())`, with Go's error messages.
pub fn parse_rfc3339(avalue: &str) -> Result<(i64, u32), String> {
    // (prefix, std, std string) of the RFC 3339 layout, as returned by nextStdChunk
    let chunks: [(&str, Std, &str); 7] = [
        ("", Std::LongYear, "2006"),
        ("-", Std::ZeroMonth, "01"),
        ("-", Std::ZeroDay, "02"),
        ("T", Std::Hour, "15"),
        (":", Std::ZeroMinute, "04"),
        (":", Std::ZeroSecond, "05"),
        ("", Std::Iso8601ColonTz, "Z07:00"),
    ];
    let (mut year, mut month, mut day, mut hour, mut min, mut sec) =
        (0i64, -1i64, -1i64, 0i64, 0i64, 0i64);
    let mut zone_offset: i64 = 0;
    let mut nsec: u32 = 0;
    let mut value = avalue.as_bytes();
    let as_str = |v: &[u8]| String::from_utf8_lossy(v).to_string();

    for (prefix, std, stdstr) in chunks.iter() {
        // skip(value, prefix)
        for p in prefix.bytes() {
            if value.is_empty() || value[0] != p {
                return Err(parse_error(avalue, prefix, &as_str(value)));
            }
            value = &value[1..];
        }
        let hold = value;
        let mut range_err = "";
        let mut ok = true;
        match std {
            Std::LongYear => {
                if value.len() < 4
                    || !is_digit(value, 0)
                    || !value[1..4].iter().all(|c| c.is_ascii_digit())
                {
                    ok = false;
                } else {
                    year = std::str::from_utf8(&value[..4])
                        .unwrap_or("0")
                        .parse()
                        .unwrap_or(0);
                    value = &value[4..];
                }
            }
            Std::ZeroMonth => match getnum(value, true) {
                Ok((m, rest)) => {
                    month = m;
                    value = rest;
                    if !(1..=12).contains(&m) {
                        range_err = "month";
                    }
                }
                Err(_) => ok = false,
            },
            Std::ZeroDay => match getnum(value, true) {
                Ok((d, rest)) => {
                    day = d;
                    value = rest;
                }
                Err(_) => ok = false,
            },
            Std::Hour => match getnum(value, false) {
                Ok((h, rest)) => {
                    hour = h;
                    value = rest;
                    if h >= 24 {
                        range_err = "hour";
                    }
                }
                Err(_) => ok = false,
            },
            Std::ZeroMinute => match getnum(value, true) {
                Ok((m, rest)) => {
                    min = m;
                    value = rest;
                    if m >= 60 {
                        range_err = "minute";
                    }
                }
                Err(_) => ok = false,
            },
            Std::ZeroSecond => match getnum(value, true) {
                Ok((s, rest)) => {
                    sec = s;
                    value = rest;
                    if s >= 60 {
                        range_err = "second";
                    } else if value.len() >= 2
                        && (value[0] == b'.' || value[0] == b',')
                        && is_digit(value, 1)
                    {
                        // fractional second in the input but not in the layout
                        let mut n = 2;
                        while n < value.len() && is_digit(value, n) {
                            n += 1;
                        }
                        // parseNanoseconds: at most 9 digits, the rest is truncated
                        let digits = &value[1..n.min(10)];
                        nsec = digits
                            .iter()
                            .fold(0u32, |acc, d| acc * 10 + u32::from(d - b'0'))
                            * 10u32.pow(9 - digits.len() as u32);
                        value = &value[n..];
                    }
                }
                Err(_) => ok = false,
            },
            Std::Iso8601ColonTz => {
                if !value.is_empty() && value[0] == b'Z' {
                    value = &value[1..];
                    zone_offset = 0;
                } else if value.len() < 6 || value[3] != b':' {
                    ok = false;
                } else {
                    let (sign, h, m) = (value[0], &value[1..3], &value[4..6]);
                    value = &value[6..];
                    let mut hr = 0;
                    let mut mm = 0;
                    match getnum(h, true) {
                        Ok((x, _)) => {
                            hr = x;
                            match getnum(m, true) {
                                Ok((y, _)) => mm = y,
                                Err(_) => ok = false,
                            }
                        }
                        Err(_) => ok = false,
                    }
                    if hr > 24 {
                        range_err = "time zone offset hour";
                    }
                    if mm > 60 {
                        range_err = "time zone offset minute";
                    }
                    zone_offset = (hr * 60 + mm) * 60;
                    match sign {
                        b'+' => {}
                        b'-' => zone_offset = -zone_offset,
                        _ => ok = false,
                    }
                }
            }
        }
        if !range_err.is_empty() {
            return Err(message_error(
                avalue,
                &format!(": {range_err} out of range"),
            ));
        }
        if !ok {
            return Err(parse_error(avalue, stdstr, &as_str(hold)));
        }
    }
    if !value.is_empty() {
        return Err(message_error(
            avalue,
            &format!(": extra text: {}", quote(&as_str(value))),
        ));
    }
    if month < 0 {
        month = 1;
    }
    if day < 0 {
        day = 1;
    }
    if day < 1 || day > days_in(month, year) {
        return Err(message_error(avalue, ": day out of range"));
    }
    let days = days_from_civil(year, month, day);
    Ok((
        days * 86400 + hour * 3600 + min * 60 + sec - zone_offset,
        nsec,
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid() {
        assert_eq!(parse_rfc3339_unix("2025-10-02T01:04:45Z"), Ok(1759367085));
        assert_eq!(
            parse_rfc3339_unix("2025-10-02T01:04:45+12:00"),
            Ok(1759367085 - 12 * 3600)
        );
        assert_eq!(
            parse_rfc3339_unix("2025-10-02T01:04:45.123-02:30"),
            Ok(1759367085 + 9000)
        );
        assert_eq!(parse_rfc3339_unix("1970-01-01T00:00:00Z"), Ok(0));
        assert_eq!(
            parse_rfc3339("1970-01-01T00:00:01.5Z"),
            Ok((1, 500_000_000))
        );
        assert_eq!(
            parse_rfc3339("1970-01-01T00:00:01,1234567899Z"),
            Ok((1, 123_456_789))
        );
        assert_eq!(parse_rfc3339("1970-01-01T1:00:00Z"), Ok((3600, 0)));
    }

    #[test]
    fn errors() {
        assert_eq!(
            parse_rfc3339_unix("2025-10-02 01:04:45Z").unwrap_err(),
            r#"parsing time "2025-10-02 01:04:45Z" as "2006-01-02T15:04:05Z07:00": cannot parse " 01:04:45Z" as "T""#
        );
        assert_eq!(
            parse_rfc3339_unix("yesterday").unwrap_err(),
            r#"parsing time "yesterday" as "2006-01-02T15:04:05Z07:00": cannot parse "yesterday" as "2006""#
        );
        assert_eq!(
            parse_rfc3339_unix("2025-13-02T01:04:45Z").unwrap_err(),
            r#"parsing time "2025-13-02T01:04:45Z": month out of range"#
        );
        assert_eq!(
            parse_rfc3339_unix("2025-02-30T01:04:45Z").unwrap_err(),
            r#"parsing time "2025-02-30T01:04:45Z": day out of range"#
        );
        assert_eq!(
            parse_rfc3339_unix("2025-02-03T01:04:45Zabc").unwrap_err(),
            r#"parsing time "2025-02-03T01:04:45Zabc": extra text: "abc""#
        );
        assert_eq!(
            parse_rfc3339_unix("2025-02-03T01:04:45").unwrap_err(),
            r#"parsing time "2025-02-03T01:04:45" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "Z07:00""#
        );
    }
}
