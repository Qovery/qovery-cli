//! `qovery container env ...` and `qovery container external-secret ...`. Port of `cmd/container_env*.go` and
//! `cmd/container_external_secret*.go`.

use crate::{
    client_or_exit, container_flag, find_container_or_exit, ids_or_exit, run_group_help,
    with_context_flags,
};
use qovery_api::Client;
use qovery_api::models::Container;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::{check_error, telemetry};
use qovery_log::pterm::FG_BLUE;

pub fn command() -> Command {
    Command::new("env")
        .short("Manage container environment variables and secrets")
        .run(run_group_help)
        .subcommand(
            Command::new("alias")
                .short("Manage container environment variable and secret aliases")
                .run(run_group_help)
                .subcommand(alias_create()),
        )
        .subcommand(create())
        .subcommand(delete())
        .subcommand(list())
        .subcommand(
            Command::new("override")
                .short("Manage container environment variable and secret overrides")
                .run(run_group_help)
                .subcommand(override_create()),
        )
        .subcommand(update())
}

pub fn external_secret_command() -> Command {
    Command::new("external-secret")
        .short("Manage container external secrets")
        .run(run_group_help)
        .subcommand(external_secret_create())
        .subcommand(external_secret_delete())
        .subcommand(external_secret_update())
}

fn key_flag(usage: &str) -> Flag {
    Flag::string("key", usage).short('k').required()
}

fn scope_flag(usage: &str) -> Flag {
    Flag::string("scope", usage).default_value("CONTAINER")
}

fn list() -> Command {
    with_context_flags(Command::new("list").short("List container environment variables"))
        .flag(container_flag().required())
        .flag(Flag::bool("show-values", "Show env var values"))
        .flag(Flag::bool("pretty-print", "Pretty print output"))
        .flag(Flag::bool("sort", "Sort environment variables by key"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn create() -> Command {
    with_context_flags(
        Command::new("create").short("Create container environment variable or secret"),
    )
    .flag(container_flag().required())
    .flag(key_flag("Environment variable or secret key"))
    .flag(
        Flag::string("value", "Environment variable or secret value")
            .short('v')
            .required(),
    )
    .flag(scope_flag(
        "Scope of this env var <PROJECT|ENVIRONMENT|CONTAINER>",
    ))
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

fn delete() -> Command {
    with_context_flags(
        Command::new("delete").short("Delete container environment variable or secret"),
    )
    .flag(container_flag().required())
    .flag(key_flag("Environment variable or secret key"))
    .run(run_delete)
}

fn update() -> Command {
    with_context_flags(
        Command::new("update").short("Update container environment variable or secret"),
    )
    .flag(container_flag().required())
    .flag(key_flag("Environment variable or secret key"))
    .flag(
        Flag::string("value", "Environment variable or secret value")
            .short('v')
            .required(),
    )
    .run(run_update)
}

fn alias_create() -> Command {
    with_context_flags(
        Command::new("create").short("Create container environment variable or secret alias"),
    )
    .flag(container_flag().required())
    .flag(key_flag("Environment variable or secret key"))
    .flag(Flag::string("alias", "Environment variable or secret alias").required())
    .flag(scope_flag(
        "Scope of this alias <PROJECT|ENVIRONMENT|CONTAINER>",
    ))
    .run(run_alias_create)
}

fn override_create() -> Command {
    with_context_flags(
        Command::new("create").short("Override container environment variable or secret"),
    )
    .flag(container_flag().required())
    .flag(key_flag("Environment variable or secret key"))
    .flag(Flag::string("value", "Environment variable or secret value").required())
    .flag(scope_flag(
        "Scope of this alias <PROJECT|ENVIRONMENT|CONTAINER>",
    ))
    .run(run_override_create)
}

fn external_secret_create() -> Command {
    with_context_flags(Command::new("create").short("Create container external secret"))
        .flag(container_flag().required())
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
        .flag(scope_flag(
            "Scope of this external secret <PROJECT|ENVIRONMENT|CONTAINER>",
        ))
        .flag(Flag::string(
            "mount-path",
            "Path where the secret will be mounted as a file",
        ))
        .run(run_external_secret_create)
}

fn external_secret_delete() -> Command {
    with_context_flags(Command::new("delete").short("Delete container external secret"))
        .flag(container_flag().required())
        .flag(key_flag("External secret key"))
        .run(run_external_secret_delete)
}

fn external_secret_update() -> Command {
    with_context_flags(Command::new("update").short("Update container external secret"))
        .flag(container_flag().required())
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

/// Common prologue: client, (organization, project, environment) ids and the `--container` container.
fn prepare(ctx: &Ctx) -> (Client, (String, String, String), Container) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let ids = ids_or_exit(&client, ctx);
    let container = find_container_or_exit(&client, &ids.2, &ctx.string("container"));
    (client, ids, container)
}

fn run_list(ctx: &Ctx) {
    let (client, _, container) = prepare(ctx);
    let env_vars = check_error(qovery_envvars::list_service_variables(
        &client,
        &container.id,
        ServiceType::Container,
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

fn run_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), container) = prepare(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_variable(
        &client,
        &project_id,
        &env_id,
        &container.id,
        &ctx.string("scope"),
        &key,
        &ctx.string("value"),
        ctx.bool("secret"),
        ctx.bool("override-if-exists"),
    ));
    qovery_log::println(format!(
        "Environment variable {} has been created",
        FG_BLUE.sprint(&key)
    ));
}

fn run_delete(ctx: &Ctx) {
    let (client, _, container) = prepare(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &container.id,
        ServiceType::Container,
        &key,
    ));
    qovery_log::println(format!(
        "Variable {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn run_update(ctx: &Ctx) {
    let (client, _, container) = prepare(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::update_service_variable(
        &client,
        &key,
        &ctx.string("value"),
        &container.id,
        ServiceType::Container,
    ));
    qovery_log::println(format!(
        "Environment variable {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}

fn run_alias_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), container) = prepare(ctx);
    let alias = ctx.string("alias");
    check_error(qovery_envvars::create_service_alias(
        &client,
        &project_id,
        &env_id,
        &container.id,
        ServiceType::Container,
        &ctx.string("key"),
        &alias,
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("Alias {} has been created", FG_BLUE.sprint(&alias)));
}

fn run_override_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), container) = prepare(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_override(
        &client,
        &project_id,
        &env_id,
        &container.id,
        ServiceType::Container,
        &key,
        &ctx.string("value"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("{} has been overidden", FG_BLUE.sprint(&key)));
}

fn run_external_secret_create(ctx: &Ctx) {
    let (client, (organization_id, project_id, env_id), container) = prepare(ctx);
    let sma_id = check_error(qovery_envvars::get_secret_manager_access_id_by_name(
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
        &container.id,
        &ctx.string("scope"),
        &key,
        &ctx.string("reference"),
        &sma_id,
        &ctx.string("mount-path"),
    ));
    qovery_log::println(format!(
        "External secret {} has been created",
        FG_BLUE.sprint(&key)
    ));
}

fn run_external_secret_delete(ctx: &Ctx) {
    let (client, _, container) = prepare(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &container.id,
        ServiceType::Container,
        &key,
    ));
    qovery_log::println(format!(
        "External secret {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn run_external_secret_update(ctx: &Ctx) {
    let (client, (organization_id, _, env_id), container) = prepare(ctx);
    let sma_id = check_error(qovery_envvars::get_secret_manager_access_id_by_name(
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
        &sma_id,
        &container.id,
        ServiceType::Container,
    ));
    qovery_log::println(format!(
        "External secret {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}
