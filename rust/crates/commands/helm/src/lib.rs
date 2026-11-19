//! `qovery helm` command. Port of `cmd/helm*.go` (and `cmd/hem_domain_delete.go`).
//!
//! * `actions`: list, deploy, redeploy, stop, delete, cancel, clone (`helm_list.go`, `helm_deploy.go`, ...)
//! * `update`: `helm update` and its request builders (`helm_update.go`)
//! * `domain`: `helm domain ...` (`helm_domain*.go`, `helm_container_create.go`, `hem_domain_delete.go`)
//! * `env`: `helm env ...` and `helm external-secret ...` (`helm_env*.go`, `helm_external_secret*.go`)

mod actions;
mod domain;
mod env;
mod go_decode;
mod update;

use qovery_api::Client;
use qovery_api::models::Helm;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, exit, find::find_by_name, telemetry};
use qovery_services::path_escape;

/// The `helm` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("helm")
        .short("Manage helms")
        .run(run_group)
        .subcommand(actions::cancel())
        .subcommand(actions::clone())
        .subcommand(actions::delete())
        .subcommand(actions::deploy())
        .subcommand(actions::list())
        .subcommand(actions::redeploy())
        .subcommand(actions::stop())
        .subcommand(update::update())
        .subcommand(domain::command())
        .subcommand(env::command())
        .subcommand(env::external_secret_command())
}

/// Go group commands: `utils.Capture(cmd)` then `cmd.Help()` + `os.Exit(0)` without args.
pub(crate) fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

/// `--organization`, `--project`, `--environment` flags declared by every helm sub-command.
pub(crate) fn context_flags(cmd: Command) -> Command {
    cmd.flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
}

/// `client.HelmsAPI.ListHelms(ctx, envId)`, exit 1 on error.
pub(crate) fn list_helms(client: &Client, env_id: &str) -> Vec<Helm> {
    check_error(client.list(&format!("/environment/{}/helm", path_escape(env_id))))
}

/// `helm %s not found` + `You can list all helms with: qovery helm list`, exit 1.
pub(crate) fn helm_not_found(name: &str) -> ! {
    qovery_log::println_error(format!("helm {name} not found"));
    qovery_log::println_info("You can list all helms with: qovery helm list");
    exit(1)
}

/// `utils.FindByHelmName(helms.GetResults(), helmName)` or the not found error.
pub(crate) fn find_helm_or_exit(helms: &[Helm], name: &str) -> Helm {
    match find_by_name(helms, name) {
        Some(h) => h.clone(),
        None => helm_not_found(name),
    }
}

/// The common prologue of most helm commands: client, `(organization, project, environment)` ids from the
/// flags/context, list of the environment helms and lookup of `--helm` (Go `GetAccessToken` +
/// `getOrganizationProjectEnvironmentContextResourcesIds` + `ListHelms` + `FindByHelmName`).
pub(crate) fn resolve_helm(ctx: &Ctx) -> (Client, (String, String, String), Helm) {
    let client = check_error(qovery_core::client::get_client());
    let ids = check_error(qovery_core::resources::ids_from_flags(&client, ctx));
    let helms = list_helms(&client, &ids.2);
    let helm = find_helm_or_exit(&helms, &ctx.string("helm"));
    (client, ids, helm)
}
