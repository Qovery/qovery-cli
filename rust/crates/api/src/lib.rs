//! Thin blocking client of the Qovery REST API (`https://api.qovery.com`).
//!
//! The Go CLI uses the generated `qovery-client-go`. Endpoint paths, methods and payloads can be read
//! from its sources: `$(go env GOMODCACHE)/github.com/qovery/qovery-client-go@*/api_*.go`.
//!
//! Errors are formatted like the Go client's `GenericOpenAPIError.Error()`, i.e. the HTTP status line
//! (`404 Not Found`) for non-2xx responses, which is what the Go commands print with `utils.PrintlnError(err)`.
//!
//! Responses can be decoded into typed structs (see [`models`]) or into `serde_json::Value`.

pub mod gourl;
pub mod models;

pub use gourl::{encode_query, path_escape, query_escape};

use serde::Serialize;
use serde::de::DeserializeOwned;
use serde_json::Value;
use std::time::Duration;

pub use reqwest::Method;

/// Go `withAPIErrorDetail(err)` (`utils/env_var.go`) on a `GenericOpenAPIError` whose `Error()` is `message`
/// and `Body()` is `body`: append the API's own message (the `detail` of the problem response) or, without a
/// JSON `detail`, the raw body. An error without a body is returned unchanged.
pub fn with_api_error_detail(message: &str, body: &str) -> String {
    if body.is_empty() {
        return message.to_string();
    }
    match problem_detail(body) {
        Some(detail) if !detail.is_empty() => format!("{message}: {detail}"),
        _ => format!("{message}: {body}"),
    }
}

/// `json.Unmarshal(body, &struct{ Detail string `json:"detail"` })`: `None` when Go fails to unmarshal.
/// Go matches the key case-insensitively and ignores `null`.
fn problem_detail(body: &str) -> Option<String> {
    let v: Value = serde_json::from_str(body).ok()?;
    match v {
        Value::Null => Some(String::new()),
        Value::Object(m) => {
            let mut detail = String::new();
            let mut exact = None;
            for (k, val) in &m {
                if !k.eq_ignore_ascii_case("detail") {
                    continue;
                }
                match val {
                    Value::Null => {}
                    Value::String(s) => {
                        if k == "detail" {
                            exact = Some(s.clone());
                        } else {
                            detail = s.clone();
                        }
                    }
                    _ => return None,
                }
            }
            Some(exact.unwrap_or(detail))
        }
        _ => None,
    }
}

/// The generated Go client's `JsonCheck`: `(?i:(?:application|text)/(?:[^;]+\+)?json)` (unanchored). A
/// response whose `Content-Type` does not match is not decoded: Go fails with `undefined response type`.
pub fn is_go_json_content_type(content_type: &str) -> bool {
    let ct = content_type.to_ascii_lowercase();
    ["application/", "text/"].iter().any(|prefix| {
        ct.match_indices(prefix).any(|(i, _)| {
            let rest = &ct[i + prefix.len()..];
            rest.starts_with("json")
                || rest
                    .match_indices("+json")
                    .any(|(j, _)| j > 0 && !rest[..j].contains(';'))
        })
    })
}

/// Error returned by API calls; `Display` matches the Go client's error string.
#[derive(Debug, Clone)]
pub struct ApiError {
    /// HTTP status code when a response was received
    pub status: Option<u16>,
    /// The message (Go: `GenericOpenAPIError.error`)
    pub message: String,
    /// Raw response body (Go: `GenericOpenAPIError.Body()`)
    pub body: String,
}

impl std::fmt::Display for ApiError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for ApiError {}

impl ApiError {
    pub fn new(message: impl Into<String>) -> Self {
        ApiError {
            status: None,
            message: message.into(),
            body: String::new(),
        }
    }
    pub fn is_unauthorized(&self) -> bool {
        self.status == Some(401)
    }
    pub fn is_not_found(&self) -> bool {
        self.status == Some(404)
    }
}

/// Go `http.Response.Status`: `"404 Not Found"`.
pub fn status_line(code: u16) -> String {
    let reason = reqwest::StatusCode::from_u16(code)
        .ok()
        .and_then(|s| s.canonical_reason())
        .unwrap_or("");
    if reason.is_empty() {
        format!("{code} status code {code}")
    } else {
        format!("{code} {reason}")
    }
}

/// Go `urlErrorOp(method)`: the operation of a `*url.Error` (`Get`, `Post`, ...): `method[:1] + strings.ToLower(method[1:])`.
pub fn go_url_error_op(method: &str) -> String {
    let mut c = method.chars();
    match c.next() {
        Some(f) => f.to_string() + &c.as_str().to_lowercase(),
        None => String::new(),
    }
}

fn go_method(m: &Method) -> String {
    go_url_error_op(m.as_str())
}

/// A raw API response.
#[derive(Debug, Clone)]
pub struct Response {
    pub status: u16,
    /// Reason phrase sent by the server when it differs from the standard one (HTTP/1.x)
    pub reason: Option<String>,
    pub body: Vec<u8>,
    pub headers: reqwest::header::HeaderMap,
}

impl Response {
    pub fn text(&self) -> String {
        String::from_utf8_lossy(&self.body).to_string()
    }
    /// Go `res.Status` (the server's reason phrase over HTTP/1.x, the standard one otherwise).
    pub fn status_line(&self) -> String {
        match &self.reason {
            Some(r) => format!("{} {}", self.status, r),
            None => status_line(self.status),
        }
    }
}

/// Qovery API client.
#[derive(Clone)]
pub struct Client {
    http: reqwest::blocking::Client,
    base_url: String,
    authorization: String,
    user_agent: String,
    verbose: bool,
}

/// `utils.GetAPIBaseURL()`: `QOVERY_API_URL` (trailing `/` trimmed) or `https://api.qovery.com`.
pub fn api_base_url() -> String {
    match std::env::var("QOVERY_API_URL") {
        Ok(u) if !u.is_empty() => u.trim_end_matches('/').to_string(),
        _ => "https://api.qovery.com".to_string(),
    }
}

/// `reqwest::blocking::Client::builder()` with Go's proxy semantics. The proxy comes from the environment as with
/// reqwest (`HTTPS_PROXY`, `HTTP_PROXY`, `ALL_PROXY`, `NO_PROXY`), but a `socks5://` proxy resolves the host names
/// at the proxy, as Go's `net/http` does (`socks5h` for reqwest). The bastion tunnel of `qovery admin` exports
/// `HTTPS_PROXY=socks5://...` (Go and kube clients reject `socks5h`) and a name resolved locally may give an
/// address the bastion cannot reach (IPv6).
pub fn http_client_builder() -> reqwest::blocking::ClientBuilder {
    let builder = reqwest::blocking::Client::builder();
    let Some([https, http, all]) = remote_dns_proxies(|n| std::env::var(n).ok()) else {
        return builder;
    };
    // the first proxy matching the request scheme wins: the scheme's own variable, then ALL_PROXY
    let no_proxy = reqwest::NoProxy::from_env();
    let proxies = [
        reqwest::Proxy::https(https),
        reqwest::Proxy::http(http),
        reqwest::Proxy::all(all),
    ];
    // explicit proxies turn the environment ones off
    let mut builder = builder.no_proxy();
    for proxy in proxies.into_iter().flatten() {
        builder = builder.proxy(proxy.no_proxy(no_proxy.clone()));
    }
    builder
}

/// The `[HTTPS, HTTP, ALL]` proxy variables read as reqwest does, with `socks5://` turned into `socks5h://`;
/// `None` when no variable is a `socks5://` one (the reqwest environment proxies are then kept as is).
fn remote_dns_proxies(var: impl Fn(&str) -> Option<String>) -> Option<[String; 3]> {
    // reqwest ignores the proxy variables in a CGI script
    if var("REQUEST_METHOD").is_some() {
        return None;
    }
    let first = |names: [&str; 2]| names.iter().find_map(|n| var(n)).unwrap_or_default();
    let proxies = [
        first(["HTTPS_PROXY", "https_proxy"]),
        first(["HTTP_PROXY", "http_proxy"]),
        first(["ALL_PROXY", "all_proxy"]),
    ];
    let is_socks5 = |v: &str| {
        v.get(..9)
            .is_some_and(|s| s.eq_ignore_ascii_case("socks5://"))
    };
    if !proxies.iter().any(|v| is_socks5(v)) {
        return None;
    }
    Some(proxies.map(|v| {
        if is_socks5(&v) {
            format!("socks5h://{}", &v[9..])
        } else {
            v
        }
    }))
}

/// `reqwest::blocking::get` through [`http_client_builder`].
pub fn http_get<U: reqwest::IntoUrl>(url: U) -> reqwest::Result<reqwest::blocking::Response> {
    http_client_builder().build()?.get(url).send()
}

impl Client {
    /// `authorization` is the full header value, e.g. `Bearer xxx` or `Token xxx`.
    pub fn new(authorization: &str, cli_version: &str, verbose: bool) -> Self {
        let http = http_client_builder()
            .timeout(Duration::from_secs(60))
            .build()
            .expect("cannot build http client");
        Client {
            http,
            base_url: api_base_url(),
            authorization: authorization.to_string(),
            user_agent: format!("CLI {cli_version}"),
            verbose,
        }
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    pub fn authorization(&self) -> &str {
        &self.authorization
    }

    /// The underlying reqwest client (60s timeout).
    pub fn http(&self) -> &reqwest::blocking::Client {
        &self.http
    }

    fn url(&self, path: &str) -> String {
        if path.starts_with("http://") || path.starts_with("https://") {
            path.to_string()
        } else if path.starts_with('/') {
            format!("{}{}", self.base_url, path)
        } else {
            format!("{}/{}", self.base_url, path)
        }
    }

    /// Go `httputil.DumpRequestOut` logged with `log.Printf("\n%s\n", dump)` (client `Debug` mode, `--verbose`).
    fn dump_request(
        &self,
        method: &Method,
        url: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
        accept: &str,
    ) {
        let parsed = url::Url::parse(url).ok();
        let host = parsed
            .as_ref()
            .map(|u| match u.port() {
                Some(p) => format!("{}:{p}", u.host_str().unwrap_or("")),
                None => u.host_str().unwrap_or("").to_string(),
            })
            .unwrap_or_default();
        let mut path = parsed
            .as_ref()
            .map(|u| u.path().to_string())
            .unwrap_or_default();
        let mut pairs: Vec<(String, String)> = parsed
            .as_ref()
            .map(|u| {
                u.query_pairs()
                    .map(|(k, v)| (k.to_string(), v.to_string()))
                    .collect()
            })
            .unwrap_or_default();
        pairs.extend(query.iter().map(|(k, v)| (k.to_string(), v.clone())));
        if !pairs.is_empty() {
            pairs.sort_by(|a, b| a.0.cmp(&b.0));
            let mut ser = url::form_urlencoded::Serializer::new(String::new());
            for (k, v) in &pairs {
                ser.append_pair(k, v);
            }
            path = format!("{path}?{}", ser.finish());
        }
        let payload = body.map(|b| serde_json::to_vec(b).unwrap_or_default());
        let mut d = format!(
            "{} {} HTTP/1.1\r\nHost: {}\r\nUser-Agent: {}\r\n",
            method, path, host, self.user_agent
        );
        if let Some(p) = &payload {
            d.push_str(&format!("Content-Length: {}\r\n", p.len()));
        }
        d.push_str(&format!("Accept: {accept}\r\n"));
        d.push_str(&format!("Authorization: {}\r\n", self.authorization));
        if payload.is_some() {
            d.push_str("Content-Type: application/json\r\n");
        }
        d.push_str("Accept-Encoding: gzip\r\n\r\n");
        if let Some(p) = &payload {
            d.push_str(&String::from_utf8_lossy(p));
        }
        go_log_printf(&format!("\n{d}\n"));
    }

    /// Send a request and return the raw response, whatever its status.
    pub fn send(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
    ) -> Result<Response, ApiError> {
        self.send_accept(method, path, query, body, "application/json")
    }

    /// [`Client::send`] with the `Accept` header of the endpoint: the Go generated client sends the media type
    /// the OpenAPI spec declares (`application/x-yaml` for the kubeconfig, ...), and the API answers
    /// `406 Not Acceptable` to `application/json` there.
    pub fn send_accept(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
        accept: &str,
    ) -> Result<Response, ApiError> {
        let url = self.url(path);
        let mut req = self
            .http
            .request(method.clone(), &url)
            .header("Authorization", &self.authorization)
            .header("User-Agent", &self.user_agent)
            .header("Accept", accept);
        if !query.is_empty() {
            req = req.query(query);
        }
        if let Some(b) = body {
            req = req
                .header("Content-Type", "application/json")
                .body(serde_json::to_vec(b).unwrap_or_default());
        }
        if self.verbose {
            self.dump_request(&method, &url, query, body, accept);
        }
        let res = req.send().map_err(|e| {
            let mut full_url = url.clone();
            if !query.is_empty()
                && let Ok(mut u) = url::Url::parse(&url)
            {
                u.query_pairs_mut()
                    .extend_pairs(query.iter().map(|(k, v)| (*k, v.as_str())));
                full_url = u.to_string();
            }
            ApiError::new(format!(
                "{} \"{}\": {}",
                go_method(&method),
                full_url,
                go_transport_error(&e, &url)
            ))
        })?;
        let status = res.status().as_u16();
        let reason = res
            .extensions()
            .get::<hyper::ext::ReasonPhrase>()
            .map(|r| String::from_utf8_lossy(r.as_bytes()).to_string());
        let headers = res.headers().clone();
        let body = res.bytes().map(|b| b.to_vec()).unwrap_or_default();
        let response = Response {
            status,
            reason,
            body,
            headers,
        };
        if self.verbose {
            dump_response(&response);
        }
        Ok(response)
    }

    /// Like the Go generated client: error (status line) when status >= 300, otherwise the response.
    pub fn execute(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
    ) -> Result<Response, ApiError> {
        self.execute_accept(method, path, query, body, "application/json")
    }

    /// [`Client::execute`] with the `Accept` header of the endpoint, see [`Client::send_accept`].
    pub fn execute_accept(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
        accept: &str,
    ) -> Result<Response, ApiError> {
        let res = self.send_accept(method, path, query, body, accept)?;
        if res.status >= 300 {
            return Err(ApiError {
                status: Some(res.status),
                message: res.status_line(),
                body: res.text(),
            });
        }
        Ok(res)
    }

    /// Decode the JSON body of a successful response (an empty body decodes as `null`).
    pub fn decode<T: DeserializeOwned>(res: Response) -> Result<T, ApiError> {
        let body = if res.body.is_empty() {
            b"null".to_vec()
        } else {
            res.body.clone()
        };
        serde_json::from_slice(&body).map_err(|e| ApiError {
            status: Some(res.status),
            message: e.to_string(),
            body: res.text(),
        })
    }

    pub fn get<T: DeserializeOwned>(&self, path: &str) -> Result<T, ApiError> {
        Self::decode(self.execute(Method::GET, path, &[], None)?)
    }

    pub fn get_with_query<T: DeserializeOwned>(
        &self,
        path: &str,
        query: &[(&str, String)],
    ) -> Result<T, ApiError> {
        Self::decode(self.execute(Method::GET, path, query, None)?)
    }

    pub fn post<T: DeserializeOwned, B: Serialize>(
        &self,
        path: &str,
        body: &B,
    ) -> Result<T, ApiError> {
        let v = serde_json::to_value(body).map_err(|e| ApiError::new(e.to_string()))?;
        Self::decode(self.execute(Method::POST, path, &[], Some(&v))?)
    }

    pub fn post_with_query<T: DeserializeOwned, B: Serialize>(
        &self,
        path: &str,
        query: &[(&str, String)],
        body: Option<&B>,
    ) -> Result<T, ApiError> {
        let v = body
            .map(|b| serde_json::to_value(b).map_err(|e| ApiError::new(e.to_string())))
            .transpose()?;
        Self::decode(self.execute(Method::POST, path, query, v.as_ref())?)
    }

    /// POST without body.
    pub fn post_empty<T: DeserializeOwned>(&self, path: &str) -> Result<T, ApiError> {
        Self::decode(self.execute(Method::POST, path, &[], None)?)
    }

    pub fn put<T: DeserializeOwned, B: Serialize>(
        &self,
        path: &str,
        body: &B,
    ) -> Result<T, ApiError> {
        let v = serde_json::to_value(body).map_err(|e| ApiError::new(e.to_string()))?;
        Self::decode(self.execute(Method::PUT, path, &[], Some(&v))?)
    }

    pub fn patch<T: DeserializeOwned, B: Serialize>(
        &self,
        path: &str,
        body: &B,
    ) -> Result<T, ApiError> {
        let v = serde_json::to_value(body).map_err(|e| ApiError::new(e.to_string()))?;
        Self::decode(self.execute(Method::PATCH, path, &[], Some(&v))?)
    }

    pub fn delete(&self, path: &str) -> Result<Response, ApiError> {
        self.execute(Method::DELETE, path, &[], None)
    }

    pub fn delete_with_query(
        &self,
        path: &str,
        query: &[(&str, String)],
    ) -> Result<Response, ApiError> {
        self.execute(Method::DELETE, path, query, None)
    }

    /// Fetch a `{"results": [...]}` list endpoint and return the results.
    pub fn list<T: DeserializeOwned>(&self, path: &str) -> Result<Vec<T>, ApiError> {
        let r: models::ResultList<T> = self.get(path)?;
        Ok(r.results)
    }
}

/// Go `log.Printf`: `2006/01/02 15:04:05 ` prefix (local time) on stderr, newline appended if missing.
pub fn go_log_printf(msg: &str) {
    let ts = chrono_like_now();
    let nl = if msg.ends_with('\n') { "" } else { "\n" };
    eprint!("{ts} {msg}{nl}");
}

fn chrono_like_now() -> String {
    // avoid a chrono dependency: use the `date` of the local time through libc-free formatting
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as i64;
    let offset = local_offset_seconds();
    let t = now + offset;
    let days = t.div_euclid(86_400);
    let secs = t.rem_euclid(86_400);
    let (y, m, d) = civil_from_days(days);
    format!(
        "{y:04}/{m:02}/{d:02} {:02}:{:02}:{:02}",
        secs / 3600,
        (secs % 3600) / 60,
        secs % 60
    )
}

fn local_offset_seconds() -> i64 {
    std::env::var("QOVERY_LOG_UTC_OFFSET")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or_else(qovery_log::local_utc_offset_seconds)
}

fn civil_from_days(z: i64) -> (i64, u32, u32) {
    let z = z + 719_468;
    let era = z.div_euclid(146_097);
    let doe = z.rem_euclid(146_097);
    let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = (doy - (153 * mp + 2) / 5 + 1) as u32;
    let m = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
    (if m <= 2 { y + 1 } else { y }, m, d)
}

/// Go canonical header key (`content-type` -> `Content-Type`).
fn canonical_header(k: &str) -> String {
    k.split('-')
        .map(|p| {
            let mut c = p.chars();
            match c.next() {
                Some(f) => f.to_uppercase().collect::<String>() + &c.as_str().to_lowercase(),
                None => String::new(),
            }
        })
        .collect::<Vec<_>>()
        .join("-")
}

/// Go `httputil.DumpResponse(res, true)` logged with `log.Printf("\n%s\n", dump)`.
fn dump_response(res: &Response) {
    let body = &res.body;
    let mut d = format!("HTTP/1.1 {}\r\n", res.status_line());
    let mut hs: Vec<(String, String)> = res
        .headers
        .iter()
        .map(|(k, v)| {
            (
                canonical_header(k.as_str()),
                v.to_str().unwrap_or("").to_string(),
            )
        })
        .collect();
    hs.sort_by(|a, b| a.0.cmp(&b.0));
    for (k, v) in hs {
        d.push_str(&format!("{k}: {v}\r\n"));
    }
    d.push_str("\r\n");
    d.push_str(&String::from_utf8_lossy(body));
    go_log_printf(&format!("\n{d}\n"));
}

/// Best effort translation of a transport error into Go's `net/http` wording.
fn go_transport_error(e: &reqwest::Error, url: &str) -> String {
    let parsed = url::Url::parse(url).ok();
    let host = parsed
        .as_ref()
        .and_then(|u| u.host_str().map(|h| h.to_string()))
        .unwrap_or_default();
    let port = parsed
        .as_ref()
        .and_then(|u| u.port_or_known_default())
        .unwrap_or(443);
    if e.is_timeout() {
        return "context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
            .to_string();
    }
    let mut source: Option<&(dyn std::error::Error + 'static)> = std::error::Error::source(e);
    while let Some(s) = source {
        if let Some(io) = s.downcast_ref::<std::io::Error>() {
            match io.kind() {
                std::io::ErrorKind::ConnectionRefused => {
                    return format!("dial tcp {host}:{port}: connect: connection refused");
                }
                std::io::ErrorKind::ConnectionReset => {
                    return format!("read tcp {host}:{port}: read: connection reset by peer");
                }
                _ => {}
            }
            let msg = io.to_string();
            if msg.contains("failed to lookup address")
                || msg.contains("Name or service not known")
                || msg.contains("No address associated")
            {
                return format!("dial tcp: lookup {host}: no such host");
            }
        }
        let msg = s.to_string();
        if msg.contains("dns error") || msg.contains("failed to lookup address") {
            return format!("dial tcp: lookup {host}: no such host");
        }
        source = s.source();
    }
    e.to_string()
}

#[cfg(test)]
mod tests {
    use super::remote_dns_proxies;

    fn proxies(vars: &[(&str, &str)]) -> Option<[String; 3]> {
        remote_dns_proxies(|n| {
            vars.iter()
                .find(|(k, _)| *k == n)
                .map(|(_, v)| v.to_string())
        })
    }

    #[test]
    fn socks5_proxies_resolve_names_remotely() {
        assert_eq!(
            proxies(&[("HTTPS_PROXY", "socks5://127.0.0.1:1080")]),
            Some(["socks5h://127.0.0.1:1080".into(), "".into(), "".into()])
        );
        assert_eq!(
            proxies(&[
                ("https_proxy", "SOCKS5://u:p@proxy:1080"),
                ("HTTP_PROXY", "http://proxy:3128"),
                ("all_proxy", "socks5://all:1080"),
            ]),
            Some([
                "socks5h://u:p@proxy:1080".into(),
                "http://proxy:3128".into(),
                "socks5h://all:1080".into()
            ])
        );
    }

    #[test]
    fn other_proxies_are_left_to_reqwest() {
        assert_eq!(proxies(&[]), None);
        assert_eq!(proxies(&[("HTTPS_PROXY", "http://proxy:3128")]), None);
        assert_eq!(
            proxies(&[("HTTPS_PROXY", "socks5h://127.0.0.1:1080")]),
            None
        );
        // an empty upper-case variable hides the lower-case one, as in reqwest
        assert_eq!(
            proxies(&[("HTTPS_PROXY", ""), ("https_proxy", "socks5://p:1080")]),
            None
        );
        assert_eq!(
            proxies(&[
                ("REQUEST_METHOD", "GET"),
                ("HTTPS_PROXY", "socks5://127.0.0.1:1080")
            ]),
            None
        );
    }
}
