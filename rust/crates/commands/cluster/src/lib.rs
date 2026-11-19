//! `qovery cluster` command. Port of `cmd/cluster*.go` and of the `pkg/` code they use
//! (`pkg/cluster/**`, `pkg/cluster.go`, `pkg/lock.go`, `pkg/organization`, `pkg/promptuifactory`, ...).
//!
//! Helpers also used by other commands are public: [`download_kubeconfig`] (terraform),
//! [`admin_locked_clusters`] and [`display_version`] (admin).

use qovery_cobra::{Command, Ctx};

mod analysis;
mod debug_pod;
mod install;
mod lock;
mod manage;
mod nodes;
mod operator;
mod preflight;
mod prompt;
mod shell;
mod token;
mod util;

#[cfg(test)]
mod testutil;

pub use lock::admin_locked_clusters;
pub use operator::display_version;
pub use token::{download_kubeconfig, get_kubeconfig_by_cluster_id, get_token_by_cluster_id};

/// The `cluster` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("cluster")
        .short("Manage clusters")
        .run(run_group)
        .subcommand(analysis::command())
        .subcommand(debug_pod::command())
        .subcommand(manage::deploy_command())
        .subcommand(token::get_token_command())
        .subcommand(install::command())
        .subcommand(token::kubeconfig_command())
        .subcommand(manage::list_command())
        .subcommand(nodes::command())
        .subcommand(lock::lock_command())
        .subcommand(lock::locked_command())
        .subcommand(operator::command())
        .subcommand(preflight::command())
        .subcommand(manage::stop_command())
        .subcommand(lock::unlock_command())
        .subcommand(manage::upgrade_command())
}

/// Go group commands: `utils.Capture(cmd); if len(args) == 0 { cmd.Help(); os.Exit(0) }`.
pub(crate) fn run_group(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        qovery_core::exit(0);
    }
}
