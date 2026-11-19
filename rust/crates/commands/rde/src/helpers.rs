//! Shared RDE helpers. Port of the helper types and functions of `cmd/rde.go`.

use chrono::{DateTime, FixedOffset, Utc};
use qovery_api::models::{
    Application, Container, Database, Environment, EnvironmentStatuses, Helm, Job, Project, Status,
};
use qovery_api::{ApiError, Client, Method};
use qovery_cobra::Ctx;
use qovery_core::{resources, status};
use qovery_envvars::{
    VariableResponse, find_environment_variable_by_key, list_environment_variables,
    list_project_variables,
};
use qovery_log::table::Table;
use qovery_services::{get_job_id, get_job_name, path_escape};
use serde_json::Value;
use std::collections::HashMap;
use std::time::{Duration, Instant};

pub const BLUEPRINT_PROJECT_ID_VAR: &str = "BLUEPRINT_PROJECT_ID";
pub const BLUEPRINT_KEY_VAR: &str = "BLUEPRINT_KEY";
pub const OWNER_EMAIL_VAR: &str = "RDE_OWNER_EMAIL";

/// Go `rdeBlueprintInfo`.
#[derive(Debug, Clone, Default)]
pub struct BlueprintInfo {
    pub project_id: String,
    pub project_name: String,
    pub env_id: String,
    pub env_name: String,
}

/// Go `rdeChildInfo`.
#[derive(Debug, Clone, Default)]
pub struct ChildInfo {
    pub project_id: String,
    pub project_name: String,
    pub env_id: String,
    pub env_name: String,
    pub blueprint_project_id: String,
    pub owner_email: String,
}

/// Go `envInfo`.
#[derive(Debug, Clone, Default)]
pub struct EnvInfo {
    pub env_id: String,
    pub env_name: String,
}

/// Go `strings.EqualFold`.
pub fn eq_fold(a: &str, b: &str) -> bool {
    a.chars()
        .flat_map(char::to_lowercase)
        .eq(b.chars().flat_map(char::to_lowercase))
}

/// `url.PathEscape` of a path parameter.
pub fn esc(s: &str) -> String {
    path_escape(s)
}

/// Go `fmt.Sprintf("rde-%s", name)` helper.
pub fn trim_rde(project_name: &str) -> String {
    project_name
        .strip_prefix("rde-")
        .unwrap_or(project_name)
        .to_string()
}

/// The value of a variable (`""` when null).
fn var_value(v: &VariableResponse) -> String {
    v.value.clone().unwrap_or_default()
}

/// Go `rdeGetOrgId`: `--organization` flag or the context.
pub fn org_id(client: &Client, ctx: &Ctx) -> Result<String, String> {
    resources::organization_id(client, &ctx.string("organization"))
}

pub fn list_projects(client: &Client, org_id: &str) -> Result<Vec<Project>, ApiError> {
    client.list(&format!("/organization/{}/project", esc(org_id)))
}

pub fn list_environments(client: &Client, project_id: &str) -> Result<Vec<Environment>, ApiError> {
    client.list(&format!("/project/{}/environment", esc(project_id)))
}

/// Go `rdeListBlueprintProjects`.
pub fn list_blueprint_projects(
    client: &Client,
    org_id: &str,
) -> Result<Vec<BlueprintInfo>, String> {
    let projects = list_projects(client, org_id).map_err(|e| e.to_string())?;
    let mut blueprints = Vec::new();
    for project in &projects {
        let Ok(vars) = list_project_variables(client, &project.id) else {
            continue;
        };
        let Some(bp_var) = find_environment_variable_by_key(BLUEPRINT_PROJECT_ID_VAR, &vars) else {
            continue;
        };
        if var_value(bp_var) != project.id {
            continue;
        }
        match find_blueprint_env(client, &project.id) {
            Ok(Some(env)) => blueprints.push(BlueprintInfo {
                project_id: project.id.clone(),
                project_name: project.name.clone(),
                env_id: env.env_id,
                env_name: env.env_name,
            }),
            _ => blueprints.push(BlueprintInfo {
                project_id: project.id.clone(),
                project_name: project.name.clone(),
                ..Default::default()
            }),
        }
    }
    Ok(blueprints)
}

/// Go `rdeFindBlueprintByProjectName`.
pub fn find_blueprint_by_project_name(
    client: &Client,
    org_id: &str,
    name: &str,
) -> Result<BlueprintInfo, String> {
    let projects = list_projects(client, org_id).map_err(|e| e.to_string())?;
    for project in &projects {
        if !eq_fold(&project.name, name) {
            continue;
        }
        let vars = list_project_variables(client, &project.id)
            .map_err(|e| format!("failed to read variables for project {name}: {e}"))?;
        let Some(bp_var) = find_environment_variable_by_key(BLUEPRINT_PROJECT_ID_VAR, &vars) else {
            return Err(format!(
                "project {name} is not a blueprint (missing {BLUEPRINT_PROJECT_ID_VAR} variable)"
            ));
        };
        let val = var_value(bp_var);
        if val != project.id {
            return Err(format!(
                "project {name} has invalid {BLUEPRINT_PROJECT_ID_VAR} variable (expected {}, got {val})",
                project.id
            ));
        }
        let env = find_blueprint_env(client, &project.id)?;
        let mut info = BlueprintInfo {
            project_id: project.id.clone(),
            project_name: project.name.clone(),
            ..Default::default()
        };
        if let Some(env) = env {
            info.env_id = env.env_id;
            info.env_name = env.env_name;
        }
        return Ok(info);
    }
    Err(format!("project {name} not found"))
}

/// Go `rdeFindBlueprintEnv`.
pub fn find_blueprint_env(client: &Client, project_id: &str) -> Result<Option<EnvInfo>, String> {
    let environments = list_environments(client, project_id).map_err(|e| e.to_string())?;
    for env in &environments {
        let Ok(vars) = list_environment_variables(client, &env.id) else {
            continue;
        };
        let Some(bk_var) = find_environment_variable_by_key(BLUEPRINT_KEY_VAR, &vars) else {
            continue;
        };
        if var_value(bk_var) == project_id {
            return Ok(Some(EnvInfo {
                env_id: env.id.clone(),
                env_name: env.name.clone(),
            }));
        }
    }
    Ok(None)
}

fn owner_email(vars: &[VariableResponse]) -> String {
    find_environment_variable_by_key(OWNER_EMAIL_VAR, vars)
        .map(var_value)
        .unwrap_or_default()
}

/// Go `rdeListChildren`.
pub fn list_children(
    client: &Client,
    org_id: &str,
    blueprint_project_id: &str,
) -> Result<Vec<ChildInfo>, String> {
    let projects = list_projects(client, org_id).map_err(|e| e.to_string())?;
    let mut children = Vec::new();
    for project in &projects {
        if project.id == blueprint_project_id {
            continue;
        }
        let Ok(environments) = list_environments(client, &project.id) else {
            continue;
        };
        for env in &environments {
            let Ok(vars) = list_environment_variables(client, &env.id) else {
                continue;
            };
            let Some(bk_var) = find_environment_variable_by_key(BLUEPRINT_KEY_VAR, &vars) else {
                continue;
            };
            if var_value(bk_var) == blueprint_project_id {
                children.push(ChildInfo {
                    project_id: project.id.clone(),
                    project_name: project.name.clone(),
                    env_id: env.id.clone(),
                    env_name: env.name.clone(),
                    blueprint_project_id: blueprint_project_id.to_string(),
                    owner_email: owner_email(&vars),
                });
            }
        }
    }
    Ok(children)
}

/// Go `rdeListAllChildren`.
pub fn list_all_children(client: &Client, org_id: &str) -> Result<Vec<ChildInfo>, String> {
    let blueprints = list_blueprint_projects(client, org_id)?;
    let mut all = Vec::new();
    for bp in &blueprints {
        if let Ok(children) = list_children(client, org_id, &bp.project_id) {
            all.extend(children);
        }
    }
    Ok(all)
}

/// Go `rdeFindChildByName`.
pub fn find_child_by_name(client: &Client, org_id: &str, name: &str) -> Result<ChildInfo, String> {
    let projects = list_projects(client, org_id).map_err(|e| e.to_string())?;
    for project in &projects {
        if !eq_fold(&project.name, name) {
            continue;
        }
        let environments = list_environments(client, &project.id)
            .map_err(|e| format!("failed to list environments for project {name}: {e}"))?;
        for env in &environments {
            let Ok(vars) = list_environment_variables(client, &env.id) else {
                continue;
            };
            let Some(bk_var) = find_environment_variable_by_key(BLUEPRINT_KEY_VAR, &vars) else {
                continue;
            };
            let val = var_value(bk_var);
            if !val.is_empty() && val != project.id {
                return Ok(ChildInfo {
                    project_id: project.id.clone(),
                    project_name: project.name.clone(),
                    env_id: env.id.clone(),
                    env_name: env.name.clone(),
                    blueprint_project_id: val,
                    owner_email: owner_email(&vars),
                });
            }
        }
        return Err(format!(
            "project {name} exists but is not an RDE child (no {BLUEPRINT_KEY_VAR} variable pointing to a different project)"
        ));
    }
    Err(format!("project {name} not found"))
}

/// Go `rdeGetEnvStatus`.
pub fn get_env_status(client: &Client, env_id: &str) -> Result<String, ApiError> {
    let statuses: EnvironmentStatuses =
        client.get(&format!("/environment/{}/statuses", esc(env_id)))?;
    Ok(statuses.environment.state)
}

/// Go `rdeGetWorkspaceUrl`.
pub fn get_workspace_url(client: &Client, env_id: &str) -> String {
    let apps: Vec<Application> =
        match client.list(&format!("/environment/{}/application", esc(env_id))) {
            Ok(a) if !a.is_empty() => a,
            _ => return String::new(),
        };
    let links: Vec<Value> = match client.list(&format!("/application/{}/link", esc(&apps[0].id))) {
        Ok(l) if !l.is_empty() => l,
        _ => return String::new(),
    };
    links[0]["url"].as_str().unwrap_or_default().to_string()
}

/// Go `rdeFormatUptime`.
pub fn format_uptime(deployed_at: Option<DateTime<FixedOffset>>) -> String {
    let Some(t) = deployed_at else {
        return "-".to_string();
    };
    let diff = Utc::now().signed_duration_since(t);
    let nanos = diff.num_nanoseconds().unwrap_or(i64::MAX) as f64;
    let secs = nanos / 1e9;
    let mins = secs / 60.0;
    let hours = mins / 60.0;
    if secs < 60.0 {
        format!("{}s", secs as i64)
    } else if mins < 60.0 {
        format!("{}m", mins as i64)
    } else if hours < 24.0 {
        format!("{}h {}m", hours as i64, (mins as i64) % 60)
    } else {
        format!("{}d {}h", (hours as i64) / 24, (hours as i64) % 24)
    }
}

/// Go `rdeGetLastDeployTime`.
pub fn get_last_deploy_time(client: &Client, env_id: &str) -> Option<DateTime<FixedOffset>> {
    let history: Vec<Value> = client
        .list(&format!("/environment/{}/deploymentHistory", esc(env_id)))
        .ok()?;
    let first = history.first()?;
    let created = first["created_at"].as_str().unwrap_or_default();
    Some(qovery_services::parse_api_time(created))
}

/// Go `rdeFindProjectByName`.
pub fn find_project_by_name(client: &Client, org_id: &str, name: &str) -> Result<Project, String> {
    let projects = list_projects(client, org_id).map_err(|e| e.to_string())?;
    projects
        .into_iter()
        .find(|p| eq_fold(&p.name, name))
        .ok_or_else(|| format!("project {name} not found"))
}

/// Go `rdeFindCustomRoleByName`: the role id of the first custom role named `role_name` (case-insensitive).
/// `Ok(Some(None))` when the role has no id.
pub fn find_custom_role_by_name(
    client: &Client,
    org_id: &str,
    role_name: &str,
) -> Result<Option<Option<String>>, ApiError> {
    let roles: Vec<Value> = client.list(&format!("/organization/{}/customRole", esc(org_id)))?;
    for role in &roles {
        if let Some(n) = role["name"].as_str()
            && eq_fold(n, role_name)
        {
            return Ok(Some(role["id"].as_str().map(|s| s.to_string())));
        }
    }
    Ok(None)
}

/// Go `rdePrintEnvServices`.
pub fn print_env_services(client: &Client, env_id: &str) {
    let statuses: EnvironmentStatuses =
        match client.get(&format!("/environment/{}/statuses", esc(env_id))) {
            Ok(s) => s,
            Err(_) => {
                qovery_log::println("  (could not retrieve statuses)");
                return;
            }
        };
    let e = esc(env_id);
    let mut names: HashMap<String, String> = HashMap::new();
    if let Ok(apps) = client.list::<Application>(&format!("/environment/{e}/application")) {
        for a in apps {
            names.insert(a.id, a.name);
        }
    }
    if let Ok(containers) = client.list::<Container>(&format!("/environment/{e}/container")) {
        for c in containers {
            names.insert(c.id, c.name);
        }
    }
    if let Ok(jobs) = client.list::<Job>(&format!("/environment/{e}/job")) {
        for j in &jobs {
            names.insert(get_job_id(j), get_job_name(j));
        }
    }
    if let Ok(dbs) = client.list::<Database>(&format!("/environment/{e}/database")) {
        for d in dbs {
            names.insert(d.id, d.name);
        }
    }
    if let Ok(helms) = client.list::<Helm>(&format!("/environment/{e}/helm")) {
        for h in helms {
            names.insert(h.id, h.name);
        }
    }

    let mut data: Vec<Vec<String>> = Vec::new();
    let mut collect = |list: &[Status], type_name: &str| {
        for s in list {
            let name = names
                .get(&s.id)
                .filter(|n| !n.is_empty())
                .cloned()
                .unwrap_or_else(|| s.id.clone());
            data.push(vec![
                name,
                type_name.to_string(),
                status::status_text_with_color(&s.state),
            ]);
        }
    };
    collect(&statuses.applications, "Application");
    collect(&statuses.containers, "Container");
    collect(&statuses.jobs, "Job");
    collect(&statuses.databases, "Database");
    collect(&statuses.helms, "Helm");

    if data.is_empty() {
        qovery_log::println("  No services found.");
        return;
    }
    qovery_log::print_table(&["Name", "Type", "Status"], &data);
}

/// Go `rdePrintKeyValueTable`: `pterm.DefaultTable.WithData(rows).Render()`.
pub fn print_key_value_table(rows: Vec<Vec<String>>) {
    Table::new(rows).render();
}

/// Go `rdeBlueprintNameForProjectId`.
pub fn blueprint_name_for_project_id(client: &Client, project_id: &str) -> String {
    match client.get::<Project>(&format!("/project/{}", esc(project_id))) {
        Ok(p) => p.name,
        Err(_) => project_id.to_string(),
    }
}

/// Go `rdeGetBlueprintClusterId`.
pub fn get_blueprint_cluster_id(client: &Client, blueprint_env_id: &str) -> String {
    match client.get::<Environment>(&format!("/environment/{}", esc(blueprint_env_id))) {
        Ok(env) => env.cluster_id,
        Err(_) => String::new(),
    }
}

/// Go `rdeIsEnvDeleted`: the statuses call answers 404.
pub fn is_env_deleted(client: &Client, env_id: &str) -> bool {
    match client.get::<EnvironmentStatuses>(&format!("/environment/{}/statuses", esc(env_id))) {
        Ok(_) => false,
        Err(e) => e.status == Some(404),
    }
}

/// Go `rdeWaitForEnvsDeletion` (the Go map is iterated in insertion order here).
pub fn wait_for_envs_deletion(client: &Client, env_ids: &[String], timeout: Duration) {
    let deadline = Instant::now() + timeout;
    let mut remaining: Vec<String> = Vec::new();
    for id in env_ids {
        if !remaining.contains(id) {
            remaining.push(id.clone());
        }
    }
    while !remaining.is_empty() && Instant::now() < deadline {
        remaining.retain(|id| !is_env_deleted(client, id));
        if !remaining.is_empty() {
            std::thread::sleep(Duration::from_secs(5));
        }
    }
    if !remaining.is_empty() {
        qovery_log::println_info(format!(
            "{} environment(s) did not finish deleting within timeout, proceeding anyway",
            remaining.len()
        ));
    }
}

/// `POST /environment/{id}/deploy` (Go `EnvironmentActionsAPI.DeployEnvironment`).
pub fn deploy_environment(client: &Client, env_id: &str) -> Result<(), ApiError> {
    client
        .execute(
            Method::POST,
            &format!("/environment/{}/deploy", esc(env_id)),
            &[],
            None,
        )
        .map(|_| ())
}

/// `POST /environment/{id}/stop` (Go `EnvironmentActionsAPI.StopEnvironment`).
pub fn stop_environment(client: &Client, env_id: &str) -> Result<(), ApiError> {
    client
        .execute(
            Method::POST,
            &format!("/environment/{}/stop", esc(env_id)),
            &[],
            None,
        )
        .map(|_| ())
}

/// `DELETE /environment/{id}`.
pub fn delete_environment(client: &Client, env_id: &str) -> Result<(), ApiError> {
    client
        .delete(&format!("/environment/{}", esc(env_id)))
        .map(|_| ())
}

/// `DELETE /project/{id}`.
pub fn delete_project(client: &Client, project_id: &str) -> Result<(), ApiError> {
    client
        .delete(&format!("/project/{}", esc(project_id)))
        .map(|_| ())
}

/// The console URL of an environment.
pub fn console_url(org_id: &str, project_id: &str, env_id: &str) -> String {
    format!(
        "https://console.qovery.com/organization/{org_id}/project/{project_id}/environment/{env_id}"
    )
}

/// Blue text (`pterm.FgBlue.Sprintf("%s", s)`).
pub fn blue(s: &str) -> String {
    qovery_log::pterm::FG_BLUE.sprint(s)
}

/// Children of the `--blueprint` flag (or of every blueprint), exiting on errors like the Go commands.
pub fn children_from_flag(client: &Client, ctx: &Ctx, org_id: &str) -> Vec<ChildInfo> {
    let bp_name = ctx.string("blueprint");
    if !bp_name.is_empty() {
        let bp = match find_blueprint_by_project_name(client, org_id, &bp_name) {
            Ok(bp) => bp,
            Err(e) => qovery_core::exit_with_error(e),
        };
        qovery_core::check_error(list_children(client, org_id, &bp.project_id))
    } else {
        qovery_core::check_error(list_all_children(client, org_id))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn uptime() {
        assert_eq!(format_uptime(None), "-");
        let now = Utc::now().fixed_offset();
        assert_eq!(
            format_uptime(Some(now - chrono::Duration::seconds(30))),
            "30s"
        );
        assert_eq!(
            format_uptime(Some(now - chrono::Duration::minutes(5))),
            "5m"
        );
        assert_eq!(
            format_uptime(Some(now - chrono::Duration::minutes(125))),
            "2h 5m"
        );
        assert_eq!(
            format_uptime(Some(now - chrono::Duration::hours(50))),
            "2d 2h"
        );
    }

    #[test]
    fn fold() {
        assert!(eq_fold("RDE-Foo", "rde-foo"));
        assert!(!eq_fold("rde-foo", "rde-fo"));
    }
}
