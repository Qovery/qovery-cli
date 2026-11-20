//! `qovery port-forward` command. Port of `cmd/port-forward.go` and `pkg/port-forward.go`.
//!
//! The resolution of the targeted service is shared with `shell` (`qovery_cmd_shell::target`).

mod exec;

use qovery_cmd_shell::query::Query;
use qovery_cmd_shell::target::{self, Feature, Target};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::telemetry;
use qovery_log::logrus;

pub use exec::exec_port_forward;

/// `pkg.PortForwardRequest`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct PortForwardRequest {
    pub service_id: String,
    pub environment_id: String,
    pub project_id: String,
    pub organization_id: String,
    pub cluster_id: String,
    pub pod_name: String,
    pub blueprint_id: String,
    /// upper case service type (`APPLICATION`, ...)
    pub service_type: String,
    pub port: u16,
    pub local_port: u16,
}

impl PortForwardRequest {
    fn from_target(t: Target, pod_name: String) -> Self {
        PortForwardRequest {
            service_id: t.service_id,
            environment_id: t.environment_id,
            project_id: t.project_id,
            organization_id: t.organization_id,
            cluster_id: t.cluster_id,
            pod_name,
            blueprint_id: t.blueprint_id,
            service_type: t.service_type.to_uppercase(),
            port: 0,
            local_port: 0,
        }
    }

    /// `query.Values(req)` (field tags of `pkg.PortForwardRequest`; `LocalPort` has no tag).
    pub fn query(&self) -> Query {
        let mut q = Query::new();
        q.add("service", self.service_id.as_str())
            .add("environment", self.environment_id.as_str())
            .add("project", self.project_id.as_str())
            .add("organization", self.organization_id.as_str())
            .add("cluster", self.cluster_id.as_str())
            .add_omitempty("pod_name", &self.pod_name)
            .add_omitempty("blueprint_id", &self.blueprint_id)
            .add("service_type", self.service_type.as_str())
            .add("port", self.port.to_string())
            .add("LocalPort", self.local_port.to_string());
        q
    }
}

/// `portForwardRequestWithoutArg()`; `pod_name` is the `--pod` flag value.
pub fn request_without_arg(pod_name: String) -> Result<PortForwardRequest, String> {
    target::without_arg(Feature::PortForward).map(|t| PortForwardRequest::from_target(t, pod_name))
}

/// `portForwardRequestWithApplicationUrl(args)`; `pod_name` is the `--pod` flag value.
pub fn request_with_application_url(
    args: &[String],
    pod_name: String,
) -> Result<PortForwardRequest, String> {
    target::with_application_url(args, Feature::PortForward)
        .map(|t| PortForwardRequest::from_target(t, pod_name))
}

/// The `port-forward` command.
pub fn command() -> Command {
    Command::new("port-forward")
        .short("Port forward a port to an application container")
        .flag(Flag::string("pod", "pod name where to forward traffic"))
        .flag(
            Flag::string_slice(
                "port",
                "port that will be forwarded. Can be specified multiple time. Format \"local_port:remote_port\" i.e: 8080:80",
            )
            .short('p')
            .required(),
        )
        .run(run_port_forward)
}

/// Go `strconv.ParseUint(s, 10, 16)`.
fn parse_port(s: &str) -> Result<u16, String> {
    if s.is_empty() || !s.bytes().all(|b| b.is_ascii_digit()) {
        return Err(format!(
            "strconv.ParseUint: parsing {}: invalid syntax",
            qovery_log::gofmt::quote(s)
        ));
    }
    let trimmed = s.trim_start_matches('0');
    if trimmed.len() > 5 {
        return Err(format!(
            "strconv.ParseUint: parsing {}: value out of range",
            qovery_log::gofmt::quote(s)
        ));
    }
    let v: u32 = if trimmed.is_empty() {
        0
    } else {
        trimmed.parse().unwrap_or(u32::MAX)
    };
    u16::try_from(v).map_err(|_| {
        format!(
            "strconv.ParseUint: parsing {}: value out of range",
            qovery_log::gofmt::quote(s)
        )
    })
}

fn run_port_forward(ctx: &Ctx) {
    telemetry::capture(ctx);

    let ports = ctx.strings("port");
    if ports.is_empty() {
        logrus::fatal("port flag must be specified at least once");
    }

    let pod_name = ctx.string("pod");
    let request = if !ctx.args().is_empty() {
        request_with_application_url(ctx.args(), pod_name)
    } else {
        request_without_arg(pod_name)
    };
    let request = match request {
        Ok(r) => r,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };

    let rt = qovery_cmd_shell::ws::runtime();
    for port in &ports {
        let ps: Vec<&str> = port.split(':').collect();
        let (local_port_str, remote_port_str) = if ps.len() > 1 {
            (ps[0], ps[1])
        } else {
            (ps[0], ps[0])
        };

        // log.Fatal("Invalid local port {} {}", port, err): fmt.Sprint does not separate strings
        let local_port = parse_port(local_port_str)
            .unwrap_or_else(|e| logrus::fatal(format!("Invalid local port {{}} {{}}{port}{e}")));
        let remote_port = parse_port(remote_port_str)
            .unwrap_or_else(|e| logrus::fatal(format!("Invalid remote port {{}} {{}}{port}{e}")));

        let mut req = request.clone();
        req.local_port = local_port;
        req.port = remote_port;
        rt.spawn(exec_port_forward(req));
    }

    rt.block_on(wait_for_signal());
    rt.shutdown_background();
}

/// `signal.Notify(done, syscall.SIGINT, syscall.SIGTERM); <-done`.
async fn wait_for_signal() {
    #[cfg(unix)]
    {
        use tokio::signal::unix::{SignalKind, signal};
        let mut term = signal(SignalKind::terminate()).ok();
        let term = async move {
            match term.as_mut() {
                Some(t) => {
                    t.recv().await;
                }
                None => std::future::pending::<()>().await,
            }
        };
        tokio::select! {
            _ = tokio::signal::ctrl_c() => {}
            _ = term => {}
        }
    }
    #[cfg(not(unix))]
    {
        let _ = tokio::signal::ctrl_c().await;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_ports() {
        assert_eq!(parse_port("8080"), Ok(8080));
        assert_eq!(parse_port("0065535"), Ok(65535));
        assert_eq!(
            parse_port("65536").unwrap_err(),
            "strconv.ParseUint: parsing \"65536\": value out of range"
        );
        assert_eq!(
            parse_port("99999999999").unwrap_err(),
            "strconv.ParseUint: parsing \"99999999999\": value out of range"
        );
        assert_eq!(
            parse_port("").unwrap_err(),
            "strconv.ParseUint: parsing \"\": invalid syntax"
        );
        assert_eq!(
            parse_port("+1").unwrap_err(),
            "strconv.ParseUint: parsing \"+1\": invalid syntax"
        );
        assert_eq!(
            parse_port("8a").unwrap_err(),
            "strconv.ParseUint: parsing \"8a\": invalid syntax"
        );
    }

    #[test]
    fn request_query() {
        let r = PortForwardRequest {
            service_id: "s".into(),
            environment_id: "e".into(),
            project_id: "p".into(),
            organization_id: "o".into(),
            cluster_id: "c".into(),
            service_type: "APPLICATION".into(),
            port: 80,
            local_port: 8080,
            ..Default::default()
        };
        assert_eq!(
            r.query().encode(),
            "LocalPort=8080&cluster=c&environment=e&organization=o&port=80&project=p&service=s&service_type=APPLICATION"
        );
    }
}
