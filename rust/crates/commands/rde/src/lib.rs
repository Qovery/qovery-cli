//! `qovery rde` command. Port of `cmd/rde*.go`.

mod blueprint;
mod create;
mod delete;
mod helpers;
mod instances;
mod sync;
mod upgrade;

use qovery_cobra::{Command, Ctx};
use qovery_core::telemetry;

const RDE_LONG: &str = "Manage Remote Development Environments (RDE).

RDE allows platform teams to provision isolated, pre-configured development
environments for developers. The system works as:

  1. Blueprints: Template projects/environments that serve as the source for cloning
  2. RDE instances: Cloned from a blueprint, with optional RBAC isolation and member invitation

Blueprint identification uses environment variables:
  - Project-level: BLUEPRINT_PROJECT_ID = <project ID> (marks a project as a blueprint)
  - Environment-level: BLUEPRINT_KEY = <blueprint project ID> (links environments to their blueprint)";

/// The `rde` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("rde")
        .short("Manage Remote Development Environments (RDE)")
        .long(RDE_LONG)
        .run(run_group)
        .subcommand(blueprint::command())
        .subcommand(create::command())
        .subcommand(delete::delete())
        .subcommand(delete::delete_all())
        .subcommand(instances::info())
        .subcommand(instances::list())
        .subcommand(instances::start())
        .subcommand(instances::start_all())
        .subcommand(instances::status())
        .subcommand(instances::stop())
        .subcommand(instances::stop_all())
        .subcommand(upgrade::command())
        .subcommand(instances::urls())
}

/// Run of the group commands (`rde`, `rde blueprint`): help when called without arguments.
pub(crate) fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        qovery_core::exit(0);
    }
}
