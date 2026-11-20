//! Kubeconfig loading (clientcmd semantics and error messages) and the real Kubernetes API access (kube-rs).

use super::collector::{KubeApi, KubeError, KubeRequest};
use crate::util::encode_query;
use kube::config::{KubeConfigOptions, Kubeconfig};
use serde_json::Value;
use std::path::{Path, PathBuf};
use std::time::Duration;

const REQUEST_TIMEOUT: Duration = Duration::from_secs(20);

/// The loaded kubeconfig: `RawConfig()` + the selected context.
pub struct LoadedConfig {
    pub kubeconfig: Kubeconfig,
}

use qovery_core::gosys::errno_text as io_reason;

fn read_file(path: &Path) -> Result<Kubeconfig, String> {
    Kubeconfig::read_from(path).map_err(|e| {
        let mut msg = e.to_string();
        let mut src = std::error::Error::source(&e);
        while let Some(s) = src {
            msg = s.to_string();
            src = s.source();
        }
        format!("error loading config file \"{}\": {msg}", path.display())
    })
}

/// clientcmd default loading rules: `--kubeconfig` (must exist), else `$KUBECONFIG` (list, missing files
/// ignored, first value wins), else `~/.kube/config` (optional). Returns `RawConfig()`.
pub fn load_raw_config(explicit_path: &str) -> Result<LoadedConfig, String> {
    let empty = || Kubeconfig::default();
    if !explicit_path.is_empty() {
        let p = Path::new(explicit_path);
        if let Err(e) = std::fs::metadata(p)
            && e.kind() == std::io::ErrorKind::NotFound
        {
            return Err(format!("stat {explicit_path}: {}", io_reason(&e)));
        }
        return Ok(LoadedConfig {
            kubeconfig: read_file(p)?,
        });
    }
    let paths: Vec<PathBuf> = match std::env::var("KUBECONFIG") {
        Ok(v) if !v.is_empty() => std::env::split_paths(&v)
            .filter(|p| !p.as_os_str().is_empty())
            .collect(),
        _ => dirs_home()
            .map(|h| vec![h.join(".kube").join("config")])
            .unwrap_or_default(),
    };
    let mut merged: Option<Kubeconfig> = None;
    for p in paths {
        if !p.exists() {
            continue;
        }
        let k = read_file(&p)?;
        merged = Some(match merged {
            None => k,
            Some(m) => m.merge(k).map_err(|e| e.to_string())?,
        });
    }
    Ok(LoadedConfig {
        kubeconfig: merged.unwrap_or_else(empty),
    })
}

fn dirs_home() -> Option<PathBuf> {
    std::env::var_os("HOME").map(PathBuf::from)
}

impl LoadedConfig {
    pub fn current_context(&self) -> String {
        self.kubeconfig.current_context.clone().unwrap_or_default()
    }

    /// `ClientConfig()` validation (`ConfirmUsable`): returns the API server url of the context.
    pub fn server(&self, context_override: &str) -> Result<String, String> {
        let k = &self.kubeconfig;
        if !context_override.is_empty() && !k.contexts.iter().any(|c| c.name == context_override) {
            return Err(format!(
                "context {} does not exist",
                qovery_log::gofmt::quote(context_override)
            ));
        }
        let context_name = if context_override.is_empty() {
            self.current_context()
        } else {
            context_override.to_string()
        };
        let mut errors: Vec<String> = Vec::new();
        let context = k
            .contexts
            .iter()
            .find(|c| c.name == context_name)
            .and_then(|c| c.context.clone());
        if !context_name.is_empty()
            && context.is_none()
            && !k.contexts.iter().any(|c| c.name == context_name)
        {
            errors.push(format!(
                "context was not found for specified context: {context_name}"
            ));
        }
        let cluster_name = context
            .as_ref()
            .map(|c| c.cluster.clone())
            .unwrap_or_default();
        let cluster = k
            .clusters
            .iter()
            .find(|c| c.name == cluster_name)
            .and_then(|c| c.cluster.clone());
        const EMPTY_CLUSTER: &str = "cluster has no server defined";
        let server = cluster
            .as_ref()
            .and_then(|c| c.server.clone())
            .unwrap_or_default();
        match &cluster {
            None => errors.push(EMPTY_CLUSTER.to_string()),
            Some(_) if server.is_empty() => errors.push(format!(
                "no server found for cluster {}",
                qovery_log::gofmt::quote(&cluster_name)
            )),
            Some(_) => {}
        }
        if errors.len() == 1 && errors[0] == EMPTY_CLUSTER {
            return Err(
                "invalid configuration: no configuration has been provided, try setting KUBERNETES_MASTER environment variable"
                    .to_string(),
            );
        }
        match errors.len() {
            0 => Ok(server),
            1 => Err(format!("invalid configuration: {}", errors[0])),
            _ => Err(format!("invalid configuration: [{}]", errors.join(", "))),
        }
    }
}

/// The real Kubernetes API, through kube-rs.
pub struct RealKubeApi {
    rt: tokio::runtime::Runtime,
    client: kube::Client,
    server: String,
}

impl RealKubeApi {
    pub fn new(loaded: &LoadedConfig, context: &str, server: &str) -> Result<RealKubeApi, String> {
        let rt = tokio::runtime::Builder::new_multi_thread()
            .enable_all()
            .build()
            .map_err(|e| e.to_string())?;
        let options = KubeConfigOptions {
            context: Some(context.to_string()),
            cluster: None,
            user: None,
        };
        let client = rt.block_on(async {
            let mut config =
                kube::Config::from_custom_kubeconfig(loaded.kubeconfig.clone(), &options)
                    .await
                    .map_err(|e| e.to_string())?;
            config.read_timeout = Some(REQUEST_TIMEOUT);
            config.connect_timeout = Some(REQUEST_TIMEOUT);
            config.default_retry = false;
            kube::Client::try_from(config).map_err(|e| e.to_string())
        })?;
        Ok(RealKubeApi {
            rt,
            client,
            server: server.trim_end_matches('/').to_string(),
        })
    }
}

use qovery_api::go_url_error_op as go_method;

/// Go transport error reason (`dial tcp 127.0.0.1:1: connect: connection refused`, ...).
fn transport_reason(url: &str, e: &(dyn std::error::Error + 'static)) -> String {
    let mut chain: Vec<String> = Vec::new();
    let mut io: Option<&std::io::Error> = None;
    let mut cur: Option<&(dyn std::error::Error + 'static)> = Some(e);
    while let Some(err) = cur {
        chain.push(err.to_string());
        if io.is_none() {
            io = err.downcast_ref::<std::io::Error>();
        }
        cur = err.source();
    }
    let all = chain.join(": ");
    let (host, port) = match url::Url::parse(url) {
        Ok(u) => (
            u.host_str().unwrap_or_default().to_string(),
            u.port_or_known_default().unwrap_or(443),
        ),
        Err(_) => (String::new(), 0),
    };
    if let Some(io) = io {
        if io.kind() == std::io::ErrorKind::ConnectionRefused {
            return format!("dial tcp {host}:{port}: connect: connection refused");
        }
        if io.kind() == std::io::ErrorKind::TimedOut {
            return format!("dial tcp {host}:{port}: i/o timeout");
        }
    }
    if all.contains("UnknownIssuer") {
        return "tls: failed to verify certificate: x509: certificate signed by unknown authority"
            .to_string();
    }
    if all.contains("dns error") || all.contains("failed to lookup address") {
        return format!("dial tcp: lookup {host}: no such host");
    }
    chain.last().cloned().unwrap_or_default()
}

/// client-go `errors.NewGenericServerResponse` message for an error response that is not a `Status`.
fn generic_server_message(
    code: u16,
    method: &str,
    server_message: &str,
    resource: &Option<(&'static str, String)>,
) -> String {
    let mut message = match code {
        409 => "the server reported a conflict".to_string(),
        404 => "the server could not find the requested resource".to_string(),
        400 => "the server rejected our request for an unknown reason".to_string(),
        401 => "the server has asked for the client to provide credentials".to_string(),
        403 => server_message.to_string(),
        406 => {
            if server_message.is_empty() || server_message == "unknown" {
                "the server was unable to respond with a content type that the client supports".to_string()
            } else {
                server_message.to_string()
            }
        }
        415 => server_message.to_string(),
        405 => "the server does not allow this method on the requested resource".to_string(),
        422 => "the server rejected our request due to an error in our request".to_string(),
        503 => "the server is currently unable to handle the request".to_string(),
        504 => "the server was unable to return a response in the time allotted, but may still be processing the request".to_string(),
        429 => "the server has received too many requests and has asked us to try again later".to_string(),
        c if c >= 500 => format!(
            "an error on the server ({}) has prevented the request from succeeding",
            qovery_log::gofmt::quote(server_message)
        ),
        c => format!("the server responded with the status code {c} but did not return more information"),
    };
    if let Some((r, name)) = resource {
        if name.is_empty() {
            message = format!("{message} ({} {r})", method.to_lowercase());
        } else {
            message = format!("{message} ({} {r} {name})", method.to_lowercase());
        }
    }
    message
}

/// client-go error of a non 2xx response.
pub fn response_error(
    status: u16,
    content_type: &str,
    body: &[u8],
    req: &KubeRequest,
) -> KubeError {
    if content_type.contains("json")
        && let Ok(v) = serde_json::from_slice::<Value>(body)
        && v.get("kind").and_then(|k| k.as_str()) == Some("Status")
    {
        let reason = v.get("reason").and_then(|r| r.as_str()).unwrap_or_default();
        let code = v
            .get("code")
            .and_then(|c| c.as_i64())
            .unwrap_or(status as i64);
        return KubeError {
            message: v
                .get("message")
                .and_then(|m| m.as_str())
                .unwrap_or_default()
                .to_string(),
            not_found: reason == "NotFound" || (reason.is_empty() && code == 404),
        };
    }
    let server_message = if content_type.starts_with("text/") {
        String::from_utf8_lossy(&body[..body.len().min(1024)])
            .trim()
            .to_string()
    } else {
        "unknown".to_string()
    };
    KubeError {
        message: generic_server_message(status, req.method, &server_message, &req.resource),
        not_found: status == 404,
    }
}

impl KubeApi for RealKubeApi {
    fn request(&self, req: &KubeRequest) -> Result<Value, KubeError> {
        let query = encode_query(&req.query);
        let path_and_query = if query.is_empty() {
            req.path.clone()
        } else {
            format!("{}?{query}", req.path)
        };
        let full_url = format!("{}{}", self.server, path_and_query);
        let body = req
            .body
            .as_ref()
            .map(|b| serde_json::to_vec(b).unwrap_or_default())
            .unwrap_or_default();
        let mut builder = http::Request::builder()
            .method(req.method)
            .uri(&path_and_query)
            .header("Accept", req.accept);
        if req.body.is_some() {
            builder = builder.header("Content-Type", "application/json");
        }
        let request = builder
            .body(kube::client::Body::from(body))
            .map_err(|e| KubeError::new(e.to_string()))?;
        let transport_err = |e: &(dyn std::error::Error + 'static)| {
            KubeError::new(format!(
                "{} {}: {}",
                go_method(req.method),
                qovery_log::gofmt::quote(&full_url),
                transport_reason(&full_url, e)
            ))
        };
        self.rt.block_on(async {
            let res = match tokio::time::timeout(REQUEST_TIMEOUT, self.client.send(request)).await {
                Err(_) => {
                    return Err(KubeError::new(format!(
                        "{} {}: context deadline exceeded (Client.Timeout exceeded while awaiting headers)",
                        go_method(req.method),
                        qovery_log::gofmt::quote(&full_url)
                    )));
                }
                Ok(Err(e)) => return Err(transport_err(&e)),
                Ok(Ok(r)) => r,
            };
            let status = res.status().as_u16();
            let content_type = res.headers().get("content-type").and_then(|v| v.to_str().ok()).unwrap_or_default().to_string();
            let bytes = match tokio::time::timeout(REQUEST_TIMEOUT, collect_body(res.into_body())).await {
                Ok(Ok(b)) => b,
                Ok(Err(e)) => return Err(KubeError::new(e)),
                Err(_) => return Err(KubeError::new("context deadline exceeded")),
            };
            if !(200..=206).contains(&status) {
                return Err(response_error(status, &content_type, &bytes, req));
            }
            if bytes.is_empty() {
                return Ok(Value::Null);
            }
            serde_json::from_slice(&bytes).map_err(|e| KubeError::new(e.to_string()))
        })
    }
}

async fn collect_body(mut body: kube::client::Body) -> Result<Vec<u8>, String> {
    use http_body_util::BodyExt;
    let mut out = Vec::new();
    while let Some(frame) = body.frame().await {
        let frame = frame.map_err(|e| e.to_string())?;
        if let Some(data) = frame.data_ref() {
            out.extend_from_slice(data);
        }
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn loaded(yaml: &str) -> LoadedConfig {
        LoadedConfig {
            kubeconfig: Kubeconfig::from_yaml(yaml).unwrap(),
        }
    }

    const KUBECONFIG: &str = "apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:1
  name: c1
contexts:
- context:
    cluster: c1
    user: u1
  name: ctx1
current-context: ctx1
users:
- name: u1
  user:
    token: abc
";

    #[test]
    fn client_config_validation() {
        let k = loaded(KUBECONFIG);
        assert_eq!(k.current_context(), "ctx1");
        assert_eq!(k.server(""), Ok("http://127.0.0.1:1".to_string()));
        assert_eq!(
            k.server("nope"),
            Err("context \"nope\" does not exist".to_string())
        );
        let empty = LoadedConfig {
            kubeconfig: Kubeconfig::default(),
        };
        assert_eq!(
            empty.server(""),
            Err("invalid configuration: no configuration has been provided, try setting KUBERNETES_MASTER environment variable".to_string())
        );
    }

    #[test]
    fn generic_messages() {
        let req = KubeRequest {
            method: "GET",
            path: "/api/v1/nodes".into(),
            query: vec![],
            body: None,
            accept: "",
            resource: Some(("nodes", String::new())),
        };
        assert_eq!(
            response_error(404, "application/json", b"{}", &req).message,
            "the server could not find the requested resource (get nodes)"
        );
        assert!(response_error(404, "application/json", b"{}", &req).not_found);
        assert_eq!(
            response_error(500, "text/plain", b"boom\n", &req).message,
            "an error on the server (\"boom\") has prevented the request from succeeding (get nodes)"
        );
        let status = br#"{"kind":"Status","status":"Failure","message":"nodes is forbidden","reason":"Forbidden","code":403}"#;
        assert_eq!(
            response_error(403, "application/json", status, &req),
            KubeError {
                message: "nodes is forbidden".into(),
                not_found: false
            }
        );
    }
}
