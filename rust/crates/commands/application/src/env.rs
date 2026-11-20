//! `application env ...` and `application external-secret ...`.

use crate::{application_flag, context_flags, find_application_or_exit, run_help};
use qovery_api::Client;
use qovery_api::models::Application;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::{check_error, client, resources, telemetry};
use qovery_envvars as envvars;
use qovery_log::pterm::FG_BLUE;

const APP: ServiceType = ServiceType::Application;

pub(crate) fn command() -> Command {
    Command::new("env")
        .short("Manage application environment variables and secrets")
        .run(run_help)
        .subcommand(alias())
        .subcommand(create())
        .subcommand(delete())
        .subcommand(list())
        .subcommand(override_cmd())
        .subcommand(update())
}

fn key_flag(usage: &str) -> Flag {
    Flag::string("key", usage).short('k')
}

fn scope_flag(usage: &str) -> Flag {
    Flag::string("scope", usage).default_value("APPLICATION")
}

/// Common start of the sub-commands: client, (organization, project, environment) ids and the application.
fn start(ctx: &Ctx) -> (Client, (String, String, String), Application) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let ids = check_error(resources::ids_from_flags(&client, ctx));
    let application = find_application_or_exit(&client, &ids.2, &ctx.string("application"));
    (client, ids, application)
}

// ---- env alias ------------------------------------------------------------------------------------------------

fn alias() -> Command {
    Command::new("alias")
        .short("Manage application environment variable and secret aliases")
        .run(run_help)
        .subcommand(
            context_flags(
                Command::new("create")
                    .short("Create application environment variable or secret alias"),
            )
            .flag(application_flag().required())
            .flag(key_flag("Environment variable or secret key").required())
            .flag(Flag::string("alias", "Environment variable or secret alias").required())
            .flag(scope_flag(
                "Scope of this alias <PROJECT|ENVIRONMENT|APPLICATION>",
            ))
            .run(run_alias_create),
        )
}

fn run_alias_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), application) = start(ctx);
    let alias = ctx.string("alias");
    check_error(envvars::create_service_alias(
        &client,
        &project_id,
        &env_id,
        &application.id,
        APP,
        &ctx.string("key"),
        &alias,
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("Alias {} has been created", FG_BLUE.sprint(&alias)));
}

// ---- env create -----------------------------------------------------------------------------------------------

fn create() -> Command {
    context_flags(Command::new("create").short("Create application environment variable or secret"))
        .flag(application_flag().required())
        .flag(key_flag("Environment variable or secret key").required())
        .flag(Flag::string("value", "Environment variable or secret value").short('v').required())
        .flag(scope_flag("Scope of this env var <PROJECT|ENVIRONMENT|APPLICATION>"))
        .flag(Flag::bool("secret", "This environment variable is a secret"))
        .flag(Flag::bool(
            "override-if-exists",
            "If the key already exists at a higher scope, create an override of it instead of failing",
        ))
        .run(run_create)
}

fn run_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), application) = start(ctx);
    let key = ctx.string("key");
    check_error(envvars::create_service_variable(
        &client,
        &project_id,
        &env_id,
        &application.id,
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

// ---- env delete -----------------------------------------------------------------------------------------------

fn delete() -> Command {
    context_flags(Command::new("delete").short("Delete application environment variable or secret"))
        .flag(application_flag().required())
        .flag(key_flag("Environment variable or secret key").required())
        .run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    let (client, _, application) = start(ctx);
    let key = ctx.string("key");
    check_error(envvars::delete_service_variable(
        &client,
        &application.id,
        APP,
        &key,
    ));
    qovery_log::println(format!(
        "Variable {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

// ---- env list -------------------------------------------------------------------------------------------------

fn list() -> Command {
    context_flags(Command::new("list").short("List application environment variables"))
        .flag(application_flag().required())
        .flag(Flag::bool("show-values", "Show env var values"))
        .flag(Flag::bool("pretty-print", "Pretty print output"))
        .flag(Flag::bool("sort", "Sort environment variables by key"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    let (client, _, application) = start(ctx);
    let env_vars = check_error(envvars::list_service_variables(
        &client,
        &application.id,
        APP,
    ));

    let mut lines = envvars::EnvVarLines::new();
    let mut variables = Vec::new();
    for v in &env_vars {
        let s = envvars::from_environment_variable_to_env_var_line_output(v);
        variables.push(s.clone());
        lines.add(s);
    }

    if ctx.bool("json") {
        qovery_log::println(envvars::get_env_var_json_output(
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

// ---- env override ---------------------------------------------------------------------------------------------

fn override_cmd() -> Command {
    Command::new("override")
        .short("Manage application environment variable and secret overrides")
        .run(run_help)
        .subcommand(
            context_flags(
                Command::new("create").short("Override application environment variable or secret"),
            )
            .flag(application_flag().required())
            .flag(key_flag("Environment variable or secret key").required())
            .flag(Flag::string("value", "Environment variable or secret value").required())
            .flag(scope_flag(
                "Scope of this alias <PROJECT|ENVIRONMENT|APPLICATION>",
            ))
            .run(run_override_create),
        )
}

fn run_override_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), application) = start(ctx);
    let key = ctx.string("key");
    check_error(envvars::create_service_override(
        &client,
        &project_id,
        &env_id,
        &application.id,
        APP,
        &key,
        &ctx.string("value"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("{} has been overidden", FG_BLUE.sprint(&key)));
}

// ---- env update -----------------------------------------------------------------------------------------------

fn update() -> Command {
    context_flags(Command::new("update").short("Update application environment variable or secret"))
        .flag(application_flag().required())
        .flag(key_flag("Environment variable or secret key").required())
        .flag(
            Flag::string("value", "Environment variable or secret value")
                .short('v')
                .required(),
        )
        .run(run_update)
}

fn run_update(ctx: &Ctx) {
    let (client, _, application) = start(ctx);
    let key = ctx.string("key");
    check_error(envvars::update_service_variable(
        &client,
        &key,
        &ctx.string("value"),
        &application.id,
        APP,
    ));
    qovery_log::println(format!(
        "Environment variable {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}

// ---- external-secret ------------------------------------------------------------------------------------------

pub(crate) fn external_secret() -> Command {
    Command::new("external-secret")
        .short("Manage application external secrets")
        .run(run_help)
        .subcommand(
            context_flags(Command::new("create").short("Create application external secret"))
                .flag(application_flag().required())
                .flag(key_flag("External secret key").required())
                .flag(
                    Flag::string(
                        "reference",
                        "Reference to the secret in the secrets provider",
                    )
                    .short('r')
                    .required(),
                )
                .flag(
                    Flag::string("secret-manager-access-name", "Secret manager access name")
                        .required(),
                )
                .flag(scope_flag(
                    "Scope of this external secret <PROJECT|ENVIRONMENT|APPLICATION>",
                ))
                .flag(Flag::string(
                    "mount-path",
                    "Path where the secret will be mounted as a file",
                ))
                .run(run_external_secret_create),
        )
        .subcommand(
            context_flags(Command::new("delete").short("Delete application external secret"))
                .flag(application_flag().required())
                .flag(key_flag("External secret key").required())
                .run(run_external_secret_delete),
        )
        .subcommand(
            context_flags(Command::new("update").short("Update application external secret"))
                .flag(application_flag().required())
                .flag(key_flag("External secret key").required())
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
                .run(run_external_secret_update),
        )
}

fn run_external_secret_create(ctx: &Ctx) {
    let (client, (organization_id, project_id, env_id), application) = start(ctx);
    let secret_manager_access_id = check_error(envvars::get_secret_manager_access_id_by_name(
        &client,
        &organization_id,
        &env_id,
        &ctx.string("secret-manager-access-name"),
    ));
    let key = ctx.string("key");
    check_error(envvars::create_service_external_secret(
        &client,
        &project_id,
        &env_id,
        &application.id,
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

fn run_external_secret_delete(ctx: &Ctx) {
    let (client, _, application) = start(ctx);
    let key = ctx.string("key");
    check_error(envvars::delete_service_variable(
        &client,
        &application.id,
        APP,
        &key,
    ));
    qovery_log::println(format!(
        "External secret {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn run_external_secret_update(ctx: &Ctx) {
    let (client, (organization_id, _, env_id), application) = start(ctx);
    let secret_manager_access_id = check_error(envvars::get_secret_manager_access_id_by_name(
        &client,
        &organization_id,
        &env_id,
        &ctx.string("secret-manager-access-name"),
    ));
    let key = ctx.string("key");
    check_error(envvars::update_service_external_secret(
        &client,
        &key,
        &ctx.string("reference"),
        &secret_manager_access_id,
        &application.id,
        APP,
    ));
    qovery_log::println(format!(
        "External secret {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}
