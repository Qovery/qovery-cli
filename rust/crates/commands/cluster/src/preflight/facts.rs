//! Port of pkg/cluster/preflight/facts.go: everything the preflight reads from the cluster.
//!
//! Every section carries the error that prevented its collection (`err`, the Go error string), so a check can
//! tell "absent" from "could not read" and never turns missing evidence into a PASS.

use std::collections::BTreeMap;

pub type Labels = BTreeMap<String, String>;

#[derive(Debug, Clone, Default)]
pub struct Facts {
    pub version: VersionFacts,
    pub access: AccessFacts,
    pub nodes: NodeList,
    pub qovery_ns: NamespaceFacts,
    pub helm_releases: HelmReleaseList,
    pub crds: CrdList,
    pub deployments: DeploymentList,
    pub storage_classes: StorageClassList,
    pub csi_drivers: NameList,
    pub cluster_dns: ClusterDnsFacts,
    pub load_balancers: LoadBalancerList,
    pub metrics_api: MetricsApiFacts,
}

#[derive(Debug, Clone, Default)]
pub struct VersionFacts {
    pub raw: String,
    pub major: i64,
    pub minor: i64,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct AccessFacts {
    /// true when a SelfSubjectAccessReview allows verb "*" on every resource of every API group, cluster-wide.
    pub cluster_admin: bool,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub struct Node {
    pub name: String,
    pub ready: bool,
    pub unschedulable: bool,
    pub os: String,
    pub arch: String,
    pub provider_id: String,
    pub labels: Labels,
    pub cpu_milli: i64,
    pub memory_bytes: i64,
}

#[derive(Debug, Clone, Default)]
pub struct NodeList {
    pub items: Vec<Node>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct NamespaceFacts {
    pub exists: bool,
    pub terminating: bool,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub struct HelmRelease {
    pub name: String,
    pub status: String,
}

#[derive(Debug, Clone, Default)]
pub struct HelmReleaseList {
    /// Helm releases stored in the qovery namespace, read from the labels of the Helm storage Secrets only.
    pub items: Vec<HelmRelease>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub struct ObjectMeta {
    pub namespace: String,
    pub name: String,
    pub labels: Labels,
    pub annotations: Labels,
}

#[derive(Debug, Clone, Default)]
pub struct CrdList {
    pub items: BTreeMap<String, ObjectMeta>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct DeploymentList {
    pub items: Vec<ObjectMeta>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub struct StorageClass {
    pub name: String,
    pub default: bool,
    pub provisioner: String,
}

#[derive(Debug, Clone, Default)]
pub struct StorageClassList {
    pub items: Vec<StorageClass>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct NameList {
    pub items: Vec<String>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub struct ClusterDnsFacts {
    pub service_found: bool,
    pub ready_endpoints: i64,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq)]
pub struct LoadBalancer {
    pub namespace: String,
    pub name: String,
    pub has_address: bool,
}

#[derive(Debug, Clone, Default)]
pub struct LoadBalancerList {
    pub items: Vec<LoadBalancer>,
    pub err: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct MetricsApiFacts {
    pub available: bool,
    pub err: Option<String>,
}
