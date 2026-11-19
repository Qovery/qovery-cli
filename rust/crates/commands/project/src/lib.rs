//! `qovery project` command. Port of `cmd/project*.go`.

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, find::find_by_name, resources, telemetry};
use qovery_log::gofmt::go_json;
use qovery_log::pterm::FG_BLUE;
use serde_json::Value;

/// The `project` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("project")
        .short("Manage Project")
        .subcommand(list())
        .subcommand(env())
}

fn list() -> Command {
    Command::new("list")
        .short("List projects")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let organization_id = check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let projects: Vec<Value> =
        check_error(client.list(&format!("/organization/{organization_id}/project")));

    if ctx.bool("json") {
        qovery_log::println(go_json(&Value::Array(projects)));
        return;
    }

    let data: Vec<Vec<String>> = projects
        .iter()
        .map(|p| vec![str_field(p, "id"), str_field(p, "name")])
        .collect();
    qovery_log::print_table(&["Id", "Name"], &data);
}

fn str_field(v: &Value, name: &str) -> String {
    v[name].as_str().unwrap_or_default().to_string()
}

/// Go group commands with a `Run` printing the help when called without args.
fn run_help(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        qovery_core::exit(0);
    }
}

fn env() -> Command {
    Command::new("env")
        .short("Manage project variables and secrets")
        .run(run_help)
        .subcommand(env_alias())
        .subcommand(env_create())
        .subcommand(env_delete())
        .subcommand(env_list())
        .subcommand(env_update())
}

fn env_alias() -> Command {
    Command::new("alias")
        .short("Manage project variable and secret aliases")
        .run(run_help)
        .subcommand(
            Command::new("create")
                .short("Create project variable or secret alias")
                .flag(Flag::string("organization", "Organization Name"))
                .flag(Flag::string("project", "Project Name").required())
                .flag(
                    Flag::string("key", "Project variable or secret key")
                        .short('k')
                        .required(),
                )
                .flag(Flag::string("alias", "Project variable or secret alias").required())
                .run(run_env_alias_create),
        )
}

fn env_create() -> Command {
    Command::new("create")
        .short("Create project variable or secret")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name").required())
        .flag(
            Flag::string("key", "Project variable or secret key")
                .short('k')
                .required(),
        )
        .flag(
            Flag::string("value", "Project variable or secret value")
                .short('v')
                .required(),
        )
        .flag(Flag::bool("secret", "This Project variable is a secret"))
        .run(run_env_create)
}

fn env_delete() -> Command {
    Command::new("delete")
        .short("Delete project variable or secret")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name").required())
        .flag(
            Flag::string("key", "Project variable or secret key")
                .short('k')
                .required(),
        )
        .run(run_env_delete)
}

fn env_list() -> Command {
    Command::new("list")
        .short("List project variables")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name").required())
        .flag(Flag::bool("show-values", "Show env var values"))
        .flag(Flag::bool("pretty-print", "Pretty print output"))
        .flag(Flag::bool("sort", "Sort environment variables by key"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_env_list)
}

fn env_update() -> Command {
    Command::new("update")
        .short("Update project variable or secret")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name").required())
        .flag(
            Flag::string("key", "Project variable or secret key")
                .short('k')
                .required(),
        )
        .flag(
            Flag::string("value", "Project variable or secret value")
                .short('v')
                .required(),
        )
        .run(run_env_update)
}

/// Common prologue of the `project env` commands: `getOrganizationProjectEnvironmentContextResourcesIds`,
/// `ListProject` and `FindByProjectName(projects, projectName)`. Returns the client and the project id.
fn resolve_project(ctx: &Ctx) -> (qovery_api::Client, String) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, _, _) = check_error(resources::ids_from_flags(&client, ctx));
    let projects: Vec<Value> =
        check_error(client.list(&format!("/organization/{organization_id}/project")));
    let project_id = match find_by_name(&projects, &ctx.string("project")) {
        Some(p) => str_field(p, "id"),
        // Go dereferences the nil *qovery.Project returned by FindByProjectName (only reachable with a blank
        // `--project`: the project id is then resolved from the context but looked up by its blank name)
        None => qovery_envvars::go_nil_pointer_panic(),
    };
    (client, project_id)
}

fn run_env_alias_create(ctx: &Ctx) {
    let (client, project_id) = resolve_project(ctx);
    check_error(qovery_envvars::create_project_alias(
        &client,
        &project_id,
        &ctx.string("key"),
        &ctx.string("alias"),
    ));
    qovery_log::println(format!(
        "Alias {} has been created",
        FG_BLUE.sprint(ctx.string("alias"))
    ));
}

fn run_env_create(ctx: &Ctx) {
    let (client, project_id) = resolve_project(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_project_variable(
        &client,
        &project_id,
        &key,
        &ctx.string("value"),
        ctx.bool("secret"),
    ));
    qovery_log::println(format!(
        "Project variable {} has been created",
        FG_BLUE.sprint(&key)
    ));
}

fn run_env_delete(ctx: &Ctx) {
    let (client, project_id) = resolve_project(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_project_var(
        &client,
        &project_id,
        &key,
    ));
    qovery_log::println(format!(
        "Variable {} has been deleted",
        FG_BLUE.sprint(&key)
    ));
}

fn run_env_list(ctx: &Ctx) {
    let (client, project_id) = resolve_project(ctx);
    let env_vars = check_error(qovery_envvars::list_project_variables(&client, &project_id));

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

fn run_env_update(ctx: &Ctx) {
    let (client, project_id) = resolve_project(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::update_project_variable(
        &client,
        &project_id,
        &key,
        &ctx.string("value"),
    ));
    qovery_log::println(format!(
        "Project variable {} has been updated",
        FG_BLUE.sprint(&key)
    ));
}
