//! `environment cancel|clone|delete|deploy|list|redeploy|statuses|stop|update`
//! (`cmd/environment_{cancel,clone,delete,deploy,list,redeploy,statuses,stop,update}.go`).

use crate::deploy_request::parse_deploy_all_request;
use crate::{fail, ope_flags};
use chrono::DateTime;
use qovery_api::models::{
    Application, Container, Environment, EnvironmentStatus, EnvironmentStatuses, Helm, Job,
};
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::find_by_name;
use qovery_core::resources::{
    environment_id_from_flags_or_exit, ids_from_flags, organization_project_ids, project_id,
};
use qovery_core::{check_error, client, exit, status, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_services::{
    DeployAllRequest, DeployAllRequestApplication, DeployAllRequestContainer, DeployAllRequestHelm,
    DeployAllRequestJob, deploy_all_services, get_git_source, get_helm_repository, get_job_docker,
    get_job_id, get_job_image, get_job_name, is_latest_commit_keyword, path_escape,
    resolve_latest_application_commit, resolve_latest_helm_chart_commit, resolve_latest_job_commit,
    state, to_iso8601, watch_environment,
};
use serde_json::{Map, Value, json};
use std::time::Duration;

const WATCH_USAGE: &str = "Watch environment status until it's ready or an error occurs";

fn watch_flag() -> Flag {
    Flag::bool("watch", WATCH_USAGE).short('w')
}

/// POST without body; the Go client decodes the response, the Rust port only checks the status.
fn post_empty(client: &Client, path: &str) -> Result<(), String> {
    client
        .execute(Method::POST, path, &[], None)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// `time.Sleep(3 * time.Second)` before watching (prevents a race with the deployment request).
fn sleep_before_watch() {
    std::thread::sleep(Duration::from_secs(3));
}

// ---------------------------------------------------------------------------------------------------------------
// cancel

pub(crate) fn cancel() -> Command {
    Command::new("cancel")
        .short("Cancel an environment deployment")
        .flags(ope_flags())
        .flag(Flag::bool("force", "Force cancel").short('f'))
        .flag(watch_flag())
        .run(run_cancel)
}

fn run_cancel(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(ids_from_flags(&client, ctx));

    let body = json!({"force_cancel": ctx.bool("force")});
    let path = format!("/environment/{}/cancelDeployment", path_escape(&env_id));
    // Go exits 1 without a message on any error, the decoding of the `EnvironmentStatus` response included
    if client
        .execute(Method::POST, &path, &[], Some(&body))
        .map_err(|e| e.to_string())
        .and_then(|res| qovery_services::util::decode_go_response::<EnvironmentStatus>(&res))
        .is_err()
    {
        exit(1);
    }

    qovery_log::println("Environment is canceling!");

    if ctx.bool("watch") {
        watch_environment(&env_id, state::CANCELED, &client);
    }
}

// ---------------------------------------------------------------------------------------------------------------
// clone

pub(crate) fn clone() -> Command {
    Command::new("clone")
        .short("Clone an environment")
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .flag(Flag::string("project", "Project Name").short('p'))
        .flag(Flag::string("environment", "Environment Name to clone").short('e'))
        .flag(Flag::string("new-environment-name", "New Environment Name").short('n').required())
        .flag(Flag::string("cluster", "Cluster Name where to clone the environment").short('c'))
        .flag(
            Flag::string(
                "environment-type",
                "Environment type for the new environment (DEVELOPMENT|STAGING|PRODUCTION)",
            )
            .short('t'),
        )
        .flag(Flag::bool(
            "apply-deployment-rule",
            "Enable applying deployment rules on the new environment instead of having a pristine clone. Default: false",
        ))
        .flag(Flag::string("target-project", "Target Project Name"))
        .run(run_clone)
}

fn run_clone(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (org_id, _, env_id) = check_error(ids_from_flags(&client, ctx));

    let mut req = Map::new();
    req.insert("name".into(), json!(ctx.string("new-environment-name")));
    req.insert(
        "apply_deployment_rule".into(),
        json!(ctx.bool("apply-deployment-rule")),
    );

    let cluster_name = ctx.string("cluster");
    if !cluster_name.is_empty() {
        let clusters: Vec<Value> =
            match client.list(&format!("/organization/{}/cluster", path_escape(&org_id))) {
                Ok(c) => c,
                Err(e) => fail(e),
            };
        let lower = cluster_name.to_lowercase();
        if let Some(c) = clusters
            .iter()
            .find(|c| c["name"].as_str().unwrap_or_default().to_lowercase() == lower)
        {
            req.insert(
                "cluster_id".into(),
                json!(c["id"].as_str().unwrap_or_default()),
            );
        }
    }

    let environment_type = ctx.string("environment-type");
    if !environment_type.is_empty() {
        let upper = environment_type.to_uppercase();
        if matches!(upper.as_str(), "DEVELOPMENT" | "PRODUCTION" | "STAGING") {
            req.insert("mode".into(), json!(upper));
        }
    }

    let target_project_name = ctx.string("target-project");
    if !target_project_name.is_empty() {
        match project_id(&client, &target_project_name, &org_id) {
            Ok(id) => {
                req.insert("project_id".into(), json!(id));
            }
            Err(e) => fail(e),
        }
    }

    let path = format!("/environment/{}/clone", path_escape(&env_id));
    if let Err(e) = client.execute(Method::POST, &path, &[], Some(&Value::Object(req))) {
        // print http body error message
        if e.status.is_some() && !e.message.contains("200") {
            qovery_log::println_error(format!("status code: {} ; body: {}", e.message, e.body));
        }
        fail(e);
    }

    qovery_log::println("Environment is cloned!");
}

// ---------------------------------------------------------------------------------------------------------------
// delete / redeploy / stop

pub(crate) fn delete() -> Command {
    Command::new("delete")
        .short("Delete an environment")
        .flags(ope_flags())
        .flag(watch_flag())
        .run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let env_id = environment_id_from_flags_or_exit(&client, ctx);

    check_error(client.delete(&format!("/environment/{}", path_escape(&env_id))));
    qovery_log::println("Request to delete environment has been queued...");
    if ctx.bool("watch") {
        sleep_before_watch();
        watch_environment(&env_id, state::DELETED, &client);
    }
}

pub(crate) fn redeploy() -> Command {
    Command::new("redeploy")
        .short("Redeploy an environment")
        .flags(ope_flags())
        .flag(watch_flag())
        .run(run_redeploy)
}

fn run_redeploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let env_id = environment_id_from_flags_or_exit(&client, ctx);
    check_error(post_empty(
        &client,
        &format!("/environment/{}/deploy", path_escape(&env_id)),
    ));
    qovery_log::println("Request to redeploy environment has been queued..");
    if ctx.bool("watch") {
        sleep_before_watch();
        watch_environment(&env_id, state::DEPLOYED, &client);
    }
}

pub(crate) fn stop() -> Command {
    Command::new("stop")
        .short("Stop an environment")
        .flags(ope_flags())
        .flag(watch_flag())
        .run(run_stop)
}

fn run_stop(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let env_id = environment_id_from_flags_or_exit(&client, ctx);
    check_error(post_empty(
        &client,
        &format!("/environment/{}/stop", path_escape(&env_id)),
    ));
    qovery_log::println("Environment stop request has been queued..");
    if ctx.bool("watch") {
        sleep_before_watch();
        watch_environment(&env_id, state::STOPPED, &client);
    }
}

// ---------------------------------------------------------------------------------------------------------------
// deploy

pub(crate) fn deploy() -> Command {
    Command::new("deploy")
        .short("Deploy an environment")
        .flags(ope_flags())
        .flag(Flag::string(
            "services",
            "Services to deploy (JSON Format: https://api-doc.qovery.com/#tag/Environment-Actions/operation/deployAllServices)",
        ))
        .flag(Flag::string(
            "applications",
            "Applications to deploy E.g. --applications app1:commit_id,app2:latest). If you omit the commit id, the same commit will be used. 'latest' deploys the newest commit of the branch",
        ))
        .flag(Flag::string(
            "containers",
            "Containers to deploy E.g. --containers container1:image_tag,container2:image_tag). If you omit the image tag, the same image tag will be used",
        ))
        .flag(Flag::string(
            "lifecycles",
            "Lifecycle to deploy E.g. --lifecycles job1:image_tag|git_commit_id,job2:image_tag|git_commit_id). If you omit the git commit id or image tag, the same version will be used. 'latest' deploys the newest commit of the branch for a git-sourced job",
        ))
        .flag(Flag::string(
            "cronjobs",
            "Cronjobs to deploy E.g. --cronjobs cronjob1:git_commit_id,cronjob2:latest). If you omit the git commit id, the same version will be used. 'latest' deploys the newest commit of the branch for a git-sourced cronjob",
        ))
        .flag(Flag::string(
            "helms",
            "Helms to deploy E.g. --helms helm1:chart_version|git_commit_id,helm2:chart_version|git_commit_id). If you omit the chart version or git commit id, the same version will be used. 'latest' deploys the newest commit of the branch for a git-sourced chart",
        ))
        .flag(watch_flag())
        .flag(Flag::bool("skip-paused-services", "Skip paused services: paused services won't be started / deployed"))
        .run(run_deploy)
}

fn run_deploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let env_id = environment_id_from_flags_or_exit(&client, ctx);

    let services_json = ctx.string("services");
    let application_names = ctx.string("applications");
    let container_names = ctx.string("containers");
    let lifecycle_names = ctx.string("lifecycles");
    let cronjob_names = ctx.string("cronjobs");
    let helm_names = ctx.string("helms");
    let skip_paused_services = ctx.bool("skip-paused-services");
    let any_names = !application_names.is_empty()
        || !container_names.is_empty()
        || !lifecycle_names.is_empty()
        || !cronjob_names.is_empty()
        || !helm_names.is_empty();

    if (!services_json.is_empty() || any_names) && skip_paused_services {
        fail(
            "you can't use --skip-paused-services flag with --services, --applications, --containers, --lifecycles, --cronjobs or --helms flags",
        );
    }

    let deploy_path = format!("/environment/{}/service/deploy", path_escape(&env_id));
    if !services_json.is_empty() {
        // convert servicesJson to DeployAllRequest
        let request = check_error(parse_deploy_all_request(&services_json));
        check_error(
            client
                .execute(Method::POST, &deploy_path, &[], Some(&request))
                .map(|_| ())
                .map_err(|e| e.to_string()),
        );
        qovery_log::println("Request to deploy services has been queued..");
    } else if any_names {
        let request = get_deployment_request_for_multiple_services(
            &client,
            &env_id,
            &application_names,
            &container_names,
            &lifecycle_names,
            &cronjob_names,
            &helm_names,
        );
        check_error(deploy_all_services(&client, &env_id, &request));
        qovery_log::println("Request to deploy services has been queued..");
    }

    if skip_paused_services {
        // Paused services shouldn't be deployed, let's gather services status
        let services = check_error(get_eligible_services(&client, &env_id, &[state::STOPPED]));

        let mut request = DeployAllRequest::default();
        for id in services.applications_ids {
            request
                .applications
                .get_or_insert_with(Vec::new)
                .push(DeployAllRequestApplication {
                    application_id: id.clone(),
                    git_commit_id: None,
                });
            qovery_log::println(format!(
                "Request to deploy application {id} has been queued.."
            ));
        }
        for id in services.containers_ids {
            request
                .containers
                .get_or_insert_with(Vec::new)
                .push(DeployAllRequestContainer {
                    id: id.clone(),
                    image_tag: None,
                });
            qovery_log::println(format!(
                "Request to deploy container {id} has been queued.."
            ));
        }
        for id in services.helms_ids {
            request
                .helms
                .get_or_insert_with(Vec::new)
                .push(DeployAllRequestHelm {
                    id: Some(id.clone()),
                    ..Default::default()
                });
            qovery_log::println(format!("Request to deploy helm {id} has been queued.."));
        }
        for id in services.jobs_ids {
            request
                .jobs
                .get_or_insert_with(Vec::new)
                .push(DeployAllRequestJob {
                    id: Some(id.clone()),
                    ..Default::default()
                });
            qovery_log::println(format!("Request to deploy job {id} has been queued.."));
        }
        for id in services.databases_ids {
            request
                .databases
                .get_or_insert_with(Vec::new)
                .push(id.clone());
            qovery_log::println(format!("Request to deploy database {id} has been queued.."));
        }

        check_error(deploy_all_services(&client, &env_id, &request));
    } else if services_json.is_empty() && !any_names {
        // Deploy the whole env
        check_error(post_empty(
            &client,
            &format!("/environment/{}/deploy", path_escape(&env_id)),
        ));
        qovery_log::println("Request to deploy environment has been queued..");
    }

    if ctx.bool("watch") {
        sleep_before_watch();
        watch_environment(&env_id, state::DEPLOYED, &client);
    }
}

/// Go `splitServiceNameAndVersion`: `name[:version]` (only the first two `:`-separated parts are used).
fn split_service_name_and_version(service: &str) -> (String, Option<String>) {
    let mut split = service.split(':');
    let name = split.next().unwrap_or_default().to_string();
    (name, split.next().map(|s| s.to_string()))
}

fn is_latest_version(version: &Option<String>) -> bool {
    version.as_deref().is_some_and(is_latest_commit_keyword)
}

fn resolve_latest_job_version(
    client: &Client,
    job: &Job,
    version: Option<String>,
) -> Option<String> {
    if !is_latest_version(&version) {
        return version;
    }
    match resolve_latest_job_commit(client, &get_job_id(job), &get_job_name(job)) {
        Ok(id) => Some(id),
        Err(e) => fail(e),
    }
}

fn list_or_exit<T: serde::de::DeserializeOwned>(client: &Client, path: &str) -> Vec<T> {
    match client.list(path) {
        Ok(v) => v,
        Err(e) => fail(e),
    }
}

/// `getDeploymentRequestForMultipleServices`.
fn get_deployment_request_for_multiple_services(
    client: &Client,
    env_id: &str,
    application_names: &str,
    container_names: &str,
    lifecycle_names: &str,
    cronjob_names: &str,
    helm_names: &str,
) -> DeployAllRequest {
    let mut request = DeployAllRequest::default();
    let env = path_escape(env_id);

    if !application_names.is_empty() {
        for name_and_version in application_names.split(',') {
            let (name, mut version) = split_service_name_and_version(name_and_version);
            let apps: Vec<Application> =
                list_or_exit(client, &format!("/environment/{env}/application"));
            let Some(app) = find_by_name(&apps, &name) else {
                fail(format!("application {name} not found"));
            };
            if is_latest_version(&version) {
                match resolve_latest_application_commit(client, &app.id, &app.name) {
                    Ok(id) => version = Some(id),
                    Err(e) => fail(e),
                }
            }
            request
                .applications
                .get_or_insert_with(Vec::new)
                .push(DeployAllRequestApplication {
                    application_id: app.id.clone(),
                    git_commit_id: version,
                });
        }
    }

    if !container_names.is_empty() {
        for name_and_version in container_names.split(',') {
            let (name, version) = split_service_name_and_version(name_and_version);
            let containers: Vec<Container> =
                list_or_exit(client, &format!("/environment/{env}/container"));
            let Some(container) = find_by_name(&containers, &name) else {
                fail(format!("container {name} not found"));
            };
            request
                .containers
                .get_or_insert_with(Vec::new)
                .push(DeployAllRequestContainer {
                    id: container.id.clone(),
                    image_tag: version,
                });
        }
    }

    if !lifecycle_names.is_empty() || !cronjob_names.is_empty() {
        let jobs: Vec<Job> = list_or_exit(client, &format!("/environment/{env}/job"));
        // utils.FindByJobName only matches cron and lifecycle jobs
        let jobs: Vec<Job> = jobs
            .into_iter()
            .filter(|j| j.is_cron() || j.is_lifecycle())
            .collect();

        for (names, kind, is_kind) in [
            (
                lifecycle_names,
                "lifecycle",
                Job::is_lifecycle as fn(&Job) -> bool,
            ),
            (cronjob_names, "cronjob", Job::is_cron as fn(&Job) -> bool),
        ] {
            if names.is_empty() {
                continue;
            }
            for name_and_version in names.split(',') {
                let (name, version) = split_service_name_and_version(name_and_version);
                let Some(job) = find_by_name(&jobs, &name) else {
                    fail(format!("{kind} {name} not found"));
                };
                if !is_kind(job) {
                    // job.LifecycleJobResponse.Id / job.CronJobResponse.Id of the other job kind
                    qovery_envvars::go_nil_pointer_panic();
                }
                let mut req = DeployAllRequestJob {
                    id: Some(job.id.clone()),
                    ..Default::default()
                };
                if get_job_docker(job).is_some() {
                    req.git_commit_id = resolve_latest_job_version(client, job, version);
                } else if get_job_image(job).is_some() {
                    req.image_tag = version;
                }
                request.jobs.get_or_insert_with(Vec::new).push(req);
            }
        }
    }

    if !helm_names.is_empty() {
        for name_and_version in helm_names.split(',') {
            let (name, mut version) = split_service_name_and_version(name_and_version);
            let helms: Vec<Helm> = list_or_exit(client, &format!("/environment/{env}/helm"));
            let Some(helm) = find_by_name(&helms, &name) else {
                fail(format!("helm {name} not found"));
            };
            let mut req = DeployAllRequestHelm {
                id: Some(helm.id.clone()),
                ..Default::default()
            };
            if get_git_source(helm).is_some() {
                if is_latest_version(&version) {
                    match resolve_latest_helm_chart_commit(client, &helm.id, &helm.name) {
                        Ok(id) => version = Some(id),
                        Err(e) => fail(e),
                    }
                }
                req.git_commit_id = version;
            } else if get_helm_repository(helm).is_some() {
                req.chart_version = version;
            }
            request.helms.get_or_insert_with(Vec::new).push(req);
        }
    }

    request
}

/// Go `Services` of `getEligibleServices`.
#[derive(Default)]
struct Services {
    applications_ids: Vec<String>,
    containers_ids: Vec<String>,
    helms_ids: Vec<String>,
    jobs_ids: Vec<String>,
    databases_ids: Vec<String>,
}

/// `getEligibleServices(client, envId, servicesStatusesToExclude)`.
fn get_eligible_services(
    client: &Client,
    env_id: &str,
    excluded: &[&str],
) -> Result<Services, String> {
    let statuses: EnvironmentStatuses = client
        .get(&format!("/environment/{}/statuses", path_escape(env_id)))
        .map_err(|e| e.to_string())?;
    let keep = |list: &[qovery_api::models::Status]| -> Vec<String> {
        list.iter()
            .filter(|s| !excluded.contains(&s.state.as_str()))
            .map(|s| s.id.clone())
            .collect()
    };
    Ok(Services {
        applications_ids: keep(&statuses.applications),
        containers_ids: keep(&statuses.containers),
        helms_ids: keep(&statuses.helms),
        jobs_ids: keep(&statuses.jobs),
        databases_ids: keep(&statuses.databases),
    })
}

// ---------------------------------------------------------------------------------------------------------------
// list

pub(crate) fn list() -> Command {
    Command::new("list")
        .short("List environments")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, project_id) = check_error(organization_project_ids(
        &client,
        &ctx.string("organization"),
        &ctx.string("project"),
    ));

    let environments: Vec<Environment> = list_or_exit(
        &client,
        &format!("/project/{}/environment", path_escape(&project_id)),
    );
    let statuses: Vec<EnvironmentStatus> = list_or_exit(
        &client,
        &format!("/project/{}/environment/status", path_escape(&project_id)),
    );

    if ctx.bool("json") {
        qovery_log::println(get_environment_json_output(&statuses, &environments));
        return;
    }

    let data: Vec<Vec<String>> = environments
        .iter()
        .map(|env| {
            let Some(cluster_name) = env.cluster_name.clone() else {
                qovery_envvars::go_nil_pointer_panic(); // *env.ClusterName
            };
            let Some(updated_at) = env.updated_at.as_deref() else {
                qovery_envvars::go_nil_pointer_panic(); // env.UpdatedAt.String()
            };
            vec![
                env.id.clone(),
                env.name.clone(),
                cluster_name,
                env.mode.clone(),
                status::environment_status_with_color(&statuses, &env.id),
                time_string_from_rfc3339(updated_at),
            ]
        })
        .collect();
    qovery_log::print_table(
        &["Id", "Name", "Cluster", "Type", "Status", "Last Update"],
        &data,
    );
}

fn get_environment_json_output(
    statuses: &[EnvironmentStatus],
    environments: &[Environment],
) -> String {
    let results: Vec<Value> = environments
        .iter()
        .map(|env| {
            let Some(cluster_name) = env.cluster_name.clone() else {
                qovery_envvars::go_nil_pointer_panic(); // *env.ClusterName
            };
            json!({
                "id": env.id,
                "created_at": to_iso8601(Some(env.created_at.as_deref().unwrap_or("0001-01-01T00:00:00Z"))),
                "updated_at": to_iso8601(env.updated_at.as_deref()),
                "name": env.name,
                "cluster_name": cluster_name,
                "cluster_id": env.cluster_id,
                "type": env.mode,
                "status": status::environment_status(statuses, &env.id),
            })
        })
        .collect();
    if results.is_empty() {
        return "null".to_string();
    }
    go_json(&Value::Array(results))
}

// ---------------------------------------------------------------------------------------------------------------
// statuses

pub(crate) fn statuses() -> Command {
    Command::new("statuses")
        .short("Get environment services statuses")
        .flags(ope_flags())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_statuses)
}

fn run_statuses(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(ids_from_flags(&client, ctx));

    // Get env and services statuses
    let raw: Value = match client.get(&format!("/environment/{}/statuses", path_escape(&env_id))) {
        Ok(v) => v,
        Err(e) => fail(e),
    };
    // The Go client fails on a response it cannot decode (type mismatch); the Rust model is lenient on
    // missing properties but reports type errors (serde's message, not Go's).
    let statuses: EnvironmentStatuses = match serde_json::from_value(raw.clone()) {
        Ok(s) => s,
        Err(e) => fail(e),
    };

    if ctx.bool("json") {
        qovery_log::println(go_json(&go_environment_statuses(&raw)));
        return;
    }

    let mut data: Vec<Vec<String>> = Vec::new();
    for (kind, list) in [
        ("application", &statuses.applications),
        ("container", &statuses.containers),
        ("helm", &statuses.helms),
        ("job", &statuses.jobs),
        ("database", &statuses.databases),
    ] {
        for s in list {
            data.push(vec![kind.to_string(), s.id.clone(), s.state.clone()]);
        }
    }

    qovery_log::println(format!(
        "\nEnvironment status: {} \n",
        statuses.environment.state
    ));
    qovery_log::print_table(&["Type", "ID", "Status"], &data);
}

/// Go `time.Time` JSON encoding (`RFC3339Nano`) of an API timestamp; unparsable values are kept.
fn go_time_json(v: &Value) -> Value {
    let Some(s) = v.as_str() else {
        return v.clone();
    };
    match DateTime::parse_from_rfc3339(s) {
        Ok(t) => Value::String(go_rfc3339_nano(&t)),
        Err(_) => v.clone(),
    }
}

use qovery_log::gofmt::time_rfc3339_nano as go_rfc3339_nano;

fn obj(v: &Value) -> Map<String, Value> {
    v.as_object().cloned().unwrap_or_default()
}

/// `qovery.Status.ToMap()`.
fn go_status(v: &Value) -> Value {
    let mut m = obj(v);
    for (k, default) in [
        ("id", json!("")),
        ("state", json!("")),
        ("service_deployment_status", json!("")),
    ] {
        if m.get(k).is_none_or(Value::is_null) {
            m.insert(k.into(), default);
        }
    }
    for k in ["last_deployment_date", "steps", "execution_id"] {
        if m.get(k).is_some_and(Value::is_null) {
            m.remove(k);
        }
    }
    if let Some(d) = m.get("last_deployment_date").cloned() {
        m.insert("last_deployment_date".into(), go_time_json(&d));
    }
    if m.get("is_part_last_deployment").is_none_or(Value::is_null) {
        m.insert("is_part_last_deployment".into(), json!(false));
    }
    let mut details = obj(m.get("status_details").unwrap_or(&Value::Null));
    for k in ["action", "status", "sub_action"] {
        if details.get(k).is_none_or(Value::is_null) {
            details.insert(k.into(), json!(""));
        }
    }
    m.insert("status_details".into(), Value::Object(details));
    if !m.contains_key("deployment_request_id") {
        m.insert("deployment_request_id".into(), Value::Null);
    }
    if m.get("deployment_requests_count")
        .is_none_or(Value::is_null)
    {
        m.insert("deployment_requests_count".into(), json!(0));
    }
    Value::Object(m)
}

/// `qovery.EnvironmentStatus.ToMap()`.
fn go_environment_status(v: &Value) -> Value {
    let mut m = obj(v);
    for k in ["id", "state", "last_deployment_state"] {
        if m.get(k).is_none_or(Value::is_null) {
            m.insert(k.into(), json!(""));
        }
    }
    for k in ["deployment_status", "metrics"] {
        if m.get(k).is_some_and(Value::is_null) {
            m.remove(k);
        }
    }
    if let Some(d) = m.get("last_deployment_date").cloned() {
        m.insert("last_deployment_date".into(), go_time_json(&d));
    }
    Value::Object(m)
}

/// `json.Marshal(statuses)` of a `qovery.EnvironmentStatuses` (`ToMap()` of the model and of its items).
fn go_environment_statuses(v: &Value) -> Value {
    let mut m = obj(v);
    m.insert(
        "environment".into(),
        go_environment_status(v.get("environment").unwrap_or(&Value::Null)),
    );
    for k in [
        "applications",
        "containers",
        "jobs",
        "databases",
        "helms",
        "terraforms",
        "agentic_workflows",
    ] {
        match m.get(k) {
            Some(Value::Array(items)) => {
                let items: Vec<Value> = items.iter().map(go_status).collect();
                m.insert(k.into(), Value::Array(items));
            }
            _ if k == "agentic_workflows" => {
                m.remove(k);
            }
            _ => {
                m.insert(k.into(), Value::Null);
            }
        }
    }
    Value::Object(m)
}

// ---------------------------------------------------------------------------------------------------------------
// update

pub(crate) fn update() -> Command {
    Command::new("update")
        .short("Update an environment")
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .flag(Flag::string("project", "Project Name").short('p'))
        .flag(Flag::string("environment", "Environment Name").short('e'))
        .flag(Flag::string("name", "New Environment Name"))
        .flag(Flag::string(
            "type",
            "Change Environment Type (DEVELOPMENT|STAGING|PRODUCTION)",
        ))
        .run(run_update)
}

/// `getEnvironmentType`: `CreateEnvironmentModeEnum`, DEVELOPMENT by default.
fn get_environment_type(environment_type: &str) -> &'static str {
    match environment_type.to_uppercase().as_str() {
        "PRODUCTION" => "PRODUCTION",
        "STAGING" => "STAGING",
        _ => "DEVELOPMENT",
    }
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, project_id) = check_error(organization_project_ids(
        &client,
        &ctx.string("organization"),
        &ctx.string("project"),
    ));

    let environments: Vec<Environment> = list_or_exit(
        &client,
        &format!("/project/{}/environment", path_escape(&project_id)),
    );

    let environment_name = ctx.string("environment");
    let Some(env) = find_by_name(&environments, &environment_name) else {
        qovery_log::println_error(format!("environment {environment_name} not found"));
        qovery_log::println_info("You can list all environments with: qovery environment list");
        exit(1);
    };

    let mut name = env.name.clone();
    let mut mode = get_environment_type(&env.mode);
    let new_name = ctx.string("name");
    if !new_name.is_empty() {
        name = new_name;
    }
    let environment_type = ctx.string("type");
    if !environment_type.is_empty() {
        mode = get_environment_type(&environment_type);
    }

    let body = json!({"name": name, "mode": mode});
    if let Err(e) = client.execute(
        Method::PUT,
        &format!("/environment/{}", path_escape(&env.id)),
        &[],
        Some(&body),
    ) {
        fail(e);
    }

    qovery_log::println("Environment is updated!");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn split_name_and_version() {
        assert_eq!(
            split_service_name_and_version("app"),
            ("app".to_string(), None)
        );
        assert_eq!(
            split_service_name_and_version("app:v1"),
            ("app".to_string(), Some("v1".to_string()))
        );
        assert_eq!(
            split_service_name_and_version("app:v1:x"),
            ("app".to_string(), Some("v1".to_string()))
        );
        assert_eq!(
            split_service_name_and_version("app:"),
            ("app".to_string(), Some(String::new()))
        );
    }

    #[test]
    fn environment_type() {
        assert_eq!(get_environment_type("staging"), "STAGING");
        assert_eq!(get_environment_type("Production"), "PRODUCTION");
        assert_eq!(get_environment_type("preview"), "DEVELOPMENT");
    }

    #[test]
    fn rfc3339_nano() {
        assert_eq!(
            go_time_json(&json!("2024-06-02T11:30:00.120Z")),
            json!("2024-06-02T11:30:00.12Z")
        );
        assert_eq!(
            go_time_json(&json!("2024-06-02T11:30:00+00:00")),
            json!("2024-06-02T11:30:00Z")
        );
        assert_eq!(
            go_time_json(&json!("2024-06-02T11:30:00+02:00")),
            json!("2024-06-02T11:30:00+02:00")
        );
    }
}
