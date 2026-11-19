//! `qovery lifecycle` command. Port of `cmd/lifecycle*.go`.

use qovery_api::Client;
use qovery_api::models::{EnvironmentStatuses, Job};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::find::find_by_name;
use qovery_core::{check_error, client, exit, exit_with_error, resources, status, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{path_escape, state};
use serde_json::{Map, Value, json};

/// The `lifecycle` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("lifecycle")
        .short("Manage lifecycle jobs")
        .run(run_group)
        .subcommand(cancel())
        .subcommand(clone())
        .subcommand(delete())
        .subcommand(deploy())
        .subcommand(env())
        .subcommand(external_secret())
        .subcommand(list())
        .subcommand(redeploy())
        .subcommand(stop())
        .subcommand(update())
}

/// Group commands with a `Run` that prints the help when called without arguments.
fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

/// `--organization`, `--project` and `--environment` flags.
fn with_context_flags(cmd: Command) -> Command {
    cmd.flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
}

fn blue(s: &str) -> String {
    FG_BLUE.sprint(s)
}

/// `client.JobsAPI.ListJobs(ctx, envId)`.
fn list_jobs(client: &Client, env_id: &str) -> Result<Vec<Job>, String> {
    client
        .list(&format!("/environment/{}/job", path_escape(env_id)))
        .map_err(|e| e.to_string())
}

/// `ListLifecycleJobs(envId, client)` (cmd/lifecycle.go).
fn list_lifecycle_jobs(client: &Client, env_id: &str) -> Result<Vec<Job>, String> {
    Ok(list_jobs(client, env_id)?
        .into_iter()
        .filter(Job::is_lifecycle)
        .collect())
}

fn lifecycle_not_found(name: &str) -> ! {
    qovery_log::println_error(format!("lifecycle {name} not found"));
    qovery_log::println_info("You can list all lifecycles with: qovery lifecycle list");
    exit(1)
}

/// `utils.FindByJobName(jobs, name)` + the `lifecycle == nil || lifecycle.LifecycleJobResponse == nil` check.
fn find_lifecycle(jobs: &[Job], name: &str) -> Job {
    match find_by_name(jobs, name) {
        Some(job) if job.is_lifecycle() => job.clone(),
        _ => lifecycle_not_found(name),
    }
}

/// The common prologue of the env / external-secret sub-commands: (organization id, project id, env id, lifecycle).
fn context_lifecycle(ctx: &Ctx) -> (Client, String, String, String, Job) {
    let client = check_error(client::get_client());
    let (organization_id, project_id, env_id) =
        check_error(resources::ids_from_flags(&client, ctx));
    let jobs = check_error(list_jobs(&client, &env_id));
    let lifecycle = find_lifecycle(&jobs, &ctx.string("lifecycle"));
    (client, organization_id, project_id, env_id, lifecycle)
}

// ---- list -------------------------------------------------------------------------------------------------

fn list() -> Command {
    with_context_flags(Command::new("list").short("List lifecycle jobs"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let lifecycles = check_error(list_lifecycle_jobs(&client, &env_id));
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{}/statuses", path_escape(&env_id))));

    if ctx.bool("json") {
        print!("{}", lifecycle_json_output(&statuses, &lifecycles));
        return;
    }

    let data: Vec<Vec<String>> = lifecycles
        .iter()
        .map(|l| {
            vec![
                l.id.clone(),
                l.name.clone(),
                "Lifecycle".to_string(),
                status::find_status_text_with_color(&statuses.jobs, &l.id),
                time_string_from_rfc3339(l.updated_at.as_deref().unwrap_or_default()),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Type", "Status", "Last Update"], &data);
}

/// `getLifecycleJsonOutput(statuses, lifecycles)`.
fn lifecycle_json_output(statuses: &EnvironmentStatuses, lifecycles: &[Job]) -> String {
    if lifecycles.is_empty() {
        // json.Marshal of a nil slice
        return "null".to_string();
    }
    let results: Vec<Value> = lifecycles
        .iter()
        .map(|l| {
            json!({
                "id": l.id,
                "name": l.name,
                "type": "Lifecycle",
                "status": status::find_status(&statuses.jobs, &l.id),
                "updated_at": qovery_services::to_iso8601(l.updated_at.as_deref()),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

// ---- deploy / redeploy / stop / delete ------------------------------------------------------------------

fn deploy() -> Command {
    with_context_flags(Command::new("deploy").short("Deploy a lifecycle job"))
        .flag(Flag::string("lifecycle", "Lifecycle Job Name").short('n'))
        .flag(Flag::string("lifecycles", "Lifecycle Job Names"))
        .flag(
            Flag::string(
                "commit-id",
                "Lifecycle Commit ID, or 'latest' for the newest commit of the branch",
            )
            .short('c'),
        )
        .flag(Flag::string("tag", "Lifecycle Tag").short('t'))
        .flag(
            Flag::bool(
                "watch",
                "Watch lifecycle status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_deploy)
}

fn run_deploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let (name, names) = (ctx.string("lifecycle"), ctx.string("lifecycles"));
    validate_lifecycle_arguments(&name, &names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);

    let (tag, commit_id) = (ctx.string("tag"), ctx.string("commit-id"));
    if !tag.is_empty() && !commit_id.is_empty() {
        exit_with_error("you can't use --tag and --commit-id at the same time");
    }

    let lifecycle_list = build_lifecycle_list_from_lifecycle_names(&client, &env_id, &name, &names);
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&lifecycle_list),
        ctx.bool("watch"),
    );
    check_error(qovery_services::deploy_jobs(
        &client,
        &env_id,
        &lifecycle_list,
        &commit_id,
        &tag,
    ));
    qovery_log::println(format!(
        "Request to deploy lifecycle job(s) {} has been queued..",
        blue(&format!("{name}{names}"))
    ));
    watch.wait(state::DEPLOYED);
}

fn redeploy() -> Command {
    with_context_flags(Command::new("redeploy").short("Redeploy a lifecycle job"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::bool(
                "watch",
                "Watch lifecycle status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_redeploy)
}

fn run_redeploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let (name, names) = (ctx.string("lifecycle"), ctx.string("lifecycles"));
    validate_lifecycle_arguments(&name, &names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);

    let lifecycle_list = build_lifecycle_list_from_lifecycle_names(&client, &env_id, &name, &names);
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&lifecycle_list),
        ctx.bool("watch"),
    );
    let job_id = qovery_services::get_job_id(&lifecycle_list[0]);
    // JobDeployRequest{}: every field is optional and unset
    check_error(
        client
            .post::<Value, _>(
                &format!("/job/{}/deploy", path_escape(&job_id)),
                &Value::Object(Map::new()),
            )
            .map_err(|e| e.to_string()),
    );
    qovery_log::println(format!(
        "Request to redeploy lifecycle job(s) {} has been queued..",
        blue(&format!("{name}{names}"))
    ));
    watch.wait(state::DEPLOYED);
}

fn stop() -> Command {
    with_context_flags(Command::new("stop").short("Stop a lifecycle job"))
        .flag(Flag::string("lifecycle", "Lifecycle Name").short('n'))
        .flag(Flag::string(
            "lifecycles",
            "Lifecycle Job Names (comma separated) (ex: --lifecycles \"lifecycle1,lifecycle2\")",
        ))
        .flag(
            Flag::bool(
                "watch",
                "Watch lifecycle status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_stop)
}

fn run_stop(ctx: &Ctx) {
    run_selected_services_action(ctx, "stop", state::STOPPED);
}

fn delete() -> Command {
    with_context_flags(Command::new("delete").short("Delete a lifecycle job"))
        .flag(Flag::string("lifecycle", "Lifecycle Job Name").short('n'))
        .flag(Flag::string(
            "lifecycles",
            "Lifecycle Job Names (comma separated) (ex: --lifecycles \"lifecycle1,lifecycle2\")",
        ))
        .flag(
            Flag::bool(
                "watch",
                "Watch lifecycle job status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    run_selected_services_action(ctx, "delete", state::DELETED);
}

/// `lifecycle stop` / `lifecycle delete`: `POST /environment/{id}/service/<action>` with the job ids.
fn run_selected_services_action(ctx: &Ctx, action: &str, final_state: &str) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let (name, names) = (ctx.string("lifecycle"), ctx.string("lifecycles"));
    validate_lifecycle_arguments(&name, &names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);

    let lifecycle_list = build_lifecycle_list_from_lifecycle_names(&client, &env_id, &name, &names);
    let watch = qovery_services::new_services_watch(
        &client,
        &env_id,
        qovery_services::job_ids(&lifecycle_list),
        ctx.bool("watch"),
    );
    let job_ids: Vec<String> = lifecycle_list
        .iter()
        .map(qovery_services::get_job_id)
        .collect();
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        action,
        &qovery_services::ServiceIdsAllRequest {
            job_ids,
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to {action} lifecycle job(s) {} has been queued...",
        blue(&format!("{name}{names}"))
    ));
    watch.wait(final_state);
}

/// `buildLifecycleListFromLifecycleNames(client, environmentId, lifecycleName, lifecycleNames)`.
fn build_lifecycle_list_from_lifecycle_names(
    client: &Client,
    environment_id: &str,
    name: &str,
    names: &str,
) -> Vec<Job> {
    let mut lifecycle_list = Vec::new();
    let lifecycles = check_error(list_jobs(client, environment_id));

    if !name.is_empty() {
        lifecycle_list.push(find_lifecycle(&lifecycles, name));
    }
    if !names.is_empty() {
        for lifecycle_name in names.split(',') {
            let trimmed = lifecycle_name.trim();
            match find_by_name(&lifecycles, trimmed) {
                Some(job) if job.is_lifecycle() => lifecycle_list.push(job.clone()),
                // the error shows the untrimmed name, like Go
                _ => lifecycle_not_found(lifecycle_name),
            }
        }
    }
    lifecycle_list
}

/// `validateLifecycleArguments(lifecycleName, lifecycleNames)`.
fn validate_lifecycle_arguments(name: &str, names: &str) {
    if name.is_empty() && names.is_empty() {
        exit_with_error(
            "use either --lifecycle \"<lifecycle name>\" or --lifecycles \"<lifecycle1 name>, <lifecycle2 name>\" but not both at the same time",
        );
    }
    if !name.is_empty() && !names.is_empty() {
        exit_with_error("you can't use --lifecycle and --lifecycles at the same time");
    }
}

// ---- cancel -----------------------------------------------------------------------------------------------

fn cancel() -> Command {
    with_context_flags(Command::new("cancel").short("Cancel a lifecycle deployment"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(Flag::bool("watch", "Watch cancel until it's done or an error occurs").short('w'))
        .run(run_cancel)
}

fn run_cancel(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, _, env_id, lifecycle) = context_lifecycle(ctx);
    let msg = check_error(qovery_services::cancel_service_deployment(
        &client,
        &env_id,
        &lifecycle.id,
        ServiceType::Job,
        ctx.bool("watch"),
    ));
    if !msg.is_empty() {
        qovery_log::println_info(msg);
        return;
    }
    qovery_log::println(format!(
        "Lifecycle {} deployment cancelled!",
        blue(&ctx.string("lifecycle"))
    ));
}

// ---- clone ------------------------------------------------------------------------------------------------

fn clone() -> Command {
    with_context_flags(Command::new("clone").short("Clone a lifecycle job"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(Flag::string("target-project", "Target Project Name"))
        .flag(Flag::string(
            "target-environment",
            "Target Environment Name",
        ))
        .flag(Flag::string(
            "target-lifecycle-name",
            "Target Lifecycle Name",
        ))
        .run(run_clone)
}

fn run_clone(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (organization_id, project_id, env_id) =
        check_error(resources::ids_from_flags(&client, ctx));
    let lifecycle_name = ctx.string("lifecycle");

    let job = match resources::job_by_name(&client, &lifecycle_name, &env_id) {
        Ok(Some(job)) if job.is_lifecycle() => job,
        _ => {
            qovery_log::println_error(format!("lifecycle {lifecycle_name} not found"));
            qovery_log::println_info("You can list all jobs with: qovery job list");
            exit(1)
        }
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

    let mut target_lifecycle_name = ctx.string("target-lifecycle-name");
    if target_lifecycle_name.is_empty() {
        // use same job name as the source job
        target_lifecycle_name = job.name.clone();
    }

    let req = json!({ "name": target_lifecycle_name, "environment_id": target_environment_id });
    let cloned: Value = match client.post(&format!("/job/{}/clone", path_escape(&job.id)), &req) {
        Ok(v) => v,
        Err(e) => {
            // print http body error message
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
    qovery_log::println(format!("Job {} cloned!", blue(name)));
}

// ---- update -----------------------------------------------------------------------------------------------

fn update() -> Command {
    with_context_flags(Command::new("update").short("Update a lifecycle"))
        .flag(Flag::string("lifecycle", "Lifecycle Name").short('n'))
        .flag(Flag::string("branch", "Lifecycle Branch").short('b'))
        .flag(Flag::string("tag", "Lifecycle Tag").short('t'))
        .flag(Flag::string("image-name", "Lifecycle Image Name"))
        .run(run_update)
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (token_type, token) = check_error(qovery_core::auth::get_access_token(false));

    let (name, tag, image_name, branch) = (
        ctx.string("lifecycle"),
        ctx.string("tag"),
        ctx.string("image-name"),
        ctx.string("branch"),
    );
    if (!tag.is_empty() || !image_name.is_empty()) && !branch.is_empty() {
        exit_with_error("you can't use --tag or --image-name with --branch at the same time");
    }
    if tag.is_empty() && image_name.is_empty() && branch.is_empty() {
        exit_with_error("you must use --tag or --image-name or --branch");
    }

    let client = client::client_for(token_type, &token);
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let lifecycles = check_error(list_lifecycle_jobs(&client, &env_id));
    let lifecycle = find_lifecycle(&lifecycles, &name);

    let docker = qovery_services::get_job_docker(&lifecycle).is_some();
    let image = qovery_services::get_job_image(&lifecycle).is_some();

    if docker && (!tag.is_empty() || !image_name.is_empty()) {
        exit_with_error(
            "you can't use --tag or --image-name with a lifecycle targetting a Dockerfile. Use --branch instead",
        );
    }
    if image && !branch.is_empty() {
        exit_with_error(
            "you can't use --branch with a lifecycle targetting an image. Use --tag and/or --image-name instead",
        );
    }

    let mut req = qovery_services::to_job_request(&lifecycle);
    if let Some(source) = req.source.as_mut() {
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
    }

    if let Err(e) = client.put::<Value, _>(
        &format!("/job/{}", path_escape(&lifecycle.id)),
        &req.to_value(),
    ) {
        let status = e.status.map(qovery_api::status_line).unwrap_or_default();
        exit_with_error(format!("status code: {status} ; body: {}", e.body));
    }

    qovery_log::println(format!("Lifecycle {} updated!", blue(&name)));
}

// ---- env --------------------------------------------------------------------------------------------------

fn env() -> Command {
    Command::new("env")
        .short("Manage lifecycle environment variables and secrets")
        .run(run_group)
        .subcommand(env_alias())
        .subcommand(env_create())
        .subcommand(env_delete())
        .subcommand(env_list())
        .subcommand(env_override())
        .subcommand(env_update())
}

fn env_list() -> Command {
    with_context_flags(Command::new("list").short("List lifecycle environment variables"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(Flag::bool("show-values", "Show env var values"))
        .flag(Flag::bool("pretty-print", "Pretty print output"))
        .flag(Flag::bool("sort", "Sort environment variables by key"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_env_list)
}

fn run_env_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, _, _, lifecycle) = context_lifecycle(ctx);
    let env_vars = check_error(qovery_envvars::list_service_variables(
        &client,
        &lifecycle.id,
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

fn env_create() -> Command {
    with_context_flags(
        Command::new("create").short("Create lifecycle environment variable or secret"),
    )
    .flag(
        Flag::string("lifecycle", "Lifecycle Name")
            .short('n')
            .required(),
    )
    .flag(
        Flag::string("key", "Environment variable or secret key")
            .short('k')
            .required(),
    )
    .flag(
        Flag::string("value", "Environment variable or secret value")
            .short('v')
            .required(),
    )
    .flag(
        Flag::string("scope", "Scope of this env var <PROJECT|ENVIRONMENT|JOB>")
            .default_value("JOB"),
    )
    .flag(Flag::bool(
        "secret",
        "This environment variable is a secret",
    ))
    .flag(Flag::bool(
        "override-if-exists",
        "If the key already exists at a higher scope, create an override of it instead of failing",
    ))
    .run(run_env_create)
}

fn run_env_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, project_id, env_id, lifecycle) = context_lifecycle(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_variable(
        &client,
        &project_id,
        &env_id,
        &lifecycle.id,
        &ctx.string("scope"),
        &key,
        &ctx.string("value"),
        ctx.bool("secret"),
        ctx.bool("override-if-exists"),
    ));
    qovery_log::println(format!(
        "Environment variable {} has been created",
        blue(&key)
    ));
}

fn env_delete() -> Command {
    with_context_flags(
        Command::new("delete").short("Delete lifecycle environment variable or secret"),
    )
    .flag(
        Flag::string("lifecycle", "Lifecycle Name")
            .short('n')
            .required(),
    )
    .flag(
        Flag::string("key", "Environment variable or secret key")
            .short('k')
            .required(),
    )
    .run(run_env_delete)
}

fn run_env_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, _, _, lifecycle) = context_lifecycle(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &lifecycle.id,
        ServiceType::Job,
        &key,
    ));
    qovery_log::println(format!("Variable {} has been deleted", blue(&key)));
}

fn env_update() -> Command {
    with_context_flags(
        Command::new("update").short("Update lifecycle environment variable or secret"),
    )
    .flag(
        Flag::string("lifecycle", "Lifecycle Name")
            .short('n')
            .required(),
    )
    .flag(
        Flag::string("key", "Environment variable or secret key")
            .short('k')
            .required(),
    )
    .flag(
        Flag::string("value", "Environment variable or secret value")
            .short('v')
            .required(),
    )
    .run(run_env_update)
}

fn run_env_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, _, _, lifecycle) = context_lifecycle(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::update_service_variable(
        &client,
        &key,
        &ctx.string("value"),
        &lifecycle.id,
        ServiceType::Job,
    ));
    qovery_log::println(format!(
        "Environment variable {} has been updated",
        blue(&key)
    ));
}

fn env_alias() -> Command {
    Command::new("alias")
        .short("Manage lifecycle environment variable and secret aliases")
        .run(run_group)
        .subcommand(env_alias_create())
}

fn env_alias_create() -> Command {
    with_context_flags(
        Command::new("create").short("Create lifecycle environment variable or secret alias"),
    )
    .flag(
        Flag::string("lifecycle", "Lifecycle Name")
            .short('n')
            .required(),
    )
    .flag(
        Flag::string("key", "Environment variable or secret key")
            .short('k')
            .required(),
    )
    .flag(Flag::string("alias", "Environment variable or secret alias").required())
    .flag(
        Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT|JOB>").default_value("JOB"),
    )
    .run(run_env_alias_create)
}

fn run_env_alias_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, _, env_id, lifecycle) = context_lifecycle(ctx);
    let alias = ctx.string("alias");
    // Go passes the package-level `projectId` variable (bound to `admin delete-project --project`), always
    // empty here, instead of the resolved project id.
    check_error(qovery_envvars::create_service_alias(
        &client,
        "",
        &env_id,
        &lifecycle.id,
        ServiceType::Job,
        &ctx.string("key"),
        &alias,
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("Alias {} has been created", blue(&alias)));
}

fn env_override() -> Command {
    Command::new("override")
        .short("Manage lifecycle environment variable and secret overrides")
        .run(run_group)
        .subcommand(env_override_create())
}

fn env_override_create() -> Command {
    with_context_flags(
        Command::new("create").short("Override lifecycle environment variable or secret"),
    )
    .flag(
        Flag::string("lifecycle", "Lifecycle Name")
            .short('n')
            .required(),
    )
    .flag(
        Flag::string("key", "Environment variable or secret key")
            .short('k')
            .required(),
    )
    .flag(Flag::string("value", "Environment variable or secret value").required())
    .flag(
        Flag::string("scope", "Scope of this alias <PROJECT|ENVIRONMENT|JOB>").default_value("JOB"),
    )
    .run(run_env_override_create)
}

fn run_env_override_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, project_id, env_id, lifecycle) = context_lifecycle(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::create_service_override(
        &client,
        &project_id,
        &env_id,
        &lifecycle.id,
        ServiceType::Job,
        &key,
        &ctx.string("value"),
        &ctx.string("scope"),
    ));
    qovery_log::println(format!("{} has been overidden", blue(&key)));
}

// ---- external-secret --------------------------------------------------------------------------------------

fn external_secret() -> Command {
    Command::new("external-secret")
        .short("Manage lifecycle external secrets")
        .run(run_group)
        .subcommand(external_secret_create())
        .subcommand(external_secret_delete())
        .subcommand(external_secret_update())
}

fn external_secret_create() -> Command {
    with_context_flags(Command::new("create").short("Create lifecycle external secret"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("key", "External secret key")
                .short('k')
                .required(),
        )
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
    telemetry::capture(ctx);
    let (client, organization_id, project_id, env_id, lifecycle) = context_lifecycle(ctx);
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
        &lifecycle.id,
        &ctx.string("scope"),
        &key,
        &ctx.string("reference"),
        &secret_manager_access_id,
        &ctx.string("mount-path"),
    ));
    qovery_log::println(format!("External secret {} has been created", blue(&key)));
}

fn external_secret_delete() -> Command {
    with_context_flags(Command::new("delete").short("Delete lifecycle external secret"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("key", "External secret key")
                .short('k')
                .required(),
        )
        .run(run_external_secret_delete)
}

fn run_external_secret_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, _, _, lifecycle) = context_lifecycle(ctx);
    let key = ctx.string("key");
    check_error(qovery_envvars::delete_service_variable(
        &client,
        &lifecycle.id,
        ServiceType::Job,
        &key,
    ));
    qovery_log::println(format!("External secret {} has been deleted", blue(&key)));
}

fn external_secret_update() -> Command {
    with_context_flags(Command::new("update").short("Update lifecycle external secret"))
        .flag(
            Flag::string("lifecycle", "Lifecycle Name")
                .short('n')
                .required(),
        )
        .flag(
            Flag::string("key", "External secret key")
                .short('k')
                .required(),
        )
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
    let (client, organization_id, _, env_id, lifecycle) = context_lifecycle(ctx);
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
        &lifecycle.id,
        ServiceType::Job,
    ));
    qovery_log::println(format!("External secret {} has been updated", blue(&key)));
}
