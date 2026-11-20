//! `qovery shell` command. Port of `cmd/shell.go` and `pkg/shell.go`.
//!
//! * [`target`]: resolution of the targeted service (shared with `port-forward` / `list-pods`)
//! * [`query`]: query string of the websocket gateway requests (go-querystring port)
//! * [`ws`]: websocket client helpers (gorilla-like, on top of `qovery_cmd_log::ws`)
//! * `exec`: the interactive session (`pkg.ExecShell`)

mod console;
mod exec;
pub mod paste;
pub mod query;
pub mod target;
pub mod ws;

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, resources, telemetry};
use query::Query;
use target::{Feature, Target};

pub use exec::exec_shell;

/// `pkg.ShellRequest`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ShellRequest {
    pub service_id: String,
    pub environment_id: String,
    pub project_id: String,
    pub organization_id: String,
    pub cluster_id: String,
    pub pod_name: String,
    pub container_name: String,
    pub command: Vec<String>,
    pub tty_width: u16,
    pub tty_height: u16,
    pub ephemeral_mode: String,
    pub cpu_override: String,
    pub memory_override: String,
}

impl ShellRequest {
    fn from_target(t: Target, ctx: &Ctx) -> Self {
        ShellRequest {
            service_id: t.service_id,
            environment_id: t.environment_id,
            project_id: t.project_id,
            organization_id: t.organization_id,
            cluster_id: t.cluster_id,
            pod_name: ctx.string("pod"),
            container_name: ctx.string("container"),
            command: ctx.strings("command"),
            ..Default::default()
        }
    }

    /// `SetTtySize(width, height)`.
    pub fn set_tty_size(&mut self, width: u16, height: u16) {
        self.tty_width = width;
        self.tty_height = height;
    }

    /// `query.Values(req)` (field tags of `pkg.ShellRequest`).
    pub fn query(&self) -> Query {
        let mut q = Query::new();
        q.add("service", self.service_id.as_str())
            .add("environment", self.environment_id.as_str())
            .add("project", self.project_id.as_str())
            .add("organization", self.organization_id.as_str())
            .add("cluster", self.cluster_id.as_str())
            .add_omitempty("pod_name", &self.pod_name)
            .add_omitempty("container_name", &self.container_name)
            .add_slice("command", &self.command)
            .add("tty_width", self.tty_width.to_string())
            .add("tty_height", self.tty_height.to_string())
            .add_omitempty("mode", &self.ephemeral_mode)
            .add_omitempty("cpu_override", &self.cpu_override)
            .add_omitempty("memory_override", &self.memory_override);
        q
    }
}

const EXAMPLE: &str = "qovery shell\n\
qovery shell <qovery_console_service_url>\n\
qovery shell --organization <organization_name> --project <project_name> --environment <environment_name> --service <service_name>\n\
qovery shell --ephemeral --mode clone --organization <organization_name> --project <project_name> --environment <environment_name> --service <service_name>\n\
qovery shell --ephemeral --mode clone --memory 2Gi --organization <organization_name> --project <project_name> --environment <environment_name> --service <service_name>\n\
qovery shell --ephemeral --mode debug --organization <organization_name> --project <project_name> --environment <environment_name> --service <service_name>";

/// The `shell` command.
pub fn command() -> Command {
    Command::new("shell")
        .short("Connect to an application container")
        .example(EXAMPLE)
        .flag(Flag::string_slice("command", "command to launch inside the pod").short('c').default_value("[sh]"))
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
        .flag(Flag::string("service", "Service Name"))
        .flag(Flag::string("pod", "pod name where to exec into").short('p'))
        .flag(Flag::string("container", "container name inside the pod"))
        .flag(Flag::bool("ephemeral", "spawn an ephemeral shell instead of connecting to an existing pod"))
        .flag(
            Flag::string(
                "mode",
                "ephemeral mode: 'clone' (new isolated pod, Heroku-style) or 'debug' (ephemeral container injected into existing pod, kubectl-debug style)",
            )
            .default_value("clone"),
        )
        .flag(Flag::string("cpu", "override CPU request+limit for the ephemeral pod (e.g. '500m', '2')"))
        .flag(Flag::string("memory", "override memory request+limit for the ephemeral pod (e.g. '512Mi', '2Gi')"))
        .run(run_shell)
}

fn run_shell(ctx: &Ctx) {
    telemetry::capture(ctx);

    let organization_name = ctx.string("organization");
    let project_name = ctx.string("project");
    let environment_name = ctx.string("environment");
    let service_name = ctx.string("service");

    let request = if !organization_name.trim().is_empty()
        || !project_name.trim().is_empty()
        || !environment_name.trim().is_empty()
        || !service_name.trim().is_empty()
    {
        for (value, what) in [
            (&organization_name, "organization"),
            (&project_name, "project"),
            (&environment_name, "environment"),
            (&service_name, "service"),
        ] {
            if value.trim().is_empty() {
                qovery_log::println_error(format!("{what} name is required"));
                return;
            }
        }
        Ok(request_with_context_flags(
            ctx,
            &organization_name,
            &project_name,
            &environment_name,
            &service_name,
        ))
    } else if ctx.args().len() == 1 {
        target::with_application_url(ctx.args(), Feature::Shell)
            .map(|t| ShellRequest::from_target(t, ctx))
    } else {
        target::without_arg(Feature::Shell).map(|t| ShellRequest::from_target(t, ctx))
    };
    let mut request = match request {
        Ok(r) => r,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };

    let mut endpoint = "/shell/exec";
    if ctx.bool("ephemeral") {
        let mode = ctx.string("mode");
        let cpu = ctx.string("cpu");
        let memory = ctx.string("memory");
        if mode != "clone" && mode != "debug" {
            qovery_log::println_error("--mode must be 'clone' or 'debug'");
            return;
        }
        if mode == "debug" && (!cpu.is_empty() || !memory.is_empty()) {
            qovery_log::println_info(
                "--cpu/--memory only apply to --mode clone; ignoring them in debug mode.",
            );
        }
        request.ephemeral_mode = mode;
        request.cpu_override = cpu;
        request.memory_override = memory;
        endpoint = "/shell/ephemeral";
    } else if ctx.changed("mode") {
        qovery_log::println_info("--mode has no effect without --ephemeral; ignoring it.");
    }
    exec_shell(request, endpoint);
}

/// `shellRequestWithContextFlags()`: every lookup error prints `Error: <err>` and exits 1.
fn request_with_context_flags(
    ctx: &Ctx,
    organization: &str,
    project: &str,
    environment: &str,
    service: &str,
) -> ShellRequest {
    let client = client::get_client_or_exit();
    let organization_id = check_error(resources::organization_id(&client, organization));
    let project_id = check_error(resources::project_id(&client, project, &organization_id));
    let environment_id = check_error(resources::environment_id(&client, environment, &project_id));
    let env = check_error(qovery_services::get_environment_by_id(&environment_id));
    let service = check_error(resources::service_by_name(
        &client,
        service,
        &environment_id,
    ))
    .unwrap_or_default();

    ShellRequest::from_target(
        Target {
            organization_id,
            project_id,
            environment_id,
            cluster_id: env.cluster_id,
            service_id: service.id,
            ..Default::default()
        },
        ctx,
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn shell_request_query() {
        let mut r = ShellRequest {
            service_id: "s".into(),
            environment_id: "e".into(),
            project_id: "p".into(),
            organization_id: "o".into(),
            cluster_id: "c".into(),
            command: vec!["sh".into(), "-c".into(), "ls -l".into()],
            ..Default::default()
        };
        r.set_tty_size(80, 24);
        assert_eq!(
            r.query().encode(),
            "cluster=c&command[0]=sh&command[1]=-c&command[2]=ls+-l&environment=e&organization=o&project=p&service=s&tty_height=24&tty_width=80"
        );
        r.ephemeral_mode = "debug".into();
        r.pod_name = "pod-1".into();
        assert!(
            r.query()
                .encode()
                .contains("&mode=debug&organization=o&pod_name=pod-1&")
        );
    }
}
