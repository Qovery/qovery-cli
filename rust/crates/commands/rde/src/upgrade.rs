//! `qovery rde upgrade`. Port of `cmd/rde_upgrade.go`.

use crate::create::{clone_request, update_ttl_job};
use crate::helpers::*;
use crate::sync::{SyncOptions, sync_services_from_blueprint};
use qovery_api::Client;
use qovery_api::models::Environment;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit_with_error, telemetry};
use qovery_envvars::create_environment_variable;
use std::collections::HashMap;
use std::time::Duration;

const UPGRADE_LONG: &str = "Upgrade one or all RDE environments using one of two strategies:

  image   (default) - Redeploy the environment (re-pulls latest images)
  reclone           - Delete the environment, re-clone from blueprint, and deploy
                      WARNING: uncommitted changes will be lost with reclone

If --name is provided, upgrades a single RDE. Otherwise, upgrades all RDEs.
When upgrading multiple RDEs with reclone, environments are deleted in parallel
for faster execution.";

pub fn command() -> Command {
    Command::new("upgrade")
        .short("Upgrade RDE(s) from the updated blueprint")
        .long(UPGRADE_LONG)
        .flag(Flag::string("name", "RDE Name (omit to upgrade all)").short('n'))
        .flag(
            Flag::string(
                "strategy",
                "Upgrade strategy: 'image' (sync source and deploy) or 'reclone' (full re-clone)",
            )
            .short('s')
            .default_value("image"),
        )
        .flag(
            Flag::string(
                "blueprint",
                "Filter by Blueprint Project Name (when upgrading all)",
            )
            .short('b'),
        )
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .flag(Flag::bool(
            "sync-all",
            "Sync all config from blueprint (resources, ports, healthchecks, storage)",
        ))
        .flag(Flag::bool(
            "sync-resources",
            "Also sync CPU, memory, and instance counts from blueprint",
        ))
        .flag(Flag::bool(
            "sync-ports",
            "Also sync port configuration from blueprint",
        ))
        .flag(Flag::bool(
            "sync-healthchecks",
            "Also sync health check configuration from blueprint",
        ))
        .flag(Flag::bool(
            "sync-storage",
            "Also sync storage volumes from blueprint",
        ))
        .run(run_upgrade)
}

fn run_upgrade(ctx: &Ctx) {
    telemetry::capture(ctx);

    let mut strategy = ctx.string("strategy");
    if strategy.is_empty() {
        strategy = "image".to_string();
    }
    if strategy != "image" && strategy != "reclone" {
        exit_with_error(format!(
            "unknown strategy '{strategy}'. Use 'image' or 'reclone'"
        ));
    }

    let opts = SyncOptions {
        all: ctx.bool("sync-all"),
        resources: ctx.bool("sync-resources"),
        ports: ctx.bool("sync-ports"),
        healthchecks: ctx.bool("sync-healthchecks"),
        storage: ctx.bool("sync-storage"),
    };

    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let name = ctx.string("name");

    if !name.is_empty() {
        let child = match find_child_by_name(&client, &org, &format!("rde-{name}")) {
            Ok(c) => c,
            Err(e) => exit_with_error(e),
        };
        if strategy == "image" {
            upgrade_image(&client, &child, &opts);
        } else {
            upgrade_reclone_single(&client, &child);
        }
        return;
    }

    let children = children_from_flag(&client, ctx, &org);
    if children.is_empty() {
        qovery_log::println("No RDE instances found.");
        return;
    }

    qovery_log::println(format!(
        "Upgrading {} RDE(s) (strategy: {strategy})...",
        children.len()
    ));
    if strategy == "image" {
        for child in &children {
            upgrade_image(&client, child, &opts);
        }
    } else {
        upgrade_reclone_all(&client, &children);
    }
    qovery_log::println("\nAll RDEs upgraded.");
}

/// Go `rdeUpgradeImage`.
fn upgrade_image(client: &Client, child: &ChildInfo, opts: &SyncOptions) {
    let name = trim_rde(&child.project_name);
    qovery_log::println(format!(
        "  Upgrading {} (strategy: image - sync from blueprint and deploy)...",
        blue(&name)
    ));

    let bp_env = match find_blueprint_env(client, &child.blueprint_project_id) {
        Ok(Some(e)) => e,
        _ => {
            qovery_log::println(format!(
                "    ERROR: Could not find blueprint environment for {name}"
            ));
            return;
        }
    };

    let synced = sync_services_from_blueprint(client, &bp_env.env_id, &child.env_id, opts);
    if synced == 0 {
        qovery_log::println(format!(
            "    No services matched between blueprint and {name}, deploying as-is..."
        ));
    } else {
        qovery_log::println(format!("    Synced {synced} service(s) from blueprint."));
    }

    match deploy_environment(client, &child.env_id) {
        Err(e) => qovery_log::println(format!("    WARNING: Deploy failed for {name}: {e}")),
        Ok(()) => qovery_log::println(format!(
            "    Request to deploy {} has been queued..",
            blue(&name)
        )),
    }
}

/// Go `rdeUpgradeRecloneSingle`.
fn upgrade_reclone_single(client: &Client, child: &ChildInfo) {
    let name = trim_rde(&child.project_name);
    qovery_log::println(format!(
        "  Upgrading {} (strategy: reclone - full re-clone from blueprint)...",
        blue(&name)
    ));
    qovery_log::println("    WARNING: Uncommitted changes will be lost. Code in git is safe.");

    let bp_env = match find_blueprint_env(client, &child.blueprint_project_id) {
        Ok(Some(e)) => e,
        _ => {
            qovery_log::println(format!(
                "    ERROR: Could not find blueprint environment for {name}"
            ));
            return;
        }
    };

    let bp_cluster_id = get_blueprint_cluster_id(client, &bp_env.env_id);
    let owner_email = child.owner_email.clone();

    qovery_log::println(format!(
        "    Deleting environment {}...",
        blue(&child.env_name)
    ));
    let _ = delete_environment(client, &child.env_id);

    qovery_log::println("    Waiting for deletion to complete...");
    wait_for_envs_deletion(
        client,
        std::slice::from_ref(&child.env_id),
        Duration::from_secs(120),
    );

    let Some(new_env) = clone_from_blueprint(client, child, &bp_env.env_id, &bp_cluster_id) else {
        return;
    };

    if !owner_email.is_empty() {
        let _ = create_environment_variable(
            client,
            &child.project_id,
            &new_env.id,
            OWNER_EMAIL_VAR,
            &owner_email,
            false,
            false,
        );
    }

    update_ttl_job(client, &new_env.id);

    match deploy_environment(client, &new_env.id) {
        Err(e) => qovery_log::println(format!(
            "    WARNING: Deploy failed after re-clone for {name}: {e}"
        )),
        Ok(()) => qovery_log::println(format!(
            "    Re-cloned and deploying {} (env: {})",
            blue(&name),
            new_env.id
        )),
    }
}

/// Go `rdeUpgradeRecloneAll`.
fn upgrade_reclone_all(client: &Client, children: &[ChildInfo]) {
    qovery_log::println("  WARNING: Uncommitted changes will be lost. Code in git is safe.");
    qovery_log::println("");

    // blueprint project id -> (blueprint env id, cluster id)
    let mut bp_refs: HashMap<String, Option<(String, String)>> = HashMap::new();
    for child in children {
        if bp_refs.contains_key(&child.blueprint_project_id) {
            continue;
        }
        if let Ok(Some(env)) = find_blueprint_env(client, &child.blueprint_project_id) {
            let cluster_id = get_blueprint_cluster_id(client, &env.env_id);
            bp_refs.insert(
                child.blueprint_project_id.clone(),
                Some((env.env_id, cluster_id)),
            );
        }
        // Go only stores the entry on success: a failed lookup is retried for the next child
    }

    qovery_log::println("  Phase 1/4: Deleting old environments...");
    let mut env_ids = Vec::new();
    for child in children {
        let name = trim_rde(&child.project_name);
        let _ = delete_environment(client, &child.env_id);
        env_ids.push(child.env_id.clone());
        qovery_log::println(format!("    Delete requested: {}", blue(&name)));
    }

    qovery_log::println("");
    qovery_log::println("  Phase 2/4: Waiting for deletions to complete...");
    wait_for_envs_deletion(client, &env_ids, Duration::from_secs(180));
    qovery_log::println("    Deletions complete.");

    qovery_log::println("");
    qovery_log::println("  Phase 3/4: Cloning from blueprint...");
    let mut results: Vec<(&ChildInfo, Environment)> = Vec::new();
    for child in children {
        let name = trim_rde(&child.project_name);
        let Some(Some((bp_env_id, cluster_id))) = bp_refs.get(&child.blueprint_project_id) else {
            qovery_log::println(format!(
                "    ERROR: No blueprint environment found for {name}, skipping"
            ));
            continue;
        };
        let Some(new_env) = clone_from_blueprint(client, child, bp_env_id, cluster_id) else {
            continue;
        };
        if !child.owner_email.is_empty() {
            let _ = create_environment_variable(
                client,
                &child.project_id,
                &new_env.id,
                OWNER_EMAIL_VAR,
                &child.owner_email,
                false,
                false,
            );
        }
        update_ttl_job(client, &new_env.id);
        qovery_log::println(format!("    Cloned: {} (env: {})", blue(&name), new_env.id));
        results.push((child, new_env));
    }

    qovery_log::println("");
    qovery_log::println("  Phase 4/4: Deploying...");
    for (child, new_env) in &results {
        let name = trim_rde(&child.project_name);
        match deploy_environment(client, &new_env.id) {
            Err(e) => qovery_log::println(format!("    WARNING: Deploy failed for {name}: {e}")),
            Ok(()) => qovery_log::println(format!(
                "    Request to deploy {} has been queued..",
                blue(&name)
            )),
        }
    }
}

/// Go `rdeCloneFromBlueprint`.
fn clone_from_blueprint(
    client: &Client,
    child: &ChildInfo,
    blueprint_env_id: &str,
    cluster_id: &str,
) -> Option<Environment> {
    let name = trim_rde(&child.project_name);
    let req = clone_request(
        &child.project_id,
        (!cluster_id.is_empty()).then_some(cluster_id),
    );
    match client.post::<Environment, _>(
        &format!("/environment/{}/clone", esc(blueprint_env_id)),
        &req,
    ) {
        Ok(env) => Some(env),
        Err(e) => {
            qovery_log::println(format!("    ERROR: Re-clone failed for {name}: {e}"));
            None
        }
    }
}
