//! `environment env ...` and `environment external-secret ...` (`cmd/environment_env*.go`,
//! `cmd/environment_external_secret*.go`).

use crate::{ope_flags, run_group};
use qovery_api::Client;
use qovery_api::models::{Environment, Project};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::find_by_name;
use qovery_core::resources::ids_from_flags;
use qovery_core::{check_error, client, exit, telemetry};
use qovery_log::pterm::FG_BLUE;
use qovery_services::path_escape;

pub(crate) fn env_command() -> Command {
    Command::new("env")
        .short("Manage environment variables and secrets")
        .run(run_group)
        .subcommand(
            Command::new("alias")
                .short("Manage environment variable and secret aliases")
                .run(run_group)
                .subcommand(alias_create()),
        )
        .subcommand(env_create())
        .subcommand(env_delete())
        .subcommand(env_list())
        .subcommand(
            Command::new("override")
                .short("Manage environment variable and secret overrides")
                .run(run_group)
                .subcommand(override_create()),
        )
        .subcommand(env_update())
}

pub(crate) fn external_secret_command() -> Command {
    Command::new("external-secret")
        .short("Manage environment external secrets")
        .run(run_group)
        .subcommand(external_secret_create())
        .subcommand(external_secret_delete())
        .subcommand(external_secret_update())
}

fn key_flag(usage: &str) -> Flag {
    Flag::string("key", usage).short('k').required()
}

/// `--project` and `--environment` are required by all these commands.
fn required_ope_flags() -> [Flag; 3] {
    let [o, p, e] = ope_flags();
    [o, p.required(), e.required()]
}

/// Common lookup of these commands: organization id from the flags/context, then the project and the
/// environment by name (`ListProject` + `FindByProjectName`, `ListEnvironment` + `FindByEnvironmentName`).
/// `check_project`: the external-secret commands report a missing project; the env commands dereference
/// the nil project (Go panic).
fn resolve(ctx: &Ctx, client: &Client, check_project: bool) -> (String, Project, Environment) {
    let (organization_id, _, _) = check_error(ids_from_flags(client, ctx));

    let projects: Vec<Project> = check_error(client.list(&format!(
        "/organization/{}/project",
        path_escape(&organization_id)
    )));
    let project_name = ctx.string("project");
    let project = match find_by_name(&projects, &project_name) {
        Some(p) => p.clone(),
        None if check_project => {
            qovery_log::println_error(format!("project {project_name} not found"));
            qovery_log::println_info("You can list all projects with: qovery project list");
            exit(1);
        }
        None => qovery_envvars::go_nil_pointer_panic(),
    };

    let environments: Vec<Environment> = check_error(client.list(&format!(
        "/project/{}/environment",
        path_escape(&project.id)
    )));
    let environment_name = ctx.string("environment");
    let Some(environment) = find_by_name(&environments, &environment_name).cloned() else {
        qovery_log::println_error(format!("environment {environment_name} not found"));
        qovery_log::println_info("You can list all environments with: qovery environment list");
        exit(1);
    };

    (organization_id, project, environment)
}

/// Go's package-level `projectId` variable, never set by these commands.
const GO_PROJECT_ID: &str = "";

// ---------------------------------------------------------------------------------------------------------------
// env

fn env_create() -> Command {
    Command::new("create")
        .short("Create environment variable or secret")
        .flags(required_ope_flags())
        .flag(key_flag("Environment variable or secret key"))
        .flag(Flag::string("value", "Environment variable or secret value").short('v').required())
        .flag(Flag::string("scope", "Scope of this env var <PROJECT|ENVIRONMENT>").default_value("ENVIRONMENT"))
        .flag(Flag::bool("secret", "This environment variable is a secret"))
        .flag(Flag::bool(
            "override-if-exists",
            "If the key already exists at a higher scope, create an override of it instead of failing",
        ))
        .run(run_env_create)
}

fn run_env_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, false);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_environment_variable(
        &client,
        GO_PROJECT_ID,
        &environment.id,
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
    Command::new("delete")
        .short("Delete environment variable or secret")
        .flags(required_ope_flags())
        .flag(key_flag("Environment variable or secret key"))
        .run(run_env_delete)
}

fn run_env_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, false);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_environment_var(
        &client,
        &environment.id,
        &key,
    ));
    qovery_log::println(format!(
        "Variable {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn env_list() -> Command {
    Command::new("list")
        .short("List environment variables")
        .flags(required_ope_flags())
        .flag(Flag::bool("show-values", "Show env var values"))
        .flag(Flag::bool("pretty-print", "Pretty print output"))
        .flag(Flag::bool("sort", "Sort environment variables by key"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_env_list)
}

fn run_env_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, false);

    let env_vars = check_error(qovery_envvars::list_environment_variables(
        &client,
        &environment.id,
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

fn env_update() -> Command {
    Command::new("update")
        .short("Update environment variable or secret")
        .flags(required_ope_flags())
        .flag(key_flag("Environment variable or secret key"))
        .flag(
            Flag::string("value", "Environment variable or secret value")
                .short('v')
                .required(),
        )
        .run(run_env_update)
}

fn run_env_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, false);
    let key = ctx.string("key");
    check_error(qovery_envvars::update_environment_variable(
        &client,
        &environment.id,
        &key,
        &ctx.string("value"),
    ));
    qovery_log::println(format!(
        "Environment variable {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}

fn alias_create() -> Command {
    Command::new("create")
        .short("Create environment variable or secret alias")
        .flags(required_ope_flags())
        .flag(key_flag("Environment variable or secret key"))
        .flag(Flag::string("alias", "Environment variable or secret alias").required())
        .flag(
            Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT>")
                .default_value("ENVIRONMENT"),
        )
        .run(run_alias_create)
}

fn run_alias_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, false);
    let alias = ctx.string("alias");
    check_error(qovery_envvars::create_environment_alias(
        &client,
        GO_PROJECT_ID,
        &environment.id,
        &ctx.string("key"),
        &alias,
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("Alias {} has been created", FG_BLUE.sprint(&alias)));
}

fn override_create() -> Command {
    Command::new("create")
        .short("Override environment variable or secret")
        .flags(required_ope_flags())
        .flag(key_flag("Environment variable or secret key"))
        .flag(Flag::string("value", "Environment variable or secret value").required())
        .flag(
            Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT>")
                .default_value("ENVIRONMENT"),
        )
        .run(run_override_create)
}

fn run_override_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, false);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_environment_override(
        &client,
        GO_PROJECT_ID,
        &environment.id,
        &key,
        &ctx.string("value"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("{} has been overidden", FG_BLUE.sprint(&key)));
}

// ---------------------------------------------------------------------------------------------------------------
// external-secret

fn external_secret_create() -> Command {
    Command::new("create")
        .short("Create environment external secret")
        .flags(required_ope_flags())
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
                "Scope of this external secret <PROJECT|ENVIRONMENT>",
            )
            .default_value("ENVIRONMENT"),
        )
        .flag(Flag::string(
            "mount-path",
            "Path where the secret will be mounted as a file",
        ))
        .run(run_external_secret_create)
}

fn run_external_secret_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, project, environment) = resolve(ctx, &client, true);

    let secret_manager_access_id =
        check_error(qovery_envvars::get_secret_manager_access_id_by_name(
            &client,
            &organization_id,
            &environment.id,
            &ctx.string("secret-manager-access-name"),
        ));

    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_external_secret(
        &client,
        &project.id,
        &environment.id,
        "",
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
    Command::new("delete")
        .short("Delete environment external secret")
        .flags(required_ope_flags())
        .flag(key_flag("External secret key"))
        .run(run_external_secret_delete)
}

fn run_external_secret_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment) = resolve(ctx, &client, true);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_environment_var(
        &client,
        &environment.id,
        &key,
    ));
    qovery_log::println(format!(
        "External secret {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn external_secret_update() -> Command {
    Command::new("update")
        .short("Update environment external secret")
        .flags(required_ope_flags())
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
    let client = check_error(client::get_client());
    let (organization_id, _, environment) = resolve(ctx, &client, true);

    let secret_manager_access_id =
        check_error(qovery_envvars::get_secret_manager_access_id_by_name(
            &client,
            &organization_id,
            &environment.id,
            &ctx.string("secret-manager-access-name"),
        ));

    let key = ctx.string("key");
    check_error(qovery_envvars::update_environment_external_secret(
        &client,
        &environment.id,
        &key,
        &ctx.string("reference"),
        &secret_manager_access_id,
    ));
    qovery_log::println(format!(
        "External secret {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}
