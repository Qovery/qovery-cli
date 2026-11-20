//! `qovery cronjob {cancel,clone,delete,deploy,list,redeploy,stop,update}`.

use crate::{
    context_flags, cronjob_flag, cronjob_not_found, find_by_job_name, find_cronjob_or_exit,
    list_cronjobs, list_jobs,
};
use qovery_api::models::{EnvironmentStatuses, Job};
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::{check_error, client, exit, exit_with_error, resources, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{path_escape, state};
use serde_json::{Value, json};

const CRONJOBS_USAGE: &str = "Cronjob Names (comma separated) (ex: --cronjobs \"cron1,cron2\")";
const WATCH_USAGE: &str = "Watch cronjob status until it's ready or an error occurs";

// ---- cancel -----------------------------------------------------------------------------------------------

pub(crate) fn cancel() -> Command {
    Command::new("cancel")
        .short("Cancel a cronjob deployment")
        .flags(context_flags())
        .flag(cronjob_flag().required())
        .flag(Flag::bool("watch", "Watch cancel until it's done or an error occurs").short('w'))
        .run(run_cancel)
}

fn run_cancel(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let jobs = check_error(list_jobs(&client, &env_id));
    let name = ctx.string("cronjob");
    let cronjob = find_cronjob_or_exit(&jobs, &name);

    let msg = check_error(qovery_services::cancel_service_deployment(
        &client,
        &env_id,
        &cronjob.id,
        ServiceType::Job,
        ctx.bool("watch"),
    ));
    if !msg.is_empty() {
        qovery_log::println_info(msg);
        return;
    }
    qovery_log::println(format!(
        "Cronjob {} deployment cancelled!",
        FG_BLUE.sprint(&name)
    ));
}

// ---- clone ------------------------------------------------------------------------------------------------

pub(crate) fn clone() -> Command {
    Command::new("clone")
        .short("Clone a cronjob")
        .flags(context_flags())
        .flag(cronjob_flag().required())
        .flag(Flag::string("target-project", "Target Project Name"))
        .flag(Flag::string(
            "target-environment",
            "Target Environment Name",
        ))
        .flag(Flag::string("target-cronjob-name", "Target Cronjob Name"))
        .run(run_clone)
}

fn run_clone(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, project_id, env_id) =
        check_error(resources::ids_from_flags(&client, ctx));
    let name = ctx.string("cronjob");

    // getJobContextResource
    let job = if env_id.trim().is_empty() {
        None
    } else {
        list_jobs(&client, &env_id)
            .ok()
            .and_then(|jobs| find_by_job_name(&jobs, &name).cloned())
    };
    let job = match job.filter(Job::is_cron) {
        Some(j) => j,
        None => {
            qovery_log::println_error(format!("cronjobName {name} not found"));
            qovery_log::println_info("You can list all jobs with: qovery job list");
            exit(1)
        }
    };

    let target_project_name = ctx.string("target-project");
    let mut target_project_id = project_id;
    if !target_project_name.is_empty() {
        target_project_id = check_error(resources::project_id(
            &client,
            &target_project_name,
            &organization_id,
        ));
    }

    let target_environment_name = ctx.string("target-environment");
    let mut target_environment_id = env_id;
    if !target_environment_name.is_empty() {
        target_environment_id = check_error(resources::environment_id(
            &client,
            &target_environment_name,
            &target_project_id,
        ));
    }

    let mut target_name = ctx.string("target-cronjob-name");
    if target_name.is_empty() {
        target_name = job.name.clone();
    }

    let body = json!({"name": target_name, "environment_id": target_environment_id});
    let res = client.execute(
        Method::POST,
        &format!("/job/{}/clone", path_escape(&job.id)),
        &[],
        Some(&body),
    );
    let cloned: Value = match res.and_then(|r| decode(&r)) {
        Ok(v) => v,
        Err(e) => {
            if let Some(status) = e.status
                && status != 200
            {
                qovery_log::println_error(format!(
                    "status code: {} ; body: {}",
                    qovery_api::status_line(status),
                    e.body
                ));
            }
            exit_with_error(e)
        }
    };

    let name = cloned
        .get("name")
        .and_then(|n| n.as_str())
        .unwrap_or_default();
    qovery_log::println(format!("Job {} cloned!", FG_BLUE.sprint(name)));
}

/// Decode a JSON response body like the Go client (an empty body is an error there too).
fn decode(res: &qovery_api::Response) -> Result<Value, qovery_api::ApiError> {
    serde_json::from_slice(&res.body).map_err(|e| qovery_api::ApiError {
        status: Some(res.status),
        message: e.to_string(),
        body: res.text(),
    })
}

// ---- delete / deploy / redeploy / stop ----------------------------------------------------------------------

pub(crate) fn delete() -> Command {
    Command::new("delete")
        .short("Delete a cronjob")
        .flags(context_flags())
        .flag(cronjob_flag())
        .flag(Flag::string("cronjobs", CRONJOBS_USAGE))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_delete)
}

pub(crate) fn deploy() -> Command {
    Command::new("deploy")
        .short("Deploy a cronjob")
        .flags(context_flags())
        .flag(cronjob_flag())
        .flag(Flag::string("cronjobs", CRONJOBS_USAGE))
        .flag(
            Flag::string(
                "commit-id",
                "Cronjob Commit ID, or 'latest' for the newest commit of the branch",
            )
            .short('c'),
        )
        .flag(Flag::string("tag", "Cronjob Tag").short('t'))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_deploy)
}

pub(crate) fn redeploy() -> Command {
    Command::new("redeploy")
        .short("Redeploy a cronjob")
        .flags(context_flags())
        .flag(cronjob_flag().required())
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_redeploy)
}

pub(crate) fn stop() -> Command {
    Command::new("stop")
        .short("Stop a cronjob")
        .flags(context_flags())
        .flag(cronjob_flag())
        .flag(Flag::string("cronjobs", CRONJOBS_USAGE))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_stop)
}

/// `validateCronjobArguments(cronjobName, cronjobNames)`.
fn validate_cronjob_arguments(name: &str, names: &str) {
    if name.is_empty() && names.is_empty() {
        exit_with_error(
            "use either --cronjob \"<cronjob name>\" or --cronjobs \"<cronjob1 name>, <cronjob2 name>\" but not both at the same time",
        );
    }
    if !name.is_empty() && !names.is_empty() {
        exit_with_error("you can't use --cronjob and --cronjobs at the same time");
    }
}

/// `buildCronJobListFromCronjobNames(client, envId, cronjobName, cronjobNames)`.
fn build_cron_job_list(client: &Client, env_id: &str, name: &str, names: &str) -> Vec<Job> {
    let jobs = check_error(list_jobs(client, env_id));
    let mut list = Vec::new();
    if !name.is_empty() {
        list.push(find_cronjob_or_exit(&jobs, name));
    }
    if !names.is_empty() {
        for n in names.split(',') {
            match find_by_job_name(&jobs, n.trim()).filter(|j| j.is_cron()) {
                Some(j) => list.push(j.clone()),
                // Go prints the untrimmed name
                None => cronjob_not_found(n),
            }
        }
    }
    list
}

/// Common start of deploy / redeploy / stop / delete: client, arguments validation, environment, cron jobs.
fn prepare(ctx: &Ctx, extra_validation: impl Fn()) -> (Client, String, Vec<Job>, String) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let name = ctx.string("cronjob");
    let names = ctx.string("cronjobs");
    validate_cronjob_arguments(&name, &names);
    extra_validation();
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);
    let list = build_cron_job_list(&client, &env_id, &name, &names);
    (client, env_id, list, format!("{name}{names}"))
}

fn run_deploy(ctx: &Ctx) {
    let commit_id = ctx.string("commit-id");
    let tag = ctx.string("tag");
    let (client, env_id, list, names) = prepare(ctx, || {
        if !tag.is_empty() && !commit_id.is_empty() {
            exit_with_error("you can't use --tag and --commit-id at the same time");
        }
    });
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&list),
        ctx.bool("watch"),
    );
    check_error(qovery_services::deploy_jobs(
        &client, &env_id, &list, &commit_id, &tag,
    ));
    qovery_log::println(format!(
        "Request to deploy cronjob(s) {} has been queued..",
        FG_BLUE.sprint(&names)
    ));
    watch.wait(state::DEPLOYED);
}

fn run_redeploy(ctx: &Ctx) {
    let (client, env_id, list, names) = prepare(ctx, || {});
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&list),
        ctx.bool("watch"),
    );
    let job_id = qovery_services::get_job_id(&list[0]);
    let res = client
        .execute(
            Method::POST,
            &format!("/job/{}/deploy", path_escape(&job_id)),
            &[],
            Some(&json!({})),
        )
        .and_then(|r| decode(&r));
    check_error(res);
    qovery_log::println(format!(
        "Request to redeploy cronjob(s) {} has been queued..",
        FG_BLUE.sprint(&names)
    ));
    watch.wait(state::DEPLOYED);
}

/// `EnvironmentServiceIdsAllRequest{JobIds: ...}`.
fn job_ids_request(list: &[Job]) -> qovery_services::ServiceIdsAllRequest {
    qovery_services::ServiceIdsAllRequest {
        job_ids: list.iter().map(qovery_services::get_job_id).collect(),
        ..Default::default()
    }
}

fn run_stop(ctx: &Ctx) {
    let (client, env_id, list, names) = prepare(ctx, || {});
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&list),
        ctx.bool("watch"),
    );
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "stop",
        &job_ids_request(&list),
    ));
    qovery_log::println(format!(
        "Request to stop cronjob(s) {} has been queued...",
        FG_BLUE.sprint(&names)
    ));
    watch.wait(state::STOPPED);
}

fn run_delete(ctx: &Ctx) {
    let (client, env_id, list, _) = prepare(ctx, || {});
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&list),
        ctx.bool("watch"),
    );
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "delete",
        &job_ids_request(&list),
    ));
    watch.wait(state::DELETED);
}

// ---- list -------------------------------------------------------------------------------------------------

pub(crate) fn list() -> Command {
    Command::new("list")
        .short("List cronjobs")
        .flags(context_flags())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let cronjobs = check_error(list_cronjobs(&client, &env_id));
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{}/statuses", path_escape(&env_id))));

    if ctx.bool("json") {
        print!("{}", cronjob_json_output(&statuses.jobs, &cronjobs));
        return;
    }

    let data: Vec<Vec<String>> = cronjobs
        .iter()
        .map(|c| {
            vec![
                c.id.clone(),
                c.name.clone(),
                "Cronjob".to_string(),
                qovery_core::status::find_status_text_with_color(&statuses.jobs, &c.id),
                time_string_from_rfc3339(c.updated_at.as_deref().unwrap_or_default()),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Type", "Status", "Last Update"], &data);
}

/// `getCronjobJsonOutput(statuses, cronjobs)` (`null` when there is no cron job, like Go's nil slice).
fn cronjob_json_output(statuses: &[qovery_api::models::Status], cronjobs: &[Job]) -> String {
    if cronjobs.is_empty() {
        return "null".to_string();
    }
    let results: Vec<Value> = cronjobs
        .iter()
        .map(|c| {
            json!({
                "id": c.id,
                "name": c.name,
                "type": "Cronjob",
                "status": qovery_core::status::find_status(statuses, &c.id),
                "updated_at": qovery_services::to_iso8601(c.updated_at.as_deref()),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

// ---- update -----------------------------------------------------------------------------------------------

pub(crate) fn update() -> Command {
    Command::new("update")
        .short("Update a cronjob")
        .flags(context_flags())
        .flag(cronjob_flag())
        .flag(Flag::string("branch", "Cronjob Branch").short('b'))
        .flag(Flag::string("tag", "Cronjob Tag").short('t'))
        .flag(Flag::string("image-name", "Cronjob Image Name"))
        .run(run_update)
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let token = qovery_core::auth::get_access_token(false);
    let (token_type, token) = check_error(token);

    let tag = ctx.string("tag");
    let image_name = ctx.string("image-name");
    let branch = ctx.string("branch");
    if (!tag.is_empty() || !image_name.is_empty()) && !branch.is_empty() {
        exit_with_error("you can't use --tag or --image-name with --branch at the same time");
    }
    if tag.is_empty() && image_name.is_empty() && branch.is_empty() {
        exit_with_error("you must use --tag or --image-name or --branch");
    }

    let client = client::client_for(token_type, &token);
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let cronjobs = check_error(list_cronjobs(&client, &env_id));
    let name = ctx.string("cronjob");
    let cronjob = find_cronjob_or_exit(&cronjobs, &name);

    let docker = qovery_services::get_job_docker(&cronjob).is_some();
    let image = qovery_services::get_job_image(&cronjob).is_some();
    if docker && (!tag.is_empty() || !image_name.is_empty()) {
        exit_with_error(
            "you can't use --tag or --image-name with a cronjob targetting a Dockerfile. Use --branch instead",
        );
    }
    if image && !branch.is_empty() {
        exit_with_error(
            "you can't use --branch with a cronjob targetting an image. Use --tag and/or --image-name instead",
        );
    }

    let mut req = qovery_services::to_job_request(&cronjob);
    let source = req.source.get_or_insert_with(Default::default);
    if docker {
        if let Some(Some(d)) = source.docker.as_mut()
            && let Some(git) = d.git_repository.as_mut()
        {
            git.branch = Some(branch.clone());
        }
        source.image = Some(None);
    } else {
        if let Some(Some(i)) = source.image.as_mut() {
            if !tag.is_empty() {
                i.tag = Some(tag.clone());
            }
            if !image_name.is_empty() {
                i.image_name = Some(image_name.clone());
            }
        }
        source.docker = Some(None);
    }

    let res = client
        .execute(
            Method::PUT,
            &format!(
                "/job/{}",
                path_escape(&qovery_services::get_job_id(&cronjob))
            ),
            &[],
            Some(&req.to_value()),
        )
        .and_then(|r| decode(&r));
    if let Err(e) = res {
        let status = e.status.map(qovery_api::status_line).unwrap_or_default();
        exit_with_error(format!("status code: {} ; body: {}", status, e.body));
    }

    qovery_log::println(format!("Cronjob {} updated!", FG_BLUE.sprint(&name)));
}
