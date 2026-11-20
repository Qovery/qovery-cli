//! `qovery rde delete|delete-all`. Port of `cmd/rde_delete.go` and `cmd/rde_delete_all.go`.

use crate::helpers::*;
use qovery_api::Client;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit, telemetry};
use serde_json::Value;
use std::time::Duration;

pub fn delete() -> Command {
    Command::new("delete")
        .short("Delete an RDE (environment, project, RBAC role, and API token)")
        .long(
            "Fully remove an RDE by:
  1. Stopping the environment (if running)
  2. Deleting the environment
  3. Deleting the project
  4. Deleting the RBAC role RDE-<name> (if exists)
  5. Deleting the API token ttl-<name> (if exists)",
        )
        .flag(Flag::string("name", "RDE Name").short('n').required())
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .flag(Flag::bool("watch", "Watch deletion status").short('w'))
        .run(run_delete)
}

pub fn delete_all() -> Command {
    Command::new("delete-all")
        .short("Delete ALL RDE environments")
        .long(
            "Delete all RDE environments permanently. Requires --confirm flag.

This will delete the environment, project, RBAC role, and API token for each RDE.",
        )
        .flag(Flag::bool(
            "confirm",
            "Confirm deletion of all RDE environments",
        ))
        .flag(Flag::string("blueprint", "Filter by Blueprint Project Name").short('b'))
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .run(run_delete_all)
}

fn run_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let name = ctx.string("name");

    let project_name = format!("rde-{name}");
    qovery_log::println(format!("Deleting RDE {}...", blue(&name)));

    let project = match find_project_by_name(&client, &org, &project_name) {
        Ok(p) => p,
        Err(_) => {
            qovery_log::println_error(format!("RDE {name} not found (no project {project_name})"));
            cleanup_role_and_token(&client, &org, &name);
            exit(1);
        }
    };

    if let Ok(environments) = list_environments(&client, &project.id) {
        for env in &environments {
            let status = get_env_status(&client, &env.id).unwrap_or_default();
            if status != "STOPPED" && !status.is_empty() {
                qovery_log::println(format!("  Stopping environment {}...", blue(&env.name)));
                let _ = stop_environment(&client, &env.id);
                std::thread::sleep(Duration::from_secs(2));
            }
            qovery_log::println(format!("  Deleting environment {}...", blue(&env.name)));
            if let Err(e) = delete_environment(&client, &env.id) {
                qovery_log::println_info(format!("Failed to delete environment: {e}"));
            }
        }
    }

    qovery_log::println(format!("  Deleting project {}...", blue(&project_name)));
    if let Err(e) = delete_project(&client, &project.id) {
        qovery_log::println_info(format!("Failed to delete project: {e}"));
    }

    cleanup_role_and_token(&client, &org, &name);

    qovery_log::println(format!("\nRDE {} fully removed.", blue(&name)));
}

/// Go `rdeCleanupRoleAndToken`.
fn cleanup_role_and_token(client: &Client, org_id: &str, name: &str) {
    let role_name = format!("RDE-{name}");
    if let Ok(Some(Some(role_id))) = find_custom_role_by_name(client, org_id, &role_name) {
        qovery_log::println(format!("  Deleting role {}...", blue(&role_name)));
        let _ = client.delete(&format!(
            "/organization/{}/customRole/{}",
            esc(org_id),
            esc(&role_id)
        ));
    }

    let token_name = format!("ttl-{name}");
    if let Ok(tokens) = client.list::<Value>(&format!("/organization/{}/apiToken", esc(org_id))) {
        for token in &tokens {
            if token["name"].as_str() == Some(token_name.as_str()) {
                let id = token["id"].as_str().unwrap_or_default();
                if !id.is_empty() {
                    qovery_log::println(format!("  Deleting API token {}...", blue(&token_name)));
                    let _ = client.delete(&format!(
                        "/organization/{}/apiToken/{}",
                        esc(org_id),
                        esc(id)
                    ));
                }
                break;
            }
        }
    }
}

fn run_delete_all(ctx: &Ctx) {
    telemetry::capture(ctx);
    if !ctx.bool("confirm") {
        qovery_log::println_error("this will delete ALL RDE environments permanently");
        qovery_log::println("Run with --confirm to proceed: qovery rde delete-all --confirm");
        exit(1);
    }

    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let children = children_from_flag(&client, ctx, &org);

    if children.is_empty() {
        qovery_log::println("No RDE instances found.");
        return;
    }

    qovery_log::println(format!("Deleting {} RDE environment(s)...", children.len()));
    qovery_log::println("");

    for child in &children {
        let name = trim_rde(&child.project_name);
        qovery_log::println(format!("=== Deleting: {} ===", blue(&child.project_name)));

        if !child.env_id.is_empty() {
            let status = get_env_status(&client, &child.env_id).unwrap_or_default();
            if status != "STOPPED" && !status.is_empty() {
                qovery_log::println("  Stopping environment...");
                let _ = stop_environment(&client, &child.env_id);
                std::thread::sleep(Duration::from_secs(2));
            }
            qovery_log::println("  Deleting environment...");
            let _ = delete_environment(&client, &child.env_id);
        }

        qovery_log::println(format!("  Deleting project {}...", child.project_name));
        let _ = delete_project(&client, &child.project_id);

        cleanup_role_and_token(&client, &org, &name);
        qovery_log::println("");
    }

    qovery_log::println("All RDE environments deleted.");
}
