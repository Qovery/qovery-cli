//! Blueprint -> RDE service configuration sync used by `rde upgrade --strategy image`.
//! Port of `cmd/rde_sync.go`.
//!
//! The request bodies are built as JSON values that serialize like the Go client models
//! (`ContainerRequest`, `ApplicationEditRequest`, `JobRequest`, `HelmRequest`): unset pointers omitted,
//! nullable fields kept when present in the response, nil slices omitted (or `null` for required ones).

use crate::helpers::{blue, esc};
use qovery_api::Client;
use qovery_api::models::Job;
use qovery_services::{
    convert_autoscaling_response_to_request, get_job_id, get_job_name, go_healthcheck,
};
use serde_json::{Map, Value, json};
use std::collections::HashMap;

/// The `--sync-*` flags.
#[derive(Debug, Clone, Default)]
pub struct SyncOptions {
    pub all: bool,
    pub resources: bool,
    pub ports: bool,
    pub healthchecks: bool,
    pub storage: bool,
}

impl SyncOptions {
    fn resources(&self) -> bool {
        self.all || self.resources
    }
    fn ports(&self) -> bool {
        self.all || self.ports
    }
    fn healthchecks(&self) -> bool {
        self.all || self.healthchecks
    }
    fn storage(&self) -> bool {
        self.all || self.storage
    }
}

/// Go `rdeSyncServicesFromBlueprint`.
pub fn sync_services_from_blueprint(
    client: &Client,
    blueprint_env_id: &str,
    child_env_id: &str,
    opts: &SyncOptions,
) -> usize {
    let mut synced = 0;
    synced += sync_containers(client, blueprint_env_id, child_env_id, opts);
    synced += sync_applications(client, blueprint_env_id, child_env_id, opts);
    synced += sync_jobs(client, blueprint_env_id, child_env_id, opts);
    synced += sync_helms(client, blueprint_env_id, child_env_id);
    synced
}

// ---- Go model field helpers -------------------------------------------------------------------------------

/// Go pointer field (`*T`): present and not null.
fn ptr(o: &Value, key: &str) -> Option<Value> {
    o.get(key).filter(|v| !v.is_null()).cloned()
}

/// Go `NullableT` field: set when the key is present (even `null`).
fn nullable(o: &Value, key: &str) -> Option<Value> {
    o.get(key).cloned()
}

/// Go slice field decoded from JSON: `None` (nil) when missing or null, `[]` kept.
fn slice(o: &Value, key: &str) -> Option<Vec<Value>> {
    o.get(key).and_then(|v| v.as_array()).cloned()
}

/// Go required string field.
fn string(o: &Value, key: &str) -> Value {
    json!(o.get(key).and_then(|v| v.as_str()).unwrap_or_default())
}

/// Go required number field (int32).
fn int(o: &Value, key: &str) -> Value {
    o.get(key)
        .filter(|v| v.is_number())
        .cloned()
        .unwrap_or(json!(0))
}

/// Go required bool field.
fn boolean(o: &Value, key: &str) -> Value {
    json!(o.get(key).and_then(|v| v.as_bool()).unwrap_or(false))
}

fn insert_opt(m: &mut Map<String, Value>, key: &str, v: Option<Value>) {
    if let Some(v) = v {
        m.insert(key.to_string(), v);
    }
}

/// `qovery.ServiceStorageRequestStorageInner` list built from `ServiceStorageStorageInner` (nil when empty).
fn storage_request(src: Option<Vec<Value>>) -> Option<Value> {
    let src = src.unwrap_or_default();
    if src.is_empty() {
        return None;
    }
    Some(Value::Array(
        src.iter()
            .map(|s| {
                json!({
                    "id": string(s, "id"),
                    "type": string(s, "type"),
                    "size": int(s, "size"),
                    "mount_point": string(s, "mount_point"),
                })
            })
            .collect(),
    ))
}

/// A `qovery.ServicePort` decoded then re-encoded (unknown keys kept, null pointers dropped).
fn go_service_port(p: &Value) -> Value {
    let Some(obj) = p.as_object() else {
        return p.clone();
    };
    let pointer_keys = [
        "name",
        "external_port",
        "is_default",
        "public_path",
        "public_path_rewrite",
    ];
    let mut out = Map::new();
    for (k, v) in obj {
        if v.is_null() && pointer_keys.contains(&k.as_str()) {
            continue;
        }
        out.insert(k.clone(), v.clone());
    }
    for (k, def) in [
        ("id", json!("")),
        ("internal_port", json!(0)),
        ("publicly_accessible", json!(false)),
        ("protocol", json!("")),
    ] {
        out.entry(k.to_string()).or_insert(def);
    }
    Value::Object(out)
}

/// `qovery.ApplicationGitRepositoryRequest` from an `ApplicationGitRepository`.
fn git_repository_request(g: &Value) -> Value {
    let mut m = Map::new();
    m.insert("url".into(), string(g, "url"));
    insert_opt(&mut m, "branch", ptr(g, "branch"));
    insert_opt(&mut m, "root_path", ptr(g, "root_path"));
    m.insert("provider".into(), string(g, "provider"));
    Value::Object(m)
}

/// List a service type of an environment as raw JSON (Go returns nil on error).
fn list_values(client: &Client, env_id: &str, kind: &str) -> Option<Vec<Value>> {
    client
        .list::<Value>(&format!("/environment/{}/{kind}", esc(env_id)))
        .ok()
}

/// Map by name; later entries override earlier ones like a Go map assignment.
fn by_name(items: &[Value]) -> HashMap<String, &Value> {
    let mut m = HashMap::new();
    for i in items {
        m.insert(i["name"].as_str().unwrap_or_default().to_string(), i);
    }
    m
}

// ---- Container sync ---------------------------------------------------------------------------------------

fn sync_containers(client: &Client, bp_env: &str, child_env: &str, opts: &SyncOptions) -> usize {
    let Some(bp_containers) = list_values(client, bp_env, "container") else {
        return 0;
    };
    let Some(child_containers) = list_values(client, child_env, "container") else {
        return 0;
    };
    let bp_map = by_name(&bp_containers);

    let mut synced = 0;
    for child in &child_containers {
        let name = child["name"].as_str().unwrap_or_default();
        let Some(bp) = bp_map.get(name) else {
            continue;
        };

        let storage = storage_request(slice(if opts.storage() { bp } else { child }, "storage"));

        let src_ports = slice(if opts.ports() { bp } else { child }, "ports").unwrap_or_default();
        let ports: Option<Value> = (!src_ports.is_empty()).then(|| {
            Value::Array(
                src_ports
                    .iter()
                    .map(|p| {
                        let mut m = Map::new();
                        insert_opt(&mut m, "name", ptr(p, "name"));
                        m.insert("internal_port".into(), int(p, "internal_port"));
                        insert_opt(&mut m, "external_port", ptr(p, "external_port"));
                        m.insert(
                            "publicly_accessible".into(),
                            boolean(p, "publicly_accessible"),
                        );
                        insert_opt(&mut m, "is_default", ptr(p, "is_default"));
                        m.insert("protocol".into(), string(p, "protocol"));
                        Value::Object(m)
                    })
                    .collect(),
            )
        });

        let res = if opts.resources() { *bp } else { child };
        let hc = if opts.healthchecks() { *bp } else { child };

        let mut req = Map::new();
        insert_opt(&mut req, "storage", storage);
        insert_opt(&mut req, "ports", ports);
        req.insert("name".into(), json!(name));
        insert_opt(&mut req, "description", ptr(child, "description"));
        req.insert(
            "registry_id".into(),
            string(bp.get("registry").unwrap_or(&Value::Null), "id"),
        );
        req.insert("image_name".into(), string(bp, "image_name"));
        req.insert("tag".into(), string(bp, "tag"));
        insert_opt(
            &mut req,
            "arguments",
            slice(child, "arguments").map(Value::Array),
        );
        insert_opt(&mut req, "entrypoint", ptr(child, "entrypoint"));
        req.insert("cpu".into(), int(res, "cpu"));
        req.insert("memory".into(), int(res, "memory"));
        req.insert(
            "min_running_instances".into(),
            int(res, "min_running_instances"),
        );
        req.insert(
            "max_running_instances".into(),
            int(res, "max_running_instances"),
        );
        req.insert(
            "healthchecks".into(),
            go_healthcheck(hc.get("healthchecks").unwrap_or(&Value::Null)),
        );
        req.insert("auto_preview".into(), boolean(child, "auto_preview"));
        req.insert(
            "auto_deploy".into(),
            ptr(child, "auto_deploy").unwrap_or(Value::Null),
        );
        insert_opt(
            &mut req,
            "autoscaling",
            convert_autoscaling_response_to_request(res.get("autoscaling")),
        );

        let id = child["id"].as_str().unwrap_or_default();
        match client.put::<Value, _>(&format!("/container/{}", esc(id)), &Value::Object(req)) {
            Err(e) => {
                qovery_log::println(format!("    WARNING: Failed to sync container {name}: {e}"))
            }
            Ok(_) => {
                qovery_log::println(format!(
                    "    Synced container: {} (tag: {})",
                    blue(name),
                    bp["tag"].as_str().unwrap_or_default()
                ));
                synced += 1;
            }
        }
    }
    synced
}

// ---- Application sync -------------------------------------------------------------------------------------

fn sync_applications(client: &Client, bp_env: &str, child_env: &str, opts: &SyncOptions) -> usize {
    let Some(bp_apps) = list_values(client, bp_env, "application") else {
        return 0;
    };
    let Some(child_apps) = list_values(client, child_env, "application") else {
        return 0;
    };
    let bp_map = by_name(&bp_apps);

    let mut synced = 0;
    for child in &child_apps {
        let name = child["name"].as_str().unwrap_or_default();
        let Some(bp) = bp_map.get(name) else {
            continue;
        };

        let git = bp.get("git_repository").filter(|g| g.is_object());
        let storage = storage_request(slice(if opts.storage() { bp } else { child }, "storage"));
        let res = if opts.resources() { *bp } else { child };
        let hc = if opts.healthchecks() { *bp } else { child };
        let ports = slice(if opts.ports() { bp } else { child }, "ports")
            .map(|p| Value::Array(p.iter().map(go_service_port).collect()));

        let mut req = Map::new();
        insert_opt(&mut req, "storage", storage);
        req.insert("name".into(), json!(name));
        insert_opt(&mut req, "description", ptr(child, "description"));
        insert_opt(&mut req, "git_repository", git.map(git_repository_request));
        insert_opt(&mut req, "build_mode", ptr(bp, "build_mode"));
        insert_opt(&mut req, "dockerfile_path", nullable(bp, "dockerfile_path"));
        insert_opt(&mut req, "cpu", ptr(res, "cpu"));
        insert_opt(&mut req, "memory", ptr(res, "memory"));
        insert_opt(
            &mut req,
            "min_running_instances",
            ptr(res, "min_running_instances"),
        );
        insert_opt(
            &mut req,
            "max_running_instances",
            ptr(res, "max_running_instances"),
        );
        req.insert(
            "healthchecks".into(),
            go_healthcheck(hc.get("healthchecks").unwrap_or(&Value::Null)),
        );
        insert_opt(&mut req, "auto_preview", ptr(child, "auto_preview"));
        insert_opt(&mut req, "ports", ports);
        insert_opt(
            &mut req,
            "arguments",
            slice(child, "arguments").map(Value::Array),
        );
        insert_opt(&mut req, "entrypoint", ptr(child, "entrypoint"));
        req.insert(
            "auto_deploy".into(),
            ptr(child, "auto_deploy").unwrap_or(Value::Null),
        );

        let id = child["id"].as_str().unwrap_or_default();
        match client.put::<Value, _>(&format!("/application/{}", esc(id)), &Value::Object(req)) {
            Err(e) => qovery_log::println(format!(
                "    WARNING: Failed to sync application {name}: {e}"
            )),
            Ok(_) => {
                let branch = git.and_then(|g| g["branch"].as_str()).unwrap_or_default();
                qovery_log::println(format!(
                    "    Synced application: {} (branch: {branch})",
                    blue(name)
                ));
                synced += 1;
            }
        }
    }
    synced
}

// ---- Job sync ---------------------------------------------------------------------------------------------

fn is_cron(j: &Value) -> bool {
    matches!(j["job_type"].as_str(), Some("CRON" | "CronJobResponse"))
}

fn is_lifecycle(j: &Value) -> bool {
    matches!(
        j["job_type"].as_str(),
        Some("LIFECYCLE" | "LifecycleJobResponse")
    )
}

/// Go `rdeJobResponseToRequestSource`: `JobRequestAllOfSource` from the job source.
fn job_request_source(job: &Value) -> Option<Value> {
    if !is_cron(job) && !is_lifecycle(job) {
        return None;
    }
    let source = job.get("source").unwrap_or(&Value::Null);
    let mut result = Map::new();
    if let Some(img) = source.get("image").filter(|v| v.is_object()) {
        result.insert(
            "image".into(),
            json!({
                "image_name": string(img, "image_name"),
                "tag": string(img, "tag"),
                "registry_id": string(img.get("registry").unwrap_or(&Value::Null), "id"),
            }),
        );
    } else if let Some(docker) = source.get("docker").filter(|v| v.is_object()) {
        let mut d = Map::new();
        insert_opt(
            &mut d,
            "git_repository",
            docker
                .get("git_repository")
                .filter(|g| g.is_object())
                .map(git_repository_request),
        );
        insert_opt(
            &mut d,
            "dockerfile_path",
            nullable(docker, "dockerfile_path"),
        );
        insert_opt(&mut d, "dockerfile_raw", nullable(docker, "dockerfile_raw"));
        insert_opt(
            &mut d,
            "docker_target_build_stage",
            nullable(docker, "docker_target_build_stage"),
        );
        result.insert("docker".into(), Value::Object(d));
    }
    Some(Value::Object(result))
}

/// Go `jobDetail`.
struct JobDetail {
    cpu: Value,
    memory: Value,
    description: Option<Value>,
    max_nb_restart: Option<Value>,
    max_duration_seconds: Option<Value>,
    auto_preview: Value,
    port: Option<Value>,
    healthchecks: Value,
    schedule: Value,
    auto_deploy: Value,
}

/// `JobRequestAllOfScheduleOnStart` copy of a lifecycle schedule entry.
fn schedule_on(v: Option<&Value>) -> Option<Value> {
    let v = v.filter(|v| v.is_object())?;
    let mut m = Map::new();
    insert_opt(&mut m, "arguments", slice(v, "arguments").map(Value::Array));
    insert_opt(&mut m, "entrypoint", ptr(v, "entrypoint"));
    Some(Value::Object(m))
}

/// Go `rdeExtractJobDetail`.
fn job_detail(job: &Value) -> Option<JobDetail> {
    let empty = Value::Object(Map::new());
    let schedule_v = job
        .get("schedule")
        .filter(|s| s.is_object())
        .unwrap_or(&empty);
    let schedule = if is_cron(job) {
        let cron = schedule_v
            .get("cronjob")
            .filter(|c| c.is_object())
            .unwrap_or(&empty);
        let mut c = Map::new();
        insert_opt(
            &mut c,
            "arguments",
            slice(cron, "arguments").map(Value::Array),
        );
        insert_opt(&mut c, "entrypoint", ptr(cron, "entrypoint"));
        c.insert("timezone".into(), string(cron, "timezone"));
        c.insert("scheduled_at".into(), string(cron, "scheduled_at"));
        json!({"cronjob": Value::Object(c)})
    } else if is_lifecycle(job) {
        let mut s = Map::new();
        insert_opt(&mut s, "on_start", schedule_on(schedule_v.get("on_start")));
        insert_opt(&mut s, "on_stop", schedule_on(schedule_v.get("on_stop")));
        insert_opt(
            &mut s,
            "on_delete",
            schedule_on(schedule_v.get("on_delete")),
        );
        Value::Object(s)
    } else {
        return None;
    };
    Some(JobDetail {
        cpu: int(job, "cpu"),
        memory: int(job, "memory"),
        description: ptr(job, "description"),
        max_nb_restart: ptr(job, "max_nb_restart"),
        max_duration_seconds: ptr(job, "max_duration_seconds"),
        auto_preview: boolean(job, "auto_preview"),
        port: nullable(job, "port"),
        healthchecks: go_healthcheck(job.get("healthchecks").unwrap_or(&Value::Null)),
        schedule,
        auto_deploy: ptr(job, "auto_deploy").unwrap_or(Value::Null),
    })
}

fn sync_jobs(client: &Client, bp_env: &str, child_env: &str, opts: &SyncOptions) -> usize {
    let Ok(bp_jobs) = client.list::<Job>(&format!("/environment/{}/job", esc(bp_env))) else {
        return 0;
    };
    let Ok(child_jobs) = client.list::<Job>(&format!("/environment/{}/job", esc(child_env))) else {
        return 0;
    };
    let mut bp_map: HashMap<String, Value> = HashMap::new();
    for j in &bp_jobs {
        bp_map.insert(
            get_job_name(j),
            serde_json::to_value(j).unwrap_or(Value::Null),
        );
    }

    let mut synced = 0;
    for child_job in &child_jobs {
        let child_name = get_job_name(child_job);
        let Some(bp) = bp_map.get(&child_name) else {
            continue;
        };
        let child_id = get_job_id(child_job);
        let child = serde_json::to_value(child_job).unwrap_or(Value::Null);
        let bp_source = job_request_source(bp);
        let (Some(child_detail), Some(bp_source)) = (job_detail(&child), bp_source) else {
            continue;
        };

        let (mut cpu, mut memory) = (child_detail.cpu.clone(), child_detail.memory.clone());
        if opts.resources()
            && let Some(d) = job_detail(bp)
        {
            cpu = d.cpu;
            memory = d.memory;
        }
        let mut healthchecks = child_detail.healthchecks.clone();
        if opts.healthchecks()
            && let Some(d) = job_detail(bp)
        {
            healthchecks = d.healthchecks;
        }

        let mut req = Map::new();
        req.insert("name".into(), json!(child_name));
        insert_opt(&mut req, "description", child_detail.description);
        req.insert("cpu".into(), cpu);
        req.insert("memory".into(), memory);
        insert_opt(&mut req, "max_nb_restart", child_detail.max_nb_restart);
        insert_opt(
            &mut req,
            "max_duration_seconds",
            child_detail.max_duration_seconds,
        );
        req.insert("auto_preview".into(), child_detail.auto_preview);
        insert_opt(&mut req, "port", child_detail.port);
        req.insert("source".into(), bp_source);
        req.insert("healthchecks".into(), healthchecks);
        req.insert("schedule".into(), child_detail.schedule);
        req.insert("auto_deploy".into(), child_detail.auto_deploy);

        match client.put::<Value, _>(&format!("/job/{}", esc(&child_id)), &Value::Object(req)) {
            Err(e) => {
                qovery_log::println(format!("    WARNING: Failed to sync job {child_name}: {e}"))
            }
            Ok(_) => {
                qovery_log::println(format!("    Synced job: {}", blue(&child_name)));
                synced += 1;
            }
        }
    }
    synced
}

// ---- Helm sync --------------------------------------------------------------------------------------------

/// Go `rdeConvertHelmSource`.
fn helm_request_source(src: &Value) -> Option<Value> {
    if let Some(git) = src.get("git").filter(|v| v.is_object()) {
        let repo = git.get("git_repository").unwrap_or(&Value::Null);
        let mut g = Map::new();
        g.insert("url".into(), string(repo, "url"));
        insert_opt(&mut g, "branch", ptr(repo, "branch"));
        insert_opt(&mut g, "root_path", ptr(repo, "root_path"));
        return Some(json!({"git_repository": Value::Object(g)}));
    }
    if let Some(repo) = src.get("repository").filter(|v| v.is_object()) {
        return Some(json!({
            "helm_repository": {
                "repository": string(repo.get("repository").unwrap_or(&Value::Null), "id"),
                "chart_name": string(repo, "chart_name"),
                "chart_version": string(repo, "chart_version"),
            }
        }));
    }
    None
}

fn sync_helms(client: &Client, bp_env: &str, child_env: &str) -> usize {
    let Some(bp_helms) = list_values(client, bp_env, "helm") else {
        return 0;
    };
    let Some(child_helms) = list_values(client, child_env, "helm") else {
        return 0;
    };
    let bp_map = by_name(&bp_helms);

    let mut synced = 0;
    for child in &child_helms {
        let name = child["name"].as_str().unwrap_or_default();
        let Some(bp) = bp_map.get(name) else {
            continue;
        };
        let Some(bp_source) = helm_request_source(bp.get("source").unwrap_or(&Value::Null)) else {
            continue;
        };

        let vo = child.get("values_override").unwrap_or(&Value::Null);
        let mut values = Map::new();
        insert_opt(&mut values, "set", slice(vo, "set").map(Value::Array));
        insert_opt(
            &mut values,
            "set_string",
            slice(vo, "set_string").map(Value::Array),
        );
        insert_opt(
            &mut values,
            "set_json",
            slice(vo, "set_json").map(Value::Array),
        );

        let mut req = Map::new();
        req.insert("name".into(), json!(name));
        insert_opt(&mut req, "description", ptr(child, "description"));
        insert_opt(&mut req, "timeout_sec", ptr(child, "timeout_sec"));
        req.insert("auto_deploy".into(), boolean(child, "auto_deploy"));
        req.insert("source".into(), bp_source);
        req.insert(
            "arguments".into(),
            slice(child, "arguments")
                .map(Value::Array)
                .unwrap_or(Value::Null),
        );
        req.insert(
            "allow_cluster_wide_resources".into(),
            boolean(child, "allow_cluster_wide_resources"),
        );
        req.insert("values_override".into(), Value::Object(values));

        let id = child["id"].as_str().unwrap_or_default();
        match client.put::<Value, _>(&format!("/helm/{}", esc(id)), &Value::Object(req)) {
            Err(e) => qovery_log::println(format!("    WARNING: Failed to sync helm {name}: {e}")),
            Ok(_) => {
                qovery_log::println(format!("    Synced helm: {}", blue(name)));
                synced += 1;
            }
        }
    }
    synced
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cron_job_detail_and_source() {
        let job = json!({
            "id": "j1", "name": "n", "job_type": "CRON", "cpu": 100, "memory": 256, "auto_preview": true,
            "port": null, "healthchecks": {},
            "source": {"image": {"image_name": "img", "tag": "1", "registry": {"id": "r1"}}},
            "schedule": {"cronjob": {"scheduled_at": "* * * * *", "timezone": "UTC", "arguments": []}},
        });
        let d = job_detail(&job).unwrap();
        assert_eq!(
            d.schedule,
            json!({"cronjob": {"scheduled_at": "* * * * *", "timezone": "UTC", "arguments": []}})
        );
        assert_eq!(d.port, Some(Value::Null));
        assert_eq!(d.auto_deploy, Value::Null);
        assert_eq!(
            job_request_source(&job).unwrap(),
            json!({"image": {"image_name": "img", "tag": "1", "registry_id": "r1"}})
        );
    }

    #[test]
    fn helm_sources() {
        let git = json!({"git": {"git_repository": {"url": "u", "branch": "main", "root_path": null, "provider": "GITHUB"}}});
        assert_eq!(
            helm_request_source(&git).unwrap(),
            json!({"git_repository": {"url": "u", "branch": "main"}})
        );
        let repo = json!({"repository": {"chart_name": "c", "chart_version": "1", "repository": {"id": "hr"}}});
        assert_eq!(
            helm_request_source(&repo).unwrap(),
            json!({"helm_repository": {"repository": "hr", "chart_name": "c", "chart_version": "1"}})
        );
    }
}
