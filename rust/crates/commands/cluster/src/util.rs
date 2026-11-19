//! Small Go compatibility helpers used by the cluster commands.
//!
//! Some of them are local copies of `utils/*.go` helpers that are not (yet) in the shared crates.

use chrono::{DateTime, FixedOffset};
use qovery_api::ApiError;

pub use qovery_api::gourl::{encode_query, path_escape};

/// Parse an RFC 3339 timestamp the way Go's `time.Time` JSON decoding does.
pub fn parse_time(s: &str) -> Option<DateTime<FixedOffset>> {
    DateTime::parse_from_rfc3339(s).ok()
}

/// `utils.ToIso8601`: `t.Format("2006-01-02T15:04:05.000Z")` (literal `Z`, in the time's own offset).
pub fn to_iso8601(s: &str) -> Option<String> {
    parse_time(s).map(|t| qovery_log::gofmt::time_iso8601(&t))
}

pub use qovery_log::gofmt::{
    time_rfc1123 as format_rfc1123, time_rfc3339 as format_rfc3339,
    time_rfc3339_nano as format_rfc3339_nano,
};

/// Go zero `time.Time` formatted with RFC1123.
pub const ZERO_TIME_RFC1123: &str = "Mon, 01 Jan 0001 00:00:00 UTC";

pub use qovery_services::{is_empty_or_blank, is_terminal_cluster_state};

/// Go `text/tabwriter` (see `qovery_log::tabwriter`) of lines given as cells (the last cell holds the `\n`).
pub fn tabwriter(lines: &[Vec<String>], minwidth: usize, padding: usize) -> String {
    qovery_log::tabwriter::tabwrite(
        &lines.iter().map(|l| l.join("\t")).collect::<Vec<_>>(),
        minwidth,
        padding,
    )
}

/// `fmt.Sprint(b)` of a `[]byte`: `[104 105]`.
pub fn go_bytes_sprint(b: &[u8]) -> String {
    format!(
        "[{}]",
        b.iter()
            .map(|x| x.to_string())
            .collect::<Vec<_>>()
            .join(" ")
    )
}

/// Port of `LogDetail` (cmd/cluster_lock.go): log the `detail` of an API error body.
pub fn log_detail(body: &[u8]) {
    let detail = serde_json::from_slice::<serde_json::Value>(body)
        .ok()
        .and_then(|v| match v {
            serde_json::Value::Object(m) => {
                // encoding/json matches field names case-insensitively; a non string detail is a decode error
                let mut detail = Some(String::new());
                for (k, v) in m.iter() {
                    if k.eq_ignore_ascii_case("detail") {
                        detail = match v {
                            serde_json::Value::String(s) => Some(s.clone()),
                            serde_json::Value::Null => detail,
                            _ => None,
                        };
                    }
                }
                detail
            }
            serde_json::Value::Null => Some(String::new()),
            _ => None,
        });
    match detail {
        Some(d) if !d.is_empty() => qovery_log::logrus::error(format!("Error detail: {d}")),
        _ => qovery_log::logrus::error(go_bytes_sprint(body)),
    }
}

/// Port of `httpError(res, err)` (cmd/cluster_analysis.go).
pub fn http_error(e: &ApiError) -> String {
    match e.status {
        None => e.message.clone(),
        Some(code) => {
            format!(
                "status code: {} ; body: {} ; original error: {}",
                qovery_api::status_line(code),
                e.body,
                e.message
            )
        }
    }
}

/// Go's `%s` of a nil error.
pub const GO_NIL_ERROR: &str = "%!s(<nil>)";

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn time_formats() {
        let t = parse_time("2024-05-01T10:00:00.123456Z").unwrap();
        assert_eq!(
            to_iso8601("2024-05-01T10:00:00.123456Z").unwrap(),
            "2024-05-01T10:00:00.123Z"
        );
        assert_eq!(format_rfc3339(&t), "2024-05-01T10:00:00Z");
        assert_eq!(format_rfc3339_nano(&t), "2024-05-01T10:00:00.123456Z");
        assert_eq!(format_rfc1123(&t), "Wed, 01 May 2024 10:00:00 UTC");
        let t = parse_time("2024-05-01T10:00:00+02:00").unwrap();
        assert_eq!(format_rfc3339(&t), "2024-05-01T10:00:00+02:00");
        assert_eq!(format_rfc1123(&t), "Wed, 01 May 2024 10:00:00 +0200");
    }

    #[test]
    fn tabwriter_aligns_like_go() {
        let lines = vec![
            vec!["".into(), " | cluster_id".into(), " | x\n".into()],
            vec!["1".into(), " | c".into(), " | yy\n".into()],
        ];
        assert_eq!(
            tabwriter(&lines, 1, 1),
            "   | cluster_id  | x\n1  | c           | yy\n"
        );
    }

    #[test]
    fn terminal_states() {
        assert!(is_terminal_cluster_state("DEPLOYED"));
        assert!(is_terminal_cluster_state("DEPLOYMENT_ERROR"));
        assert!(!is_terminal_cluster_state("DEPLOYING"));
    }

    #[test]
    fn escapes() {
        assert_eq!(path_escape("a b/c"), "a%20b%2Fc");
        assert_eq!(encode_query(&[("z", "1"), ("a", "x y")]), "a=x+y&z=1");
    }
}
