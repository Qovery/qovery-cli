//! `qovery admin cluster status`. Port of `cmd/admin_cluster_status.go` (one message read from the websocket).

use qovery_cobra::{Command, Ctx, Flag};
use qovery_log::pterm::{BOLD, red};
use serde::Deserialize;

pub fn command() -> Command {
    Command::new("status")
        .short("Get cluster status")
        .flag(Flag::string(
            "organization-id",
            "The cluster's organization ",
        ))
        .flag(Flag::string("cluster-id", "The cluster id to target"))
        .run(run)
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct ClusterStatusDto {
    pub nodes: Option<Vec<ClusterNodeDto>>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct ClusterNodeDto {
    pub name: Option<String>,
    pub resources_allocated: Option<NodeResourceAllocatedDto>,
    pub pods: Option<Vec<NodePodInfoDto>>,
    pub metrics_usage: Option<MetricsUsageDto>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct NodeResourceAllocatedDto {
    pub memory_mib: Option<u32>,
    pub cpu_milli: Option<u32>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(default)]
pub struct NodePodInfoDto {
    pub name: Option<String>,
    pub error_container_statuses: Option<Vec<serde_json::Value>>,
    pub cpu_milli_request: Option<u32>,
    pub memory_mib_request: Option<u32>,
    pub metrics_usage: Option<MetricsUsageDto>,
}

#[derive(Debug, Default, Clone, Deserialize)]
#[serde(default)]
pub struct MetricsUsageDto {
    pub cpu_milli_usage: Option<u32>,
    pub cpu_percent_usage: Option<u32>,
    pub memory_mib_rss_usage: Option<u32>,
    pub memory_percent_rss_usage: Option<u32>,
    pub disk_mib_usage: Option<u32>,
    pub disk_percent_usage: Option<u32>,
}

use qovery_api::query_escape;

fn read_cluster_status(
    organization_id: &str,
    cluster_id: &str,
) -> Result<ClusterStatusDto, String> {
    // query.Values(req).Encode(): keys sorted
    let url = format!(
        "{}/cluster/status?cluster={}&organization={}",
        qovery_core::client::websocket_url(),
        query_escape(cluster_id),
        query_escape(organization_id)
    );
    let authorization = crate::common::authorization()?;

    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|e| e.to_string())?;
    runtime.block_on(async move {
        use futures_util::StreamExt;
        use tokio_tungstenite::tungstenite::{Error, Message, client::IntoClientRequest};

        let mut request = url
            .as_str()
            .into_client_request()
            .map_err(|_| "malformed ws or wss URL".to_string())?;
        request.headers_mut().insert(
            "Authorization",
            authorization
                .parse()
                .map_err(|_| "invalid authorization header".to_string())?,
        );
        let (mut stream, _) =
            tokio_tungstenite::connect_async(request)
                .await
                .map_err(|e| match e {
                    Error::Http(_) => "websocket: bad handshake".to_string(),
                    Error::Url(_) => "malformed ws or wss URL".to_string(),
                    Error::Io(e) => go_dial_error(&url, &e),
                    other => other.to_string(),
                })?;
        let msg = loop {
            match stream.next().await {
                None => {
                    return Err(
                        "websocket: close 1006 (abnormal closure): unexpected EOF".to_string()
                    );
                }
                Some(Err(e)) => return Err(e.to_string()),
                Some(Ok(Message::Ping(_) | Message::Pong(_) | Message::Frame(_))) => continue,
                Some(Ok(m)) => break m,
            }
        };
        let _ = stream.close(None).await;
        match msg {
            Message::Text(t) => {
                serde_json::from_str::<ClusterStatusDto>(&t).map_err(|e| e.to_string())
            }
            Message::Binary(b) => Err(format!(
                "received invalid message while fetching cluster status: \u{2} {}",
                String::from_utf8_lossy(&b)
            )),
            Message::Close(frame) => Err(match frame {
                Some(f) => format!("websocket: close {} {}", u16::from(f.code), f.reason),
                None => "websocket: close 1005 (no status)".to_string(),
            }),
            _ => Err("websocket: unexpected message".to_string()),
        }
    })
}

fn go_dial_error(url: &str, e: &std::io::Error) -> String {
    if e.kind() == std::io::ErrorKind::ConnectionRefused {
        let addr = url::Url::parse(url)
            .ok()
            .and_then(|u| Some(format!("{}:{}", u.host_str()?, u.port_or_known_default()?)))
            .unwrap_or_default();
        return format!("dial tcp {addr}: connect: connection refused");
    }
    e.to_string()
}

fn usage(a: Option<u32>, b: Option<u32>, unit: &str) -> String {
    match (a, b) {
        (Some(a), Some(b)) => format!("{a}{unit}({b}%)"),
        _ => "--(--%)".to_string(),
    }
}

/// `renderClusterStatus`.
pub fn render(status: &ClusterStatusDto) -> String {
    let bold = |s: &str| BOLD.sprint(s);
    let mut out = format!(
        "{:<71} {:<20} {:<20} {:<20} {:<20} {:<20}\n\n",
        "",
        bold("RAM Alloc"),
        bold("RAM Usage"),
        bold("CPU Alloc"),
        bold("CPU Usage"),
        bold("Disk Usage")
    );
    let mut nodes: Vec<&ClusterNodeDto> = status.nodes.iter().flatten().collect();
    nodes.sort_by(|a, b| {
        a.name
            .as_deref()
            .unwrap_or("")
            .cmp(b.name.as_deref().unwrap_or(""))
    });
    for (i, node) in nodes.iter().enumerate() {
        let alloc = node.resources_allocated.as_ref();
        let m = node.metrics_usage.clone().unwrap_or_default();
        out.push_str(&format!(
            "{:<79} {:<20} {:<20} {:<20} {:<20} {:<20}\n",
            bold(node.name.as_deref().unwrap_or("")),
            bold(&format!(
                "{}Mi",
                alloc.and_then(|a| a.memory_mib).unwrap_or(0)
            )),
            bold(&usage(
                m.memory_mib_rss_usage,
                m.memory_percent_rss_usage,
                "Mi"
            )),
            bold(&format!(
                "{}m",
                alloc.and_then(|a| a.cpu_milli).unwrap_or(0)
            )),
            bold(&usage(m.cpu_milli_usage, m.cpu_percent_usage, "m")),
            bold(&usage(m.disk_mib_usage, m.disk_percent_usage, "Mi")),
        ));
        let mut pods: Vec<&NodePodInfoDto> = node.pods.iter().flatten().collect();
        pods.sort_by(|a, b| {
            a.name
                .as_deref()
                .unwrap_or("")
                .cmp(b.name.as_deref().unwrap_or(""))
        });
        for (j, pod) in pods.iter().enumerate() {
            let prefix = if j == pods.len() - 1 {
                "└─ "
            } else {
                "├─ "
            };
            let pm = pod.metrics_usage.clone().unwrap_or_default();
            let name = pod.name.as_deref().unwrap_or("");
            let pod_name = if pod
                .error_container_statuses
                .as_ref()
                .is_some_and(|e| !e.is_empty())
            {
                format!("{:<77}", red(name))
            } else {
                format!("{name:<68}")
            };
            out.push_str(&format!(
                "{prefix}{pod_name} {:<12} {:<12} {:<12} {:<12} {:<12}\n",
                pod.memory_mib_request
                    .map(|v| format!("{v}Mi"))
                    .unwrap_or_else(|| "--".into()),
                usage(pm.memory_mib_rss_usage, pm.memory_percent_rss_usage, "Mi"),
                pod.cpu_milli_request
                    .map(|v| format!("{v}m"))
                    .unwrap_or_else(|| "--".into()),
                usage(pm.cpu_milli_usage, pm.cpu_percent_usage, "m"),
                usage(pm.disk_mib_usage, pm.disk_percent_usage, "Mi"),
            ));
        }
        if i != nodes.len() - 1 {
            out.push('\n');
        }
    }
    out
}

fn run(ctx: &Ctx) {
    let status = read_cluster_status(&ctx.string("organization-id"), &ctx.string("cluster-id"))
        .unwrap_or_else(|e| qovery_core::exit_with_error(e));
    print!("{}", render(&status));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn renders_nodes_and_pods_sorted() {
        let status: ClusterStatusDto = serde_json::from_str(
            r#"{"nodes":[
                {"name":"node-b","resources_allocated":{"memory_mib":100,"cpu_milli":200},"metrics_usage":{},"pods":[]},
                {"name":"node-a","resources_allocated":{"memory_mib":1,"cpu_milli":2},
                 "metrics_usage":{"memory_mib_rss_usage":5,"memory_percent_rss_usage":6,"cpu_milli_usage":7,"cpu_percent_usage":8},
                 "pods":[{"name":"z","error_container_statuses":[{"container_name":"c"}],"metrics_usage":{}},
                         {"name":"a","cpu_milli_request":10,"memory_mib_request":20,"metrics_usage":{"disk_mib_usage":1,"disk_percent_usage":2}}]}
            ]}"#,
        )
        .unwrap();
        let out = render(&status);
        let lines: Vec<&str> = out.lines().collect();
        assert!(lines[2].starts_with(&BOLD.sprint("node-a")));
        assert!(lines[2].contains(&BOLD.sprint("5Mi(6%)")));
        assert_eq!(
            lines[3],
            format!(
                "├─ {:<68} {:<12} {:<12} {:<12} {:<12} {:<12}",
                "a", "20Mi", "--(--%)", "10m", "--(--%)", "1Mi(2%)"
            )
        );
        assert!(lines[4].starts_with(&format!("└─ {}", red("z"))));
        assert_eq!(lines[5], "");
        assert!(lines[6].starts_with(&BOLD.sprint("node-b")));
        assert_eq!(lines.len(), 7);
    }

    #[test]
    fn query_escaping() {
        assert_eq!(query_escape("a b/c"), "a+b%2Fc");
    }
}
