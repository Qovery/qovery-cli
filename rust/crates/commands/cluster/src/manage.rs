//! `cluster list`, `cluster deploy`, `cluster stop`, `cluster upgrade` (cmd/cluster_list.go, cluster_deploy.go,
//! cluster_stop.go, cluster_upgrade_to_next_kubernetes_version.go) and the cluster service
//! (pkg/cluster/cluster_service.go).

use crate::prompt::{PromptUiFactory, PromptUiFactoryImpl};
use crate::util::{self, GO_NIL_ERROR, path_escape};
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, exit, exit_with_error, resources, status, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::pterm;
use serde_json::{Value, json};

fn flag_organization() -> Flag {
    Flag::string("organization", "Organization Name")
}

fn flag_cluster() -> Flag {
    Flag::string("cluster", "Cluster Name")
        .short('n')
        .required()
}

fn flag_watch() -> Flag {
    Flag::bool(
        "watch",
        "Watch cluster status until it's ready or an error occurs",
    )
    .short('w')
}

pub fn list_command() -> Command {
    Command::new("list")
        .short("List clusters")
        .flag(flag_organization())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

pub fn deploy_command() -> Command {
    Command::new("deploy")
        .short("Deploy a cluster")
        .flag(flag_organization())
        .flag(flag_cluster())
        .flag(flag_watch())
        .run(run_deploy)
}

pub fn stop_command() -> Command {
    Command::new("stop")
        .short("Stop a cluster")
        .flag(flag_organization())
        .flag(flag_cluster())
        .flag(flag_watch())
        .run(run_stop)
}

pub fn upgrade_command() -> Command {
    Command::new("upgrade")
        .short("Upgrade a cluster to next kubernetes version available for the cluster")
        .flag(Flag::bool("skip-confirmation", "Skip prompt confirmation if passed").short('y'))
        .flag(flag_organization())
        .flag(flag_cluster())
        .flag(flag_watch())
        .run(run_upgrade)
}

/// Cluster name lookup (`utils.FindByClusterName`).
pub fn find_cluster_by_name<'a>(clusters: &'a [Value], name: &str) -> Option<&'a Value> {
    qovery_core::find::find_by_name(clusters, name)
}

pub fn str_field<'a>(v: &'a Value, key: &str) -> &'a str {
    v.get(key).and_then(|x| x.as_str()).unwrap_or_default()
}

const CLUSTER_REQUIRED: [&str; 6] = [
    "id",
    "created_at",
    "organization",
    "name",
    "region",
    "cloud_provider",
];
const CLOUD_VENDORS: [&str; 11] = [
    "AWS",
    "SCW",
    "GCP",
    "DO",
    "AZURE",
    "OVH",
    "CIVO",
    "HETZNER",
    "ORACLE",
    "IBM",
    "ON_PREMISE",
];
const KUBERNETES_MODES: [&str; 3] = ["MANAGED", "SELF_MANAGED", "PARTIALLY_MANAGED"];
const CLUSTER_STATES: [&str; 24] = [
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

/// The Go client refuses a `Cluster` lacking a required property or with an unknown enum value.
fn validate_cluster(c: &Value) -> Result<(), String> {
    let Some(obj) = c.as_object() else {
        return Ok(());
    };
    for p in CLUSTER_REQUIRED {
        if !obj.contains_key(p) {
            return Err(format!("no value given for required property {p}"));
        }
    }
    for (field, allowed, name) in [
        ("cloud_provider", &CLOUD_VENDORS[..], "CloudVendorEnum"),
        ("kubernetes", &KUBERNETES_MODES[..], "KubernetesEnum"),
        ("status", &CLUSTER_STATES[..], "ClusterStateEnum"),
    ] {
        if let Some(Value::String(v)) = obj.get(field)
            && !allowed.contains(&v.as_str())
        {
            return Err(format!("{v} is not a valid {name}"));
        }
    }
    Ok(())
}

/// `client.ClustersAPI.ListOrganizationCluster(ctx, organizationId).Execute()`.
pub fn list_organization_clusters(
    client: &Client,
    organization_id: &str,
) -> Result<Vec<Value>, String> {
    let clusters: Vec<Value> = client
        .list(&format!(
            "/organization/{}/cluster",
            path_escape(organization_id)
        ))
        .map_err(|e| e.to_string())?;
    for c in &clusters {
        validate_cluster(c)?;
    }
    Ok(clusters)
}

/// Port of `cluster.ClusterService` (pkg/cluster/cluster_service.go).
pub trait ClusterService {
    fn list_clusters(&self, organization_id: &str) -> Result<Vec<Value>, String>;
    fn list_cluster_regions(&self, cloud_provider_type: &str) -> Result<Vec<Value>, String>;
    fn ask_to_edit_storage_class(&self, cluster: &Value) -> Result<(), String>;
}

pub struct ClusterServiceImpl<'a> {
    pub client: &'a Client,
    pub prompt: &'a dyn PromptUiFactory,
}

impl<'a> ClusterServiceImpl<'a> {
    pub fn new(client: &'a Client, prompt: &'a dyn PromptUiFactory) -> Self {
        ClusterServiceImpl { client, prompt }
    }

    fn find_cluster(
        &self,
        organization_name: &str,
        cluster_name: &str,
    ) -> Result<(String, Value), String> {
        let org_id = resources::organization_id(self.client, organization_name)?;
        let clusters = self.list_clusters(&org_id)?;
        match find_cluster_by_name(&clusters, cluster_name) {
            Some(c) => Ok((org_id, c.clone())),
            None => Err(format!(
                "cluster {cluster_name} not found. You can list all clusters with: qovery cluster list"
            )),
        }
    }

    fn get_cluster_status(&self, org_id: &str, cluster_id: &str) -> Result<Value, String> {
        self.client
            .get(&format!(
                "/organization/{}/cluster/{}/status",
                path_escape(org_id),
                path_escape(cluster_id)
            ))
            .map_err(|e| e.to_string())
    }

    fn watch(&self, org_id: &str, cluster_id: &str) -> Result<(), String> {
        loop {
            let status = self.get_cluster_status(org_id, cluster_id)?;
            let state = str_field(&status, "status");
            if util::is_terminal_cluster_state(state) {
                return Ok(());
            }
            qovery_log::println(format!(
                "Cluster status: {}",
                status::cluster_status_text_with_color(state)
            ));
            std::thread::sleep(std::time::Duration::from_secs(5));
        }
    }

    /// `DeployCluster(organizationName, clusterName, watchFlag)`.
    pub fn deploy_cluster(
        &self,
        organization_name: &str,
        cluster_name: &str,
        watch: bool,
    ) -> Result<(), String> {
        let (org_id, cluster) = self.find_cluster(organization_name, cluster_name)?;
        let cluster_id = str_field(&cluster, "id");
        let res = self
            .client
            .send(
                Method::POST,
                &format!(
                    "/organization/{}/cluster/{}/deploy",
                    path_escape(&org_id),
                    path_escape(cluster_id)
                ),
                &[],
                None,
            )
            .map_err(|e| e.to_string());
        match res {
            // Go dereferences a nil response on transport errors (panic)
            Err(e) => return Err(e),
            Ok(r) if r.status != 200 => {
                let err = if r.status >= 300 {
                    r.status_line()
                } else {
                    GO_NIL_ERROR.to_string()
                };
                return Err(format!(
                    "status code: {} ; body: {} ; error: {}",
                    r.status_line(),
                    r.text(),
                    err
                ));
            }
            Ok(_) => {}
        }
        if watch {
            self.watch(&org_id, cluster_id)?;
            qovery_log::println(format!(
                "Cluster {} deployed!",
                pterm::FG_BLUE.sprint(cluster_name)
            ));
        } else {
            qovery_log::println(format!(
                "Deploying cluster {} in progress..",
                pterm::FG_BLUE.sprint(cluster_name)
            ));
        }
        Ok(())
    }

    /// `StopCluster(organizationName, clusterName, watchFlag)`.
    pub fn stop_cluster(
        &self,
        organization_name: &str,
        cluster_name: &str,
        watch: bool,
    ) -> Result<(), String> {
        let (org_id, cluster) = self.find_cluster(organization_name, cluster_name)?;
        let cluster_id = str_field(&cluster, "id");
        let _: Value = self
            .client
            .post_empty(&format!(
                "/organization/{}/cluster/{}/stop",
                path_escape(&org_id),
                path_escape(cluster_id)
            ))
            .map_err(|e| e.to_string())?;
        if watch {
            self.watch(&org_id, cluster_id)?;
            qovery_log::println(format!(
                "Cluster {} stopped!",
                pterm::FG_BLUE.sprint(cluster_name)
            ));
        } else {
            qovery_log::println(format!(
                "Stopping cluster {} in progress..",
                pterm::FG_BLUE.sprint(cluster_name)
            ));
        }
        Ok(())
    }

    /// `GetClusterByID(organizationId, clusterId)`.
    #[allow(
        dead_code,
        reason = "port of the Go ClusterService.GetClusterById, kept for parity"
    )]
    pub fn get_cluster_by_id(
        &self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<Value, String> {
        let clusters = self.list_clusters(organization_id)?;
        clusters
            .into_iter()
            .find(|c| str_field(c, "id") == cluster_id)
            .ok_or_else(|| {
                format!(
                    "Cluster with id {cluster_id} doesn't exists in organization {organization_id}"
                )
            })
    }
}

impl ClusterService for ClusterServiceImpl<'_> {
    fn list_clusters(&self, organization_id: &str) -> Result<Vec<Value>, String> {
        list_organization_clusters(self.client, organization_id)
    }

    fn list_cluster_regions(&self, cloud_provider_type: &str) -> Result<Vec<Value>, String> {
        let path = match cloud_provider_type {
            "GCP" => "/gcp/region",
            "AWS" => "/aws/region",
            "SCW" => "/scaleway/region",
            other => return Err(format!("cannot list regions for '{other}' cloud provider")),
        };
        self.client.list(path).map_err(|e| e.to_string())
    }

    fn ask_to_edit_storage_class(&self, cluster: &Value) -> Result<(), String> {
        let storage_class_name = self.prompt.run_prompt(
            "We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name",
            "",
        )?;
        if util::is_empty_or_blank(&storage_class_name) {
            return Err("storage class name should be defined and cannot be empty".to_string());
        }
        let org_id = cluster
            .get("organization")
            .map(|o| str_field(o, "id"))
            .unwrap_or_default();
        let path = format!(
            "/organization/{}/cluster/{}/advancedSettings",
            path_escape(org_id),
            path_escape(str_field(cluster, "id"))
        );
        let mut settings: Value = self.client.get(&path).map_err(|e| e.to_string())?;
        if !settings.is_object() {
            settings = json!({});
        }
        settings["storageclass.fast_ssd"] = Value::String(storage_class_name);
        let _: Value = self
            .client
            .put(&path, &settings)
            .map_err(|e| e.to_string())?;
        Ok(())
    }
}

/// `getClusterJsonOutput`.
fn cluster_json_output(clusters: &[Value]) -> String {
    if clusters.is_empty() {
        return "null".to_string();
    }
    let results: Vec<Value> = clusters
        .iter()
        .map(|c| {
            json!({
                "id": str_field(c, "id"),
                "updated_at": c.get("updated_at").and_then(|v| v.as_str()).and_then(util::to_iso8601),
                "type": "cluster",
                "name": str_field(c, "name"),
                "status": c.get("status").cloned().unwrap_or(Value::Null),
            })
        })
        .collect();
    go_json(&Value::Array(results))
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    let organization_id = qovery_core::check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let clusters = qovery_core::check_error(
        ClusterServiceImpl::new(&client, &PromptUiFactoryImpl).list_clusters(&organization_id),
    );

    if ctx.bool("json") {
        qovery_log::println(cluster_json_output(&clusters));
        return;
    }

    let data: Vec<Vec<String>> = clusters
        .iter()
        .map(|c| {
            vec![
                str_field(c, "id").to_string(),
                str_field(c, "name").to_string(),
                "cluster".to_string(),
                status::cluster_status_text_with_color(str_field(c, "status")),
                time_string_from_rfc3339(str_field(c, "updated_at")),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Type", "Status", "Last Update"], &data);
}

fn run_deploy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    let service = ClusterServiceImpl::new(&client, &PromptUiFactoryImpl);
    if let Err(e) = service.deploy_cluster(
        &ctx.string("organization"),
        &ctx.string("cluster"),
        ctx.bool("watch"),
    ) {
        exit_with_error(e);
    }
}

fn run_stop(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    let service = ClusterServiceImpl::new(&client, &PromptUiFactoryImpl);
    if let Err(e) = service.stop_cluster(
        &ctx.string("organization"),
        &ctx.string("cluster"),
        ctx.bool("watch"),
    ) {
        exit_with_error(e);
    }
}

fn run_upgrade(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    let org_id = qovery_core::check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let clusters = qovery_core::check_error(list_organization_clusters(&client, &org_id));
    let cluster_name = ctx.string("cluster");
    let Some(cluster) = find_cluster_by_name(&clusters, &cluster_name) else {
        qovery_log::println_error(format!("cluster {cluster_name} not found"));
        qovery_log::println_info("You can list all clusters with: qovery cluster list");
        exit(1);
    };
    let cluster_id = str_field(cluster, "id").to_string();
    let status_path = format!(
        "/organization/{}/cluster/{}/status",
        path_escape(&org_id),
        path_escape(&cluster_id)
    );
    let status: Value = qovery_core::check_error(client.get(&status_path));
    if status
        .get("next_k8s_available_version")
        .is_none_or(|v| v.is_null())
    {
        exit_with_error("no available kubernetes version to upgrade to for this cluster");
    }

    qovery_log::println(format!(
        "A new kubernetes version `` is available for your cluster {cluster_name}."
    ));
    if !ctx.bool("skip-confirmation") {
        let answer = match crate::prompt::select(
            "Do you want to proceed with cluster upgrade? [Yes/No]",
            &["Yes", "No"],
        ) {
            Ok(a) => a,
            Err(e) => exit_with_error(e),
        };
        if answer.trim_matches(' ').to_lowercase() != "yes" {
            qovery_log::println("Cluster upgrade aborted");
            exit(1);
        }
    } else {
        qovery_log::println("Skipping confirmation, proceeding with cluster upgrade..");
    }

    match client.send(
        Method::POST,
        &format!("/cluster/{}/upgrade", path_escape(&cluster_id)),
        &[],
        None,
    ) {
        Err(e) => exit_with_error(e),
        Ok(r) if r.status >= 300 => {
            qovery_log::println_error(r.status_line());
            if r.status != 200 {
                qovery_log::println_error(format!(
                    "status code: {} ; body: {}",
                    r.status_line(),
                    r.text()
                ));
            }
            exit(1);
        }
        Ok(_) => {}
    }

    let watch_name = pterm::FG_BLUE.sprint(&cluster_name);
    if ctx.bool("watch") {
        loop {
            let status: Value = match client.get(&status_path) {
                Ok(s) => s,
                Err(e) => {
                    // Go prints the error then dereferences the nil status (panic)
                    qovery_log::println_error(e);
                    exit(2);
                }
            };
            let state = str_field(&status, "status");
            if util::is_terminal_cluster_state(state) {
                break;
            }
            qovery_log::println(format!(
                "Cluster status: {}",
                status::cluster_status_text_with_color(state)
            ));
            std::thread::sleep(std::time::Duration::from_secs(5));
        }
        qovery_log::println(format!("Cluster {watch_name} upgraded!"));
    } else {
        qovery_log::println(format!("Upgrading cluster {watch_name} in progress.."));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::prompt::PromptUiFactoryMock;
    use crate::testutil::{MockServer, Route, cluster_json};

    fn org() -> Value {
        json!({"id": "org-test", "name": "TestOrganization", "created_at": "2024-01-01T00:00:00Z", "plan": "FREE"})
    }

    #[test]
    fn list_clusters_empty_and_non_empty() {
        let server = MockServer::start(vec![Route::get(
            "/organization/org-test/cluster",
            json!({"results": []}),
        )]);
        let prompt = PromptUiFactoryMock::new(&[], &[]);
        let client = server.client();
        let service = ClusterServiceImpl::new(&client, &prompt);
        assert_eq!(service.list_clusters("org-test").unwrap().len(), 0);

        let cluster = cluster_json("c-1", "TestCluster", "org-test", "AWS", "MANAGED");
        let server = MockServer::start(vec![Route::get(
            "/organization/org-test/cluster",
            json!({"results": [cluster]}),
        )]);
        let client = server.client();
        let service = ClusterServiceImpl::new(&client, &prompt);
        let clusters = service.list_clusters("org-test").unwrap();
        assert_eq!(clusters.len(), 1);
        assert_eq!(str_field(&clusters[0], "id"), "c-1");
    }

    fn deploy_routes(action: &str, with_status: bool) -> Vec<Route> {
        let cluster = cluster_json("c-1", "TestCluster", "org-test", "AWS", "MANAGED");
        let status = json!({"cluster_id": "c-1", "status": "DEPLOYED", "is_deployed": true, "reason": "UNSPECIFIED"});
        let mut routes = vec![
            Route::get("/organization", json!({"results": [org()]})),
            Route::get(
                "/organization/org-test/cluster",
                json!({"results": [cluster]}),
            ),
            Route::new(
                "POST",
                &format!("/organization/org-test/cluster/c-1/{action}"),
                200,
                json!({"cluster_id": "c-1", "status": "DEPLOYING", "is_deployed": false, "reason": "UNSPECIFIED"}),
            ),
        ];
        if with_status {
            routes.push(Route::get(
                "/organization/org-test/cluster/c-1/status",
                status,
            ));
        }
        routes
    }

    #[test]
    fn deploy_managed_cluster() {
        let prompt = PromptUiFactoryMock::new(&[], &[]);
        for watch in [false, true] {
            let server = MockServer::start(deploy_routes("deploy", watch));
            let client = server.client();
            let service = ClusterServiceImpl::new(&client, &prompt);
            assert_eq!(
                service.deploy_cluster("TestOrganization", "TestCluster", watch),
                Ok(())
            );
        }
    }

    #[test]
    fn stop_managed_cluster() {
        let prompt = PromptUiFactoryMock::new(&[], &[]);
        for watch in [false, true] {
            let server = MockServer::start(deploy_routes("stop", watch));
            let client = server.client();
            let service = ClusterServiceImpl::new(&client, &prompt);
            assert_eq!(
                service.stop_cluster("TestOrganization", "TestCluster", watch),
                Ok(())
            );
        }
    }

    #[test]
    fn deploy_unknown_cluster() {
        let prompt = PromptUiFactoryMock::new(&[], &[]);
        let server = MockServer::start(deploy_routes("deploy", false));
        let client = server.client();
        let service = ClusterServiceImpl::new(&client, &prompt);
        assert_eq!(
            service.deploy_cluster("TestOrganization", "nope", false),
            Err(
                "cluster nope not found. You can list all clusters with: qovery cluster list"
                    .to_string()
            )
        );
    }

    const STORAGE_LABEL: &str = "We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name";

    #[test]
    fn ask_to_edit_storage_class() {
        let cluster = cluster_json(
            "c-1",
            "TestCluster",
            "org-test",
            "ON_PREMISE",
            "SELF_MANAGED",
        );
        let path = "/organization/org-test/cluster/c-1/advancedSettings";
        let server = MockServer::start(vec![
            Route::get(path, json!({"storageclass.fast_ssd": "old"})),
            Route::new("PUT", path, 200, json!({"storageclass.fast_ssd": "my-sc"})),
        ]);
        let client = server.client();

        // succeed
        let prompt = PromptUiFactoryMock::new(&[], &[(STORAGE_LABEL, "my-sc")]);
        let service = ClusterServiceImpl::new(&client, &prompt);
        assert_eq!(service.ask_to_edit_storage_class(&cluster), Ok(()));
        let put = server
            .requests()
            .into_iter()
            .find(|r| r.method == "PUT")
            .unwrap();
        assert_eq!(put.body["storageclass.fast_ssd"], "my-sc");

        // prompt fails
        let prompt = PromptUiFactoryMock::new(&[STORAGE_LABEL], &[]);
        let service = ClusterServiceImpl::new(&client, &prompt);
        assert!(service.ask_to_edit_storage_class(&cluster).is_err());

        // empty name
        let prompt = PromptUiFactoryMock::new(&[], &[(STORAGE_LABEL, "  ")]);
        let service = ClusterServiceImpl::new(&client, &prompt);
        assert_eq!(
            service.ask_to_edit_storage_class(&cluster),
            Err("storage class name should be defined and cannot be empty".to_string())
        );
    }

    #[test]
    fn list_cluster_regions() {
        let region = json!({"name": "eu-west-3", "country_code": "FR", "country": "France", "city": "Paris"});
        let server = MockServer::start(vec![
            Route::get("/aws/region", json!({"results": [region]})),
            Route::get("/scaleway/region", json!({"results": [region]})),
            Route::get("/gcp/region", json!({"results": [region]})),
        ]);
        let client = server.client();
        let prompt = PromptUiFactoryMock::new(&[], &[]);
        let service = ClusterServiceImpl::new(&client, &prompt);
        for provider in ["AWS", "SCW", "GCP"] {
            assert_eq!(
                service.list_cluster_regions(provider).unwrap().len(),
                1,
                "{provider}"
            );
        }
        assert_eq!(
            service.list_cluster_regions("ON_PREMISE"),
            Err("cannot list regions for 'ON_PREMISE' cloud provider".to_string())
        );
    }

    #[test]
    fn json_output() {
        assert_eq!(cluster_json_output(&[]), "null");
        let c = json!({"id": "c", "name": "n", "status": "DEPLOYED", "updated_at": "2024-06-02T11:30:00Z"});
        assert_eq!(
            cluster_json_output(&[c]),
            r#"[{"id":"c","name":"n","status":"DEPLOYED","type":"cluster","updated_at":"2024-06-02T11:30:00.000Z"}]"#
        );
    }
}
