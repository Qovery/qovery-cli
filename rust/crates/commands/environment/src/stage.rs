//! `environment stage create|delete|edit|list|move|skip|unskip` (`cmd/environment_stage*.go`).

use crate::{fail, ope_flags, run_group};
use qovery_api::models::Job;
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::resources::ids_from_flags;
use qovery_core::{check_error, client, telemetry};
use qovery_log::gofmt::go_json;
use qovery_log::pterm::Section;
use qovery_services::{get_job_name, get_service_name_by_id_and_type, path_escape};
use serde_json::{Map, Value, json};

pub(crate) fn command() -> Command {
    Command::new("stage")
        .short("Manage deployment stages")
        .run(run_group)
        .subcommand(create())
        .subcommand(delete())
        .subcommand(edit())
        .subcommand(list())
        .subcommand(move_())
        .subcommand(skip())
        .subcommand(unskip())
}

fn str_of<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(Value::as_str).unwrap_or_default()
}

/// `ListEnvironmentDeploymentStage(envId)`: `GET /environment/{id}/deploymentStage`.
fn list_stages(client: &Client, environment_id: &str) -> Result<Vec<Value>, String> {
    client
        .list(&format!(
            "/environment/{}/deploymentStage",
            path_escape(environment_id)
        ))
        .map_err(|e| e.to_string())
}

fn stages_or_exit(ctx: &Ctx) -> (Client, Vec<Value>) {
    let client = check_error(client::get_client());
    let (_, _, environment_id) = check_error(ids_from_flags(&client, ctx));
    let stages = match list_stages(&client, &environment_id) {
        Ok(s) => s,
        Err(e) => fail(e),
    };
    (client, stages)
}

/// `GetStageByName`.
fn get_stage_by_name<'a>(stages: &'a [Value], stage_name: &str) -> Result<&'a Value, String> {
    stages
        .iter()
        .find(|s| str_of(s, "name") == stage_name)
        .ok_or_else(|| "stage not found".to_string())
}

/// `qovery.DeploymentStageRequest`: `description` only when not empty.
fn stage_request(name: &str, description: &str) -> Value {
    let mut m = Map::new();
    m.insert("name".into(), json!(name));
    if !description.is_empty() {
        m.insert("description".into(), json!(description));
    }
    Value::Object(m)
}

fn services_of(stage: &Value) -> Vec<Value> {
    stage
        .get("services")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default()
}

// ---------------------------------------------------------------------------------------------------------------

fn create() -> Command {
    Command::new("create")
        .short("Create deployment stage")
        .flags(ope_flags())
        .flag(
            Flag::string("name", "StageLevel Name")
                .short('n')
                .required(),
        )
        .flag(Flag::string("description", "StageLevel Description").short('d'))
        .run(run_create)
}

fn run_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment_id) = check_error(ids_from_flags(&client, ctx));

    let body = stage_request(&ctx.string("name"), &ctx.string("description"));
    let path = format!(
        "/environment/{}/deploymentStage",
        path_escape(&environment_id)
    );
    if let Err(e) = client.execute(Method::POST, &path, &[], Some(&body)) {
        fail(e);
    }

    qovery_log::println("StageLevel created successfully");
}

fn delete() -> Command {
    Command::new("delete")
        .short("Delete deployment stage")
        .flags(ope_flags())
        .flag(
            Flag::string("name", "StageLevel Name")
                .short('n')
                .required(),
        )
        .run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, stages) = stages_or_exit(ctx);
    let stage = match get_stage_by_name(&stages, &ctx.string("name")) {
        Ok(s) => s,
        Err(e) => fail(e),
    };
    if let Err(e) = client.delete(&format!(
        "/deploymentStage/{}",
        path_escape(str_of(stage, "id"))
    )) {
        fail(e);
    }
    qovery_log::println("StageLevel deleted successfully");
}

fn edit() -> Command {
    Command::new("edit")
        .short("Edit deployment stage")
        .flags(ope_flags())
        .flag(
            Flag::string("name", "StageLevel Name")
                .short('n')
                .required(),
        )
        .flag(Flag::string("new-name", "New StageLevel Name").required())
        .flag(Flag::string(
            "new-description",
            "New StageLevel Description",
        ))
        .run(run_edit)
}

fn run_edit(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, stages) = stages_or_exit(ctx);
    let stage = match get_stage_by_name(&stages, &ctx.string("name")) {
        Ok(s) => s,
        Err(e) => fail(e),
    };
    let body = stage_request(&ctx.string("new-name"), &ctx.string("new-description"));
    let path = format!("/deploymentStage/{}", path_escape(str_of(stage, "id")));
    if let Err(e) = client.execute(Method::PUT, &path, &[], Some(&body)) {
        fail(e);
    }
    qovery_log::println("StageLevel updated successfully");
}

// ---------------------------------------------------------------------------------------------------------------
// list

fn list() -> Command {
    Command::new("list")
        .short("List deployment stages")
        .flags(ope_flags())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn service_name(client: &Client, service: &Value) -> String {
    get_service_name_by_id_and_type(
        client,
        str_of(service, "service_id"),
        str_of(service, "service_type"),
    )
}

fn is_skipped(service: &Value) -> bool {
    service
        .get("is_skipped")
        .and_then(Value::as_bool)
        .unwrap_or(false)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, stages) = stages_or_exit(ctx);

    if ctx.bool("json") {
        qovery_log::println(get_environment_stage_json_output(&client, &stages));
        return;
    }

    // Collect all skipped services across all stages
    let mut skipped_data: Vec<Vec<String>> = Vec::new();
    for stage in &stages {
        for service in services_of(stage) {
            if is_skipped(&service) {
                skipped_data.push(vec![
                    str_of(&service, "id").to_string(),
                    str_of(&service, "service_type").to_string(),
                    service_name(&client, &service),
                    str_of(stage, "name").to_string(),
                ]);
            }
        }
    }

    // Show skipped services section first
    if !skipped_data.is_empty() {
        Section::default()
            .with_bottom_padding(0)
            .println("Skipped services (excluded from environment-level deployments)");
        qovery_log::println("");
        qovery_log::print_table(&["Id", "Type", "Name", "Stage"], &skipped_data);
        qovery_log::println("");
    }

    // Show each stage with only non-skipped services
    for stage in &stages {
        let order = stage
            .get("deployment_order")
            .and_then(Value::as_i64)
            .unwrap_or(0) as i32;
        Section::default().with_bottom_padding(0).println(&format!(
            "deployment stage {}: \"{}\"",
            order.wrapping_add(1),
            str_of(stage, "name")
        ));
        qovery_log::println(format!("Stage id: {}", str_of(stage, "id")));
        let description = str_of(stage, "description");
        if !description.is_empty() {
            qovery_log::println(description);
        }

        qovery_log::println("");

        let services = services_of(stage);
        let data: Vec<Vec<String>> = services
            .iter()
            .filter(|s| !is_skipped(s))
            .map(|s| {
                vec![
                    str_of(s, "id").to_string(),
                    str_of(s, "service_type").to_string(),
                    service_name(&client, s),
                ]
            })
            .collect();

        if data.is_empty() {
            if services.is_empty() {
                qovery_log::println("<no service>");
            } else {
                qovery_log::println("<all services skipped>");
            }
        } else {
            qovery_log::print_table(&["Id", "Type", "Name"], &data);
        }

        qovery_log::println("");
    }
}

/// Go `*string` field: `null` when missing.
fn nullable(v: &Value, k: &str) -> Value {
    v.get(k).cloned().unwrap_or(Value::Null)
}

fn get_environment_stage_json_output(client: &Client, stages: &[Value]) -> String {
    let mut skipped_services: Vec<Value> = Vec::new();
    let mut results: Vec<Value> = Vec::new();

    for (idx, stage) in stages.iter().enumerate() {
        let mut services: Vec<Value> = Vec::new();

        for service in services_of(stage) {
            services.push(json!({
                "id": nullable(&service, "service_id"),
                "type": nullable(&service, "service_type"),
                "name": service_name(client, &service),
                "is_skipped": is_skipped(&service),
            }));

            if is_skipped(&service) {
                skipped_services.push(json!({
                    "id": nullable(&service, "service_id"),
                    "type": nullable(&service, "service_type"),
                    "name": service_name(client, &service),
                    "stage": nullable(stage, "name"),
                }));
            }
        }

        results.push(json!({
            "stage_order": idx + 1,
            "stage_id": str_of(stage, "id"),
            "stage_name": nullable(stage, "name"),
            "stage_description": nullable(stage, "description"),
            "services": if services.is_empty() { Value::Null } else { Value::Array(services) },
        }));
    }

    go_json(&json!({
        "skipped_services": if skipped_services.is_empty() { Value::Null } else { Value::Array(skipped_services) },
        "stages": if results.is_empty() { Value::Null } else { Value::Array(results) },
    }))
}

// ---------------------------------------------------------------------------------------------------------------
// move / skip / unskip

/// `getServiceByName(client, services, name)`: the first service of the stage whose name (fetched from the
/// API according to its type) is `name`. Any API error or unsupported service type ends the search.
fn get_service_by_name(client: &Client, services: &[Value], name: &str) -> Result<Value, String> {
    for service in services {
        let id = path_escape(str_of(service, "service_id"));
        let service_name = match str_of(service, "service_type") {
            "APPLICATION" => fetch_name(client, &format!("/application/{id}"))?,
            "DATABASE" => fetch_name(client, &format!("/database/{id}"))?,
            "CONTAINER" => fetch_name(client, &format!("/container/{id}"))?,
            "JOB" => {
                let job: Job = client
                    .get(&format!("/job/{id}"))
                    .map_err(|e| e.to_string())?;
                get_job_name(&job)
            }
            _ => return Err("service type not found".to_string()),
        };
        if service_name == name {
            return Ok(service.clone());
        }
    }
    Err("service not found".to_string())
}

fn fetch_name(client: &Client, path: &str) -> Result<String, String> {
    let v: Value = client.get(path).map_err(|e| e.to_string())?;
    Ok(str_of(&v, "name").to_string())
}

/// Search the service named `name` in every stage: (service, id of its stage).
fn find_service(client: &Client, stages: &[Value], name: &str) -> Option<(Value, String)> {
    for stage in stages {
        if let Ok(service) = get_service_by_name(client, &services_of(stage), name) {
            return Some((service, str_of(stage, "id").to_string()));
        }
    }
    None
}

fn move_() -> Command {
    Command::new("move")
        .short("Move service into deployment stage")
        .flags(ope_flags())
        .flag(
            Flag::string("name", "ServiceLevel Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("stage", "Target StageLevel Name")
                .short('s')
                .required(),
        )
        .run(run_move)
}

fn run_move(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, stages) = stages_or_exit(ctx);

    let Some((service, _)) = find_service(&client, &stages, &ctx.string("name")) else {
        fail("service not found");
    };

    let stage_name = ctx.string("stage");
    let stage = match get_stage_by_name(&stages, &stage_name) {
        Ok(s) => s,
        Err(e) => fail(e),
    };

    // The Go client sends the nil `AttachServiceToDeploymentStageRequest` pointer: a `null` body.
    let path = format!(
        "/deploymentStage/{}/service/{}",
        path_escape(str_of(stage, "id")),
        path_escape(str_of(&service, "service_id"))
    );
    if let Err(e) = client.execute(Method::PUT, &path, &[], Some(&Value::Null)) {
        fail(e);
    }

    qovery_log::println(format!("Application moved into stage \"{stage_name}\""));
}

fn skip_command(name: &str, short: &str, long: &str, run: fn(&Ctx)) -> Command {
    Command::new(name)
        .short(short)
        .long(long)
        .flags(ope_flags())
        .flag(Flag::string("name", "Service Name").short('n').required())
        .run(run)
}

fn skip() -> Command {
    skip_command(
        "skip",
        "Skip service from environment-level deployments",
        "Mark a service as skipped so it is excluded from environment-level bulk deployments while staying in its current stage. To reverse this, use 'environment stage unskip' or move the service to a different deployment stage, which automatically clears the skipped status.",
        run_skip,
    )
}

fn unskip() -> Command {
    skip_command(
        "unskip",
        "Unskip service from environment-level deployments",
        "Remove the skipped flag from a service so it is included again in environment-level bulk deployments.",
        run_unskip,
    )
}

fn run_skip(ctx: &Ctx) {
    set_skipped(ctx, true);
}

fn run_unskip(ctx: &Ctx) {
    set_skipped(ctx, false);
}

fn set_skipped(ctx: &Ctx, skipped: bool) {
    telemetry::capture(ctx);
    let (client, stages) = stages_or_exit(ctx);

    let service_name = ctx.string("name");
    let Some((service, current_stage_id)) = find_service(&client, &stages, &service_name) else {
        fail("service not found");
    };

    let path = format!(
        "/deploymentStage/{}/service/{}",
        path_escape(&current_stage_id),
        path_escape(str_of(&service, "service_id"))
    );
    check_error(client.execute(
        Method::PUT,
        &path,
        &[],
        Some(&json!({"is_skipped": skipped})),
    ));

    if skipped {
        qovery_log::println(format!(
            "Service \"{service_name}\" is now skipped from environment-level deployments"
        ));
    } else {
        qovery_log::println(format!(
            "Service \"{service_name}\" is no longer skipped from environment-level deployments"
        ));
    }
}
