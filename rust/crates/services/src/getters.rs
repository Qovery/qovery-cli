//! Lookups by id of `utils/qovery.go` (`GetOrganizationById`, `GetApplicationById`, ...),
//! `GetServiceNameByIdAndType` and `GetDeploymentStageId`.
//!
//! Like Go, the `get_xxx_by_id(id)` functions create their own API client from the stored/env token
//! (`GetAccessToken(false)` + `GetQoveryClient`); the `_with` variants take an existing client.

use crate::util::{path_escape, str_field};
use qovery_api::models::EnvironmentStatuses;
use qovery_api::{ApiError, Client};
use qovery_core::client::get_client;
use qovery_core::context::{EnvironmentRef, IdName, Service, ServiceType};
use serde_json::Value;

/// Error of the `GetXxxById` helpers: `Received <status> response while getting <what> <id>` when the API
/// answered >= 400, the client error otherwise.
fn by_id_error(e: ApiError, what: &str) -> String {
    match e.status {
        Some(code) if code >= 400 => format!(
            "Received {} response while getting {what}",
            qovery_api::status_line(code)
        ),
        _ => e.to_string(),
    }
}

fn get_by_id(client: &Client, kind: &str, id: &str) -> Result<Value, String> {
    client
        .get::<Value>(&format!("/{kind}/{}", path_escape(id)))
        .map_err(|e| by_id_error(e, &format!("{kind} {id}")))
}

fn id_name(v: &Value) -> IdName {
    IdName {
        id: str_field(v, "id").to_string(),
        name: str_field(v, "name").to_string(),
    }
}

fn service(v: &Value, t: ServiceType) -> Service {
    Service {
        id: str_field(v, "id").to_string(),
        name: str_field(v, "name").to_string(),
        service_type: t.as_str().to_string(),
        blueprint_id: String::new(),
    }
}

/// Port of `utils.GetOrganizationById(id)` (`GET /organization/{id}`).
pub fn get_organization_by_id(id: &str) -> Result<IdName, String> {
    get_organization_by_id_with(&get_client()?, id)
}

/// [`get_organization_by_id`] with an existing client.
pub fn get_organization_by_id_with(client: &Client, id: &str) -> Result<IdName, String> {
    get_by_id(client, "organization", id).map(|v| id_name(&v))
}

/// Port of `utils.GetProjectById(id)` (`GET /project/{id}`).
pub fn get_project_by_id(id: &str) -> Result<IdName, String> {
    get_project_by_id_with(&get_client()?, id)
}

/// [`get_project_by_id`] with an existing client.
pub fn get_project_by_id_with(client: &Client, id: &str) -> Result<IdName, String> {
    get_by_id(client, "project", id).map(|v| id_name(&v))
}

/// Port of `utils.GetEnvironmentById(id)` (`GET /environment/{id}`).
pub fn get_environment_by_id(id: &str) -> Result<EnvironmentRef, String> {
    get_environment_by_id_with(&get_client()?, id)
}

/// [`get_environment_by_id`] with an existing client.
pub fn get_environment_by_id_with(client: &Client, id: &str) -> Result<EnvironmentRef, String> {
    get_by_id(client, "environment", id).map(|v| EnvironmentRef {
        id: str_field(&v, "id").to_string(),
        cluster_id: str_field(&v, "cluster_id").to_string(),
        name: str_field(&v, "name").to_string(),
    })
}

/// `utils.EnvironmentService`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EnvironmentService {
    pub id: String,
    pub service_type: ServiceType,
}

/// Port of `utils.GetEnvironmentServicesById(id)` (`GET /environment/{id}/statuses`): applications,
/// containers, jobs, databases, helms then terraforms, in that order.
pub fn get_environment_services_by_id(id: &str) -> Result<Vec<EnvironmentService>, String> {
    get_environment_services_by_id_with(&get_client()?, id)
}

/// [`get_environment_services_by_id`] with an existing client.
pub fn get_environment_services_by_id_with(
    client: &Client,
    id: &str,
) -> Result<Vec<EnvironmentService>, String> {
    let statuses: EnvironmentStatuses = client
        .get(&format!("/environment/{}/statuses", path_escape(id)))
        .map_err(|e| match e.status {
            // no space before the id in Go
            Some(code) if code >= 400 => format!(
                "Received {} response while getting environment services{id}",
                qovery_api::status_line(code)
            ),
            _ => e.to_string(),
        })?;
    let mut services = Vec::new();
    for (list, t) in [
        (&statuses.applications, ServiceType::Application),
        (&statuses.containers, ServiceType::Container),
        (&statuses.jobs, ServiceType::Job),
        (&statuses.databases, ServiceType::Database),
        (&statuses.helms, ServiceType::Helm),
        (&statuses.terraforms, ServiceType::Terraform),
    ] {
        services.extend(list.iter().map(|s| EnvironmentService {
            id: s.id.clone(),
            service_type: t,
        }));
    }
    Ok(services)
}

/// Port of `utils.GetApplicationById(id)` (`GET /application/{id}`).
pub fn get_application_by_id(id: &str) -> Result<IdName, String> {
    get_application_by_id_with(&get_client()?, id)
}

/// [`get_application_by_id`] with an existing client.
pub fn get_application_by_id_with(client: &Client, id: &str) -> Result<IdName, String> {
    get_by_id(client, "application", id).map(|v| id_name(&v))
}

/// Port of `utils.GetContainerById(id)` (`GET /container/{id}`).
pub fn get_container_by_id(id: &str) -> Result<IdName, String> {
    get_container_by_id_with(&get_client()?, id)
}

/// [`get_container_by_id`] with an existing client.
pub fn get_container_by_id_with(client: &Client, id: &str) -> Result<IdName, String> {
    get_by_id(client, "container", id).map(|v| id_name(&v))
}

/// Port of `utils.GetDatabaseById(id)` (`GET /database/{id}`): a `database` service.
pub fn get_database_by_id(id: &str) -> Result<Service, String> {
    get_database_by_id_with(&get_client()?, id)
}

/// [`get_database_by_id`] with an existing client.
pub fn get_database_by_id_with(client: &Client, id: &str) -> Result<Service, String> {
    get_by_id(client, "database", id).map(|v| service(&v, ServiceType::Database))
}

/// Port of `utils.GetHelmById(id)` (`GET /helm/{id}`): a `helm` service.
pub fn get_helm_by_id(id: &str) -> Result<Service, String> {
    get_helm_by_id_with(&get_client()?, id)
}

/// [`get_helm_by_id`] with an existing client.
pub fn get_helm_by_id_with(client: &Client, id: &str) -> Result<Service, String> {
    get_by_id(client, "helm", id).map(|v| service(&v, ServiceType::Helm))
}

/// Port of `utils.GetTerraformById(id)` (`GET /terraform/{id}`): a `terraform` service with its `blueprint_id`.
pub fn get_terraform_by_id(id: &str) -> Result<Service, String> {
    get_terraform_by_id_with(&get_client()?, id)
}

/// [`get_terraform_by_id`] with an existing client.
pub fn get_terraform_by_id_with(client: &Client, id: &str) -> Result<Service, String> {
    get_by_id(client, "terraform", id).map(|v| Service {
        blueprint_id: str_field(&v, "blueprint_id").to_string(),
        ..service(&v, ServiceType::Terraform)
    })
}

/// Port of `utils.GetJobById(id)` (`GET /job/{id}`); `invalid job response` when the job is neither a
/// `CRON` nor a `LIFECYCLE` job.
pub fn get_job_by_id(id: &str) -> Result<IdName, String> {
    get_job_by_id_with(&get_client()?, id)
}

/// [`get_job_by_id`] with an existing client.
pub fn get_job_by_id_with(client: &Client, id: &str) -> Result<IdName, String> {
    let v = get_by_id(client, "job", id)?;
    match str_field(&v, "job_type") {
        "CRON" | "LIFECYCLE" | "CronJobResponse" | "LifecycleJobResponse" => Ok(id_name(&v)),
        _ => Err("invalid job response".to_string()),
    }
}

/// Port of `utils.GetServiceNameByIdAndType(client, serviceId, serviceType)`: the name of an
/// `APPLICATION` / `DATABASE` / `CONTAINER` / `JOB` / `HELM`, `""` on API error, `Unknown` for other types.
pub fn get_service_name_by_id_and_type(
    client: &Client,
    service_id: &str,
    service_type: &str,
) -> String {
    let kind = match service_type {
        "APPLICATION" => "application",
        "DATABASE" => "database",
        "CONTAINER" => "container",
        "JOB" => "job",
        "HELM" => "helm",
        _ => return "Unknown".to_string(),
    };
    let Ok(v) = client.get::<Value>(&format!("/{kind}/{}", path_escape(service_id))) else {
        return String::new();
    };
    if kind == "job"
        && !matches!(
            str_field(&v, "job_type"),
            "CRON" | "LIFECYCLE" | "CronJobResponse" | "LifecycleJobResponse"
        )
    {
        // GetJobName of a job that is neither a cron nor a lifecycle job
        return String::new();
    }
    str_field(&v, "name").to_string()
}

/// Port of `utils.GetDeploymentStageId(client, serviceId)` (`GET /service/{id}/deploymentStage`):
/// prints `Error: <err>` and exits 1 on error.
pub fn get_deployment_stage_id(client: &Client, service_id: &str) -> String {
    match client.get::<Value>(&format!(
        "/service/{}/deploymentStage",
        path_escape(service_id)
    )) {
        Ok(v) => str_field(&v, "id").to_string(),
        Err(e) => qovery_core::exit_with_error(e),
    }
}
