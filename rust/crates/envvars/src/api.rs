//! Variable API calls of `utils/env_var.go` (`VariableMainCallsAPI`).
//!
//! Errors are returned as the strings Go's `error.Error()` would produce.

use crate::model::{VariableResponse, VariableScope};
use qovery_api::models::ResultList;
use qovery_api::{ApiError, Client, Method};
use qovery_core::context::ServiceType;
use qovery_log::pterm::FG_RED;
use serde_json::{Map, Value, json};

/// Go `withAPIErrorDetail(err)`: append the API's own message (the `detail` of the problem response, e.g.
/// which scope already holds the key on a 409) to the bare status line, see [`qovery_api::with_api_error_detail`].
pub fn with_api_error_detail(err: &ApiError) -> String {
    qovery_api::with_api_error_detail(&err.message, &err.body)
}

fn not_found(what: &str, key: &str) -> String {
    format!("{what} {} not found", FG_RED.sprint(key))
}

/// `POST /variable` (Go `CreateVariable`), the error decorated by [`with_api_error_detail`].
fn create_variable(client: &Client, body: Value) -> Result<(), String> {
    let res = client
        .send(Method::POST, "/variable", &[], Some(&body))
        .map_err(|e| with_api_error_detail(&e))?;
    if res.status >= 300 {
        let message = if res.status == 409 {
            variable_conflict_error(&res)
        } else {
            res.status_line()
        };
        let err = ApiError {
            status: Some(res.status),
            message,
            body: res.text(),
        };
        return Err(with_api_error_detail(&err));
    }
    Client::decode::<Value>(res)
        .map(|_| ())
        .map_err(|e| with_api_error_detail(&e))
}

/// `GenericOpenAPIError.Error()` of a 409 on `POST /variable`: the Go client decodes the body as a
/// `VariableConflictResponse` (`status` and `detail` required) and formats it with `formatErrorMessage`, i.e.
/// `<status line> <title> (<detail>)` where `title` is a `*string` printed with `%s`: `%!s(*string=<nil>)` when
/// absent, a pointer address otherwise (it changes on every run, rendered here with a fixed address). A body
/// that cannot be decoded gives the decoding error alone. Field type errors are not reproduced.
fn variable_conflict_error(res: &qovery_api::Response) -> String {
    let empty = Map::new();
    let model = if res.body.is_empty() {
        // `decode` leaves the model zeroed
        &empty
    } else {
        let content_type = res
            .headers
            .get("content-type")
            .and_then(|v| v.to_str().ok())
            .unwrap_or_default();
        if !qovery_api::is_go_json_content_type(content_type) {
            return "undefined response type".to_string();
        }
        if let Err(e) = qovery_log::gojson::go_json_check_valid(&res.body) {
            return e;
        }
        let kind = match serde_json::from_slice::<Value>(&res.body) {
            Ok(Value::Object(m)) => {
                if let Some(p) = ["status", "detail"].iter().find(|p| !m.contains_key(**p)) {
                    return format!("no value given for required property {p}");
                }
                return format_variable_conflict(&res.status_line(), &m);
            }
            // `json.Unmarshal(null, &map)` leaves the map empty
            Ok(Value::Null) => return "no value given for required property status".to_string(),
            Ok(Value::Array(_)) => "array",
            Ok(Value::String(_)) => "string",
            Ok(Value::Number(_)) => "number",
            Ok(Value::Bool(_)) => "bool",
            Err(e) => return e.to_string(),
        };
        return format!(
            "json: cannot unmarshal {kind} into Go value of type map[string]interface {{}}"
        );
    };
    format_variable_conflict(&res.status_line(), model)
}

/// `formatErrorMessage(status, &VariableConflictResponse)`.
fn format_variable_conflict(status_line: &str, model: &Map<String, Value>) -> String {
    let title = match model.get("title") {
        None | Some(Value::Null) => "%!s(*string=<nil>)",
        Some(_) => "%!s(*string=0xc000010000)",
    };
    let detail = model
        .get("detail")
        .and_then(Value::as_str)
        .unwrap_or_default();
    format!("{status_line} {title} ({detail})")
        .trim()
        .to_string()
}

/// `PUT /variable/{id}` (Go `EditVariable`).
fn edit_variable(client: &Client, variable_id: &str, body: Value) -> Result<(), String> {
    client
        .put::<Value, _>(&format!("/variable/{variable_id}"), &body)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// `DELETE /variable/{id}` (Go `DeleteVariable`).
fn delete_variable(client: &Client, variable_id: &str) -> Result<(), String> {
    client
        .delete(&format!("/variable/{variable_id}"))
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// Go `VariableRequest` serialization: `mount_path`, `secret_manager_access_id` and `override_if_exists`
/// only when set.
#[allow(clippy::too_many_arguments)]
fn variable_request(
    key: &str,
    value: &str,
    is_secret: bool,
    scope: VariableScope,
    parent_id: &str,
    mount_path: Option<&str>,
    secret_manager_access_id: Option<&str>,
    override_if_exists: bool,
) -> Value {
    let mut m = Map::new();
    m.insert("key".into(), json!(key));
    m.insert("value".into(), json!(value));
    if let Some(p) = mount_path {
        m.insert("mount_path".into(), json!(p));
    }
    m.insert("is_secret".into(), json!(is_secret));
    m.insert("variable_scope".into(), json!(scope.as_str()));
    m.insert("variable_parent_id".into(), json!(parent_id));
    if let Some(id) = secret_manager_access_id {
        m.insert("secret_manager_access_id".into(), json!(id));
    }
    if override_if_exists {
        m.insert("override_if_exists".into(), json!(true));
    }
    Value::Object(m)
}

/// Go `VariableEditRequest` serialization (`value` / `secret_manager_access_id` only when set).
fn variable_edit_request(
    key: &str,
    value: Option<&str>,
    secret_manager_access_id: Option<&str>,
) -> Value {
    let mut m = Map::new();
    m.insert("key".into(), json!(key));
    if let Some(v) = value {
        m.insert("value".into(), json!(v));
    }
    if let Some(id) = secret_manager_access_id {
        m.insert("secret_manager_access_id".into(), json!(id));
    }
    Value::Object(m)
}

/// Go `utils.CreateServiceVariable(...)`: create a variable (or secret) at `scope` (`PROJECT`, `ENVIRONMENT`,
/// `APPLICATION`, `CONTAINER`, `JOB`, `HELM`, `TERRAFORM`) from a service command.
#[allow(clippy::too_many_arguments)]
pub fn create_service_variable(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    service_id: &str,
    scope: &str,
    key: &str,
    value: &str,
    is_secret: bool,
    override_if_exists: bool,
) -> Result<(), String> {
    let (parent_id, parent_scope) =
        get_parent_id_by_scope(scope, project_id, environment_id, service_id)?;
    create_variable(
        client,
        variable_request(
            key,
            value,
            is_secret,
            parent_scope,
            &parent_id,
            None,
            None,
            override_if_exists,
        ),
    )
}

/// Go `utils.CreateServiceExternalSecret(...)`: create an external secret (`value` = reference in the secret
/// manager, `is_secret` false, `mount_path` only when not empty).
#[allow(clippy::too_many_arguments)]
pub fn create_service_external_secret(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    service_id: &str,
    scope: &str,
    key: &str,
    reference: &str,
    secret_manager_access_id: &str,
    mount_path: &str,
) -> Result<(), String> {
    let (parent_id, parent_scope) =
        get_parent_id_by_scope(scope, project_id, environment_id, service_id)?;
    let mount_path = (!mount_path.is_empty()).then_some(mount_path);
    create_variable(
        client,
        variable_request(
            key,
            reference,
            false,
            parent_scope,
            &parent_id,
            mount_path,
            Some(secret_manager_access_id),
            false,
        ),
    )
}

/// Go `utils.UpdateServiceExternalSecret(...)`: edit the external secret `key` of a service (`reference` and
/// `secret_manager_access_id` only sent when not empty).
pub fn update_service_external_secret(
    client: &Client,
    key: &str,
    reference: &str,
    secret_manager_access_id: &str,
    service_id: &str,
    service_type: ServiceType,
) -> Result<(), String> {
    let env_vars = list_service_variables(client, service_id, service_type)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("external secret", key))?;
    edit_variable(
        client,
        &env_var.id,
        variable_edit_request(
            key,
            (!reference.is_empty()).then_some(reference),
            (!secret_manager_access_id.is_empty()).then_some(secret_manager_access_id),
        ),
    )
}

/// Go `utils.UpdateEnvironmentExternalSecret(...)`: edit the external secret `key` of an environment.
pub fn update_environment_external_secret(
    client: &Client,
    environment_id: &str,
    key: &str,
    reference: &str,
    secret_manager_access_id: &str,
) -> Result<(), String> {
    let env_vars = list_environment_variables(client, environment_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("external secret", key))?;
    edit_variable(
        client,
        &env_var.id,
        variable_edit_request(
            key,
            (!reference.is_empty()).then_some(reference),
            (!secret_manager_access_id.is_empty()).then_some(secret_manager_access_id),
        ),
    )
}

/// Go `utils.CreateEnvironmentVariable(...)`: create a variable (or secret) at the `ENVIRONMENT` scope.
/// `project_id` is unused (like Go).
pub fn create_environment_variable(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    key: &str,
    value: &str,
    is_secret: bool,
    override_if_exists: bool,
) -> Result<(), String> {
    let _ = project_id;
    create_variable(
        client,
        variable_request(
            key,
            value,
            is_secret,
            VariableScope::Environment,
            environment_id,
            None,
            None,
            override_if_exists,
        ),
    )
}

/// Go `utils.CreateProjectVariable(...)`: create a variable (or secret) at the `PROJECT` scope.
pub fn create_project_variable(
    client: &Client,
    project_id: &str,
    key: &str,
    value: &str,
    is_secret: bool,
) -> Result<(), String> {
    create_variable(
        client,
        variable_request(
            key,
            value,
            is_secret,
            VariableScope::Project,
            project_id,
            None,
            None,
            false,
        ),
    )
}

/// Go `utils.UpdateServiceVariable(...)`: set the value of the variable `key` of a service.
pub fn update_service_variable(
    client: &Client,
    key: &str,
    value: &str,
    service_id: &str,
    service_type: ServiceType,
) -> Result<(), String> {
    let env_vars = list_service_variables(client, service_id, service_type)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("environment variable", key))?;
    edit_variable(
        client,
        &env_var.id,
        variable_edit_request(key, Some(value), None),
    )
}

/// Go `utils.UpdateEnvironmentVariable(...)`: set the value of the variable `key` of an environment.
pub fn update_environment_variable(
    client: &Client,
    environment_id: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    let env_vars = list_environment_variables(client, environment_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("environment variable", key))?;
    edit_variable(
        client,
        &env_var.id,
        variable_edit_request(key, Some(value), None),
    )
}

/// Go `utils.UpdateProjectVariable(...)`: set the value of the variable `key` of a project.
pub fn update_project_variable(
    client: &Client,
    project_id: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    let env_vars = list_project_variables(client, project_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("project variable", key))?;
    edit_variable(
        client,
        &env_var.id,
        variable_edit_request(key, Some(value), None),
    )
}

/// Go `utils.FindEnvironmentVariableByKey(key, envVars)`: the first variable named `key`.
pub fn find_environment_variable_by_key<'a>(
    key: &str,
    env_vars: &'a [VariableResponse],
) -> Option<&'a VariableResponse> {
    env_vars.iter().find(|v| v.key == key)
}

/// `GET /variable?parent_id=...&scope=...` (Go `ListVariables`); `empty_error` when the body is empty/null.
fn list_variables(
    client: &Client,
    parent_id: &str,
    scope: VariableScope,
    empty_error: &str,
) -> Result<Vec<VariableResponse>, String> {
    let res: Option<ResultList<VariableResponse>> = client
        .get_with_query(
            "/variable",
            &[
                ("parent_id", parent_id.to_string()),
                ("scope", scope.as_str().to_string()),
            ],
        )
        .map_err(|e| e.to_string())?;
    match res {
        Some(r) => Ok(r.results),
        None => Err(empty_error.to_string()),
    }
}

/// Go `utils.ListServiceVariables(client, serviceId, serviceType)`: all the variables visible from a service.
pub fn list_service_variables(
    client: &Client,
    service_id: &str,
    service_type: ServiceType,
) -> Result<Vec<VariableResponse>, String> {
    let scope = service_type_to_scope(service_type)?;
    list_variables(client, service_id, scope, "invalid service type")
}

/// Go `utils.ListEnvironmentVariables(client, environmentId)`.
pub fn list_environment_variables(
    client: &Client,
    environment_id: &str,
) -> Result<Vec<VariableResponse>, String> {
    list_variables(
        client,
        environment_id,
        VariableScope::Environment,
        "invalid environment",
    )
}

/// Go `utils.ListProjectVariables(client, projectId)`.
pub fn list_project_variables(
    client: &Client,
    project_id: &str,
) -> Result<Vec<VariableResponse>, String> {
    list_variables(
        client,
        project_id,
        VariableScope::Project,
        "invalid project",
    )
}

/// Go `utils.ServiceTypeToScope(serviceType)`: `the service type database is not supported` for databases.
pub fn service_type_to_scope(service_type: ServiceType) -> Result<VariableScope, String> {
    match service_type {
        ServiceType::Application => Ok(VariableScope::Application),
        ServiceType::Container => Ok(VariableScope::Container),
        ServiceType::Job => Ok(VariableScope::Job),
        ServiceType::Helm => Ok(VariableScope::Helm),
        ServiceType::Terraform => Ok(VariableScope::Terraform),
        ServiceType::Database => Err(format!("the service type {service_type} is not supported")),
    }
}

/// Go `getParentIdByScope(scope, projectId, environmentId, serviceId)`: the parent id and API scope of a
/// `--scope` flag value (`scope X not supported` otherwise; the comparison is case-sensitive).
pub fn get_parent_id_by_scope(
    scope: &str,
    project_id: &str,
    environment_id: &str,
    service_id: &str,
) -> Result<(String, VariableScope), String> {
    Ok(match scope {
        "PROJECT" => (project_id.to_string(), VariableScope::Project),
        "ENVIRONMENT" => (environment_id.to_string(), VariableScope::Environment),
        "APPLICATION" => (service_id.to_string(), VariableScope::Application),
        "CONTAINER" => (service_id.to_string(), VariableScope::Container),
        "JOB" => (service_id.to_string(), VariableScope::Job),
        "HELM" => (service_id.to_string(), VariableScope::Helm),
        "TERRAFORM" => (service_id.to_string(), VariableScope::Terraform),
        _ => return Err(format!("scope {scope} not supported")),
    })
}

/// Go `utils.DeleteServiceVariable(client, serviceId, serviceType, key)`.
pub fn delete_service_variable(
    client: &Client,
    service_id: &str,
    service_type: ServiceType,
    key: &str,
) -> Result<(), String> {
    let env_vars = list_service_variables(client, service_id, service_type)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("environment variable", key))?;
    delete_variable(client, &env_var.id)
}

/// Go `utils.DeleteEnvironmentVar(client, environmentId, key)`.
pub fn delete_environment_var(
    client: &Client,
    environment_id: &str,
    key: &str,
) -> Result<(), String> {
    let env_vars = list_environment_variables(client, environment_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("environment variable", key))?;
    delete_variable(client, &env_var.id)
}

/// Go `utils.DeleteProjectVar(client, projectId, key)` (message says `environment variable`, like Go).
pub fn delete_project_var(client: &Client, project_id: &str, key: &str) -> Result<(), String> {
    let env_vars = list_project_variables(client, project_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars)
        .ok_or_else(|| not_found("environment variable", key))?;
    delete_variable(client, &env_var.id)
}

/// Go `utils.CreateEnvironmentVariableAlias(client, aliasParentId, aliasScope, variableId, alias)`:
/// `POST /variable/{variableId}/alias`.
pub fn create_environment_variable_alias(
    client: &Client,
    alias_parent_id: &str,
    alias_scope: VariableScope,
    variable_id: &str,
    alias: &str,
) -> Result<(), String> {
    let body = json!({"key": alias, "alias_scope": alias_scope.as_str(), "alias_parent_id": alias_parent_id});
    client
        .post::<Value, _>(&format!("/variable/{variable_id}/alias"), &body)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// Go `utils.CreateServiceAlias(...)`: alias the variable `key` (visible from the service) as `alias` at `scope`.
#[allow(clippy::too_many_arguments)]
pub fn create_service_alias(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    service_id: &str,
    service_type: ServiceType,
    key: &str,
    alias: &str,
    scope: &str,
) -> Result<(), String> {
    let env_vars = list_service_variables(client, service_id, service_type)?;
    let env_var = find_environment_variable_by_key(key, &env_vars);
    let (parent_id, parent_scope) =
        get_parent_id_by_scope(scope, project_id, environment_id, service_id)?;
    match env_var {
        Some(v) => {
            create_environment_variable_alias(client, &parent_id, parent_scope, &v.id, alias)
        }
        None => Err(not_found("Environment variable or secret", key)),
    }
}

/// Go `utils.CreateEnvironmentAlias(...)`.
pub fn create_environment_alias(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    key: &str,
    alias: &str,
    scope: &str,
) -> Result<(), String> {
    let env_vars = list_environment_variables(client, environment_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars);
    let (parent_id, parent_scope) = get_parent_id_by_scope(scope, project_id, environment_id, "")?;
    match env_var {
        Some(v) => {
            create_environment_variable_alias(client, &parent_id, parent_scope, &v.id, alias)
        }
        None => Err(not_found("Environment variable or secret", key)),
    }
}

/// Go `utils.CreateProjectAlias(...)` (alias created at the `PROJECT` scope).
pub fn create_project_alias(
    client: &Client,
    project_id: &str,
    key: &str,
    alias: &str,
) -> Result<(), String> {
    let env_vars = list_project_variables(client, project_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars);
    let (parent_id, parent_scope) = get_parent_id_by_scope("PROJECT", project_id, "", "")?;
    match env_var {
        Some(v) => {
            create_environment_variable_alias(client, &parent_id, parent_scope, &v.id, alias)
        }
        None => Err(not_found("Project variable or secret", key)),
    }
}

/// Go `utils.CreateEnvironmentVariableOverride(client, overrideParentId, overrideScope, variableId, value)`:
/// `POST /variable/{variableId}/override`.
pub fn create_environment_variable_override(
    client: &Client,
    override_parent_id: &str,
    override_scope: VariableScope,
    variable_id: &str,
    value: &str,
) -> Result<(), String> {
    let body = json!({"value": value, "override_scope": override_scope.as_str(), "override_parent_id": override_parent_id});
    client
        .post::<Value, _>(&format!("/variable/{variable_id}/override"), &body)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// Go `utils.CreateServiceOverride(...)`: override the variable `key` (visible from the service) at `scope`.
#[allow(clippy::too_many_arguments)]
pub fn create_service_override(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    service_id: &str,
    service_type: ServiceType,
    key: &str,
    value: &str,
    scope: &str,
) -> Result<(), String> {
    let env_vars = list_service_variables(client, service_id, service_type)?;
    let env_var = find_environment_variable_by_key(key, &env_vars);
    let (parent_id, parent_scope) =
        get_parent_id_by_scope(scope, project_id, environment_id, service_id)?;
    match env_var {
        Some(v) => {
            create_environment_variable_override(client, &parent_id, parent_scope, &v.id, value)
        }
        None => Err(not_found("Environment variable or secret", key)),
    }
}

/// Go `utils.CreateEnvironmentOverride(...)`.
pub fn create_environment_override(
    client: &Client,
    project_id: &str,
    environment_id: &str,
    key: &str,
    value: &str,
    scope: &str,
) -> Result<(), String> {
    let env_vars = list_environment_variables(client, environment_id)?;
    let env_var = find_environment_variable_by_key(key, &env_vars);
    let (parent_id, parent_scope) = get_parent_id_by_scope(scope, project_id, environment_id, "")?;
    match env_var {
        Some(v) => {
            create_environment_variable_override(client, &parent_id, parent_scope, &v.id, value)
        }
        None => Err(not_found("Environment variable or secret", key)),
    }
}
