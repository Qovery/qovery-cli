//! `helm env ...` (helm_env*.go) and `helm external-secret ...` (helm_external_secret*.go).

use crate::{context_flags, run_group};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::{check_error, telemetry};
use qovery_log::pterm::FG_BLUE;

pub(crate) fn command() -> Command {
    Command::new("env")
        .short("Manage helm environment variables and secrets")
        .run(run_group)
        .subcommand(list())
        .subcommand(create())
        .subcommand(update())
        .subcommand(delete())
        .subcommand(
            Command::new("alias")
                .short("Manage helm environment variable and secret aliases")
                .run(run_group)
                .subcommand(alias_create()),
        )
        .subcommand(
            Command::new("override")
                .short("Manage helm environment variable and secret overrides")
                .run(run_group)
                .subcommand(override_create()),
        )
}

pub(crate) fn external_secret_command() -> Command {
    Command::new("external-secret")
        .short("Manage helm external secrets")
        .run(run_group)
        .subcommand(external_secret_create())
        .subcommand(external_secret_update())
        .subcommand(external_secret_delete())
}

fn helm_flag(cmd: Command, usage: &str) -> Command {
    context_flags(cmd).flag(Flag::string("helm", usage).short('n').required())
}

fn key_flag(usage: &str) -> Flag {
    Flag::string("key", usage).short('k').required()
}

fn blue_key(ctx: &Ctx) -> String {
    FG_BLUE.sprint(ctx.string("key"))
}

// ---------------------------------------------------------------------------------------------------------
// env list

fn list() -> Command {
    helm_flag(
        Command::new("list").short("List helm environment variables"),
        "helm Name",
    )
    .flag(Flag::bool("show-values", "Show env var values"))
    .flag(Flag::bool("pretty-print", "Pretty print output"))
    .flag(Flag::bool("sort", "Sort environment variables by key"))
    .flag(Flag::bool("json", "JSON output"))
    .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    let env_vars = check_error(qovery_envvars::list_service_variables(
        &client,
        &helm.id,
        ServiceType::Helm,
    ));

    let mut lines = qovery_envvars::EnvVarLines::new();
    let mut variables = Vec::new();
    for v in &env_vars {
        let s = qovery_envvars::from_environment_variable_to_env_var_line_output(v);
        variables.push(s.clone());
        lines.add(s);
    }

    if ctx.bool("json") {
        qovery_log::println(qovery_envvars::get_env_var_json_output(
            &variables,
            ctx.bool("sort"),
        ));
        return;
    }
    qovery_log::print_table(
        &lines.header(ctx.bool("pretty-print")),
        &lines.lines(
            ctx.bool("show-values"),
            ctx.bool("pretty-print"),
            ctx.bool("sort"),
        ),
    );
}

// ---------------------------------------------------------------------------------------------------------
// env create / update / delete

fn create() -> Command {
    helm_flag(
        Command::new("create").short("Create helm environment variable or secret"),
        "helm Name",
    )
    .flag(key_flag("Environment variable or secret key"))
    .flag(
        Flag::string("value", "Environment variable or secret value")
            .short('v')
            .required(),
    )
    .flag(
        Flag::string("scope", "Scope of this env var <PROJECT|ENVIRONMENT|HELM>")
            .default_value("HELM"),
    )
    .flag(Flag::bool(
        "secret",
        "This environment variable is a secret",
    ))
    .flag(Flag::bool(
        "override-if-exists",
        "If the key already exists at a higher scope, create an override of it instead of failing",
    ))
    .run(run_create)
}

fn run_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, (_, project_id, env_id), helm) = crate::resolve_helm(ctx);
    check_error(qovery_envvars::create_service_variable(
        &client,
        &project_id,
        &env_id,
        &helm.id,
        &ctx.string("scope"),
        &ctx.string("key"),
        &ctx.string("value"),
        ctx.bool("secret"),
        ctx.bool("override-if-exists"),
    ));
    qovery_log::println(format!(
        "Environment variable {} has been created",
        blue_key(ctx)
    ));
}

fn update() -> Command {
    helm_flag(
        Command::new("update").short("Update helm environment variable or secret"),
        "helm Name",
    )
    .flag(key_flag("Environment variable or secret key"))
    .flag(
        Flag::string("value", "Environment variable or secret value")
            .short('v')
            .required(),
    )
    .run(run_update)
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    check_error(qovery_envvars::update_service_variable(
        &client,
        &ctx.string("key"),
        &ctx.string("value"),
        &helm.id,
        ServiceType::Helm,
    ));
    qovery_log::println(format!(
        "Environment variable {} has been updated",
        blue_key(ctx)
    ));
}

fn delete() -> Command {
    helm_flag(
        Command::new("delete").short("Delete helm environment variable or secret"),
        "helm Name",
    )
    .flag(key_flag("Environment variable or secret key"))
    .run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &helm.id,
        ServiceType::Helm,
        &ctx.string("key"),
    ));
    qovery_log::println(format!("Variable {} has been deleted", blue_key(ctx)));
}

// ---------------------------------------------------------------------------------------------------------
// env alias create / env override create

fn alias_create() -> Command {
    helm_flag(
        Command::new("create").short("Create helm environment variable or secret alias"),
        "helm Name",
    )
    .flag(key_flag("Environment variable or secret key"))
    .flag(Flag::string("alias", "Environment variable or secret alias").required())
    .flag(
        Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT|HELM>")
            .default_value("HELM"),
    )
    .run(run_alias_create)
}

fn run_alias_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, (_, _, env_id), helm) = crate::resolve_helm(ctx);
    // The Go code passes the package-level `projectId` variable (cmd/admin.go), which is always empty here.
    check_error(qovery_envvars::create_service_alias(
        &client,
        "",
        &env_id,
        &helm.id,
        ServiceType::Helm,
        &ctx.string("key"),
        &ctx.string("alias"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!(
        "Alias {} has been created",
        FG_BLUE.sprint(ctx.string("alias"))
    ));
}

fn override_create() -> Command {
    helm_flag(
        Command::new("create").short("Override helm environment variable or secret"),
        "helm Name",
    )
    .flag(key_flag("Environment variable or secret key"))
    .flag(Flag::string(
        "value",
        "Environment variable or secret value",
    ))
    .flag(
        Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT|HELM>")
            .default_value("HELM"),
    )
    .run(run_override_create)
}

fn run_override_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, (_, project_id, env_id), helm) = crate::resolve_helm(ctx);
    check_error(qovery_envvars::create_service_override(
        &client,
        &project_id,
        &env_id,
        &helm.id,
        ServiceType::Helm,
        &ctx.string("key"),
        &ctx.string("value"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("{} has been overidden", blue_key(ctx)));
}

// ---------------------------------------------------------------------------------------------------------
// external-secret create / update / delete

fn external_secret_create() -> Command {
    helm_flag(
        Command::new("create").short("Create helm external secret"),
        "Helm Name",
    )
    .flag(key_flag("External secret key"))
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
            "Scope of this external secret <PROJECT|ENVIRONMENT|HELM>",
        )
        .default_value("HELM"),
    )
    .flag(Flag::string(
        "mount-path",
        "Path where the secret will be mounted as a file",
    ))
    .run(run_external_secret_create)
}

fn run_external_secret_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, (organization_id, project_id, env_id), helm) = crate::resolve_helm(ctx);
    let secret_manager_access_id =
        check_error(qovery_envvars::get_secret_manager_access_id_by_name(
            &client,
            &organization_id,
            &env_id,
            &ctx.string("secret-manager-access-name"),
        ));
    check_error(qovery_envvars::create_service_external_secret(
        &client,
        &project_id,
        &env_id,
        &helm.id,
        &ctx.string("scope"),
        &ctx.string("key"),
        &ctx.string("reference"),
        &secret_manager_access_id,
        &ctx.string("mount-path"),
    ));
    qovery_log::println(format!(
        "External secret {} has been created",
        blue_key(ctx)
    ));
}

fn external_secret_update() -> Command {
    helm_flag(
        Command::new("update").short("Update helm external secret"),
        "Helm Name",
    )
    .flag(key_flag("External secret key"))
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
    .run(run_external_secret_update)
}

fn run_external_secret_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, (organization_id, _, env_id), helm) = crate::resolve_helm(ctx);
    let secret_manager_access_id =
        check_error(qovery_envvars::get_secret_manager_access_id_by_name(
            &client,
            &organization_id,
            &env_id,
            &ctx.string("secret-manager-access-name"),
        ));
    check_error(qovery_envvars::update_service_external_secret(
        &client,
        &ctx.string("key"),
        &ctx.string("reference"),
        &secret_manager_access_id,
        &helm.id,
        ServiceType::Helm,
    ));
    qovery_log::println(format!(
        "External secret {} has been updated",
        blue_key(ctx)
    ));
}

fn external_secret_delete() -> Command {
    helm_flag(
        Command::new("delete").short("Delete helm external secret"),
        "Helm Name",
    )
    .flag(key_flag("External secret key"))
    .run(run_external_secret_delete)
}

fn run_external_secret_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &helm.id,
        ServiceType::Helm,
        &ctx.string("key"),
    ));
    qovery_log::println(format!(
        "External secret {} has been deleted",
        blue_key(ctx)
    ));
}
