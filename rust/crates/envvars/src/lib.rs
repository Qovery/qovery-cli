//! Shared environment variable helpers of the Qovery CLI: port of `utils/env_var.go` and
//! `cmd/external_secret_helpers.go`, used by the `env` / `external-secret` sub-commands of `application`,
//! `container`, `cronjob`, `lifecycle`, `helm`, `terraform`, `environment`, `project` and `rde`.
//!
//! Everything is re-exported at the crate root. API functions take a `&qovery_api::Client` (Go
//! `*qovery.APIClient`) and return `Result<_, String>` where the error is Go's `err.Error()`; print it with
//! `qovery_log::println_error` / `qovery_core::exit_with_error` like the Go commands do.
//!
//! The package-level flag variables of `env_var.go` (`utils.ShowValues`, `utils.PrettyPrint`, `utils.SortKeys`,
//! `utils.IsSecret`, `utils.OverrideIfExists`, `utils.Key`, `utils.Value`, `utils.Alias`, `utils.*Scope`,
//! `utils.SecretManagerAccessName`, `utils.Reference`, `utils.MountPath`) are command flags: read them with
//! `ctx.bool("show-values")`, `ctx.string("key")`, ... in the command crates.
//!
//! | Go | Rust |
//! |---|---|
//! | `utils.Var` | [`Var`] |
//! | `qovery.VariableResponse`, `VariableAlias`, `VariableOverride` | [`VariableResponse`], [`VariableAlias`], [`VariableOverride`] |
//! | `qovery.APIVariableScopeEnum` (`qovery.APIVARIABLESCOPEENUM_*`) | [`VariableScope`] |
//! | `utils.EnvVarLines`, `utils.NewEnvVarLines()` | [`EnvVarLines`], [`EnvVarLines::new`] |
//! | `EnvVarLines.Add / Header / Lines` | [`EnvVarLines::add`], [`EnvVarLines::header`], [`EnvVarLines::lines`] |
//! | `utils.EnvVarLineOutput`, `EnvVarLineOutput.Data(showValues)` | [`EnvVarLineOutput`], [`EnvVarLineOutput::data`] |
//! | `utils.FromEnvironmentVariableToEnvVarLineOutput` | [`from_environment_variable_to_env_var_line_output`] |
//! | `utils.GetEnvVarJsonOutput` | [`get_env_var_json_output`] |
//! | `getInterpolatedValue`, `insertAtIndex`, `getValueOrDefault` | [`get_interpolated_value`], [`insert_at_index`], [`get_value_or_default`] |
//! | `utils.ToIso8601` (of an API timestamp), `t.Format(time.RFC822)` | [`to_iso8601`], [`format_rfc822`] |
//! | `utils.CreateServiceVariable` | [`create_service_variable`] |
//! | `utils.CreateServiceExternalSecret` | [`create_service_external_secret`] |
//! | `utils.UpdateServiceExternalSecret` | [`update_service_external_secret`] |
//! | `utils.UpdateEnvironmentExternalSecret` | [`update_environment_external_secret`] |
//! | `utils.CreateEnvironmentVariable` | [`create_environment_variable`] |
//! | `utils.CreateProjectVariable` | [`create_project_variable`] |
//! | `withAPIErrorDetail` | [`with_api_error_detail`] |
//! | `utils.UpdateServiceVariable` / `UpdateEnvironmentVariable` / `UpdateProjectVariable` | [`update_service_variable`], [`update_environment_variable`], [`update_project_variable`] |
//! | `utils.FindEnvironmentVariableByKey` | [`find_environment_variable_by_key`] |
//! | `utils.ListServiceVariables` / `ListEnvironmentVariables` / `ListProjectVariables` | [`list_service_variables`], [`list_environment_variables`], [`list_project_variables`] |
//! | `utils.ServiceTypeToScope` | [`service_type_to_scope`] |
//! | `getParentIdByScope` | [`get_parent_id_by_scope`] |
//! | `utils.DeleteServiceVariable` / `DeleteEnvironmentVar` / `DeleteProjectVar` | [`delete_service_variable`], [`delete_environment_var`], [`delete_project_var`] |
//! | `utils.CreateEnvironmentVariableAlias` | [`create_environment_variable_alias`] |
//! | `utils.CreateServiceAlias` / `CreateEnvironmentAlias` / `CreateProjectAlias` | [`create_service_alias`], [`create_environment_alias`], [`create_project_alias`] |
//! | `utils.CreateEnvironmentVariableOverride` | [`create_environment_variable_override`] |
//! | `utils.CreateServiceOverride` / `CreateEnvironmentOverride` | [`create_service_override`], [`create_environment_override`] |
//! | `getSecretManagerAccessIdByName` (cmd/external_secret_helpers.go) | [`get_secret_manager_access_id_by_name`] |
//! | Go `sort.Slice` (unstable pdqsort, same permutation) | [`go_sort_slice`] |
//!
//! Typical `env list` port:
//! ```ignore
//! let env_vars = check_error(qovery_envvars::list_service_variables(&client, &app.id, ServiceType::Application));
//! let mut lines = qovery_envvars::EnvVarLines::new();
//! let mut variables = Vec::new();
//! for v in &env_vars {
//!     let s = qovery_envvars::from_environment_variable_to_env_var_line_output(v);
//!     variables.push(s.clone());
//!     lines.add(s);
//! }
//! if ctx.bool("json") {
//!     qovery_log::println(qovery_envvars::get_env_var_json_output(&variables, ctx.bool("sort")));
//!     return;
//! }
//! qovery_log::print_table(
//!     &lines.header(ctx.bool("pretty-print")),
//!     &lines.lines(ctx.bool("show-values"), ctx.bool("pretty-print"), ctx.bool("sort")),
//! );
//! ```

mod api;
mod gosort;
mod json;
mod lines;
mod model;
mod secrets;

pub use api::*;
pub use gosort::go_sort_slice;
pub use json::*;
pub use lines::*;
pub use model::*;
pub use secrets::*;
