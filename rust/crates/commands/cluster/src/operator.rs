//! `cluster operator ...` (cmd/cluster_operator*.go).

use crate::manage::{find_cluster_by_name, str_field};
use crate::util::{self, path_escape};
use qovery_api::Client;
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::{client, exit, resources, telemetry};
use qovery_log::gofmt::go_json_indent;
use serde_json::Value;

pub fn command() -> Command {
    Command::new("operator")
        .short("Manage the Qovery Operator on a cluster")
        .args(ArgsRule::NoArgs)
        .run(run_operator)
        .subcommand(status_command())
}

fn run_operator(ctx: &Ctx) {
    telemetry::capture(ctx);
    ctx.help();
    exit(0);
}

fn status_command() -> Command {
    Command::new("status")
        .short("Show the Qovery Operator connection and version status")
        .flag(Flag::string("organization", "Organization name"))
        .flag(Flag::string("cluster", "Cluster name").required())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_status)
}

/// `operatorCommandContext`.
struct OperatorCommandContext {
    api: Client,
    cluster_id: String,
    organization_id: String,
}

/// `newOperatorCommandContext(organizationName, clusterName)`.
fn new_operator_command_context(
    organization_name: &str,
    cluster_name: &str,
) -> Result<OperatorCommandContext, String> {
    let client = client::get_client()?;
    let organization_id = resources::organization_id(&client, organization_name)?;
    let clusters = crate::manage::list_organization_clusters(&client, &organization_id)?;
    let cluster = find_cluster_by_name(&clusters, cluster_name)
        .ok_or_else(|| format!("cluster {cluster_name} not found"))?;
    Ok(OperatorCommandContext {
        cluster_id: str_field(cluster, "id").to_string(),
        api: client,
        organization_id,
    })
}

/// `displayVersion(version qovery.NullableString)`: the version, or `unknown` when null or empty.
pub fn display_version(version: Option<&str>) -> String {
    match version {
        Some(v) if !v.is_empty() => v.to_string(),
        _ => "unknown".to_string(),
    }
}

fn fail(e: impl std::fmt::Display) -> ! {
    qovery_log::println_error_to_stderr(e);
    exit(1)
}

/// Go marshals `time.Time` values with RFC 3339 nano: normalize the timestamps of the API response.
fn normalize_times(mut v: Value) -> Value {
    if let Some(Value::String(s)) = v.get("last_heartbeat")
        && let Some(t) = util::parse_time(s)
    {
        v["last_heartbeat"] = Value::String(util::format_rfc3339_nano(&t));
    }
    v
}

fn run_status(ctx: &Ctx) {
    telemetry::capture(ctx);
    let cc = new_operator_command_context(&ctx.string("organization"), &ctx.string("cluster"))
        .unwrap_or_else(|e| fail(e));
    let status: Value = cc
        .api
        .get(&format!(
            "/organization/{}/cluster/{}/operator/status",
            path_escape(&cc.organization_id),
            path_escape(&cc.cluster_id)
        ))
        .unwrap_or_else(|e| fail(e));

    if ctx.bool("json") {
        qovery_log::println(go_json_indent(&normalize_times(status), "  "));
        return;
    }

    let connected = if status
        .get("operator_connected")
        .and_then(|v| v.as_bool())
        .unwrap_or(false)
    {
        "yes"
    } else {
        "no"
    };
    let last_heartbeat = status
        .get("last_heartbeat")
        .and_then(|v| v.as_str())
        .and_then(util::parse_time)
        .map(|t| util::format_rfc3339(&t))
        .unwrap_or_else(|| "never".to_string());
    let version = |k: &str| display_version(status.get(k).and_then(|v| v.as_str()));
    let data = vec![vec![
        str_field(&status, "status").to_string(),
        connected.to_string(),
        str_field(&status, "reported_runtime").to_string(),
        last_heartbeat,
        version("operator_version"),
        version("desired_image_version"),
        version("reported_chart_version"),
        version("desired_chart_version"),
    ]];
    qovery_log::print_table(
        &[
            "Status",
            "Connected",
            "Runtime",
            "Last heartbeat",
            "Image",
            "Target image",
            "Chart",
            "Target chart",
        ],
        &data,
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn display_version_like_go() {
        assert_eq!(display_version(None), "unknown");
        assert_eq!(display_version(Some("")), "unknown");
        assert_eq!(display_version(Some("1.2.3")), "1.2.3");
    }
}
