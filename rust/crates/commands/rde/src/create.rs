//! `qovery rde create`. Port of `cmd/rde_create.go`.

use crate::helpers::*;
use qovery_api::Client;
use qovery_api::models::{Cluster, Environment, Job, Project};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit_with_error, telemetry};
use qovery_envvars::create_environment_variable;
use qovery_services::{get_job_id, get_job_name};
use serde_json::{Map, Value, json};

pub fn command() -> Command {
    Command::new("create")
        .short("Provision a new Remote Development Environment from a blueprint")
        .long(
            "Create a new RDE by cloning a blueprint environment into a new project.

This command:
  1. Creates a new project for the RDE
  2. Creates an RBAC role with scoped permissions (unless --skip-rbac)
  3. Clones the blueprint environment into the new project
  4. Updates the TTL job to target the new environment (if present)
  5. Invites the developer via email (unless --skip-invite)
  6. Triggers deployment (unless --skip-deploy)",
        )
        .flag(
            Flag::string("blueprint", "Blueprint Project Name to clone from")
                .short('b')
                .required(),
        )
        .flag(
            Flag::string(
                "name",
                "Name for the new RDE (will create project rde-<name>)",
            )
            .short('n')
            .required(),
        )
        .flag(Flag::string("email", "Email address to invite the developer").short('e'))
        .flag(Flag::string("cluster", "Cluster Name where to create the RDE").short('c'))
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .flag(Flag::bool("skip-rbac", "Skip RBAC role creation"))
        .flag(Flag::bool("skip-invite", "Skip member invitation"))
        .flag(Flag::bool("skip-deploy", "Skip deployment after cloning"))
        .run(run_create)
}

fn run_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));

    let bp_name = ctx.string("blueprint");
    let name = ctx.string("name");
    let email = ctx.string("email");
    let cluster_name = ctx.string("cluster");
    let skip_rbac = ctx.bool("skip-rbac");
    let mut skip_invite = ctx.bool("skip-invite");
    let skip_deploy = ctx.bool("skip-deploy");

    if bp_name.is_empty() {
        exit_with_error("--blueprint is required");
    }
    if name.is_empty() {
        exit_with_error("--name is required");
    }
    if email.is_empty() && !skip_invite {
        qovery_log::println_info(
            "No --email provided, skipping member invitation (use --skip-invite to suppress this message)",
        );
        skip_invite = true;
    }

    // Step 1: Resolve blueprint
    qovery_log::println(format!("Resolving blueprint {}...", blue(&bp_name)));
    let bp = match find_blueprint_by_project_name(&client, &org, &bp_name) {
        Ok(bp) => bp,
        Err(e) => exit_with_error(e),
    };
    if bp.env_id.is_empty() {
        exit_with_error(format!(
            "blueprint {} has no environment with {BLUEPRINT_KEY_VAR} set",
            bp.project_name
        ));
    }
    qovery_log::println(format!(
        "  Blueprint: {} (env: {})",
        bp.project_name, bp.env_id
    ));

    // Step 2: Create project
    let project_name = format!("rde-{name}");
    qovery_log::println(format!(
        "\nStep 1/6: Creating project {}...",
        blue(&project_name)
    ));
    let desc = format!("RDE for {name} (blueprint: {})", bp.project_name);
    let project: Project = match client.post(
        &format!("/organization/{}/project", esc(&org)),
        &json!({"name": project_name, "description": desc}),
    ) {
        Ok(p) => p,
        Err(err) => match find_project_by_name(&client, &org, &project_name) {
            Ok(existing) => {
                qovery_log::println_info(format!(
                    "Project {project_name} already exists, reusing..."
                ));
                existing
            }
            Err(_) => exit_with_error(format!("failed to create project: {err}")),
        },
    };
    qovery_log::println(format!("  Project: {}", project.id));

    // Step 3: Create RBAC role
    let mut role_id = String::new();
    if !skip_rbac {
        let role_name = format!("RDE-{name}");
        qovery_log::println(format!(
            "\nStep 2/6: Creating RBAC role {}...",
            blue(&role_name)
        ));
        let role_desc = format!("Access to {project_name} only");
        match client.post::<Value, _>(
            &format!("/organization/{}/customRole", esc(&org)),
            &json!({"name": role_name, "description": role_desc}),
        ) {
            Err(err) => match find_custom_role_by_name(&client, &org, &role_name) {
                Ok(Some(Some(id))) => {
                    qovery_log::println_info(format!(
                        "Role {role_name} already exists, reusing..."
                    ));
                    role_id = id;
                }
                _ => {
                    qovery_log::println_error(format!("failed to create RBAC role: {err}"));
                    qovery_log::println_info(
                        "Continuing without RBAC role (use --skip-rbac to suppress)",
                    );
                }
            },
            Ok(role) => {
                if let Some(id) = role["id"].as_str() {
                    role_id = id.to_string();
                }
            }
        }

        if !role_id.is_empty() {
            if let Err(e) = set_role_permissions(
                &client,
                &org,
                &role_id,
                &role_name,
                &project.id,
                &cluster_name,
            ) {
                qovery_log::println_error(format!("failed to set role permissions: {e}"));
                qovery_log::println_info("RBAC role created but permissions may be incomplete");
            }
            qovery_log::println(format!("  Role: {role_id}"));
        }
    } else {
        qovery_log::println("\nStep 2/6: Skipping RBAC role creation (--skip-rbac)");
    }

    // Step 4: Clone blueprint
    qovery_log::println("\nStep 3/6: Cloning blueprint environment...");
    let mut cluster_id: Option<String> = None;
    if let Ok(bp_env) = client.get::<Environment>(&format!("/environment/{}", esc(&bp.env_id))) {
        cluster_id = Some(bp_env.cluster_id.clone());
        let display = bp_env
            .cluster_name
            .clone()
            .unwrap_or(bp_env.cluster_id.clone());
        qovery_log::println(format!("  Using blueprint cluster: {}", blue(&display)));
    }

    if !cluster_name.is_empty()
        && let Ok(clusters) =
            client.list::<Cluster>(&format!("/organization/{}/cluster", esc(&org)))
    {
        match clusters.iter().find(|c| eq_fold(&c.name, &cluster_name)) {
            Some(c) => {
                cluster_id = Some(c.id.clone());
                qovery_log::println(format!("  Overriding cluster to: {}", blue(&cluster_name)));
            }
            None => exit_with_error(format!("cluster {cluster_name} not found")),
        }
    }

    let clone_req = clone_request(&project.id, cluster_id.as_deref());
    let cloned_env_id = match client.post::<Environment, _>(
        &format!("/environment/{}/clone", esc(&bp.env_id)),
        &clone_req,
    ) {
        Ok(env) => env.id,
        Err(err) => match find_blueprint_env(&client, &project.id) {
            Ok(Some(info)) => {
                qovery_log::println_info("Environment already exists, reusing...");
                info.env_id
            }
            _ => exit_with_error(format!("failed to clone blueprint: {err}")),
        },
    };
    qovery_log::println(format!("  Environment: {cloned_env_id}"));

    if !email.is_empty() {
        let _ = create_environment_variable(
            &client,
            &project.id,
            &cloned_env_id,
            OWNER_EMAIL_VAR,
            &email,
            false,
            false,
        );
    }

    // Step 5: Update TTL job
    qovery_log::println("\nStep 4/6: Checking for TTL job...");
    update_ttl_job(&client, &cloned_env_id);

    // Step 6: Invite member
    if !skip_invite && !email.is_empty() {
        qovery_log::println(format!("\nStep 5/6: Inviting {email}..."));
        let mut req = Map::new();
        req.insert("email".into(), json!(email));
        if !role_id.is_empty() {
            req.insert("role_id".into(), json!(role_id));
        }
        match client.post::<Value, _>(
            &format!("/organization/{}/inviteMember", esc(&org)),
            &Value::Object(req),
        ) {
            Err(e) => qovery_log::println_info(format!("Invitation failed or already sent: {e}")),
            Ok(_) => qovery_log::println(format!("  Invited: {email}")),
        }
    } else {
        qovery_log::println("\nStep 5/6: Skipping invitation");
    }

    // Step 7: Deploy
    if !skip_deploy {
        qovery_log::println("\nStep 6/6: Deploying...");
        match deploy_environment(&client, &cloned_env_id) {
            Err(e) => qovery_log::println_info(format!("Deploy failed: {e} (deploy from Console)")),
            Ok(()) => qovery_log::println("  Deployment triggered"),
        }
    } else {
        qovery_log::println("\nStep 6/6: Skipping deployment (--skip-deploy)");
    }

    qovery_log::println("");
    qovery_log::println(format!("RDE {} provisioned successfully!", blue(&name)));
    qovery_log::println("");
    print_key_value_table(vec![
        vec!["Project".to_string(), project.id.clone()],
        vec!["Environment".to_string(), cloned_env_id.clone()],
        vec![
            "Console".to_string(),
            console_url(&org, &project.id, &cloned_env_id),
        ],
    ]);
    qovery_log::println("");
    qovery_log::println_info("Workspace URL will be available once deployment completes.");
}

/// `qovery.CloneEnvironmentRequest{Name: "workspace", ProjectId, Mode: DEVELOPMENT, ClusterId?}`.
pub fn clone_request(project_id: &str, cluster_id: Option<&str>) -> Value {
    let mut m = Map::new();
    m.insert("name".into(), json!("workspace"));
    if let Some(c) = cluster_id {
        m.insert("cluster_id".into(), json!(c));
    }
    m.insert("mode".into(), json!("DEVELOPMENT"));
    m.insert("project_id".into(), json!(project_id));
    Value::Object(m)
}

const ENV_MODES: [&str; 4] = ["DEVELOPMENT", "STAGING", "PRODUCTION", "PREVIEW"];

/// Go `rdeSetRolePermissions`.
fn set_role_permissions(
    client: &Client,
    org_id: &str,
    role_id: &str,
    role_name: &str,
    target_project_id: &str,
    cluster_name: &str,
) -> Result<(), String> {
    let clusters: Vec<Cluster> = client
        .list(&format!("/organization/{}/cluster", esc(org_id)))
        .map_err(|e| format!("failed to list clusters: {e}"))?;
    let cluster_perms: Vec<Value> = clusters
        .iter()
        .map(|c| {
            let perm = if cluster_name.is_empty() || eq_fold(&c.name, cluster_name) {
                "ENV_CREATOR"
            } else {
                "VIEWER"
            };
            json!({"cluster_id": c.id, "permission": perm})
        })
        .collect();

    let projects =
        list_projects(client, org_id).map_err(|e| format!("failed to list projects: {e}"))?;
    let project_perms: Vec<Value> = projects
        .iter()
        .map(|p| {
            let perms = if p.id == target_project_id {
                ["DEPLOYER", "VIEWER", "NO_ACCESS", "DEPLOYER"]
            } else {
                ["NO_ACCESS"; 4]
            };
            let permissions: Vec<Value> = ENV_MODES
                .iter()
                .zip(perms)
                .map(|(mode, perm)| {
                    json!({"environment_type": mode, "permission": perm, "shell": false, "port_forward": false})
                })
                .collect();
            json!({"project_id": p.id, "is_admin": false, "permissions": permissions})
        })
        .collect();

    // Go nil slices are serialized as null
    let nil_or = |v: Vec<Value>| {
        if v.is_empty() {
            Value::Null
        } else {
            Value::Array(v)
        }
    };
    let body = json!({
        "name": role_name,
        "cluster_permissions": nil_or(cluster_perms),
        "project_permissions": nil_or(project_perms),
    });
    client
        .put::<Value, _>(
            &format!("/organization/{}/customRole/{}", esc(org_id), esc(role_id)),
            &body,
        )
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// Go `rdeUpdateTTLJob`.
pub fn update_ttl_job(client: &Client, env_id: &str) {
    let jobs: Vec<Job> = match client.list(&format!("/environment/{}/job", esc(env_id))) {
        Ok(j) => j,
        Err(_) => {
            qovery_log::println("  No jobs found (non-critical)");
            return;
        }
    };
    for job in &jobs {
        if get_job_name(job) == "ttl-auto-shutdown" {
            qovery_log::println(format!("  Found TTL job: {}", get_job_id(job)));
            qovery_log::println("  TTL job preserved from blueprint clone");
            return;
        }
    }
    qovery_log::println("  No TTL job found (non-critical)");
}
