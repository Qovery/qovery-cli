//! Deploy / delete / reboot / cancel helpers of `utils/qovery.go`, with the request models serialized like
//! the Go client (`DeployAllRequest`, `TerraformDeployRequest`, `RebootServicesRequest`).
//!
//! Like the Go client, the responses of `DeployAllServices`, `CancelEnvironmentDeployment` and `DeployTerraform`
//! are decoded (an invalid body is an error); `StopSelectedServices` / `DeleteSelectedServices` have no response
//! model and `RebootServices` ignores its decoding error, so only their HTTP status is checked.

use crate::commit::{
    LATEST_COMMIT_KEYWORD, is_latest_commit_keyword, resolve_latest_application_commit,
    resolve_latest_helm_chart_commit, resolve_latest_job_commit, resolve_latest_terraform_commit,
};
use crate::jobs::{
    get_git_source, get_helm_repository, get_job_docker, get_job_id, get_job_image, get_job_name,
};
use crate::util::{decode_go_response, path_escape, str_field};
use crate::watch::{is_terminal_state, state, watch_environment_with_options};
use qovery_api::models::{
    Application, Container, Database, EnvironmentStatus, EnvironmentStatuses, Helm, Job, Status,
    Terraform,
};
use qovery_api::{Client, Method};
use qovery_core::context::ServiceType;
use serde::Serialize;
use serde_json::Value;

/// `qovery.DeployAllRequest` (`POST /environment/{id}/service/deploy`); `None` lists are omitted.
#[derive(Debug, Clone, Default, Serialize)]
pub struct DeployAllRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub applications: Option<Vec<DeployAllRequestApplication>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub databases: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub containers: Option<Vec<DeployAllRequestContainer>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub jobs: Option<Vec<DeployAllRequestJob>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub helms: Option<Vec<DeployAllRequestHelm>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub terraforms: Option<Vec<TerraformDeployRequest>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub agentic_workflows: Option<Vec<String>>,
}

/// `qovery.DeployAllRequestApplicationsInner`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct DeployAllRequestApplication {
    pub application_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_commit_id: Option<String>,
}

/// `qovery.DeployAllRequestContainersInner`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct DeployAllRequestContainer {
    pub id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_tag: Option<String>,
}

/// `qovery.DeployAllRequestJobsInner`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct DeployAllRequestJob {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_tag: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_commit_id: Option<String>,
}

/// `qovery.DeployAllRequestHelmsInner`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct DeployAllRequestHelm {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub chart_version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_commit_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub values_override_git_commit_id: Option<String>,
}

/// `qovery.TerraformDeployRequest` (`id` and `action` are nullable: `Some(None)` sends `null`).
#[derive(Debug, Clone, Default, Serialize)]
pub struct TerraformDeployRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<Option<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_commit_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub action: Option<Option<String>>,
}

/// `qovery.RebootServicesRequest` (`POST /environment/{id}/service/restart-service`).
#[derive(Debug, Clone, Default, Serialize)]
pub struct RebootServicesRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub application_ids: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub database_ids: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub container_ids: Option<Vec<String>>,
}

/// `qovery.EnvironmentServiceIdsAllRequest` (`POST /environment/{id}/service/{stop,delete}`); empty lists are
/// omitted (`omitempty`).
#[derive(Debug, Clone, Default, Serialize)]
pub struct ServiceIdsAllRequest {
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub application_ids: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub container_ids: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub database_ids: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub job_ids: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub helm_ids: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub terraform_ids: Vec<String>,
}

/// `EnvironmentActionsAPI.StopSelectedServices` / `DeleteSelectedServices`: `POST /environment/{id}/service/<action>`
/// (`action` is `stop` or `delete`); any 2xx response is a success.
pub fn selected_services_action(
    client: &Client,
    env_id: &str,
    action: &str,
    request: &ServiceIdsAllRequest,
) -> Result<(), String> {
    post(
        client,
        &format!("/environment/{}/service/{action}", path_escape(env_id)),
        Some(&to_value(request)),
    )
}

fn to_value<T: Serialize>(v: &T) -> Value {
    serde_json::to_value(v).unwrap_or(Value::Null)
}

fn post(client: &Client, path: &str, body: Option<&Value>) -> Result<(), String> {
    client
        .execute(Method::POST, path, &[], body)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// `post` whose 2xx response body is decoded into `T` like the Go `Execute()` (the value is not used).
fn post_decoded<T: serde::de::DeserializeOwned>(
    client: &Client,
    path: &str,
    body: Option<&Value>,
) -> Result<(), String> {
    let res = client
        .execute(Method::POST, path, &[], body)
        .map_err(|e| e.to_string())?;
    decode_go_response::<T>(&res).map(|_| ())
}

/// [`post_decoded`] with the error decorated like Go `withAPIErrorDetail(err)`: the generated client's
/// errors carry the response body (status >= 300 and 2xx decoding errors alike).
fn post_decoded_with_detail<T: serde::de::DeserializeOwned>(
    client: &Client,
    path: &str,
    body: Option<&Value>,
) -> Result<(), String> {
    let res = client
        .execute(Method::POST, path, &[], body)
        .map_err(|e| qovery_api::with_api_error_detail(&e.message, &e.body))?;
    decode_go_response::<T>(&res)
        .map(|_| ())
        .map_err(|e| qovery_api::with_api_error_detail(&e, &res.text()))
}

/// Port of `deployAllServices(client, envId, req)`: `POST /environment/{id}/service/deploy`.
pub fn deploy_all_services(
    client: &Client,
    env_id: &str,
    req: &DeployAllRequest,
) -> Result<(), String> {
    post_decoded::<EnvironmentStatus>(
        client,
        &format!("/environment/{}/service/deploy", path_escape(env_id)),
        Some(&to_value(req)),
    )
}

/// Port of `utils.DeployApplications(client, envId, applicationList, commitId)`: deploys with `commitId`
/// (`latest` resolves the newest commit of each application), or the deployed commit when empty.
pub fn deploy_applications(
    client: &Client,
    env_id: &str,
    application_list: &[Application],
    commit_id: &str,
) -> Result<(), String> {
    if application_list.is_empty() {
        return Ok(());
    }
    let mut applications = Vec::new();
    for application in application_list {
        // if commitId is not set, use the deployed commit id
        let mut application_commit_id = application
            .git_repository
            .as_ref()
            .and_then(|g| g.get("deployed_commit_id"))
            .and_then(|c| c.as_str())
            .map(|s| s.to_string());
        if is_latest_commit_keyword(commit_id) {
            application_commit_id = Some(resolve_latest_application_commit(
                client,
                &application.id,
                &application.name,
            )?);
        } else if !commit_id.is_empty() {
            application_commit_id = Some(commit_id.to_string());
        }
        applications.push(DeployAllRequestApplication {
            application_id: application.id.clone(),
            git_commit_id: application_commit_id,
        });
    }
    deploy_all_services(
        client,
        env_id,
        &DeployAllRequest {
            applications: Some(applications),
            ..Default::default()
        },
    )
}

/// Port of `utils.DeployContainers(client, envId, containerList, tag)`: image containers are deployed with
/// `tag`, or their current tag when empty; a Git container is sent without image tag (it deploys its current
/// commit) and rejects a `tag`. The error carries the API's message (`withAPIErrorDetail`).
pub fn deploy_containers(
    client: &Client,
    env_id: &str,
    container_list: &[Container],
    tag: &str,
) -> Result<(), String> {
    if container_list.is_empty() {
        return Ok(());
    }
    let mut containers = Vec::new();
    for c in container_list {
        check_container_image_tag(c, tag)?;
        let image_tag = get_container_git_repository(c).is_none().then(|| {
            if tag.is_empty() {
                c.tag.clone()
            } else {
                tag.to_string()
            }
        });
        containers.push(DeployAllRequestContainer {
            id: c.id.clone(),
            image_tag,
        });
    }
    post_decoded_with_detail::<EnvironmentStatus>(
        client,
        &format!("/environment/{}/service/deploy", path_escape(env_id)),
        Some(&to_value(&DeployAllRequest {
            containers: Some(containers),
            ..Default::default()
        })),
    )
}

/// Port of `utils.DeployContainersAtCommit(client, containerList, commitId)`: `POST /container/{id}/deploy`
/// with `git_commit_id` for each Git container, after checking all of them (a list with an image container
/// deploys nothing).
pub fn deploy_containers_at_commit(
    client: &Client,
    container_list: &[Container],
    commit_id: &str,
) -> Result<(), String> {
    for c in container_list {
        check_container_commit_id(c)?;
    }
    for c in container_list {
        post_decoded_with_detail::<Status>(
            client,
            &format!("/container/{}/deploy", path_escape(&c.id)),
            Some(&serde_json::json!({ "git_commit_id": commit_id })),
        )
        .map_err(|e| {
            format!(
                "cannot deploy container {} at commit {commit_id}: {e}",
                c.name
            )
        })?;
    }
    Ok(())
}

/// Port of `utils.CheckContainerImageTag(container, tag)`: a Git container takes no image tag, Qovery builds
/// its image from a commit.
pub fn check_container_image_tag(container: &Container, tag: &str) -> Result<(), String> {
    let Some(repository) = get_container_git_repository(container) else {
        return Ok(());
    };
    if tag.is_empty() {
        return Ok(());
    }
    Err(format!(
        "container {0} is a Git container built from {1}: it cannot be deployed with an image tag. Deploy a commit with: qovery container deploy -n {0} --commit-id <commit sha>",
        container.name,
        git_repository_label(repository)
    ))
}

/// Port of `utils.CheckContainerCommitId(container)`: an image container has no Git repository to build.
pub fn check_container_commit_id(container: &Container) -> Result<(), String> {
    if get_container_git_repository(container).is_some() {
        return Ok(());
    }
    Err(format!(
        "container {} runs the image {} from registry {}: --commit-id only applies to Git containers. Deploy another image with --tag",
        container.name,
        container_image_label(container),
        container_registry_name(container)
    ))
}

/// Port of `utils.GetContainerGitRepository(container)`: the `git_repository` of the container's `source`,
/// `None` for an image container, including when an older API sends no source. (The Go client's strict
/// `oneOf` decoding of `source` is not reproduced.)
pub fn get_container_git_repository(container: &Container) -> Option<&Value> {
    container
        .extra
        .get("source")?
        .get("git_repository")
        .filter(|r| r.is_object())
}

/// `container.Registry.Name`.
pub fn container_registry_name(container: &Container) -> &str {
    container
        .extra
        .get("registry")
        .map(|r| str_field(r, "name"))
        .unwrap_or_default()
}

/// Port of `utils.ContainerSourceLabel(container)`: `Git <url> (branch <branch>)` or `Registry <name>`.
pub fn container_source_label(container: &Container) -> String {
    match get_container_git_repository(container) {
        Some(repository) => format!("Git {}", git_repository_label(repository)),
        None => format!("Registry {}", container_registry_name(container)),
    }
}

/// Port of `utils.ContainerImageLabel(container)`: the image the container runs (on a Git container, the last
/// image Qovery built).
pub fn container_image_label(container: &Container) -> String {
    format!("{}:{}", container.image_name, container.tag)
}

/// `gitRepositoryLabel(repository)`.
fn git_repository_label(repository: &Value) -> String {
    let url = str_field(repository, "url");
    match str_field(repository, "branch") {
        "" => url.to_string(),
        branch => format!("{url} (branch {branch})"),
    }
}

/// Port of `utils.DeployJobs(client, envId, jobList, commitId, tag)`: docker jobs get `commitId` (`latest`
/// resolved, deployed commit when empty), image jobs get `tag` (current tag when empty).
pub fn deploy_jobs(
    client: &Client,
    env_id: &str,
    job_list: &[Job],
    commit_id: &str,
    tag: &str,
) -> Result<(), String> {
    if job_list.is_empty() {
        return Ok(());
    }
    let mut jobs = Vec::new();
    for job in job_list {
        let mut m_commit_id = None;
        let mut m_tag = None;
        if let Some(docker) = get_job_docker(job) {
            m_commit_id = docker
                .get("git_repository")
                .and_then(|g| g.get("deployed_commit_id"))
                .and_then(|c| c.as_str())
                .map(|s| s.to_string());
            if is_latest_commit_keyword(commit_id) {
                m_commit_id = Some(resolve_latest_job_commit(
                    client,
                    &get_job_id(job),
                    &get_job_name(job),
                )?);
            } else if !commit_id.is_empty() {
                m_commit_id = Some(commit_id.to_string());
            }
        } else {
            m_tag = Some(
                get_job_image(job)
                    .map(|i| str_field(i, "tag").to_string())
                    .unwrap_or_default(),
            );
            if !tag.is_empty() {
                m_tag = Some(tag.to_string());
            }
        }
        jobs.push(DeployAllRequestJob {
            id: Some(get_job_id(job)),
            image_tag: m_tag,
            git_commit_id: m_commit_id,
        });
    }
    deploy_all_services(
        client,
        env_id,
        &DeployAllRequest {
            jobs: Some(jobs),
            ..Default::default()
        },
    )
}

/// Port of `utils.DeployDatabases(client, envId, databaseList)`.
pub fn deploy_databases(
    client: &Client,
    env_id: &str,
    database_list: &[Database],
) -> Result<(), String> {
    if database_list.is_empty() {
        return Ok(());
    }
    let databases = database_list.iter().map(|d| d.id.clone()).collect();
    deploy_all_services(
        client,
        env_id,
        &DeployAllRequest {
            databases: Some(databases),
            ..Default::default()
        },
    )
}

/// Port of `utils.DeployHelms(client, envId, helmList, chartVersion, chartGitCommitId, valuesOverrideCommitId)`.
/// `latest` is resolved for `chart_git_commit_id` (git helms) and rejected for `values_override_commit_id`.
pub fn deploy_helms(
    client: &Client,
    env_id: &str,
    helm_list: &[Helm],
    chart_version: &str,
    chart_git_commit_id: &str,
    values_override_commit_id: &str,
) -> Result<(), String> {
    if helm_list.is_empty() {
        return Ok(());
    }
    if is_latest_commit_keyword(values_override_commit_id) {
        return Err(format!(
            "`{LATEST_COMMIT_KEYWORD}` is not supported for --values_override_git_commit_id: the API exposes no commit list for the values-override repository. Pass an explicit commit id, or omit the flag to keep the deployed one"
        ));
    }
    let mut helms = Vec::new();
    for helm in helm_list {
        let git_source = get_git_source(helm);
        let helm_repository_source = get_helm_repository(helm);
        if git_source.is_some() && helm_repository_source.is_some() {
            return Err("invalid helm".to_string());
        }
        let mut m_commit_id = None;
        let mut m_chart_version = None;
        if git_source.is_some() {
            if is_latest_commit_keyword(chart_git_commit_id) {
                m_commit_id = Some(resolve_latest_helm_chart_commit(
                    client, &helm.id, &helm.name,
                )?);
            } else if !chart_git_commit_id.is_empty() {
                m_commit_id = Some(chart_git_commit_id.to_string());
            }
        }
        if helm_repository_source.is_some() && !chart_version.is_empty() {
            m_chart_version = Some(chart_version.to_string());
        }
        let m_values_override_commit_id =
            (!values_override_commit_id.is_empty()).then(|| values_override_commit_id.to_string());
        helms.push(DeployAllRequestHelm {
            id: Some(helm.id.clone()),
            chart_version: m_chart_version,
            git_commit_id: m_commit_id,
            values_override_git_commit_id: m_values_override_commit_id,
        });
    }
    deploy_all_services(
        client,
        env_id,
        &DeployAllRequest {
            helms: Some(helms),
            ..Default::default()
        },
    )
}

/// Port of `terraformGitCommitId(client, terraform, commitId)`: the commit id to deploy (`latest`
/// resolved), `None` when none was requested.
pub fn terraform_git_commit_id(
    client: &Client,
    terraform: &Terraform,
    commit_id: &str,
) -> Result<Option<String>, String> {
    if is_latest_commit_keyword(commit_id) {
        return resolve_latest_terraform_commit(client, &terraform.id, &terraform.name).map(Some);
    }
    if !commit_id.is_empty() {
        return Ok(Some(commit_id.to_string()));
    }
    Ok(None)
}

/// Port of `utils.DeployTerraforms(client, envId, terraformList, commitId, action)`.
///
/// With an `action` (PLAN, FORCE_UNLOCK, MIGRATE_STATE) every commit id is resolved first, then each
/// terraform is deployed with `POST /terraform/{id}/deploy` `{"action", "git_commit_id"}`; without action
/// (PLAN_AND_APPLY) they are deployed together with `deployAllServices` (`{"terraforms": [{"id", ...}]}`).
pub fn deploy_terraforms(
    client: &Client,
    env_id: &str,
    terraform_list: &[Terraform],
    commit_id: &str,
    action: Option<&str>,
) -> Result<(), String> {
    if terraform_list.is_empty() {
        return Ok(());
    }
    if let Some(action) = action {
        // resolve every commit id first: a failure on a later service must leave no earlier service triggered
        let mut requests = Vec::with_capacity(terraform_list.len());
        for terraform in terraform_list {
            requests.push(TerraformDeployRequest {
                id: None,
                git_commit_id: terraform_git_commit_id(client, terraform, commit_id)?,
                action: Some(Some(action.to_string())),
            });
        }
        for (terraform, req) in terraform_list.iter().zip(&requests) {
            post_decoded::<Status>(
                client,
                &format!("/terraform/{}/deploy", path_escape(&terraform.id)),
                Some(&to_value(req)),
            )?;
        }
        return Ok(());
    }

    let mut terraforms = Vec::new();
    for terraform in terraform_list {
        terraforms.push(TerraformDeployRequest {
            id: Some(Some(terraform.id.clone())),
            git_commit_id: terraform_git_commit_id(client, terraform, commit_id)?,
            action: None,
        });
    }
    deploy_all_services(
        client,
        env_id,
        &DeployAllRequest {
            terraforms: Some(terraforms),
            ..Default::default()
        },
    )
}

/// Port of `utils.DeleteTerraforms(client, envId, terraformList, skipDestroy, resourcesOnly)`:
/// `DELETE /terraform/{id}` with `resources_only=true` and/or `force_terraform_action=SKIP_DESTROY`.
pub fn delete_terraforms(
    client: &Client,
    _env_id: &str,
    terraform_list: &[Terraform],
    skip_destroy: bool,
    resources_only: bool,
) -> Result<(), String> {
    for terraform in terraform_list {
        // url.Values.Encode() sorts the keys
        let mut query: Vec<(&str, String)> = Vec::new();
        if skip_destroy {
            query.push(("force_terraform_action", "SKIP_DESTROY".to_string()));
        }
        if resources_only {
            query.push(("resources_only", "true".to_string()));
        }
        client
            .delete_with_query(
                &format!("/terraform/{}", path_escape(&terraform.id)),
                &query,
            )
            .map_err(|e| e.to_string())?;
    }
    Ok(())
}

/// Port of `utils.RebootServices(client, envId, request)`: `POST /environment/{id}/service/restart-service`;
/// any 2xx response is a success whatever its body.
pub fn reboot_services(
    client: &Client,
    env_id: &str,
    request: &RebootServicesRequest,
) -> Result<(), String> {
    post(
        client,
        &format!(
            "/environment/{}/service/restart-service",
            path_escape(env_id)
        ),
        Some(&to_value(request)),
    )
}

/// Port of `utils.CancelEnvironmentDeployment(client, envId, watchFlag)`: `POST /environment/{id}/cancelDeployment`
/// (no body), then `WatchEnvironmentWithOptions(envId, CANCELED, client, true)` when `watch_flag`.
pub fn cancel_environment_deployment(
    client: &Client,
    env_id: &str,
    watch_flag: bool,
) -> Result<(), String> {
    post_decoded::<EnvironmentStatus>(
        client,
        &format!("/environment/{}/cancelDeployment", path_escape(env_id)),
        None,
    )?;
    if watch_flag {
        watch_environment_with_options(env_id, state::CANCELED, client, true);
    }
    Ok(())
}

/// Port of `utils.CancelServiceDeployment(client, envId, serviceId, serviceType, watchFlag)`.
///
/// Returns `there is no deployment in progress. Nothing to cancel` when the environment is in a terminal
/// state, `""` once the environment deployment is canceled because the service is not in a terminal state;
/// otherwise prints `Info: waiting for previous deployment to be completed...`, sleeps 5 seconds and retries.
pub fn cancel_service_deployment(
    client: &Client,
    env_id: &str,
    service_id: &str,
    service_type: ServiceType,
    watch_flag: bool,
) -> Result<String, String> {
    loop {
        let statuses: EnvironmentStatuses = client
            .get(&format!("/environment/{}/statuses", path_escape(env_id)))
            .map_err(|e| e.to_string())?;

        if is_terminal_state(&statuses.environment.state) {
            // if the environment is in a terminal state, there is nothing to cancel
            return Ok("there is no deployment in progress. Nothing to cancel".to_string());
        }

        // cancel deployment if the targeted service is a non-terminal state
        let list = match service_type {
            ServiceType::Application => Some(&statuses.applications),
            ServiceType::Database => Some(&statuses.databases),
            ServiceType::Container => Some(&statuses.containers),
            ServiceType::Job => Some(&statuses.jobs),
            ServiceType::Helm => Some(&statuses.helms),
            ServiceType::Terraform => None,
        };
        if let Some(list) = list
            && list
                .iter()
                .any(|s| s.id == service_id && !is_terminal_state(&s.state))
        {
            cancel_environment_deployment(client, env_id, watch_flag)?;
            return Ok(String::new());
        }

        qovery_log::println_info("waiting for previous deployment to be completed...");
        // sleep here to avoid too many requests
        std::thread::sleep(std::time::Duration::from_secs(5));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn requests_serialize_like_go() {
        let req = DeployAllRequest {
            applications: Some(vec![
                DeployAllRequestApplication {
                    application_id: "a".into(),
                    git_commit_id: None,
                },
                DeployAllRequestApplication {
                    application_id: "b".into(),
                    git_commit_id: Some("c".into()),
                },
            ]),
            ..Default::default()
        };
        assert_eq!(
            to_value(&req),
            json!({"applications": [{"application_id": "a"}, {"application_id": "b", "git_commit_id": "c"}]})
        );

        let tf = TerraformDeployRequest {
            id: Some(Some("t".into())),
            git_commit_id: None,
            action: None,
        };
        assert_eq!(to_value(&tf), json!({"id": "t"}));
        let tf = TerraformDeployRequest {
            id: None,
            git_commit_id: Some("c".into()),
            action: Some(Some("PLAN".into())),
        };
        assert_eq!(
            to_value(&tf),
            json!({"action": "PLAN", "git_commit_id": "c"})
        );

        let reboot = RebootServicesRequest {
            application_ids: Some(vec![]),
            ..Default::default()
        };
        assert_eq!(to_value(&reboot), json!({"application_ids": []}));

        let job = DeployAllRequestJob {
            id: Some("j".into()),
            image_tag: Some("1".into()),
            git_commit_id: None,
        };
        assert_eq!(to_value(&job), json!({"id": "j", "image_tag": "1"}));
    }
}
