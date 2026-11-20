//! `qovery context` command. Port of `cmd/context.go` and `cmd/context_set.go`.

use qovery_cobra::{Command, Ctx};
use qovery_core::{context, telemetry};

/// The `context` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("context")
        .short("Manage CLI context")
        .run(run_context)
        .subcommand(
            Command::new("set")
                .short("Set Qovery CLI context")
                .run(run_set),
        )
}

fn run_context(ctx: &Ctx) {
    telemetry::capture(ctx);
    qovery_log::println_info("Current context:");
    if context::print_context().is_err() {
        println!("Context not yet configured. ");
    }
    qovery_log::go_builtin_println("");
    qovery_log::println_info("You can set a new context using 'qovery context set'. ");
}

fn run_set(ctx: &Ctx) {
    telemetry::capture(ctx);
    if let Err(e) = context::set_context(true, true, true, true) {
        qovery_log::println_error(e);
    }
}
