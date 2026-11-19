//! Port of `utils/posthog.go`: PostHog usage events. Disabled with `QOVERY_TELEMETRY=false`.

use serde_json::{Map, Value, json};

pub const DEFAULT_EVENT_NAME: &str = "cli-command-execution";
pub const END_OF_EXECUTION_EVENT_NAME: &str = "cli-command-execution-end";
pub const END_OF_EXECUTION_ERROR_EVENT_NAME: &str = "cli-command-execution-error";
pub const PHASE_FINISHED_EVENT_NAME: &str = "cli-command-phase-finished";

const API_KEY: &str = "phc_IgdG1K2GveDUte1gJ6hlwNbFHCv9nViWETUyLMU7ciq";
const ENDPOINT: &str = "https://e.qovery.com";

/// `utils.CommandExecutionProperties`.
#[derive(Debug, Clone, Default)]
pub struct CommandExecutionProperties {
    pub attempt_id: String,
    pub workflow_type: String,
    pub implementation: String,
    pub cluster_id: String,
    pub phase: String,
    pub result: String,
    pub duration_millis: i64,
    pub error_code: String,
    pub safe_error_message: String,
}

/// Identity of the command an event is about.
pub struct CommandInfo {
    /// `qovery application list`
    pub path: String,
    /// names of the flags set on the command line
    pub changed_flags: Vec<String>,
}

impl CommandInfo {
    pub fn from_ctx(ctx: &qovery_cobra::Ctx) -> Self {
        CommandInfo {
            path: ctx.command_path(),
            changed_flags: ctx
                .all_flags()
                .into_iter()
                .filter(|(_, _, c)| *c)
                .map(|(n, _, _)| n)
                .collect(),
        }
    }
    pub fn root() -> Self {
        CommandInfo {
            path: "qovery".to_string(),
            changed_flags: Vec::new(),
        }
    }
}

/// `utils.Capture(cmd)`.
pub fn capture(ctx: &qovery_cobra::Ctx) {
    capture_with_event_and_properties(&CommandInfo::from_ctx(ctx), DEFAULT_EVENT_NAME, Map::new());
}

/// `utils.CaptureWithEvent(cmd, event)`.
pub fn capture_with_event(ctx: &qovery_cobra::Ctx, event: &str) {
    capture_with_event_and_properties(&CommandInfo::from_ctx(ctx), event, Map::new());
}

/// `utils.CaptureError(cmd, stdout, stderr)`.
pub fn capture_error(ctx: &qovery_cobra::Ctx, stdout: &str, stderr: &str) {
    let mut p = Map::new();
    p.insert("stdout".into(), json!(stdout));
    p.insert("stderr".into(), json!(stderr));
    capture_with_event_and_properties(
        &CommandInfo::from_ctx(ctx),
        END_OF_EXECUTION_ERROR_EVENT_NAME,
        p,
    );
}

/// `utils.CaptureCommandExecution(cmd, event, execution)`.
pub fn capture_command_execution(
    ctx: &qovery_cobra::Ctx,
    event: &str,
    e: &CommandExecutionProperties,
) {
    let mut p = Map::new();
    p.insert("attempt_id".into(), json!(e.attempt_id));
    p.insert("workflow_type".into(), json!(e.workflow_type));
    p.insert("implementation".into(), json!(e.implementation));
    if !e.attempt_id.is_empty() {
        p.insert("$session_id".into(), json!(e.attempt_id));
    }
    if !e.result.is_empty() {
        p.insert("result".into(), json!(e.result));
    }
    if !e.cluster_id.is_empty() {
        p.insert("cluster_id".into(), json!(e.cluster_id));
    }
    if !e.phase.is_empty() {
        p.insert("phase".into(), json!(e.phase));
    }
    if e.duration_millis > 0 {
        p.insert("duration_ms".into(), json!(e.duration_millis));
    }
    if !e.error_code.is_empty() {
        p.insert("error_code".into(), json!(e.error_code));
    }
    if !e.safe_error_message.is_empty() {
        p.insert("safe_error_message".into(), json!(e.safe_error_message));
    }
    capture_with_event_and_properties(&CommandInfo::from_ctx(ctx), event, p);
}

use crate::gosys::{go_arch, go_os};

/// `utils.CaptureWithEventAndProperties(cmd, event, properties)`.
pub fn capture_with_event_and_properties(
    cmd: &CommandInfo,
    event: &str,
    mut properties: Map<String, Value>,
) {
    if std::env::var("QOVERY_TELEMETRY").is_ok_and(|v| v.eq_ignore_ascii_case("false")) {
        return;
    }
    let Ok(ctx) = crate::context::get_current_context() else {
        return;
    };
    // posthog-go `Capture.Validate()`: `Enqueue` rejects an empty `DistinctId` (no user in the context,
    // e.g. a `qov_` static token session) and nothing is sent.
    if ctx.user.is_empty() || event.is_empty() {
        return;
    }
    let token_type = if ctx.access_token.starts_with("qov_") {
        "static"
    } else {
        "jwt"
    };
    for (k, v) in [
        ("organization", ctx.organization_name.as_str()),
        ("organization_id", ctx.organization_id.as_str()),
        ("project", ctx.project_name.as_str()),
        ("project_id", ctx.project_id.as_str()),
        ("environment", ctx.environment_name.as_str()),
        ("environment_id", ctx.environment_id.as_str()),
        ("service", ctx.service_name.as_str()),
        ("service_id", ctx.service_id.as_str()),
        ("token_type", token_type),
        ("os", go_os()),
        ("arch", go_arch()),
        ("cli_version", crate::version::VERSION),
        ("command", cmd.path.as_str()),
    ] {
        properties.insert(k.to_string(), json!(v));
    }
    properties.insert("flags".into(), json!(cmd.changed_flags.join(" ")));
    properties.insert("$lib".into(), json!("posthog-go"));
    let body = json!({
        "api_key": API_KEY,
        "batch": [{
            "type": "capture",
            "uuid": uuid::Uuid::new_v4().to_string(),
            "distinct_id": ctx.user,
            "event": event,
            "timestamp": chrono::Utc::now().to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
            "properties": properties,
        }],
    });
    let Ok(client) = reqwest::blocking::Client::builder()
        .timeout(std::time::Duration::from_secs(5))
        .build()
    else {
        return;
    };
    let _ = client.post(format!("{ENDPOINT}/batch/")).json(&body).send();
}
