//! Port of `uploadErrorLogs` & co (`cmd/demo_up.go`): upload of the debug log after a failed `qovery demo up`.

use qovery_core::auth::TokenType;
use serde::Serialize;
use std::time::Duration;

/// `demoDebugLogRetryDelay`.
pub(crate) const DEMO_DEBUG_LOG_RETRY_DELAY: Duration = Duration::from_secs(2);

/// Error of a debug log upload (`*demoDebugLogUploadError` when a response was received).
#[derive(Debug, Clone, PartialEq)]
pub(crate) struct UploadError {
    pub message: String,
    pub status_code: Option<u16>,
}

impl std::fmt::Display for UploadError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

/// `isDemoDebugLogAuthFailure(err)`.
pub(crate) fn is_demo_debug_log_auth_failure(err: &UploadError) -> bool {
    matches!(err.status_code, Some(401) | Some(403))
}

fn go_method_url_error(endpoint: &str, e: &reqwest::Error) -> String {
    let mut reason = e.to_string();
    let mut source = std::error::Error::source(e);
    while let Some(s) = source {
        reason = s.to_string();
        source = s.source();
    }
    format!("Post {}: {}", qovery_log::gofmt::quote(endpoint), reason)
}

/// `postDemoDebugLogOnce`: (retryable, error).
fn post_demo_debug_log_once(
    http: &reqwest::blocking::Client,
    endpoint: &str,
    authorization: &str,
    payload: &[u8],
) -> (bool, Result<(), UploadError>) {
    let Ok(url) = reqwest::Url::parse(endpoint) else {
        let message = format!(
            "parse {}: missing protocol scheme",
            qovery_log::gofmt::quote(endpoint)
        );
        return (
            false,
            Err(UploadError {
                message,
                status_code: None,
            }),
        );
    };
    let response = http
        .post(url)
        .header("Authorization", authorization)
        .header("Content-Type", "application/json")
        .body(payload.to_vec())
        .send();
    let response = match response {
        Ok(r) => r,
        Err(e) => {
            return (
                true,
                Err(UploadError {
                    message: go_method_url_error(endpoint, &e),
                    status_code: None,
                }),
            );
        }
    };
    let status = response.status().as_u16();
    if status == 200 {
        return (false, Ok(()));
    }
    let body = response.bytes().map(|b| b.to_vec()).unwrap_or_default();
    let message = format!(
        "{} {}",
        qovery_api::status_line(status),
        String::from_utf8_lossy(&body)
    );
    (
        status >= 500,
        Err(UploadError {
            message,
            status_code: Some(status),
        }),
    )
}

/// `postDemoDebugLog(httpClient, endpoint, authorization, payload, retryDelay)`.
pub(crate) fn post_demo_debug_log(
    http: &reqwest::blocking::Client,
    endpoint: &str,
    authorization: &str,
    payload: &[u8],
    retry_delay: Duration,
) -> Result<(), UploadError> {
    let (retryable, result) = post_demo_debug_log_once(http, endpoint, authorization, payload);
    if result.is_ok() || !retryable {
        return result;
    }
    std::thread::sleep(retry_delay);
    post_demo_debug_log_once(http, endpoint, authorization, payload).1
}

#[derive(Serialize)]
struct Payload<'a> {
    organization: &'a str,
    cluster_name: &'a str,
    content: String,
    os: &'a str,
    cpu_arch: String,
    cli_version: &'a str,
    timestamp: String,
}

/// `uploadErrorLogs(tokenType, token, organization, clusterName, debugLogsPath)`.
pub(crate) fn upload_error_logs(
    token_type: TokenType,
    token: &str,
    organization: &str,
    cluster_name: &str,
    debug_logs_path: &str,
) {
    let content = std::fs::read(debug_logs_path)
        .map(|c| String::from_utf8_lossy(&c).to_string())
        .unwrap_or_default();
    let payload = Payload {
        organization,
        cluster_name,
        content,
        os: crate::go_os(),
        cpu_arch: crate::detect_architecture(),
        cli_version: qovery_core::version::VERSION,
        timestamp: qovery_core::auth::go_time_json(chrono::Local::now()),
    };
    let payload_json = qovery_log::gofmt::go_marshal(&payload);

    let client = qovery_core::client::client_for(token_type, token);
    let query = url::form_urlencoded::Serializer::new(String::new())
        .append_pair("clusterName", cluster_name)
        .append_pair("organization", organization)
        .finish();
    let endpoint = format!("{}/admin/demoDebugLog?{}", client.base_url(), query);
    let http = reqwest::blocking::Client::builder()
        .timeout(Duration::from_secs(30))
        .build()
        .expect("http client");
    let authorization = qovery_core::auth::authorization_header_value(token_type, token);
    if let Err(err) = post_demo_debug_log(
        &http,
        &endpoint,
        &authorization,
        payload_json.as_bytes(),
        DEMO_DEBUG_LOG_RETRY_DELAY,
    ) {
        qovery_log::println_error(format!("error uploading debug logs: {err}"));
        if is_demo_debug_log_auth_failure(&err) {
            qovery_log::println_info(
                "May be caused by a wrong context set, please set it again: `qovery context set`",
            );
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{BufRead, BufReader, Read, Write};
    use std::sync::{Arc, Mutex};

    /// A server answering the given statuses in sequence, recording bodies and checking headers.
    fn server(statuses: Vec<u16>) -> (String, Arc<Mutex<Vec<String>>>) {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!(
            "http://{}/admin/demoDebugLog",
            listener.local_addr().unwrap()
        );
        let bodies = Arc::new(Mutex::new(Vec::new()));
        let shared = bodies.clone();
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                let Ok(mut stream) = stream else { continue };
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut line = String::new();
                let _ = reader.read_line(&mut line);
                let (mut length, mut auth, mut ctype) = (0usize, String::new(), String::new());
                loop {
                    let mut header = String::new();
                    if reader.read_line(&mut header).is_err() || header.trim().is_empty() {
                        break;
                    }
                    if let Some((k, v)) = header.split_once(':') {
                        let v = v.trim().to_string();
                        match k.to_ascii_lowercase().as_str() {
                            "content-length" => length = v.parse().unwrap_or(0),
                            "authorization" => auth = v,
                            "content-type" => ctype = v,
                            _ => {}
                        }
                    }
                }
                assert_eq!("Token abc", auth);
                assert_eq!("application/json", ctype);
                let mut body = vec![0u8; length];
                let _ = reader.read_exact(&mut body);
                let index = {
                    let mut b = shared.lock().unwrap();
                    b.push(String::from_utf8_lossy(&body).to_string());
                    b.len() - 1
                };
                let status = statuses[index.min(statuses.len() - 1)];
                let response = format!(
                    "HTTP/1.1 {}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
                    qovery_api::status_line(status)
                );
                let _ = stream.write_all(response.as_bytes());
            }
        });
        (url, bodies)
    }

    #[test]
    fn retries_once_on_server_errors() {
        let cases: Vec<(Vec<u16>, usize, bool, bool)> = vec![
            (vec![200], 1, false, false),
            (vec![500, 200], 2, false, false),
            (vec![502, 503], 2, true, false),
            (vec![400], 1, true, false),
            (vec![401], 1, true, true),
            (vec![403], 1, true, true),
        ];
        for (statuses, want_requests, want_err, want_auth_error) in cases {
            let (url, bodies) = server(statuses);
            let http = reqwest::blocking::Client::new();
            let result = post_demo_debug_log(
                &http,
                &url,
                "Token abc",
                br#"{"content":"log"}"#,
                Duration::ZERO,
            );
            let bodies = bodies.lock().unwrap().clone();
            assert_eq!(want_requests, bodies.len());
            for body in bodies {
                assert_eq!(r#"{"content":"log"}"#, body);
            }
            match result {
                Ok(()) => assert!(!want_err),
                Err(e) => {
                    assert!(want_err);
                    assert_eq!(want_auth_error, is_demo_debug_log_auth_failure(&e));
                }
            }
        }
    }

    #[test]
    fn retries_once_on_a_network_error() {
        // A port nobody listens on.
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!(
            "http://{}/admin/demoDebugLog",
            listener.local_addr().unwrap()
        );
        drop(listener);
        let http = reqwest::blocking::Client::new();
        let err = post_demo_debug_log(&http, &url, "Token abc", b"{}", Duration::ZERO).unwrap_err();
        assert!(!is_demo_debug_log_auth_failure(&err));
        assert!(err.message.starts_with("Post \""), "{}", err.message);
    }

    #[test]
    fn does_not_retry_an_invalid_endpoint() {
        let started_at = std::time::Instant::now();
        let http = reqwest::blocking::Client::new();
        let err = post_demo_debug_log(
            &http,
            "://no-scheme",
            "Token abc",
            b"{}",
            Duration::from_millis(100),
        )
        .unwrap_err();
        assert!(err.status_code.is_none());
        assert!(started_at.elapsed() < Duration::from_millis(100));
    }
}
