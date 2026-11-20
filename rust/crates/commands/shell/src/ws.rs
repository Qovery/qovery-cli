//! gorilla/websocket-like client pieces used by `shell`, `port-forward` and `list-pods`, on top of the
//! helpers of `qovery_cmd_log::ws` (port of `pkg/wserror.go`, gorilla error formatting).
//!
//! [`dial`] also returns the handshake response headers (`qovery_cmd_log::ws::dial_with_response`), and [`next_frame`] works on the read half
//! of a split connection and reports pongs (the shell resets its read deadline on each pong).

use futures_util::StreamExt;
use futures_util::stream::{SplitSink, SplitStream};
use qovery_cmd_log::ws::{
    CLOSE_ABNORMAL_CLOSURE, CLOSE_NO_STATUS_RECEIVED, CloseError, ReadError, WsStream,
};
use tokio_tungstenite::MaybeTlsStream;
use tokio_tungstenite::tungstenite::{self, Message};

pub use qovery_cmd_log::ws::{
    is_agent_response_timeout, is_internal_server_error, is_permanent_close_error, runtime,
    service_unavailable_message,
};

/// Write half of a connection.
pub type WsSink = SplitSink<WsStream, Message>;
/// Read half of a connection.
pub type WsSource = SplitStream<WsStream>;

/// HTTP headers of the handshake response.
pub use qovery_cmd_log::ws::Headers;

/// `websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {authorization}})`: the response headers
/// when the server answered the upgrade request (successfully or not), and the connection or gorilla's error.
pub async fn dial(url: &str, authorization: &str) -> (Option<Headers>, Result<WsStream, String>) {
    qovery_cmd_log::ws::dial_with_response(url, &[("Authorization", authorization.to_string())])
        .await
}

/// `resp.Header.Get(name)`.
pub fn header(headers: &Headers, name: &str) -> String {
    headers
        .get(name)
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default()
        .to_string()
}

/// `local->peer` addresses of the TCP connection, as printed in Go `*net.OpError`s.
pub fn addrs(ws: &WsStream) -> (String, String) {
    let tcp = match ws.get_ref() {
        MaybeTlsStream::Plain(s) => Some(s),
        MaybeTlsStream::Rustls(s) => Some(s.get_ref().0),
        _ => None,
    };
    let fmt =
        |a: std::io::Result<std::net::SocketAddr>| a.map(|a| a.to_string()).unwrap_or_default();
    match tcp {
        Some(s) => (fmt(s.local_addr()), fmt(s.peer_addr())),
        None => (String::new(), String::new()),
    }
}

/// A data message, or a pong (control frames are otherwise handled by tungstenite, like gorilla's handlers).
#[derive(Debug)]
pub enum Frame {
    Text(Vec<u8>),
    Binary(Vec<u8>),
    Pong,
}

fn close_error(code: u16, text: impl Into<String>) -> ReadError {
    ReadError::Close(CloseError {
        code,
        text: text.into(),
    })
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
        tungstenite::Error::Io(io)
            if matches!(
                io.kind(),
                std::io::ErrorKind::UnexpectedEof
                    | std::io::ErrorKind::ConnectionReset
                    | std::io::ErrorKind::BrokenPipe
            ) =>
        {
            close_error(CLOSE_ABNORMAL_CLOSURE, "unexpected EOF")
        }
        other => ReadError::Other(other.to_string()),
    }
}

/// gorilla `Conn.ReadMessage()` on the read half (plus pongs).
pub async fn next_frame(source: &mut WsSource) -> Result<Frame, ReadError> {
    loop {
        match source.next().await {
            None => return Err(close_error(CLOSE_ABNORMAL_CLOSURE, "unexpected EOF")),
            Some(Ok(Message::Text(t))) => return Ok(Frame::Text(t.into_bytes())),
            Some(Ok(Message::Binary(b))) => return Ok(Frame::Binary(b)),
            Some(Ok(Message::Pong(_))) => return Ok(Frame::Pong),
            Some(Ok(Message::Close(frame))) => {
                return Err(match frame {
                    Some(f) => close_error(u16::from(f.code), f.reason.to_string()),
                    None => close_error(CLOSE_NO_STATUS_RECEIVED, ""),
                });
            }
            Some(Ok(_)) => continue,
            Some(Err(e)) => return Err(read_error(e)),
        }
    }
}

/// gorilla `Conn.ReadMessage()` skipping pongs: `(is_text, payload)`.
pub async fn read_message(source: &mut WsSource) -> Result<(bool, Vec<u8>), ReadError> {
    loop {
        match next_frame(source).await? {
            Frame::Text(t) => return Ok((true, t)),
            Frame::Binary(b) => return Ok((false, b)),
            Frame::Pong => continue,
        }
    }
}

/// The error of a read on a connection closed locally (Go `net.ErrClosed` wrapped in a `*net.OpError`).
pub fn closed_connection_error(local: &str, peer: &str) -> String {
    format!("read tcp {local}->{peer}: use of closed network connection")
}

/// The error of a read that hit its deadline.
pub fn read_timeout_error(local: &str, peer: &str) -> String {
    format!("read tcp {local}->{peer}: i/o timeout")
}

pub use qovery_core::auth::authorization;

/// [`authorization`] from async code (the token retrieval uses the blocking HTTP client).
pub async fn authorization_async() -> Result<String, String> {
    tokio::task::spawn_blocking(authorization)
        .await
        .unwrap_or_else(|e| Err(e.to_string()))
}
