//! `qovery environment` command. Port of `cmd/environment*.go` (and of `pkg/deployment_log.go`, used by
//! `environment deployment logs`).
//!
//! * [`actions`]: `cancel`, `clone`, `delete`, `deploy`, `list`, `redeploy`, `statuses`, `stop`, `update`
//! * [`deployment`]: `deployment list|logs|explain`
//! * [`deployment_log`]: port of `pkg/deployment_log.go` (filters / rendering of deployment log lines)
//! * [`treeprint`]: port of `github.com/xlab/treeprint` (output of `deployment explain`)
//! * [`env`]: `env ...` and `external-secret ...`
//! * [`stage`]: `stage create|delete|edit|list|move|skip|unskip`

mod actions;
mod deploy_request;
mod deployment;
pub mod deployment_log;
mod env;
mod stage;
pub mod treeprint;

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{exit, telemetry};

/// The `environment` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("environment")
        .short("Manage environments")
        .run(run_group)
        .subcommand(actions::cancel())
        .subcommand(actions::clone())
        .subcommand(actions::delete())
        .subcommand(actions::deploy())
        .subcommand(deployment::command())
        .subcommand(env::env_command())
        .subcommand(env::external_secret_command())
        .subcommand(actions::list())
        .subcommand(actions::redeploy())
        .subcommand(stage::command())
        .subcommand(actions::statuses())
        .subcommand(actions::stop())
        .subcommand(actions::update())
}

/// Go groups: `utils.Capture(cmd); if len(args) == 0 { _ = cmd.Help(); os.Exit(0) }`.
pub(crate) fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

/// `--organization`, `--project`, `--environment` (no shorthand), the usual first flags of the sub-commands.
pub(crate) fn ope_flags() -> [Flag; 3] {
    [
        Flag::string("organization", "Organization Name"),
        Flag::string("project", "Project Name"),
        Flag::string("environment", "Environment Name"),
    ]
}

/// Go `fmt.Errorf(...)` printed with `utils.PrintlnError` then `os.Exit(1)`.
pub(crate) fn fail(msg: impl std::fmt::Display) -> ! {
    qovery_core::exit_with_error(msg)
}
