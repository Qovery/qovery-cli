//! `qovery container list|create|update|clone` and `qovery container registry list`. Port of
//! `cmd/container_{list,create,update,clone,registry,registry_list}.go`.

use crate::{
    client_or_exit, container_flag, container_not_found, fail, find_container_or_exit, ids_or_exit,
    list_containers, run_group_help, with_context_flags,
};
use qovery_api::models::{EnvironmentStatuses, Resource};
use qovery_api::{ApiError, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, telemetry};
use qovery_log::gofmt::{go_json, go_marshal, time_string_from_rfc3339};
use qovery_log::pterm::FG_BLUE;
use serde::Serialize;
use serde_json::{Map, Value, json};

pub fn list() -> Command {
    with_context_flags(Command::new("list").short("List containers"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

pub fn create() -> Command {
    with_context_flags(Command::new("create").short("Create a container service"))
        .flag(container_flag().required())
        .flag(Flag::string("registry", "Container Registry ID").required())
        .flag(Flag::string("image-name", "Container Image Name").required())
        .flag(
            Flag::string("tag", "Container Image Tag")
                .short('t')
                .required(),
        )
        .flag(Flag::int32("port", "Container Port (0 = no port exposed)").short('p'))
        .flag(Flag::int32("cpu", "CPU in millicores (e.g. 500 = 0.5 vCPU)").default_value("500"))
        .flag(Flag::int32("memory", "Memory in MB").default_value("512"))
        .flag(
            Flag::int32("min-instances", "Minimum number of running instances").default_value("1"),
        )
        .flag(
            Flag::int32("max-instances", "Maximum number of running instances").default_value("1"),
        )
        .flag(Flag::bool("json", "JSON output"))
        .run(run_create)
}

pub fn update() -> Command {
    with_context_flags(Command::new("update").short("Update a container"))
        .flag(container_flag().required())
        .flag(Flag::string("image-name", "Container Image Name"))
        .flag(Flag::string("tag", "Container Tag"))
        .run(run_update)
}

pub fn clone() -> Command {
    with_context_flags(Command::new("clone").short("Clone a container"))
        .flag(container_flag().required())
        .flag(Flag::string("target-project", "Target Project Name"))
        .flag(Flag::string(
            "target-environment",
            "Target Environment Name",
        ))
        .flag(Flag::string(
            "target-container-name",
            "Target Container Name",
        ))
        .run(run_clone)
}

pub fn registry() -> Command {
    Command::new("registry")
        .short("Manage container registries")
        .run(run_group_help)
        .subcommand(
            Command::new("list")
                .short("List container registries")
                .flag(Flag::string("organization", "Organization Name"))
                .flag(Flag::bool("json", "JSON output"))
                .run(run_registry_list),
        )
}

fn str_of<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(|x| x.as_str()).unwrap_or("")
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let (_, _, env_id) = ids_or_exit(&client, ctx);
    let containers = list_containers(&client, &env_id);
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{env_id}/statuses")));

    if ctx.bool("json") {
        // Go `getContainerJsonOutput`: the status is looked up in the application statuses (sic)
        let results: Vec<Value> = containers
            .iter()
            .map(|c| {
                let mut result = json!({
                    "id": c.id,
                    "name": c.name,
                    "type": "Container",
                    "status": qovery_core::status::find_status(&statuses.applications, &c.id),
                    "last_update": time_string_from_rfc3339(c.updated_at.as_deref().unwrap_or("")),
                    "source": "image",
                    "registry": qovery_services::container_registry_name(c),
                    "image_name": c.image_name,
                    "tag": c.tag,
                });
                if let Some(repository) = qovery_services::get_container_git_repository(c) {
                    result["source"] = json!("git_repository");
                    result["git_repository"] = json!({
                        "url": repository.get("url").and_then(Value::as_str).unwrap_or_default(),
                        "branch": repository.get("branch").and_then(Value::as_str).unwrap_or_default(),
                    });
                }
                result
            })
            .collect();
        let out = if results.is_empty() {
            Value::Null
        } else {
            Value::Array(results)
        };
        qovery_log::println(go_json(&out));
        return;
    }

    let data: Vec<Vec<String>> = containers
        .iter()
        .map(|c| {
            vec![
                c.id.clone(),
                c.name.clone(),
                "Container".to_string(),
                qovery_core::status::find_status_text_with_color(&statuses.containers, &c.id),
                time_string_from_rfc3339(c.updated_at.as_deref().unwrap_or("")),
                qovery_services::container_source_label(c),
                qovery_services::container_image_label(c),
            ]
        })
        .collect();
    qovery_log::print_table(
        &[
            "Id",
            "Name",
            "Type",
            "Status",
            "Last Update",
            "Source",
            "Image",
        ],
        &data,
    );
}

/// Go: `utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))`.
fn print_status_body(e: &ApiError) {
    if let Some(status) = e.status {
        qovery_log::println_error(format!(
            "status code: {} ; body: {}",
            qovery_api::status_line(status),
            e.body
        ));
    }
}

#[derive(Serialize)]
struct CreateOutput {
    id: String,
    name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    public_link: String,
}

fn run_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let (_, _, env_id) = ids_or_exit(&client, ctx);

    let port = ctx.int32("port");
    let mut req = Map::new();
    if port > 0 {
        req.insert(
            "ports".into(),
            json!([{
                "name": format!("p{port}"),
                "internal_port": port,
                "external_port": 443,
                "publicly_accessible": true,
                "is_default": true,
                "protocol": "HTTP",
            }]),
        );
    }
    req.insert("name".into(), json!(ctx.string("container")));
    req.insert("registry_id".into(), json!(ctx.string("registry")));
    req.insert("image_name".into(), json!(ctx.string("image-name")));
    req.insert("tag".into(), json!(ctx.string("tag")));
    req.insert("cpu".into(), json!(ctx.int32("cpu")));
    req.insert("memory".into(), json!(ctx.int32("memory")));
    req.insert(
        "min_running_instances".into(),
        json!(ctx.int32("min-instances")),
    );
    req.insert(
        "max_running_instances".into(),
        json!(ctx.int32("max-instances")),
    );
    req.insert("healthchecks".into(), json!({}));

    let created: Value = match client.post(
        &format!("/environment/{env_id}/container"),
        &Value::Object(req),
    ) {
        Ok(v) => v,
        Err(e) => {
            if e.status.is_some_and(|s| s != 201) {
                print_status_body(&e);
            }
            fail(e)
        }
    };
    let created_id = str_of(&created, "id").to_string();
    let created_name = str_of(&created, "name").to_string();

    let mut public_link = String::new();
    if port > 0
        && let Ok(links) = client.list::<Value>(&format!("/container/{created_id}/link"))
        && let Some(link) = links.first()
    {
        public_link = str_of(link, "url").to_string();
    }

    if ctx.bool("json") {
        qovery_log::println(go_marshal(&CreateOutput {
            id: created_id,
            name: created_name,
            public_link,
        }));
        return;
    }

    let mut msg = format!(
        "Container service {} created! (id: {})",
        FG_BLUE.sprint(&created_name),
        FG_BLUE.sprint(&created_id)
    );
    if !public_link.is_empty() {
        msg += &format!(" - Public link: {}", FG_BLUE.sprint(&public_link));
    }
    qovery_log::println(msg);
}

/// The `ContainerRequest` of `container update`, built from the `ContainerResponse` like the Go code
/// (unset optional fields omitted, like the Go client).
fn update_request(c: &Value, image_name: &str, tag: &str) -> Value {
    let get = |k: &str| c.get(k).filter(|v| !v.is_null());
    let int = |k: &str| json!(c.get(k).and_then(|v| v.as_i64()).unwrap_or(0));
    let mut req = Map::new();

    let storage: Vec<Value> = get("storage")
        .and_then(|s| s.as_array())
        .map(|list| {
            list.iter()
                .map(|s| {
                    json!({
                        "id": str_of(s, "id"),
                        "type": str_of(s, "type"),
                        "size": s.get("size").and_then(|v| v.as_i64()).unwrap_or(0),
                        "mount_point": str_of(s, "mount_point"),
                    })
                })
                .collect()
        })
        .unwrap_or_default();
    if !storage.is_empty() {
        req.insert("storage".into(), Value::Array(storage));
    }

    let ports: Vec<Value> = get("ports")
        .and_then(|s| s.as_array())
        .map(|list| {
            list.iter()
                .map(|p| {
                    let mut m = Map::new();
                    for k in ["name", "external_port", "is_default"] {
                        if let Some(v) = p.get(k).filter(|v| !v.is_null()) {
                            m.insert(k.into(), v.clone());
                        }
                    }
                    m.insert(
                        "internal_port".into(),
                        json!(p.get("internal_port").and_then(|v| v.as_i64()).unwrap_or(0)),
                    );
                    m.insert(
                        "publicly_accessible".into(),
                        json!(
                            p.get("publicly_accessible")
                                .and_then(|v| v.as_bool())
                                .unwrap_or(false)
                        ),
                    );
                    m.insert("protocol".into(), json!(str_of(p, "protocol")));
                    Value::Object(m)
                })
                .collect()
        })
        .unwrap_or_default();
    if !ports.is_empty() {
        req.insert("ports".into(), Value::Array(ports));
    }

    req.insert("name".into(), json!(str_of(c, "name")));
    if let Some(d) = get("description") {
        req.insert("description".into(), d.clone());
    }
    req.insert(
        "registry_id".into(),
        json!(c.get("registry").map(|r| str_of(r, "id")).unwrap_or("")),
    );
    req.insert("image_name".into(), json!(image_name));
    req.insert("tag".into(), json!(tag));
    if let Some(a) = get("arguments") {
        req.insert("arguments".into(), a.clone());
    }
    if let Some(e) = get("entrypoint") {
        req.insert("entrypoint".into(), e.clone());
    }
    req.insert("cpu".into(), int("cpu"));
    req.insert("memory".into(), int("memory"));
    req.insert("min_running_instances".into(), int("min_running_instances"));
    req.insert("max_running_instances".into(), int("max_running_instances"));
    req.insert(
        "healthchecks".into(),
        get("healthchecks").cloned().unwrap_or_else(|| json!({})),
    );
    req.insert(
        "auto_preview".into(),
        json!(
            c.get("auto_preview")
                .and_then(|v| v.as_bool())
                .unwrap_or(false)
        ),
    );
    req.insert(
        "auto_deploy".into(),
        c.get("auto_deploy").cloned().unwrap_or(Value::Null),
    );
    if let Some(a) = qovery_services::convert_autoscaling_response_to_request(c.get("autoscaling"))
    {
        req.insert("autoscaling".into(), a);
    }
    Value::Object(req)
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let (_, _, env_id) = ids_or_exit(&client, ctx);
    let name = ctx.string("container");
    let container = find_container_or_exit(&client, &env_id, &name).to_value();

    let image_flag = ctx.string("image-name");
    let image_name = if image_flag.is_empty() {
        str_of(&container, "image_name").to_string()
    } else {
        image_flag
    };
    let tag_flag = ctx.string("tag");
    let tag = if tag_flag.is_empty() {
        str_of(&container, "tag").to_string()
    } else {
        tag_flag
    };

    let req = update_request(&container, &image_name, &tag);
    let id = str_of(&container, "id").to_string();
    if let Err(e) = client.execute(Method::PUT, &format!("/container/{id}"), &[], Some(&req)) {
        if e.status != Some(200) {
            print_status_body(&e);
        }
        fail(e);
    }

    qovery_log::println(format!("Container {} updated!", FG_BLUE.sprint(&name)));
}

fn run_clone(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let (organization_id, project_id, env_id) = ids_or_exit(&client, ctx);
    let name = ctx.string("container");

    let container = match qovery_core::resources::container_by_name(&client, &name, &env_id) {
        Ok(Some(c)) => c,
        _ => container_not_found(&name),
    };

    let mut target_project_id = project_id;
    let target_project_name = ctx.string("target-project");
    if !target_project_name.is_empty() {
        target_project_id = check_error(qovery_core::resources::project_id(
            &client,
            &target_project_name,
            &organization_id,
        ));
    }

    let mut target_environment_id = env_id;
    let target_environment_name = ctx.string("target-environment");
    if !target_environment_name.is_empty() {
        target_environment_id = check_error(qovery_core::resources::environment_id(
            &client,
            &target_environment_name,
            &target_project_id,
        ));
    }

    let mut target_container_name = ctx.string("target-container-name");
    if target_container_name.is_empty() {
        target_container_name = container.name.clone();
    }

    let req = json!({"name": target_container_name, "environment_id": target_environment_id});
    let cloned: Value = match client.post(&format!("/container/{}/clone", container.id), &req) {
        Ok(v) => v,
        Err(e) => {
            if e.status != Some(200) {
                print_status_body(&e);
            }
            fail(e)
        }
    };

    qovery_log::println(format!(
        "Container {} cloned!",
        FG_BLUE.sprint(str_of(&cloned, "name"))
    ));
}

fn run_registry_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let organization_id = check_error(qovery_core::resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let registries: Vec<Value> = check_error(client.list(&format!(
        "/organization/{organization_id}/containerRegistry"
    )));

    let fields = |r: &Value| -> [String; 4] {
        [
            str_of(r, "id").to_string(),
            str_of(r, "name").to_string(),
            str_of(r, "kind").to_string(),
            str_of(r, "url").to_string(),
        ]
    };

    if ctx.bool("json") {
        let results: Vec<Value> = registries
            .iter()
            .map(|r| {
                let [id, name, kind, url] = fields(r);
                json!({"id": id, "name": name, "kind": kind, "url": url})
            })
            .collect();
        let out = if results.is_empty() {
            Value::Null
        } else {
            Value::Array(results)
        };
        qovery_log::println(go_json(&out));
        return;
    }

    let data: Vec<Vec<String>> = registries.iter().map(|r| fields(r).to_vec()).collect();
    qovery_log::print_table(&["Id", "Name", "Kind", "URL"], &data);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn update_request_like_go_client() {
        let c = json!({
            "id": "c", "name": "n", "image_name": "img", "tag": "1", "registry": {"id": "r"},
            "cpu": 250, "memory": 256, "min_running_instances": 1, "max_running_instances": 2,
            "healthchecks": {"checks": []}, "auto_preview": true, "arguments": [], "description": null,
            "ports": [{"id": "p", "internal_port": 80, "publicly_accessible": true, "protocol": "HTTP", "name": "p80"}],
            "storage": [{"id": "s", "type": "FAST_SSD", "size": 10, "mount_point": "/data"}],
        });
        let req = update_request(&c, "img2", "1");
        assert_eq!(
            req,
            json!({
                "name": "n", "registry_id": "r", "image_name": "img2", "tag": "1", "arguments": [],
                "cpu": 250, "memory": 256, "min_running_instances": 1, "max_running_instances": 2,
                "healthchecks": {"checks": []}, "auto_preview": true, "auto_deploy": null,
                "ports": [{"name": "p80", "internal_port": 80, "publicly_accessible": true, "protocol": "HTTP"}],
                "storage": [{"id": "s", "type": "FAST_SSD", "size": 10, "mount_point": "/data"}],
            })
        );
    }
}
