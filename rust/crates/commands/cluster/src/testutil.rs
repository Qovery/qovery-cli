//! Test helpers: a tiny HTTP mock of the Qovery API (port of the `httpmock` responders used by the Go tests).

use serde_json::{Value, json};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{Arc, Mutex};

#[derive(Clone, Debug)]
pub struct Route {
    pub method: String,
    pub path: String,
    pub status: u16,
    pub body: String,
}

impl Route {
    pub fn new(method: &str, path: &str, status: u16, body: Value) -> Self {
        Route {
            method: method.to_string(),
            path: path.to_string(),
            status,
            body: body.to_string(),
        }
    }
    pub fn get(path: &str, body: Value) -> Self {
        Route::new("GET", path, 200, body)
    }
    pub fn raw(method: &str, path: &str, status: u16, body: &str) -> Self {
        Route {
            method: method.to_string(),
            path: path.to_string(),
            status,
            body: body.to_string(),
        }
    }
}

#[derive(Clone, Debug)]
#[allow(dead_code, reason = "fields recorded for test assertions")]
pub struct Request {
    pub method: String,
    pub path: String,
    pub body: Value,
}

pub struct MockServer {
    pub url: String,
    requests: Arc<Mutex<Vec<Request>>>,
}

static ENV_LOCK: Mutex<()> = Mutex::new(());

impl MockServer {
    pub fn start(routes: Vec<Route>) -> MockServer {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("http://{}", listener.local_addr().unwrap());
        let requests = Arc::new(Mutex::new(Vec::new()));
        let reqs = requests.clone();
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                let Ok(mut stream) = stream else { break };
                let routes = routes.clone();
                let reqs = reqs.clone();
                std::thread::spawn(move || {
                    let mut reader = BufReader::new(stream.try_clone().unwrap());
                    loop {
                        let mut line = String::new();
                        if reader.read_line(&mut line).unwrap_or(0) == 0 {
                            return;
                        }
                        let mut parts = line.split_whitespace();
                        let method = parts.next().unwrap_or_default().to_string();
                        let target = parts.next().unwrap_or_default().to_string();
                        let path = target.split('?').next().unwrap_or_default().to_string();
                        let mut len = 0usize;
                        loop {
                            let mut h = String::new();
                            if reader.read_line(&mut h).unwrap_or(0) == 0 {
                                return;
                            }
                            let h = h.trim_end();
                            if h.is_empty() {
                                break;
                            }
                            if let Some((k, v)) = h.split_once(':')
                                && k.eq_ignore_ascii_case("content-length")
                            {
                                len = v.trim().parse().unwrap_or(0);
                            }
                        }
                        let mut body = vec![0u8; len];
                        if len > 0 {
                            reader.read_exact(&mut body).unwrap();
                        }
                        let body_value = serde_json::from_slice(&body).unwrap_or_else(|_| {
                            if body.is_empty() {
                                Value::Null
                            } else {
                                Value::String(String::from_utf8_lossy(&body).to_string())
                            }
                        });
                        reqs.lock().unwrap().push(Request {
                            method: method.clone(),
                            path: path.clone(),
                            body: body_value,
                        });
                        let route = routes.iter().find(|r| r.method == method && r.path == path);
                        let (status, payload) = match route {
                            Some(r) => (r.status, r.body.clone()),
                            None => (404, json!({"error": "no route"}).to_string()),
                        };
                        let resp = format!(
                            "HTTP/1.1 {} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
                            status,
                            qovery_api::status_line(status)
                                .split_once(' ')
                                .map(|x| x.1)
                                .unwrap_or(""),
                            payload.len(),
                            payload
                        );
                        if stream.write_all(resp.as_bytes()).is_err() {
                            return;
                        }
                    }
                });
            }
        });
        MockServer { url, requests }
    }

    /// A Qovery API client pointing to this server.
    pub fn client(&self) -> qovery_api::Client {
        let _guard = ENV_LOCK.lock().unwrap_or_else(|e| e.into_inner());
        // SAFETY: tests serialize the env access needed to build the client with ENV_LOCK
        unsafe { std::env::set_var("QOVERY_API_URL", &self.url) };
        qovery_api::Client::new("Token test", "test", false)
    }

    pub fn requests(&self) -> Vec<Request> {
        self.requests.lock().unwrap().clone()
    }
}

/// `CreateTestCluster` / `CreateSelfManagedTestCluster`.
pub fn cluster_json(
    id: &str,
    name: &str,
    org_id: &str,
    cloud_provider: &str,
    kubernetes: &str,
) -> Value {
    json!({
        "id": id,
        "created_at": "2024-01-01T00:00:00Z",
        "organization": {"id": org_id},
        "name": name,
        "region": "eu-west-3",
        "cloud_provider": cloud_provider,
        "kubernetes": kubernetes,
    })
}
