//! `qovery terraform external-secret`. Port of `cmd/terraform_external_secret*.go`.

use qovery_api::Client;
use qovery_api::models::Terraform;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::{check_error, client, resources, telemetry};
use qovery_log::pterm::FG_BLUE;
use qovery_services::path_escape;

pub(crate) fn command() -> Command {
    Command::new("external-secret")
        .short("Manage terraform external secrets")
        .run(crate::run_group)
        .subcommand(create())
        .subcommand(delete())
        .subcommand(update())
}

fn create() -> Command {
    crate::context_flags(Command::new("create").short("Create terraform external secret"))
        .flag(
            Flag::string("terraform", "Terraform Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("key", "External secret key")
                .short('k')
                .required(),
        )
        .flag(
            Flag::string(
                "reference",
                "Reference to the secret in the secrets provider",
            )
            .short('r')
            .required(),
        )
        .flag(Flag::string("secret-manager-access-name", "Secret manager access name").required())
        .flag(
            Flag::string(
                "scope",
                "Scope of this external secret <PROJECT|ENVIRONMENT|TERRAFORM>",
            )
            .default_value("TERRAFORM"),
        )
        .flag(Flag::string(
            "mount-path",
            "Path where the secret will be mounted as a file",
        ))
        .run(run_create)
}

fn delete() -> Command {
    crate::context_flags(Command::new("delete").short("Delete terraform external secret"))
        .flag(
            Flag::string("terraform", "Terraform Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("key", "External secret key")
                .short('k')
                .required(),
        )
        .run(run_delete)
}

fn update() -> Command {
    crate::context_flags(Command::new("update").short("Update terraform external secret"))
        .flag(
            Flag::string("terraform", "Terraform Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("key", "External secret key")
                .short('k')
                .required(),
        )
        .flag(
            Flag::string(
                "reference",
                "New reference to the secret in the secrets provider",
            )
            .short('r'),
        )
        .flag(Flag::string(
            "secret-manager-access-name",
            "New secret manager access name",
        ))
        .run(run_update)
}

/// Client, `(organization, project, environment)` ids and the `--terraform` service (exits when not found).
fn prelude(ctx: &Ctx) -> (Client, String, String, String, Terraform) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, project_id, env_id) =
        check_error(resources::ids_from_flags(&client, ctx));
    let terraforms: Vec<Terraform> =
        check_error(client.list(&format!("/environment/{}/terraform", path_escape(&env_id))));
    let name = ctx.string("terraform");
    let terraform = match qovery_core::find::find_by_name(&terraforms, &name) {
        Some(t) => t.clone(),
        None => crate::terraform_not_found(&name),
    };
    (client, organization_id, project_id, env_id, terraform)
}

fn run_create(ctx: &Ctx) {
    let (client, organization_id, project_id, env_id, terraform) = prelude(ctx);
    let secret_manager_access_id =
        check_error(qovery_envvars::get_secret_manager_access_id_by_name(
            &client,
            &organization_id,
            &env_id,
            &ctx.string("secret-manager-access-name"),
        ));
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_external_secret(
        &client,
        &project_id,
        &env_id,
        &terraform.id,
        &ctx.string("scope"),
        &key,
        &ctx.string("reference"),
        &secret_manager_access_id,
        &ctx.string("mount-path"),
    ));
    qovery_log::println(format!(
        "External secret {} has been created",
        FG_BLUE.sprint(&key)
    ));
}

fn run_delete(ctx: &Ctx) {
    let (client, _, _, _, terraform) = prelude(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &terraform.id,
        ServiceType::Terraform,
        &key,
    ));
    qovery_log::println(format!(
        "External secret {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn run_update(ctx: &Ctx) {
    let (client, organization_id, _, env_id, terraform) = prelude(ctx);
    let secret_manager_access_id =
        check_error(qovery_envvars::get_secret_manager_access_id_by_name(
            &client,
            &organization_id,
            &env_id,
            &ctx.string("secret-manager-access-name"),
        ));
    let key = ctx.string("key");
    check_error(qovery_envvars::update_service_external_secret(
        &client,
        &key,
        &ctx.string("reference"),
        &secret_manager_access_id,
        &terraform.id,
        ServiceType::Terraform,
    ));
    qovery_log::println(format!(
        "External secret {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}
