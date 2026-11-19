//! Port of `pkg.ExecShell` (`pkg/shell.go`): the shell session over the websocket gateway, reconnecting
//! every 5 seconds until the remote shell exits (close 1000), the gateway rejects the session (1007/1008),
//! stdin fails, or SIGTERM is received.
//!
//! * stdin is a terminal: raw mode, the terminal size is sent in the request, input and output go through the
//!   console (Go `console.Current()`: the first of stderr, stdout, stdin that is a terminal).
//! * otherwise (`kubectl exec -i` like): stdin / stdout are piped through, the size stays 0x0, and the end of
//!   stdin sends an EOT (0x04) so that the remote side gets EOF.
//!
//! Like Go, the terminal size is only sent when connecting (no resize handling).
//! Unlike Go, the end of the session does not wait for one more key press to stop the stdin reader.

use crate::ShellRequest;
use crate::console::{Console, ConsoleHandle};
use crate::paste::handle_bracketed_paste_fragmentation;
use crate::ws::{self, Frame, WsSource};
use futures_util::SinkExt;
use futures_util::StreamExt;
use is_terminal::IsTerminal;
use qovery_cmd_log::ws::{ReadError, is_agent_response_timeout, service_unavailable_message};
use qovery_core::context::go_path_error;
use qovery_log::logrus;
use std::io::{Read, Write};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;
use tokio::sync::{mpsc, oneshot, watch};
use tokio::time::Instant;
use tokio_tungstenite::tungstenite::Message;

const STDIN_BUFFER_SIZE: usize = 4096;
const RECONNECT_DELAY: Duration = Duration::from_secs(5);
const PING_INTERVAL: Duration = Duration::from_secs(30);
/// Must be > 2 x PING_INTERVAL so that a healthy connection always receives a pong before the deadline fires.
const READ_TIMEOUT: Duration = Duration::from_secs(75);

/// Shared session state: Go's `ctx`/`cancel`, `userCancelled` and `normalExit`.
struct State {
    cancel: watch::Sender<bool>,
    user_cancelled: AtomicBool,
    normal_exit: AtomicBool,
}

impl State {
    fn new() -> Self {
        State {
            cancel: watch::channel(false).0,
            user_cancelled: AtomicBool::new(false),
            normal_exit: AtomicBool::new(false),
        }
    }
    fn cancel(&self) {
        self.cancel.send_replace(true);
    }
    fn is_cancelled(&self) -> bool {
        *self.cancel.borrow()
    }
    async fn cancelled(&self) {
        let mut rx = self.cancel.subscribe();
        let _ = rx.wait_for(|c| *c).await;
    }
    fn user_cancelled(&self) -> bool {
        self.user_cancelled.load(Ordering::SeqCst)
    }
    fn normal_exit(&self) -> bool {
        self.normal_exit.load(Ordering::SeqCst)
    }
    fn done(&self) -> bool {
        self.is_cancelled() || self.user_cancelled() || self.normal_exit()
    }
}

/// Where the remote output goes / the user input comes from: stdout / stdin, or the console in interactive mode.
#[derive(Clone, Copy)]
enum Io {
    Std,
    Console(ConsoleHandle),
}

impl Io {
    fn write(&self, data: &[u8]) -> Result<(), String> {
        let path = |p: &str| std::path::PathBuf::from(p);
        match self {
            Io::Console(c) => c
                .write_all(data)
                .map_err(|e| go_path_error("write", &path(c.name()), &e)),
            Io::Std => {
                let mut out = std::io::stdout().lock();
                out.write_all(data)
                    .and_then(|_| out.flush())
                    .map_err(|e| go_path_error("write", &path("/dev/stdout"), &e))
            }
        }
    }

    fn name(&self) -> &'static str {
        match self {
            Io::Console(c) => c.name(),
            Io::Std => "/dev/stdin",
        }
    }

    fn read(&self, buf: &mut [u8]) -> std::io::Result<usize> {
        match self {
            Io::Console(c) => c.read(buf),
            Io::Std => std::io::stdin().lock().read(buf),
        }
    }
}

/// `pkg.ExecShell(req, path)`.
pub fn exec_shell(mut req: ShellRequest, path: &str) {
    // Allocate a PTY only when stdin is a real terminal; otherwise behave like `kubectl exec -i`.
    let interactive = std::io::stdin().is_terminal();

    let mut io = Io::Std;
    let mut console = None;
    if interactive {
        let Some(mut current) = Console::current() else {
            logrus::panic("provided file is not a console");
        };
        if let Err(e) = current.set_raw() {
            logrus::fatal(format!("error while setting up console{e}"));
        }
        match current.size() {
            Ok((width, height)) => req.set_tty_size(width, height),
            Err(e) => logrus::fatal(format!("Cannot get terminal size{e}")),
        }
        io = Io::Console(current.handle());
        console = Some(current);
    }

    let rt = ws::runtime();
    rt.block_on(run(req, path, interactive, io));
    // do not wait for the blocking stdin reader thread
    rt.shutdown_background();

    if let Some(c) = console {
        c.reset();
    }
}

async fn run(req: ShellRequest, path: &str, interactive: bool, io: Io) {
    let state = Arc::new(State::new());

    #[cfg(unix)]
    {
        use tokio::signal::unix::{SignalKind, signal};
        if let Ok(mut sigterm) = signal(SignalKind::terminate()) {
            let st = state.clone();
            tokio::spawn(async move {
                sigterm.recv().await;
                st.user_cancelled.store(true, Ordering::SeqCst);
                st.cancel();
            });
        }
    }

    let (stdin_tx, mut stdin_rx) = mpsc::channel::<Vec<u8>>(1);
    {
        let st = state.clone();
        std::thread::spawn(move || read_user_console(st, interactive, io, stdin_tx));
    }

    let url = crate::query::websocket_request_url(path, &req.query());

    loop {
        if state.done() {
            logrus::info("Shell exited, not reconnecting.");
            break;
        }

        logrus::info("Attempting to (re)connect");
        let mut request_id = String::new();

        let (headers, conn) = match ws::authorization_async().await {
            Ok(auth) => ws::dial(&url, &auth).await,
            Err(e) => (None, Err(e)),
        };
        if let Some(h) = &headers {
            request_id = ws::header(h, "X-Qovery-Request-Id");
            logrus::info(format!("Connected to shell with requestId: {request_id}"));
        }
        let conn = match conn {
            Ok(c) => c,
            Err(e) => {
                logrus::error(format!("WebSocket connection failed: {e} {request_id}"));
                if state.done() {
                    logrus::info("User cancelled or shell exited during connection attempt.");
                    break;
                }
                tokio::time::sleep(RECONNECT_DELAY).await;
                continue;
            }
        };

        let (local, peer) = ws::addrs(&conn);
        let (mut sink, source) = conn.split();
        let (close_tx, close_rx) = oneshot::channel::<()>();
        let mut reader = tokio::spawn(read_websocket_connection(
            state.clone(),
            source,
            request_id.clone(),
            io,
            close_rx,
            local,
            peer,
        ));
        let mut reader_finished = false;
        let mut ping = tokio::time::interval_at(Instant::now() + PING_INTERVAL, PING_INTERVAL);

        loop {
            tokio::select! {
                _ = state.cancelled() => break,
                _ = &mut reader => {
                    reader_finished = true;
                    break;
                }
                Some(msg) = stdin_rx.recv() => {
                    if let Err(e) = sink.send(Message::Binary(msg)).await {
                        logrus::error(format!("Write error:{e}"));
                        break;
                    }
                }
                _ = ping.tick() => {
                    if let Err(e) = sink.send(Message::Ping(Vec::new())).await {
                        logrus::error(format!("Ping error:{e}"));
                        break;
                    }
                }
            }
            if state.done() {
                break;
            }
        }

        // wsConn.Close(): a reader blocked on the connection fails with "use of closed network connection"
        if !reader_finished {
            let _ = close_tx.send(());
            let _ = reader.await;
        }
        drop(sink);

        if state.normal_exit() || state.user_cancelled() {
            state.cancel();
        }
        if !state.done() {
            tokio::time::sleep(RECONNECT_DELAY).await;
        }
    }
}

/// `readWebsocketConnection`.
async fn read_websocket_connection(
    state: Arc<State>,
    mut source: WsSource,
    request_id: String,
    out: Io,
    mut closed: oneshot::Receiver<()>,
    local: String,
    peer: String,
) {
    let mut deadline = Instant::now() + READ_TIMEOUT;
    loop {
        if state.is_cancelled() {
            return;
        }
        let frame = tokio::select! {
            biased;
            _ = &mut closed => {
                logrus::error(format!(
                    "error while reading on websocket {request_id}: {}  ",
                    ws::closed_connection_error(&local, &peer)
                ));
                return;
            }
            r = tokio::time::timeout_at(deadline, ws::next_frame(&mut source)) => r,
        };
        let frame = match frame {
            Err(_) => {
                logrus::error(format!(
                    "error while reading on websocket {request_id}: {}  ",
                    ws::read_timeout_error(&local, &peer)
                ));
                return;
            }
            Ok(f) => f,
        };
        match frame {
            Ok(Frame::Pong) => deadline = Instant::now() + READ_TIMEOUT,
            Ok(Frame::Text(_)) => continue,
            Ok(Frame::Binary(msg)) => {
                if let Err(e) = out.write(&msg) {
                    logrus::error(format!("error while writing in console: {e}"));
                    return;
                }
            }
            Err(ReadError::Other(e)) => {
                logrus::error(format!(
                    "error while reading on websocket {request_id}: {e}  "
                ));
                return;
            }
            Err(err @ ReadError::Close(_)) => {
                let ReadError::Close(c) = &err else {
                    unreachable!()
                };
                match c.code {
                    1000 => {
                        logrus::info("** shell terminated bye **");
                        state.normal_exit.store(true, Ordering::SeqCst);
                    }
                    1007 | 1008 => {
                        logrus::error(format!(
                            "Shell connection {request_id} rejected: check your permissions or run 'qovery auth'"
                        ));
                        state.cancel();
                    }
                    _ if is_agent_response_timeout(&err) => logrus::warn(format!(
                        "Shell session {request_id} timed out while the agent was preparing your connection. Retrying..."
                    )),
                    1011 => logrus::warn(format!(
                        "{} Closing {request_id} and Retrying...",
                        service_unavailable_message("Shell")
                    )),
                    _ => logrus::error(format!("connection {request_id} closed by server: {c}")),
                }
                return;
            }
        }
    }
}

/// `readUserConsole`: runs on its own thread (blocking reads).
fn read_user_console(
    state: Arc<State>,
    interactive: bool,
    input: Io,
    stdin: mpsc::Sender<Vec<u8>>,
) {
    let mut buffer = [0u8; STDIN_BUFFER_SIZE];
    // persistent buffer to handle fragmented bracketed paste sequences
    let mut pending: Vec<u8> = Vec::new();

    loop {
        if state.is_cancelled() || state.normal_exit() {
            return;
        }

        let count = match input.read(&mut buffer) {
            Ok(0) => {
                // In non-interactive mode, EOF means the input stream is exhausted but the remote command may
                // still produce output: flush the buffered bytes, then send EOT (0x04) so that the remote PTY
                // propagates EOF to the remote process. The websocket stays open to drain the remote output.
                if !interactive {
                    if !pending.is_empty()
                        && stdin.blocking_send(std::mem::take(&mut pending)).is_err()
                    {
                        return;
                    }
                    let _ = stdin.blocking_send(vec![0x04]);
                    return;
                }
                logrus::error("error while reading on console:EOF");
                state.cancel();
                return;
            }
            Ok(n) => n,
            Err(e) if e.kind() == std::io::ErrorKind::Interrupted => continue,
            Err(e) => {
                let e = go_path_error("read", std::path::Path::new(input.name()), &e);
                logrus::error(format!("error while reading on console:{e}"));
                state.cancel();
                return;
            }
        };

        // Ctrl+C is not handled, in order to be able to kill commands inside the container
        let mut data = std::mem::take(&mut pending);
        data.extend_from_slice(&buffer[..count]);

        let (to_send, rest) = handle_bracketed_paste_fragmentation(&data);
        pending = rest;

        if !to_send.is_empty() && stdin.blocking_send(to_send).is_err() {
            return;
        }
    }
}
