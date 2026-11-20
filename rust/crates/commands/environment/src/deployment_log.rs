//! Port of `pkg/deployment_log.go`: filtering and rendering of environment deployment log lines
//! (`qovery.EnvironmentLogs`, `GET /environment/{id}/logs`).

use chrono::{DateTime, FixedOffset};
use qovery_services::{go_zero_time, to_iso8601_time};
use serde::{Deserialize, Deserializer};
use serde_json::{Map, Value, json};
use std::collections::BTreeSet;

/// `PreCheckStep`: the value of `details.stage.step` carried by the pre-check log lines.
pub const PRE_CHECK_STEP: &str = "PreCheck";

/// `environmentWideTransmitters`: transmitter types emitting lines about the deployment as a whole.
const ENVIRONMENT_WIDE_TRANSMITTERS: [&str; 2] = ["Environment", "TaskManager"];

fn de_time<'de, D: Deserializer<'de>>(d: D) -> Result<DateTime<FixedOffset>, D::Error> {
    let s: Option<String> = Option::deserialize(d)?;
    Ok(s.and_then(|s| DateTime::parse_from_rfc3339(&s).ok())
        .unwrap_or_else(go_zero_time))
}

fn default_time() -> DateTime<FixedOffset> {
    go_zero_time()
}

/// `qovery.EnvironmentLogs`.
#[derive(Debug, Clone, Deserialize)]
pub struct EnvironmentLog {
    #[serde(default, rename = "type")]
    pub log_type: Option<String>,
    #[serde(default = "default_time", deserialize_with = "de_time")]
    pub timestamp: DateTime<FixedOffset>,
    #[serde(default)]
    pub details: Option<Details>,
    #[serde(default)]
    pub error: Option<LogError>,
    #[serde(default)]
    pub message: Option<LogMessage>,
}

impl Default for EnvironmentLog {
    fn default() -> Self {
        EnvironmentLog {
            log_type: None,
            timestamp: go_zero_time(),
            details: None,
            error: None,
            message: None,
        }
    }
}

/// `qovery.EnvironmentLogsDetails`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Details {
    #[serde(default)]
    pub transmitter: Option<Transmitter>,
    #[serde(default)]
    pub stage: Option<Stage>,
}

/// `qovery.EnvironmentLogsDetailsTransmitter`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Transmitter {
    #[serde(default)]
    pub id: Option<String>,
    #[serde(default)]
    pub name: Option<String>,
    #[serde(default, rename = "type")]
    pub transmitter_type: Option<String>,
}

/// `qovery.EnvironmentLogsDetailsStage`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Stage {
    #[serde(default)]
    pub step: Option<String>,
    #[serde(default)]
    pub id: Option<String>,
    #[serde(default)]
    pub name: Option<String>,
}

/// `qovery.EnvironmentLogsError`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct LogError {
    #[serde(default)]
    pub tag: Option<String>,
    #[serde(default)]
    pub user_log_message: Option<String>,
    #[serde(default)]
    pub link: Option<String>,
    #[serde(default)]
    pub hint_message: Option<String>,
}

/// `qovery.EnvironmentLogsMessage`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct LogMessage {
    #[serde(default)]
    pub safe_message: Option<String>,
}

fn s(v: &Option<String>) -> String {
    v.clone().unwrap_or_default()
}

impl EnvironmentLog {
    /// `l.Details.Stage.GetStep()`
    pub fn step(&self) -> String {
        self.details
            .as_ref()
            .and_then(|d| d.stage.as_ref())
            .map(|st| s(&st.step))
            .unwrap_or_default()
    }
    /// `l.Details.Stage.GetName()`
    pub fn stage_name(&self) -> String {
        self.details
            .as_ref()
            .and_then(|d| d.stage.as_ref())
            .map(|st| s(&st.name))
            .unwrap_or_default()
    }
    /// `l.Details.Transmitter.GetId()`
    pub fn transmitter_id(&self) -> String {
        self.details
            .as_ref()
            .and_then(|d| d.transmitter.as_ref())
            .map(|t| s(&t.id))
            .unwrap_or_default()
    }
    /// `l.Details.Transmitter.GetName()`
    pub fn transmitter_name(&self) -> String {
        self.details
            .as_ref()
            .and_then(|d| d.transmitter.as_ref())
            .map(|t| s(&t.name))
            .unwrap_or_default()
    }
    /// `l.Details.Transmitter.GetType()`
    pub fn transmitter_type(&self) -> String {
        self.details
            .as_ref()
            .and_then(|d| d.transmitter.as_ref())
            .map(|t| s(&t.transmitter_type))
            .unwrap_or_default()
    }
    /// `l.Message.Get().GetSafeMessage()`
    pub fn safe_message(&self) -> String {
        self.message
            .as_ref()
            .map(|m| s(&m.safe_message))
            .unwrap_or_default()
    }
}

/// `DeploymentLogFilter`: the zero value keeps every line.
#[derive(Debug, Clone, Default)]
pub struct DeploymentLogFilter {
    pub service_id: String,
    pub service_name: String,
    pub pre_check_only: bool,
    pub errors_only: bool,
}

impl DeploymentLogFilter {
    /// `IsServiceScoped`.
    pub fn is_service_scoped(&self) -> bool {
        !self.service_id.is_empty() || !self.service_name.is_empty()
    }
}

/// `FilterDeploymentLogs`: keep the lines matching `f`, sorted chronologically (stable).
pub fn filter_deployment_logs(
    logs: &[EnvironmentLog],
    f: &DeploymentLogFilter,
) -> Vec<EnvironmentLog> {
    let mut filtered: Vec<EnvironmentLog> = logs
        .iter()
        .filter(|l| keep_deployment_log(l, f))
        .cloned()
        .collect();
    filtered.sort_by_key(|l| l.timestamp);
    filtered
}

fn keep_deployment_log(l: &EnvironmentLog, f: &DeploymentLogFilter) -> bool {
    let is_pre_check = l.step() == PRE_CHECK_STEP;

    if f.pre_check_only && !is_pre_check {
        return false;
    }

    if f.is_service_scoped() {
        if is_pre_check {
            return false;
        }
        if !is_environment_wide(l) && !matches_service(l, f) {
            return false;
        }
    }

    if f.errors_only && !has_deployment_log_error(l) {
        return false;
    }

    true
}

fn has_deployment_log_error(l: &EnvironmentLog) -> bool {
    l.error.is_some()
}

fn is_environment_wide(l: &EnvironmentLog) -> bool {
    ENVIRONMENT_WIDE_TRANSMITTERS.contains(&l.transmitter_type().as_str())
}

fn matches_service(l: &EnvironmentLog, f: &DeploymentLogFilter) -> bool {
    (!f.service_id.is_empty() && l.transmitter_id() == f.service_id)
        || (!f.service_name.is_empty() && l.transmitter_name() == f.service_name)
}

/// `DeploymentLogServices`: distinct (sorted) names of the services that emitted at least one line.
pub fn deployment_log_services(logs: &[EnvironmentLog]) -> Vec<String> {
    let mut seen = BTreeSet::new();
    for l in logs {
        if is_environment_wide(l) || l.step() == PRE_CHECK_STEP {
            continue;
        }
        let name = l.transmitter_name();
        if !name.is_empty() {
            seen.insert(name);
        }
    }
    seen.into_iter().collect()
}

/// `HasServiceSpecificLines`.
pub fn has_service_specific_lines(logs: &[EnvironmentLog]) -> bool {
    logs.iter()
        .any(|l| !is_environment_wide(l) && l.step() != PRE_CHECK_STEP)
}

/// `FormatDeploymentLogLine`: `| 2026-09-06 14:03:12.418 | Build | Application/my-app | message`, followed by
/// the error / hint / link of a line carrying an error.
pub fn format_deployment_log_line(l: &EnvironmentLog) -> String {
    let mut sb = format!(
        "| {} | {} | {} | {}",
        l.timestamp.format("%Y-%m-%d %H:%M:%S%.3f"),
        or_dash(&l.step()),
        or_dash(&deployment_log_transmitter(l)),
        l.safe_message(),
    );

    if let Some(e) = &l.error {
        for (label, value) in [
            ("error", &e.user_log_message),
            ("hint", &e.hint_message),
            ("link", &e.link),
        ] {
            let value = s(value);
            if value.is_empty() {
                continue;
            }
            sb.push_str(&format!("\n    {label}: {value}"));
        }
    }

    sb
}

/// `DeploymentLogJSON`: projection of a line onto the field names of the MCP tool.
pub fn deployment_log_json(l: &EnvironmentLog) -> Value {
    let mut out = Map::new();
    out.insert("timestamp".into(), json!(to_iso8601_time(&l.timestamp)));
    out.insert("stage".into(), json!(l.stage_name()));
    out.insert("step".into(), json!(l.step()));
    out.insert("transmitter".into(), json!(l.transmitter_name()));
    out.insert("transmitter_type".into(), json!(l.transmitter_type()));
    out.insert("message".into(), json!(l.safe_message()));

    if let Some(e) = &l.error {
        out.insert(
            "error".into(),
            json!({
                "tag": s(&e.tag),
                "user_log_message": s(&e.user_log_message),
                "hint_message": s(&e.hint_message),
                "link": s(&e.link),
            }),
        );
    }

    Value::Object(out)
}

/// `deploymentLogTransmitter`: `Type/Name`, degrading to whichever half is present.
fn deployment_log_transmitter(l: &EnvironmentLog) -> String {
    let transmitter_type = l.transmitter_type();
    let name = l.transmitter_name();
    match (transmitter_type.is_empty(), name.is_empty()) {
        (false, false) => format!("{transmitter_type}/{name}"),
        (_, false) => name,
        _ => transmitter_type,
    }
}

fn or_dash(s: &str) -> String {
    if s.is_empty() {
        "-".to_string()
    } else {
        s.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::{TimeZone, Utc};

    #[derive(Default)]
    struct Opts {
        step: &'static str,
        stage_name: &'static str,
        transmitter_id: &'static str,
        transmitter_type: &'static str,
        transmitter_name: &'static str,
        message: &'static str,
        err_user_message: &'static str,
        err_hint: &'static str,
        err_link: &'static str,
        err_tag: &'static str,
    }

    fn some(s: &str) -> Option<String> {
        Some(s.to_string())
    }

    fn new_log(o: Opts) -> EnvironmentLog {
        let mut l = EnvironmentLog {
            log_type: some("debug"),
            timestamp: Utc
                .with_ymd_and_hms(2026, 9, 6, 14, 3, 12)
                .unwrap()
                .fixed_offset()
                + chrono::Duration::milliseconds(418),
            details: Some(Details {
                stage: Some(Stage {
                    step: some(o.step),
                    id: None,
                    name: some(o.stage_name),
                }),
                transmitter: Some(Transmitter {
                    id: some(o.transmitter_id),
                    transmitter_type: some(o.transmitter_type),
                    name: some(o.transmitter_name),
                }),
            }),
            error: None,
            message: Some(LogMessage {
                safe_message: some(o.message),
            }),
        };
        if !o.err_user_message.is_empty()
            || !o.err_hint.is_empty()
            || !o.err_link.is_empty()
            || !o.err_tag.is_empty()
        {
            l.error = Some(LogError {
                tag: some(o.err_tag),
                user_log_message: some(o.err_user_message),
                hint_message: some(o.err_hint),
                link: some(o.err_link),
            });
        }
        l
    }

    const SERVICE_UNDER_TEST: &str = "11111111-1111-1111-1111-111111111111";
    const OTHER_SERVICE: &str = "22222222-2222-2222-2222-222222222222";

    fn messages_of(logs: &[EnvironmentLog]) -> Vec<String> {
        logs.iter().map(|l| l.safe_message()).collect()
    }

    #[test]
    fn filter_deployment_logs_cases() {
        let pre_check = new_log(Opts {
            step: PRE_CHECK_STEP,
            transmitter_type: "Environment",
            message: "pre-check",
            ..Default::default()
        });
        let env_wide = new_log(Opts {
            step: "Deployed",
            transmitter_type: "Environment",
            message: "env level",
            ..Default::default()
        });
        let task_manager = new_log(Opts {
            step: "Queued",
            transmitter_type: "TaskManager",
            message: "queued",
            ..Default::default()
        });
        let mine = new_log(Opts {
            step: "Build",
            transmitter_type: "Application",
            transmitter_id: SERVICE_UNDER_TEST,
            transmitter_name: "my-app",
            message: "mine",
            ..Default::default()
        });
        let theirs = new_log(Opts {
            step: "Build",
            transmitter_type: "Application",
            transmitter_id: OTHER_SERVICE,
            transmitter_name: "their-app",
            message: "theirs",
            ..Default::default()
        });
        let failed = new_log(Opts {
            step: "Deployed",
            transmitter_type: "Application",
            transmitter_id: SERVICE_UNDER_TEST,
            transmitter_name: "my-app",
            message: "boom",
            err_user_message: "liveness probe failed",
            ..Default::default()
        });
        let all = vec![pre_check, env_wide, task_manager, mine, theirs, failed];

        let f = |service_id: &str, service_name: &str, pre_check_only: bool, errors_only: bool| {
            DeploymentLogFilter {
                service_id: service_id.into(),
                service_name: service_name.into(),
                pre_check_only,
                errors_only,
            }
        };
        let cases: Vec<(&str, DeploymentLogFilter, Vec<&str>)> = vec![
            (
                "zero value keeps everything",
                f("", "", false, false),
                vec!["pre-check", "env level", "queued", "mine", "theirs", "boom"],
            ),
            (
                "service scope",
                f(SERVICE_UNDER_TEST, "", false, false),
                vec!["env level", "queued", "mine", "boom"],
            ),
            ("pre-check only", f("", "", true, false), vec!["pre-check"]),
            ("errors only", f("", "", false, true), vec!["boom"]),
            (
                "errors only + service scope",
                f(SERVICE_UNDER_TEST, "", false, true),
                vec!["boom"],
            ),
            (
                "unknown service",
                f("33333333-3333-3333-3333-333333333333", "", false, false),
                vec!["env level", "queued"],
            ),
            (
                "service scope by name",
                f("", "my-app", false, false),
                vec!["env level", "queued", "mine", "boom"],
            ),
            (
                "unknown service name",
                f("", "does-not-exist", false, false),
                vec!["env level", "queued"],
            ),
        ];
        for (name, filter, want) in cases {
            assert_eq!(
                messages_of(&filter_deployment_logs(&all, &filter)),
                want,
                "{name}"
            );
        }
    }

    #[test]
    fn filter_handles_missing_details() {
        let bare = EnvironmentLog {
            log_type: some("debug"),
            timestamp: Utc::now().fixed_offset(),
            ..Default::default()
        };
        assert_eq!(
            filter_deployment_logs(std::slice::from_ref(&bare), &DeploymentLogFilter::default())
                .len(),
            1
        );
        let scoped = DeploymentLogFilter {
            service_id: SERVICE_UNDER_TEST.into(),
            ..Default::default()
        };
        assert_eq!(
            filter_deployment_logs(std::slice::from_ref(&bare), &scoped).len(),
            0
        );
        assert!(format_deployment_log_line(&bare).contains("| - | - |"));
    }

    #[test]
    fn format_line() {
        let plain = new_log(Opts {
            step: "Build",
            transmitter_type: "Application",
            transmitter_name: "my-app",
            message: "building image",
            ..Default::default()
        });
        assert_eq!(
            format_deployment_log_line(&plain),
            "| 2026-09-06 14:03:12.418 | Build | Application/my-app | building image"
        );
    }

    #[test]
    fn format_line_with_error() {
        let failed = new_log(Opts {
            step: "Deployed",
            transmitter_type: "Application",
            transmitter_name: "my-app",
            message: "deployment failed",
            err_user_message: "liveness probe failed",
            err_hint: "check the probe path",
            err_link: "https://hub.qovery.com/docs/probes",
            ..Default::default()
        });
        let got = format_deployment_log_line(&failed);
        let lines: Vec<&str> = got.split('\n').collect();
        assert_eq!(
            lines,
            vec![
                "| 2026-09-06 14:03:12.418 | Deployed | Application/my-app | deployment failed",
                "    error: liveness probe failed",
                "    hint: check the probe path",
                "    link: https://hub.qovery.com/docs/probes",
            ]
        );
    }

    #[test]
    fn format_line_omits_empty_error_fields() {
        let failed = new_log(Opts {
            step: "Build",
            transmitter_type: "Application",
            transmitter_name: "my-app",
            message: "build failed",
            err_user_message: "Dockerfile not found",
            ..Default::default()
        });
        let got = format_deployment_log_line(&failed);
        assert_eq!(got.split('\n').count(), 2);
        assert!(got.ends_with("    error: Dockerfile not found"));
    }

    #[test]
    fn json_projection() {
        let l = new_log(Opts {
            step: "Deployed",
            stage_name: "backend",
            transmitter_type: "Application",
            transmitter_name: "my-app",
            transmitter_id: SERVICE_UNDER_TEST,
            message: "deployment failed",
            err_tag: "K8S_CANNOT_APPLY_CONFIG",
            err_user_message: "liveness probe failed",
            err_hint: "check the probe path",
            err_link: "https://hub.qovery.com/docs/probes",
        });
        let got = deployment_log_json(&l);
        assert_eq!(got["stage"], "backend");
        assert_eq!(got["step"], "Deployed");
        assert_eq!(got["transmitter"], "my-app");
        assert_eq!(got["transmitter_type"], "Application");
        assert_eq!(got["message"], "deployment failed");
        assert_eq!(got["timestamp"], "2026-09-06T14:03:12.418Z");
        assert_eq!(got["error"]["tag"], "K8S_CANNOT_APPLY_CONFIG");
        assert_eq!(got["error"]["user_log_message"], "liveness probe failed");
        assert_eq!(got["error"]["hint_message"], "check the probe path");
        assert_eq!(got["error"]["link"], "https://hub.qovery.com/docs/probes");
    }

    #[test]
    fn json_omits_error_when_absent() {
        let l = new_log(Opts {
            step: "Build",
            message: "building image",
            ..Default::default()
        });
        assert!(deployment_log_json(&l).get("error").is_none());
    }

    #[test]
    fn is_service_scoped() {
        assert!(!DeploymentLogFilter::default().is_service_scoped());
        assert!(
            DeploymentLogFilter {
                service_id: SERVICE_UNDER_TEST.into(),
                ..Default::default()
            }
            .is_service_scoped()
        );
        assert!(
            DeploymentLogFilter {
                service_name: "my-app".into(),
                ..Default::default()
            }
            .is_service_scoped()
        );
        assert!(
            !DeploymentLogFilter {
                errors_only: true,
                ..Default::default()
            }
            .is_service_scoped()
        );
    }

    #[test]
    fn services_list() {
        let logs = vec![
            new_log(Opts {
                step: PRE_CHECK_STEP,
                transmitter_type: "Environment",
                transmitter_name: "Development",
                ..Default::default()
            }),
            new_log(Opts {
                step: "Deployed",
                transmitter_type: "Environment",
                transmitter_name: "Development",
                ..Default::default()
            }),
            new_log(Opts {
                step: "Queued",
                transmitter_type: "TaskManager",
                transmitter_name: "tm",
                ..Default::default()
            }),
            new_log(Opts {
                step: "Build",
                transmitter_type: "Terraform",
                transmitter_name: "S3",
                ..Default::default()
            }),
            new_log(Opts {
                step: "Build",
                transmitter_type: "Application",
                transmitter_name: "my-app",
                ..Default::default()
            }),
            new_log(Opts {
                step: "Deployed",
                transmitter_type: "Application",
                transmitter_name: "my-app",
                ..Default::default()
            }),
        ];
        assert_eq!(deployment_log_services(&logs), vec!["S3", "my-app"]);
    }

    #[test]
    fn service_specific_lines() {
        let mut env_only = vec![
            new_log(Opts {
                step: "Deployed",
                transmitter_type: "Environment",
                ..Default::default()
            }),
            new_log(Opts {
                step: "Queued",
                transmitter_type: "TaskManager",
                ..Default::default()
            }),
        ];
        assert!(!has_service_specific_lines(&env_only));
        env_only.push(new_log(Opts {
            step: "Build",
            transmitter_type: "Application",
            transmitter_name: "my-app",
            ..Default::default()
        }));
        assert!(has_service_specific_lines(&env_only));

        let pre_check_only = vec![new_log(Opts {
            step: PRE_CHECK_STEP,
            transmitter_type: "Application",
            transmitter_name: "my-app",
            ..Default::default()
        })];
        assert!(!has_service_specific_lines(&pre_check_only));
        assert!(deployment_log_services(&pre_check_only).is_empty());
    }

    #[test]
    fn services_hint_values_are_accepted_by_the_filter() {
        let logs = vec![new_log(Opts {
            step: "Build",
            transmitter_type: "Terraform",
            transmitter_name: "S3",
            ..Default::default()
        })];
        for name in deployment_log_services(&logs) {
            let filtered = filter_deployment_logs(
                &logs,
                &DeploymentLogFilter {
                    service_name: name.clone(),
                    ..Default::default()
                },
            );
            assert!(
                has_service_specific_lines(&filtered),
                "hint offers {name} but --service matches nothing"
            );
        }
    }

    #[test]
    fn sorts_chronologically_and_stably() {
        let at = |sec: u32, msg: &'static str| {
            let mut l = new_log(Opts {
                step: "Build",
                message: msg,
                ..Default::default()
            });
            l.timestamp = Utc
                .with_ymd_and_hms(2026, 9, 6, 14, 0, sec)
                .unwrap()
                .fixed_offset();
            l
        };
        let got = filter_deployment_logs(
            &[at(30, "c"), at(10, "a"), at(20, "b")],
            &DeploymentLogFilter::default(),
        );
        assert_eq!(messages_of(&got), vec!["a", "b", "c"]);

        let got = filter_deployment_logs(
            &[at(0, "#1"), at(0, "#2"), at(0, "#3")],
            &DeploymentLogFilter::default(),
        );
        assert_eq!(messages_of(&got), vec!["#1", "#2", "#3"]);
    }

    #[test]
    fn decodes_api_payload() {
        let v = json!({"type": "info", "timestamp": "2026-09-06T14:03:12.418+02:00",
            "details": {"stage": {"step": "Build", "name": null}, "transmitter": null},
            "error": null, "message": {"safe_message": "hi"}});
        let l: EnvironmentLog = serde_json::from_value(v).unwrap();
        assert!(l.error.is_none());
        assert_eq!(
            format_deployment_log_line(&l),
            "| 2026-09-06 14:03:12.418 | Build | - | hi"
        );
        assert_eq!(
            deployment_log_json(&l)["timestamp"],
            "2026-09-06T14:03:12.418Z"
        );
    }
}
