//! `helm update` (helm_update.go): `PUT /helm/{id}` with a `qovery.HelmRequest` rebuilt from the
//! `HelmResponse`, the chart name / version / branches being overridable by flags.
//!
//! The JSON body reproduces the Go client serialization: pointer fields are omitted when nil (absent or `null`
//! in the response), `Nullable*` fields are sent when they were present in the response (even `null`).

use crate::actions::exit_with_api_error;
use crate::context_flags;
use qovery_api::Method;
use qovery_api::models::Helm;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{exit_with_error, telemetry};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{get_git_source, get_helm_repository, path_escape};
use serde_json::{Map, Value, json};

pub(crate) fn update() -> Command {
    context_flags(Command::new("update").short("Update a helm"))
        .flag(Flag::string("helm", "helm Name").short('n').required())
        .flag(Flag::string("chart_name", "helm chart name"))
        .flag(Flag::string("chart_version", "helm chart version"))
        .flag(Flag::string(
            "chart_git_commit_branch",
            "helm chart version",
        ))
        .flag(Flag::string(
            "values_override_git_commit_branch",
            "helm chart version",
        ))
        .run(run_update)
}

/// A Go `*T` field decoded from the response: `None` when absent or `null`.
fn ptr<'a>(obj: &'a Value, key: &str) -> Option<&'a Value> {
    obj.get(key).filter(|v| !v.is_null())
}

/// Copy a Go pointer field (omitted when nil).
fn copy_ptr(dst: &mut Map<String, Value>, src: &Value, key: &str) {
    if let Some(v) = ptr(src, key) {
        dst.insert(key.to_string(), v.clone());
    }
}

/// Copy a Go `Nullable*` field (sent when it was present in the response, `null` included).
fn copy_nullable(dst: &mut Map<String, Value>, src: &Value, key: &str) {
    if let Some(v) = src.get(key) {
        dst.insert(key.to_string(), v.clone());
    }
}

/// Go value of a non-pointer string field (zero value when absent / null).
fn string_field(obj: &Value, key: &str) -> Value {
    Value::String(
        obj.get(key)
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string(),
    )
}

/// Go value of a non-pointer field of any type, `zero` when absent / null.
fn field_or(obj: &Value, key: &str, zero: Value) -> Value {
    ptr(obj, key).cloned().unwrap_or(zero)
}

/// Go value of a slice field (`null` when nil).
fn slice_field(obj: &Value, key: &str) -> Value {
    obj.get(key).cloned().unwrap_or(Value::Null)
}

/// `[]qovery.HelmPortRequestPortsInner` from the `HelmPortResponseWithServiceName` ports (nil when none).
fn ports_request(helm: &Value) -> Option<Value> {
    let ports = helm.get("ports").and_then(Value::as_array)?;
    let mut out = Vec::new();
    for p in ports {
        let port_type = p
            .get("port_type")
            .and_then(Value::as_str)
            .unwrap_or_default();
        if port_type != "SERVICE_NAME" && port_type != "HelmPortResponseWithServiceName" {
            continue;
        }
        let mut m = Map::new();
        copy_ptr(&mut m, p, "name");
        m.insert(
            "internal_port".into(),
            field_or(p, "internal_port", json!(0)),
        );
        copy_ptr(&mut m, p, "external_port");
        copy_ptr(&mut m, p, "namespace");
        m.insert("protocol".into(), string_field(p, "protocol"));
        m.insert("service_name".into(), string_field(p, "service_name"));
        out.push(Value::Object(m));
    }
    if out.is_empty() {
        None
    } else {
        Some(Value::Array(out))
    }
}

/// Go `GetHelmSource(helm, chartName, chartVersion, charGitCommitBranch)`.
fn get_helm_source(
    helm: &Helm,
    chart_name: &str,
    chart_version: &str,
    chart_git_commit_branch: &str,
) -> Result<Value, String> {
    if let Some(git) = get_git_source(helm) {
        let repo = git.get("git_repository").cloned().unwrap_or(Value::Null);
        let mut m = Map::new();
        m.insert("url".into(), string_field(&repo, "url"));
        if !chart_git_commit_branch.is_empty() {
            m.insert("branch".into(), json!(chart_git_commit_branch));
        } else {
            copy_ptr(&mut m, &repo, "branch");
        }
        copy_ptr(&mut m, &repo, "root_path");
        copy_nullable(&mut m, &repo, "git_token_id");
        return Ok(json!({ "git_repository": Value::Object(m) }));
    }
    if let Some(repository) = get_helm_repository(helm) {
        let chart_name = if chart_name.is_empty() {
            string_field(repository, "chart_name")
        } else {
            json!(chart_name)
        };
        let chart_version = if chart_version.is_empty() {
            string_field(repository, "chart_version")
        } else {
            json!(chart_version)
        };
        let repository_id = repository
            .get("repository")
            .map(|r| string_field(r, "id"))
            .unwrap_or(json!(""));
        return Ok(json!({
            "helm_repository": {
                "repository": repository_id,
                "chart_name": chart_name,
                "chart_version": chart_version,
            }
        }));
    }
    Err("invalid Helm source".to_string())
}

/// Go `GetHelmValuesOverride(helm, valuesOverrideCommitBranch)`.
fn get_helm_values_override(
    helm: &Value,
    values_override_commit_branch: &str,
) -> Result<Value, String> {
    let vo = helm.get("values_override").cloned().unwrap_or(Value::Null);
    let mut m = Map::new();
    copy_ptr(&mut m, &vo, "set");
    copy_ptr(&mut m, &vo, "set_string");
    copy_ptr(&mut m, &vo, "set_json");

    let file = ptr(&vo, "file");
    if let Some(git) = file.and_then(|f| ptr(f, "git")) {
        let repo = git.get("git_repository").cloned().unwrap_or(Value::Null);
        let mut r = Map::new();
        r.insert("url".into(), string_field(&repo, "url"));
        if !values_override_commit_branch.is_empty() {
            r.insert("branch".into(), json!(values_override_commit_branch));
        } else {
            copy_ptr(&mut r, &repo, "branch");
        }
        copy_ptr(&mut r, &repo, "root_path");
        copy_nullable(&mut r, &repo, "git_token_id");
        r.insert("provider".into(), string_field(&repo, "provider"));
        m.insert(
            "file".into(),
            json!({
                "git": { "git_repository": Value::Object(r), "paths": slice_field(git, "paths") },
                "raw": Value::Null,
            }),
        );
        return Ok(Value::Object(m));
    }
    if let Some(raw) = file.and_then(|f| ptr(f, "raw")) {
        let values: Vec<Value> = raw
            .get("values")
            .and_then(Value::as_array)
            .map(|vs| {
                vs.iter()
                    .map(|v| json!({ "name": string_field(v, "name"), "content": string_field(v, "content") }))
                    .collect()
            })
            .unwrap_or_default();
        m.insert("file".into(), json!({ "raw": { "values": values } }));
        return Ok(Value::Object(m));
    }
    Err("invalid Helm values orerride".to_string())
}

fn run_update(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    let helm_value = qovery_api::models::Resource::to_value(&helm);

    let ports = ports_request(&helm_value);
    let source = get_helm_source(
        &helm,
        &ctx.string("chart_name"),
        &ctx.string("chart_version"),
        &ctx.string("chart_git_commit_branch"),
    )
    .unwrap_or_else(|e| exit_with_error(e));
    let values_override = get_helm_values_override(
        &helm_value,
        &ctx.string("values_override_git_commit_branch"),
    )
    .unwrap_or_else(|e| exit_with_error(e));

    let mut req = Map::new();
    if let Some(p) = ports {
        req.insert("ports".into(), p);
    }
    req.insert("name".into(), json!(helm.name));
    copy_ptr(&mut req, &helm_value, "description");
    copy_ptr(&mut req, &helm_value, "timeout_sec");
    req.insert(
        "auto_preview".into(),
        field_or(&helm_value, "auto_preview", json!(false)),
    );
    req.insert(
        "auto_deploy".into(),
        field_or(&helm_value, "auto_deploy", json!(false)),
    );
    req.insert("source".into(), source);
    req.insert("arguments".into(), slice_field(&helm_value, "arguments"));
    req.insert(
        "allow_cluster_wide_resources".into(),
        field_or(&helm_value, "allow_cluster_wide_resources", json!(false)),
    );
    req.insert("values_override".into(), values_override);

    let body = Value::Object(req);
    if let Err(e) = client.execute(
        Method::PUT,
        &format!("/helm/{}", path_escape(&helm.id)),
        &[],
        Some(&body),
    ) {
        exit_with_api_error(e);
    }
    qovery_log::println(format!(
        "helm {} updated!",
        FG_BLUE.sprint(ctx.string("helm"))
    ));
}

#[cfg(test)]
mod tests {
    use super::*;

    fn helm(v: Value) -> Helm {
        serde_json::from_value(v).unwrap()
    }

    #[test]
    fn source_repository_overrides() {
        let h = helm(
            json!({"id": "h", "name": "n", "source": {"repository": {"chart_name": "redis", "chart_version": "1", "repository": {"id": "r"}}}}),
        );
        assert_eq!(
            get_helm_source(&h, "", "2", "").unwrap(),
            json!({"helm_repository": {"repository": "r", "chart_name": "redis", "chart_version": "2"}})
        );
    }

    #[test]
    fn source_git_keeps_nullable_token() {
        let h = helm(
            json!({"id": "h", "name": "n", "source": {"git": {"git_repository": {"url": "u", "branch": "main", "root_path": null, "git_token_id": null}}}}),
        );
        assert_eq!(
            get_helm_source(&h, "", "", "dev").unwrap(),
            json!({"git_repository": {"url": "u", "branch": "dev", "git_token_id": null}})
        );
        let h = helm(json!({"id": "h", "name": "n", "source": null}));
        assert_eq!(
            get_helm_source(&h, "", "", "").unwrap_err(),
            "invalid Helm source"
        );
    }

    #[test]
    fn values_override_variants() {
        let v = json!({"values_override": {"set": [["a", "b"]], "set_json": null, "file": {"raw": {"values": [{"name": "x", "content": "y"}]}}}});
        assert_eq!(
            get_helm_values_override(&v, "").unwrap(),
            json!({"set": [["a", "b"]], "file": {"raw": {"values": [{"name": "x", "content": "y"}]}}})
        );
        let v = json!({"values_override": {"file": {"git": {"paths": ["/v.yaml"], "git_repository": {"url": "u", "provider": "GITHUB"}}}}});
        assert_eq!(
            get_helm_values_override(&v, "b").unwrap(),
            json!({"file": {"git": {"paths": ["/v.yaml"], "git_repository": {"url": "u", "branch": "b", "provider": "GITHUB"}}, "raw": null}})
        );
        let v = json!({"values_override": {"file": null}});
        assert_eq!(
            get_helm_values_override(&v, "").unwrap_err(),
            "invalid Helm values orerride"
        );
    }
}
