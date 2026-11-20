//! `qovery container` command. Port of `cmd/container*.go` (including `container registry` and the
//! `domain`, `env` and `external-secret` sub-commands).
//!
//! * [`actions`]: deploy / redeploy / restart / stop / delete / cancel
//! * [`crud`]: list / create / update / clone / registry list
//! * [`domain`]: custom domains
//! * [`env`]: environment variables, aliases, overrides and external secrets

mod actions;
mod crud;
mod domain;
mod env;

use qovery_api::Client;
use qovery_api::models::Container;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, exit, exit_with_error, telemetry};

/// The `container` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("container")
        .short("Manage containers")
        .run(run_group_help)
        .subcommand(actions::cancel())
        .subcommand(crud::clone())
        .subcommand(crud::create())
        .subcommand(actions::delete())
        .subcommand(actions::deploy())
        .subcommand(domain::command())
        .subcommand(env::command())
        .subcommand(env::external_secret_command())
        .subcommand(crud::list())
        .subcommand(actions::redeploy())
        .subcommand(crud::registry())
        .subcommand(actions::restart())
        .subcommand(actions::stop())
        .subcommand(crud::update())
}

/// Go groups `Run: func(cmd, args) { utils.Capture(cmd); if len(args) == 0 { _ = cmd.Help(); os.Exit(0) } }`.
pub(crate) fn run_group_help(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

/// `--organization`, `--project` and `--environment` flags shared by most sub-commands.
pub(crate) fn with_context_flags(cmd: Command) -> Command {
    cmd.flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
}

/// `--container` / `-n` flag.
pub(crate) fn container_flag() -> Flag {
    Flag::string("container", "Container Name").short('n')
}

/// `utils.GetAccessToken(false)` + `utils.GetQoveryClient(...)`, `PrintlnError` + exit 1 on error.
pub(crate) fn client_or_exit() -> Client {
    check_error(qovery_core::client::get_client())
}

/// `getOrganizationProjectEnvironmentContextResourcesIds(client)`, `PrintlnError` + exit 1 on error.
pub(crate) fn ids_or_exit(client: &Client, ctx: &Ctx) -> (String, String, String) {
    check_error(qovery_core::resources::ids_from_flags(client, ctx))
}

/// `client.ContainersAPI.ListContainer(ctx, envId)`.
pub(crate) fn list_containers(client: &Client, env_id: &str) -> Vec<Container> {
    check_error(client.list::<Container>(&format!("/environment/{env_id}/container")))
}

/// Go: `container %s not found` error + `You can list all containers with: qovery container list` info, exit 1.
pub(crate) fn container_not_found(name: &str) -> ! {
    qovery_log::println_error(format!("container {name} not found"));
    qovery_log::println_info("You can list all containers with: qovery container list");
    exit(1)
}

/// List the containers of the environment and find `name` (`utils.FindByContainerName`), exit 1 when missing.
pub(crate) fn find_container_or_exit(client: &Client, env_id: &str, name: &str) -> Container {
    let containers = list_containers(client, env_id);
    match qovery_core::find::find_by_name(&containers, name) {
        Some(c) => c.clone(),
        None => container_not_found(name),
    }
}

/// `utils.PrintlnError(err); os.Exit(1)` of an error (re-exported for the modules).
pub(crate) fn fail(e: impl std::fmt::Display) -> ! {
    exit_with_error(e)
}
