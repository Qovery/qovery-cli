//! `qovery rde blueprint ...`. Port of `cmd/rde_blueprint*.go`.

use crate::helpers::*;
use qovery_api::models::Environment;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit_with_error, status, telemetry};
use qovery_envvars::{
    create_environment_variable, create_project_variable, find_environment_variable_by_key,
    list_environment_variables, list_project_variables,
};
use qovery_log::gofmt::go_json;
use serde_json::{Value, json};
use std::time::Duration;

pub fn command() -> Command {
    Command::new("blueprint")
        .short("Manage RDE blueprints")
        .long(
            "Manage RDE blueprint projects and environments.

A blueprint is a project with a template environment that serves as the source
for cloning new Remote Development Environments. Blueprints are identified by
a project-level environment variable BLUEPRINT_PROJECT_ID.",
        )
        .run(crate::run_group)
        .subcommand(deploy())
        .subcommand(list())
        .subcommand(register())
        .subcommand(status_cmd())
        .subcommand(stop())
        .subcommand(unregister())
}

fn org_flag() -> Flag {
    Flag::string("organization", "Organization Name").short('o')
}

fn deploy() -> Command {
    Command::new("deploy")
        .short("Deploy a blueprint environment")
        .flag(
            Flag::string("project", "Blueprint Project Name")
                .short('p')
                .required(),
        )
        .flag(org_flag())
        .flag(
            Flag::bool(
                "watch",
                "Watch deployment status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_deploy)
}

fn stop() -> Command {
    Command::new("stop")
        .short("Stop a blueprint environment")
        .flag(
            Flag::string("project", "Blueprint Project Name")
                .short('p')
                .required(),
        )
        .flag(org_flag())
        .flag(
            Flag::bool(
                "watch",
                "Watch stop status until it completes or an error occurs",
            )
            .short('w'),
        )
        .run(run_stop)
}

fn list() -> Command {
    Command::new("list")
        .short("List all RDE blueprints")
        .flag(org_flag())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn register() -> Command {
    Command::new("register")
        .short("Register a project as an RDE blueprint")
        .long(
            "Register an existing project as an RDE blueprint by setting the
BLUEPRINT_PROJECT_ID project-level variable and BLUEPRINT_KEY on the
first DEVELOPMENT environment.

The project must already exist and contain at least one environment.",
        )
        .flag(
            Flag::string("project", "Project Name to register as a blueprint")
                .short('p')
                .required(),
        )
        .flag(org_flag())
        .run(run_register)
}

fn status_cmd() -> Command {
    Command::new("status")
        .short("Show detailed status of a blueprint")
        .flag(
            Flag::string("project", "Blueprint Project Name")
                .short('p')
                .required(),
        )
        .flag(org_flag())
        .run(run_status)
}

fn unregister() -> Command {
    Command::new("unregister")
        .short("Unregister a project as an RDE blueprint")
        .long(
            "Remove the BLUEPRINT_PROJECT_ID and BLUEPRINT_KEY environment variables
from the project and its environment. This does NOT delete the project itself.",
        )
        .flag(
            Flag::string("project", "Blueprint Project Name to unregister")
                .short('p')
                .required(),
        )
        .flag(org_flag())
        .run(run_unregister)
}

/// Common start of the deploy / stop / status / unregister commands.
fn resolve_blueprint(ctx: &Ctx) -> (qovery_api::Client, String, BlueprintInfo) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let bp = match find_blueprint_by_project_name(&client, &org, &ctx.string("project")) {
        Ok(bp) => bp,
        Err(e) => exit_with_error(e),
    };
    (client, org, bp)
}

fn run_deploy(ctx: &Ctx) {
    let (client, _, bp) = resolve_blueprint(ctx);
    if bp.env_id.is_empty() {
        exit_with_error(format!(
            "blueprint {} has no environment with {BLUEPRINT_KEY_VAR} set",
            bp.project_name
        ));
    }
    if let Err(e) = deploy_environment(&client, &bp.env_id) {
        exit_with_error(format!("deploy failed: {e}"));
    }
    qovery_log::println(format!(
        "Request to deploy blueprint {} has been queued..",
        blue(&bp.project_name)
    ));
    if ctx.bool("watch") {
        std::thread::sleep(Duration::from_secs(3));
        qovery_services::watch_environment(&bp.env_id, "DEPLOYED", &client);
    }
}

fn run_stop(ctx: &Ctx) {
    let (client, _, bp) = resolve_blueprint(ctx);
    if bp.env_id.is_empty() {
        exit_with_error(format!(
            "blueprint {} has no environment with {BLUEPRINT_KEY_VAR} set",
            bp.project_name
        ));
    }
    if let Err(e) = stop_environment(&client, &bp.env_id) {
        exit_with_error(format!("stop failed: {e}"));
    }
    qovery_log::println(format!(
        "Request to stop blueprint {} has been queued..",
        blue(&bp.project_name)
    ));
    if ctx.bool("watch") {
        std::thread::sleep(Duration::from_secs(3));
        qovery_services::watch_environment(&bp.env_id, "STOPPED", &client);
    }
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let blueprints = check_error(list_blueprint_projects(&client, &org));

    if blueprints.is_empty() {
        qovery_log::println("No RDE blueprints found.");
        return;
    }

    if ctx.bool("json") {
        let mut results = Vec::new();
        for bp in &blueprints {
            let mut status = String::new();
            if !bp.env_id.is_empty()
                && let Ok(s) = get_env_status(&client, &bp.env_id)
            {
                status = s;
            }
            results.push(json!({
                "project_id": bp.project_id,
                "project_name": bp.project_name,
                "env_id": bp.env_id,
                "env_name": bp.env_name,
                "status": status,
            }));
        }
        qovery_log::println(go_json(&Value::Array(results)));
        return;
    }

    let mut data = Vec::new();
    for bp in &blueprints {
        let mut st = "NO_ENV".to_string();
        if !bp.env_id.is_empty()
            && let Ok(s) = get_env_status(&client, &bp.env_id)
        {
            st = status::status_text_with_color(&s);
        }
        let env_name = if bp.env_name.is_empty() {
            "-".to_string()
        } else {
            bp.env_name.clone()
        };
        data.push(vec![
            bp.project_name.clone(),
            env_name,
            st,
            bp.project_id.clone(),
        ]);
    }
    qovery_log::print_table(
        &["Project Name", "Environment", "Status", "Project ID"],
        &data,
    );
    qovery_log::println(format!("\nTotal: {} blueprint(s)", blueprints.len()));
}

fn run_register(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let name = ctx.string("project");

    let project = match find_project_by_name(&client, &org, &name) {
        Ok(p) => p,
        Err(_) => exit_with_error(format!("project {name} not found in organization")),
    };

    if let Ok(vars) = list_project_variables(&client, &project.id)
        && find_environment_variable_by_key(BLUEPRINT_PROJECT_ID_VAR, &vars).is_some()
    {
        qovery_log::println_info(format!(
            "Project {name} is already registered as a blueprint"
        ));
        return;
    }

    qovery_log::println(format!(
        "Step 1/2: Setting {BLUEPRINT_PROJECT_ID_VAR} on project {name}..."
    ));
    if let Err(e) = create_project_variable(
        &client,
        &project.id,
        BLUEPRINT_PROJECT_ID_VAR,
        &project.id,
        false,
    ) {
        exit_with_error(format!(
            "failed to create project variable {BLUEPRINT_PROJECT_ID_VAR}: {e}"
        ));
    }

    qovery_log::println("Step 2/2: Setting BLUEPRINT_KEY on environment...");
    let environments: Vec<Environment> = match list_environments(&client, &project.id) {
        Ok(e) => e,
        Err(e) => exit_with_error(format!("failed to list environments: {e}")),
    };
    if environments.is_empty() {
        exit_with_error(format!(
            "project {name} has no environments - create at least one environment first"
        ));
    }
    let target = environments
        .iter()
        .find(|e| e.mode == "DEVELOPMENT")
        .unwrap_or(&environments[0]);

    if let Err(e) = create_environment_variable(
        &client,
        &project.id,
        &target.id,
        BLUEPRINT_KEY_VAR,
        &project.id,
        false,
        false,
    ) {
        exit_with_error(format!(
            "failed to create environment variable {BLUEPRINT_KEY_VAR}: {e}"
        ));
    }

    qovery_log::println("");
    qovery_log::println("Blueprint registered successfully!");
    qovery_log::println(format!("  Project:     {} ({})", project.name, project.id));
    qovery_log::println(format!("  Environment: {} ({})", target.name, target.id));
    qovery_log::println(format!(
        "  Console:     {}",
        console_url(&org, &project.id, &target.id)
    ));
}

fn run_status(ctx: &Ctx) {
    let (client, org, bp) = resolve_blueprint(ctx);
    let mut rows = vec![
        vec!["Blueprint".to_string(), blue(&bp.project_name)],
        vec!["Project".to_string(), bp.project_id.clone()],
    ];
    if bp.env_id.is_empty() {
        rows.push(vec!["Environment".to_string(), "(none)".to_string()]);
        print_key_value_table(rows);
        return;
    }
    rows.push(vec![
        "Environment".to_string(),
        format!("{} ({})", bp.env_name, bp.env_id),
    ]);
    if let Ok(s) = get_env_status(&client, &bp.env_id) {
        rows.push(vec![
            "Status".to_string(),
            status::status_text_with_color(&s),
        ]);
    }
    rows.push(vec![
        "Console".to_string(),
        console_url(&org, &bp.project_id, &bp.env_id),
    ]);
    if let Ok(children) = list_children(&client, &org, &bp.project_id) {
        rows.push(vec![
            "Children".to_string(),
            format!("{} RDE(s)", children.len()),
        ]);
    }
    print_key_value_table(rows);

    qovery_log::println("");
    print_env_services(&client, &bp.env_id);
}

fn run_unregister(ctx: &Ctx) {
    let (client, _, bp) = resolve_blueprint(ctx);

    qovery_log::println(format!(
        "Removing {BLUEPRINT_PROJECT_ID_VAR} from project {}...",
        bp.project_name
    ));
    if let Ok(vars) = list_project_variables(&client, &bp.project_id)
        && let Some(v) = find_environment_variable_by_key(BLUEPRINT_PROJECT_ID_VAR, &vars)
        && let Err(e) = client.delete(&format!("/variable/{}", esc(&v.id)))
    {
        qovery_log::println_error(format!("failed to delete {BLUEPRINT_PROJECT_ID_VAR}: {e}"));
    }

    if !bp.env_id.is_empty() {
        qovery_log::println(format!(
            "Removing {BLUEPRINT_KEY_VAR} from environment {}...",
            bp.env_name
        ));
        if let Ok(vars) = list_environment_variables(&client, &bp.env_id)
            && let Some(v) = find_environment_variable_by_key(BLUEPRINT_KEY_VAR, &vars)
            && let Err(e) = client.delete(&format!("/variable/{}", esc(&v.id)))
        {
            qovery_log::println_error(format!("failed to delete {BLUEPRINT_KEY_VAR}: {e}"));
        }
    }

    qovery_log::println("");
    qovery_log::println(format!(
        "Blueprint {} unregistered. Project and environments are preserved.",
        bp.project_name
    ));
}
