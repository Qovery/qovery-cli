//! `environment deployment list|logs|explain` (`cmd/environment_deployment*.go`).

use crate::deployment_log::{
    DeploymentLogFilter, EnvironmentLog, deployment_log_json, deployment_log_services,
    filter_deployment_logs, format_deployment_log_line, has_service_specific_lines,
};
use crate::treeprint;
use crate::{fail, ope_flags, run_group};
use chrono::{DateTime, FixedOffset};
use qovery_api::models::Environment;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::resources::ids_from_flags;
use qovery_core::{check_error, client, exit, status, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::{println_error_to_stderr, println_info_to_stderr};
use qovery_services::{get_duration, go_zero_time, parse_api_time, path_escape, to_iso8601_time};
use serde_json::{Value, json};

pub(crate) fn command() -> Command {
    Command::new("deployment")
        .short("Manage environment deployments")
        .run(run_group)
        .subcommand(explain())
        .subcommand(list())
        .subcommand(logs())
}

// ---------------------------------------------------------------------------------------------------------------
// list

fn list() -> Command {
    Command::new("list")
        .short("List environment deployments")
        .flags(ope_flags())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, environment_id) = check_error(ids_from_flags(&client, ctx));

    let deployments: Vec<Value> = match client.list(&format!(
        "/environment/{}/deploymentHistory",
        path_escape(&environment_id)
    )) {
        Ok(d) => d,
        Err(e) => fail(e),
    };

    if ctx.bool("json") {
        qovery_log::println(to_deployment_list_json_output(&deployments));
        return;
    }

    let data: Vec<Vec<String>> = deployments
        .iter()
        .map(|d| {
            let created_at = d["created_at"].as_str().unwrap_or_default();
            let updated_at = d["updated_at"].as_str().unwrap_or_default();
            vec![
                d["id"].as_str().unwrap_or_default().to_string(),
                time_string_or_zero(created_at),
                status::status_text_with_color(d["status"].as_str().unwrap_or_default()),
                get_duration(&parse_api_time(created_at), &parse_api_time(updated_at)),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Deployed At", "Status", "Duration"], &data);
}

fn time_string_or_zero(s: &str) -> String {
    if DateTime::parse_from_rfc3339(s).is_ok() {
        time_string_from_rfc3339(s)
    } else {
        qovery_log::gofmt::ZERO_TIME_STRING.to_string()
    }
}

/// `int(updatedAt.Sub(createdAt).Seconds())` (`Sub` saturates like Go).
fn duration_in_seconds(start: &DateTime<FixedOffset>, end: &DateTime<FixedOffset>) -> i64 {
    let delta = end.signed_duration_since(*start);
    let nanos = delta
        .num_nanoseconds()
        .unwrap_or(if delta < chrono::TimeDelta::zero() {
            i64::MIN
        } else {
            i64::MAX
        });
    nanos / 1_000_000_000
}

fn to_deployment_list_json_output(deployments: &[Value]) -> String {
    let results: Vec<Value> = deployments
        .iter()
        .map(|d| {
            let created_at = parse_api_time(d["created_at"].as_str().unwrap_or_default());
            let updated_at = parse_api_time(d["updated_at"].as_str().unwrap_or_default());
            json!({
                "id": d["id"].as_str().unwrap_or_default(),
                "created_at": to_iso8601_time(&created_at),
                "status": d["status"].as_str().unwrap_or_default(),
                "deployment_duration_in_seconds": duration_in_seconds(&created_at, &updated_at),
            })
        })
        .collect();
    if results.is_empty() {
        return "null".to_string();
    }
    go_json(&Value::Array(results))
}

// ---------------------------------------------------------------------------------------------------------------
// logs

const LOGS_LONG: &str = r#"Print the deployment logs of an environment -- the engine output shown in the "Deployment logs" tab of the console.

These are the logs of a deployment itself (build, deploy, health checks), not the runtime output of a
running service: for that, use "qovery log".

Without --execution-id, the logs of the latest deployment are printed. The API returns at most the
last 1000 lines.

--service matches the service name carried by each log line, so it works for every service type
(including Terraform services). Use --service-id to match on the service uuid instead; the two are mutually exclusive. Either way,
the environment-level lines are kept for context.

Note: the console additionally scopes lines to the deployment stage the service belongs to, which
the CLI cannot resolve without an extra lookup, so a few stage-level lines from other stages may
appear in a service-scoped view."#;

fn logs() -> Command {
    // Go also accepts `--id` as a spelling of `--execution-id` (pflag `SetNormalizeFunc`).
    Command::new("logs")
        .short("Print the deployment logs of an environment")
        .long(LOGS_LONG)
        .flags(ope_flags())
        .flag(Flag::string(
            "execution-id",
            "Execution Id of a past deployment, as listed by 'qovery environment deployment list' (default: latest deployment). Also accepted as --id, matching 'deployment explain'",
        ).alias("id"))
        .flag(Flag::string("service", "Service Name -- only print the log lines related to that service's deployment").short('s'))
        .flag(Flag::string("service-id", "Service Id -- same as --service, skipping the name lookup"))
        .flag(Flag::bool("pre-check", "Only print the pre-check log lines"))
        .flag(Flag::bool("errors-only", "Only print the log lines carrying an error"))
        .flag(Flag::bool("json", "Print the logs as JSON"))
        .run(run_logs)
}

fn fail_stderr(e: impl std::fmt::Display) -> ! {
    println_error_to_stderr(e);
    exit(1)
}

/// `ListEnvironmentLogs(envId)[.Version(id)]`: `GET /environment/{id}/logs[?version=...]`.
fn list_environment_logs(
    client: &qovery_api::Client,
    environment_id: &str,
    version: &str,
) -> Result<Vec<EnvironmentLog>, String> {
    let path = format!("/environment/{}/logs", path_escape(environment_id));
    let query: Vec<(&str, String)> = if version.is_empty() {
        vec![]
    } else {
        vec![("version", version.to_string())]
    };
    let v: Value = client
        .get_with_query(&path, &query)
        .map_err(|e| e.to_string())?;
    if v.is_null() {
        return Ok(Vec::new());
    }
    serde_json::from_value(v).map_err(|e| e.to_string())
}

fn run_logs(ctx: &Ctx) {
    telemetry::capture(ctx);

    let execution_id = ctx.string("execution-id");
    let service_name = ctx.string("service");
    let service_id = ctx.string("service-id");
    let pre_check = ctx.bool("pre-check");
    let errors_only = ctx.bool("errors-only");

    if pre_check && (!service_name.is_empty() || !service_id.is_empty()) {
        fail_stderr(
            "--pre-check cannot be combined with --service or --service-id: pre-check runs before any service is deployed",
        );
    }

    // They are two spellings of one selector, and the filter matches either, so passing
    // both would widen the result to two services rather than narrow it.
    if !service_name.is_empty() && !service_id.is_empty() {
        fail_stderr(
            "--service and --service-id select the same thing two different ways: pass one, not both",
        );
    }

    let client = match client::get_client() {
        Ok(c) => c,
        Err(e) => fail_stderr(e),
    };
    let (_, _, environment_id) = match ids_from_flags(&client, ctx) {
        Ok(ids) => ids,
        Err(e) => fail_stderr(e),
    };

    let all = match list_environment_logs(&client, &environment_id, &execution_id) {
        Ok(l) => l,
        Err(e) => fail_stderr(e),
    };

    let filter = DeploymentLogFilter {
        service_id,
        service_name,
        pre_check_only: pre_check,
        errors_only,
    };

    // Apply the scope first and the error filter second, so the two reasons a result can
    // come back empty stay distinguishable.
    let scope_filter = DeploymentLogFilter {
        errors_only: false,
        ..filter.clone()
    };
    let scoped = filter_deployment_logs(&all, &scope_filter);

    let logs = if filter.errors_only {
        filter_deployment_logs(
            &scoped,
            &DeploymentLogFilter {
                errors_only: true,
                ..Default::default()
            },
        )
    } else {
        scoped.clone()
    };

    // All of these go to stderr, not stdout.
    if filter.is_service_scoped() && !has_service_specific_lines(&scoped) {
        let services = deployment_log_services(&all);
        if services.is_empty() {
            println_info_to_stderr("No service emitted deployment logs in this deployment.");
        } else {
            println_info_to_stderr(format!(
                "No deployment logs for that service. Services in this deployment: {}",
                services.join(", ")
            ));
        }
    } else if filter.errors_only && logs.is_empty() && filter.is_service_scoped() {
        println_info_to_stderr("No errors for that service in this deployment.");
    } else if filter.errors_only && logs.is_empty() {
        println_info_to_stderr("No errors in this deployment.");
    }

    if ctx.bool("json") {
        let projected: Vec<Value> = logs.iter().map(deployment_log_json).collect();
        println!("{}", go_json(&Value::Array(projected)));
        return;
    }

    for l in &logs {
        println!("{}", format_deployment_log_line(l));
    }
}

// ---------------------------------------------------------------------------------------------------------------
// explain

const STAGE_LEVEL: i32 = 1;
const SERVICE_LEVEL: i32 = 2;
const STEP_LEVEL: i32 = 3;
const MESSAGE_LEVEL: i32 = 4;
const ALL_LEVEL: i32 = 5;

fn explain() -> Command {
    Command::new("explain")
        .short("Explain environment deployment -- give details about what happened during the deployment")
        .flags(ope_flags())
        .flag(Flag::string("id", "Deployment Id"))
        .flag(Flag::string("level", "Show only: <all|stage|service|step|message> (default: all)").default_value("all"))
        .run(run_explain)
}

fn run_explain(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());

    let level = ctx.string("level");
    if !matches!(
        level.as_str(),
        "" | "all" | "stage" | "service" | "step" | "message"
    ) {
        fail(format!("invalid value for --show-only: {level}"));
    }

    let (_, _, environment_id) = check_error(ids_from_flags(&client, ctx));

    let environment: Environment =
        match client.get(&format!("/environment/{}", path_escape(&environment_id))) {
            Ok(e) => e,
            Err(e) => fail(e),
        };

    let logs = match list_environment_logs(&client, &environment_id, &ctx.string("id")) {
        Ok(l) => l,
        Err(e) => fail(e),
    };

    let m_level = match level.as_str() {
        "stage" => STAGE_LEVEL,
        "service" => SERVICE_LEVEL,
        "step" => STEP_LEVEL,
        "message" => MESSAGE_LEVEL,
        _ => ALL_LEVEL,
    };

    let mut tree = treeprint::Node::new();
    let env_branch = tree.add_branch(format!(
        "Environment: {} [duration: {}]",
        environment.name,
        get_duration_from_logs(&logs)
    ));

    for (stage_idx, stage) in get_stages_from_logs(&logs).iter().enumerate() {
        let (start, end) = start_end(logs.iter().filter(|l| &l.stage_name() == stage));
        let branch = env_branch.add_branch(format!(
            "Stage {}: {} [duration: {}]",
            stage_idx + 1,
            stage,
            get_duration(&start, &end)
        ));

        if m_level >= SERVICE_LEVEL {
            for service in get_services_from_logs_by_stage(stage, &logs) {
                let (start, end) = start_end(
                    logs.iter()
                        .filter(|l| &l.stage_name() == stage && l.transmitter_name() == service),
                );
                let service_branch = branch.add_branch(format!(
                    "{service} [duration: {}]",
                    get_duration(&start, &end)
                ));

                if m_level >= STEP_LEVEL {
                    let mut step_idx = 0;
                    for step in get_steps_from_logs_by_service(&service, &logs) {
                        let by_service_and_step = || {
                            logs.iter()
                                .filter(|l| l.transmitter_name() == service && l.step() == step)
                        };
                        let (start, end) = start_end(by_service_and_step());
                        if end.signed_duration_since(start) > chrono::TimeDelta::zero() {
                            step_idx += 1;
                            // only display if step took more than 0 seconds
                            let step_branch = service_branch.add_branch(format!(
                                "Step {step_idx}: {step} [duration: {}]",
                                get_duration(&start, &end)
                            ));

                            if m_level >= MESSAGE_LEVEL {
                                for step_log in by_service_and_step() {
                                    step_branch.add_node(step_log.safe_message());
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    println!("{}", tree.string());
}

/// Min / max timestamps of `logs` (Go's zero time when there is none).
fn start_end<'a>(
    logs: impl Iterator<Item = &'a EnvironmentLog>,
) -> (DateTime<FixedOffset>, DateTime<FixedOffset>) {
    let zero = go_zero_time();
    let mut start: Option<DateTime<FixedOffset>> = None;
    let mut end: Option<DateTime<FixedOffset>> = None;
    for l in logs {
        // Go: `startTime.IsZero() || startTime.After(log.Timestamp)`
        if start.is_none_or(|s| s == zero || s > l.timestamp) {
            start = Some(l.timestamp);
        }
        if end.is_none_or(|e| e == zero || e < l.timestamp) {
            end = Some(l.timestamp);
        }
    }
    (start.unwrap_or(zero), end.unwrap_or(zero))
}

fn get_duration_from_logs(logs: &[EnvironmentLog]) -> String {
    let (start, end) = start_end(logs.iter());
    get_duration(&start, &end)
}

/// Distinct values in order of appearance.
fn distinct(values: impl Iterator<Item = String>) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    for v in values {
        if !out.contains(&v) {
            out.push(v);
        }
    }
    out
}

fn get_stages_from_logs(logs: &[EnvironmentLog]) -> Vec<String> {
    distinct(logs.iter().map(|l| l.stage_name()))
}

fn get_services_from_logs_by_stage(stage: &str, logs: &[EnvironmentLog]) -> Vec<String> {
    distinct(
        logs.iter()
            .filter(|l| l.stage_name() == stage && l.transmitter_type() != "Environment")
            .map(|l| l.transmitter_name()),
    )
}

fn get_steps_from_logs_by_service(service: &str, logs: &[EnvironmentLog]) -> Vec<String> {
    distinct(
        logs.iter()
            .filter(|l| l.transmitter_name() == service)
            .map(|l| l.step()),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn duration_seconds_saturates() {
        let created = parse_api_time("2024-06-02T11:30:00Z");
        assert_eq!(
            duration_in_seconds(&created, &parse_api_time("2024-06-02T11:31:05.9Z")),
            65
        );
        assert_eq!(
            duration_in_seconds(&created, &parse_api_time("")),
            -9223372036
        );
    }

    #[test]
    fn start_end_of_logs() {
        let mk = |ts: &str| EnvironmentLog {
            timestamp: parse_api_time(ts),
            ..Default::default()
        };
        let logs = [
            mk("2024-06-02T11:30:05Z"),
            mk("2024-06-02T11:30:00Z"),
            mk("2024-06-02T11:31:00Z"),
        ];
        let (s, e) = start_end(logs.iter());
        assert_eq!(get_duration(&s, &e), "1 minute and 0 seconds");
        let (s, e) = start_end(std::iter::empty());
        assert_eq!(get_duration(&s, &e), "0 seconds");
    }
}
