//! `qovery application` command. Port of `cmd/application*.go`.
//!
//! * [`actions`]: `list`, `deploy`, `redeploy`, `restart`, `stop`, `delete`, `cancel`, `clone`, `update`
//!   (and the `buildApplicationListFromApplicationNames` / `validateApplicationArguments` helpers of
//!   `cmd/application_stop.go`)
//! * [`domain`]: `domain create|delete|edit|list`
//! * [`env`]: `env ...` and `external-secret ...`

mod actions;
mod domain;
mod env;
mod go_decode;

use qovery_api::Client;
use qovery_api::models::Application;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, find::find_by_name, telemetry};

/// The `application` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("application")
        .short("Manage applications")
        .run(run_help)
        .subcommand(actions::cancel())
        .subcommand(actions::clone())
        .subcommand(actions::delete())
        .subcommand(actions::deploy())
        .subcommand(domain::command())
        .subcommand(env::command())
        .subcommand(env::external_secret())
        .subcommand(actions::list())
        .subcommand(actions::redeploy())
        .subcommand(actions::restart())
        .subcommand(actions::stop())
        .subcommand(actions::update())
}

/// Go group commands with a `Run` printing the help when called without args.
pub(crate) fn run_help(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        qovery_core::exit(0);
    }
}

/// `--organization`, `--project`, `--environment` flags declared by every sub-command.
pub(crate) fn context_flags(cmd: Command) -> Command {
    cmd.flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
}

/// `--application` / `-n`.
pub(crate) fn application_flag() -> Flag {
    Flag::string("application", "Application Name").short('n')
}

/// `client.ApplicationsAPI.ListApplication(envId)`; exits like Go `utils.PrintlnError(err); os.Exit(1)`.
pub(crate) fn list_applications(client: &Client, env_id: &str) -> Vec<Application> {
    check_error(client.list(&format!(
        "/environment/{}/application",
        qovery_services::path_escape(env_id)
    )))
}

/// `application %s not found` + the list hint, then exit 1.
pub(crate) fn application_not_found(name: &str) -> ! {
    qovery_log::println_error(format!("application {name} not found"));
    qovery_log::println_info("You can list all applications with: qovery application list");
    qovery_core::exit(1)
}

/// `ListApplication` + `utils.FindByApplicationName` + the not found error used by most sub-commands.
pub(crate) fn find_application_or_exit(client: &Client, env_id: &str, name: &str) -> Application {
    let applications = list_applications(client, env_id);
    match find_by_name(&applications, name) {
        Some(a) => a.clone(),
        None => application_not_found(name),
    }
}
