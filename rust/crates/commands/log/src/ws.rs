//! Websocket helpers shared by the commands talking to the Qovery websocket gateway (`log`, `shell`,
//! `port-forward`): a gorilla/websocket-like client on top of tokio-tungstenite, and the port of
//! `pkg/wserror.go`.
//!
//! Errors are formatted like gorilla's, e.g. `websocket: bad handshake` for a failed upgrade and
//! `websocket: close 1008 (policy violation): denied` for a close frame received from the server.

use futures_util::{SinkExt, StreamExt};
use std::fmt;
use tokio::net::TcpStream;
use tokio_tungstenite::tungstenite::{self, Message, client::IntoClientRequest, http::HeaderValue};
use tokio_tungstenite::{MaybeTlsStream, WebSocketStream};

pub use tokio_tungstenite::tungstenite as tungstenite_crate;

/// A connected websocket.
pub type WsStream = WebSocketStream<MaybeTlsStream<TcpStream>>;

pub const CLOSE_NORMAL_CLOSURE: u16 = 1000;
pub const CLOSE_GOING_AWAY: u16 = 1001;
pub const CLOSE_NO_STATUS_RECEIVED: u16 = 1005;
pub const CLOSE_ABNORMAL_CLOSURE: u16 = 1006;
pub const CLOSE_INVALID_FRAME_PAYLOAD_DATA: u16 = 1007;
pub const CLOSE_POLICY_VIOLATION: u16 = 1008;
pub const CLOSE_INTERNAL_SERVER_ERR: u16 = 1011;

/// gorilla `*websocket.CloseError`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CloseError {
    pub code: u16,
    pub text: String,
}

impl fmt::Display for CloseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "websocket: close {}", self.code)?;
        let desc = match self.code {
            1000 => " (normal)",
            1001 => " (going away)",
            1002 => " (protocol error)",
            1003 => " (unsupported data)",
            1005 => " (no status)",
            1006 => " (abnormal closure)",
            1007 => " (invalid payload data)",
            1008 => " (policy violation)",
            1009 => " (message too big)",
            1010 => " (mandatory extension missing)",
            1011 => " (internal server error)",
            1015 => " (TLS handshake error)",
            _ => "",
        };
        f.write_str(desc)?;
        if !self.text.is_empty() {
            write!(f, ": {}", self.text)?;
        }
        Ok(())
    }
}

impl std::error::Error for CloseError {}

/// Error of [`read_message`]: a close (frame received, or connection dropped) or any other error.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ReadError {
    Close(CloseError),
    Other(String),
}

impl fmt::Display for ReadError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ReadError::Close(c) => c.fmt(f),
            ReadError::Other(s) => f.write_str(s),
        }
    }
}

impl std::error::Error for ReadError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            ReadError::Close(c) => Some(c),
            ReadError::Other(_) => None,
        }
    }
}

/// Go `errors.As(err, &closeErr)`: the [`CloseError`] in the error chain, if any.
pub fn as_close_error<'a>(err: &'a (dyn std::error::Error + 'static)) -> Option<&'a CloseError> {
    let mut cur: Option<&(dyn std::error::Error + 'static)> = Some(err);
    while let Some(e) = cur {
        if let Some(c) = e.downcast_ref::<CloseError>() {
            return Some(c);
        }
        if let Some(ReadError::Close(c)) = e.downcast_ref::<ReadError>() {
            return Some(c);
        }
        cur = e.source();
    }
    None
}

/// `pkg.IsPermanentCloseError`: true if the websocket close error should NOT be retried
/// (permission denied, auth/policy violation).
pub fn is_permanent_close_error(err: &(dyn std::error::Error + 'static)) -> bool {
    matches!(
        as_close_error(err).map(|c| c.code),
        Some(CLOSE_INVALID_FRAME_PAYLOAD_DATA) | Some(CLOSE_POLICY_VIOLATION)
    )
}

/// `pkg.IsInternalServerError`: true if the websocket close error is code 1011 (Internal Error).
pub fn is_internal_server_error(err: &(dyn std::error::Error + 'static)) -> bool {
    as_close_error(err).is_some_and(|c| c.code == CLOSE_INTERNAL_SERVER_ERR)
}

/// `pkg.IsAgentResponseTimeout`: close code 1011 with a message telling that the shell agent or the
/// gateway timed out (transient). A strict subset of [`is_internal_server_error`]: check it first.
pub fn is_agent_response_timeout(err: &(dyn std::error::Error + 'static)) -> bool {
    let Some(c) = as_close_error(err) else {
        return false;
    };
    if c.code != CLOSE_INTERNAL_SERVER_ERR {
        return false;
    }
    [
        "exceeded for receiving agent response",
        "while connecting to pod",
        "while setting up port forward",
        "Retry budget exhausted",
    ]
    .iter()
    .any(|s| c.text.contains(s))
}

/// `pkg.ServiceUnavailableMessage`.
pub fn service_unavailable_message(feature: &str) -> String {
    format!(
        "{feature} is not available. Please verify that the cluster hosting this service is running and healthy."
    )
}

/// A current-thread tokio runtime to drive the websocket from synchronous command code.
pub fn runtime() -> tokio::runtime::Runtime {
    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .expect("cannot build tokio runtime")
}

/// rustls needs a process-wide crypto provider when several are compiled in.
fn ensure_crypto_provider() {
    if rustls::crypto::CryptoProvider::get_default().is_none() {
        let _ = rustls::crypto::ring::default_provider().install_default();
    }
}

fn host_port(url: &str) -> String {
    match url::Url::parse(url) {
        Ok(u) => {
            let host = u.host_str().unwrap_or_default().to_string();
            let port = u.port_or_known_default().unwrap_or(0);
            if host.contains(':') {
                format!("[{host}]:{port}")
            } else {
                format!("{host}:{port}")
            }
        }
        Err(_) => String::new(),
    }
}

/// gorilla-like message of a dial error.
pub fn dial_error_message(url: &str, e: &tungstenite::Error) -> String {
    match e {
        tungstenite::Error::Http(_) => "websocket: bad handshake".to_string(),
        tungstenite::Error::Url(_) => "malformed ws or wss URL".to_string(),
        tungstenite::Error::Io(io) if io.get_ref().is_some_and(|e| e.is::<rustls::Error>()) => {
            let tls = io.get_ref().and_then(|e| e.downcast_ref::<rustls::Error>());
            match tls {
                Some(rustls::Error::InvalidMessage(rustls::InvalidMessage::InvalidContentType)) => {
                    "tls: first record does not look like a TLS handshake".to_string()
                }
                Some(rustls::Error::InvalidCertificate(rustls::CertificateError::UnknownIssuer)) => {
                    "tls: failed to verify certificate: x509: certificate signed by unknown authority".to_string()
                }
                Some(other) => format!("tls: {other}"),
                None => io.to_string(),
            }
        }
        tungstenite::Error::Io(io) => {
            let reason = match io.kind() {
                std::io::ErrorKind::ConnectionRefused => "connect: connection refused".to_string(),
                std::io::ErrorKind::TimedOut => "i/o timeout".to_string(),
                _ => {
                    let s = io.to_string();
                    match s.find(" (os error") {
                        Some(i) => s[..i].to_lowercase(),
                        None => s,
                    }
                }
            };
            format!("dial tcp {}: {reason}", host_port(url))
        }
        other => other.to_string(),
    }
}

/// HTTP headers of the handshake response.
pub type Headers = tungstenite::http::HeaderMap;

/// `websocket.DefaultDialer.Dial(url, headers)`.
pub async fn dial(url: &str, headers: &[(&str, String)]) -> Result<WsStream, String> {
    dial_with_response(url, headers).await.1
}

/// `websocket.DefaultDialer.Dial(url, headers)` also returning the response headers when the server answered
/// the upgrade request (gorilla returns the response for a successful upgrade and for a `bad handshake`).
pub async fn dial_with_response(
    url: &str,
    headers: &[(&str, String)],
) -> (Option<Headers>, Result<WsStream, String>) {
    ensure_crypto_provider();
    // gorilla only accepts ws:// and wss:// URLs, without user info
    match url::Url::parse(url) {
        Ok(u)
            if (u.scheme() == "ws" || u.scheme() == "wss")
                && u.username().is_empty()
                && u.password().is_none() => {}
        _ => return (None, Err("malformed ws or wss URL".to_string())),
    }
    let mut request = match url.into_client_request() {
        Ok(r) => r,
        Err(e) => return (None, Err(dial_error_message(url, &e))),
    };
    for (k, v) in headers {
        let value = match HeaderValue::from_str(v) {
            Ok(v) => v,
            Err(e) => return (None, Err(e.to_string())),
        };
        let name = match tungstenite::http::HeaderName::from_bytes(k.as_bytes()) {
            Ok(n) => n,
            Err(e) => return (None, Err(e.to_string())),
        };
        request.headers_mut().insert(name, value);
    }
    let connect = tokio_tungstenite::connect_async(request);
    match tokio::time::timeout(std::time::Duration::from_secs(45), connect).await {
        Ok(Ok((ws, response))) => (Some(response.headers().clone()), Ok(ws)),
        Ok(Err(tungstenite::Error::Http(response))) => (
            Some(response.headers().clone()),
            Err("websocket: bad handshake".to_string()),
        ),
        Ok(Err(e)) => (None, Err(dial_error_message(url, &e))),
        Err(_) => (None, Err("websocket: handshake timeout".to_string())),
    }
}

fn close_error(code: u16, text: impl Into<String>) -> ReadError {
    ReadError::Close(CloseError {
        code,
        text: text.into(),
    })
}

/// gorilla `Conn.ReadMessage()`: the next text or binary message (control frames are handled).
/// Returns `(is_text, payload)`.
pub async fn read_message(ws: &mut WsStream) -> Result<(bool, Vec<u8>), ReadError> {
    loop {
        match ws.next().await {
            None => return Err(close_error(CLOSE_ABNORMAL_CLOSURE, "unexpected EOF")),
            Some(Ok(Message::Text(t))) => return Ok((true, t.into_bytes())),
            Some(Ok(Message::Binary(b))) => return Ok((false, b)),
            Some(Ok(Message::Close(frame))) => {
                return Err(match frame {
                    Some(f) => close_error(u16::from(f.code), f.reason.to_string()),
                    None => close_error(CLOSE_NO_STATUS_RECEIVED, ""),
                });
            }
            Some(Ok(_)) => continue, // ping / pong / raw frames
            Some(Err(e)) => return Err(read_error(e)),
        }
    }
}

fn read_error(e: tungstenite::Error) -> ReadError {
    use tungstenite::error::ProtocolError;
    match e {
        tungstenite::Error::ConnectionClosed | tungstenite::Error::AlreadyClosed => {
            close_error(CLOSE_ABNORMAL_CLOSURE, "unexpected EOF")
        }
        tungstenite::Error::Protocol(ProtocolError::ResetWithoutClosingHandshake) => {
            close_error(CLOSE_ABNORMAL_CLOSURE, "unexpected EOF")
        }
        tungstenite::Error::Io(io) if io.kind() == std::io::ErrorKind::UnexpectedEof => {
            close_error(CLOSE_ABNORMAL_CLOSURE, "unexpected EOF")
        }
        other => ReadError::Other(other.to_string()),
    }
}

/// gorilla `Conn.WriteMessage(TextMessage | BinaryMessage, data)`.
pub async fn write_message(ws: &mut WsStream, text: bool, data: Vec<u8>) -> Result<(), String> {
    let msg = if text {
        Message::Text(String::from_utf8_lossy(&data).to_string())
    } else {
        Message::Binary(data)
    };
    ws.send(msg).await.map_err(|e| e.to_string())
}

/// gorilla `Conn.Close()` (closes the underlying connection without a close handshake).
pub async fn close(ws: WsStream) -> Result<(), String> {
    drop(ws);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn close(code: u16, text: &str) -> ReadError {
        close_error(code, text)
    }

    #[test]
    fn close_error_display() {
        assert_eq!(
            close(1000, "").to_string(),
            "websocket: close 1000 (normal)"
        );
        assert_eq!(
            close(1008, "denied").to_string(),
            "websocket: close 1008 (policy violation): denied"
        );
        assert_eq!(close(4000, "x").to_string(), "websocket: close 4000: x");
    }

    // port of pkg/wserror_test.go
    #[test]
    fn is_permanent_close_error_cases() {
        let cases: Vec<(ReadError, bool)> = vec![
            (close(1007, "permission denied"), true),
            (close(1008, "invalid token"), true),
            (close(1006, ""), false),
            (close(1001, ""), false),
            (close(1011, ""), false),
            (close(1000, ""), false),
            (
                ReadError::Other("dial tcp: connection refused".into()),
                false,
            ),
        ];
        for (err, expected) in cases {
            assert_eq!(is_permanent_close_error(&err), expected, "{err}");
        }
    }

    #[test]
    fn is_internal_server_error_cases() {
        let cases: Vec<(ReadError, bool)> = vec![
            (close(1011, "internal error"), true),
            (close(1007, ""), false),
            (close(1008, ""), false),
            (close(1006, ""), false),
            (ReadError::Other("some error".into()), false),
        ];
        for (err, expected) in cases {
            assert_eq!(is_internal_server_error(&err), expected, "{err}");
        }
    }

    #[test]
    fn is_agent_response_timeout_cases() {
        let cases: Vec<(ReadError, bool)> = vec![
            (
                close(
                    1011,
                    "Deadline of 90s exceeded for receiving agent response",
                ),
                true,
            ),
            (
                close(1011, "Timed out after 45s while connecting to pod"),
                true,
            ),
            (
                close(1011, "Timed out after 45s while setting up port forward"),
                true,
            ),
            (
                close(
                    1011,
                    "Retry budget exhausted: only 2s remaining, need at least 45s for K8s exec setup",
                ),
                true,
            ),
            (
                close(
                    1011,
                    "Retry budget exhausted: only 1s remaining, need at least 45s for K8s port-forward setup",
                ),
                true,
            ),
            (close(1011, "some other internal error"), false),
            (close(1007, "exceeded for receiving agent response"), false),
            (ReadError::Other("plain network error".into()), false),
        ];
        for (err, expected) in cases {
            assert_eq!(is_agent_response_timeout(&err), expected, "{err}");
        }
        // a timeout is always an internal server error
        assert!(is_internal_server_error(&close(
            1011,
            "Retry budget exhausted"
        )));
    }

    #[test]
    fn wrapped_close_error() {
        #[derive(Debug)]
        struct Wrapper(CloseError);
        impl fmt::Display for Wrapper {
            fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                write!(f, "wrapped: {}", self.0)
            }
        }
        impl std::error::Error for Wrapper {
            fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
                Some(&self.0)
            }
        }
        let w = Wrapper(CloseError {
            code: 1008,
            text: "x".into(),
        });
        assert!(is_permanent_close_error(&w));
    }

    #[test]
    fn service_unavailable() {
        assert_eq!(
            service_unavailable_message("Shell"),
            "Shell is not available. Please verify that the cluster hosting this service is running and healthy."
        );
    }
}
