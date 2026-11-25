//! `qovery container deploy|redeploy|restart|stop|delete|cancel`. Port of `cmd/container_{deploy,redeploy,
//! restart,stop,delete,cancel}.go`.

use crate::{
    client_or_exit, container_flag, container_not_found, fail, find_container_or_exit, ids_or_exit,
    list_containers, with_context_flags,
};
use qovery_api::models::Container;
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::ServiceType;
use qovery_core::find::find_by_name;
use qovery_core::resources::environment_id_from_flags_or_exit;
use qovery_core::{check_error, client, telemetry};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{RebootServicesRequest, container_ids, new_services_watch, state};
use serde_json::json;

const WATCH_USAGE: &str = "Watch container status until it's ready or an error occurs";
const CONTAINERS_USAGE: &str =
    "Container Names (comma separated) (ex: --containers \"container1,container2\")";

pub fn deploy() -> Command {
    with_context_flags(Command::new("deploy").short("Deploy a container"))
        .flag(container_flag())
        .flag(Flag::string("containers", CONTAINERS_USAGE))
        .flag(Flag::string("tag", "Container Tag (image containers only)").short('t'))
        .flag(Flag::string("commit-id", "Container Commit ID (Git containers only)").short('c'))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_deploy)
}

pub fn redeploy() -> Command {
    with_context_flags(Command::new("redeploy").short("Redeploy a container"))
        .flag(container_flag().required())
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_redeploy)
}

pub fn restart() -> Command {
    with_context_flags(Command::new("restart").short("Restart a container without redeploying it"))
        .flag(container_flag())
        .flag(Flag::string(
            "containers",
            "Container Names (comma separated) Example: --containers \"container1,container2\"",
        ))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_restart)
}

pub fn stop() -> Command {
    with_context_flags(Command::new("stop").short("Stop a container"))
        .flag(container_flag())
        .flag(Flag::string("containers", CONTAINERS_USAGE))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_stop)
}

pub fn delete() -> Command {
    with_context_flags(Command::new("delete").short("Delete a container"))
        .flag(container_flag())
        .flag(Flag::string("containers", CONTAINERS_USAGE))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_delete)
}

pub fn cancel() -> Command {
    with_context_flags(Command::new("cancel").short("Cancel a container deployment"))
        .flag(container_flag().required())
        .flag(Flag::bool("watch", "Watch cancel until it's done or an error occurs").short('w'))
        .run(run_cancel)
}

/// `validateContainerArguments(containerName, containerNames)`.
fn validate_container_arguments(container_name: &str, container_names: &str) {
    if container_name.is_empty() && container_names.is_empty() {
        fail(
            "use either --container \"<container name>\" or --containers \"<container1 name>, <container2 name>\" but not both at the same time",
        );
    }
    if !container_name.is_empty() && !container_names.is_empty() {
        fail("you can't use --container and --containers at the same time");
    }
}

/// `buildContainerListFromContainerNames(client, envId, containerName, containerNames)`.
fn build_container_list(
    client: &Client,
    env_id: &str,
    container_name: &str,
    container_names: &str,
) -> Vec<Container> {
    let containers = list_containers(client, env_id);
    let mut list = Vec::new();
    if !container_name.is_empty() {
        match find_by_name(&containers, container_name) {
            Some(c) => list.push(c.clone()),
            None => container_not_found(container_name),
        }
    }
    if !container_names.is_empty() {
        for name in container_names.split(',') {
            match find_by_name(&containers, name.trim()) {
                Some(c) => list.push(c.clone()),
                // Go reports the untrimmed name
                None => container_not_found(name),
            }
        }
    }
    list
}

/// Common prologue of the multi-container actions: client, arguments, environment and container list.
fn prepare(ctx: &Ctx) -> (Client, String, Vec<Container>) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    validate_container_arguments(&ctx.string("container"), &ctx.string("containers"));
    let env_id = environment_id_from_flags_or_exit(&client, ctx);
    let list = build_container_list(
        &client,
        &env_id,
        &ctx.string("container"),
        &ctx.string("containers"),
    );
    (client, env_id, list)
}

fn names_label(ctx: &Ctx) -> String {
    FG_BLUE.sprint(format!(
        "{}{}",
        ctx.string("container"),
        ctx.string("containers")
    ))
}

/// `validateContainerDeployVersion(tag, commitId)`: checked before signing in.
fn validate_container_deploy_version(tag: &str, commit_id: &str) -> Result<(), String> {
    if !tag.is_empty() && !commit_id.is_empty() {
        return Err("use either --tag or --commit-id, not both".to_string());
    }
    if qovery_services::is_latest_commit_keyword(commit_id) {
        return Err(format!(
            "--commit-id {} is not supported for Git containers: pass a commit SHA",
            qovery_services::LATEST_COMMIT_KEYWORD
        ));
    }
    Ok(())
}

/// `checkContainersMatchDeployVersion(containerList, tag, commitId)`: checked before `--watch` reads statuses.
fn check_containers_match_deploy_version(
    list: &[Container],
    tag: &str,
    commit_id: &str,
) -> Result<(), String> {
    for container in list {
        if commit_id.is_empty() {
            qovery_services::check_container_image_tag(container, tag)?;
        } else {
            qovery_services::check_container_commit_id(container)?;
        }
    }
    Ok(())
}

fn run_deploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (tag, commit_id) = (ctx.string("tag"), ctx.string("commit-id"));
    check_error(validate_container_deploy_version(&tag, &commit_id));
    let client = client::get_client_or_exit();
    validate_container_arguments(&ctx.string("container"), &ctx.string("containers"));
    let env_id = environment_id_from_flags_or_exit(&client, ctx);
    let list = build_container_list(
        &client,
        &env_id,
        &ctx.string("container"),
        &ctx.string("containers"),
    );
    check_error(check_containers_match_deploy_version(
        &list, &tag, &commit_id,
    ));
    let watch = new_services_watch(&client, &env_id, container_ids(&list), ctx.bool("watch"));
    // `deployContainers`: --commit-id deploys each container at that commit, otherwise deploy-all
    check_error(if commit_id.is_empty() {
        qovery_services::deploy_containers(&client, &env_id, &list, &tag)
    } else {
        qovery_services::deploy_containers_at_commit(&client, &list, &commit_id)
    });
    qovery_log::println(format!(
        "Request to deploy container(s) {} has been queued..",
        names_label(ctx)
    ));
    watch.wait(state::DEPLOYED);
}

fn run_redeploy(ctx: &Ctx) {
    let (client, env_id, list) = prepare(ctx);
    let watch = new_services_watch(&client, &env_id, container_ids(&list), ctx.bool("watch"));
    check_error(client.execute(
        Method::POST,
        &format!("/container/{}/deploy", list[0].id),
        &[],
        Some(&json!({})),
    ));
    qovery_log::println(format!(
        "Request to redeploy container(s) {} has been queued..",
        names_label(ctx)
    ));
    watch.wait(state::DEPLOYED);
}

fn run_restart(ctx: &Ctx) {
    let (client, env_id, list) = prepare(ctx);
    let ids = container_ids(&list);
    let watch = new_services_watch(&client, &env_id, ids.clone(), ctx.bool("watch"));
    check_error(qovery_services::reboot_services(
        &client,
        &env_id,
        &RebootServicesRequest {
            container_ids: Some(ids),
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to restart container(s) {} has been queued...",
        names_label(ctx)
    ));
    watch.wait(state::RESTARTED);
}

fn run_stop(ctx: &Ctx) {
    let (client, env_id, list) = prepare(ctx);
    let ids = container_ids(&list);
    let watch = new_services_watch(&client, &env_id, ids.clone(), ctx.bool("watch"));
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "stop",
        &qovery_services::ServiceIdsAllRequest {
            container_ids: ids,
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to stop container(s) {} has been queued...",
        ctx.string("container")
    ));
    watch.wait(state::STOPPED);
}

fn run_delete(ctx: &Ctx) {
    let (client, env_id, list) = prepare(ctx);
    let ids = container_ids(&list);
    let watch = new_services_watch(&client, &env_id, ids.clone(), ctx.bool("watch"));
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "delete",
        &qovery_services::ServiceIdsAllRequest {
            container_ids: ids,
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to delete container(s) {} has been queued...",
        names_label(ctx)
    ));
    watch.wait(state::DELETED);
}

fn run_cancel(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let (_, _, env_id) = ids_or_exit(&client, ctx);
    let name = ctx.string("container");
    let container = find_container_or_exit(&client, &env_id, &name);

    let msg = check_error(qovery_services::cancel_service_deployment(
        &client,
        &env_id,
        &container.id,
        ServiceType::Container,
        ctx.bool("watch"),
    ));
    if !msg.is_empty() {
        qovery_log::println_info(msg);
        return;
    }
    qovery_log::println(format!(
        "Container {} deployment cancelled!",
        FG_BLUE.sprint(&name)
    ));
}
