//! `qovery service` command. Port of `cmd/service.go`, `cmd/service_list.go` and `cmd/service_deploy.go`.
//!
//! The context helpers of `cmd/service_list.go` live in `qovery_core::resources`.

mod deploy;
mod list;

use qovery_cobra::{Command, Ctx};

/// The `service` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("service")
        .short("Manage services")
        .run(run_group)
        .subcommand(deploy::command())
        .subcommand(list::command())
}

/// Go: `utils.Capture(cmd); if len(args) == 0 { cmd.Help(); os.Exit(0) }`.
fn run_group(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        qovery_core::exit(0);
    }
}
