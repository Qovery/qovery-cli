//! `qovery version` command. Port of `cmd/version.go`.

use qovery_cobra::{Command, Ctx};
use qovery_core::{telemetry, version};

/// The `version` command.
pub fn command() -> Command {
    Command::new("version")
        .short("Print installed version of the Qovery CLI")
        .run(run)
}

fn run(ctx: &Ctx) {
    telemetry::capture(ctx);
    match version::current_version() {
        Ok(v) => qovery_log::println_info(format!("{v}\n")),
        Err(e) => qovery_core::exit_with_error(e),
    }
}
