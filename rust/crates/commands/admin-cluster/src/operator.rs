//! `qovery admin cluster operator [list|update]`.
//! Port of `cmd/admin_cluster_operator*.go` (and the helpers of `cmd/cluster_operator_helpers.go` they use).

use crate::common::{HttpResponse, Method, http_request, http_request_ctx, validate};
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_log::gofmt::{duration_string, duration_string_nanos, go_json_indent};
use qovery_log::println;
use serde_json::Value;
use std::cell::RefCell;
use std::collections::{BTreeMap, BTreeSet};
use std::time::{Duration, Instant};

const SELF_MANAGED_CLUSTER_KINDS: &[&str] = &[
    "EKS_SELF_MANAGED",
    "SCW_SELF_MANAGED",
    "GKE_SELF_MANAGED",
    "AKS_SELF_MANAGED",
    "EKS_ANYWHERE",
    "SELF_MANAGED",
];
const OPERATOR_RUNTIMES: &[&str] = &["KUBERNETES", "DOCKER", "UNKNOWN"];
const FLEET_STATUSES: &[(&str, &str)] = &[
    (
        "NOT_ATTACHED",
        "cluster not attached to the Operator path (never selected: only attached clusters are)",
    ),
    (
        "DISCONNECTED",
        "no Operator heartbeat in the last 2 minutes",
    ),
    (
        "TARGET_UNKNOWN",
        "q-core has no target image or chart version",
    ),
    (
        "REPORTED_VERSION_UNKNOWN",
        "the Operator did not report its image or chart version",
    ),
    ("CURRENT", "reported image and chart match the target"),
    ("OUTDATED_IMAGE", "reported image differs from the target"),
    ("OUTDATED_CHART", "reported chart differs from the target"),
    (
        "OUTDATED_IMAGE_AND_CHART",
        "reported image and chart both differ from the target",
    ),
];
const FILTER_KEYS: &[&str] = &[
    "OrganizationId",
    "ClusterId",
    "ClusterName",
    "ClusterKind",
    "Runtime",
    "Status",
    "Connected",
];
const FILTERS_USAGE: &str =
    "Value(s) to filter the property selected separated by comma when multiple values are defined";

fn fleet_statuses() -> Vec<&'static str> {
    FLEET_STATUSES.iter().map(|(s, _)| *s).collect()
}

/// `clusterOperatorFilterHelp()`.
pub fn filter_help() -> String {
    let mut help = String::from(
        "Filters (-f, --filters Key=Values):
  Comma separated values are a logical OR; different keys are a logical AND.
  Values are matched exactly and are case-sensitive, except Connected. A key repeated in several -f keeps only
  the last one: put all values of a key in one -f, separated by commas.

",
    );
    let keys: [(&str, String); 7] = [
        ("OrganizationId", "organization UUID".into()),
        ("ClusterId", "cluster UUID".into()),
        ("ClusterName", "exact cluster name".into()),
        ("ClusterKind", SELF_MANAGED_CLUSTER_KINDS.join(", ")),
        ("Runtime", OPERATOR_RUNTIMES.join(", ")),
        ("Status", "a fleet status below".into()),
        ("Connected", "true or false".into()),
    ];
    for (k, v) in keys {
        help.push_str(&format!("  {k:<15} {v}\n"));
    }
    help.push_str("\nFleet statuses:\n");
    for (status, description) in FLEET_STATUSES {
        help.push_str(&format!("  {status:<25} {description}\n"));
    }
    help
}

pub fn command() -> Command {
    Command::new("operator")
        .short("Manage the Qovery Operator fleet")
        .args(ArgsRule::NoArgs)
        .run(|ctx| {
            qovery_core::telemetry::capture(ctx);
            ctx.help();
            qovery_core::exit(0)
        })
        .subcommand(list())
        .subcommand(update())
}

fn list() -> Command {
    let long = format!(
        "List the clusters attached to the Qovery Operator execution path.\n\n{}
Examples:
qovery admin cluster operator list --outdated
qovery admin cluster operator list -f Runtime=KUBERNETES -f Status=OUTDATED_IMAGE,OUTDATED_CHART
qovery admin cluster operator list -f Connected=false --json
qovery admin cluster operator list -f ClusterKind=EKS_ANYWHERE
qovery admin cluster operator list -f Runtime=DOCKER,KUBERNETES -f Connected=true",
        filter_help()
    );
    Command::new("list")
        .short("List the Qovery Operator fleet")
        .long(&long)
        .flag(Flag::bool("json", "JSON output"))
        .flag(Flag::string_to_string("filters", FILTERS_USAGE).short('f'))
        .flag(Flag::bool(
            "outdated",
            "Only show clusters whose Operator image or chart is outdated",
        ))
        .run(run_list)
}

fn update() -> Command {
    let long = format!(
        "Update the Qovery Operator image (and Helm chart for Kubernetes) on attached clusters.

The selection is the attached Operator fleet, narrowed with the filters below.
Without filters the whole attached fleet is considered.

A cluster is updated only when its Operator is connected, its runtime is KUBERNETES or DOCKER, and there is something to do:
* without --image-version / --chart-version: its status is OUTDATED_IMAGE, OUTDATED_CHART or OUTDATED_IMAGE_AND_CHART;
* with explicit versions: its reported image (or chart, for KUBERNETES) differs from the version the update would install.
Without --image-version q-core picks its target image. A KUBERNETES cluster without --chart-version gets its desired chart.
DOCKER clusters never receive a chart version.

The command runs in dry run by default and only prints the plan. Use --disable-dry-run (-y) to trigger the updates.
Up to --parallel-run updates run at the same time. After the first failure no new update is started.

{}
Examples:
qovery admin cluster operator update
qovery admin cluster operator update -f Runtime=DOCKER -y
qovery admin cluster operator update -f ClusterId=<cluster-id> --image-version=v1.204.0 --chart-version=0.3.0 -y
qovery admin cluster operator update -f Status=OUTDATED_IMAGE -n 5 --refresh-delay=30 --timeout=20m -y -c
qovery admin cluster operator update -f ClusterKind=EKS_ANYWHERE
qovery admin cluster operator update -f Runtime=DOCKER,KUBERNETES -f Connected=true",
        filter_help()
    );
    Command::new("update")
        .short("Update the Qovery Operator on attached clusters")
        .long(&long)
        .flag(Flag::string_to_string("filters", FILTERS_USAGE).short('f'))
        .flag(Flag::string("image-version", "Operator image version to install (default: q-core target)"))
        .flag(Flag::string(
            "chart-version",
            "Operator Helm chart version to install on KUBERNETES clusters (default: desired chart version)",
        ))
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .flag(Flag::bool("no-confirm", "Do not prompt for confirmation").short('c'))
        .flag(
            Flag::int("parallel-run", "Number of clusters to update in parallel - must be between 1 and 10")
                .short('n')
                .default_value("1"),
        )
        .flag(
            Flag::int("refresh-delay", "Time in seconds between progress checks - must be between 5 and 120")
                .short('r')
                .default_value("30"),
        )
        .flag(Flag::duration("timeout", "Maximum time to wait for each cluster update").default_value("15m0s"))
        .run(run_update)
}

// ---------------------------------------------------------------------------------------------------------------
// model

/// `qovery.ClusterOperatorFleetInventoryResponse`.
#[derive(Debug, Clone, Default)]
pub struct FleetEntry {
    pub organization_id: String,
    pub cluster_id: String,
    pub cluster_name: String,
    pub cluster_kind: String,
    pub attached: bool,
    pub connected: bool,
    pub last_heartbeat: Option<chrono::DateTime<chrono::FixedOffset>>,
    pub desired_image_version: Option<String>,
    pub reported_image_version: Option<String>,
    pub desired_chart_version: Option<String>,
    pub reported_chart_version: Option<String>,
    pub reported_runtime: String,
    pub status: String,
    /// The decoded JSON object (`ToMap()` re-serializes every property, additional ones included).
    pub raw: Value,
}

impl FleetEntry {
    /// Go `UnmarshalJSON` (required properties and enum values are checked).
    fn decode(v: &Value) -> Result<FleetEntry, String> {
        let obj = v.as_object().ok_or_else(|| {
            "json: cannot unmarshal into Go value of type map[string]interface {}".to_string()
        })?;
        for p in [
            "organization_id",
            "cluster_id",
            "cluster_name",
            "cluster_kind",
            "attached",
            "connected",
            "reported_runtime",
            "status",
        ] {
            if !obj.contains_key(p) {
                return Err(format!("no value given for required property {p}"));
            }
        }
        let s = |k: &str| {
            v.get(k)
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string()
        };
        let ns = |k: &str| v.get(k).and_then(|x| x.as_str()).map(|x| x.to_string());
        let check = |k: &str, allowed: &[&str], name: &str| -> Result<String, String> {
            let value = s(k);
            if allowed.contains(&value.as_str()) {
                Ok(value)
            } else {
                Err(format!("{value} is not a valid {name}"))
            }
        };
        let cluster_kind = check(
            "cluster_kind",
            SELF_MANAGED_CLUSTER_KINDS,
            "SelfManagedClusterKind",
        )?;
        let reported_runtime = check(
            "reported_runtime",
            OPERATOR_RUNTIMES,
            "ClusterOperatorRuntime",
        )?;
        let status = check("status", &fleet_statuses(), "ClusterOperatorFleetStatus")?;
        Ok(FleetEntry {
            organization_id: s("organization_id"),
            cluster_id: s("cluster_id"),
            cluster_name: s("cluster_name"),
            cluster_kind,
            attached: v["attached"].as_bool().unwrap_or(false),
            connected: v["connected"].as_bool().unwrap_or(false),
            last_heartbeat: v
                .get("last_heartbeat")
                .and_then(|x| x.as_str())
                .and_then(|t| chrono::DateTime::parse_from_rfc3339(t).ok()),
            desired_image_version: ns("desired_image_version"),
            reported_image_version: ns("reported_image_version"),
            desired_chart_version: ns("desired_chart_version"),
            reported_chart_version: ns("reported_chart_version"),
            reported_runtime,
            status,
            raw: v.clone(),
        })
    }

    /// `MarshalJSON()` (`ToMap()`): every decoded property, `last_heartbeat` re-formatted as a Go `time.Time`.
    fn to_json(&self) -> Value {
        let mut raw = self.raw.clone();
        if let Some(obj) = raw.as_object_mut()
            && let Some(t) = &self.last_heartbeat
        {
            obj.insert(
                "last_heartbeat".into(),
                Value::String(crate::common::rfc3339_nano(t)),
            );
        }
        raw
    }

    #[cfg(test)]
    #[allow(clippy::too_many_arguments)]
    fn new(
        org: &str,
        id: &str,
        name: &str,
        kind: &str,
        attached: bool,
        connected: bool,
        runtime: &str,
        status: &str,
    ) -> Self {
        FleetEntry {
            organization_id: org.into(),
            cluster_id: id.into(),
            cluster_name: name.into(),
            cluster_kind: kind.into(),
            attached,
            connected,
            reported_runtime: runtime.into(),
            status: status.into(),
            ..Default::default()
        }
    }
}

/// `ClusterOperatorFleetInventoryResponseList` decoding.
fn decode_fleet(body: &[u8]) -> Result<Vec<FleetEntry>, String> {
    let v: Value = serde_json::from_slice(body).map_err(|e| e.to_string())?;
    let obj = v.as_object().ok_or_else(|| {
        "json: cannot unmarshal into Go value of type map[string]interface {}".to_string()
    })?;
    if !obj.contains_key("results") {
        return Err("no value given for required property results".to_string());
    }
    match &v["results"] {
        Value::Array(items) => items.iter().map(FleetEntry::decode).collect(),
        _ => Ok(Vec::new()),
    }
}

/// `displayVersion`.
fn display_version(v: &Option<String>) -> String {
    match v {
        Some(s) if !s.is_empty() => s.clone(),
        _ => "unknown".to_string(),
    }
}

/// `nullableStringValue`.
fn nullable(v: &Option<String>) -> String {
    v.clone().unwrap_or_default()
}

/// `displayRuntime`.
fn display_runtime(runtime: &str) -> String {
    if runtime.is_empty() {
        "UNKNOWN".to_string()
    } else {
        runtime.to_string()
    }
}

fn is_outdated(status: &str) -> bool {
    matches!(
        status,
        "OUTDATED_IMAGE" | "OUTDATED_CHART" | "OUTDATED_IMAGE_AND_CHART"
    )
}

/// Go `strconv.ParseBool`.
fn parse_bool(s: &str) -> Option<bool> {
    match s {
        "1" | "t" | "T" | "TRUE" | "true" | "True" => Some(true),
        "0" | "f" | "F" | "FALSE" | "false" | "False" => Some(false),
        _ => None,
    }
}

type ParsedFilters = BTreeMap<String, BTreeSet<String>>;

/// `parseClusterOperatorFilters`.
pub fn parse_filters(filters: &BTreeMap<String, String>) -> Result<ParsedFilters, String> {
    let mut parsed = ParsedFilters::new();
    for (key, value) in filters {
        if !FILTER_KEYS.contains(&key.as_str()) {
            return Err(format!(
                "filter property '{key}' not available: valid values are: {}",
                FILTER_KEYS.join(", ")
            ));
        }
        let mut values = BTreeSet::new();
        for item in value.split(',') {
            values.insert(normalize_filter_value(key, item.trim())?);
        }
        parsed.insert(key.clone(), values);
    }
    Ok(parsed)
}

fn normalize_filter_value(key: &str, value: &str) -> Result<String, String> {
    let statuses = fleet_statuses();
    let allowed: &[&str] = match key {
        "ClusterKind" => SELF_MANAGED_CLUSTER_KINDS,
        "Runtime" => OPERATOR_RUNTIMES,
        "Status" => &statuses,
        "Connected" => {
            return parse_bool(value)
                .map(crate::common::format_bool)
                .ok_or_else(|| format!("invalid value '{value}' for filter Connected: valid values are: true, false"));
        }
        _ => return Ok(value.to_string()),
    };
    if !allowed.contains(&value) {
        return Err(format!(
            "invalid value '{value}' for filter {key}: valid values are: {}",
            allowed.join(", ")
        ));
    }
    Ok(value.to_string())
}

fn filter_value(c: &FleetEntry, key: &str) -> String {
    match key {
        "OrganizationId" => c.organization_id.clone(),
        "ClusterId" => c.cluster_id.clone(),
        "ClusterName" => c.cluster_name.clone(),
        "ClusterKind" => c.cluster_kind.clone(),
        "Runtime" => c.reported_runtime.clone(),
        "Status" => c.status.clone(),
        "Connected" => crate::common::format_bool(c.connected),
        _ => String::new(),
    }
}

/// `selectClusterOperators`.
pub fn select(
    clusters: &[FleetEntry],
    filters: &BTreeMap<String, String>,
    outdated_only: bool,
) -> Result<Vec<FleetEntry>, String> {
    let parsed = parse_filters(filters)?;
    Ok(clusters
        .iter()
        .filter(|c| c.attached)
        .filter(|c| !outdated_only || is_outdated(&c.status))
        .filter(|c| {
            parsed
                .iter()
                .all(|(k, values)| values.contains(&filter_value(c, k)))
        })
        .cloned()
        .collect())
}

/// `sortClusterOperators` (insertion sort for the small slices of the CLI: stable).
fn sort_clusters(clusters: &mut [FleetEntry]) {
    clusters.sort_by(|a, b| {
        a.organization_id
            .cmp(&b.organization_id)
            .then_with(|| a.cluster_name.cmp(&b.cluster_name))
    });
}

fn organization_display_name(names: &BTreeMap<String, String>, organization_id: &str) -> String {
    match names.get(organization_id) {
        Some(n) if !n.is_empty() => n.clone(),
        _ => organization_id.to_string(),
    }
}

/// `clusterOperatorFleetRows`.
fn fleet_rows(clusters: &mut [FleetEntry], names: &BTreeMap<String, String>) -> Vec<Vec<String>> {
    sort_clusters(clusters);
    clusters
        .iter()
        .map(|c| {
            let last_heartbeat = c
                .last_heartbeat
                .as_ref()
                .map(crate::common::rfc3339)
                .unwrap_or_else(|| "never".to_string());
            let (mut reported_chart, mut desired_chart) = (
                display_version(&c.reported_chart_version),
                display_version(&c.desired_chart_version),
            );
            if c.reported_runtime == "DOCKER" {
                reported_chart = "-".to_string();
                desired_chart = "-".to_string();
            }
            vec![
                organization_display_name(names, &c.organization_id),
                c.cluster_id.clone(),
                c.cluster_name.clone(),
                c.cluster_kind.clone(),
                c.reported_runtime.clone(),
                crate::common::format_bool(c.attached),
                crate::common::format_bool(c.connected),
                last_heartbeat,
                c.status.clone(),
                display_version(&c.reported_image_version),
                display_version(&c.desired_image_version),
                reported_chart,
                desired_chart,
            ]
        })
        .collect()
}

fn organization_ids(clusters: &[FleetEntry]) -> Vec<String> {
    let mut ids: Vec<String> = Vec::new();
    for c in clusters {
        if !ids.contains(&c.organization_id) {
            ids.push(c.organization_id.clone());
        }
    }
    ids
}

/// `lookupOrganizationNames`: one bounded lookup per organization, errors ignored.
fn lookup_organization_names(
    ids: &[String],
    lookup: &dyn Fn(Duration, &str) -> Result<String, String>,
    overall: Duration,
    per_call: Duration,
) -> BTreeMap<String, String> {
    let mut names = BTreeMap::new();
    let deadline = Instant::now() + overall;
    for id in ids {
        let now = Instant::now();
        if now >= deadline {
            break;
        }
        let timeout = per_call.min(deadline - now);
        if let Ok(name) = lookup(timeout, id)
            && !name.is_empty()
        {
            names.insert(id.clone(), name);
        }
    }
    names
}

/// `organizationNameLookup`: `GET /organization/{id}` on the Qovery API.
fn organization_name(
    authorization: &str,
    timeout: Duration,
    organization_id: &str,
) -> Result<String, String> {
    let res = api_request(
        Method::GET,
        &format!("/organization/{organization_id}"),
        authorization,
        timeout,
    )?;
    if res.status >= 300 {
        return Err(res.status_line());
    }
    let v: Value = serde_json::from_slice(&res.body).map_err(|e| e.to_string())?;
    for p in ["id", "created_at", "name", "plan"] {
        if v.get(p).is_none() {
            return Err(format!("no value given for required property {p}"));
        }
    }
    Ok(v["name"].as_str().unwrap_or_default().to_string())
}

/// A Qovery API call bounded by a context deadline.
fn api_request(
    method: Method,
    path_and_query: &str,
    authorization: &str,
    timeout: Duration,
) -> Result<HttpResponse, String> {
    let url = format!("{}{}", qovery_api::api_base_url(), path_and_query);
    let ua = format!("CLI {}", qovery_core::version::VERSION);
    http_request_ctx(
        method,
        &url,
        &[
            ("Authorization", authorization),
            ("User-Agent", &ua),
            ("Accept", "application/json"),
        ],
        None,
        timeout,
    )
}

/// `httpStatusError` and the other errors of the admin calls.
#[derive(Debug, Clone)]
pub struct OpError {
    pub unauthorized: bool,
    pub message: String,
}

impl std::fmt::Display for OpError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl From<String> for OpError {
    fn from(message: String) -> Self {
        OpError {
            unauthorized: false,
            message,
        }
    }
}

fn http_status_error(operation: &str, res: &HttpResponse) -> OpError {
    let body = &res.body[..res.body.len().min(1 << 20)];
    OpError {
        unauthorized: res.status == 401,
        message: format!(
            "{operation} returned {}: {}",
            res.status_line(),
            String::from_utf8_lossy(body).trim()
        ),
    }
}

/// `getClusterOperatorFleet`.
fn get_fleet(
    admin_url: &str,
    authorization: &str,
    timeout: Option<Duration>,
    context: bool,
) -> Result<Vec<FleetEntry>, OpError> {
    let url = format!("{}/operator/clusters", admin_url.trim_end_matches('/'));
    let headers = [
        ("Authorization", authorization),
        ("Accept", "application/json"),
    ];
    let res = match (timeout, context) {
        (Some(t), true) => http_request_ctx(Method::GET, &url, &headers, None, t)?,
        (t, _) => http_request(Method::GET, &url, &headers, None, t)?,
    };
    if res.status != 200 {
        return Err(http_status_error("operator fleet API", &res));
    }
    Ok(decode_fleet(&res.body)?)
}

fn run_list(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    let filters = ctx.string_map("filters");
    let fail = |e: String| -> ! {
        qovery_log::println_error_to_stderr(e);
        qovery_core::exit(1)
    };
    if let Err(e) = parse_filters(&filters) {
        fail(e);
    }
    let (tt, token) = qovery_core::auth::get_access_token(false).unwrap_or_else(|e| fail(e));
    let authorization = qovery_core::auth::authorization_header_value(tt, &token);
    let fleet = get_fleet(
        &crate::common::admin_url(),
        &authorization,
        Some(Duration::from_secs(60)),
        false,
    )
    .unwrap_or_else(|e| fail(e.message));
    let mut clusters = select(&fleet, &filters, ctx.bool("outdated")).unwrap_or_else(|e| fail(e));
    if ctx.bool("json") {
        let v = Value::Array(clusters.iter().map(FleetEntry::to_json).collect());
        println(go_json_indent(&v, "  "));
        return;
    }
    let names = lookup_organization_names(
        &organization_ids(&clusters),
        &|timeout, id| organization_name(&authorization, timeout, id),
        Duration::from_secs(15),
        Duration::from_secs(10),
    );
    let rows = fleet_rows(&mut clusters, &names);
    qovery_log::print_table(
        &[
            "Organization",
            "Cluster ID",
            "Cluster",
            "Kind",
            "Runtime",
            "Attached",
            "Connected",
            "Last heartbeat",
            "Status",
            "Image",
            "Target image",
            "Chart",
            "Target chart",
        ],
        &rows,
    );
}

// ---------------------------------------------------------------------------------------------------------------
// update

#[derive(Debug, Clone, Default)]
pub struct Target {
    pub image_version: String,
    pub chart_version: String,
}

impl Target {
    fn explicit(&self) -> bool {
        !self.image_version.is_empty() || !self.chart_version.is_empty()
    }
}

/// `qovery.ClusterOperatorUpdateRequest`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct UpdateRequest {
    pub image_version: Option<String>,
    pub chart_version: Option<String>,
}

impl UpdateRequest {
    fn to_json(&self) -> String {
        let mut m = serde_json::Map::new();
        if let Some(c) = &self.chart_version {
            m.insert("chart_version".into(), Value::String(c.clone()));
        }
        if let Some(i) = &self.image_version {
            m.insert("image_version".into(), Value::String(i.clone()));
        }
        qovery_log::gofmt::go_json(&Value::Object(m))
    }
}

/// `qovery.ClusterOperatorUpdateResponse`.
#[derive(Debug, Clone, Default)]
pub struct UpdateResponse {
    pub execution_id: String,
    pub image_version: String,
    pub chart_version: Option<String>,
}

#[derive(Debug, Clone)]
pub struct PlanItem {
    pub cluster: FleetEntry,
    pub request: Option<UpdateRequest>,
    pub skip_reason: String,
}

/// `clusterOperatorUpdateEligibility`.
pub fn eligibility(cluster: &FleetEntry, target: &Target) -> (Option<UpdateRequest>, String) {
    if !cluster.connected {
        return (None, "operator disconnected".into());
    }
    if !matches!(cluster.reported_runtime.as_str(), "KUBERNETES" | "DOCKER") {
        return (
            None,
            format!(
                "runtime {} not supported",
                display_runtime(&cluster.reported_runtime)
            ),
        );
    }
    let request = build_request(cluster, target);
    if cluster.reported_runtime == "KUBERNETES" && request.chart_version.is_none() {
        return (None, "no target chart version".into());
    }
    if !target.explicit() {
        if !is_outdated(&cluster.status) {
            return (None, format!("status {}", cluster.status));
        }
        return (Some(request), String::new());
    }
    if cluster.reported_runtime == "DOCKER" && target.image_version.is_empty() {
        return (None, "chart version does not apply to DOCKER".into());
    }
    if cluster.reported_runtime == "DOCKER"
        && !docker_heartbeat_can_confirm_image(
            &target.image_version,
            &cluster.desired_image_version,
        )
    {
        return (
            None,
            "DOCKER heartbeat cannot confirm a non-release image tag".into(),
        );
    }
    if !differs_from_target(cluster, target) {
        return (None, "already on requested version".into());
    }
    (Some(request), String::new())
}

/// `releaseImageVersionPattern`: `^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`.
fn is_release_image_version(v: &str) -> bool {
    let Some(rest) = v.strip_prefix('v') else {
        return false;
    };
    let (core, pre) = match rest.split_once('-') {
        Some((c, p)) => (c, Some(p)),
        None => (rest, None),
    };
    let parts: Vec<&str> = core.split('.').collect();
    if parts.len() != 3
        || parts
            .iter()
            .any(|p| p.is_empty() || !p.chars().all(|c| c.is_ascii_digit()))
    {
        return false;
    }
    match pre {
        None => true,
        Some(p) => {
            !p.is_empty()
                && p.chars()
                    .all(|c| c.is_ascii_alphanumeric() || c == '.' || c == '-')
        }
    }
}

fn docker_heartbeat_can_confirm_image(image_version: &str, desired: &Option<String>) -> bool {
    image_version == nullable(desired) || is_release_image_version(image_version)
}

fn differs_from_target(cluster: &FleetEntry, target: &Target) -> bool {
    let image = effective_image_version(cluster, target);
    if !image.is_empty() && nullable(&cluster.reported_image_version) != image {
        return true;
    }
    let chart = effective_chart_version(cluster, target);
    !chart.is_empty() && nullable(&cluster.reported_chart_version) != chart
}

fn effective_image_version(cluster: &FleetEntry, target: &Target) -> String {
    if !target.image_version.is_empty() {
        target.image_version.clone()
    } else {
        nullable(&cluster.desired_image_version)
    }
}

fn effective_chart_version(cluster: &FleetEntry, target: &Target) -> String {
    if cluster.reported_runtime != "KUBERNETES" {
        return String::new();
    }
    if !target.chart_version.is_empty() {
        target.chart_version.clone()
    } else {
        nullable(&cluster.desired_chart_version)
    }
}

fn build_request(cluster: &FleetEntry, target: &Target) -> UpdateRequest {
    let chart = effective_chart_version(cluster, target);
    UpdateRequest {
        image_version: (!target.image_version.is_empty()).then(|| target.image_version.clone()),
        chart_version: (!chart.is_empty()).then_some(chart),
    }
}

pub fn plan_updates(clusters: &[FleetEntry], target: &Target) -> Vec<PlanItem> {
    clusters
        .iter()
        .map(|c| {
            let (request, skip_reason) = eligibility(c, target);
            PlanItem {
                cluster: c.clone(),
                request,
                skip_reason,
            }
        })
        .collect()
}

fn eligible(plan: &[PlanItem]) -> Vec<PlanItem> {
    plan.iter()
        .filter(|p| p.request.is_some())
        .cloned()
        .collect()
}

/// `clusterOperatorUpdatePlanRows`.
fn plan_rows(
    plan: &[PlanItem],
    image_version: &str,
    chart_version: &str,
    names: &BTreeMap<String, String>,
) -> Vec<Vec<String>> {
    plan.iter()
        .map(|item| {
            let c = &item.cluster;
            let target_image = if image_version.is_empty() {
                display_version(&c.desired_image_version)
            } else {
                image_version.to_string()
            };
            let chart = if c.reported_runtime != "DOCKER" {
                let target_chart = if chart_version.is_empty() {
                    display_version(&c.desired_chart_version)
                } else {
                    chart_version.to_string()
                };
                format!(
                    "{} -> {}",
                    display_version(&c.reported_chart_version),
                    target_chart
                )
            } else {
                "-".to_string()
            };
            let action = if item.request.is_none() {
                format!("skip: {}", item.skip_reason)
            } else {
                "update".to_string()
            };
            vec![
                organization_display_name(names, &c.organization_id),
                c.cluster_id.clone(),
                c.cluster_name.clone(),
                c.cluster_kind.clone(),
                display_runtime(&c.reported_runtime),
                format!(
                    "{} -> {}",
                    display_version(&c.reported_image_version),
                    target_image
                ),
                chart,
                action,
            ]
        })
        .collect()
}

pub struct Options<'a> {
    pub parallel_runs: i64,
    pub refresh_delay: Duration,
    pub timeout: Duration,
    pub poll_timeout: Duration,
    pub expiry_grace: Duration,
    pub organization_names: BTreeMap<String, String>,
    pub print: &'a dyn Fn(String),
}

/// `newClusterOperatorUpdateOptions` (bounds only).
pub fn validate_options(
    parallel_runs: i64,
    refresh_delay: i64,
    timeout_nanos: i64,
) -> Result<(), String> {
    if !(1..=10).contains(&parallel_runs) {
        return Err(format!(
            "--parallel-run must be between 1 and 10, got {parallel_runs}"
        ));
    }
    if !(5..=120).contains(&refresh_delay) {
        return Err(format!(
            "--refresh-delay must be between 5 and 120 seconds, got {refresh_delay}"
        ));
    }
    // a signed time.Duration: `--timeout=-5m` reports `got -5m0s`
    if timeout_nanos <= 0 {
        return Err(format!(
            "--timeout must be positive, got {}",
            duration_string_nanos(timeout_nanos)
        ));
    }
    Ok(())
}

/// One deployment history entry (`qovery.ClusterDeploymentHistory`).
#[derive(Debug, Clone)]
pub struct HistoryEntry {
    pub execution_id: Option<String>,
    pub status: String,
}

/// The calls made by the update runner (`clusterOperatorUpdateAPI`). `timeout` is the time left before the
/// deadline of the Go context of the call.
pub trait UpdateApi {
    fn update(
        &self,
        timeout: Duration,
        cluster_id: &str,
        request: &UpdateRequest,
    ) -> Result<UpdateResponse, OpError>;
    fn fleet(&self, timeout: Option<Duration>) -> Result<Vec<FleetEntry>, OpError>;
    fn history(
        &self,
        timeout: Duration,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<Vec<HistoryEntry>, OpError>;
}

#[derive(Debug, Clone)]
pub struct UpdateResult {
    pub cluster: FleetEntry,
    pub started: bool,
    pub execution_id: String,
    pub err: Option<String>,
}

pub struct Run {
    index: usize,
    cluster: FleetEntry,
    response: UpdateResponse,
    deadline: Instant,
    started_at: Instant,
    execution_status: String,
    execution_finished: bool,
    reported: Option<FleetEntry>,
    last_poll_error: Option<String>,
}

#[derive(Default)]
struct Snapshot {
    fetched: bool,
    clusters: Vec<FleetEntry>,
    err: Option<String>,
}

fn left(deadline: Instant) -> Duration {
    deadline.saturating_duration_since(Instant::now())
}

/// `runClusterOperatorUpdates`.
pub fn run_updates(
    api: &dyn UpdateApi,
    updates: &[PlanItem],
    options: &Options,
) -> Vec<UpdateResult> {
    let mut results: Vec<UpdateResult> = updates
        .iter()
        .map(|u| UpdateResult {
            cluster: u.cluster.clone(),
            started: false,
            execution_id: String::new(),
            err: None,
        })
        .collect();
    let mut running: Vec<Run> = Vec::new();
    let mut next = 0;
    let mut failed = false;
    loop {
        while !failed && next < updates.len() && (running.len() as i64) < options.parallel_runs {
            let index = next;
            next += 1;
            results[index].started = true;
            match start_update(api, index, &updates[index], options) {
                Err(e) => {
                    failed = true;
                    (options.print)(format!(
                        "{} failed: {}",
                        log_prefix(&updates[index].cluster, &options.organization_names),
                        e
                    ));
                    results[index].err = Some(e);
                    continue;
                }
                Ok(run) => {
                    results[index].execution_id = run.response.execution_id.clone();
                    running.push(run);
                }
            }
        }
        if running.is_empty() {
            return results;
        }
        (options.print)(format!(
            "Checking Operator updates in {} seconds",
            options.refresh_delay.as_secs()
        ));
        std::thread::sleep(options.refresh_delay);

        let mut snapshot = Snapshot::default();
        let mut still_running = Vec::new();
        for mut run in running {
            let (done, err) = poll_update(api, &mut run, &mut snapshot, options);
            if !done {
                still_running.push(run);
                continue;
            }
            if let Some(e) = &err {
                (options.print)(format!(
                    "{} failed: {}",
                    log_prefix(&run.cluster, &options.organization_names),
                    e
                ));
                failed = true;
            }
            results[run.index].err = err;
        }
        running = still_running;
    }
}

fn start_update(
    api: &dyn UpdateApi,
    index: usize,
    item: &PlanItem,
    options: &Options,
) -> Result<Run, String> {
    let deadline = Instant::now() + options.timeout;
    let cluster = &item.cluster;
    let request = item.request.clone().unwrap_or_default();
    let response = api
        .update(left(deadline), &cluster.cluster_id, &request)
        .map_err(|e| format!("update request failed: {e}"))?;
    (options.print)(format!(
        "{} - Starting Operator update {} -> {} - execution {} - https://console.qovery.com/organization/{}/cluster/{}/cluster-logs",
        log_prefix(cluster, &options.organization_names),
        version_pair(
            &cluster.reported_runtime,
            &nullable(&cluster.reported_image_version),
            &nullable(&cluster.reported_chart_version)
        ),
        version_pair(
            &cluster.reported_runtime,
            &response.image_version,
            &nullable(&response.chart_version)
        ),
        response.execution_id,
        cluster.organization_id,
        cluster.cluster_id,
    ));
    Ok(Run {
        index,
        cluster: cluster.clone(),
        response,
        deadline,
        started_at: Instant::now(),
        execution_status: String::new(),
        execution_finished: false,
        reported: None,
        last_poll_error: None,
    })
}

/// `pollClusterOperatorUpdate`.
fn poll_update(
    api: &dyn UpdateApi,
    run: &mut Run,
    snapshot: &mut Snapshot,
    options: &Options,
) -> (bool, Option<String>) {
    let prefix = log_prefix(&run.cluster, &options.organization_names);
    let expired = Instant::now() >= run.deadline;
    let timeout_error = |run: &Run| {
        timeout_error(
            options.timeout,
            &run.response.execution_id,
            run.execution_finished,
            run.last_poll_error.as_deref(),
        )
    };
    let grace = expired.then(|| Instant::now() + options.expiry_grace);
    let history_parent = grace.unwrap_or(run.deadline);
    let run_expired = |run: &Run| Instant::now() >= run.deadline;

    if !run.execution_finished {
        let timeout = left(history_parent).min(options.poll_timeout);
        match api.history(
            timeout,
            &run.cluster.organization_id,
            &run.cluster.cluster_id,
        ) {
            Err(e) => {
                if expired || run_expired(run) {
                    return (true, Some(timeout_error(run)));
                }
                run.last_poll_error = Some(e.message);
                (options.print)(run.progress_line(&prefix, run.started_at.elapsed()));
                return (false, None);
            }
            Ok(history) => {
                run.last_poll_error = None;
                if let Some(status) = execution_status(&history, &run.response.execution_id) {
                    run.execution_status = status;
                }
                let (finished, err) = evaluate_execution(&history, &run.response.execution_id);
                if err.is_some() {
                    return (true, err);
                }
                if !finished {
                    if expired {
                        return (true, Some(timeout_error(run)));
                    }
                    (options.print)(run.progress_line(&prefix, run.started_at.elapsed()));
                    return (false, None);
                }
                run.execution_finished = true;
                (options.print)(format!(
                    "{prefix} execution {} finished, waiting for the Operator to report the target version",
                    run.response.execution_id
                ));
            }
        }
    }

    if !snapshot.fetched {
        let timeout = match grace {
            Some(g) => left(g).min(options.poll_timeout),
            None => options.poll_timeout,
        };
        match api.fleet(Some(timeout)) {
            Ok(c) => snapshot.clusters = c,
            Err(e) => snapshot.err = Some(e.message),
        }
        snapshot.fetched = true;
    }
    if let Some(e) = &snapshot.err {
        run.last_poll_error = Some(e.clone());
        if expired || run_expired(run) {
            return (true, Some(timeout_error(run)));
        }
        (options.print)(run.progress_line(&prefix, run.started_at.elapsed()));
        return (false, None);
    }
    run.last_poll_error = None;
    run.reported = snapshot
        .clusters
        .iter()
        .find(|c| c.cluster_id == run.cluster.cluster_id)
        .cloned();
    if reports_target(&snapshot.clusters, &run.cluster.cluster_id, &run.response) {
        (options.print)(format!(
            "{prefix} Operator reports {}",
            update_versions(&run.response)
        ));
        return (true, None);
    }
    if expired || run_expired(run) {
        return (true, Some(timeout_error(run)));
    }
    (options.print)(run.progress_line(&prefix, run.started_at.elapsed()));
    (false, None)
}

/// Go `time.Duration.Round(time.Second)`.
fn round_seconds(d: Duration) -> Duration {
    let ms = d.as_millis();
    Duration::from_secs(((ms + 500) / 1000) as u64)
}

impl Run {
    fn progress_line(&self, prefix: &str, elapsed: Duration) -> String {
        let elapsed_text = duration_string(round_seconds(elapsed));
        let mut line = if self.execution_finished {
            let reported = match &self.reported {
                Some(r) => {
                    let mut s = version_pair(
                        &self.cluster.reported_runtime,
                        &nullable(&r.reported_image_version),
                        &nullable(&r.reported_chart_version),
                    );
                    if !r.connected {
                        s.push_str(" (disconnected)");
                    }
                    s
                }
                None => "unknown".to_string(),
            };
            let target = version_pair(
                &self.cluster.reported_runtime,
                &self.response.image_version,
                &nullable(&self.response.chart_version),
            );
            format!(
                "{prefix} - waiting for the Operator heartbeat: reports {reported}, target {target} - {elapsed_text} elapsed"
            )
        } else if self.execution_status.is_empty() {
            format!(
                "{prefix} - execution {} not in the deployment history yet - {elapsed_text} elapsed",
                self.response.execution_id
            )
        } else {
            format!(
                "{prefix} - execution {} {} - {elapsed_text} elapsed",
                self.response.execution_id, self.execution_status
            )
        };
        if let Some(e) = &self.last_poll_error {
            line.push_str(&format!(" - last poll error: {e}"));
        }
        line
    }
}

fn execution_status(history: &[HistoryEntry], execution_id: &str) -> Option<String> {
    history
        .iter()
        .find(|e| nullable(&e.execution_id) == execution_id)
        .map(|e| e.status.clone())
}

use qovery_services::is_terminal_state;

/// `evaluateClusterOperatorUpdateExecution`.
pub fn evaluate_execution(history: &[HistoryEntry], execution_id: &str) -> (bool, Option<String>) {
    for entry in history {
        if nullable(&entry.execution_id) != execution_id {
            continue;
        }
        if entry.status == "DEPLOYED" {
            return (true, None);
        }
        if is_terminal_state(&entry.status) {
            return (
                true,
                Some(format!(
                    "execution {execution_id} ended with status {}",
                    entry.status
                )),
            );
        }
        return (false, None);
    }
    (false, None)
}

/// `clusterOperatorReportsTarget`.
pub fn reports_target(fleet: &[FleetEntry], cluster_id: &str, response: &UpdateResponse) -> bool {
    for c in fleet {
        if c.cluster_id != cluster_id {
            continue;
        }
        if !c.connected || nullable(&c.reported_image_version) != response.image_version {
            return false;
        }
        if c.reported_runtime == "KUBERNETES" {
            return nullable(&c.reported_chart_version) == nullable(&response.chart_version);
        }
        return true;
    }
    false
}

/// `clusterOperatorVersionPair`.
fn version_pair(runtime: &str, image: &str, chart: &str) -> String {
    let image = if image.is_empty() { "unknown" } else { image };
    if runtime != "KUBERNETES" {
        return image.to_string();
    }
    let chart = if chart.is_empty() { "unknown" } else { chart };
    format!("{image}/{chart}")
}

/// `clusterOperatorUpdateParameters`.
pub fn update_parameters(
    parallel_runs: i64,
    refresh_delay: Duration,
    timeout: Duration,
    target: &Target,
) -> Vec<String> {
    let image = if target.image_version.is_empty() {
        "q-core target".to_string()
    } else {
        target.image_version.clone()
    };
    let chart = if target.chart_version.is_empty() {
        "desired chart (KUBERNETES only)".to_string()
    } else {
        target.chart_version.clone()
    };
    vec![
        "-------------------------------------------".to_string(),
        "- DryRunDisabled: true".to_string(),
        format!("- ParallelRun: {parallel_runs}"),
        format!("- RefreshDelay: {} seconds", refresh_delay.as_secs()),
        format!("- Timeout: {}", duration_string(timeout)),
        format!("- ImageVersion: {image}"),
        format!("- ChartVersion: {chart}"),
        "-------------------------------------------".to_string(),
    ]
}

/// `clusterOperatorUpdateTimeoutError`.
pub fn timeout_error(
    timeout: Duration,
    execution_id: &str,
    execution_finished: bool,
    last_poll_error: Option<&str>,
) -> String {
    let mut message = if execution_finished {
        format!(
            "timed out after {} waiting for the Operator to report the target version",
            duration_string(timeout)
        )
    } else {
        format!(
            "timed out after {} waiting for execution {execution_id}",
            duration_string(timeout)
        )
    };
    if let Some(e) = last_poll_error {
        message.push_str(&format!(" (last poll error: {e})"));
    }
    message
}

fn update_versions(response: &UpdateResponse) -> String {
    match &response.chart_version {
        Some(c) if !c.is_empty() => format!("image {}, chart {c}", response.image_version),
        _ => format!("image {}", response.image_version),
    }
}

/// `clusterOperatorLogPrefix`.
fn log_prefix(cluster: &FleetEntry, names: &BTreeMap<String, String>) -> String {
    format!(
        "[Organization '{}' - Cluster '{}']",
        organization_display_name(names, &cluster.organization_id),
        cluster.cluster_name
    )
}

/// `printClusterOperatorUpdateSummary`.
fn print_summary(results: &[UpdateResult], names: &BTreeMap<String, String>) -> bool {
    let mut failed = false;
    let rows: Vec<Vec<String>> = results
        .iter()
        .map(|r| {
            let outcome = if !r.started {
                "not started".to_string()
            } else if let Some(e) = &r.err {
                failed = true;
                format!("failed: {e}")
            } else {
                "succeeded".to_string()
            };
            vec![
                organization_display_name(names, &r.cluster.organization_id),
                r.cluster.cluster_id.clone(),
                r.cluster.cluster_name.clone(),
                r.execution_id.clone(),
                outcome,
            ]
        })
        .collect();
    println("");
    qovery_log::print_table(
        &[
            "Organization",
            "Cluster ID",
            "Cluster",
            "Execution ID",
            "Result",
        ],
        &rows,
    );
    failed
}

/// `adminCredentials` + `withAdminReauthentication`.
struct Credentials {
    authorization: RefCell<String>,
    browser_reauthentication: bool,
    browser_reauthenticated: RefCell<bool>,
}

impl Credentials {
    fn with_reauthentication<T>(
        &self,
        call: impl Fn(&str) -> Result<T, OpError>,
    ) -> Result<T, OpError> {
        let stale = self.authorization.borrow().clone();
        let result = call(&stale);
        match &result {
            Err(e) if e.unauthorized => {}
            _ => return result,
        }
        let err = result.err().unwrap();
        // credentials.refresh
        let reloaded = crate::common::authorization();
        match reloaded {
            Ok(a) if a != stale => *self.authorization.borrow_mut() = a,
            _ => {
                if !self.browser_reauthentication || *self.browser_reauthenticated.borrow() {
                    return Err(err);
                }
                *self.browser_reauthenticated.borrow_mut() = true;
                println("Needs to re-authenticate as the response is UNAUTHORIZED (401)");
                qovery_cmd_auth::service::do_request_user_to_authenticate(false, true);
                let a = crate::common::authorization().map_err(OpError::from)?;
                *self.authorization.borrow_mut() = a;
            }
        }
        let current = self.authorization.borrow().clone();
        call(&current)
    }
}

struct AdminUpdateApi {
    admin_url: String,
    credentials: Credentials,
}

impl UpdateApi for AdminUpdateApi {
    fn update(
        &self,
        timeout: Duration,
        cluster_id: &str,
        request: &UpdateRequest,
    ) -> Result<UpdateResponse, OpError> {
        self.credentials.with_reauthentication(|authorization| {
            let url = format!(
                "{}/cluster/operator/update/{}",
                self.admin_url.trim_end_matches('/'),
                url_path_escape(cluster_id)
            );
            let res = http_request_ctx(
                Method::POST,
                &url,
                &[
                    ("Authorization", authorization),
                    ("Content-Type", "application/json"),
                    ("Accept", "application/json"),
                ],
                Some(request.to_json().into_bytes()),
                timeout.min(Duration::from_secs(60)),
            )?;
            if res.status != 202 {
                return Err(http_status_error("operator update API", &res));
            }
            let v: Value = serde_json::from_slice(&res.body).map_err(|e| e.to_string())?;
            for p in ["execution_id", "image_version"] {
                if v.get(p).is_none() {
                    return Err(format!("no value given for required property {p}").into());
                }
            }
            Ok(UpdateResponse {
                execution_id: v["execution_id"].as_str().unwrap_or_default().to_string(),
                image_version: v["image_version"].as_str().unwrap_or_default().to_string(),
                chart_version: v
                    .get("chart_version")
                    .and_then(|c| c.as_str())
                    .map(|c| c.to_string()),
            })
        })
    }

    fn fleet(&self, timeout: Option<Duration>) -> Result<Vec<FleetEntry>, OpError> {
        self.credentials
            .with_reauthentication(|authorization| match timeout {
                Some(t) => get_fleet(
                    &self.admin_url,
                    authorization,
                    Some(t.min(Duration::from_secs(60))),
                    true,
                ),
                None => get_fleet(
                    &self.admin_url,
                    authorization,
                    Some(Duration::from_secs(60)),
                    false,
                ),
            })
    }

    fn history(
        &self,
        timeout: Duration,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<Vec<HistoryEntry>, OpError> {
        self.credentials.with_reauthentication(|authorization| {
            let res = api_request(
                Method::GET,
                &format!("/organization/{organization_id}/cluster/{cluster_id}/deploymentHistoryV2?pageSize=10"),
                authorization,
                timeout,
            )?;
            if res.status == 401 {
                return Err(OpError {
                    unauthorized: true,
                    message: format!("cluster deployment history API returned {}: ", res.status_line()),
                });
            }
            if res.status >= 300 {
                return Err(res.status_line().into());
            }
            decode_history(&res.body).map_err(OpError::from)
        })
    }
}

fn decode_history(body: &[u8]) -> Result<Vec<HistoryEntry>, String> {
    if body.is_empty() {
        return Ok(Vec::new());
    }
    let v: Value = serde_json::from_slice(body).map_err(|e| e.to_string())?;
    for p in ["page", "page_size"] {
        if v.get(p).is_none() {
            return Err(format!("no value given for required property {p}"));
        }
    }
    let mut out = Vec::new();
    for e in v
        .get("results")
        .and_then(|r| r.as_array())
        .cloned()
        .unwrap_or_default()
    {
        for p in [
            "identifier",
            "auditing_data",
            "status",
            "action_status",
            "trigger_action",
            "operation",
            "is_dry_run",
            "reason",
        ] {
            if e.get(p).is_none() {
                return Err(format!("no value given for required property {p}"));
            }
        }
        for p in ["deployment_id", "cluster_id"] {
            if e["identifier"].get(p).is_none() {
                return Err(format!("no value given for required property {p}"));
            }
        }
        out.push(HistoryEntry {
            execution_id: e["identifier"]
                .get("execution_id")
                .and_then(|x| x.as_str())
                .map(|x| x.to_string()),
            status: e["status"].as_str().unwrap_or_default().to_string(),
        });
    }
    Ok(out)
}

/// Go `url.PathEscape`.
fn url_path_escape(s: &str) -> String {
    let mut out = String::new();
    for b in s.bytes() {
        if b.is_ascii_alphanumeric()
            || matches!(
                b,
                b'-' | b'_' | b'.' | b'~' | b'$' | b'&' | b'+' | b',' | b':' | b'=' | b'@'
            )
        {
            out.push(b as char);
        } else {
            out.push_str(&format!("%{b:02X}"));
        }
    }
    out
}

fn run_update(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    let parallel_runs = ctx.int("parallel-run");
    let refresh_delay = ctx.int("refresh-delay");
    if let Err(e) = validate_options(parallel_runs, refresh_delay, ctx.duration_nanos("timeout")) {
        qovery_core::exit_with_error(e);
    }
    let timeout = ctx.duration("timeout");
    let filters = ctx.string_map("filters");
    if let Err(e) = parse_filters(&filters) {
        qovery_core::exit_with_error(e);
    }
    let authorization =
        crate::common::authorization().unwrap_or_else(|e| qovery_core::exit_with_error(e));
    let api = AdminUpdateApi {
        admin_url: crate::common::admin_url(),
        credentials: Credentials {
            authorization: RefCell::new(authorization),
            browser_reauthentication: qovery_core::auth::access_token_from_env().is_empty(),
            browser_reauthenticated: RefCell::new(false),
        },
    };
    let fleet = api
        .fleet(None)
        .unwrap_or_else(|e| qovery_core::exit_with_error(e));
    let mut clusters =
        select(&fleet, &filters, false).unwrap_or_else(|e| qovery_core::exit_with_error(e));
    sort_clusters(&mut clusters);
    let names = {
        let authorization = api.credentials.authorization.borrow().clone();
        lookup_organization_names(
            &organization_ids(&clusters),
            &|timeout, id| organization_name(&authorization, timeout, id),
            Duration::from_secs(15),
            Duration::from_secs(10),
        )
    };
    let target = Target {
        image_version: ctx.string("image-version"),
        chart_version: ctx.string("chart-version"),
    };
    let plan = plan_updates(&clusters, &target);
    qovery_log::print_table(
        &[
            "Organization",
            "Cluster ID",
            "Cluster",
            "Kind",
            "Runtime",
            "Image",
            "Chart",
            "Action",
        ],
        &plan_rows(&plan, &target.image_version, &target.chart_version, &names),
    );
    let updates = eligible(&plan);
    println(format!(
        "{} cluster(s) to update, {} skipped",
        updates.len(),
        plan.len() - updates.len()
    ));
    if updates.is_empty() {
        return;
    }
    if !ctx.bool("disable-dry-run") {
        println("Dry run: no update triggered. Use --disable-dry-run (-y) to execute.");
        return;
    }
    let refresh = Duration::from_secs(refresh_delay as u64);
    for line in update_parameters(parallel_runs, refresh, timeout, &target) {
        println(line);
    }
    if !ctx.bool("no-confirm") {
        println("Do you want to continue the Operator update?");
        if !validate("operator update") {
            println("Exiting: Validation failed");
            return;
        }
    }
    let print = |s: String| println(s);
    let options = Options {
        parallel_runs,
        refresh_delay: refresh,
        timeout,
        poll_timeout: Duration::from_secs(60),
        expiry_grace: Duration::from_secs(10),
        organization_names: names.clone(),
        print: &print,
    };
    let results = run_updates(&api, &updates, &options);
    if print_summary(&results, &names) {
        qovery_core::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::Cell;

    fn fleet_entry(id: &str, runtime: &str, connected: bool, status: &str) -> FleetEntry {
        FleetEntry::new(
            &format!("org-{id}"),
            id,
            &format!("name-{id}"),
            "SELF_MANAGED",
            true,
            connected,
            runtime,
            status,
        )
    }

    fn operator_cluster(
        runtime: &str,
        connected: bool,
        status: &str,
        image: &str,
        chart: &str,
        desired_chart: &str,
    ) -> FleetEntry {
        let mut c = fleet_entry("cluster-1", runtime, connected, status);
        if !image.is_empty() {
            c.reported_image_version = Some(image.into());
        }
        if !chart.is_empty() {
            c.reported_chart_version = Some(chart.into());
        }
        c.desired_image_version = Some("v2".into());
        if !desired_chart.is_empty() {
            c.desired_chart_version = Some(desired_chart.into());
        }
        c
    }

    fn ids(c: &[FleetEntry]) -> Vec<String> {
        c.iter().map(|c| c.cluster_id.clone()).collect()
    }

    fn map(pairs: &[(&str, &str)]) -> BTreeMap<String, String> {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect()
    }

    #[test]
    fn fleet_rows_sort_and_format() {
        let hb = chrono::DateTime::parse_from_rfc3339("2026-08-18T12:28:46Z").unwrap();
        let mut current = FleetEntry::new(
            "org-1",
            "cluster-1",
            "customer-cluster",
            "SELF_MANAGED",
            true,
            true,
            "KUBERNETES",
            "CURRENT",
        );
        current.last_heartbeat = Some(hb);
        current.reported_image_version = Some("v1.203.0".into());
        current.desired_image_version = Some("v1.203.0".into());
        current.reported_chart_version = Some("0.2.1".into());
        current.desired_chart_version = Some("0.2.1".into());
        let disconnected = FleetEntry::new(
            "org-1",
            "cluster-2",
            "another-cluster",
            "EKS_SELF_MANAGED",
            true,
            false,
            "DOCKER",
            "DISCONNECTED",
        );
        let mut powens = FleetEntry::new(
            "org-2",
            "cluster-3",
            "powens",
            "EKS_ANYWHERE",
            true,
            true,
            "DOCKER",
            "CURRENT",
        );
        powens.last_heartbeat = Some(hb);
        powens.reported_image_version = Some("v1.203.0".into());
        powens.desired_image_version = Some("v1.203.0".into());
        powens.desired_chart_version = Some("0.2.3".into());
        let mut clusters = vec![powens, current, disconnected];
        let rows = fleet_rows(&mut clusters, &map(&[("org-1", "Acme")]));
        assert_eq!(rows.len(), 3);
        assert_eq!(rows[0][0], "Acme");
        assert_eq!(rows[1][0], "Acme");
        assert_eq!(
            rows[2],
            vec![
                "org-2",
                "cluster-3",
                "powens",
                "EKS_ANYWHERE",
                "DOCKER",
                "true",
                "true",
                "2026-08-18T12:28:46Z",
                "CURRENT",
                "v1.203.0",
                "v1.203.0",
                "-",
                "-"
            ]
        );
        assert_eq!(
            (
                rows[0][2].as_str(),
                rows[0][4].as_str(),
                rows[0][7].as_str(),
                rows[0][8].as_str()
            ),
            ("another-cluster", "DOCKER", "never", "DISCONNECTED")
        );
        assert_eq!(
            (
                rows[1][4].as_str(),
                rows[1][7].as_str(),
                rows[1][9].as_str(),
                rows[1][11].as_str()
            ),
            ("KUBERNETES", "2026-08-18T12:28:46Z", "v1.203.0", "0.2.1")
        );
    }

    #[test]
    #[allow(clippy::type_complexity)]
    fn select_cluster_operators() {
        let mut eks_anywhere = fleet_entry("c4", "KUBERNETES", true, "OUTDATED_IMAGE_AND_CHART");
        eks_anywhere.cluster_kind = "EKS_ANYWHERE".into();
        let mut not_attached = fleet_entry("c6", "DOCKER", true, "OUTDATED_IMAGE");
        not_attached.attached = false;
        let fleet = vec![
            fleet_entry("c1", "KUBERNETES", true, "CURRENT"),
            fleet_entry("c2", "DOCKER", true, "OUTDATED_IMAGE"),
            fleet_entry("c3", "UNKNOWN", false, "DISCONNECTED"),
            eks_anywhere,
            fleet_entry("c5", "KUBERNETES", true, "OUTDATED_CHART"),
            not_attached,
        ];
        let cases: Vec<(Vec<(&str, &str)>, bool, Vec<&str>)> = vec![
            (vec![], false, vec!["c1", "c2", "c3", "c4", "c5"]),
            (vec![("Runtime", "DOCKER")], false, vec!["c2"]),
            (
                vec![("Status", "CURRENT,OUTDATED_CHART")],
                false,
                vec!["c1", "c5"],
            ),
            (vec![("Connected", "false")], false, vec!["c3"]),
            (vec![("ClusterKind", "EKS_ANYWHERE")], false, vec!["c4"]),
            (
                vec![
                    ("OrganizationId", "org-c1,org-c2"),
                    ("ClusterName", "name-c2"),
                ],
                false,
                vec!["c2"],
            ),
            (vec![("ClusterId", "c5")], false, vec!["c5"]),
            (vec![], true, vec!["c2", "c4", "c5"]),
            (vec![("Runtime", "KUBERNETES")], true, vec!["c4", "c5"]),
            (
                vec![("Connected", "TRUE")],
                false,
                vec!["c1", "c2", "c4", "c5"],
            ),
        ];
        for (filters, outdated, expected) in cases {
            assert_eq!(
                ids(&select(&fleet, &map(&filters), outdated).unwrap()),
                expected,
                "{filters:?}"
            );
        }
    }

    #[test]
    fn select_rejects_invalid_filters() {
        for (filters, expected) in [
            (
                vec![("Runtime", "KUBERNETES,docker")],
                "invalid value 'docker' for filter Runtime: valid values are: KUBERNETES, DOCKER, UNKNOWN",
            ),
            (
                vec![("Status", "OUTDATED")],
                "invalid value 'OUTDATED' for filter Status: valid values are: NOT_ATTACHED,",
            ),
            (
                vec![("ClusterKind", "EKS")],
                "invalid value 'EKS' for filter ClusterKind: valid values are:",
            ),
            (
                vec![("Connected", "yes")],
                "invalid value 'yes' for filter Connected: valid values are: true, false",
            ),
        ] {
            let err = select(&[], &map(&filters), false).unwrap_err();
            assert!(err.starts_with(expected), "{err}");
        }
        assert!(
            select(&[], &map(&[("CurrentStatus", "DEPLOYED")]), false)
                .unwrap_err()
                .contains("CurrentStatus")
        );
    }

    #[test]
    fn filter_help_lists_every_value() {
        let help = filter_help();
        assert!(help.contains(&format!(
            "  {:<15} {}\n",
            "ClusterKind",
            SELF_MANAGED_CLUSTER_KINDS.join(", ")
        )));
        assert!(help.contains(&format!(
            "  {:<15} {}\n",
            "Runtime",
            OPERATOR_RUNTIMES.join(", ")
        )));
        for key in FILTER_KEYS {
            assert!(help.contains(&format!("  {key:<15} ")), "{key}");
        }
        for (status, _) in FLEET_STATUSES {
            assert!(help.contains(&format!("  {status:<25} ")), "{status}");
        }
    }

    #[test]
    fn update_eligibility() {
        let t = |i: &str, c: &str| Target {
            image_version: i.into(),
            chart_version: c.into(),
        };
        let none = Target::default();
        let cases: Vec<(FleetEntry, Target, &str, &str)> = vec![
            (
                operator_cluster("KUBERNETES", false, "DISCONNECTED", "v1", "0.1.0", "0.2.0"),
                none.clone(),
                "",
                "operator disconnected",
            ),
            (
                operator_cluster("UNKNOWN", true, "OUTDATED_IMAGE", "v1", "", ""),
                none.clone(),
                "",
                "runtime UNKNOWN not supported",
            ),
            (
                operator_cluster("", true, "OUTDATED_IMAGE", "v1", "", ""),
                none.clone(),
                "",
                "runtime UNKNOWN not supported",
            ),
            (
                operator_cluster("KUBERNETES", true, "OUTDATED_IMAGE", "v1", "0.2.0", "0.2.0"),
                none.clone(),
                r#"{"chart_version":"0.2.0"}"#,
                "",
            ),
            (
                operator_cluster("KUBERNETES", true, "CURRENT", "v2", "0.2.0", "0.2.0"),
                none.clone(),
                "",
                "status CURRENT",
            ),
            (
                operator_cluster("KUBERNETES", true, "OUTDATED_IMAGE", "v1", "0.2.0", ""),
                none.clone(),
                "",
                "no target chart version",
            ),
            (
                operator_cluster(
                    "DOCKER",
                    true,
                    "OUTDATED_IMAGE_AND_CHART",
                    "v1",
                    "",
                    "0.2.0",
                ),
                none.clone(),
                "{}",
                "",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v2", "", ""),
                none.clone(),
                "",
                "status CURRENT",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v1.3.0", "", ""),
                t("v1.3.0", ""),
                "",
                "already on requested version",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v2", "", ""),
                t("v1.3.0", "0.3.0"),
                r#"{"image_version":"v1.3.0"}"#,
                "",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v2", "", ""),
                t("v1.3.0-rc.1", ""),
                r#"{"image_version":"v1.3.0-rc.1"}"#,
                "",
            ),
            (
                operator_cluster("DOCKER", true, "OUTDATED_IMAGE", "v1", "", ""),
                t("v2", ""),
                r#"{"image_version":"v2"}"#,
                "",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v2", "", ""),
                t("main-3f2a1c", ""),
                "",
                "DOCKER heartbeat cannot confirm a non-release image tag",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v2", "", ""),
                t("v1.3.0+build", ""),
                "",
                "DOCKER heartbeat cannot confirm a non-release image tag",
            ),
            (
                operator_cluster("DOCKER", true, "CURRENT", "v2", "", ""),
                t("release-v1.3.0", ""),
                "",
                "DOCKER heartbeat cannot confirm a non-release image tag",
            ),
            (
                operator_cluster(
                    "KUBERNETES",
                    true,
                    "OUTDATED_CHART",
                    "v1.3.0",
                    "0.1.0",
                    "0.2.0",
                ),
                t("v1.3.0", ""),
                r#"{"chart_version":"0.2.0","image_version":"v1.3.0"}"#,
                "",
            ),
            (
                operator_cluster("KUBERNETES", true, "CURRENT", "v1.3.0", "0.2.0", "0.2.0"),
                t("v1.3.0", ""),
                "",
                "already on requested version",
            ),
            (
                operator_cluster("KUBERNETES", true, "OUTDATED_IMAGE", "v1", "0.3.0", "0.2.0"),
                t("", "0.3.0"),
                r#"{"chart_version":"0.3.0"}"#,
                "",
            ),
            (
                operator_cluster("KUBERNETES", true, "CURRENT", "v2", "0.3.0", "0.2.0"),
                t("", "0.3.0"),
                "",
                "already on requested version",
            ),
            (
                operator_cluster("KUBERNETES", true, "CURRENT", "v2", "0.2.0", "0.2.0"),
                t("main-3f2a1c", ""),
                r#"{"chart_version":"0.2.0","image_version":"main-3f2a1c"}"#,
                "",
            ),
            (
                operator_cluster("DOCKER", true, "OUTDATED_IMAGE", "v1", "", ""),
                t("", "0.3.0"),
                "",
                "chart version does not apply to DOCKER",
            ),
            (
                operator_cluster("KUBERNETES", true, "CURRENT", "v2", "0.2.0", "0.2.0"),
                t("", "0.3.0"),
                r#"{"chart_version":"0.3.0"}"#,
                "",
            ),
            (
                operator_cluster("KUBERNETES", true, "OUTDATED_IMAGE", "v3", "0.3.0", "0.2.0"),
                t("v3", "0.3.0"),
                "",
                "already on requested version",
            ),
            (
                operator_cluster("KUBERNETES", true, "CURRENT", "v2", "0.2.0", "0.2.0"),
                t("v3", ""),
                r#"{"chart_version":"0.2.0","image_version":"v3"}"#,
                "",
            ),
        ];
        for (cluster, target, request, skip) in cases {
            let (r, s) = eligibility(&cluster, &target);
            assert_eq!(s, skip, "{cluster:?} {target:?}");
            assert_eq!(
                r.map(|r| r.to_json()).unwrap_or_default(),
                request,
                "{cluster:?} {target:?}"
            );
        }
    }

    #[test]
    fn plan_keeps_skipped_clusters() {
        let updatable = operator_cluster("DOCKER", true, "OUTDATED_IMAGE", "v1", "", "");
        let disconnected = fleet_entry("cluster-2", "DOCKER", false, "DISCONNECTED");
        let plan = plan_updates(&[updatable, disconnected], &Target::default());
        let rows = plan_rows(&plan, "", "", &map(&[("org-cluster-1", "Acme")]));
        assert_eq!(eligible(&plan).len(), 1);
        assert_eq!(
            (rows[0][0].as_str(), rows[1][0].as_str()),
            ("Acme", "org-cluster-2")
        );
        assert_eq!(
            (
                rows[0][5].as_str(),
                rows[0][6].as_str(),
                rows[0][7].as_str()
            ),
            ("v1 -> v2", "-", "update")
        );
        assert_eq!(rows[1][7], "skip: operator disconnected");
    }

    fn history(exec: &str, status: &str) -> HistoryEntry {
        HistoryEntry {
            execution_id: Some(exec.into()),
            status: status.into(),
        }
    }

    #[test]
    fn evaluate_execution_cases() {
        assert_eq!(evaluate_execution(&[], "exec-1"), (false, None));
        assert_eq!(
            evaluate_execution(&[history("exec-0", "DEPLOYMENT_ERROR")], "exec-1"),
            (false, None)
        );
        assert_eq!(
            evaluate_execution(&[history("exec-1", "DEPLOYING")], "exec-1"),
            (false, None)
        );
        assert_eq!(
            evaluate_execution(&[history("exec-1", "DEPLOYED")], "exec-1"),
            (true, None)
        );
        assert!(
            evaluate_execution(&[history("exec-1", "DEPLOYMENT_ERROR")], "exec-1")
                .1
                .is_some()
        );
        assert!(
            evaluate_execution(&[history("exec-1", "CANCELED")], "exec-1")
                .1
                .is_some()
        );
    }

    #[test]
    fn reports_target_cases() {
        let k = UpdateResponse {
            execution_id: "exec-1".into(),
            image_version: "v3".into(),
            chart_version: Some("0.3.0".into()),
        };
        let d = UpdateResponse {
            execution_id: "exec-1".into(),
            image_version: "v3".into(),
            chart_version: None,
        };
        let c = |rt: &str, conn: bool, img: &str, chart: &str| {
            vec![operator_cluster(rt, conn, "CURRENT", img, chart, "")]
        };
        assert!(reports_target(
            &c("KUBERNETES", true, "v3", "0.3.0"),
            "cluster-1",
            &k
        ));
        assert!(!reports_target(
            &c("KUBERNETES", true, "v3", "0.2.0"),
            "cluster-1",
            &k
        ));
        assert!(!reports_target(
            &c("KUBERNETES", true, "v2", "0.3.0"),
            "cluster-1",
            &k
        ));
        assert!(!reports_target(
            &c("KUBERNETES", false, "v3", "0.3.0"),
            "cluster-1",
            &k
        ));
        assert!(reports_target(
            &c("DOCKER", true, "v3", ""),
            "cluster-1",
            &d
        ));
        assert!(!reports_target(&[], "cluster-1", &d));
    }

    #[derive(Default)]
    struct FakeApi {
        update_errors: BTreeMap<String, String>,
        history_failures: Cell<usize>,
        fleet_failures: Cell<usize>,
        history_status: String,
        deployed_after: usize,
        history_calls: Cell<usize>,
        reported_images: BTreeMap<String, String>,
        updated: RefCell<Vec<String>>,
    }

    impl UpdateApi for FakeApi {
        fn update(
            &self,
            _: Duration,
            cluster_id: &str,
            _: &UpdateRequest,
        ) -> Result<UpdateResponse, OpError> {
            self.updated.borrow_mut().push(cluster_id.into());
            if let Some(e) = self.update_errors.get(cluster_id) {
                return Err(e.clone().into());
            }
            Ok(UpdateResponse {
                execution_id: format!("exec-{cluster_id}"),
                image_version: "v3".into(),
                chart_version: None,
            })
        }
        fn fleet(&self, _: Option<Duration>) -> Result<Vec<FleetEntry>, OpError> {
            if self.fleet_failures.get() > 0 {
                self.fleet_failures.set(self.fleet_failures.get() - 1);
                return Err("fleet unavailable".to_string().into());
            }
            Ok(self
                .reported_images
                .iter()
                .map(|(id, image)| {
                    let mut c = fleet_entry(id, "DOCKER", true, "CURRENT");
                    c.reported_image_version = Some(image.clone());
                    c
                })
                .collect())
        }
        fn history(
            &self,
            _: Duration,
            _: &str,
            cluster_id: &str,
        ) -> Result<Vec<HistoryEntry>, OpError> {
            if self.history_failures.get() > 0 {
                self.history_failures.set(self.history_failures.get() - 1);
                return Err("history unavailable".to_string().into());
            }
            self.history_calls.set(self.history_calls.get() + 1);
            if self.deployed_after > 0 && self.history_calls.get() > self.deployed_after {
                return Ok(vec![history(&format!("exec-{cluster_id}"), "DEPLOYED")]);
            }
            Ok(vec![history(
                &format!("exec-{cluster_id}"),
                &self.history_status,
            )])
        }
    }

    fn docker_updates(ids: &[&str]) -> Vec<PlanItem> {
        ids.iter()
            .map(|id| PlanItem {
                cluster: fleet_entry(id, "DOCKER", true, "OUTDATED_IMAGE"),
                request: Some(UpdateRequest::default()),
                skip_reason: String::new(),
            })
            .collect()
    }

    fn options<'a>(timeout: Duration, print: &'a dyn Fn(String)) -> Options<'a> {
        Options {
            parallel_runs: 1,
            refresh_delay: Duration::from_millis(1),
            timeout,
            poll_timeout: Duration::from_secs(1),
            expiry_grace: Duration::from_secs(1),
            organization_names: BTreeMap::new(),
            print,
        }
    }

    fn images(pairs: &[(&str, &str)]) -> BTreeMap<String, String> {
        map(pairs)
    }

    #[test]
    fn run_succeeds_when_target_is_reported() {
        let api = FakeApi {
            history_status: "DEPLOYED".into(),
            reported_images: images(&[("c1", "v3"), ("c2", "v3")]),
            ..Default::default()
        };
        let results = run_updates(
            &api,
            &docker_updates(&["c1", "c2"]),
            &options(Duration::from_secs(60), &|_| {}),
        );
        for r in results {
            assert!(
                r.started
                    && r.err.is_none()
                    && r.execution_id == format!("exec-{}", r.cluster.cluster_id),
                "{r:?}"
            );
        }
    }

    #[test]
    fn run_fails_on_history_error() {
        let api = FakeApi {
            history_status: "DEPLOYMENT_ERROR".into(),
            reported_images: images(&[("c1", "v3")]),
            ..Default::default()
        };
        let results = run_updates(
            &api,
            &docker_updates(&["c1"]),
            &options(Duration::from_secs(60), &|_| {}),
        );
        assert!(
            results[0]
                .err
                .as_ref()
                .unwrap()
                .contains("DEPLOYMENT_ERROR")
        );
    }

    #[test]
    fn run_times_out_until_target_is_reported() {
        let api = FakeApi {
            history_status: "DEPLOYED".into(),
            reported_images: images(&[("c1", "v2")]),
            ..Default::default()
        };
        let results = run_updates(
            &api,
            &docker_updates(&["c1"]),
            &options(Duration::from_millis(50), &|_| {}),
        );
        assert!(results[0].err.as_ref().unwrap().contains("timed out"));
    }

    #[test]
    fn timeout_error_messages() {
        assert_eq!(
            timeout_error(Duration::from_secs(60), "exec-1", false, None),
            "timed out after 1m0s waiting for execution exec-1"
        );
        assert_eq!(
            timeout_error(Duration::from_secs(60), "exec-1", true, Some("503")),
            "timed out after 1m0s waiting for the Operator to report the target version (last poll error: 503)"
        );
    }

    #[test]
    fn run_stops_after_first_failure() {
        let api = FakeApi {
            update_errors: map(&[("c2", "412 Precondition Failed")]),
            history_status: "DEPLOYED".into(),
            reported_images: images(&[("c1", "v3"), ("c2", "v3"), ("c3", "v3"), ("c4", "v3")]),
            ..Default::default()
        };
        let results = run_updates(
            &api,
            &docker_updates(&["c1", "c2", "c3", "c4"]),
            &options(Duration::from_secs(60), &|_| {}),
        );
        assert!(results[0].err.is_none() && results[0].started);
        assert!(results[1].err.as_ref().unwrap().contains("412"));
        assert!(!results[2].started && !results[3].started);
        assert_eq!(api.updated.borrow().join(","), "c1,c2");
    }

    #[test]
    fn options_bounds() {
        let m = 60_000_000_000;
        assert!(validate_options(1, 15, m).is_ok());
        assert!(validate_options(10, 120, m).is_ok());
        assert!(validate_options(0, 15, m).is_err());
        assert!(validate_options(11, 15, m).is_err());
        assert!(validate_options(1, 4, m).is_err());
        assert!(validate_options(1, 121, m).is_err());
        assert_eq!(
            validate_options(1, 15, 0),
            Err("--timeout must be positive, got 0s".to_string())
        );
        assert_eq!(
            validate_options(1, 15, -5 * m),
            Err("--timeout must be positive, got -5m0s".to_string())
        );
    }

    #[test]
    fn log_prefixes() {
        let c = fleet_entry("c1", "DOCKER", true, "CURRENT");
        assert_eq!(
            log_prefix(&c, &map(&[("org-c1", "Acme")])),
            "[Organization 'Acme' - Cluster 'name-c1']"
        );
        assert_eq!(
            log_prefix(&c, &map(&[("org-other", "Other")])),
            "[Organization 'org-c1' - Cluster 'name-c1']"
        );
        assert_eq!(
            log_prefix(&c, &map(&[("org-c1", "")])),
            "[Organization 'org-c1' - Cluster 'name-c1']"
        );
    }

    #[test]
    fn progress_lines() {
        let k8s = fleet_entry("c1", "KUBERNETES", true, "OUTDATED_IMAGE");
        let docker = fleet_entry("c2", "DOCKER", true, "OUTDATED_IMAGE");
        let kr = UpdateResponse {
            execution_id: "exec-1".into(),
            image_version: "v2".into(),
            chart_version: Some("0.2.0".into()),
        };
        let dr = UpdateResponse {
            execution_id: "exec-2".into(),
            image_version: "v2".into(),
            chart_version: None,
        };
        let run = |cluster: &FleetEntry, response: &UpdateResponse| Run {
            index: 0,
            cluster: cluster.clone(),
            response: response.clone(),
            deadline: Instant::now(),
            started_at: Instant::now(),
            execution_status: String::new(),
            execution_finished: false,
            reported: None,
            last_poll_error: None,
        };
        let elapsed = Duration::from_millis(90_400);
        assert_eq!(
            run(&k8s, &kr).progress_line("[P]", elapsed),
            "[P] - execution exec-1 not in the deployment history yet - 1m30s elapsed"
        );
        let mut r = run(&k8s, &kr);
        r.execution_status = "DEPLOYMENT_QUEUED".into();
        assert_eq!(
            r.progress_line("[P]", elapsed),
            "[P] - execution exec-1 DEPLOYMENT_QUEUED - 1m30s elapsed"
        );
        r.execution_status = "DEPLOYING".into();
        r.last_poll_error = Some("503".into());
        assert_eq!(
            r.progress_line("[P]", elapsed),
            "[P] - execution exec-1 DEPLOYING - 1m30s elapsed - last poll error: 503"
        );
        let mut r = run(&k8s, &kr);
        r.execution_finished = true;
        assert_eq!(
            r.progress_line("[P]", elapsed),
            "[P] - waiting for the Operator heartbeat: reports unknown, target v2/0.2.0 - 1m30s elapsed"
        );
        r.reported = Some(operator_cluster(
            "KUBERNETES",
            true,
            "OUTDATED_IMAGE",
            "v1",
            "0.2.0",
            "0.2.0",
        ));
        assert_eq!(
            r.progress_line("[P]", elapsed),
            "[P] - waiting for the Operator heartbeat: reports v1/0.2.0, target v2/0.2.0 - 1m30s elapsed"
        );
        let mut r = run(&docker, &dr);
        r.execution_finished = true;
        r.reported = Some(operator_cluster(
            "DOCKER",
            false,
            "DISCONNECTED",
            "v1",
            "",
            "",
        ));
        assert_eq!(
            r.progress_line("[P]", elapsed),
            "[P] - waiting for the Operator heartbeat: reports v1 (disconnected), target v2 - 1m30s elapsed"
        );
    }

    #[test]
    fn parameters() {
        let defaults = update_parameters(
            2,
            Duration::from_secs(30),
            Duration::from_secs(900),
            &Target::default(),
        )
        .join("\n");
        for e in [
            "- DryRunDisabled: true",
            "- ParallelRun: 2",
            "- RefreshDelay: 30 seconds",
            "- Timeout: 15m0s",
            "- ImageVersion: q-core target",
            "- ChartVersion: desired chart (KUBERNETES only)",
        ] {
            assert!(defaults.contains(e), "{e}");
        }
        let explicit = update_parameters(
            2,
            Duration::from_secs(30),
            Duration::from_secs(900),
            &Target {
                image_version: "v1.3.0".into(),
                chart_version: "0.3.0".into(),
            },
        )
        .join("\n");
        assert!(
            explicit.contains("- ImageVersion: v1.3.0")
                && explicit.contains("- ChartVersion: 0.3.0")
        );
    }

    #[test]
    fn organization_lookups() {
        let fleet = vec![
            fleet_entry("c1", "DOCKER", true, "CURRENT"),
            {
                let mut c = fleet_entry("c2", "DOCKER", true, "CURRENT");
                c.organization_id = "org-c1".into();
                c
            },
            fleet_entry("c3", "DOCKER", true, "CURRENT"),
        ];
        let looked_up = RefCell::new(Vec::new());
        let names = lookup_organization_names(
            &organization_ids(&fleet),
            &|_, id| {
                looked_up.borrow_mut().push(id.to_string());
                if id == "org-c3" {
                    Err("403 Forbidden".into())
                } else {
                    Ok("Acme".into())
                }
            },
            Duration::from_secs(1),
            Duration::from_secs(1),
        );
        assert_eq!(looked_up.borrow().join(","), "org-c1,org-c3");
        assert_eq!(names, map(&[("org-c1", "Acme")]));
    }

    #[test]
    fn run_reports_progress() {
        let api = FakeApi {
            history_status: "DEPLOYED".into(),
            reported_images: images(&[("c1", "v3")]),
            ..Default::default()
        };
        let lines = RefCell::new(Vec::new());
        let print = |s: String| lines.borrow_mut().push(s);
        let mut o = options(Duration::from_secs(60), &print);
        o.organization_names = map(&[("org-c1", "Acme")]);
        let results = run_updates(&api, &docker_updates(&["c1"]), &o);
        assert!(results[0].err.is_none());
        assert_eq!(
            lines.borrow().join("\n"),
            [
                "[Organization 'Acme' - Cluster 'name-c1'] - Starting Operator update unknown -> v3 - execution exec-c1 - https://console.qovery.com/organization/org-c1/cluster/c1/cluster-logs",
                "Checking Operator updates in 0 seconds",
                "[Organization 'Acme' - Cluster 'name-c1'] execution exec-c1 finished, waiting for the Operator to report the target version",
                "[Organization 'Acme' - Cluster 'name-c1'] Operator reports image v3",
            ]
            .join("\n")
        );
    }

    #[test]
    fn run_reports_execution_state() {
        let api = FakeApi {
            history_status: "DEPLOYING".into(),
            deployed_after: 2,
            reported_images: images(&[("c1", "v3")]),
            ..Default::default()
        };
        let lines = RefCell::new(Vec::new());
        let print = |s: String| lines.borrow_mut().push(s);
        let results = run_updates(
            &api,
            &docker_updates(&["c1"]),
            &options(Duration::from_secs(60), &print),
        );
        assert!(results[0].err.is_none());
        let progress = lines
            .borrow()
            .iter()
            .filter(|l| {
                l.starts_with(
                    "[Organization 'org-c1' - Cluster 'name-c1'] - execution exec-c1 DEPLOYING - ",
                )
            })
            .count();
        assert_eq!(progress, 2);
    }

    #[test]
    fn run_forgets_recovered_poll_errors() {
        let api = FakeApi {
            fleet_failures: Cell::new(1),
            history_status: "DEPLOYED".into(),
            reported_images: images(&[("c1", "v3")]),
            ..Default::default()
        };
        let results = run_updates(
            &api,
            &docker_updates(&["c1"]),
            &options(Duration::from_secs(60), &|_| {}),
        );
        assert!(results[0].err.is_none());
        let api = FakeApi {
            history_failures: Cell::new(1),
            history_status: "DEPLOYED".into(),
            reported_images: images(&[("c1", "v3")]),
            ..Default::default()
        };
        let results = run_updates(
            &api,
            &docker_updates(&["c1"]),
            &options(Duration::from_secs(60), &|_| {}),
        );
        assert!(results[0].err.is_none());
    }

    #[test]
    fn release_image_versions() {
        for (v, ok) in [
            ("v1.2.3", true),
            ("v1.2.3-rc.1", true),
            ("v1.2.3+b", false),
            ("1.2.3", false),
            ("v1.2", false),
            ("v1.2.3-", false),
            ("release-v1.3.0", false),
        ] {
            assert_eq!(is_release_image_version(v), ok, "{v}");
        }
    }

    #[test]
    fn fleet_json_output() {
        let v: Value = serde_json::from_str(r#"{"results":[{"organization_id":"o","cluster_id":"c","cluster_name":"n","cluster_kind":"SELF_MANAGED","attached":true,"connected":true,"reported_runtime":"KUBERNETES","status":"CURRENT","last_heartbeat":"2026-08-18T12:28:46.120+00:00","extra":1}]}"#).unwrap();
        let fleet = decode_fleet(v.to_string().as_bytes()).unwrap();
        assert_eq!(
            fleet[0].to_json()["last_heartbeat"],
            "2026-08-18T12:28:46.12Z"
        );
        assert_eq!(
            decode_fleet(b"{}").unwrap_err(),
            "no value given for required property results"
        );
        assert_eq!(
            decode_fleet(br#"{"results":[{"organization_id":"o"}]}"#).unwrap_err(),
            "no value given for required property cluster_id"
        );
    }
}
