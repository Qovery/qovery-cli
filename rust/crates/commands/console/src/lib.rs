//! `qovery console` command. Port of `cmd/console.go`.

use qovery_cobra::{Command, Ctx};
use qovery_core::{context, exit, telemetry};

/// The `console` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("console")
        .short("Opens the application in Qovery Console in your browser")
        .run(run_console)
}

fn fail(e: impl std::fmt::Display) -> ! {
    qovery_log::println_error(e);
    exit(0)
}

fn run_console(ctx: &Ctx) {
    telemetry::capture(ctx);
    let organization = context::current_organization(true).unwrap_or_else(|e| fail(e));
    let project = context::current_project(true).unwrap_or_else(|e| fail(e));
    let environment = context::current_environment(true).unwrap_or_else(|e| fail(e));
    let service = context::current_service(true).unwrap_or_else(|e| fail(e));

    let url = format!(
        "https://console.qovery.com/organization/{}/project/{}/environment/{}/{}/{}/general",
        organization.id, project.id, environment.id, service.service_type, service.id
    );
    qovery_log::println_info(format!("Opening {url}"));
    if let Err(e) = browser::open_url(&url) {
        fail(e);
    }
}

/// `github.com/pkg/browser` (shared with `qovery auth`).
pub use qovery_core::browser;
