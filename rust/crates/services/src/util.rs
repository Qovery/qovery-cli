//! Small helpers: `utils/date.go`, `utils/random.go`, `utils/string.go`, `utils/map.go`, `utils.GetDuration`,
//! plus Go's `url.PathEscape` used by the generated client for path parameters.

use chrono::{DateTime, FixedOffset, NaiveDate, TimeDelta, TimeZone, Utc};
use rand::Rng;

pub use qovery_api::path_escape;

/// Go's zero `time.Time` (`0001-01-01T00:00:00Z`).
pub fn go_zero_time() -> DateTime<FixedOffset> {
    let naive = NaiveDate::from_ymd_opt(1, 1, 1)
        .and_then(|d| d.and_hms_opt(0, 0, 0))
        .unwrap_or_default();
    Utc.from_utc_datetime(&naive).fixed_offset()
}

/// Parse an API timestamp (RFC 3339), Go's zero time when missing or invalid
/// (what the Go getters such as `GetUpdatedAt()` return for a nil time).
pub fn parse_api_time(s: &str) -> DateTime<FixedOffset> {
    DateTime::parse_from_rfc3339(s).unwrap_or_else(|_| go_zero_time())
}

/// Port of `utils.ToIso8601(*time.Time) *string` for a time value:
/// `t.Format("2006-01-02T15:04:05.000Z")` (local time of the offset, milliseconds truncated, literal `Z`).
pub fn to_iso8601_time<Tz: TimeZone>(t: &DateTime<Tz>) -> String
where
    Tz::Offset: std::fmt::Display,
{
    qovery_log::gofmt::time_iso8601(t)
}

/// Port of `utils.ToIso8601(*time.Time) *string` for a timestamp as returned by the API (RFC 3339 string):
/// `None` stays `None` (Go nil -> JSON `null`); an unparsable string is returned unchanged.
pub fn to_iso8601(t: Option<&str>) -> Option<String> {
    let s = t?;
    match DateTime::parse_from_rfc3339(s) {
        Ok(d) => Some(to_iso8601_time(&d)),
        Err(_) => Some(s.to_string()),
    }
}

/// `utils.ToIso8601` applied to an optional JSON field (`null`/missing -> `null`), ready for `go_json` outputs.
pub fn to_iso8601_value(v: Option<&serde_json::Value>) -> serde_json::Value {
    match to_iso8601(v.and_then(|v| v.as_str())) {
        Some(s) => serde_json::Value::String(s),
        None => serde_json::Value::Null,
    }
}

const LETTER_BYTES: &[u8] = b"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";

/// Port of `utils.RandStringBytes(n)`: `n` random ASCII letters.
pub fn rand_string_bytes(n: usize) -> String {
    let mut rng = rand::thread_rng();
    (0..n)
        .map(|_| LETTER_BYTES[rng.gen_range(0..LETTER_BYTES.len())] as char)
        .collect()
}

/// Port of `utils.IsEmptyOrBlank(str)`: `len(strings.Trim(str, " ")) == 0` (only spaces are trimmed).
pub fn is_empty_or_blank(s: &str) -> bool {
    s.trim_matches(' ').is_empty()
}

/// Port of the generic `utils.Map(slice, transformer)`.
pub fn map<T, U>(slice: &[T], transformer: impl Fn(&T) -> U) -> Vec<U> {
    slice.iter().map(transformer).collect()
}

/// Port of `utils.GetDuration(startTime, endTime)`: `"12 seconds"`, `"1 minute and 5 seconds"`,
/// `"3 minutes and 0 seconds"`. Overflowing durations saturate like Go's `time.Time.Sub`.
pub fn get_duration<A: TimeZone, B: TimeZone>(
    start_time: &DateTime<A>,
    end_time: &DateTime<B>,
) -> String {
    let delta = end_time.clone().signed_duration_since(start_time.clone());
    let nanos = delta
        .num_nanoseconds()
        .unwrap_or(if delta < TimeDelta::zero() {
            i64::MIN
        } else {
            i64::MAX
        });
    go_duration_text(nanos)
}

/// [`get_duration`] for RFC 3339 timestamps; a missing/invalid timestamp is Go's zero time.
pub fn get_duration_rfc3339(start_time: &str, end_time: &str) -> String {
    get_duration(&parse_api_time(start_time), &parse_api_time(end_time))
}

/// `GetDuration` text of a Go `time.Duration` (nanoseconds).
pub fn go_duration_text(d: i64) -> String {
    const MINUTE: i64 = 60_000_000_000;
    const SECOND: i64 = 1_000_000_000;
    // time.Duration.Minutes() / Seconds()
    let minutes = (d / MINUTE) as f64 + (d % MINUTE) as f64 / 60e9;
    let seconds = (d / SECOND) as f64 + (d % SECOND) as f64 / 1e9;
    if minutes < 1.0 {
        return format!("{} seconds", seconds as i64);
    }
    if minutes < 2.0 {
        return format!(
            "{} minute and {} seconds",
            minutes as i64,
            (seconds as i64) % 60
        );
    }
    if minutes > 0.0 && seconds == 0.0 {
        return format!("{} minutes", minutes as i64);
    }
    format!(
        "{} minutes and {} seconds",
        minutes as i64,
        (seconds as i64) % 60
    )
}

/// Read a string field of a JSON object (`""` when missing or not a string), like Go's `GetXxx()` getters.
pub(crate) fn str_field<'a>(v: &'a serde_json::Value, key: &str) -> &'a str {
    v.get(key).and_then(|x| x.as_str()).unwrap_or("")
}

/// Port of the generated Go client `decode` of a 2xx response body into a `*Model` (`Execute()`): an empty
/// body (or `null`) is `None`, a non-JSON `Content-Type` is `undefined response type` (XML bodies are not
/// emulated), invalid JSON fails with the Go `json.Unmarshal` syntax error, a type mismatch with the serde one
/// (the Rust models stay lenient on missing required properties).
pub fn decode_go_response<T: serde::de::DeserializeOwned>(
    res: &qovery_api::Response,
) -> Result<Option<T>, String> {
    let content_type = res
        .headers
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    decode_go_body(&res.body, content_type)
}

fn decode_go_body<T: serde::de::DeserializeOwned>(
    body: &[u8],
    content_type: &str,
) -> Result<Option<T>, String> {
    if body.is_empty() {
        return Ok(None);
    }
    // Go `JsonCheck`
    static JSON_CHECK: std::sync::LazyLock<regex::Regex> = std::sync::LazyLock::new(|| {
        regex::Regex::new(r"(?i:(?:application|text)/(?:[^;]+\+)?json)").expect("valid regex")
    });
    if !JSON_CHECK.is_match(content_type) {
        return Err("undefined response type".to_string());
    }
    qovery_log::gojson::go_json_check_valid(body)?;
    serde_json::from_slice::<Option<T>>(body).map_err(|e| e.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn duration_texts() {
        assert_eq!(
            get_duration_rfc3339("2024-01-01T00:00:00Z", "2024-01-01T00:00:12.9Z"),
            "12 seconds"
        );
        assert_eq!(
            get_duration_rfc3339("2024-01-01T00:00:00Z", "2024-01-01T00:01:05Z"),
            "1 minute and 5 seconds"
        );
        assert_eq!(
            get_duration_rfc3339("2024-01-01T00:00:00Z", "2024-01-01T00:03:00Z"),
            "3 minutes and 0 seconds"
        );
        assert_eq!(
            get_duration_rfc3339("2024-01-01T00:00:00Z", "2024-01-01T01:02:03+01:00"),
            "2 minutes and 3 seconds"
        );
        assert_eq!(
            get_duration_rfc3339("2024-01-01T00:00:10Z", "2024-01-01T00:00:00Z"),
            "-10 seconds"
        );
        // zero time end: Go saturates to minDuration
        assert_eq!(
            get_duration_rfc3339("2024-01-01T00:00:00Z", ""),
            "-9223372036 seconds"
        );
    }

    #[test]
    fn iso8601() {
        assert_eq!(
            to_iso8601(Some("2024-01-02T15:04:05.123456Z")).unwrap(),
            "2024-01-02T15:04:05.123Z"
        );
        assert_eq!(
            to_iso8601(Some("2024-01-02T15:04:05+02:00")).unwrap(),
            "2024-01-02T15:04:05.000Z"
        );
        assert_eq!(to_iso8601(None), None);
        assert_eq!(to_iso8601_value(None), serde_json::Value::Null);
    }

    #[test]
    fn misc() {
        assert!(is_empty_or_blank("   "));
        assert!(!is_empty_or_blank(" \t "));
        assert_eq!(rand_string_bytes(12).len(), 12);
        assert!(
            rand_string_bytes(30)
                .bytes()
                .all(|b| b.is_ascii_alphabetic())
        );
        assert_eq!(path_escape("a b/c@d:e"), "a%20b%2Fc@d:e");
        assert_eq!(map(&[1, 2], |x| x * 2), vec![2, 4]);
    }

    #[test]
    fn decode_go_body_cases() {
        #[derive(Debug, serde::Deserialize, PartialEq)]
        struct M {
            #[serde(default)]
            id: String,
        }
        let json = "application/json";
        assert_eq!(decode_go_body::<M>(b"", "").unwrap(), None);
        assert_eq!(decode_go_body::<M>(b"null", json).unwrap(), None);
        assert_eq!(
            decode_go_body::<M>(br#"{"id":"a"}"#, "Application/problem+JSON; charset=utf-8")
                .unwrap(),
            Some(M { id: "a".into() })
        );
        assert_eq!(
            decode_go_body::<M>(b"{}", "text/plain").unwrap_err(),
            "undefined response type"
        );
        assert_eq!(
            decode_go_body::<M>(b"<html>", json).unwrap_err(),
            "invalid character '<' looking for beginning of value"
        );
        assert!(decode_go_body::<M>(br#""a""#, json).is_err());
    }
}
