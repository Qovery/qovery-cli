//! Resolution of the `--organization` / `--project` / `--environment` / service flags against the API and the
//! current context. Port of the helpers of `cmd/service_list.go` and `pkg/usercontext`.
//!
//! When a name flag is empty the id comes from the context (prompting to set it up when missing, like Go);
//! otherwise the resource is looked up by name.

use crate::context::{
    Service, ServiceType, current_environment, current_organization, current_project,
    current_service,
};
use crate::find::find_by_name;
use qovery_api::Client;
use qovery_api::models::*;

/// `usercontext.GetOrganizationContextResourceId(client, organizationName)`.
pub fn organization_id(client: &Client, organization_name: &str) -> Result<String, String> {
    if organization_name.trim().is_empty() {
        return Ok(current_organization(true)?.id);
    }
    let orgs: Vec<Organization> = client.list("/organization").map_err(|e| e.to_string())?;
    find_by_name(&orgs, organization_name)
        .map(|o| o.id.clone())
        .ok_or_else(|| format!("organization {organization_name} not found"))
}

/// `getProjectContextResourceId(client, projectName, organizationId)`.
pub fn project_id(
    client: &Client,
    project_name: &str,
    organization_id: &str,
) -> Result<String, String> {
    if project_name.trim().is_empty() {
        return Ok(current_project(true)?.id);
    }
    if organization_id.trim().is_empty() {
        return Ok(String::new());
    }
    let projects: Vec<Project> = client
        .list(&format!("/organization/{organization_id}/project"))
        .map_err(|e| e.to_string())?;
    find_by_name(&projects, project_name)
        .map(|p| p.id.clone())
        .ok_or_else(|| format!("project {project_name} not found"))
}

/// `getEnvironmentContextResourceId(client, environmentName, projectId)`.
pub fn environment_id(
    client: &Client,
    environment_name: &str,
    project_id: &str,
) -> Result<String, String> {
    if environment_name.trim().is_empty() {
        return Ok(current_environment(true)?.id);
    }
    if project_id.trim().is_empty() {
        return Ok(String::new());
    }
    let envs: Vec<Environment> = client
        .list(&format!("/project/{project_id}/environment"))
        .map_err(|e| e.to_string())?;
    find_by_name(&envs, environment_name)
        .map(|e| e.id.clone())
        .ok_or_else(|| format!("environment {environment_name} not found"))
}

/// `getOrganizationProjectContextResourcesIds(client)` -> (organization id, project id).
pub fn organization_project_ids(
    client: &Client,
    organization_name: &str,
    project_name: &str,
) -> Result<(String, String), String> {
    let org = organization_id(client, organization_name)?;
    let project = project_id(client, project_name, &org)?;
    Ok((org, project))
}

/// `getOrganizationProjectEnvironmentContextResourcesIds(client)` -> (organization id, project id, environment id).
pub fn organization_project_environment_ids(
    client: &Client,
    organization_name: &str,
    project_name: &str,
    environment_name: &str,
) -> Result<(String, String, String), String> {
    let org = organization_id(client, organization_name)?;
    let project = project_id(client, project_name, &org)?;
    let env = environment_id(client, environment_name, &project)?;
    Ok((org, project, env))
}

/// Same as [`organization_project_environment_ids`], reading the `organization`, `project` and
/// `environment` flags of the command (empty when the command does not define them).
pub fn ids_from_flags(
    client: &Client,
    ctx: &qovery_cobra::Ctx,
) -> Result<(String, String, String), String> {
    organization_project_environment_ids(
        client,
        &ctx.string("organization"),
        &ctx.string("project"),
        &ctx.string("environment"),
    )
}

/// `getEnvironmentIdFromContextPanicInCaseOfError(client)`: environment id from the flags/context, exit 1 on error.
pub fn environment_id_from_flags_or_exit(client: &Client, ctx: &qovery_cobra::Ctx) -> String {
    crate::check_error(ids_from_flags(client, ctx)).2
}

/// `getApplicationContextResource`.
pub fn application_by_name(
    client: &Client,
    name: &str,
    environment_id: &str,
) -> Result<Option<Application>, String> {
    if environment_id.trim().is_empty() {
        return Ok(None);
    }
    let items: Vec<Application> = client
        .list(&format!("/environment/{environment_id}/application"))
        .map_err(|e| e.to_string())?;
    find_by_name(&items, name)
        .cloned()
        .map(Some)
        .ok_or_else(|| format!("application {name} not found"))
}

/// `getContainerContextResource`.
pub fn container_by_name(
    client: &Client,
    name: &str,
    environment_id: &str,
) -> Result<Option<Container>, String> {
    if environment_id.trim().is_empty() {
        return Ok(None);
    }
    let items: Vec<Container> = client
        .list(&format!("/environment/{environment_id}/container"))
        .map_err(|e| e.to_string())?;
    find_by_name(&items, name)
        .cloned()
        .map(Some)
        .ok_or_else(|| format!("container {name} not found"))
}

/// `getJobContextResource`.
pub fn job_by_name(
    client: &Client,
    name: &str,
    environment_id: &str,
) -> Result<Option<Job>, String> {
    if environment_id.trim().is_empty() {
        return Ok(None);
    }
    let items: Vec<Job> = client
        .list(&format!("/environment/{environment_id}/job"))
        .map_err(|e| e.to_string())?;
    // Go `FindByJobName` only considers cron and lifecycle jobs
    let items: Vec<Job> = items
        .into_iter()
        .filter(|j| j.is_cron() || j.is_lifecycle())
        .collect();
    find_by_name(&items, name)
        .cloned()
        .map(Some)
        .ok_or_else(|| format!("job {name} not found"))
}

/// `getHelmContextResource`.
pub fn helm_by_name(
    client: &Client,
    name: &str,
    environment_id: &str,
) -> Result<Option<Helm>, String> {
    if environment_id.trim().is_empty() {
        return Ok(None);
    }
    let items: Vec<Helm> = client
        .list(&format!("/environment/{environment_id}/helm"))
        .map_err(|e| e.to_string())?;
    find_by_name(&items, name)
        .cloned()
        .map(Some)
        .ok_or_else(|| format!("helm {name} not found"))
}

/// `getDatabaseContextResource`. The Go code reports `application <applicationName> not found`, using the
/// global `--application` flag value (usually empty): pass it as `application_name_for_error`.
pub fn database_by_name(
    client: &Client,
    name: &str,
    environment_id: &str,
    application_name_for_error: &str,
) -> Result<Option<Database>, String> {
    if environment_id.trim().is_empty() {
        return Ok(None);
    }
    let items: Vec<Database> = client
        .list(&format!("/environment/{environment_id}/database"))
        .map_err(|e| e.to_string())?;
    find_by_name(&items, name)
        .cloned()
        .map(Some)
        .ok_or_else(|| format!("application {application_name_for_error} not found"))
}

/// `getServiceContextResourceId(client, serviceName, environmentId)`.
pub fn service_by_name(
    client: &Client,
    service_name: &str,
    environment_id: &str,
) -> Result<Option<Service>, String> {
    if service_name.trim().is_empty() {
        return current_service(true).map(Some);
    }
    if environment_id.trim().is_empty() {
        return Ok(None);
    }
    let mk = |id: &str, name: &str, t: ServiceType| Service {
        id: id.to_string(),
        name: name.to_string(),
        service_type: t.as_str().to_string(),
        blueprint_id: String::new(),
    };
    if let Ok(Some(a)) = application_by_name(client, service_name, environment_id) {
        return Ok(Some(mk(&a.id, &a.name, ServiceType::Application)));
    }
    if let Ok(Some(c)) = container_by_name(client, service_name, environment_id) {
        return Ok(Some(mk(&c.id, &c.name, ServiceType::Container)));
    }
    if let Ok(Some(j)) = job_by_name(client, service_name, environment_id)
        && (j.is_cron() || j.is_lifecycle())
    {
        return Ok(Some(mk(&j.id, &j.name, ServiceType::Job)));
    }
    if let Ok(Some(h)) = helm_by_name(client, service_name, environment_id) {
        return Ok(Some(mk(&h.id, &h.name, ServiceType::Helm)));
    }
    if let Ok(Some(d)) = database_by_name(client, service_name, environment_id, "") {
        return Ok(Some(mk(&d.id, &d.name, ServiceType::Database)));
    }
    Err(format!("service {service_name} not found"))
}
