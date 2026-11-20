//! `qovery cronjob env ...` and `qovery cronjob external-secret ...`.

use crate::{context_flags, cronjob_flag, find_cronjob_or_exit, list_jobs, run_group};
use qovery_api::Client;
use qovery_api::models::Job;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::{check_error, client, resources, telemetry};
use qovery_log::pterm::FG_BLUE;

pub(crate) fn env() -> Command {
    Command::new("env")
        .short("Manage cronjob environment variables and secrets")
        .run(run_group)
        .subcommand(
            Command::new("alias")
                .short("Manage cronjob environment variable and secret aliases")
                .run(run_group)
                .subcommand(alias_create()),
        )
        .subcommand(env_create())
        .subcommand(env_delete())
        .subcommand(env_list())
        .subcommand(
            Command::new("override")
                .short("Manage cronjob environment variable and secret overrides")
                .run(run_group)
                .subcommand(override_create()),
        )
        .subcommand(env_update())
}

pub(crate) fn external_secret() -> Command {
    Command::new("external-secret")
        .short("Manage cronjob external secrets")
        .run(run_group)
        .subcommand(external_secret_create())
        .subcommand(external_secret_delete())
        .subcommand(external_secret_update())
}

fn key_flag(usage: &str) -> Flag {
    Flag::string("key", usage).short('k').required()
}

fn base(name: &str, short: &str) -> Command {
    Command::new(name)
        .short(short)
        .flags(context_flags())
        .flag(cronjob_flag().required())
}

/// Common start: client, (organization, project, environment) ids, the cron job named by `--cronjob`.
fn resolve(ctx: &Ctx) -> (Client, (String, String, String), Job) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let ids = check_error(resources::ids_from_flags(&client, ctx));
    let jobs = check_error(list_jobs(&client, &ids.2));
    let cronjob = find_cronjob_or_exit(&jobs, &ctx.string("cronjob"));
    (client, ids, cronjob)
}

// ---- env --------------------------------------------------------------------------------------------------

fn alias_create() -> Command {
    base(
        "create",
        "Create cronjob environment variable or secret alias",
    )
    .flag(key_flag("Environment variable or secret key"))
    .flag(Flag::string("alias", "Environment variable or secret alias").required())
    .flag(
        Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT|JOB>").default_value("JOB"),
    )
    .run(run_alias_create)
}

fn run_alias_create(ctx: &Ctx) {
    let (client, (_, _, env_id), cronjob) = resolve(ctx);
    let alias = ctx.string("alias");
    // Go passes the package-level `projectId` variable (cmd/admin.go), always empty here
    check_error(qovery_envvars::create_service_alias(
        &client,
        "",
        &env_id,
        &cronjob.id,
        ServiceType::Job,
        &ctx.string("key"),
        &alias,
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("Alias {} has been created", FG_BLUE.sprint(&alias)));
}

fn env_create() -> Command {
    base("create", "Create cronjob environment variable or secret")
        .flag(key_flag("Environment variable or secret key"))
        .flag(Flag::string("value", "Environment variable or secret value").short('v').required())
        .flag(Flag::string("scope", "Scope of this env var <PROJECT|ENVIRONMENT|JOB>").default_value("JOB"))
        .flag(Flag::bool("secret", "This environment variable is a secret"))
        .flag(Flag::bool(
            "override-if-exists",
            "If the key already exists at a higher scope, create an override of it instead of failing",
        ))
        .run(run_env_create)
}

fn run_env_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), cronjob) = resolve(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_variable(
        &client,
        &project_id,
        &env_id,
        &cronjob.id,
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

fn env_delete() -> Command {
    base("delete", "Delete cronjob environment variable or secret")
        .flag(key_flag("Environment variable or secret key"))
        .run(run_env_delete)
}

fn run_env_delete(ctx: &Ctx) {
    let (client, _, cronjob) = resolve(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &cronjob.id,
        ServiceType::Job,
        &key,
    ));
    qovery_log::println(format!(
        "Variable {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn env_list() -> Command {
    base("list", "List cronjob environment variables")
        .flag(Flag::bool("show-values", "Show env var values"))
        .flag(Flag::bool("pretty-print", "Pretty print output"))
        .flag(Flag::bool("sort", "Sort environment variables by key"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_env_list)
}

fn run_env_list(ctx: &Ctx) {
    let (client, _, cronjob) = resolve(ctx);
    let env_vars = check_error(qovery_envvars::list_service_variables(
        &client,
        &cronjob.id,
        ServiceType::Job,
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

fn override_create() -> Command {
    base("create", "Override cronjob environment variable or secret")
        .flag(key_flag("Environment variable or secret key"))
        .flag(Flag::string("value", "Environment variable or secret value").required())
        .flag(
            Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT|JOB>")
                .default_value("JOB"),
        )
        .run(run_override_create)
}

fn run_override_create(ctx: &Ctx) {
    let (client, (_, project_id, env_id), cronjob) = resolve(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_override(
        &client,
        &project_id,
        &env_id,
        &cronjob.id,
        ServiceType::Job,
        &key,
        &ctx.string("value"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("{} has been overidden", FG_BLUE.sprint(&key)));
}

fn env_update() -> Command {
    base("update", "Update cronjob environment variable or secret")
        .flag(key_flag("Environment variable or secret key"))
        .flag(
            Flag::string("value", "Environment variable or secret value")
                .short('v')
                .required(),
        )
        .run(run_env_update)
}

fn run_env_update(ctx: &Ctx) {
    let (client, _, cronjob) = resolve(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::update_service_variable(
        &client,
        &key,
        &ctx.string("value"),
        &cronjob.id,
        ServiceType::Job,
    ));
    qovery_log::println(format!(
        "Environment variable {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}

// ---- external-secret --------------------------------------------------------------------------------------

fn external_secret_create() -> Command {
    base("create", "Create cronjob external secret")
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
                "Scope of this external secret <PROJECT|ENVIRONMENT|JOB>",
            )
            .default_value("JOB"),
        )
        .flag(Flag::string(
            "mount-path",
            "Path where the secret will be mounted as a file",
        ))
        .run(run_external_secret_create)
}

fn run_external_secret_create(ctx: &Ctx) {
    let (client, (organization_id, project_id, env_id), cronjob) = resolve(ctx);
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
        &cronjob.id,
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

fn external_secret_delete() -> Command {
    base("delete", "Delete cronjob external secret")
        .flag(key_flag("External secret key"))
        .run(run_external_secret_delete)
}

fn run_external_secret_delete(ctx: &Ctx) {
    let (client, _, cronjob) = resolve(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &cronjob.id,
        ServiceType::Job,
        &key,
    ));
    qovery_log::println(format!(
        "External secret {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn external_secret_update() -> Command {
    base("update", "Update cronjob external secret")
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
    let (client, (organization_id, _, env_id), cronjob) = resolve(ctx);
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
        &cronjob.id,
        ServiceType::Job,
    ));
    qovery_log::println(format!(
        "External secret {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}
