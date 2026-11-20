//! Port of `pkg/log.go`: streams the logs of a service from the websocket gateway.

use crate::ws;
use qovery_core::{auth, client};
use qovery_log::logrus;
use serde::de::{Deserializer, MapAccess, Visitor};
use serde_json::value::RawValue;
use std::fmt;
use std::io::Write;

/// `pkg.LogRequest`.
#[derive(Debug, Clone, Default)]
pub struct LogRequest {
    pub service_id: String,
    pub environment_id: String,
    pub project_id: String,
    pub organization_id: String,
    pub cluster_id: String,
    pub raw_format: bool,
}

/// `pkg.LogMessage` (`created_at` is a unix timestamp in milliseconds).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LogMessage {
    /// `None` until a `created_at` is decoded (Go zero `time.Time`)
    pub created_at_ms: Option<i64>,
    pub message: String,
    pub version: String,
    pub pod_name: String,
}

/// `pkg.ExecLog(req)`: prints every log line received until the connection is closed.
pub fn exec_log(req: &LogRequest) {
    let rt = ws::runtime();
    rt.block_on(async {
        let mut conn = match create_log_websocket(req).await {
            Ok(c) => c,
            // logrus.Fatal(args...) joins a string and an error without a space
            Err(e) => logrus::fatal(format!("error while creating websocket connection{e}")),
        };

        let mut log_message = LogMessage::default();
        loop {
            let msg = match ws::read_message(&mut conn).await {
                Ok((_, msg)) => msg,
                Err(ws::ReadError::Close(e)) => {
                    logrus::error(format!("connection closed by server: {e}"));
                    break;
                }
                Err(e) => {
                    logrus::error(format!("error while reading on websocket:{e}"));
                    break;
                }
            };

            let mut out = std::io::stdout().lock();
            if req.raw_format {
                let _ = out.write_all(&msg);
                let _ = out.write_all(b"\n");
            } else {
                drop(out);
                if let Err(e) = decode_log_message(&mut log_message, &msg) {
                    logrus::fatal(format!("%{e}"));
                }
                let mut out = std::io::stdout().lock();
                let _ = writeln!(
                    out,
                    "| {} | {} | {}",
                    format_millis(log_message.created_at_ms),
                    log_message.pod_name,
                    log_message.message
                );
            }
            let _ = std::io::stdout().flush();
        }
        if let Err(e) = ws::close(conn).await {
            logrus::fatal(format!("error while closing websocket connection{e}"));
        }
    });
}

/// `createLogWebsocket(req)`.
pub async fn create_log_websocket(req: &LogRequest) -> Result<ws::WsStream, String> {
    let url = format!(
        "{}/service/logs?service={}&cluster={}&environment={}&organization={}&project={}",
        client::websocket_url(),
        req.service_id,
        req.cluster_id,
        req.environment_id,
        req.organization_id,
        req.project_id,
    );
    let (token_type, token) = auth::get_access_token(false)?;
    ws::dial(
        &url,
        &[(
            "Authorization",
            auth::authorization_header_value(token_type, &token),
        )],
    )
    .await
}

/// Go `time.UnixMilli(ms).Format("2006-01-02 15:04:05.000")` in the local time zone (`None`: zero time).
///
/// Go covers every `int64`; instants outside chrono's range (about ±262000 years) are shifted by whole 400-year
/// Gregorian cycles (the calendar and the weekday-based DST rules repeat) and the year is corrected afterwards.
/// Years are printed like Go (`10000`, `-0001`), not like chrono (`+10000`).
pub fn format_millis(ms: Option<i64>) -> String {
    let Some(ms) = ms else {
        return "0001-01-01 00:00:00.000".to_string();
    };
    format_millis_in(ms, &chrono::Local)
}

fn format_millis_in<Tz: chrono::TimeZone>(ms: i64, tz: &Tz) -> String {
    use chrono::{Datelike, Timelike};
    const CYCLE_MS: i64 = 146_097 * 86_400_000;
    const LIMIT: i64 = 8_000_000_000_000_000;
    let cycles = if ms > LIMIT {
        (ms - LIMIT) / CYCLE_MS + 1
    } else if ms < -LIMIT {
        -((-LIMIT - ms) / CYCLE_MS + 1)
    } else {
        0
    };
    let shifted = ms - cycles * CYCLE_MS;
    let Some(t) = tz.timestamp_millis_opt(shifted).single() else {
        return String::new();
    };
    let year = i64::from(t.year()) + cycles * 400;
    let year = if year < 0 {
        format!("-{:04}", -year)
    } else {
        format!("{year:04}")
    };
    format!(
        "{year}-{:02}-{:02} {:02}:{:02}:{:02}.{:03}",
        t.month(),
        t.day(),
        t.hour(),
        t.minute(),
        t.second(),
        t.timestamp_subsec_millis()
    )
}

/// The entries of a JSON object in document order; values are kept raw so that numbers out of the `f64` range
/// (`1e400`) reach the Go-compatible decoding below instead of failing in serde.
struct Entries(Vec<(String, Box<RawValue>)>);

impl<'de> serde::Deserialize<'de> for Entries {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        struct V;
        impl<'de> Visitor<'de> for V {
            type Value = Entries;
            fn expecting(&self, f: &mut fmt::Formatter) -> fmt::Result {
                f.write_str("a JSON object")
            }
            fn visit_map<A: MapAccess<'de>>(self, mut map: A) -> Result<Entries, A::Error> {
                let mut entries = Vec::new();
                while let Some(entry) = map.next_entry::<String, Box<RawValue>>()? {
                    entries.push(entry);
                }
                Ok(Entries(entries))
            }
        }
        d.deserialize_map(V)
    }
}

/// `encoding/json` kind of a valid raw JSON value (`UnmarshalTypeError` wording).
fn kind_of(raw: &str) -> &'static str {
    match raw.trim_start().as_bytes().first() {
        Some(b'{') => "object",
        Some(b'[') => "array",
        Some(b'"') => "string",
        Some(b't' | b'f') => "bool",
        Some(b'n') => "null",
        _ => "number",
    }
}

pub use qovery_log::gojson::go_json_check_valid;

/// `json.Unmarshal(msg, &logMessage)`, reusing the previous values of the fields absent from `data`.
/// Returns Go's error message on failure (after printing `error decoding timestamp` like `Timestamp.UnmarshalJSON`).
pub fn decode_log_message(m: &mut LogMessage, data: &[u8]) -> Result<(), String> {
    go_json_check_valid(data)?;
    let doc: Box<RawValue> = serde_json::from_slice(data).map_err(|e| e.to_string())?;
    let entries = match kind_of(doc.get()) {
        "null" => return Ok(()),
        "object" => {
            serde_json::from_str::<Entries>(doc.get())
                .map_err(|e| e.to_string())?
                .0
        }
        kind => {
            return Err(format!(
                "json: cannot unmarshal {kind} into Go value of type pkg.LogMessage"
            ));
        }
    };
    const FIELDS: [&str; 4] = ["created_at", "message", "version", "pod_name"];
    let mut saved_error: Option<String> = None;
    for (key, raw) in entries {
        let field = FIELDS
            .iter()
            .find(|f| **f == key)
            .or_else(|| FIELDS.iter().find(|f| f.eq_ignore_ascii_case(&key)));
        let Some(field) = field else { continue };
        let text = raw.get().trim();
        let kind = kind_of(text);
        if *field == "created_at" {
            let ms: i64 = match kind {
                "null" => 0,
                "number" => {
                    let digits = text.strip_prefix('-').unwrap_or(text);
                    match text.parse::<i64>() {
                        Ok(i)
                            if !digits.is_empty() && digits.bytes().all(|b| b.is_ascii_digit()) =>
                        {
                            i
                        }
                        _ => {
                            timestamp_error(&mut saved_error, &format!("number {text}"));
                            continue;
                        }
                    }
                }
                other => {
                    timestamp_error(&mut saved_error, other);
                    continue;
                }
            };
            m.created_at_ms = Some(ms);
            continue;
        }
        let target = match *field {
            "message" => &mut m.message,
            "version" => &mut m.version,
            _ => &mut m.pod_name,
        };
        match kind {
            "string" => *target = serde_json::from_str(text).map_err(|e| e.to_string())?,
            "null" => {}
            other => {
                if saved_error.is_none() {
                    saved_error = Some(format!(
                        "json: cannot unmarshal {other} into Go struct field LogMessage.{field} of type string"
                    ));
                }
            }
        }
    }
    match saved_error {
        Some(e) => Err(e),
        None => Ok(()),
    }
}

/// `Timestamp.UnmarshalJSON` prints its error; json keeps the first error and goes on decoding.
fn timestamp_error(saved_error: &mut Option<String>, value: &str) {
    let e = format!("json: cannot unmarshal {value} into Go value of type int64");
    let mut out = std::io::stdout().lock();
    let _ = writeln!(out, "error decoding timestamp: {e}");
    let _ = out.flush();
    if saved_error.is_none() {
        *saved_error = Some(e);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decode_keeps_previous_values() {
        let mut m = LogMessage::default();
        decode_log_message(
            &mut m,
            br#"{"created_at": 1700000000123, "message": "hello", "pod_name": "p1", "x": 1}"#,
        )
        .unwrap();
        assert_eq!(m.created_at_ms, Some(1700000000123));
        decode_log_message(&mut m, br#"{"MESSAGE": "again"}"#).unwrap();
        assert_eq!((m.message.as_str(), m.pod_name.as_str()), ("again", "p1"));
        decode_log_message(&mut m, b"null").unwrap();
        assert_eq!(m.message, "again");
    }

    #[test]
    fn decode_errors() {
        let mut m = LogMessage::default();
        assert_eq!(
            decode_log_message(&mut m, b"[1]").unwrap_err(),
            "json: cannot unmarshal array into Go value of type pkg.LogMessage"
        );
        assert_eq!(
            decode_log_message(&mut m, br#"{"message": 12}"#).unwrap_err(),
            "json: cannot unmarshal number into Go struct field LogMessage.message of type string"
        );
        assert_eq!(
            decode_log_message(&mut m, b"{").unwrap_err(),
            "unexpected end of JSON input"
        );
        assert_eq!(
            decode_log_message(&mut m, b"not json").unwrap_err(),
            "invalid character 'o' in literal null (expecting 'u')"
        );
    }

    #[test]
    fn millis_format() {
        // SAFETY: tests of this crate do not read TZ concurrently
        unsafe { std::env::set_var("TZ", "UTC") };
        assert_eq!(
            format_millis(Some(1700000000123)),
            "2023-11-14 22:13:20.123"
        );
        assert_eq!(format_millis(None), "0001-01-01 00:00:00.000");
    }

    #[test]
    fn millis_format_whole_int64_range() {
        // expected values: Go time.UnixMilli(ms).UTC().Format("2006-01-02 15:04:05.000")
        for (ms, go) in [
            (i64::MAX, "292278994-08-17 07:12:55.807"),
            (i64::MIN, "-292275055-05-16 16:47:04.192"),
            (8210298412800000, "262144-01-01 00:00:00.000"),
            (8210298412799999, "262143-12-31 23:59:59.999"),
            (-8334632851200001, "-262145-12-31 23:59:59.999"),
            (-8334632851200000, "-262144-01-01 00:00:00.000"),
            (-62135596800001, "0000-12-31 23:59:59.999"),
            (-62167219200001, "-0001-12-31 23:59:59.999"),
            (253402300800000, "10000-01-01 00:00:00.000"),
            (-1, "1969-12-31 23:59:59.999"),
            (100000000000000000, "3170843-11-07 09:46:40.000"),
            (-100000000000000000, "-3166904-02-24 14:13:20.000"),
        ] {
            assert_eq!(format_millis_in(ms, &chrono::Utc), go, "{ms}");
        }
    }

    #[test]
    fn decode_out_of_range_numbers() {
        let mut m = LogMessage::default();
        assert_eq!(
            decode_log_message(&mut m, br#"{"message": 1e400}"#).unwrap_err(),
            "json: cannot unmarshal number into Go struct field LogMessage.message of type string"
        );
        assert_eq!(
            decode_log_message(&mut m, b"1e400").unwrap_err(),
            "json: cannot unmarshal number into Go value of type pkg.LogMessage"
        );
        assert_eq!(
            decode_log_message(&mut m, br#"{"created_at": 1e400}"#).unwrap_err(),
            "json: cannot unmarshal number 1e400 into Go value of type int64"
        );
    }
}
