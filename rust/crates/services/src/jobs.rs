//! Job and helm source helpers of `utils/qovery.go` (`GetJobDocker`, `GetJobImage`, `GetJobId`, `GetJobName`,
//! `GetGitSource`, `GetHelmRepository`) and `ToJobRequest` with a `qovery.JobRequest` equivalent that
//! serializes like the Go client (unset optional fields omitted, nullable fields kept as `null` when set).
//!
//! Nullable Go fields (`NullableString`, `NullableInt32`, ...) are `Option<Option<T>>`: `None` = unset
//! (omitted), `Some(None)` = explicit `null`, `Some(Some(v))` = value.

use qovery_api::models::{Helm, Job};
use serde::Serialize;
use serde_json::{Map, Value};

fn is_job(job: &Job) -> bool {
    job.is_cron() || job.is_lifecycle()
}

/// oneOf `{key: {...}}` source: the inner object when `key` is set and `other` is not.
fn one_of<'a>(source: Option<&'a Value>, key: &str, other: &str) -> Option<&'a Value> {
    let s = source?;
    let inner = s.get(key).filter(|v| v.is_object())?;
    if s.get(other).is_some_and(|v| !v.is_null()) {
        return None;
    }
    Some(inner)
}

/// Port of `utils.GetJobDocker(job)`: the `source.docker` object (`JobSourceDockerResponse`) of a docker job.
pub fn get_job_docker(job: &Job) -> Option<&Value> {
    if !is_job(job) {
        return None;
    }
    one_of(job.source.as_ref(), "docker", "image")
}

/// Port of `utils.GetJobImage(job)`: the `source.image` object (`ContainerSource`: `image_name`, `tag`,
/// `registry`) of an image job.
pub fn get_job_image(job: &Job) -> Option<&Value> {
    if !is_job(job) {
        return None;
    }
    one_of(job.source.as_ref(), "image", "docker")
}

/// Port of `utils.GetJobId(job)`: the id of a cron or lifecycle job, `""` otherwise.
pub fn get_job_id(job: &Job) -> String {
    if is_job(job) {
        job.id.clone()
    } else {
        String::new()
    }
}

/// Port of `utils.GetJobName(job)`: the name of a cron or lifecycle job, `""` otherwise.
pub fn get_job_name(job: &Job) -> String {
    if is_job(job) {
        job.name.clone()
    } else {
        String::new()
    }
}

/// Port of `utils.GetGitSource(helm)`: the `source.git` object (`HelmSourceGitResponse`) of a git helm.
pub fn get_git_source(helm: &Helm) -> Option<&Value> {
    one_of(helm.source.as_ref(), "git", "repository")
}

/// Port of `utils.GetHelmRepository(helm)`: the `source.repository` object (`HelmSourceRepositoryResponse`).
pub fn get_helm_repository(helm: &Helm) -> Option<&Value> {
    one_of(helm.source.as_ref(), "repository", "git")
}

/// `qovery.JobRequest`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct JobRequest {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cpu: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gpu: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ephemeral_storage_in_gib: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_nb_restart: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_duration_seconds: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub auto_preview: Option<bool>,
    /// NullableInt32
    #[serde(skip_serializing_if = "Option::is_none")]
    pub port: Option<Option<i64>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source: Option<JobRequestSource>,
    /// NullableCpuArchitectureEnum
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cpu_architecture: Option<Option<String>>,
    /// `Healthcheck` (always serialized), normalized like a Go `Healthcheck` round trip.
    pub healthchecks: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub schedule: Option<JobRequestSchedule>,
    /// NullableBool
    #[serde(skip_serializing_if = "Option::is_none")]
    pub auto_deploy: Option<Option<bool>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub annotations_groups: Option<Vec<Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels_groups: Option<Vec<Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub icon_uri: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub build_settings: Option<Value>,
}

impl JobRequest {
    /// The JSON body sent by the Go client.
    pub fn to_value(&self) -> Value {
        serde_json::to_value(self).unwrap_or(Value::Null)
    }
}

/// `qovery.JobRequestAllOfSource` (`image` / `docker` are nullable: `Some(None)` sends `null`).
#[derive(Debug, Clone, Default, Serialize)]
pub struct JobRequestSource {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image: Option<Option<JobRequestSourceImage>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub docker: Option<Option<JobRequestSourceDocker>>,
}

/// `qovery.JobRequestAllOfSourceImage`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct JobRequestSourceImage {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tag: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub registry_id: Option<String>,
}

/// `qovery.JobRequestAllOfSourceDocker`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct JobRequestSourceDocker {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_repository: Option<ApplicationGitRepositoryRequest>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub dockerfile_path: Option<Option<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub dockerfile_raw: Option<Option<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub docker_target_build_stage: Option<Option<String>>,
}

/// `qovery.ApplicationGitRepositoryRequest`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct ApplicationGitRepositoryRequest {
    pub url: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub branch: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub root_path: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub git_token_id: Option<Option<String>>,
    pub provider: String,
}

/// `qovery.JobRequestAllOfSchedule` (`on_start` / `on_stop` / `on_delete` are `JobRequestAllOfScheduleOnStart`
/// objects: `arguments`, `entrypoint`).
#[derive(Debug, Clone, Default, Serialize)]
pub struct JobRequestSchedule {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub on_start: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub on_stop: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub on_delete: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cronjob: Option<JobRequestScheduleCronjob>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lifecycle_type: Option<String>,
}

/// `qovery.JobRequestAllOfScheduleCronjob`.
#[derive(Debug, Clone, Default, Serialize)]
pub struct JobRequestScheduleCronjob {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub arguments: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub entrypoint: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timezone: Option<String>,
    pub scheduled_at: String,
}

// ---- Go model round-trip helpers --------------------------------------------------------------------------

/// Go `*string` field: `None` when missing or null.
fn ptr_str(o: &Value, key: &str) -> Option<String> {
    o.get(key).and_then(|v| v.as_str()).map(|s| s.to_string())
}

/// Go `*int32` field.
fn ptr_i64(o: &Value, key: &str) -> Option<i64> {
    o.get(key).and_then(|v| v.as_i64())
}

/// Go `NullableString` field: set when the key is present.
fn nullable_str(o: &Value, key: &str) -> Option<Option<String>> {
    o.get(key).map(|v| v.as_str().map(|s| s.to_string()))
}

/// Go `[]string` field (`omitempty` + `IsNil`): `None` when missing or null, `Some([])` for `[]`.
fn string_slice(o: &Value, key: &str) -> Option<Vec<String>> {
    o.get(key).and_then(|v| v.as_array()).map(|a| {
        a.iter()
            .map(|x| x.as_str().unwrap_or_default().to_string())
            .collect()
    })
}

/// Round trip of a Go model with `AdditionalProperties`: unknown keys kept as-is, `ptr_keys` (pointer /
/// slice / map fields) dropped when null, `nullable_keys` kept (including `null`), `nested` objects
/// normalized recursively when not null.
/// A nested model field and its round-trip function.
type Nested<'a> = (&'a str, fn(&Value) -> Value);

fn round_trip(v: &Value, ptr_keys: &[&str], nested: &[Nested<'_>]) -> Value {
    let Some(obj) = v.as_object() else {
        return v.clone();
    };
    let mut out = Map::new();
    for (k, val) in obj {
        if let Some((_, f)) = nested.iter().find(|(n, _)| n == k) {
            if !val.is_null() {
                out.insert(k.clone(), f(val));
            } else if !ptr_keys.contains(&k.as_str()) {
                out.insert(k.clone(), Value::Null);
            }
            continue;
        }
        if val.is_null() && ptr_keys.contains(&k.as_str()) {
            continue;
        }
        out.insert(k.clone(), val.clone());
    }
    Value::Object(out)
}

fn probe_type_tcp(v: &Value) -> Value {
    round_trip(v, &["port"], &[])
}
fn probe_type_http(v: &Value) -> Value {
    round_trip(v, &["path", "scheme", "port"], &[])
}
fn probe_type_exec(v: &Value) -> Value {
    round_trip(v, &["command"], &[])
}
fn probe_type_grpc(v: &Value) -> Value {
    round_trip(v, &["port"], &[])
}
fn probe_type(v: &Value) -> Value {
    // tcp / http / exec / grpc are nullable
    round_trip(
        v,
        &[],
        &[
            ("tcp", probe_type_tcp),
            ("http", probe_type_http),
            ("exec", probe_type_exec),
            ("grpc", probe_type_grpc),
        ],
    )
}
fn probe(v: &Value) -> Value {
    round_trip(
        v,
        &[
            "type",
            "initial_delay_seconds",
            "period_seconds",
            "timeout_seconds",
            "success_threshold",
            "failure_threshold",
        ],
        &[("type", probe_type)],
    )
}

/// A `qovery.Healthcheck` decoded then re-encoded by the Go client (`readiness_probe` / `liveness_probe`
/// are nullable probes).
pub fn go_healthcheck(v: &Value) -> Value {
    if !v.is_object() {
        return Value::Object(Map::new());
    }
    round_trip(
        v,
        &[],
        &[("readiness_probe", probe), ("liveness_probe", probe)],
    )
}

/// A `qovery.JobRequestAllOfScheduleOnStart` round trip (`arguments`, `entrypoint` omitted when null);
/// `None` for a nil pointer.
fn go_schedule_on(v: Option<&Value>) -> Option<Value> {
    let v = v.filter(|v| !v.is_null())?;
    Some(round_trip(v, &["arguments", "entrypoint"], &[]))
}

/// Port of `utils.ToJobRequest(job)`: the `JobRequest` to edit a job, built from its current values.
///
/// Like Go, both `source.image` and `source.docker` are set (the one not matching the job source is an
/// empty object): callers set the unused one to `Some(None)` (`null`), as the Go commands do with `Set(nil)`.
pub fn to_job_request(job: &Job) -> JobRequest {
    let v = serde_json::to_value(job).unwrap_or(Value::Null);
    let empty = Value::Object(Map::new());

    let source_image = match get_job_image(job) {
        Some(image) => JobRequestSourceImage {
            image_name: Some(ptr_str(image, "image_name").unwrap_or_default()),
            tag: Some(ptr_str(image, "tag").unwrap_or_default()),
            registry_id: Some(
                image
                    .get("registry")
                    .map(|r| ptr_str(r, "id").unwrap_or_default())
                    .unwrap_or_default(),
            ),
        },
        None => JobRequestSourceImage::default(),
    };

    let source_docker = match get_job_docker(job) {
        Some(docker) => {
            let git = docker
                .get("git_repository")
                .filter(|g| g.is_object())
                .unwrap_or(&empty);
            JobRequestSourceDocker {
                git_repository: Some(ApplicationGitRepositoryRequest {
                    url: ptr_str(git, "url").unwrap_or_default(),
                    branch: ptr_str(git, "branch"),
                    root_path: ptr_str(git, "root_path"),
                    git_token_id: nullable_str(git, "git_token_id"),
                    provider: ptr_str(git, "provider").unwrap_or_default(),
                }),
                dockerfile_path: nullable_str(docker, "dockerfile_path"),
                dockerfile_raw: None,
                docker_target_build_stage: None,
            }
        }
        None => JobRequestSourceDocker::default(),
    };

    let source = JobRequestSource {
        image: Some(Some(source_image)),
        docker: Some(Some(source_docker)),
    };
    let schedule_v = v
        .get("schedule")
        .filter(|s| s.is_object())
        .unwrap_or(&empty);

    let schedule = if job.is_lifecycle() {
        JobRequestSchedule {
            on_start: go_schedule_on(schedule_v.get("on_start")),
            on_stop: go_schedule_on(schedule_v.get("on_stop")),
            on_delete: go_schedule_on(schedule_v.get("on_delete")),
            lifecycle_type: ptr_str(schedule_v, "lifecycle_type"),
            cronjob: None,
        }
    } else {
        let cron = schedule_v
            .get("cronjob")
            .filter(|c| c.is_object())
            .unwrap_or(&empty);
        JobRequestSchedule {
            cronjob: Some(JobRequestScheduleCronjob {
                arguments: string_slice(cron, "arguments"),
                entrypoint: ptr_str(cron, "entrypoint"),
                timezone: None,
                scheduled_at: ptr_str(cron, "scheduled_at").unwrap_or_default(),
            }),
            ..Default::default()
        }
    };

    JobRequest {
        name: job.name.clone(),
        description: ptr_str(&v, "description"),
        cpu: Some(ptr_i64(&v, "cpu").unwrap_or(0)),
        memory: Some(ptr_i64(&v, "memory").unwrap_or(0)),
        max_nb_restart: ptr_i64(&v, "max_nb_restart"),
        max_duration_seconds: ptr_i64(&v, "max_duration_seconds"),
        auto_preview: Some(
            v.get("auto_preview")
                .and_then(|b| b.as_bool())
                .unwrap_or(false),
        ),
        port: v.get("port").map(|p| p.as_i64()),
        source: Some(source),
        healthchecks: go_healthcheck(v.get("healthchecks").unwrap_or(&Value::Null)),
        schedule: Some(schedule),
        auto_deploy: Some(v.get("auto_deploy").and_then(|b| b.as_bool())),
        ..Default::default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn job(v: Value) -> Job {
        serde_json::from_value(v).unwrap()
    }

    #[test]
    fn job_sources() {
        let docker = job(
            json!({"id": "j1", "name": "n", "job_type": "CRON", "source": {"docker": {"dockerfile_path": "Dockerfile"}}}),
        );
        assert!(get_job_docker(&docker).is_some());
        assert!(get_job_image(&docker).is_none());
        assert_eq!(get_job_id(&docker), "j1");
        let image = job(
            json!({"id": "j2", "name": "n", "job_type": "LIFECYCLE", "source": {"image": {"tag": "1"}}}),
        );
        assert!(get_job_docker(&image).is_none());
        assert_eq!(get_job_image(&image).unwrap()["tag"], "1");
        let other = job(
            json!({"id": "j3", "name": "n", "job_type": "OTHER", "source": {"image": {"tag": "1"}}}),
        );
        assert!(get_job_image(&other).is_none());
        assert_eq!(get_job_id(&other), "");
        assert_eq!(get_job_name(&other), "");
    }

    #[test]
    fn cron_image_job_request() {
        let j = job(json!({
            "id": "j1", "name": "my-cron", "job_type": "CRON", "cpu": 500, "memory": 512, "auto_preview": false,
            "max_nb_restart": null, "max_duration_seconds": 300, "port": null, "description": null,
            "source": {"image": {"image_name": "nginx", "tag": "1.0", "registry": {"id": "reg-1", "name": "r"}}},
            "healthchecks": {"readiness_probe": null, "liveness_probe": {"type": {"tcp": {"port": 80, "host": null}, "http": null}, "initial_delay_seconds": 30, "period_seconds": null}},
            "schedule": {"cronjob": {"scheduled_at": "* * * * *", "timezone": "Etc/UTC", "arguments": [], "entrypoint": null}},
        }));
        let mut req = to_job_request(&j);
        req.source.as_mut().unwrap().docker = Some(None);
        assert_eq!(
            req.to_value(),
            json!({
                "name": "my-cron", "cpu": 500, "memory": 512, "auto_preview": false, "max_duration_seconds": 300, "port": null,
                "source": {"image": {"image_name": "nginx", "tag": "1.0", "registry_id": "reg-1"}, "docker": null},
                "healthchecks": {"readiness_probe": null, "liveness_probe": {"type": {"tcp": {"port": 80, "host": null}, "http": null}, "initial_delay_seconds": 30}},
                "schedule": {"cronjob": {"scheduled_at": "* * * * *", "arguments": []}},
                "auto_deploy": null,
            })
        );
    }

    #[test]
    fn lifecycle_docker_job_request() {
        let j = job(json!({
            "id": "j1", "name": "my-lifecycle", "job_type": "LIFECYCLE", "cpu": 250, "memory": 256, "auto_preview": true,
            "auto_deploy": true, "healthchecks": {},
            "source": {"docker": {"dockerfile_path": "Dockerfile", "dockerfile_raw": null,
                "git_repository": {"url": "https://github.com/a/b.git", "branch": "main", "root_path": "/", "provider": "GITHUB", "name": "a/b", "owner": "a", "git_token_id": null}}},
            "schedule": {"on_start": {"arguments": ["a"], "entrypoint": null}, "on_stop": null, "lifecycle_type": "GENERIC"},
        }));
        let req = to_job_request(&j);
        assert_eq!(
            req.to_value(),
            json!({
                "name": "my-lifecycle", "cpu": 250, "memory": 256, "auto_preview": true, "auto_deploy": true, "healthchecks": {},
                "source": {"image": {}, "docker": {"dockerfile_path": "Dockerfile",
                    "git_repository": {"url": "https://github.com/a/b.git", "branch": "main", "root_path": "/", "provider": "GITHUB", "git_token_id": null}}},
                "schedule": {"on_start": {"arguments": ["a"]}, "lifecycle_type": "GENERIC"},
            })
        );
    }
}
