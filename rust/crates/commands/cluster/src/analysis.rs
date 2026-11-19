//! `cluster analysis ...` (cmd/cluster_analysis*.go).

use crate::util::{self, http_error, path_escape};
use qovery_api::{ApiError, Client};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, exit, exit_with_error, telemetry};
use qovery_log::gofmt::{go_json, quote};
use qovery_log::pterm;
use serde_json::{Value, json};

pub fn command() -> Command {
    Command::new("analysis")
        .short("Run and inspect read-only cluster analyses (e.g. cost recommendations)")
        .run(crate::run_group)
        .subcommand(cost_recommendation_command())
        .subcommand(deprecated_api_command())
        .subcommand(list_command())
        .subcommand(logs_command())
}

fn run_flags(cmd: Command) -> Command {
    cmd.flag(
        Flag::string("cluster-id", "Cluster ID")
            .short('c')
            .required(),
    )
    .flag(Flag::string("output", "Report output: table, json, csv").default_value("json"))
    .flag(
        Flag::bool(
            "watch",
            "Wait for the analysis to finish and print its report",
        )
        .default_value("true"),
    )
    .flag(Flag::bool(
        "no-logs",
        "Do not print the report logs when finished",
    ))
}

const COST_EXAMPLE: &str = "  qovery cluster analysis cost-recommendation \\
    -c <cluster_id> \\
    --output json \\
    --cmd-arg=--history_duration \\
    --cmd-arg=336 \\
    --cmd-arg=--timeframe_duration \\
    --cmd-arg=2.5 \\
    --cmd-arg=--cpu-request \\
    --cmd-arg=99 \\
    --cmd-arg=--cpu-limit \\
    --cmd-arg=99 \\
    --cmd-arg=--memory-buffer-percentage \\
    --cmd-arg=15 \\
    --cmd-arg=--use-oomkill-data \\
    --cmd-arg=--oom-memory-buffer-percentage \\
    --cmd-arg=25 \\
    --cmd-arg=--allow-hpa";

fn cost_recommendation_command() -> Command {
    run_flags(
        Command::new("cost-recommendation")
            .short("Start a cluster cost recommendation analysis, optionally wait for completion, then print its report")
            .example(COST_EXAMPLE),
    )
    .flag(Flag::string("prometheus-url", "Optional Prometheus URL"))
    .flag(Flag::string_array(
        "cmd-arg",
        "Optional allowlisted command argument. Repeat for each argument, e.g. --cmd-arg=--history_duration --cmd-arg=336",
    ))
    .run(run_cost_recommendation)
}

fn deprecated_api_command() -> Command {
    run_flags(
        Command::new("deprecated-api")
            .short("Start a deprecated Kubernetes API analysis, optionally wait for completion, then print its report"),
    )
    .flag(Flag::string("target-kubernetes-version", "Optional target Kubernetes version"))
    .run(run_deprecated_api)
}

fn list_command() -> Command {
    Command::new("list")
        .short("List previous analyses for a cluster")
        .flag(
            Flag::string("cluster-id", "Cluster ID")
                .short('c')
                .required(),
        )
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn logs_command() -> Command {
    Command::new("logs")
        .short("Print the report/logs of a past cluster analysis")
        .flag(
            Flag::string("cluster-id", "Cluster ID")
                .short('c')
                .required(),
        )
        .flag(
            Flag::string("analysis-id", "Analysis ID")
                .short('a')
                .required(),
        )
        .flag(Flag::bool("json", "JSON output"))
        .run(run_logs)
}

/// `parseAnalysisOutput`: CLI `--output` value to the engine output format.
pub fn parse_analysis_output(s: &str) -> Result<&'static str, String> {
    match s.trim().to_lowercase().as_str() {
        "" | "json" => Ok("JSON"),
        "table" => Ok("TABLE"),
        "csv" => Ok("CSV"),
        _ => Err(format!(
            "invalid output format {} (allowed: table, json, csv)",
            quote(s)
        )),
    }
}

/// `isFinalAnalysisStatus`.
pub fn is_final_analysis_status(status: &str) -> bool {
    matches!(status, "SUCCEEDED" | "FAILED" | "TERMINATED")
}

fn s<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(|x| x.as_str()).unwrap_or_default()
}

fn run_cost_recommendation(ctx: &Ctx) {
    telemetry::capture(ctx);
    let output = match parse_analysis_output(&ctx.string("output")) {
        Ok(o) => o,
        Err(e) => exit_with_error(e),
    };
    let mut request = json!({"kind": "COST_RECOMMENDATION", "output_format": output});
    let prometheus = ctx.string("prometheus-url");
    if !prometheus.is_empty() {
        request["prometheus_url"] = json!(prometheus);
    }
    let cmd_args = ctx.strings("cmd-arg");
    if !cmd_args.is_empty() {
        request["cmd_args"] = json!(cmd_args);
    }
    run_cluster_analysis(ctx, &request);
}

fn run_deprecated_api(ctx: &Ctx) {
    telemetry::capture(ctx);
    let output = match parse_analysis_output(&ctx.string("output")) {
        Ok(o) => o,
        Err(e) => exit_with_error(e),
    };
    let mut request = json!({"kind": "DEPRECATED_API_CHECK", "output_format": output});
    let target = ctx.string("target-kubernetes-version");
    if !target.is_empty() {
        request["target_kubernetes_version"] = json!(target);
    }
    run_cluster_analysis(ctx, &request);
}

fn api_err(e: ApiError) -> ! {
    exit_with_error(http_error(&e))
}

/// `runClusterAnalysis`.
fn run_cluster_analysis(ctx: &Ctx, request: &Value) {
    let client = client::get_client_or_exit();
    let cluster_id = ctx.string("cluster-id");
    let base = format!("/clusters/{}/analysis", path_escape(&cluster_id));
    let analysis: Value = client.post(&base, request).unwrap_or_else(|e| api_err(e));
    let analysis_id = s(&analysis, "id").to_string();
    qovery_log::println(format!(
        "Analysis {} started ({})",
        pterm::FG_BLUE.sprint(&analysis_id),
        s(&analysis, "status")
    ));

    if !ctx.bool("watch") {
        qovery_log::println_info(format!(
            "Run 'qovery cluster analysis logs --cluster-id {cluster_id} --analysis-id {analysis_id}' to fetch the report once finished."
        ));
        return;
    }

    let mut last_status = s(&analysis, "status").to_string();
    while !is_final_analysis_status(&last_status) {
        std::thread::sleep(std::time::Duration::from_secs(5));
        let current: Value = client
            .get(&format!("{base}/{}", path_escape(&analysis_id)))
            .unwrap_or_else(|e| api_err(e));
        let current_status = s(&current, "status").to_string();
        if current_status != last_status {
            last_status = current_status.clone();
            qovery_log::println(format!("Status: {last_status}"));
        }
        if is_final_analysis_status(&current_status) {
            last_status = current_status;
            let err_msg = s(&current, "error_message");
            if !err_msg.is_empty() {
                qovery_log::println(pterm::error_printer().sprint(err_msg));
            }
            break;
        }
    }

    if !ctx.bool("no-logs")
        && let Err(e) = print_analysis_logs(&client, &cluster_id, &analysis_id)
    {
        exit_with_error(e);
    }

    if last_status != "SUCCEEDED" {
        qovery_log::println(pterm::error_printer().sprint(&format!(
            "Analysis {analysis_id} ended with status {last_status}"
        )));
        exit(1);
    }
    qovery_log::println(pterm::FG_GREEN.sprint(format!("Analysis {analysis_id} succeeded")));
}

fn list_logs(client: &Client, cluster_id: &str, analysis_id: &str) -> Result<Vec<Value>, ApiError> {
    client.list(&format!(
        "/clusters/{}/analysis/{}/logs",
        path_escape(cluster_id),
        path_escape(analysis_id)
    ))
}

/// `printAnalysisLogs`.
fn print_analysis_logs(client: &Client, cluster_id: &str, analysis_id: &str) -> Result<(), String> {
    let logs = list_logs(client, cluster_id, analysis_id).map_err(|e| http_error(&e))?;
    qovery_log::println(analysis_report_from_logs(&logs));
    Ok(())
}

/// `analysisReportFromLogs`.
fn analysis_report_from_logs(logs: &[Value]) -> String {
    logs.iter()
        .map(|l| s(l, "message"))
        .collect::<Vec<_>>()
        .join("\n")
}

fn time_iso(v: &Value, k: &str) -> Value {
    // Go: utils.ToIso8601(&t) of a (possibly zero) time.Time
    match v.get(k).and_then(|x| x.as_str()).and_then(util::to_iso8601) {
        Some(t) => json!(t),
        None => json!("0001-01-01T00:00:00.000Z"),
    }
}

fn analysis_json_output(analyses: &[Value]) -> String {
    if analyses.is_empty() {
        return "null".to_string();
    }
    let results: Vec<Value> = analyses
        .iter()
        .map(|a| {
            json!({
                "id": s(a, "id"),
                "cluster_id": s(a, "cluster_id"),
                "kind": s(a, "kind"),
                "status": s(a, "status"),
                "created_at": time_iso(a, "created_at"),
                "updated_at": time_iso(a, "updated_at"),
                "triggered_by": s(a, "triggered_by"),
                "error": s(a, "error_message"),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let analyses: Vec<Value> = client
        .list(&format!(
            "/clusters/{}/analysis",
            path_escape(&ctx.string("cluster-id"))
        ))
        .unwrap_or_else(|e| api_err(e));

    if ctx.bool("json") {
        qovery_log::println(analysis_json_output(&analyses));
        return;
    }

    let data: Vec<Vec<String>> = analyses
        .iter()
        .map(|a| {
            let created = a
                .get("created_at")
                .and_then(|x| x.as_str())
                .and_then(util::parse_time)
                .map(|t| util::format_rfc3339(&t))
                .unwrap_or_else(|| "0001-01-01T00:00:00Z".to_string());
            vec![
                s(a, "id").to_string(),
                s(a, "kind").to_string(),
                s(a, "status").to_string(),
                created,
                s(a, "triggered_by").to_string(),
                s(a, "error_message").to_string(),
            ]
        })
        .collect();
    qovery_log::print_table(
        &[
            "Id",
            "Kind",
            "Status",
            "Created At",
            "Triggered By",
            "Error",
        ],
        &data,
    );
}

fn logs_json_output(logs: &[Value]) -> String {
    if logs.is_empty() {
        return "null".to_string();
    }
    let results: Vec<Value> = logs
        .iter()
        .map(|l| {
            json!({
                "timestamp": time_iso(l, "timestamp"),
                "level": s(l, "level"),
                "message": s(l, "message"),
                "line_order": l.get("line_order").and_then(|x| x.as_i64()).unwrap_or(0),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

fn run_logs(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let logs = list_logs(
        &client,
        &ctx.string("cluster-id"),
        &ctx.string("analysis-id"),
    )
    .unwrap_or_else(|e| api_err(e));
    if ctx.bool("json") {
        qovery_log::println(logs_json_output(&logs));
        return;
    }
    qovery_log::println(analysis_report_from_logs(&logs));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_analysis_output_like_go() {
        let cases = [
            ("", Some("JSON")),
            ("json", Some("JSON")),
            ("table", Some("TABLE")),
            ("csv", Some("CSV")),
            (" CSV ", Some("CSV")),
            ("yaml", None),
        ];
        for (input, expected) in cases {
            match expected {
                Some(e) => assert_eq!(parse_analysis_output(input), Ok(e), "{input}"),
                None => assert!(parse_analysis_output(input).is_err(), "{input}"),
            }
        }
        assert_eq!(
            parse_analysis_output("yaml"),
            Err("invalid output format \"yaml\" (allowed: table, json, csv)".to_string())
        );
    }

    #[test]
    fn final_analysis_status() {
        for (status, expected) in [
            ("PENDING", false),
            ("RUNNING", false),
            ("SUCCEEDED", true),
            ("FAILED", true),
            ("TERMINATED", true),
        ] {
            assert_eq!(is_final_analysis_status(status), expected, "{status}");
        }
    }

    #[test]
    fn report_from_logs() {
        let logs = vec![json!({"message": "a"}), json!({"message": "b"})];
        assert_eq!(analysis_report_from_logs(&logs), "a\nb");
        assert_eq!(analysis_report_from_logs(&[]), "");
    }
}
