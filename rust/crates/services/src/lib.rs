//! Shared service helpers of the Qovery CLI: port of the parts of the Go `utils` package (and of
//! `cmd/watch.go`) that are not in `qovery-core`: lookups by id, deploy / cancel / reboot / delete helpers,
//! environment and services watches, `latest` commit resolution, autoscaling conversion, file helpers...
//!
//! Every helper prints and calls the API like its Go counterpart (same messages, colors, methods, paths,
//! query parameters and JSON bodies; unset optional fields are omitted like the Go client does).
//! Functions taking a client take `&qovery_api::Client` (Go `*qovery.APIClient`) and keep the Go argument
//! order; Go errors are `Result<_, String>`; Go `qovery.StateEnum` values are `&str` (see [`watch::state`]).
//! Everything is re-exported at the crate root.
//!
//! | Go | Rust |
//! |---|---|
//! | `utils.GetOrganizationById(id)` | [`get_organization_by_id`] / [`get_organization_by_id_with`] -> `IdName` |
//! | `utils.GetProjectById(id)` | [`get_project_by_id`] / [`get_project_by_id_with`] -> `IdName` |
//! | `utils.GetEnvironmentById(id)` | [`get_environment_by_id`] / [`get_environment_by_id_with`] -> `EnvironmentRef` |
//! | `utils.GetEnvironmentServicesById(id)` | [`get_environment_services_by_id`] / `_with` -> `Vec<EnvironmentService>` |
//! | `utils.GetApplicationById(id)` | [`get_application_by_id`] / `_with` -> `IdName` |
//! | `utils.GetContainerById(id)` | [`get_container_by_id`] / `_with` -> `IdName` |
//! | `utils.GetDatabaseById(id)` | [`get_database_by_id`] / `_with` -> `Service` |
//! | `utils.GetHelmById(id)` | [`get_helm_by_id`] / `_with` -> `Service` |
//! | `utils.GetTerraformById(id)` | [`get_terraform_by_id`] / `_with` -> `Service` (with `blueprint_id`) |
//! | `utils.GetJobById(id)` | [`get_job_by_id`] / `_with` -> `IdName` |
//! | `utils.AddEnvironmentVariable` / `DeleteEnvironmentVariable` | [`add_environment_variable`] / [`delete_environment_variable`] (+ `_with`) |
//! | `utils.AddSecret` / `DeleteSecret` | [`add_secret`] / [`delete_secret`] (+ `_with`) |
//! | `utils.AddContainerEnvironmentVariable` / `DeleteContainerEnvironmentVariable` | [`add_container_environment_variable`] / [`delete_container_environment_variable`] (+ `_with`) |
//! | `utils.AddContainerSecret` / `DeleteContainerSecret` | [`add_container_secret`] / [`delete_container_secret`] (+ `_with`) |
//! | `utils.SelectRole`, `utils.SelectTokenInformation`, `utils.Role`, `utils.TokenInformation` | [`select_role`], [`select_token_information`], [`Role`], [`TokenInformation`] |
//! | `utils.WatchEnvironment` / `WatchEnvironmentWithOptions` | [`watch_environment`] / [`watch_environment_with_options`] |
//! | `isFinalState` / `isErrorState` / `countStatus` | [`is_final_state`] / [`is_error_state`] / [`count_status`] |
//! | `utils.IsTerminalState` / `IsTerminalClusterState` | [`is_terminal_state`] / [`is_terminal_cluster_state`] |
//! | `utils.Status` (`Continue`/`Stop`/`Err`) | [`WatchStatus`] |
//! | `utils.ServicesWatch`, `NewServicesWatch`, `(*ServicesWatch).Wait` | [`ServicesWatch`], [`new_services_watch`], [`ServicesWatch::wait`] |
//! | `snapshotExecutions`, `watchServices`, `watchTimeout`, `servicesTracker`, `allStatuses`, `executionIds` | [`snapshot_executions`], [`watch_services`], [`watch_timeout`], [`ServicesTracker`], [`EnvironmentStatusesWithStages::all_statuses`], [`EnvironmentStatusesWithStages::execution_ids`] |
//! | `qovery.EnvironmentStatusesWithStages` | [`EnvironmentStatusesWithStages`], [`fetch_statuses_with_stages`] |
//! | `cmd.applicationIds` / `containerIds` / `databaseIds` / `jobIds` / `helmIds` / `terraformIds` | [`application_ids`] / [`container_ids`] / [`database_ids`] / [`job_ids`] / [`helm_ids`] / [`terraform_ids`] |
//! | `utils.GetServiceNameByIdAndType` | [`get_service_name_by_id_and_type`] |
//! | `utils.GetDeploymentStageId` | [`get_deployment_stage_id`] |
//! | `utils.DeployApplications` / `DeployContainers` / `DeployJobs` / `DeployDatabases` / `DeployHelms` / `DeployTerraforms` | [`deploy_applications`] / [`deploy_containers`] / [`deploy_jobs`] / [`deploy_databases`] / [`deploy_helms`] / [`deploy_terraforms`] |
//! | `deployAllServices`, `qovery.DeployAllRequest*`, `qovery.TerraformDeployRequest` | [`deploy_all_services`], [`DeployAllRequest`] & co, [`TerraformDeployRequest`] |
//! | `terraformGitCommitId` | [`terraform_git_commit_id`] |
//! | `utils.DeleteTerraforms` | [`delete_terraforms`] |
//! | `utils.RebootServices`, `qovery.RebootServicesRequest` | [`reboot_services`], [`RebootServicesRequest`] |
//! | `utils.CancelEnvironmentDeployment` / `CancelServiceDeployment` | [`cancel_environment_deployment`] / [`cancel_service_deployment`] |
//! | `utils.GetJobDocker` / `GetJobImage` / `GetJobId` / `GetJobName` | [`get_job_docker`] / [`get_job_image`] / [`get_job_id`] / [`get_job_name`] |
//! | `utils.GetGitSource` / `GetHelmRepository` | [`get_git_source`] / [`get_helm_repository`] |
//! | `utils.ToJobRequest`, `qovery.JobRequest` & nested request models | [`to_job_request`], [`JobRequest`], [`JobRequestSource`], ... |
//! | `utils.GetDuration` | [`get_duration`] / [`get_duration_rfc3339`] |
//! | `utils.LatestCommitKeyword`, `IsLatestCommitKeyword`, `ResolveLatest{Application,Job,HelmChart,Terraform}Commit` | [`LATEST_COMMIT_KEYWORD`], [`is_latest_commit_keyword`], [`resolve_latest_application_commit`], ... |
//! | `newestCommit`, `resolveLatestCommit`, `firstLine`, `qovery.Commit` | [`newest_commit`], [`resolve_latest_commit`], [`first_line`], [`Commit`] |
//! | `utils.ConvertAutoscalingResponseToRequest` | [`convert_autoscaling_response_to_request`] |
//! | `utils.WriteInFile` / `GetFullPath` / `DeleteFile` / `DeleteFolder` | [`write_in_file`] / [`get_full_path`] / [`delete_file`] / [`delete_folder`] |
//! | `utils.ToIso8601` | [`to_iso8601`] (RFC 3339 string) / [`to_iso8601_time`] / [`to_iso8601_value`] (JSON field) |
//! | `utils.RandStringBytes` / `IsEmptyOrBlank` / `Map` | [`rand_string_bytes`] / [`is_empty_or_blank`] / [`map`] |
//! | `utils.Int32(v)` / `utils.Bool(v)` | `Some(v)` (no helper needed) |
//! | `url.PathEscape` (path parameters of the Go client) | [`path_escape`] |

pub mod autoscaling;
pub mod commit;
pub mod deploy;
pub mod env_vars;
pub mod files;
pub mod getters;
pub mod jobs;
pub mod token;
pub mod util;
pub mod watch;

pub use autoscaling::*;
pub use commit::*;
pub use deploy::*;
pub use env_vars::*;
pub use files::*;
pub use getters::*;
pub use jobs::*;
pub use token::*;
pub use util::{
    get_duration, get_duration_rfc3339, go_duration_text, go_zero_time, is_empty_or_blank, map,
    parse_api_time, path_escape, rand_string_bytes, to_iso8601, to_iso8601_time, to_iso8601_value,
};
pub use watch::*;
