//! `application list|deploy|redeploy|restart|stop|delete|cancel|clone|update`.

use crate::{
    application_flag, application_not_found, context_flags, find_application_or_exit,
    list_applications,
};
use qovery_api::models::{Application, EnvironmentStatuses};
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::find::find_by_name;
use qovery_core::{check_error, client, exit_with_error, resources, status, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{
    RebootServicesRequest, application_ids, cancel_service_deployment,
    check_applications_commit_id, convert_autoscaling_response_to_request, deploy_applications,
    go_healthcheck, is_latest_commit_keyword, new_services_watch, path_escape, reboot_services,
    resolve_latest_application_commit, state,
};
use serde_json::{Map, Value, json};

const APPLICATIONS_USAGE: &str =
    "Application Names (comma separated) Example: --applications \"app1,app2,app3\"";
const WATCH_USAGE: &str = "Watch application status until it's ready or an error occurs";
const COMMIT_ID_LATEST_USAGE: &str =
    "Application Commit ID, or 'latest' for the newest commit of the branch";

// ---- list ---------------------------------------------------------------------------------------------------

pub(crate) fn list() -> Command {
    context_flags(Command::new("list").short("List applications"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let applications = list_applications(&client, &env_id);
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{}/statuses", path_escape(&env_id))));

    if ctx.bool("json") {
        qovery_log::println(app_json_output(&applications, &statuses));
        return;
    }

    let data: Vec<Vec<String>> = applications
        .iter()
        .map(|a| {
            vec![
                a.id.clone(),
                a.name.clone(),
                "Application".to_string(),
                status::find_status_text_with_color(&statuses.applications, &a.id),
                updated_at_string(a),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Type", "Status", "Last Update"], &data);
}

/// `application.UpdatedAt.String()`.
fn updated_at_string(a: &Application) -> String {
    time_string_from_rfc3339(a.updated_at.as_deref().unwrap_or_default())
}

/// Port of `getAppJsonOutput` (a nil slice marshals as `null`).
fn app_json_output(applications: &[Application], statuses: &EnvironmentStatuses) -> String {
    if applications.is_empty() {
        return "null".to_string();
    }
    let results: Vec<Value> = applications
        .iter()
        .map(|a| {
            json!({
                "id": a.id,
                "name": a.name,
                "type": "Application",
                "status": status::find_status(&statuses.applications, &a.id),
                "last_update": updated_at_string(a),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

// ---- multi-services helpers (cmd/application_stop.go) ----------------------------------------------------------

/// Port of `validateApplicationArguments(applicationName, applicationNames)`.
fn validate_application_arguments(application_name: &str, application_names: &str) {
    if application_name.is_empty() && application_names.is_empty() {
        exit_with_error(
            "use either --application \"<app name>\" or --applications \"<app1 name>, <app2 name>\" but not both at the same time",
        );
    }
    if !application_name.is_empty() && !application_names.is_empty() {
        exit_with_error("you can't use --application and --applications at the same time");
    }
}

/// Port of `buildApplicationListFromApplicationNames(client, envId, applicationName, applicationNames)`.
fn build_application_list(
    client: &Client,
    env_id: &str,
    application_name: &str,
    application_names: &str,
) -> Vec<Application> {
    let mut list = Vec::new();
    let applications = list_applications(client, env_id);

    if !application_name.is_empty() {
        match find_by_name(&applications, application_name) {
            Some(a) => list.push(a.clone()),
            None => application_not_found(application_name),
        }
    }
    if !application_names.is_empty() {
        for name in application_names.split(',') {
            match find_by_name(&applications, name.trim()) {
                Some(a) => list.push(a.clone()),
                // like Go, the error shows the untrimmed name
                None => application_not_found(name),
            }
        }
    }
    list
}

/// Common start of the multi-services commands: client, arguments validation, environment id, applications.
fn multi_services_start(ctx: &Ctx) -> (Client, String, Vec<Application>, String) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let name = ctx.string("application");
    let names = ctx.string("applications");
    validate_application_arguments(&name, &names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);
    let list = build_application_list(&client, &env_id, &name, &names);
    (client, env_id, list, format!("{name}{names}"))
}

/// `EnvironmentServiceIdsAllRequest{ApplicationIds: ...}`.
fn application_ids_request(applications: &[Application]) -> qovery_services::ServiceIdsAllRequest {
    qovery_services::ServiceIdsAllRequest {
        application_ids: application_ids(applications),
        ..Default::default()
    }
}

// ---- deploy -------------------------------------------------------------------------------------------------

pub(crate) fn deploy() -> Command {
    context_flags(Command::new("deploy").short("Deploy an application"))
        .flag(application_flag())
        .flag(Flag::string("applications", APPLICATIONS_USAGE))
        .flag(Flag::string("commit-id", COMMIT_ID_LATEST_USAGE).short('c'))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_deploy)
}

fn run_deploy(ctx: &Ctx) {
    let (client, env_id, list, names) = multi_services_start(ctx);
    check_error(check_applications_commit_id(
        &list,
        &ctx.string("commit-id"),
        "--commit-id",
    ));
    let watch = new_services_watch(&client, &env_id, application_ids(&list), ctx.bool("watch"));
    check_error(deploy_applications(
        &client,
        &env_id,
        &list,
        &ctx.string("commit-id"),
    ));
    qovery_log::println(format!(
        "Request to deploy application(s) {} has been queued..",
        FG_BLUE.sprint(names)
    ));
    watch.wait(state::DEPLOYED);
}

// ---- redeploy -----------------------------------------------------------------------------------------------

pub(crate) fn redeploy() -> Command {
    context_flags(Command::new("redeploy").short("Redeploy an application"))
        .flag(application_flag().required())
        .flag(Flag::string("commit-id", COMMIT_ID_LATEST_USAGE).short('c'))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_redeploy)
}

fn run_redeploy(ctx: &Ctx) {
    let (client, env_id, list, names) = multi_services_start(ctx);
    check_error(check_applications_commit_id(
        &list,
        &ctx.string("commit-id"),
        "--commit-id",
    ));
    let watch = new_services_watch(&client, &env_id, application_ids(&list), ctx.bool("watch"));
    let application = &list[0];

    let commit_flag = ctx.string("commit-id");
    let commit_id = if is_latest_commit_keyword(&commit_flag) {
        check_error(resolve_latest_application_commit(
            &client,
            &application.id,
            &application.name,
        ))
    } else if !commit_flag.is_empty() {
        commit_flag
    } else {
        application
            .git_repository
            .as_ref()
            .and_then(|g| g.get("deployed_commit_id"))
            .and_then(|c| c.as_str())
            .unwrap_or_default()
            .to_string()
    };

    check_error(client.execute(
        Method::POST,
        &format!("/application/{}/deploy", path_escape(&application.id)),
        &[],
        Some(&json!({ "git_commit_id": commit_id })),
    ));
    qovery_log::println(format!(
        "Request to redeploy application(s) {} has been queued..",
        FG_BLUE.sprint(names)
    ));
    watch.wait(state::DEPLOYED);
}

// ---- restart ------------------------------------------------------------------------------------------------

pub(crate) fn restart() -> Command {
    context_flags(Command::new("restart").short("Restart an application without redeploying it"))
        .flag(application_flag())
        .flag(Flag::string(
            "applications",
            "Application Names (comma separated) Example: --applications \"app1,app2\"",
        ))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_restart)
}

fn run_restart(ctx: &Ctx) {
    let (client, env_id, list, names) = multi_services_start(ctx);
    let watch = new_services_watch(&client, &env_id, application_ids(&list), ctx.bool("watch"));
    check_error(reboot_services(
        &client,
        &env_id,
        &RebootServicesRequest {
            application_ids: Some(application_ids(&list)),
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to restart application(s) {} has been queued...",
        FG_BLUE.sprint(names)
    ));
    watch.wait(state::RESTARTED);
}

// ---- stop ---------------------------------------------------------------------------------------------------

pub(crate) fn stop() -> Command {
    context_flags(Command::new("stop").short("Stop an application"))
        .flag(application_flag())
        .flag(Flag::string("applications", APPLICATIONS_USAGE))
        .flag(Flag::string("commit-id", "Application Commit ID").short('c'))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_stop)
}

fn run_stop(ctx: &Ctx) {
    let (client, env_id, list, names) = multi_services_start(ctx);
    let watch = new_services_watch(&client, &env_id, application_ids(&list), ctx.bool("watch"));
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "stop",
        &application_ids_request(&list),
    ));
    qovery_log::println(format!(
        "Request to stop application(s) {} has been queued...",
        FG_BLUE.sprint(names)
    ));
    watch.wait(state::STOPPED);
}

// ---- delete -------------------------------------------------------------------------------------------------

pub(crate) fn delete() -> Command {
    context_flags(Command::new("delete").short("Delete an application"))
        .flag(application_flag())
        .flag(Flag::string("applications", APPLICATIONS_USAGE))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    let (client, env_id, list, names) = multi_services_start(ctx);
    let watch = new_services_watch(&client, &env_id, application_ids(&list), ctx.bool("watch"));
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "delete",
        &application_ids_request(&list),
    ));
    qovery_log::println(format!(
        "Request to delete application(s) {} has been queued...",
        FG_BLUE.sprint(names)
    ));
    watch.wait(state::DELETED);
}

// ---- cancel -------------------------------------------------------------------------------------------------

pub(crate) fn cancel() -> Command {
    context_flags(Command::new("cancel").short("Cancel an application deployment"))
        .flag(application_flag().required())
        .flag(Flag::bool("watch", "Watch cancel until it's done or an error occurs").short('w'))
        .run(run_cancel)
}

fn run_cancel(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let name = ctx.string("application");
    let application = find_application_or_exit(&client, &env_id, &name);

    let msg = check_error(cancel_service_deployment(
        &client,
        &env_id,
        &application.id,
        ServiceType::Application,
        ctx.bool("watch"),
    ));
    if !msg.is_empty() {
        qovery_log::println_info(msg);
        return;
    }
    qovery_log::println(format!(
        "Application {} deployment cancelled!",
        FG_BLUE.sprint(&name)
    ));
}

// ---- clone --------------------------------------------------------------------------------------------------

pub(crate) fn clone() -> Command {
    context_flags(Command::new("clone").short("Clone an application"))
        .flag(application_flag().required())
        .flag(Flag::string("target-project", "Target Project Name"))
        .flag(Flag::string(
            "target-environment",
            "Target Environment Name",
        ))
        .flag(Flag::string(
            "target-application-name",
            "Target Application Name",
        ))
        .run(run_clone)
}

fn run_clone(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, project_id, env_id) =
        check_error(resources::ids_from_flags(&client, ctx));
    let name = ctx.string("application");

    let application = match resources::application_by_name(&client, &name, &env_id) {
        Ok(Some(a)) => a,
        // Go dereferences a nil application here (empty environment id): report it as not found
        _ => application_not_found(&name),
    };

    let mut target_project_id = project_id; // use same project as the source project
    let target_project_name = ctx.string("target-project");
    if !target_project_name.is_empty() {
        target_project_id = check_error(resources::project_id(
            &client,
            &target_project_name,
            &organization_id,
        ));
    }

    let mut target_environment_id = env_id; // use same env as the source env
    let target_environment_name = ctx.string("target-environment");
    if !target_environment_name.is_empty() {
        target_environment_id = check_error(resources::environment_id(
            &client,
            &target_environment_name,
            &target_project_id,
        ));
    }

    let mut target_application_name = ctx.string("target-application-name");
    if target_application_name.is_empty() {
        // use same app name as the source app
        target_application_name = application.name.clone();
    }

    let req = json!({"name": target_application_name, "environment_id": target_environment_id});
    let res = client.send(
        Method::POST,
        &format!("/application/{}/clone", path_escape(&application.id)),
        &[],
        Some(&req),
    );
    let res = match res {
        Ok(r) => r,
        Err(e) => exit_with_error(e),
    };
    let decoded: Result<Value, String> = if res.status >= 300 {
        Err(res.status_line())
    } else {
        let body = if res.body.is_empty() {
            b"null".to_vec()
        } else {
            res.body.clone()
        };
        serde_json::from_slice(&body).map_err(|e| e.to_string())
    };
    let cloned = match decoded {
        Ok(v) => v,
        Err(e) => {
            // print http body error message
            if res.status != 200 {
                qovery_log::println_error(format!(
                    "status code: {} ; body: {}",
                    res.status_line(),
                    res.text()
                ));
            }
            exit_with_error(e)
        }
    };

    let cloned_name = cloned
        .get("name")
        .and_then(|n| n.as_str())
        .unwrap_or_default();
    qovery_log::println(format!(
        "Application {} cloned!",
        FG_BLUE.sprint(cloned_name)
    ));
}

// ---- update -------------------------------------------------------------------------------------------------

pub(crate) fn update() -> Command {
    context_flags(Command::new("update").short("Update an application"))
        .flag(application_flag().required())
        .flag(Flag::string("branch", "Application Git Branch"))
        .flag(Flag::bool("auto-deploy", "Application Auto Deploy"))
        .flag(Flag::bool(
            "auto-preview",
            "Add the service to pull-request preview environments; previews build pull-request code with the environment's secrets",
        ))
        .run(run_update)
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let name = ctx.string("application");
    let application = find_application_or_exit(&client, &env_id, &name);

    let mut req = check_error(application_edit_request(
        &serde_json::to_value(&application).unwrap_or(Value::Null),
        &ctx.string("branch"),
    ));
    if ctx.changed("auto-deploy") {
        req.insert("auto_deploy".into(), Value::Bool(ctx.bool("auto-deploy")));
    }
    if ctx.changed("auto-preview") {
        req.insert("auto_preview".into(), Value::Bool(ctx.bool("auto-preview")));
    }

    check_error(client.execute(
        Method::PUT,
        &format!("/application/{}", path_escape(&application.id)),
        &[],
        Some(&Value::Object(req)),
    ));
    qovery_log::println(format!("Application {} updated!", FG_BLUE.sprint(&name)));
}

/// Copies `key` when present and not null (Go pointer / slice fields with `omitempty`).
fn copy_ptr(src: &Value, dst: &mut Map<String, Value>, key: &str) {
    if let Some(v) = src.get(key).filter(|v| !v.is_null()) {
        dst.insert(key.to_string(), v.clone());
    }
}

/// Copies `key` when present, even null (Go `NullableXxx` fields).
fn copy_nullable(src: &Value, dst: &mut Map<String, Value>, key: &str) {
    if let Some(v) = src.get(key) {
        dst.insert(key.to_string(), v.clone());
    }
}

/// A `qovery.ServicePort` decoded then re-encoded by the Go client (optional fields omitted when null).
fn go_service_port(p: &Value) -> Value {
    let Some(obj) = p.as_object() else {
        return p.clone();
    };
    let optional = [
        "name",
        "external_port",
        "is_default",
        "public_path",
        "public_path_rewrite",
    ];
    Value::Object(
        obj.iter()
            .filter(|(k, v)| !(v.is_null() && optional.contains(&k.as_str())))
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect(),
    )
}

/// The `qovery.ApplicationEditRequest` built by `cmd/application_update.go` from the current application,
/// serialized like the Go client (`ToMap`), with `branch` when set. An application without Git repository
/// is sent without `git_repository` and rejects a `branch`.
fn application_edit_request(app: &Value, branch: &str) -> Result<Map<String, Value>, String> {
    let mut req = Map::new();

    // storage: only when the application has some (Go appends to a nil slice)
    if let Some(storage) = app
        .get("storage")
        .and_then(|s| s.as_array())
        .filter(|s| !s.is_empty())
    {
        let items: Vec<Value> = storage
            .iter()
            .map(|s| {
                json!({
                    "id": s.get("id").and_then(|v| v.as_str()).unwrap_or_default(),
                    "type": s.get("type").and_then(|v| v.as_str()).unwrap_or_default(),
                    "size": s.get("size").and_then(|v| v.as_i64()).unwrap_or_default(),
                    "mount_point": s.get("mount_point").and_then(|v| v.as_str()).unwrap_or_default(),
                })
            })
            .collect();
        req.insert("storage".into(), Value::Array(items));
    }

    req.insert(
        "name".into(),
        Value::String(
            app.get("name")
                .and_then(|v| v.as_str())
                .unwrap_or_default()
                .to_string(),
        ),
    );
    copy_ptr(app, &mut req, "description");

    if let Some(git) = app.get("git_repository").filter(|g| g.is_object()) {
        let mut git_req = Map::new();
        git_req.insert(
            "url".into(),
            Value::String(
                git.get("url")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default()
                    .to_string(),
            ),
        );
        copy_ptr(git, &mut git_req, "branch");
        if !branch.is_empty() {
            git_req.insert("branch".into(), Value::String(branch.to_string()));
        }
        copy_ptr(git, &mut git_req, "root_path");
        copy_nullable(git, &mut git_req, "git_token_id");
        git_req.insert(
            "provider".into(),
            Value::String(
                git.get("provider")
                    .and_then(|v| v.as_str())
                    .unwrap_or_default()
                    .to_string(),
            ),
        );
        req.insert("git_repository".into(), Value::Object(git_req));
    } else if !branch.is_empty() {
        return Err(format!(
            "application {} runs an image, not a Git repository: --branch only applies to Git applications",
            app.get("name").and_then(|v| v.as_str()).unwrap_or_default()
        ));
    }

    copy_ptr(app, &mut req, "build_mode");
    copy_nullable(app, &mut req, "dockerfile_path");
    copy_ptr(app, &mut req, "cpu");
    copy_ptr(app, &mut req, "memory");
    copy_ptr(app, &mut req, "min_running_instances");
    copy_ptr(app, &mut req, "max_running_instances");
    req.insert(
        "healthchecks".into(),
        go_healthcheck(app.get("healthchecks").unwrap_or(&Value::Null)),
    );
    copy_ptr(app, &mut req, "auto_preview");
    if let Some(ports) = app.get("ports").and_then(|p| p.as_array()) {
        req.insert(
            "ports".into(),
            Value::Array(ports.iter().map(go_service_port).collect()),
        );
    }
    copy_ptr(app, &mut req, "arguments");
    copy_ptr(app, &mut req, "entrypoint");
    // AutoDeploy: *qovery.NewNullableBool(application.AutoDeploy) is always set
    req.insert(
        "auto_deploy".into(),
        app.get("auto_deploy")
            .filter(|v| v.is_boolean())
            .cloned()
            .unwrap_or(Value::Null),
    );
    if let Some(autoscaling) = convert_autoscaling_response_to_request(app.get("autoscaling")) {
        req.insert("autoscaling".into(), autoscaling);
    }
    Ok(req)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn edit_request_like_go() {
        let app = json!({
            "id": "app-1", "name": "my-app", "description": null, "storage": [],
            "git_repository": {"provider": "GITHUB", "url": "u", "branch": "main", "root_path": "/", "owner": "o", "name": "n"},
            "dockerfile_path": null, "cpu": 500, "ports": [{"id": "p", "internal_port": 80, "name": null, "publicly_accessible": true, "protocol": "HTTP"}],
            "arguments": [], "healthchecks": {"checks": []}
        });
        let req = application_edit_request(&app, "").unwrap();
        assert_eq!(
            Value::Object(req),
            json!({
                "name": "my-app",
                "git_repository": {"url": "u", "branch": "main", "root_path": "/", "provider": "GITHUB"},
                "dockerfile_path": null, "cpu": 500,
                "healthchecks": {"checks": []},
                "ports": [{"id": "p", "internal_port": 80, "publicly_accessible": true, "protocol": "HTTP"}],
                "arguments": [], "auto_deploy": null
            })
        );
    }

    #[test]
    fn edit_request_without_git_repository_keeps_the_other_fields() {
        let app = json!({"id": "app-image", "name": "web", "description": "serves the site", "git_repository": null, "auto_preview": false});
        let req = application_edit_request(&app, "").unwrap();
        assert_eq!(
            Value::Object(req),
            json!({"name": "web", "description": "serves the site", "auto_preview": false, "healthchecks": {}, "auto_deploy": null})
        );
    }

    #[test]
    fn edit_request_branch_without_git_repository_is_rejected() {
        let app = json!({"id": "app-image", "name": "web"});
        assert_eq!(
            application_edit_request(&app, "feature"),
            Err("application web runs an image, not a Git repository: --branch only applies to Git applications".to_string())
        );
    }

    #[test]
    fn edit_request_branch_on_git_application_changes_only_the_branch() {
        let app = json!({"name": "api", "git_repository": {"provider": "GITHUB", "url": "u", "branch": "main", "root_path": "/"}});
        let req = application_edit_request(&app, "feature").unwrap();
        assert_eq!(
            req.get("git_repository"),
            Some(&json!({"url": "u", "branch": "feature", "root_path": "/", "provider": "GITHUB"}))
        );
    }
}
