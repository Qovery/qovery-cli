//! `qovery admin cluster list` and `qovery admin cluster deploy`.
//! Port of `cmd/admin_cluster_list.go`, `cmd/admin_cluster_deploy.go`, `pkg/admin_cluster_services.go`,
//! `pkg/admin_cluster_list.go` and `pkg/admin_cluster_deploy_by_batch.go`.

use crate::common::{
    Method, admin_request, admin_url, authorization, format_bool, go_field, validate,
};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_log::{fatih, println};
use serde_json::Value;
use std::collections::BTreeMap;

const FILTERS_USAGE: &str =
    "Value(s) to filter the property selected separated by comma when multiple values are defined";

pub fn list() -> Command {
    Command::new("list")
        .short("List clusters by applying any filter")
        .long(
            r#"This command is used to list clusters information using filters.
The endpoint fetched by the CLI return all clusters except the locked ones.

> Filters
---------
Apply filters using the "--filters" option: filters can be applied to one or more values separated by comma interpreted as logical OR.
The fields usable as filters are the following ones:
* OrganizationId
* OrganizationName
* OrganizationPlan
* ClusterId
* ClusterName
* ClusterType
* ClusterK8sVersion
* Mode
* IsProduction
* CurrentStatus
* HasPendingUpdate

Not implemented yet: filtering from last deployed date or created date

> Examples
----------
* Display every production cluster on cloud providers AWS and GCP:
qovery admin cluster list -f IsProduction=true -f ClusterType=AWS,GCP

* Display every deployed cluster on organization "FooBar":
qovery admin cluster list -f OrganizationName=FooBar -f CurrentStatus=DEPLOYED
"#,
        )
        .flag(Flag::string_to_string("filters", FILTERS_USAGE).short('f'))
        .run(run_list)
}

pub fn deploy() -> Command {
    Command::new("deploy")
        .short("Deploy or upgrade clusters")
        .long(DEPLOY_LONG)
        .flag(Flag::bool("no-confirm", "Do not prompt for confirmation").short('c'))
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .flag(
            Flag::int("parallel-run", "Number of clusters to update in parallel - must be set between 1 and 20")
                .short('n')
                .default_value("5"),
        )
        .flag(
            Flag::int(
                "refresh-delay",
                "Time in seconds to wait before checking clusters status during deployment - must be between [5-120]",
            )
            .short('r')
            .default_value("30"),
        )
        .flag(Flag::string_to_string("filters", FILTERS_USAGE).short('f'))
        .flag(
            Flag::string(
                "execution-mode",
                "Batch execution mode - 'batch' will wait for the N deployments to be finished and ask validation to continue - 'on-the-fly' will deploy continuously as soon as a slot is available",
            )
            .short('e')
            .default_value("batch"),
        )
        .flag(Flag::string("new-k8s-version", "K8S version when upgrading clusters").short('k'))
        .run(run_deploy)
}

const DEPLOY_LONG: &str = r#"This command has 2 main purposes:
* deploy / redeploy clusters: mainly used to update Qovery components (agent / charts / etc.)
* upgrade clusters: used to upgrade to next kube version supported

> Filters
---------
Apply filters using the "--filters" option: filters can be applied to one or more values separated by comma interpreted as logical OR.
The fields usable as filters are the following ones:
* OrganizationId
* OrganizationName
* OrganizationPlan
* ClusterId
* ClusterName
* ClusterType
* ClusterK8sVersion
* Mode
* IsProduction
* CurrentStatus
* HasPendingUpdate

Not implemented yet: filtering from last deployed date or created date

> Parallel Run number
---------------------
The option "--parallel-run" (-n) specifies the number of parallel cluster deployments to be launched (default = 5)
The deployments are launched locally on the workstation, not on a server-side thread.
* if the value is > 20 the cluster autoscaler should be updated manually (the command displays a message and requires an approval to be launched)
* the maximum value cannot exceed 100

> Execution Mode
----------------
The option "--execution-mode" specifies which mode is applied on execution:
* "--execution-mode=batch" (default): deployments are triggered sequentially by batch of N parallel-runs. The next batch of deployments will be launched only after all previous batch deployments
* "--execution-mode=on-the-fly": deployments are triggered as soon as there is a slot available in a thread pool of N parallel-runs

> New K8S Version
-----------------
The option "--new-k8s-version" specifies the next kubernetes version to be applied.
When using this option, the recommendation is to have a low parallel runs number and an execution mode on batch, to be able to monitor clusters peacefully.

> Refresh Delay
---------------
The option "--refresh-delay" specifies the amount of time to wait before fetching new cluster statuses during the deployments.

> Disable Dry Run
-----------------
This option "--disable-dry-run" is mandatory to trigger the deployments

> Examples
----------
* Redeploy only 2 clusters and ensure they are non production
qovery admin cluster deploy -f ClusterName="ClusterA,ClusterB" -f IsProduction=false

* Upgrade cluster having id "80981324-b6u7-400b-97fc-e2173d46a00e" to kube version "1.28" with refreshing statuses locally every "100" seconds
qovery admin cluster deploy -f ClusterId=80981324-b6u7-400b-97fc-e2173d46a00e --new-k8s-version=1.28 --refresh-delay=100 --disable-dry-run

* Upgrade by batch of "8" parallel runs every "1.27" Kubernetes "Production" clusters on "AWS" to kubernetes version "1.28"  with refreshing statuses locally every "100" seconds
qovery admin cluster deploy -f IsProduction=true --parallel-run=8 --refresh-delay=100 -f ClusterK8sVersion=1.27 --new-k8s-version=1.28 -f ClusterType=AWS --disable-dry-run

* Redeploy by batch of "9" parallel runs every "1.27" Kubernetes clusters on "GCP" that have the last deployment status to "DEPLOYMENT_ERROR"
qovery admin cluster deploy -f ClusterType=GCP --parallel-run=9 -f ClusterK8sVersion=1.27 -f CurrentStatus=DEPLOYMENT_ERROR --disable-dry-run
"#;

/// `ClusterDetails`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct ClusterDetails {
    pub organization_id: String,
    pub organization_name: String,
    pub organization_plan: String,
    pub cluster_id: String,
    pub cluster_name: String,
    pub cluster_type: String,
    pub cluster_created_at: String,
    pub cluster_last_deployed_at: String,
    pub cluster_k8s_version: String,
    pub mode: String,
    pub is_production: bool,
    pub current_status: String,
    pub has_karpenter: bool,
    pub has_pending_update: bool,
    pub has_metrics_feature: bool,
}

impl ClusterDetails {
    fn from_json(v: &Value) -> ClusterDetails {
        let s = |k: &str| {
            go_field(v, k)
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string()
        };
        let b = |k: &str| go_field(v, k).and_then(|x| x.as_bool()).unwrap_or(false);
        ClusterDetails {
            organization_id: s("organization_id"),
            organization_name: s("organization_name"),
            organization_plan: s("organization_plan"),
            cluster_id: s("cluster_id"),
            cluster_name: s("cluster_name"),
            cluster_type: s("cluster_type"),
            cluster_created_at: s("cluster_created_at"),
            cluster_last_deployed_at: s("cluster_last_deployed_at"),
            cluster_k8s_version: s("cluster_k8s_version"),
            mode: s("mode"),
            is_production: b("is_production"),
            current_status: s("current_status"),
            has_karpenter: b("has_karpenter"),
            has_pending_update: b("has_pending_update"),
            has_metrics_feature: b("has_metrics_feature"),
        }
    }

    /// Reflection based lookup of the Go struct field used by the filters (`FieldByName`).
    fn field(&self, name: &str) -> String {
        match name {
            "OrganizationId" => self.organization_id.clone(),
            "OrganizationName" => self.organization_name.clone(),
            "OrganizationPlan" => self.organization_plan.clone(),
            "ClusterId" => self.cluster_id.clone(),
            "ClusterName" => self.cluster_name.clone(),
            "ClusterType" => self.cluster_type.clone(),
            "ClusterK8sVersion" => self.cluster_k8s_version.clone(),
            "CurrentStatus" => self.current_status.clone(),
            "Mode" => self.mode.clone(),
            "IsProduction" => format_bool(self.is_production),
            "HasKarpenter" => format_bool(self.has_karpenter),
            "HasPendingUpdate" => format_bool(self.has_pending_update),
            "HasMetricsFeature" => format_bool(self.has_metrics_feature),
            _ => String::new(),
        }
    }
}

/// `PrintClustersTable`.
pub fn print_clusters_table(clusters: &[ClusterDetails]) {
    println("");
    let data: Vec<Vec<String>> = clusters
        .iter()
        .map(|c| {
            vec![
                c.organization_id.clone(),
                c.organization_name.clone(),
                c.organization_plan.clone(),
                c.cluster_id.clone(),
                c.cluster_name.clone(),
                c.cluster_type.clone(),
                c.cluster_k8s_version.clone(),
                c.mode.clone(),
                format_bool(c.is_production),
                c.current_status.clone(),
                format_bool(c.has_karpenter),
                format_bool(c.has_metrics_feature),
                c.cluster_created_at.clone(),
                c.cluster_last_deployed_at.clone(),
                format_bool(c.has_pending_update),
            ]
        })
        .collect();
    qovery_log::print_table(
        &[
            "OrganizationId",
            "OrganizationName",
            "OrganizationPlan",
            "ClusterId",
            "ClusterName",
            "ClusterType",
            "ClusterK8sVersion",
            "Mode",
            "IsProduction",
            "CurrentStatus",
            "HasKarpenter",
            "HasMetricsFeature",
            "ClusterCreatedAt",
            "ClusterLastDeployedAt",
            "HasPendingUpdate",
        ],
        &data,
    );
}

/// `allowedFilterProperties` (Go map: its iteration order, used in the error message, is random).
const ALLOWED_FILTER_PROPERTIES: &[&str] = &[
    "OrganizationId",
    "OrganizationName",
    "OrganizationPlan",
    "ClusterId",
    "ClusterName",
    "ClusterType",
    "ClusterK8sVersion",
    "CurrentStatus",
    "Mode",
    "IsProduction",
    "HasKarpenter",
    "HasPendingUpdate",
    "HasMetricsFeature",
];

/// `AdminClusterListServiceImpl`.
pub struct ListService {
    filters: BTreeMap<String, String>,
}

impl ListService {
    /// `NewAdminClusterListServiceImpl`.
    pub fn new(filters: BTreeMap<String, String>) -> Result<ListService, String> {
        for key in filters.keys() {
            if !ALLOWED_FILTER_PROPERTIES.contains(&key.as_str()) {
                return Err(format!(
                    "Filter property '{key}' not available: valid values are: {}",
                    ALLOWED_FILTER_PROPERTIES.join(", ")
                ));
            }
        }
        Ok(ListService { filters })
    }

    /// `SelectClusters`.
    pub fn select_clusters(&self) -> Result<Vec<ClusterDetails>, String> {
        let fetched = fetch_clusters_eligible_to_update()?;
        Ok(filter_by_predicates(&fetched, &self.filters))
    }
}

fn fetch_clusters_eligible_to_update() -> Result<Vec<ClusterDetails>, String> {
    let auth = authorization()?;
    let res = admin_request(
        Method::GET,
        &format!("{}/listClustersEligibleToUpdate", admin_url()),
        &auth,
        None,
    )?;
    if res.status != 200 {
        return Err(format!(
            "cannot fetch clusters (status_code={})",
            res.status
        ));
    }
    let v: Value = serde_json::from_slice(&res.body).map_err(|e| e.to_string())?;
    Ok(go_field(&v, "Results")
        .and_then(|r| r.as_array())
        .map(|a| a.iter().map(ClusterDetails::from_json).collect())
        .unwrap_or_default())
}

pub fn filter_by_predicates(
    clusters: &[ClusterDetails],
    filters: &BTreeMap<String, String>,
) -> Vec<ClusterDetails> {
    clusters
        .iter()
        .filter(|c| {
            filters
                .iter()
                .all(|(property, value)| value.split(',').any(|v| v == c.field(property)))
        })
        .cloned()
        .collect()
}

fn run_list(ctx: &Ctx) {
    admin_url();
    let service = ListService::new(ctx.string_map("filters"))
        .unwrap_or_else(|e| qovery_core::exit_with_error(e));
    let clusters = service
        .select_clusters()
        .unwrap_or_else(|e| qovery_core::exit_with_error(e));
    println(format!("Found {} clusters", clusters.len()));
    print_clusters_table(&clusters);
}

/// `AdminClusterBatchDeployServiceImpl`.
struct DeployService {
    dry_run_disabled: bool,
    parallel_run: i64,
    refresh_delay: i64,
    complete_batch_before_continue: bool,
    upgrade_cluster_new_k8s_version: Option<String>,
}

impl DeployService {
    fn new(
        dry_run: bool,
        parallel_run: i64,
        refresh_delay: i64,
        execution_mode: &str,
        new_k8s_version: &str,
        no_confirm: bool,
    ) -> Result<DeployService, String> {
        let parallel_run = parallel_run.max(1);
        if parallel_run > 20 && !no_confirm {
            println("");
            println(format!(
                "Please increase the cluster engine autoscaler to {parallel_run}, then type 'yes' to continue"
            ));
            if !validate("autoscaler-increase") {
                println("Exiting");
                return Err("exit on autoscaler validation failed".to_string());
            }
            println("");
        }
        let upgrade = (!new_k8s_version.is_empty()).then(|| new_k8s_version.to_string());
        Ok(DeployService {
            dry_run_disabled: dry_run,
            parallel_run,
            refresh_delay,
            complete_batch_before_continue: execution_mode != "on-the-fly" || upgrade.is_some(),
            upgrade_cluster_new_k8s_version: upgrade,
        })
    }

    fn print_parameters(&self) {
        println("-------------------------------------------");
        println(format!("- DryRunDisabled: {}", self.dry_run_disabled));
        println(format!("- ParallelRun: {}", self.parallel_run));
        println(format!("- RefreshDelay: {} seconds", self.refresh_delay));
        println(format!(
            "- BatchMode: {}",
            self.complete_batch_before_continue
        ));
        match &self.upgrade_cluster_new_k8s_version {
            Some(v) => println(format!("- UpgradeMode: true (NewK8sVersion = {v})")),
            None => println("- UpgradeMode: false"),
        }
        println("-------------------------------------------");
    }

    fn deploy(
        &self,
        clusters: &[ClusterDetails],
    ) -> Result<(Vec<ClusterDetails>, Vec<ClusterDetails>), String> {
        if !self.dry_run_disabled {
            println(
                "dry-run-disabled is false: trigger cluster deployment dry-run mode (no changes will be made)",
            );
        }
        let mut processed: Vec<ClusterDetails> = Vec::new();
        // Go map: iteration order is random, insertion order is kept here
        let mut deploying: Vec<ClusterDetails> = Vec::new();
        let mut pending: Vec<ClusterDetails> = Vec::new();
        let mut index: i64 = -1;
        let last = clusters.len() as i64 - 1;
        loop {
            let mut client = qovery_core::client::get_client()?;
            let mut wait = false;
            if self.complete_batch_before_continue && index != -1 {
                if !deploying.is_empty() {
                    wait = true;
                } else {
                    println(format!(
                        "Do you want to continue next batch of {} deployments ?",
                        self.parallel_run
                    ));
                    if !validate("deploy") {
                        println("Exiting");
                        return Err("user stopped the command after batch terminated".to_string());
                    }
                }
            }
            if !wait && (deploying.len() as i64) < self.parallel_run && index < last {
                let mut i = deploying.len() as i64;
                while i < self.parallel_run {
                    index += 1;
                    let mut cluster = clusters[index as usize].clone();
                    let status = cluster_status(&mut client, &cluster)?;
                    if is_terminal_cluster_state(&status) {
                        println(format!(
                            "[Organization '{}' - Cluster '{}'] - Starting deployment - https://console.qovery.com/organization/{}/cluster/{}/cluster-logs",
                            cluster.organization_name,
                            cluster.cluster_name,
                            cluster.organization_id,
                            cluster.cluster_id
                        ));
                        let res = if self.upgrade_cluster_new_k8s_version.is_some() {
                            upgrade_cluster(&client, &cluster.cluster_id, self.dry_run_disabled)
                        } else {
                            deploy_cluster(&cluster.cluster_id, self.dry_run_disabled)
                        };
                        if let Err(e) = res {
                            println(fatih::red(&format!(
                                "[Organization '{}' - Cluster '{}'] - Error on deploy: {} ",
                                cluster.organization_name, cluster.cluster_name, e
                            )));
                        }
                        cluster.current_status = "DEPLOYING".to_string();
                        deploying.retain(|c| c.cluster_id != cluster.cluster_id);
                        deploying.push(cluster);
                    } else {
                        println(format!(
                            "[Organization '{}' - Cluster '{}'] - Cluster's state is '{}' (not a terminal state), sending it to waiting queue to be processed later",
                            cluster.organization_name, cluster.cluster_name, status
                        ));
                        pending.push(cluster);
                    }
                    if index == last {
                        break;
                    }
                    i += 1;
                }
            }

            println(format!(
                "Checking clusters' status in {} seconds",
                self.refresh_delay
            ));
            if self.refresh_delay > 0 {
                std::thread::sleep(std::time::Duration::from_secs(self.refresh_delay as u64));
            }

            let mut finished: Vec<String> = Vec::new();
            for cluster in deploying.iter_mut() {
                let status = cluster_status(&mut client, cluster)?;
                cluster.current_status = status.clone();
                if is_terminal_cluster_state(&status) || status == "INTERNAL_ERROR" {
                    println(cluster_status_color(
                        &status,
                        &format!(
                            "[Organization '{}' - Cluster '{}'] - Cluster deployed with '{}' status ",
                            cluster.organization_name, cluster.cluster_name, status
                        ),
                    ));
                    processed.push(cluster.clone());
                    finished.push(cluster.cluster_id.clone());
                }
            }
            deploying.retain(|c| !finished.contains(&c.cluster_id));

            if deploying.is_empty() && index == last {
                break;
            }
        }
        println("No more deployment to process");
        Ok((processed, pending))
    }
}

pub use qovery_services::is_terminal_cluster_state;

/// `clusterStatusColor(status).Sprintf(text)`: green for success, red for failures, yellow otherwise.
pub fn cluster_status_color(status: &str, text: &str) -> String {
    let attr = if matches!(status, "DEPLOYED" | "READY" | "RESTARTED") {
        fatih::Attr::FgGreen
    } else if status.ends_with("ERROR") || status == "INVALID_CREDENTIALS" {
        fatih::Attr::FgRed
    } else {
        fatih::Attr::FgYellow
    };
    fatih::sprint(&[attr], text)
}

const CLUSTER_STATES: &[&str] = &[
    "BUILDING",
    "BUILD_ERROR",
    "CANCELED",
    "CANCELING",
    "DELETED",
    "DELETE_ERROR",
    "DELETE_QUEUED",
    "DELETING",
    "DEPLOYED",
    "DEPLOYING",
    "DEPLOYMENT_ERROR",
    "DEPLOYMENT_QUEUED",
    "DRY_RUN",
    "QUEUED",
    "READY",
    "STOPPED",
    "STOPPING",
    "STOP_ERROR",
    "STOP_QUEUED",
    "RESTART_QUEUED",
    "RESTARTING",
    "RESTARTED",
    "RESTART_ERROR",
    "INVALID_CREDENTIALS",
];

/// Decode a `qovery.ClusterStatus` like the Go client (required properties, enum validation).
fn decode_cluster_status(body: &[u8]) -> Result<String, String> {
    if body.is_empty() {
        return Ok(String::new());
    }
    let v: Value = serde_json::from_slice(body).map_err(|e| e.to_string())?;
    for p in ["cluster_id", "status", "is_deployed", "reason"] {
        if v.get(p).is_none() {
            return Err(format!("no value given for required property {p}"));
        }
    }
    let status = v["status"].as_str().unwrap_or_default().to_string();
    if !CLUSTER_STATES.contains(&status.as_str()) {
        return Err(format!("{status} is not a valid ClusterStateEnum"));
    }
    let reason = v["reason"].as_str().unwrap_or_default();
    if !matches!(reason, "UNSPECIFIED" | "MAINTENANCE") {
        return Err(format!("{reason} is not a valid DeploymentInfraReason"));
    }
    Ok(status)
}

/// `GetClusterStatus` wrapped in `RetryQoveryClientApiRequestOnUnauthorized`; Go returns the error (or nil)
/// whenever the response is missing or not a 200.
fn cluster_status(
    client: &mut qovery_api::Client,
    cluster: &ClusterDetails,
) -> Result<String, String> {
    let path = format!(
        "/organization/{}/cluster/{}/status",
        cluster.organization_id, cluster.cluster_id
    );
    let res =
        qovery_cmd_auth::service::retry_qovery_client_api_request_on_unauthorized(|refetch| {
            if refetch {
                *client = qovery_core::client::get_client().map_err(qovery_api::ApiError::new)?;
            }
            client.execute(Method::GET, &path, &[], None)
        })
        .map_err(|e| e.to_string())?;
    let status = decode_cluster_status(&res.body)?;
    if res.status > 200 {
        // Go: `return nil, err` with a nil error: the caller dereferences a nil result and panics
        return Err(String::new());
    }
    Ok(status)
}

/// `execAdminRequest` + `deployCluster`.
fn deploy_cluster(cluster_id: &str, dry_run_disabled: bool) -> Result<(), String> {
    let url = format!("{}/cluster/deploy/{}", admin_url(), cluster_id);
    let exec = || -> crate::common::HttpResponse {
        let auth = match authorization() {
            Ok(a) => a,
            Err(e) => {
                qovery_log::println_error(e);
                qovery_core::exit(0)
            }
        };
        let body = format!(
            "{{ \"metadata\": {{ \"dry_run_deploy\": {} }} }}",
            !dry_run_disabled
        );
        match admin_request(Method::POST, &url, &auth, Some(body.into_bytes())) {
            Ok(r) => r,
            Err(e) => qovery_log::logrus::fatal(e),
        }
    };
    let mut res = exec();
    if res.status == 401 {
        qovery_cmd_auth::service::do_request_user_to_authenticate(false, true);
        res = exec();
    }
    if res.status != 200 {
        return Err(format!(
            "could not deploy cluster : {}. {}",
            res.status_line(),
            res.text()
        ));
    }
    Ok(())
}

fn upgrade_cluster(
    client: &qovery_api::Client,
    cluster_id: &str,
    dry_run_disabled: bool,
) -> Result<(), String> {
    if !dry_run_disabled {
        println("dry-run-disabled is false: skip cluster upgrade");
        return Ok(());
    }
    let res = client
        .execute(
            Method::POST,
            &format!("/cluster/{cluster_id}/upgrade"),
            &[],
            None,
        )
        .map_err(|e| e.to_string())?;
    decode_cluster_status(&res.body).map(|_| ())
}

fn run_deploy(ctx: &Ctx) {
    admin_url();
    if let Err(e) = qovery_core::auth::get_access_token(false) {
        qovery_core::exit_with_error(e);
    }
    let mut filters = ctx.string_map("filters");
    filters
        .entry("CurrentStatus".to_string())
        .or_insert_with(|| "DEPLOYED".to_string());
    let no_confirm = ctx.bool("no-confirm");
    let list = ListService::new(filters).unwrap_or_else(|e| qovery_core::exit_with_error(e));
    let deploy = DeployService::new(
        ctx.bool("disable-dry-run"),
        ctx.int("parallel-run"),
        ctx.int("refresh-delay"),
        &ctx.string("execution-mode"),
        &ctx.string("new-k8s-version"),
        no_confirm,
    )
    .unwrap_or_else(|e| qovery_core::exit_with_error(e));
    if let Err(e) = deploy_clusters_by_batch(&list, &deploy, no_confirm) {
        qovery_core::exit_with_error(e);
    }
}

/// `DeployClustersByBatch`.
fn deploy_clusters_by_batch(
    list: &ListService,
    deploy: &DeployService,
    no_confirm: bool,
) -> Result<(), String> {
    let clusters = list.select_clusters()?;
    println(format!("{} clusters to deploy:", clusters.len()));
    print_clusters_table(&clusters);
    deploy.print_parameters();
    if !no_confirm {
        println("Do you want to continue deploy process ?");
        if !validate("deploy") {
            println("Exiting: Validation failed");
            return Ok(());
        }
    }
    let (processed, pending) = match deploy.deploy(&clusters) {
        Ok(r) => r,
        Err(e) if e.is_empty() => {
            // Go dereferences a nil result: runtime panic (exit status 2)
            qovery_log::go_builtin_println(
                "panic: runtime error: invalid memory address or nil pointer dereference",
            );
            qovery_core::exit(2)
        }
        Err(e) => return Err(e),
    };
    if !pending.is_empty() {
        println(format!(
            "{} clusters not triggered because in non-terminal state (queue not implemented yet):",
            pending.len()
        ));
        print_clusters_table(&pending);
    }
    if !processed.is_empty() {
        println(format!("{} clusters deployed:", clusters.len()));
        print_clusters_table(&processed);
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn cluster(id: &str, kind: &str, prod: bool) -> ClusterDetails {
        ClusterDetails {
            cluster_id: id.into(),
            cluster_type: kind.into(),
            is_production: prod,
            ..Default::default()
        }
    }

    #[test]
    fn filters_are_or_within_a_key_and_and_between_keys() {
        let clusters = vec![
            cluster("a", "AWS", true),
            cluster("b", "GCP", true),
            cluster("c", "AWS", false),
        ];
        let mut f = BTreeMap::new();
        f.insert("ClusterType".to_string(), "AWS,GCP".to_string());
        f.insert("IsProduction".to_string(), "true".to_string());
        let ids: Vec<String> = filter_by_predicates(&clusters, &f)
            .into_iter()
            .map(|c| c.cluster_id)
            .collect();
        assert_eq!(ids, vec!["a", "b"]);
    }

    #[test]
    fn unknown_filter_is_rejected() {
        let mut f = BTreeMap::new();
        f.insert("Foo".to_string(), "x".to_string());
        assert!(
            ListService::new(f)
                .err()
                .unwrap()
                .starts_with("Filter property 'Foo' not available: valid values are: ")
        );
    }

    #[test]
    fn status_colors() {
        // port of TestClusterStatusColor (colors are disabled when stdout is not a tty)
        for (status, attr) in [
            ("DEPLOYED", fatih::Attr::FgGreen),
            ("READY", fatih::Attr::FgGreen),
            ("RESTARTED", fatih::Attr::FgGreen),
            ("DEPLOYMENT_ERROR", fatih::Attr::FgRed),
            ("BUILD_ERROR", fatih::Attr::FgRed),
            ("INTERNAL_ERROR", fatih::Attr::FgRed),
            ("INVALID_CREDENTIALS", fatih::Attr::FgRed),
            ("CANCELED", fatih::Attr::FgYellow),
            ("STOPPED", fatih::Attr::FgYellow),
            ("DELETED", fatih::Attr::FgYellow),
        ] {
            assert_eq!(
                cluster_status_color(status, "x"),
                fatih::sprint(&[attr], "x"),
                "{status}"
            );
        }
    }
}
