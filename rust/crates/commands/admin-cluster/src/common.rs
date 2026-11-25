//! Helpers shared by the admin cluster commands: raw HTTP calls (Go `net/http` semantics and error strings),
//! access token, `utils.Validate`, Go time formats, `text/tabwriter`, `exec.LookPath`, `utils.WriteInFile`.

use std::time::Duration;

pub use reqwest::Method;

/// A raw HTTP response (Go `*http.Response` once its body is read).
#[derive(Debug, Clone)]
pub struct HttpResponse {
    pub status: u16,
    pub body: Vec<u8>,
}

impl HttpResponse {
    /// Go `res.Status` (`"404 Not Found"`).
    pub fn status_line(&self) -> String {
        qovery_api::status_line(self.status)
    }
    pub fn text(&self) -> String {
        String::from_utf8_lossy(&self.body).to_string()
    }
}

fn go_method(m: &Method) -> String {
    qovery_api::go_url_error_op(m.as_str())
}

fn innermost_error(e: &(dyn std::error::Error + 'static)) -> String {
    let mut cur: &(dyn std::error::Error + 'static) = e;
    while let Some(s) = cur.source() {
        cur = s;
    }
    cur.to_string()
}

/// Go `*url.Error` message for a failed `client.Do(req)`: `Post "http://x/y": dial tcp 127.0.0.1:1: connect: connection refused`.
/// `context_deadline`: the timeout comes from a `context.WithTimeout` (Go) rather than `http.Client.Timeout`.
pub fn go_transport_error(
    method: &Method,
    url: &str,
    e: &reqwest::Error,
    context_deadline: bool,
) -> String {
    let inner = innermost_error(e);
    let reason = if e.is_timeout() && context_deadline {
        "context deadline exceeded".to_string()
    } else if e.is_timeout() {
        "context deadline exceeded (Client.Timeout exceeded while awaiting headers)".to_string()
    } else if inner.contains("Connection refused") || inner.contains("connection refused") {
        let addr = url::Url::parse(url)
            .ok()
            .and_then(|u| {
                let host = u.host_str()?.to_string();
                let port = u.port_or_known_default()?;
                Some(format!("{host}:{port}"))
            })
            .unwrap_or_default();
        format!("dial tcp {addr}: connect: connection refused")
    } else {
        inner
    };
    format!(
        "{} {}: {}",
        go_method(method),
        qovery_log::gofmt::quote(url),
        reason
    )
}

/// Send a request like Go's `http.NewRequest` + `client.Do` and read the whole body.
/// Errors are formatted like Go's (`*url.Error`).
pub fn http_request(
    method: Method,
    url: &str,
    headers: &[(&str, &str)],
    body: Option<Vec<u8>>,
    timeout: Option<Duration>,
) -> Result<HttpResponse, String> {
    http_request_with(method, url, headers, body, timeout, false)
}

/// [`http_request`] bounded by a Go context deadline (`timeout` is the time left before it).
pub fn http_request_ctx(
    method: Method,
    url: &str,
    headers: &[(&str, &str)],
    body: Option<Vec<u8>>,
    timeout: Duration,
) -> Result<HttpResponse, String> {
    if timeout.is_zero() {
        return Err(format!(
            "{} {}: context deadline exceeded",
            go_method(&method),
            qovery_log::gofmt::quote(url)
        ));
    }
    http_request_with(method, url, headers, body, Some(timeout), true)
}

fn http_request_with(
    method: Method,
    url: &str,
    headers: &[(&str, &str)],
    body: Option<Vec<u8>>,
    timeout: Option<Duration>,
    context_deadline: bool,
) -> Result<HttpResponse, String> {
    let parsed = match url::Url::parse(url) {
        Ok(u) => u,
        Err(url::ParseError::RelativeUrlWithoutBase) => {
            return Err(format!(
                "{} {}: unsupported protocol scheme \"\"",
                go_method(&method),
                qovery_log::gofmt::quote(url)
            ));
        }
        Err(e) => return Err(format!("parse {}: {}", qovery_log::gofmt::quote(url), e)),
    };
    if !matches!(parsed.scheme(), "http" | "https") {
        return Err(format!(
            "{} {}: unsupported protocol scheme {}",
            go_method(&method),
            qovery_log::gofmt::quote(url),
            qovery_log::gofmt::quote(parsed.scheme())
        ));
    }
    let mut builder = qovery_api::http_client_builder();
    builder = builder.timeout(timeout);
    let client = builder.build().map_err(|e| e.to_string())?;
    let mut req = client.request(method.clone(), parsed);
    for (k, v) in headers {
        req = req.header(*k, *v);
    }
    if let Some(b) = body {
        req = req.body(b);
    }
    let mut res = req
        .send()
        .map_err(|e| go_transport_error(&method, url, &e, context_deadline))?;
    let status = res.status().as_u16();
    // Go's callers read the body with `body, _ := io.ReadAll(res.Body)`: a read error is ignored
    // and the bytes read before it are kept
    let mut body = Vec::new();
    let _ = std::io::Read::read_to_end(&mut res, &mut body);
    Ok(HttpResponse { status, body })
}

pub use qovery_core::auth::authorization;

/// Admin request with the `Authorization` and `Content-Type: application/json` headers set by most `pkg/*.go` helpers.
pub fn admin_request(
    method: Method,
    url: &str,
    authorization: &str,
    body: Option<Vec<u8>>,
) -> Result<HttpResponse, String> {
    http_request(
        method,
        url,
        &[
            ("Authorization", authorization),
            ("Content-Type", "application/json"),
        ],
        body,
        None,
    )
}

/// `utils.Validate(actionType)` (promptui byte-identical on a non-terminal stdin).
pub use qovery_core::select::validate;

/// `utils.GetAdminUrl()`.
pub fn admin_url() -> String {
    qovery_core::client::admin_url()
}

pub use qovery_log::gofmt::{
    time_rfc1123 as rfc1123, time_rfc3339 as rfc3339, time_rfc3339_nano as rfc3339_nano,
};

pub use qovery_log::tabwriter::tabwrite as tabwriter;

pub use qovery_core::gosys::look_path;

pub use qovery_core::gosys::errno_text as go_os_error;

/// Run a child process with inherited stdio like Go's `cmd.Run()`; errors formatted like Go's.
pub fn run_inherited(program: &str, args: &[&str]) -> Result<(), String> {
    let path = if program.contains('/') {
        std::path::PathBuf::from(program)
    } else {
        look_path(program)?
    };
    let status = std::process::Command::new(&path)
        .args(args)
        .status()
        .map_err(|e| format!("fork/exec {}: {}", path.display(), go_os_error(&e)))?;
    if status.success() {
        return Ok(());
    }
    #[cfg(unix)]
    let signal = std::os::unix::process::ExitStatusExt::signal(&status);
    // a Windows process always has an exit code
    #[cfg(not(unix))]
    let signal: Option<i32> = None;
    match (status.code(), signal) {
        (Some(code), _) => Err(format!("exit status {code}")),
        (None, Some(sig)) => Err(format!("signal: {}", signal_name(sig))),
        _ => Err("exit status -1".to_string()),
    }
}

fn signal_name(sig: i32) -> String {
    match sig {
        1 => "hangup".into(),
        2 => "interrupt".into(),
        9 => "killed".into(),
        15 => "terminated".into(),
        n => format!("signal {n}"),
    }
}

/// `utils.WriteInFile` / `utils.DeleteFolder`.
pub use qovery_services::{delete_folder, write_in_file};

/// `os.Setenv` (the process is single threaded when commands call it).
pub fn set_env(key: &str, value: &str) {
    // SAFETY: called from the single command thread, before any other thread reads the environment.
    unsafe { std::env::set_var(key, value) }
}

/// `os.Unsetenv`.
pub fn unset_env(key: &str) {
    // SAFETY: see `set_env`.
    unsafe { std::env::remove_var(key) }
}

/// Value of an object key, Go `encoding/json` style: exact match first, then case-insensitive.
pub fn go_field<'a>(obj: &'a serde_json::Value, key: &str) -> Option<&'a serde_json::Value> {
    let map = obj.as_object()?;
    if let Some(v) = map.get(key) {
        return Some(v);
    }
    map.iter()
        .find(|(k, _)| k.eq_ignore_ascii_case(key))
        .map(|(_, v)| v)
}

/// Go `strconv.FormatBool`.
pub fn format_bool(b: bool) -> String {
    if b { "true".into() } else { "false".into() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tabwriter_aligns_columns() {
        let lines = vec![
            "\t | cluster_id\t | ttl\n".to_string(),
            "1\t | c-1\t | infinite\n".to_string(),
            "10\t | cluster-22\t | 3\n".to_string(),
        ];
        assert_eq!(
            tabwriter(&lines, 1, 1),
            "    | cluster_id  | ttl\n1   | c-1         | infinite\n10  | cluster-22  | 3\n"
        );
    }

    #[test]
    fn go_times() {
        let t = chrono::DateTime::parse_from_rfc3339("2024-01-02T03:04:05.120Z").unwrap();
        assert_eq!(rfc3339(&t), "2024-01-02T03:04:05Z");
        assert_eq!(rfc3339_nano(&t), "2024-01-02T03:04:05.12Z");
        assert_eq!(rfc1123(&t), "Tue, 02 Jan 2024 03:04:05 UTC");
        let t = chrono::DateTime::parse_from_rfc3339("2024-01-02T03:04:05+02:00").unwrap();
        assert_eq!(rfc3339(&t), "2024-01-02T03:04:05+02:00");
        assert_eq!(rfc1123(&t), "Tue, 02 Jan 2024 03:04:05 +0200");
    }

    #[test]
    fn truncated_body_is_kept_like_go_read_all() {
        use std::io::{Read, Write};
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let server = std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut buf = [0u8; 4096];
            let _ = stream.read(&mut buf);
            // announces 100 bytes, sends 7, then closes the connection
            stream
                .write_all(
                    b"HTTP/1.1 500 Internal Server Error\r\nContent-Length: 100\r\n\r\npartial",
                )
                .unwrap();
        });
        let res = http_request(Method::GET, &format!("http://{addr}/"), &[], None, None).unwrap();
        server.join().unwrap();
        assert_eq!(res.status, 500);
        assert_eq!(res.text(), "partial");
    }
}
