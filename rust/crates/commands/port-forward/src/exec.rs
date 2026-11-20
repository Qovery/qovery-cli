//! Port of `pkg.ExecPortForward` / `handleConnection` (`pkg/port-forward.go`): a local TCP listener whose
//! connections are each tunnelled over their own websocket (`/shell/portforward`).

use crate::PortForwardRequest;
use futures_util::{SinkExt, StreamExt};
use qovery_cmd_log::ws::{
    CLOSE_NORMAL_CLOSURE, ReadError, is_agent_response_timeout, is_internal_server_error,
    is_permanent_close_error, service_unavailable_message,
};
use qovery_cmd_shell::ws;
use qovery_log::logrus;
use std::io::Write;
use std::net::SocketAddr;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio_tungstenite::tungstenite::Message;

/// io.Copy buffer size: a websocket message bigger than this is truncated like `copy(p, msg)` in Go.
const COPY_BUFFER_SIZE: usize = 32 * 1024;

fn print(s: String) {
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(s.as_bytes());
    let _ = out.flush();
}

use qovery_core::gosys::errno_text as io_reason;

/// Go `net.Listen("tcp", "localhost:<port>")`: the first IPv4 address of localhost.
fn listen_address(port: u16) -> SocketAddr {
    use std::net::ToSocketAddrs;
    let addrs: Vec<SocketAddr> = ("localhost", port)
        .to_socket_addrs()
        .map(|a| a.collect())
        .unwrap_or_default();
    addrs
        .iter()
        .find(|a| a.is_ipv4())
        .or_else(|| addrs.first())
        .copied()
        .unwrap_or_else(|| SocketAddr::from(([127, 0, 0, 1], port)))
}

/// `pkg.ExecPortForward(req)`.
pub async fn exec_port_forward(req: PortForwardRequest) {
    let addr = listen_address(req.local_port);
    let listener = match TcpListener::bind(addr).await {
        Ok(l) => l,
        Err(e) => {
            print(format!("listen tcp {addr}: bind: {}\n", io_reason(&e)));
            return;
        }
    };
    let local = listener
        .local_addr()
        .map(|a| a.to_string())
        .unwrap_or_default();
    print(format!("Listening on {local} => {}\n", req.port));

    loop {
        match listener.accept().await {
            Ok((con, _)) => {
                tokio::spawn(handle_connection(con, req.clone()));
            }
            Err(e) => print(format!("accept tcp {local}: {}\n", io_reason(&e))),
        }
    }
}

/// The error ending a tunnelled connection (`errRet`).
enum PfError {
    /// error of `io.Copy(con, wsConn)` (`TCPConn.ReadFrom` wraps it: `readfrom tcp <local>-><remote>: <err>`)
    Ws {
        prefix: String,
        err: ReadError,
    },
    Other(String),
}

/// `handleConnection(con, req)`.
async fn handle_connection(con: TcpStream, req: PortForwardRequest) {
    let remote = con.peer_addr().map(|a| a.to_string()).unwrap_or_default();
    let local = con.local_addr().map(|a| a.to_string()).unwrap_or_default();
    print(format!(
        "Connection accepted from {remote} => {}\n",
        req.port
    ));

    let err_ret = tunnel(con, &req, &local, &remote).await;

    print(format!("Connection closed from {remote} => {}\n", req.port));
    match &err_ret {
        Some(PfError::Ws { err: e, .. }) if is_permanent_close_error(e) => logrus::error(
            "Port-forward connection rejected: check your permissions or run 'qovery auth'",
        ),
        Some(PfError::Ws { err: e, .. }) if is_agent_response_timeout(e) => logrus::warn(
            "Port-forward timed out (agent could not reach the pod or set up the forward). Reconnect to try again.",
        ),
        Some(PfError::Ws { err: e, .. }) if is_internal_server_error(e) => logrus::warn(format!(
            "{} Reconnect to try again.",
            service_unavailable_message("Port-forward")
        )),
        Some(PfError::Ws {
            err: ReadError::Close(c),
            ..
        }) if c.code == CLOSE_NORMAL_CLOSURE => {}
        Some(PfError::Ws { prefix, err }) => {
            logrus::error(format!("Port-forward connection terminated: {prefix}{err}"))
        }
        Some(PfError::Other(e)) => {
            logrus::error(format!("Port-forward connection terminated: {e}"))
        }
        None => {}
    }
}

/// `mkWebsocketConn(req)` + the two `io.Copy`; the TCP connection is closed when this returns.
async fn tunnel(
    con: TcpStream,
    req: &PortForwardRequest,
    local: &str,
    remote: &str,
) -> Option<PfError> {
    let url = qovery_cmd_shell::query::websocket_request_url("/shell/portforward", &req.query());
    let conn = match ws::authorization_async().await {
        Ok(auth) => ws::dial(&url, &auth).await.1,
        Err(e) => Err(e),
    };
    let conn = match conn {
        Ok(c) => c,
        Err(e) => {
            logrus::error(format!("error while creating websocket connection: {e}"));
            return Some(PfError::Other(e));
        }
    };

    let (mut sink, mut source) = conn.split();
    let (mut con_read, mut con_write) = con.into_split();

    // go io.Copy(wsConn, con): each read becomes a binary message; it just stops on error / EOF
    let to_ws = tokio::spawn(async move {
        let mut buf = vec![0u8; COPY_BUFFER_SIZE];
        loop {
            match con_read.read(&mut buf).await {
                Ok(0) | Err(_) => return,
                Ok(n) => {
                    if sink.send(Message::Binary(buf[..n].to_vec())).await.is_err() {
                        return;
                    }
                }
            }
        }
    });

    // io.Copy(con, wsConn)
    let prefix = format!("readfrom tcp {local}->{remote}: ");
    let err = loop {
        match ws::read_message(&mut source).await {
            Err(e) => break PfError::Ws { prefix, err: e },
            Ok((true, _)) => continue,
            Ok((false, msg)) => {
                let n = msg.len().min(COPY_BUFFER_SIZE);
                if n == 0 {
                    continue;
                }
                if let Err(e) = con_write.write_all(&msg[..n]).await {
                    break PfError::Other(format!(
                        "{prefix}write tcp {local}->{remote}: write: {}",
                        io_reason(&e)
                    ));
                }
            }
        }
    };
    // wsConn.ws.Close() and con.Close()
    to_ws.abort();
    drop(source);
    drop(con_write);
    Some(err)
}
