//! `helm list`, `deploy`, `redeploy`, `stop`, `delete`, `cancel`, `clone`.

use crate::{context_flags, find_helm_or_exit, helm_not_found, list_helms};
use qovery_api::models::{EnvironmentStatuses, Helm};
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::find::find_by_name;
use qovery_core::{check_error, client, exit_with_error, resources, status, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{helm_ids, new_services_watch, path_escape, state};
use serde_json::{Map, Value, json};

// ---------------------------------------------------------------------------------------------------------
// list (helm_list.go)

pub(crate) fn list() -> Command {
    context_flags(Command::new("list").short("List helms"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

/// Go `helm.UpdatedAt.String()` (`UpdatedAt` is a `*time.Time`: nil pointer panic when missing).
fn updated_at_string(helm: &Helm) -> String {
    match helm.updated_at.as_deref() {
        Some(t) => time_string_from_rfc3339(t),
        None => qovery_envvars::go_nil_pointer_panic(),
    }
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let helms = list_helms(&client, &env_id);
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{}/statuses", path_escape(&env_id))));

    if ctx.bool("json") {
        qovery_log::println(get_helm_json_output(&helms, &statuses));
        return;
    }

    let data: Vec<Vec<String>> = helms
        .iter()
        .map(|helm| {
            vec![
                helm.id.clone(),
                helm.name.clone(),
                "Helm".to_string(),
                status::find_status_text_with_color(&statuses.helms, &helm.id),
                updated_at_string(helm),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Type", "Status", "Last Update"], &data);
}

/// Go `getHelmJsonOutput(helms, statuses)` (`null` when there is no helm).
fn get_helm_json_output(helms: &[Helm], statuses: &EnvironmentStatuses) -> String {
    if helms.is_empty() {
        return go_json(&Value::Null);
    }
    let results: Vec<Value> = helms
        .iter()
        .map(|helm| {
            json!({
                "id": helm.id,
                "name": helm.name,
                "type": "Helm",
                "status": status::find_status(&statuses.helms, &helm.id),
                "last_update": updated_at_string(helm),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

// ---------------------------------------------------------------------------------------------------------
// helpers of helm_stop.go

/// Go `validateHelmArguments(helmName, helmNames)`.
fn validate_helm_arguments(helm_name: &str, helm_names: &str) {
    if helm_name.is_empty() && helm_names.is_empty() {
        exit_with_error(
            "use either --helm \"<helm name>\" or --helms \"<helm1 name>, <helm2 name>\" but not both at the same time",
        );
    }
    if !helm_name.is_empty() && !helm_names.is_empty() {
        exit_with_error("you can't use --helm and --helms at the same time");
    }
}

/// Go `buildHelmListFromHelmNames(client, environmentId, helmName, helmNames)`.
fn build_helm_list_from_helm_names(
    client: &Client,
    env_id: &str,
    helm_name: &str,
    helm_names: &str,
) -> Vec<Helm> {
    let helms = list_helms(client, env_id);
    let mut helm_list = Vec::new();
    if !helm_name.is_empty() {
        helm_list.push(find_helm_or_exit(&helms, helm_name));
    }
    if !helm_names.is_empty() {
        for name in helm_names.split(',') {
            match find_by_name(&helms, name.trim()) {
                Some(h) => helm_list.push(h.clone()),
                // the Go code reports the untrimmed name
                None => helm_not_found(name),
            }
        }
    }
    helm_list
}

/// The prologue of deploy / redeploy / stop / delete: client, validation, environment id and helm list.
fn selected_helms(ctx: &Ctx) -> (Client, String, Vec<Helm>) {
    let client = client::get_client_or_exit();
    let (helm_name, helm_names) = (ctx.string("helm"), ctx.string("helms"));
    validate_helm_arguments(&helm_name, &helm_names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);
    let helm_list = build_helm_list_from_helm_names(&client, &env_id, &helm_name, &helm_names);
    (client, env_id, helm_list)
}

/// `pterm.FgBlue.Sprintf("%s%s", helmName, helmNames)`.
fn blue_names(ctx: &Ctx) -> String {
    FG_BLUE.sprint(format!("{}{}", ctx.string("helm"), ctx.string("helms")))
}

fn helms_flag(cmd: Command, usage: &str) -> Command {
    cmd.flag(Flag::string("helms", usage))
}

// ---------------------------------------------------------------------------------------------------------
// deploy (helm_deploy.go)

pub(crate) fn deploy() -> Command {
    let cmd = context_flags(Command::new("deploy").short("Deploy a helm"))
        .flag(Flag::string("helm", "helm Name").short('n'));
    helms_flag(
        cmd,
        "helm Names (comma separated) (ex: --helms \"helm1,helm2\")",
    )
    .flag(Flag::string("chart_version", "helm chart version"))
    .flag(Flag::string(
        "chart_git_commit_id",
        "helm chart git commit id, or 'latest' for the newest commit of the branch",
    ))
    .flag(Flag::string(
        "values_override_git_commit_id",
        "helm values override git commit id",
    ))
    .flag(
        Flag::bool(
            "watch",
            "Watch helm status until it's ready or an error occurs",
        )
        .short('w'),
    )
    .run(run_deploy)
}

fn run_deploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, env_id, helm_list) = selected_helms(ctx);
    let watch = new_services_watch(&client, &env_id, helm_ids(&helm_list), ctx.bool("watch"));
    check_error(qovery_services::deploy_helms(
        &client,
        &env_id,
        &helm_list,
        &ctx.string("chart_version"),
        &ctx.string("chart_git_commit_id"),
        &ctx.string("values_override_git_commit_id"),
    ));
    qovery_log::println(format!(
        "Request to deploy helm(s) {} has been queued..",
        blue_names(ctx)
    ));
    watch.wait(state::DEPLOYED);
}

// ---------------------------------------------------------------------------------------------------------
// redeploy (helm_redeploy.go)

pub(crate) fn redeploy() -> Command {
    context_flags(Command::new("redeploy").short("Redeploy a helm"))
        .flag(Flag::string("helm", "Helm Name").short('n').required())
        .flag(
            Flag::bool(
                "watch",
                "Watch helm status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_redeploy)
}

fn run_redeploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, env_id, helm_list) = selected_helms(ctx);
    let watch = new_services_watch(&client, &env_id, helm_ids(&helm_list), ctx.bool("watch"));
    // HelmActionsAPI.DeployHelm(helmList[0].Id) with an empty HelmDeployRequest
    let body = Value::Object(Map::new());
    check_error(client.execute(
        Method::POST,
        &format!("/helm/{}/deploy", path_escape(&helm_list[0].id)),
        &[],
        Some(&body),
    ));
    qovery_log::println(format!(
        "Request to redeploy helm(s) {} has been queued..",
        blue_names(ctx)
    ));
    watch.wait(state::DEPLOYED);
}

// ---------------------------------------------------------------------------------------------------------
// stop (helm_stop.go) / delete (helm_delete.go)

pub(crate) fn stop() -> Command {
    let cmd = context_flags(Command::new("stop").short("Stop a helm"))
        .flag(Flag::string("helm", "Helm Name").short('n'));
    helms_flag(
        cmd,
        "Helm Names (comma separated) (ex: --helms \"helm1,helm2\")",
    )
    .flag(
        Flag::bool(
            "watch",
            "Watch helm status until it's ready or an error occurs",
        )
        .short('w'),
    )
    .run(run_stop)
}

pub(crate) fn delete() -> Command {
    let cmd = context_flags(Command::new("delete").short("Delete a helm"))
        .flag(Flag::string("helm", "Helm Name").short('n'));
    helms_flag(
        cmd,
        "Helm Names (comma separated) (ex: --helms \"helm1,helm2\")",
    )
    .flag(
        Flag::bool(
            "watch",
            "Watch helm status until it's ready or an error occurs",
        )
        .short('w'),
    )
    .run(run_delete)
}

/// `EnvironmentActionsAPI.{Stop,Delete}SelectedServices` with `EnvironmentServiceIdsAllRequest{HelmIds}`.
fn selected_services_action(ctx: &Ctx, action: &str, verb: &str, final_state: &str) {
    telemetry::capture(ctx);
    let (client, env_id, helm_list) = selected_helms(ctx);
    let watch = new_services_watch(&client, &env_id, helm_ids(&helm_list), ctx.bool("watch"));
    let request = qovery_services::ServiceIdsAllRequest {
        helm_ids: helm_ids(&helm_list),
        ..Default::default()
    };
    check_error(qovery_services::selected_services_action(
        &client, &env_id, action, &request,
    ));
    qovery_log::println(format!(
        "Request to {verb} helm(s) {} has been queued...",
        blue_names(ctx)
    ));
    watch.wait(final_state);
}

fn run_stop(ctx: &Ctx) {
    selected_services_action(ctx, "stop", "stop", state::STOPPED);
}

fn run_delete(ctx: &Ctx) {
    selected_services_action(ctx, "delete", "delete", state::DELETED);
}

// ---------------------------------------------------------------------------------------------------------
// cancel (helm_cancel.go)

pub(crate) fn cancel() -> Command {
    context_flags(Command::new("cancel").short("Cancel a helm deployment"))
        .flag(Flag::string("helm", "Helm Name").short('n').required())
        .flag(Flag::bool("watch", "Watch cancel until it's done or an error occurs").short('w'))
        .run(run_cancel)
}

fn run_cancel(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, (_, _, env_id), helm) = crate::resolve_helm(ctx);
    let msg = check_error(qovery_services::cancel_service_deployment(
        &client,
        &env_id,
        &helm.id,
        ServiceType::Helm,
        ctx.bool("watch"),
    ));
    if !msg.is_empty() {
        qovery_log::println_info(msg);
        return;
    }
    qovery_log::println(format!(
        "helm {} deployment cancelled!",
        FG_BLUE.sprint(ctx.string("helm"))
    ));
}

// ---------------------------------------------------------------------------------------------------------
// clone (helm_clone.go)

pub(crate) fn clone() -> Command {
    context_flags(Command::new("clone").short("Clone a helm"))
        .flag(Flag::string("helm", "Helm Name").short('n').required())
        .flag(Flag::string("target-project", "Target Project Name"))
        .flag(Flag::string(
            "target-environment",
            "Target Environment Name",
        ))
        .flag(Flag::string("target-helm-name", "Target Helm Name"))
        .run(run_clone)
}

/// Go: `utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, body))` then the error,
/// exit 1 (used by `clone` and `update` when the API answers an error).
pub(crate) fn exit_with_api_error(err: qovery_api::ApiError) -> ! {
    if let Some(code) = err.status
        && code != 200
    {
        qovery_log::println_error(format!(
            "status code: {} ; body: {}",
            qovery_api::status_line(code),
            err.body
        ));
    }
    exit_with_error(err)
}

/// Required properties of the Go client's `HelmResponse` (`parity/required_fields.py helm_response`).
const HELM_RESPONSE_REQUIRED: &[&str] = &[
    "id",
    "created_at",
    "environment",
    "name",
    "auto_preview",
    "auto_deploy",
    "source",
    "arguments",
    "allow_cluster_wide_resources",
    "values_override",
    "icon_uri",
    "service_type",
];

fn run_clone(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, project_id, env_id) =
        check_error(resources::ids_from_flags(&client, ctx));
    let helm_name = ctx.string("helm");

    // getHelmContextResource: (nil, nil) without environment id (then a nil pointer dereference)
    if env_id.trim().is_empty() {
        qovery_envvars::go_nil_pointer_panic();
    }
    let helm = match resources::helm_by_name(&client, &helm_name, &env_id) {
        Ok(Some(h)) => h,
        _ => helm_not_found(&helm_name),
    };

    let mut target_project_id = project_id;
    let target_project_name = ctx.string("target-project");
    if !target_project_name.is_empty() {
        target_project_id = check_error(resources::project_id(
            &client,
            &target_project_name,
            &organization_id,
        ));
    }

    let mut target_environment_id = env_id;
    let target_environment_name = ctx.string("target-environment");
    if !target_environment_name.is_empty() {
        target_environment_id = check_error(resources::environment_id(
            &client,
            &target_environment_name,
            &target_project_id,
        ));
    }

    let mut target_helm_name = ctx.string("target-helm-name");
    if target_helm_name.is_empty() {
        target_helm_name = helm.name.clone();
    }

    let body = json!({ "name": target_helm_name, "environment_id": target_environment_id });
    let res = match client.execute(
        Method::POST,
        &format!("/helm/{}/clone", path_escape(&helm.id)),
        &[],
        Some(&body),
    ) {
        Ok(r) => r,
        Err(e) => exit_with_api_error(e),
    };
    // `*HelmResponse` (nil for an empty / `null` body); decoding errors are reported like the API errors
    let cloned = match crate::go_decode::go_decode_model(&res, HELM_RESPONSE_REQUIRED) {
        Ok(cloned) => cloned,
        Err(message) => exit_with_api_error(qovery_api::ApiError {
            status: Some(res.status),
            message,
            body: res.text(),
        }),
    };
    let name = cloned
        .as_ref()
        .and_then(|c| c.get("name"))
        .and_then(Value::as_str)
        .unwrap_or_default();
    qovery_log::println(format!("Helm {} cloned!", FG_BLUE.sprint(name)));
}
