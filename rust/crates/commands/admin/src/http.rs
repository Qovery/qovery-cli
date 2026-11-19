//! Raw HTTP calls to the admin API (`ADMIN_URL`), the equivalent of the Go
//! `http.NewRequest(...)` + `http.DefaultClient.Do(req)` blocks of `cmd/admin_*.go` and `pkg/*.go`.

use reqwest::Method;
use std::time::Duration;

/// A response, whatever its status.
pub struct Res {
    pub status: u16,
    /// What `io.ReadAll(res.Body)` returned: the whole body, or what was read before `read_err`.
    pub body: Vec<u8>,
    /// The `io.ReadAll(res.Body)` error (Go text, best effort). Most Go commands ignore it
    /// (`body, _ := io.ReadAll(...)`) and go on with the partial body; the others check it.
    pub read_err: Option<String>,
}

impl Res {
    /// Go `res.Status` (`"404 Not Found"`).
    pub fn status_line(&self) -> String {
        qovery_api::status_line(self.status)
    }
    pub fn text(&self) -> String {
        String::from_utf8_lossy(&self.body).to_string()
    }
}

pub use qovery_core::auth::authorization;

fn go_method(m: &Method) -> String {
    qovery_api::go_url_error_op(m.as_str())
}

/// Request options.
pub struct Req<'a> {
    pub method: Method,
    pub url: &'a str,
    pub authorization: &'a str,
    pub body: Option<Vec<u8>>,
    /// Go sets `Content-Type: application/json` on most admin requests.
    pub json_content_type: bool,
    /// `http.DefaultClient` has no timeout; some commands use `&http.Client{Timeout: ...}`.
    pub timeout: Option<Duration>,
}

impl<'a> Req<'a> {
    pub fn new(method: Method, url: &'a str, authorization: &'a str) -> Self {
        Req {
            method,
            url,
            authorization,
            body: None,
            json_content_type: true,
            timeout: None,
        }
    }
    pub fn body(mut self, body: impl Into<Vec<u8>>) -> Self {
        self.body = Some(body.into());
        self
    }
    pub fn no_content_type(mut self) -> Self {
        self.json_content_type = false;
        self
    }
    pub fn timeout(mut self, d: Duration) -> Self {
        self.timeout = Some(d);
        self
    }

    /// `client.Do(req)`; the error is formatted like Go's `*url.Error`.
    pub fn send(self) -> Result<Res, String> {
        let go_err = |reason: String| {
            format!(
                "{} {}: {}",
                go_method(&self.method),
                qovery_log::gofmt::quote(self.url),
                reason
            )
        };
        let scheme = self.url.split_once("://").map(|(s, _)| s.to_lowercase());
        match scheme.as_deref() {
            Some("http") | Some("https") => {}
            Some(s) => {
                return Err(go_err(format!(
                    "unsupported protocol scheme {}",
                    qovery_log::gofmt::quote(s)
                )));
            }
            None => return Err(go_err("unsupported protocol scheme \"\"".to_string())),
        }
        let client = reqwest::blocking::Client::builder()
            .timeout(self.timeout)
            .build()
            .map_err(|e| go_err(e.to_string()))?;
        let mut req = client
            .request(self.method.clone(), self.url)
            .header("Authorization", self.authorization);
        if self.json_content_type {
            req = req.header("Content-Type", "application/json");
        }
        if let Some(b) = &self.body {
            req = req.body(b.clone());
        }
        let res = req
            .send()
            .map_err(|e| go_err(transport_reason(&e, self.url)))?;
        let status = res.status().as_u16();
        let mut res = res;
        let mut body = Vec::new();
        let read_err = std::io::Read::read_to_end(&mut res, &mut body)
            .err()
            .map(|e| body_read_reason(&e));
        Ok(Res {
            status,
            body,
            read_err,
        })
    }
}

/// Best effort translation of a body read error into the Go `io.ReadAll(res.Body)` error text.
fn body_read_reason(e: &std::io::Error) -> String {
    let mut src: Option<&(dyn std::error::Error + 'static)> = Some(e);
    let mut innermost = e.to_string();
    while let Some(s) = src {
        if let Some(r) = s.downcast_ref::<reqwest::Error>()
            && r.is_timeout()
        {
            return "context deadline exceeded (Client.Timeout or context cancellation while reading body)"
                .to_string();
        }
        if let Some(io) = s.downcast_ref::<std::io::Error>() {
            match io.kind() {
                std::io::ErrorKind::UnexpectedEof => return "unexpected EOF".to_string(),
                std::io::ErrorKind::TimedOut => {
                    return "context deadline exceeded (Client.Timeout or context cancellation while reading body)"
                        .to_string();
                }
                _ => {}
            }
        }
        let text = s.to_string();
        // hyper: the connection closed before the announced Content-Length / last chunk
        if text.contains("end of file before message length reached")
            || text.contains("connection closed before message completed")
        {
            return "unexpected EOF".to_string();
        }
        innermost = text;
        src = s.source();
    }
    innermost
}

/// Best effort translation of a transport error into Go's `net` error text.
fn transport_reason(e: &reqwest::Error, url: &str) -> String {
    let mut src: Option<&dyn std::error::Error> = Some(e);
    while let Some(s) = src {
        if let Some(io) = s.downcast_ref::<std::io::Error>() {
            let addr = host_port(url);
            return match io.kind() {
                std::io::ErrorKind::ConnectionRefused => {
                    format!("dial tcp {addr}: connect: connection refused")
                }
                _ => format!("dial tcp {addr}: {io}"),
            };
        }
        src = s.source();
    }
    if e.is_timeout() {
        return "context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
            .to_string();
    }
    std::error::Error::source(e)
        .map(|s| s.to_string())
        .unwrap_or_else(|| e.to_string())
}

fn host_port(url: &str) -> String {
    let after = url.split_once("://").map(|(_, r)| r).unwrap_or(url);
    let host = &after[..after.find(['/', '?', '#']).unwrap_or(after.len())];
    if host.contains(':') {
        host.to_string()
    } else if url.starts_with("https") {
        format!("{host}:443")
    } else {
        format!("{host}:80")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};

    /// One-shot HTTP server answering `response` then closing the connection.
    fn serve_once(response: &'static [u8]) -> String {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        std::thread::spawn(move || {
            let (mut s, _) = listener.accept().unwrap();
            let mut buf = [0u8; 4096];
            let _ = s.read(&mut buf);
            s.write_all(response).unwrap();
        });
        format!("http://{addr}/x")
    }

    #[test]
    fn whole_body() {
        let url = serve_once(b"HTTP/1.1 404 Not Found\r\nContent-Length: 7\r\n\r\nmissing");
        let res = Req::new(Method::GET, &url, "t").send().unwrap();
        assert_eq!(res.status, 404);
        assert_eq!(res.body, b"missing");
        assert_eq!(res.read_err, None);
    }

    /// Go `io.ReadAll(res.Body)` returns what was read with `unexpected EOF` when the connection
    /// closes before the announced length.
    #[test]
    fn truncated_body() {
        let url = serve_once(b"HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\npartial");
        let res = Req::new(Method::GET, &url, "t").send().unwrap();
        assert_eq!(res.status, 200);
        assert_eq!(res.body, b"partial");
        assert_eq!(res.read_err.as_deref(), Some("unexpected EOF"));
    }
}
