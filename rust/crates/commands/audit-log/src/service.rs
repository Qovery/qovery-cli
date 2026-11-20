//! Port of `pkg/auditlog/audit_log_service.go`.

use qovery_api::models::null_as_default;
use qovery_core::check_error;
use qovery_log::gotime::parse_rfc3339_unix;
use serde::Deserialize;
use std::io::Write;

// Go's `json.Unmarshal` leaves a field untouched on `null` (empty string / slice, zero struct), also for
// `null` items of `events` (a zero `AuditEvent`).
#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct AuditLogResponse {
    #[serde(deserialize_with = "null_as_default")]
    pub links: Links,
    #[serde(deserialize_with = "null_items_as_default")]
    pub events: Vec<AuditEvent>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct Links {
    #[serde(deserialize_with = "null_as_default")]
    pub next: String,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct AuditEvent {
    #[serde(deserialize_with = "null_as_default")]
    pub id: String,
    #[serde(deserialize_with = "null_as_default")]
    pub timestamp: String,
    #[serde(deserialize_with = "null_as_default")]
    pub event_type: String,
    #[serde(deserialize_with = "null_as_default")]
    pub target_id: String,
    #[serde(deserialize_with = "null_as_default")]
    pub target_name: String,
    #[serde(deserialize_with = "null_as_default")]
    pub target_type: String,
    #[serde(deserialize_with = "null_as_default")]
    pub sub_target_type: String,
    #[serde(deserialize_with = "null_as_default")]
    pub origin: String,
    #[serde(deserialize_with = "null_as_default")]
    pub triggered_by: String,
    #[serde(deserialize_with = "null_as_default")]
    pub project_id: String,
    #[serde(deserialize_with = "null_as_default")]
    pub project_name: String,
    #[serde(deserialize_with = "null_as_default")]
    pub environment_id: String,
    #[serde(deserialize_with = "null_as_default")]
    pub environment_name: String,
    #[serde(deserialize_with = "null_as_default")]
    pub environment_type: String,
    #[serde(deserialize_with = "null_as_default")]
    pub user_agent: String,
    #[serde(deserialize_with = "null_as_default")]
    pub change: String,
}

/// `null` items of a slice decode as the zero value (see [`null_as_default`]).
fn null_items_as_default<'de, D, T>(d: D) -> Result<Vec<T>, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Default + Deserialize<'de>,
{
    let items: Option<Vec<Option<T>>> = Option::deserialize(d)?;
    Ok(items
        .unwrap_or_default()
        .into_iter()
        .map(Option::unwrap_or_default)
        .collect())
}

/// `auditlog.DownloadOptions`.
pub struct DownloadOptions {
    pub organization_id: String,
    pub from_date: String,
    pub to_date: String,
    pub token_type: String,
    pub token: String,
}

/// `encoding/csv` `Writer.fieldNeedsQuotes` + quoting (comma separator, `\n` line terminator).
pub fn csv_record(fields: &[&str]) -> String {
    let mut line = String::new();
    for (i, f) in fields.iter().enumerate() {
        if i > 0 {
            line.push(',');
        }
        let needs_quotes = !f.is_empty()
            && (*f == "\\."
                || f.bytes().any(|c| matches!(c, b'\n' | b'\r' | b'"' | b','))
                || f.chars().next().is_some_and(char::is_whitespace));
        if needs_quotes {
            line.push('"');
            line.push_str(&f.replace('"', "\"\""));
            line.push('"');
        } else {
            line.push_str(f);
        }
    }
    line.push('\n');
    line
}

use qovery_api::query_escape;

/// `buildAPIURL`: the Go code always targets the production API, whatever `QOVERY_API_URL` says.
pub fn build_api_url(
    organization_id: &str,
    from_timestamp: i64,
    to_timestamp: i64,
    continue_token: &str,
) -> String {
    let base_url = format!("https://api.qovery.com/organization/{organization_id}/events");
    let mut params: Vec<(&str, String)> = vec![
        ("fromTimestamp", from_timestamp.to_string()),
        ("toTimestamp", to_timestamp.to_string()),
        ("pageSize", "100".to_string()),
    ];
    if !continue_token.is_empty() {
        params.push(("continueToken", continue_token.to_string()));
    }
    params.sort_by(|a, b| a.0.cmp(b.0));
    let query: Vec<String> = params
        .iter()
        .map(|(k, v)| format!("{}={}", query_escape(k), query_escape(v)))
        .collect();
    format!("{base_url}?{}", query.join("&"))
}

fn make_http_request(
    http: &reqwest::blocking::Client,
    api_url: &str,
    token_type: &str,
    token: &str,
) -> Result<AuditLogResponse, String> {
    let resp = check_error(
        http.get(api_url)
            .header("Authorization", format!("{token_type} {token}"))
            .header("Content-Type", "application/json")
            .send()
            .map_err(|e| {
                let reason = std::error::Error::source(&e)
                    .map(|s| s.to_string())
                    .unwrap_or_else(|| e.to_string());
                format!("Get \"{api_url}\": {reason}")
            }),
    );
    let status = resp.status().as_u16();
    let body = resp.bytes().map(|b| b.to_vec()).unwrap_or_default();
    if status != 200 {
        return Err(format!(
            "API request failed with status {status}: {}",
            String::from_utf8_lossy(&body)
        ));
    }
    // `json.Unmarshal(body, &response)`: Go's syntax error texts; `null` keeps the zero response
    check_error(qovery_log::gojson::go_json_check_valid(&body));
    let response: Option<AuditLogResponse> =
        check_error(serde_json::from_slice(&body).map_err(|e| e.to_string()));
    Ok(response.unwrap_or_default())
}

/// `extractContinueToken`.
pub fn extract_continue_token(next_url: &str) -> Result<String, String> {
    let u = check_error(url::Url::parse(next_url).map_err(|e| e.to_string()));
    let token = u
        .query_pairs()
        .find(|(k, _)| k == "continueToken")
        .map(|(_, v)| v.to_string())
        .unwrap_or_default();
    if token.is_empty() {
        return Err("continueToken not found in URL".to_string());
    }
    Ok(token)
}

/// `Service.DownloadAuditLogs`: download the audit logs into `audit_logs_<now>.csv`.
pub fn download_audit_logs(options: &DownloadOptions) -> Result<(), String> {
    let from_timestamp = check_error(parse_rfc3339_unix(&options.from_date));
    let to_timestamp = if !options.to_date.is_empty() {
        check_error(parse_rfc3339_unix(&options.to_date))
    } else {
        chrono::Utc::now().timestamp()
    };

    let filename = format!(
        "audit_logs_{}.csv",
        chrono::Local::now().format("%Y-%m-%d_%H-%M-%S")
    );
    let file = check_error(std::fs::File::create(&filename).map_err(|e| {
        qovery_core::context::go_path_error("open", std::path::Path::new(&filename), &e)
    }));
    // Go's csv.Writer buffers 4096 bytes, flushed at the end (never when exiting on an error)
    let mut csv = std::io::BufWriter::with_capacity(4096, file);

    let header = [
        "timestamp",
        "event_type",
        "target_id",
        "target_name",
        "target_type",
        "sub_target_type",
        "origin",
        "triggered_by",
        "project_id",
        "project_name",
        "environment_id",
        "environment_name",
        "environment_type",
        "user_agent",
        "change",
    ];
    check_error(
        csv.write_all(csv_record(&header).as_bytes())
            .map_err(|e| e.to_string()),
    );

    println!("Downloading audit logs to: {filename}");
    qovery_log::flush();

    let mut continue_token = String::new();
    let mut total_events = 0usize;
    let http = reqwest::blocking::Client::builder()
        .timeout(None)
        .build()
        .map_err(|e| e.to_string())?;

    loop {
        let api_url = build_api_url(
            &options.organization_id,
            from_timestamp,
            to_timestamp,
            &continue_token,
        );
        let response = check_error(make_http_request(
            &http,
            &api_url,
            &options.token_type,
            &options.token,
        ));

        for e in &response.events {
            let record = csv_record(&[
                &e.timestamp,
                &e.event_type,
                &e.target_id,
                &e.target_name,
                &e.target_type,
                &e.sub_target_type,
                &e.origin,
                &e.triggered_by,
                &e.project_id,
                &e.project_name,
                &e.environment_id,
                &e.environment_name,
                &e.environment_type,
                &e.user_agent,
                &e.change,
            ]);
            check_error(csv.write_all(record.as_bytes()).map_err(|e| e.to_string()));
        }

        total_events += response.events.len();
        print!("\r🔄 Processing {total_events} events...");
        qovery_log::flush();

        if response.links.next.is_empty() {
            break;
        }
        match extract_continue_token(&response.links.next) {
            Ok(t) => continue_token = t,
            Err(e) => {
                print!(
                    "\nWarning: Could not parse continue token from URL: {}, error: {e}\n",
                    response.links.next
                );
                break;
            }
        }
    }

    println!("\n✅ Download complete!");
    if let Err(e) = csv.flush() {
        println!("Warning: failed to close file: {e}");
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn csv_quoting() {
        assert_eq!(
            csv_record(&["a", "", "b,c", "{\"k\":\"v\"}", " x", "\\."]),
            "a,,\"b,c\",\"{\"\"k\"\":\"\"v\"\"}\",\" x\",\"\\.\"\n"
        );
    }

    #[test]
    fn api_url() {
        assert_eq!(
            build_api_url("org-1", 10, 20, "a b/c"),
            "https://api.qovery.com/organization/org-1/events?continueToken=a+b%2Fc&fromTimestamp=10&pageSize=100&toTimestamp=20"
        );
    }

    #[test]
    fn null_fields_decode_as_zero_values() {
        let r: AuditLogResponse = serde_json::from_str(
            r#"{"links":{"next":null},"events":[{"id":"e1","target_name":null,"change":null},null]}"#,
        )
        .unwrap();
        assert_eq!(r.links.next, "");
        assert_eq!(r.events.len(), 2);
        assert_eq!(r.events[0].id, "e1");
        assert_eq!(r.events[0].target_name, "");
        assert_eq!(r.events[1].id, "");
        let r: AuditLogResponse = serde_json::from_str(r#"{"links":null,"events":null}"#).unwrap();
        assert!(r.events.is_empty());
        let r: Option<AuditLogResponse> = serde_json::from_str("null").unwrap();
        assert!(r.is_none());
        // type errors still fail, like Go
        assert!(serde_json::from_str::<AuditLogResponse>(r#"{"events":[{"id":1}]}"#).is_err());
    }

    #[test]
    fn continue_token() {
        assert_eq!(
            extract_continue_token("https://x/y?continueToken=abc&z=1"),
            Ok("abc".to_string())
        );
        assert!(extract_continue_token("https://x/y?z=1").is_err());
    }
}
