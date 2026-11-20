//! Websocket helpers with gorilla/websocket compatible errors, and a local port of `pkg.ExecShell`
//! (pkg/shell.go, pkg/wserror.go) used by `cluster debug-pod`.
//!
//! `pkg.ExecShell` is shared with the `shell` command: this is a private copy (to be moved to a shared crate).

use futures_util::{SinkExt, StreamExt};
use is_terminal::IsTerminal;
use std::io::{Read, Write};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;
use tokio_tungstenite::tungstenite::{
    self, Message, client::IntoClientRequest, protocol::frame::coding::CloseCode,
};

pub type WsStream =
    tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>;

const STDIN_BUFFER_SIZE: usize = 4096;
const RECONNECT_DELAY: Duration = Duration::from_secs(5);
const PING_INTERVAL: Duration = Duration::from_secs(30);
const READ_TIMEOUT: Duration = Duration::from_secs(75);

/// The websocket url of `path` with an already encoded query (`utils.WebsocketUrl() + path`).
pub fn ws_url(path: &str, raw_query: &str) -> String {
    let base = qovery_core::client::websocket_url();
    if raw_query.is_empty() {
        format!("{base}{path}")
    } else {
        format!("{base}{path}?{raw_query}")
    }
}

/// Go dial error (`dial tcp host:port: connect: connection refused`).
fn go_dial_error(url: &str, e: &std::io::Error) -> String {
    let (host, port) = match url::Url::parse(url) {
        Ok(u) => {
            let host = u.host_str().unwrap_or_default().to_string();
            let port = u.port_or_known_default().unwrap_or(80);
            let host = if host.contains(':') && !host.starts_with('[') {
                format!("[{host}]")
            } else {
                host
            };
            (host, port)
        }
        Err(_) => (String::new(), 0),
    };
    let reason = match e.kind() {
        std::io::ErrorKind::ConnectionRefused => "connect: connection refused".to_string(),
        std::io::ErrorKind::TimedOut => "i/o timeout".to_string(),
        _ => {
            let s = e.to_string();
            match s.find(" (os error") {
                Some(i) => s[..i].to_lowercase(),
                None => s,
            }
        }
    };
    format!("dial tcp {host}:{port}: {reason}")
}

/// Result of a websocket dial: the connection, or the error with the HTTP response headers when one was received.
pub struct DialError {
    pub message: String,
    pub request_id: Option<String>,
}

/// `websocket.DefaultDialer.Dial(url, {"Authorization": ...})`.
pub async fn dial(url: &str, authorization: &str) -> Result<(WsStream, Option<String>), DialError> {
    let mut request = url.into_client_request().map_err(|e| DialError {
        message: e.to_string(),
        request_id: None,
    })?;
    if let Ok(v) = authorization.parse() {
        request.headers_mut().insert("Authorization", v);
    }
    match tokio_tungstenite::connect_async(request).await {
        Ok((ws, resp)) => {
            let rid = resp
                .headers()
                .get("X-Qovery-Request-Id")
                .and_then(|v| v.to_str().ok())
                .unwrap_or_default()
                .to_string();
            Ok((ws, Some(rid)))
        }
        Err(tungstenite::Error::Http(resp)) => {
            let rid = resp
                .headers()
                .get("X-Qovery-Request-Id")
                .and_then(|v| v.to_str().ok())
                .unwrap_or_default()
                .to_string();
            Err(DialError {
                message: "websocket: bad handshake".to_string(),
                request_id: Some(rid),
            })
        }
        Err(tungstenite::Error::Io(e)) => Err(DialError {
            message: go_dial_error(url, &e),
            request_id: None,
        }),
        Err(e) => Err(DialError {
            message: e.to_string(),
            request_id: None,
        }),
    }
}

/// gorilla `CloseError`.
#[derive(Debug, Clone)]
pub struct CloseError {
    pub code: u16,
    pub text: String,
}

impl std::fmt::Display for CloseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let name = match self.code {
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
        write!(f, "websocket: close {}{}", self.code, name)?;
        if !self.text.is_empty() {
            write!(f, ": {}", self.text)?;
        }
        Ok(())
    }
}

/// A message read like gorilla's `ReadMessage`: close frames are errors.
pub enum ReadResult {
    Text(Vec<u8>),
    Binary(Vec<u8>),
    Close(CloseError),
    Err(String),
}

/// Read the next data message (ping/pong are handled by tungstenite).
pub async fn read_message(ws: &mut WsStream) -> ReadResult {
    loop {
        match ws.next().await {
            None => {
                return ReadResult::Close(CloseError {
                    code: 1006,
                    text: "unexpected EOF".to_string(),
                });
            }
            Some(Ok(Message::Text(t))) => return ReadResult::Text(t.as_bytes().to_vec()),
            Some(Ok(Message::Binary(b))) => return ReadResult::Binary(b.to_vec()),
            Some(Ok(Message::Close(frame))) => {
                return ReadResult::Close(match frame {
                    Some(f) => CloseError {
                        code: u16::from(f.code),
                        text: f.reason.to_string(),
                    },
                    None => CloseError {
                        code: u16::from(CloseCode::Status),
                        text: String::new(),
                    },
                });
            }
            Some(Ok(_)) => continue,
            Some(Err(tungstenite::Error::ConnectionClosed))
            | Some(Err(tungstenite::Error::AlreadyClosed)) => {
                return ReadResult::Close(CloseError {
                    code: 1006,
                    text: String::new(),
                });
            }
            Some(Err(tungstenite::Error::Protocol(
                tungstenite::error::ProtocolError::ResetWithoutClosingHandshake,
            ))) => {
                return ReadResult::Err("unexpected EOF".to_string());
            }
            Some(Err(e)) => return ReadResult::Err(e.to_string()),
        }
    }
}

/// `IsAgentResponseTimeout`.
pub fn is_agent_response_timeout(e: &CloseError) -> bool {
    e.code == 1011
        && (e.text.contains("exceeded for receiving agent response")
            || e.text.contains("while connecting to pod")
            || e.text.contains("while setting up port forward")
            || e.text.contains("Retry budget exhausted"))
}

/// `ServiceUnavailableMessage(feature)`.
pub fn service_unavailable_message(feature: &str) -> String {
    format!(
        "{feature} is not available. Please verify that the cluster hosting this service is running and healthy."
    )
}

/// `isPotentialBracketedPastePrefix`.
fn is_potential_bracketed_paste_prefix(data: &[u8]) -> bool {
    const START: [u8; 6] = [0x1b, b'[', b'2', b'0', b'0', b'~'];
    const END: [u8; 6] = [0x1b, b'[', b'2', b'0', b'1', b'~'];
    if data.is_empty() || data.len() >= 6 {
        return false;
    }
    data.iter().enumerate().all(|(i, b)| *b == START[i])
        || data.iter().enumerate().all(|(i, b)| *b == END[i])
}

/// `handleBracketedPasteFragmentation`: (data to send, pending bytes of an incomplete sequence).
pub fn handle_bracketed_paste_fragmentation(data: &[u8]) -> (Vec<u8>, Vec<u8>) {
    if data.is_empty() {
        return (Vec::new(), Vec::new());
    }
    for check_len in 1..6 {
        if check_len > data.len() {
            break;
        }
        let tail = &data[data.len() - check_len..];
        if is_potential_bracketed_paste_prefix(tail) {
            return (data[..data.len() - check_len].to_vec(), tail.to_vec());
        }
    }
    (data.to_vec(), Vec::new())
}

/// Terminal raw mode, restored on drop (`console.Current().SetRaw()` / `Reset()`).
struct RawMode;

impl Drop for RawMode {
    fn drop(&mut self) {
        let _ = crossterm::terminal::disable_raw_mode();
    }
}

/// Port of `pkg.ExecShell(req, path)`: `query` builds the encoded query string from the tty size.
pub fn exec_shell(query: &dyn Fn(u16, u16) -> String, path: &str) {
    let rt = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime");
    rt.block_on(exec_shell_async(query, path));
    rt.shutdown_background();
}

async fn exec_shell_async(query: &dyn Fn(u16, u16) -> String, path: &str) {
    let cancelled = Arc::new(AtomicBool::new(false));
    let user_cancelled = Arc::new(AtomicBool::new(false));
    let normal_exit = Arc::new(AtomicBool::new(false));

    {
        let user_cancelled = user_cancelled.clone();
        let cancelled = cancelled.clone();
        tokio::spawn(async move {
            #[cfg(unix)]
            if let Ok(mut sig) =
                tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            {
                sig.recv().await;
                user_cancelled.store(true, Ordering::SeqCst);
                cancelled.store(true, Ordering::SeqCst);
            }
            // Go delivers CTRL_CLOSE_EVENT as SIGTERM on Windows
            #[cfg(windows)]
            if let Ok(mut sig) = tokio::signal::windows::ctrl_close() {
                sig.recv().await;
                user_cancelled.store(true, Ordering::SeqCst);
                cancelled.store(true, Ordering::SeqCst);
            }
        });
    }

    let interactive = std::io::stdin().is_terminal();
    let (mut width, mut height) = (0u16, 0u16);
    let _raw = if interactive {
        if let Err(e) = crossterm::terminal::enable_raw_mode() {
            qovery_log::logrus::fatal(format!("error while setting up console{e}"));
        }
        match crossterm::terminal::size() {
            Ok((w, h)) => {
                width = w;
                height = h;
            }
            Err(e) => qovery_log::logrus::fatal(format!("Cannot get terminal size{e}")),
        }
        Some(RawMode)
    } else {
        None
    };

    // readUserConsole
    let (stdin_tx, mut stdin_rx) = tokio::sync::mpsc::channel::<Vec<u8>>(1);
    {
        let cancelled = cancelled.clone();
        let normal_exit = normal_exit.clone();
        std::thread::spawn(move || {
            let mut stdin = std::io::stdin();
            let mut buffer = vec![0u8; STDIN_BUFFER_SIZE];
            let mut pending: Vec<u8> = Vec::new();
            loop {
                if cancelled.load(Ordering::SeqCst) || normal_exit.load(Ordering::SeqCst) {
                    return;
                }
                match stdin.read(&mut buffer) {
                    Ok(0) if !interactive => {
                        if !pending.is_empty() && stdin_tx.blocking_send(pending.clone()).is_err() {
                            return;
                        }
                        let _ = stdin_tx.blocking_send(vec![0x04]);
                        return;
                    }
                    Ok(0) => {
                        qovery_log::logrus::error("error while reading on console:EOF");
                        cancelled.store(true, Ordering::SeqCst);
                        return;
                    }
                    Ok(n) => {
                        let mut data = std::mem::take(&mut pending);
                        data.extend_from_slice(&buffer[..n]);
                        let (to_send, rest) = handle_bracketed_paste_fragmentation(&data);
                        pending = rest;
                        if !to_send.is_empty() && stdin_tx.blocking_send(to_send).is_err() {
                            return;
                        }
                    }
                    Err(e) => {
                        qovery_log::logrus::error(format!("error while reading on console:{e}"));
                        cancelled.store(true, Ordering::SeqCst);
                        return;
                    }
                }
            }
        });
    }

    let is_done = |c: &AtomicBool, u: &AtomicBool, n: &AtomicBool| {
        c.load(Ordering::SeqCst) || u.load(Ordering::SeqCst) || n.load(Ordering::SeqCst)
    };

    loop {
        if is_done(&cancelled, &user_cancelled, &normal_exit) {
            qovery_log::logrus::info("Shell exited, not reconnecting.");
            break;
        }
        qovery_log::logrus::info("Attempting to (re)connect");

        let token = qovery_core::auth::get_access_token(false);
        let dial_result = match token {
            Err(e) => Err(DialError {
                message: e,
                request_id: None,
            }),
            Ok((tt, tok)) => {
                let url = ws_url(path, &query(width, height));
                dial(
                    &url,
                    &qovery_core::auth::authorization_header_value(tt, &tok),
                )
                .await
            }
        };
        let (ws, request_id) = match dial_result {
            Ok((ws, rid)) => {
                let rid = rid.unwrap_or_default();
                qovery_log::logrus::info(format!("Connected to shell with requestId: {rid}"));
                (ws, rid)
            }
            Err(e) => {
                let rid = e.request_id.clone().unwrap_or_default();
                if e.request_id.is_some() {
                    qovery_log::logrus::info(format!("Connected to shell with requestId: {rid}"));
                }
                qovery_log::logrus::error(format!(
                    "WebSocket connection failed: {} {}",
                    e.message, rid
                ));
                if is_done(&cancelled, &user_cancelled, &normal_exit) {
                    qovery_log::logrus::info(
                        "User cancelled or shell exited during connection attempt.",
                    );
                    break;
                }
                tokio::time::sleep(RECONNECT_DELAY).await;
                continue;
            }
        };

        let (mut sink, mut stream) = ws.split();
        let (done_tx, mut done_rx) = tokio::sync::oneshot::channel::<()>();
        let reader = {
            let normal_exit = normal_exit.clone();
            let cancelled = cancelled.clone();
            let request_id = request_id.clone();
            tokio::spawn(async move {
                let _done = done_tx;
                let mut out = std::io::stdout();
                loop {
                    let next = match tokio::time::timeout(READ_TIMEOUT, stream.next()).await {
                        Ok(n) => n,
                        Err(_) => {
                            qovery_log::logrus::error(format!(
                                "error while reading on websocket {request_id}: i/o timeout  "
                            ));
                            return;
                        }
                    };
                    let close = match next {
                        Some(Ok(Message::Binary(b))) => {
                            if let Err(e) = out.write_all(&b).and_then(|_| out.flush()) {
                                qovery_log::logrus::error(format!(
                                    "error while writing in console: {e}"
                                ));
                                return;
                            }
                            continue;
                        }
                        Some(Ok(Message::Close(frame))) => match frame {
                            Some(f) => CloseError {
                                code: u16::from(f.code),
                                text: f.reason.to_string(),
                            },
                            None => CloseError {
                                code: 1005,
                                text: String::new(),
                            },
                        },
                        Some(Ok(_)) => continue,
                        None => CloseError {
                            code: 1006,
                            text: "unexpected EOF".to_string(),
                        },
                        Some(Err(e)) => {
                            qovery_log::logrus::error(format!(
                                "error while reading on websocket {request_id}: {e}  "
                            ));
                            return;
                        }
                    };
                    match close.code {
                        1000 => {
                            qovery_log::logrus::info("** shell terminated bye **");
                            normal_exit.store(true, Ordering::SeqCst);
                        }
                        1007 | 1008 => {
                            qovery_log::logrus::error(format!(
                                "Shell connection {request_id} rejected: check your permissions or run 'qovery auth'"
                            ));
                            cancelled.store(true, Ordering::SeqCst);
                        }
                        _ if is_agent_response_timeout(&close) => {
                            qovery_log::logrus::warn(format!(
                                "Shell session {request_id} timed out while the agent was preparing your connection. Retrying..."
                            ))
                        }
                        1011 => qovery_log::logrus::warn(format!(
                            "{} Closing {request_id} and Retrying...",
                            service_unavailable_message("Shell")
                        )),
                        _ => qovery_log::logrus::error(format!(
                            "connection {request_id} closed by server: {close}"
                        )),
                    }
                    return;
                }
            })
        };

        let mut ping = tokio::time::interval(PING_INTERVAL);
        ping.tick().await;
        loop {
            if is_done(&cancelled, &user_cancelled, &normal_exit) {
                break;
            }
            tokio::select! {
                _ = &mut done_rx => break,
                msg = stdin_rx.recv() => {
                    let Some(msg) = msg else {
                        // stdin reader finished: keep draining the remote output
                        let _ = (&mut done_rx).await;
                        break;
                    };
                    if let Err(e) = sink.send(Message::Binary(msg)).await {
                        qovery_log::logrus::error(format!("Write error:{e}"));
                        break;
                    }
                }
                _ = ping.tick() => {
                    if let Err(e) = sink.send(Message::Ping(Vec::new())).await {
                        qovery_log::logrus::error(format!("Ping error:{e}"));
                        break;
                    }
                }
                _ = tokio::time::sleep(Duration::from_millis(200)) => {}
            }
        }
        let _ = sink.close().await;
        reader.abort();

        if normal_exit.load(Ordering::SeqCst) || user_cancelled.load(Ordering::SeqCst) {
            cancelled.store(true, Ordering::SeqCst);
        }
        if !is_done(&cancelled, &user_cancelled, &normal_exit) {
            tokio::time::sleep(RECONNECT_DELAY).await;
        }
    }
    let _ = std::io::stdout().flush();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bracketed_paste_fragmentation() {
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"abc"),
            (b"abc".to_vec(), vec![])
        );
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"abc\x1b[20"),
            (b"abc".to_vec(), b"\x1b[20".to_vec())
        );
        assert_eq!(
            handle_bracketed_paste_fragmentation(b"\x1b[200~x"),
            (b"\x1b[200~x".to_vec(), vec![])
        );
        assert_eq!(handle_bracketed_paste_fragmentation(b""), (vec![], vec![]));
    }

    #[test]
    fn close_error_display() {
        assert_eq!(
            CloseError {
                code: 1000,
                text: String::new()
            }
            .to_string(),
            "websocket: close 1000 (normal)"
        );
        assert_eq!(
            CloseError {
                code: 4000,
                text: "x".into()
            }
            .to_string(),
            "websocket: close 4000: x"
        );
    }
}
