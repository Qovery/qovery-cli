//! Port of pkg/cluster/preflight/checks.go: the checks are pure functions over [`Facts`].

use super::facts::*;
use qovery_log::gofmt::quote;
use serde::Serialize;
use std::collections::BTreeMap;

/// Status of one check. Only FAIL makes the preflight fail. SKIP means the check could not be evaluated or
/// does not apply; it is never a PASS.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Default)]
pub enum Status {
    #[serde(rename = "PASS")]
    Pass,
    #[serde(rename = "WARN")]
    Warn,
    #[serde(rename = "FAIL")]
    Fail,
    #[serde(rename = "SKIP")]
    #[default]
    Skip,
}

impl Status {
    pub fn as_str(&self) -> &'static str {
        match self {
            Status::Pass => "PASS",
            Status::Warn => "WARN",
            Status::Fail => "FAIL",
            Status::Skip => "SKIP",
        }
    }
}

/// Result of one check (JSON field order of the Go struct).
#[derive(Debug, Clone, Serialize, Default, PartialEq)]
pub struct CheckResult {
    pub id: String,
    pub title: String,
    pub status: Status,
    pub detail: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub fix: String,
}

impl CheckResult {
    fn new(id: &str, title: &str) -> Self {
        CheckResult {
            id: id.to_string(),
            title: title.to_string(),
            ..Default::default()
        }
    }
}

/// The way Qovery gets installed on the cluster. `Auto` picks it from the provider of the nodes.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum InstallPath {
    Auto,
    Operator,
    Cli,
}

impl InstallPath {
    pub fn as_str(&self) -> &'static str {
        match self {
            InstallPath::Auto => "",
            InstallPath::Operator => "operator",
            InstallPath::Cli => "cli",
        }
    }
}

/// `ParseInstallPath`: the value of the `--install-path` flag.
pub fn parse_install_path(s: &str) -> Result<InstallPath, String> {
    match s.trim().to_lowercase().as_str() {
        "" | "auto" => Ok(InstallPath::Auto),
        "operator" => Ok(InstallPath::Operator),
        "cli" => Ok(InstallPath::Cli),
        _ => Err(format!(
            "invalid install path {}: use auto, operator or cli",
            quote(s)
        )),
    }
}

/// Options change which checks apply.
#[derive(Debug, Clone, Copy)]
pub struct Options {
    /// Loki stores logs in object storage instead of the default pvc storage: Loki then needs no volume.
    pub loki_object_storage: bool,
    pub install_path: InstallPath,
}

impl Default for Options {
    fn default() -> Self {
        Options {
            loki_object_storage: false,
            install_path: InstallPath::Auto,
        }
    }
}

pub const BYOK_LEGACY_DOC_URL: &str =
    "https://www.qovery.com/docs/configuration/integrations/kubernetes/byok-legacy";

pub const QOVERY_NAMESPACE: &str = "qovery";
const LEGACY_RELEASE_NAME: &str = "qovery";
pub const HELM_RELEASE_NAME_ANNOTATION: &str = "meta.helm.sh/release-name";
pub const HELM_RELEASE_NAMESPACE_ANNOTATION: &str = "meta.helm.sh/release-namespace";
pub const CRD_CERT_MANAGER: &str = "certificates.cert-manager.io";
pub const CRD_ENVOY_PROXY: &str = "envoyproxies.gateway.envoyproxy.io";
pub const CRD_GATEWAY_API: &str = "gateways.gateway.networking.k8s.io";
pub const EBS_CSI_DRIVER_NAME: &str = "ebs.csi.aws.com";
const BUILDER_CPU_MILLI: i64 = 4000;
const BUILDER_MEMORY_BYTES: i64 = 8 << 30;

/// Every check, in execution order.
pub const CHECK_IDS: [&str; 16] = [
    "KUBERNETES_VERSION",
    "RBAC_INSUFFICIENT",
    "CLOUD_PROVIDER",
    "NODE_ARCHITECTURE_UNSUPPORTED",
    "NODE_CAPACITY_INSUFFICIENT",
    "NAMESPACE_TERMINATING",
    "LEGACY_QOVERY_RELEASE",
    "INCOMPATIBLE_CERT_MANAGER",
    "EXISTING_EXTERNAL_DNS",
    "EXISTING_ENVOY_GATEWAY",
    "CRD_OWNERSHIP_CONFLICT",
    "DEFAULT_STORAGE_CLASS_MISSING",
    "EBS_CSI_DRIVER_MISSING",
    "CLUSTER_DNS_UNHEALTHY",
    "LOAD_BALANCER_UNVERIFIED",
    "METRICS_SERVER_MISSING",
];

pub fn check_title(id: &str) -> &'static str {
    match id {
        "KUBERNETES_VERSION" => "Kubernetes API and version",
        "RBAC_INSUFFICIENT" => "cluster-admin permissions",
        "CLOUD_PROVIDER" => "Cloud provider and installation",
        "NODE_ARCHITECTURE_UNSUPPORTED" => "Node OS and architecture",
        "NODE_CAPACITY_INSUFFICIENT" => "Node size for builds",
        "NAMESPACE_TERMINATING" => "Namespace qovery",
        "LEGACY_QOVERY_RELEASE" => "Legacy Qovery installation",
        "INCOMPATIBLE_CERT_MANAGER" => "Existing cert-manager",
        "EXISTING_EXTERNAL_DNS" => "Existing ExternalDNS",
        "EXISTING_ENVOY_GATEWAY" => "Existing Envoy Gateway",
        "CRD_OWNERSHIP_CONFLICT" => "Existing Gateway API CRDs",
        "DEFAULT_STORAGE_CLASS_MISSING" => "Default StorageClass",
        "EBS_CSI_DRIVER_MISSING" => "Amazon EBS CSI driver",
        "CLUSTER_DNS_UNHEALTHY" => "Cluster DNS",
        "LOAD_BALANCER_UNVERIFIED" => "LoadBalancer Services",
        "METRICS_SERVER_MISSING" => "metrics-server",
        _ => "",
    }
}

/// `Evaluate`: run every check over the collected facts. It never touches the cluster.
pub fn evaluate(f: &Facts, opts: Options) -> Vec<CheckResult> {
    if f.version.err.is_some() {
        let mut results = vec![check_kubernetes_version(&f.version)];
        for id in &CHECK_IDS[1..] {
            results.push(CheckResult {
                id: id.to_string(),
                title: check_title(id).to_string(),
                status: Status::Skip,
                detail: "Not evaluated: the Kubernetes API is unreachable.".to_string(),
                fix: String::new(),
            });
        }
        return results;
    }
    let provider = detect_provider(&f.nodes);
    let path = resolve_install_path(f, opts.install_path);
    let cli_installed = has_release(&f.helm_releases, LEGACY_RELEASE_NAME);
    let component_check = |mut check: CheckResult| -> CheckResult {
        if path == InstallPath::Operator
            || check.status == Status::Pass
            || check.status == Status::Skip
        {
            return check;
        }
        if check.id == "CRD_OWNERSHIP_CONFLICT" && cli_installed {
            return CheckResult {
                id: check.id,
                title: check.title,
                status: Status::Pass,
                detail: "The Gateway API CRDs are present and Qovery is already installed with qovery cluster install, which applies them.".to_string(),
                fix: String::new(),
            };
        }
        check.status = Status::Warn;
        check.fix = "qovery cluster install also installs this component. In the Helm values file it generates, turn off the Qovery copy before you run the install, or remove the existing one if nothing else uses it. Contact Qovery support if both must stay.".to_string();
        check
    };
    vec![
        check_kubernetes_version(&f.version),
        check_cluster_admin(&f.access),
        check_cloud_provider(&f.nodes, provider, opts.install_path, path),
        check_node_architecture(&f.nodes),
        check_node_capacity(&f.nodes),
        check_qovery_namespace(&f.qovery_ns),
        check_legacy_qovery_release(&f.helm_releases, path),
        component_check(check_cert_manager(f)),
        component_check(check_external_dns(&f.deployments)),
        component_check(check_envoy_gateway(f)),
        component_check(check_gateway_api_crds(f)),
        check_default_storage_class(&f.storage_classes, opts),
        check_ebs_csi_driver(&f.csi_drivers, provider, opts),
        check_cluster_dns(&f.cluster_dns),
        check_load_balancer(&f.load_balancers, provider),
        check_metrics_server(&f.metrics_api),
    ]
}

/// `ResolveInstallPath`: a forced path wins; otherwise Operator for AWS, GCP and Scaleway nodes, CLI for every
/// other provider, Operator when the nodes cannot be read.
pub fn resolve_install_path(f: &Facts, requested: InstallPath) -> InstallPath {
    if requested != InstallPath::Auto {
        return requested;
    }
    if f.nodes.err.is_some() {
        return InstallPath::Operator;
    }
    if operator_provider(detect_provider(&f.nodes)) {
        return InstallPath::Operator;
    }
    InstallPath::Cli
}

/// `Failed`: whether at least one check failed.
pub fn failed(results: &[CheckResult]) -> bool {
    results.iter().any(|r| r.status == Status::Fail)
}

const MIN_KUBERNETES_MINOR: i64 = 30;
const RECOMMENDED_KUBERNETES_MINOR: i64 = 31;
const NEWEST_KNOWN_KUBERNETES_MINOR: i64 = 36;

pub fn check_kubernetes_version(v: &VersionFacts) -> CheckResult {
    let mut r = CheckResult::new("KUBERNETES_VERSION", "Kubernetes API and version");
    if let Some(e) = &v.err {
        r.status = Status::Fail;
        r.detail = format!("Cannot reach the Kubernetes API: {e}");
        r.fix = "Check that your kubeconfig points to the cluster (kubectl config current-context) and that you can reach its API server (VPN, private endpoint, expired credentials).".to_string();
        return r;
    }
    if v.major != 1 || v.minor < MIN_KUBERNETES_MINOR {
        r.status = Status::Fail;
        r.detail = format!(
            "Kubernetes {} is too old. Qovery supports Kubernetes 1.{RECOMMENDED_KUBERNETES_MINOR} or later.",
            v.raw
        );
        r.fix =
            format!("Upgrade the cluster to Kubernetes 1.{RECOMMENDED_KUBERNETES_MINOR} or later.");
    } else if v.minor < RECOMMENDED_KUBERNETES_MINOR {
        r.status = Status::Warn;
        r.detail = format!(
            "Kubernetes {} is older than the supported versions (1.{RECOMMENDED_KUBERNETES_MINOR} or later). Some components need 1.31, for example the Gateway API CRDs of the Qovery Operator installation (their TLSRoute validation uses a CEL function added in Kubernetes 1.31).",
            v.raw
        );
        r.fix =
            format!("Upgrade the cluster to Kubernetes 1.{RECOMMENDED_KUBERNETES_MINOR} or later.");
    } else if v.minor > NEWEST_KNOWN_KUBERNETES_MINOR {
        r.status = Status::Warn;
        r.detail = format!(
            "Kubernetes {} is newer than the versions Qovery has validated (up to 1.{NEWEST_KNOWN_KUBERNETES_MINOR}).",
            v.raw
        );
        r.fix = "The installation can still work. If it fails, contact Qovery support with the deployment logs.".to_string();
    } else {
        r.status = Status::Pass;
        r.detail = format!("Kubernetes {}", v.raw);
    }
    r
}

pub fn check_cluster_admin(a: &AccessFacts) -> CheckResult {
    let mut r = CheckResult::new("RBAC_INSUFFICIENT", "cluster-admin permissions");
    if let Some(e) = &a.err {
        r.status = Status::Skip;
        r.detail = format!("Could not verify your permissions: {e}");
        r.fix = "Run kubectl auth can-i '*' '*' --all-namespaces. It must answer yes.".to_string();
    } else if !a.cluster_admin {
        r.status = Status::Fail;
        r.detail = "Your kubeconfig user does not have cluster-admin rights.".to_string();
        r.fix = "Use a kubeconfig user bound to the cluster-admin ClusterRole. Qovery grants cluster-admin to its agents, and Kubernetes only lets you grant permissions you hold.".to_string();
    } else {
        r.status = Status::Pass;
        r.detail = "Your kubeconfig user has cluster-admin rights.".to_string();
    }
    r
}

/// Cloud provider of the nodes (`""` when unknown).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Provider {
    Unknown,
    Aws,
    Azure,
    Gcp,
    Scaleway,
}

impl Provider {
    pub fn as_str(&self) -> &'static str {
        match self {
            Provider::Aws => "AWS",
            Provider::Gcp => "GCP",
            Provider::Scaleway => "Scaleway",
            Provider::Azure => "Azure",
            Provider::Unknown => "",
        }
    }
}

fn node_provider(n: &Node) -> Provider {
    let id = n.provider_id.to_lowercase();
    if id.starts_with("aws://") {
        return Provider::Aws;
    }
    if id.starts_with("gce://") {
        return Provider::Gcp;
    }
    if id.starts_with("scaleway://") {
        return Provider::Scaleway;
    }
    if id.starts_with("azure://") {
        return Provider::Azure;
    }
    if n.labels.contains_key("k8s.scaleway.com/pool-name") {
        return Provider::Scaleway;
    }
    Provider::Unknown
}

/// `detectProvider`: the most frequent provider; ties are broken by the greatest provider name (Go strings).
pub fn detect_provider(nodes: &NodeList) -> Provider {
    let mut counts: BTreeMap<Provider, usize> = BTreeMap::new();
    for n in &nodes.items {
        *counts.entry(node_provider(n)).or_default() += 1;
    }
    let mut best = Provider::Unknown;
    let mut best_count = 0;
    for (p, c) in counts {
        if c > best_count || (c == best_count && p.as_str() > best.as_str()) {
            best = p;
            best_count = c;
        }
    }
    best
}

fn provider_distribution(nodes: &NodeList) -> &'static str {
    for n in &nodes.items {
        let id = n.provider_id.to_lowercase();
        if id.starts_with("k3s://") {
            return "k3s";
        }
        if id.starts_with("kind://") {
            return "kind";
        }
        if id.starts_with("rke2://") {
            return "RKE2";
        }
        if n.labels
            .get("node.kubernetes.io/instance-type")
            .map(|s| s.as_str())
            == Some("k3s")
        {
            return "k3s";
        }
    }
    ""
}

fn operator_provider(p: Provider) -> bool {
    matches!(p, Provider::Aws | Provider::Gcp | Provider::Scaleway)
}

fn provider_name(nodes: &NodeList, p: Provider) -> String {
    if p != Provider::Unknown {
        return p.as_str().to_string();
    }
    let d = provider_distribution(nodes);
    if !d.is_empty() {
        return d.to_string();
    }
    "an unrecognized provider or on-premise".to_string()
}

pub fn check_cloud_provider(
    nodes: &NodeList,
    p: Provider,
    requested: InstallPath,
    path: InstallPath,
) -> CheckResult {
    let mut r = CheckResult::new("CLOUD_PROVIDER", "Cloud provider and installation");
    if let Some(e) = &nodes.err {
        r.status = Status::Skip;
        r.detail = format!(
            "Could not read the nodes: {e}. Checked for the Qovery Operator installation; run with --install-path cli if you install with qovery cluster install."
        );
        return r;
    }
    let name = provider_name(nodes, p);
    let supported = operator_provider(p);
    if path == InstallPath::Operator && supported {
        r.status = Status::Pass;
        r.detail = format!(
            "{name} cluster. Use the Qovery Operator installation (the Helm command shown in the Qovery Console)."
        );
    } else if path == InstallPath::Operator {
        r.status = Status::Warn;
        r.detail = format!(
            "The nodes run on {name}. The Qovery Operator installation supports EKS, GKE and Scaleway Kapsule only."
        );
        r.fix = format!(
            "Install Qovery with the Qovery CLI (qovery cluster install) and run the preflight with --install-path cli: {BYOK_LEGACY_DOC_URL}"
        );
    } else if requested == InstallPath::Cli && supported {
        r.status = Status::Pass;
        r.detail = format!(
            "{name} cluster, checked for the installation with the Qovery CLI (qovery cluster install). The Qovery Operator installation is also available for {name}."
        );
    } else {
        r.status = Status::Pass;
        r.detail = format!(
            "The nodes run on {name}. Install Qovery with the Qovery CLI: qovery cluster install ({BYOK_LEGACY_DOC_URL}). The Qovery Operator installation covers EKS, GKE and Scaleway Kapsule."
        );
    }
    r
}

pub fn check_node_architecture(nodes: &NodeList) -> CheckResult {
    let mut r = CheckResult::new("NODE_ARCHITECTURE_UNSUPPORTED", "Node OS and architecture");
    if let Some(e) = &nodes.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read the nodes: {e}");
        return r;
    }
    let mut supported = 0;
    let mut others: Vec<String> = Vec::new();
    for n in &nodes.items {
        if n.os == "linux" && (n.arch == "amd64" || n.arch == "arm64") {
            if n.ready {
                supported += 1;
            }
            continue;
        }
        others.push(format!("{} ({}/{})", n.name, n.os, n.arch));
    }
    if supported == 0 {
        r.status = Status::Fail;
        r.detail = "No Ready Linux node with an amd64 or arm64 CPU.".to_string();
        r.fix = "Add a Linux node pool with amd64 or arm64 nodes. Qovery images are published for these architectures only.".to_string();
    } else if !others.is_empty() {
        r.status = Status::Warn;
        r.detail = format!(
            "{supported} Ready Linux amd64/arm64 node(s). Qovery cannot run on: {}.",
            others.join(", ")
        );
        r.fix = "Keep at least one Linux amd64 or arm64 node pool for Qovery and its builds."
            .to_string();
    } else {
        r.status = Status::Pass;
        r.detail = format!("{supported} Ready Linux amd64/arm64 node(s)");
    }
    r
}

pub fn check_node_capacity(nodes: &NodeList) -> CheckResult {
    let mut r = CheckResult::new("NODE_CAPACITY_INSUFFICIENT", "Node size for builds");
    if let Some(e) = &nodes.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read the nodes: {e}");
        return r;
    }
    for n in &nodes.items {
        if n.ready
            && !n.unschedulable
            && n.os == "linux"
            && n.cpu_milli >= BUILDER_CPU_MILLI
            && n.memory_bytes >= BUILDER_MEMORY_BYTES
        {
            r.status = Status::Pass;
            r.detail = format!(
                "Node {} can host a builder pod (4 vCPU, 8 GiB) when it has that much free.",
                n.name
            );
            return r;
        }
    }
    r.status = Status::Warn;
    r.detail = "No schedulable node has 4 vCPU and 8 GiB allocatable. Builds with the default settings will stay on Provisioning builder.".to_string();
    r.fix = "Add a node with at least 4 vCPU and 8 GiB free (for example an 8 vCPU node), or lower CPU and RAM in the build settings of each service. With a node autoscaler such as Karpenter, make sure it can create such a node.".to_string();
    r
}

pub fn check_qovery_namespace(ns: &NamespaceFacts) -> CheckResult {
    let mut r = CheckResult::new("NAMESPACE_TERMINATING", "Namespace qovery");
    if let Some(e) = &ns.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read namespace qovery: {e}");
    } else if ns.terminating {
        r.status = Status::Fail;
        r.detail = "Namespace qovery is being deleted.".to_string();
        r.fix = "Wait until kubectl get namespace qovery returns NotFound, then install again. If it stays Terminating, check the finalizers of the resources left in it.".to_string();
    } else if !ns.exists {
        r.status = Status::Pass;
        r.detail = "Namespace qovery does not exist yet. The installation creates it.".to_string();
    } else {
        r.status = Status::Pass;
        r.detail = "Namespace qovery exists and is active.".to_string();
    }
    r
}

pub fn check_legacy_qovery_release(releases: &HelmReleaseList, path: InstallPath) -> CheckResult {
    let mut r = CheckResult::new("LEGACY_QOVERY_RELEASE", "Legacy Qovery installation");
    if let Some(e) = &releases.err {
        r.status = Status::Skip;
        r.detail = format!("Could not list the Helm releases of namespace qovery: {e}");
        return r;
    }
    for rel in &releases.items {
        if rel.name != LEGACY_RELEASE_NAME {
            continue;
        }
        if path == InstallPath::Cli {
            r.status = Status::Pass;
            r.detail = format!(
                "Namespace qovery holds the Qovery Helm release qovery (status {}), installed with the Qovery CLI. qovery cluster install manages this release.",
                rel.status
            );
            return r;
        }
        r.status = Status::Fail;
        r.detail = format!(
            "Namespace qovery holds the legacy Qovery Helm release qovery (status {}).",
            rel.status
        );
        r.fix = "The Qovery Operator does not take over an installation made with qovery cluster install. Keep that installation (run the preflight with --install-path cli), or uninstall it (helm -n qovery uninstall qovery) and remove its cert-manager, ExternalDNS and ingress resources before installing the Operator.".to_string();
        return r;
    }
    r.status = Status::Pass;
    r.detail = "No legacy Qovery Helm release found.".to_string();
    r
}

fn annotation<'a>(m: &'a ObjectMeta, key: &str) -> &'a str {
    m.annotations
        .get(key)
        .map(|s| s.as_str())
        .unwrap_or_default()
}

fn label<'a>(m: &'a ObjectMeta, key: &str) -> &'a str {
    m.labels.get(key).map(|s| s.as_str()).unwrap_or_default()
}

fn helm_owner(m: &ObjectMeta) -> String {
    let name = annotation(m, HELM_RELEASE_NAME_ANNOTATION);
    if name.is_empty() {
        return String::new();
    }
    format!(
        "{}/{}",
        annotation(m, HELM_RELEASE_NAMESPACE_ANNOTATION),
        name
    )
}

fn owner_description(m: &ObjectMeta) -> String {
    let owner = helm_owner(m);
    if !owner.is_empty() {
        return format!("Helm release {owner}");
    }
    "not managed by Helm".to_string()
}

/// `ownedByQovery`.
fn owned_by_qovery(m: &ObjectMeta, releases: &HelmReleaseList, names: &[&str]) -> bool {
    if annotation(m, HELM_RELEASE_NAMESPACE_ANNOTATION) == QOVERY_NAMESPACE {
        let release = annotation(m, HELM_RELEASE_NAME_ANNOTATION);
        if release == LEGACY_RELEASE_NAME {
            return true;
        }
        return names.contains(&release);
    }
    if !helm_owner(m).is_empty() {
        return false;
    }
    releases
        .items
        .iter()
        .any(|rel| names.contains(&rel.name.as_str()))
}

fn deployment_matches(d: &ObjectMeta, label_names: &[&str], names: &[&str]) -> bool {
    for key in ["app.kubernetes.io/name", "app"] {
        if label_names.contains(&label(d, key)) {
            return true;
        }
    }
    names.contains(&d.name.as_str())
}

fn foreign_deployments(
    deps: &DeploymentList,
    releases: &HelmReleaseList,
    label_names: &[&str],
    names: &[&str],
    qovery_releases: &[&str],
) -> Vec<String> {
    let mut found: Vec<String> = Vec::new();
    for d in &deps.items {
        if !deployment_matches(d, label_names, names) {
            continue;
        }
        if d.namespace == QOVERY_NAMESPACE && owned_by_qovery(d, releases, qovery_releases) {
            continue;
        }
        found.push(format!(
            "Deployment {}/{} ({})",
            d.namespace,
            d.name,
            owner_description(d)
        ));
    }
    found.sort();
    found
}

fn foreign_crd(
    crds: &CrdList,
    releases: &HelmReleaseList,
    name: &str,
    qovery_releases: &[&str],
) -> String {
    match crds.items.get(name) {
        Some(m) if !owned_by_qovery(m, releases, qovery_releases) => {
            format!("CRD {name} ({})", owner_description(m))
        }
        _ => String::new(),
    }
}

pub fn join_evidence(items: &[String]) -> String {
    const MAX: usize = 4;
    if items.len() > MAX {
        return format!(
            "{}; and {} more",
            items[..MAX].join("; "),
            items.len() - MAX
        );
    }
    items.join("; ")
}

fn err_str(e: &Option<String>) -> String {
    match e {
        Some(e) => e.clone(),
        None => "<nil>".to_string(),
    }
}

pub fn check_cert_manager(f: &Facts) -> CheckResult {
    let mut r = CheckResult::new("INCOMPATIBLE_CERT_MANAGER", "Existing cert-manager");
    if f.crds.err.is_some() && f.deployments.err.is_some() {
        r.status = Status::Skip;
        r.detail = format!(
            "Could not read CRDs or Deployments: {}",
            err_str(&f.crds.err)
        );
        return r;
    }
    let mut evidence = Vec::new();
    let e = foreign_crd(
        &f.crds,
        &f.helm_releases,
        CRD_CERT_MANAGER,
        &["cert-manager"],
    );
    if !e.is_empty() {
        evidence.push(e);
    }
    evidence.extend(foreign_deployments(
        &f.deployments,
        &f.helm_releases,
        &["cert-manager"],
        &["cert-manager"],
        &["cert-manager"],
    ));
    if evidence.is_empty() {
        r.status = Status::Pass;
        r.detail = "No cert-manager installation found outside Qovery.".to_string();
        return r;
    }
    r.status = Status::Fail;
    r.detail = format!(
        "cert-manager is already installed: {}.",
        join_evidence(&evidence)
    );
    r.fix = "Qovery installs its own cert-manager in namespace qovery and does not take over an existing one. Use a cluster without cert-manager, or uninstall it and its CRDs if nothing else on the cluster uses it.".to_string();
    r
}

pub fn check_external_dns(deps: &DeploymentList) -> CheckResult {
    let mut r = CheckResult::new("EXISTING_EXTERNAL_DNS", "Existing ExternalDNS");
    if let Some(e) = &deps.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read Deployments: {e}");
        return r;
    }
    let mut evidence: Vec<String> = Vec::new();
    for d in &deps.items {
        let matches = label(d, "app.kubernetes.io/name") == "external-dns"
            || label(d, "app") == "external-dns"
            || d.name.contains("external-dns");
        if !matches
            || (d.namespace == QOVERY_NAMESPACE
                && annotation(d, HELM_RELEASE_NAMESPACE_ANNOTATION) == QOVERY_NAMESPACE)
        {
            continue;
        }
        evidence.push(format!(
            "Deployment {}/{} ({})",
            d.namespace,
            d.name,
            owner_description(d)
        ));
    }
    evidence.sort();
    if evidence.is_empty() {
        r.status = Status::Pass;
        r.detail = "No ExternalDNS installation found outside Qovery.".to_string();
        return r;
    }
    r.status = Status::Fail;
    r.detail = format!(
        "ExternalDNS is already installed: {}.",
        join_evidence(&evidence)
    );
    r.fix = "Qovery installs its own ExternalDNS for the Qovery-provided domain and does not take over an existing one. Use a cluster without ExternalDNS, or uninstall it if nothing else uses it.".to_string();
    r
}

pub fn check_envoy_gateway(f: &Facts) -> CheckResult {
    let mut r = CheckResult::new("EXISTING_ENVOY_GATEWAY", "Existing Envoy Gateway");
    if f.crds.err.is_some() && f.deployments.err.is_some() {
        r.status = Status::Skip;
        r.detail = format!(
            "Could not read CRDs or Deployments: {}",
            err_str(&f.crds.err)
        );
        return r;
    }
    let mut evidence = Vec::new();
    let e = foreign_crd(
        &f.crds,
        &f.helm_releases,
        CRD_ENVOY_PROXY,
        &["envoy-gateway-crd"],
    );
    if !e.is_empty() {
        evidence.push(e);
    }
    evidence.extend(foreign_deployments(
        &f.deployments,
        &f.helm_releases,
        &["gateway-helm"],
        &["envoy-gateway"],
        &["envoy-gateway"],
    ));
    if evidence.is_empty() {
        r.status = Status::Pass;
        r.detail = "No Envoy Gateway installation found outside Qovery.".to_string();
        return r;
    }
    r.status = Status::Fail;
    r.detail = format!(
        "Envoy Gateway is already installed: {}.",
        join_evidence(&evidence)
    );
    r.fix = "Qovery installs its own Envoy Gateway in namespace qovery and does not take over an existing one. Use a cluster without Envoy Gateway, or uninstall it and its CRDs if nothing else uses it.".to_string();
    r
}

/// `^v1\.[0-4](\.|$)`.
fn gateway_bundle_before_15(bundle: &str) -> bool {
    let Some(rest) = bundle.strip_prefix("v1.") else {
        return false;
    };
    let mut chars = rest.chars();
    match chars.next() {
        Some(c) if ('0'..='4').contains(&c) => matches!(chars.next(), None | Some('.')),
        _ => false,
    }
}

pub fn check_gateway_api_crds(f: &Facts) -> CheckResult {
    let mut r = CheckResult::new("CRD_OWNERSHIP_CONFLICT", "Existing Gateway API CRDs");
    if let Some(e) = &f.crds.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read CRDs: {e}");
        return r;
    }
    let Some(m) = f.crds.items.get(CRD_GATEWAY_API) else {
        r.status = Status::Pass;
        r.detail = "No Gateway API CRDs installed yet. Qovery installs them.".to_string();
        return r;
    };
    if owned_by_qovery(m, &f.helm_releases, &["envoy-gateway-crd"]) {
        r.status = Status::Pass;
        r.detail = "The Gateway API CRDs belong to the Qovery installation.".to_string();
        return r;
    }
    let channel = annotation(m, "gateway.networking.k8s.io/channel");
    let bundle = annotation(m, "gateway.networking.k8s.io/bundle-version");
    r.status = Status::Warn;
    r.detail = format!(
        "The Gateway API CRDs are already installed ({}, channel {}, bundle {}). Qovery replaces them with the experimental channel, bundle v1.6.1.",
        owner_description(m),
        quote(channel),
        quote(bundle)
    );
    if gateway_bundle_before_15(bundle) {
        r.detail.push_str(" Bundles older than v1.5 are refused by the Gateway API upgrade policy Qovery installs.");
    }
    r.fix = "If another tool installed and reconciles these CRDs (GKE Gateway API, Istio, Cilium, a load balancer controller), it can conflict with Qovery. On GKE, disable the GKE-managed Gateway API if nothing uses it. Otherwise, contact Qovery support before installing.".to_string();
    r
}

pub fn check_default_storage_class(scs: &StorageClassList, opts: Options) -> CheckResult {
    let mut r = CheckResult::new("DEFAULT_STORAGE_CLASS_MISSING", "Default StorageClass");
    if opts.loki_object_storage {
        r.status = Status::Skip;
        r.detail = "Loki uses object storage and needs no volume.".to_string();
        return r;
    }
    if let Some(e) = &scs.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read StorageClasses: {e}");
        return r;
    }
    let all: Vec<&str> = scs.items.iter().map(|s| s.name.as_str()).collect();
    let defaults: Vec<&str> = scs
        .items
        .iter()
        .filter(|s| s.default)
        .map(|s| s.name.as_str())
        .collect();
    if defaults.is_empty() {
        r.status = Status::Fail;
        r.detail = "No default StorageClass. Loki stores logs on a volume by default and will stay Pending.".to_string();
        if !all.is_empty() {
            r.detail
                .push_str(&format!(" StorageClasses found: {}.", all.join(", ")));
        }
        r.fix = "Mark one as default: kubectl annotate storageclass <NAME> storageclass.kubernetes.io/is-default-class=true. Or choose object storage for Loki before the first deployment and run the preflight with --loki-object-storage.".to_string();
    } else if defaults.len() > 1 {
        r.status = Status::Warn;
        r.detail = format!(
            "Several default StorageClasses: {}. Kubernetes picks the most recent one.",
            defaults.join(", ")
        );
        r.fix = "Keep a single default StorageClass.".to_string();
    } else {
        r.status = Status::Pass;
        r.detail = format!("Default StorageClass: {}", defaults[0]);
    }
    r
}

pub fn check_ebs_csi_driver(drivers: &NameList, p: Provider, opts: Options) -> CheckResult {
    let mut r = CheckResult::new("EBS_CSI_DRIVER_MISSING", "Amazon EBS CSI driver");
    if p != Provider::Aws {
        r.status = Status::Skip;
        r.detail = "Only needed on AWS.".to_string();
        return r;
    }
    if let Some(e) = &drivers.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read CSIDrivers: {e}");
        return r;
    }
    if drivers.items.iter().any(|d| d == EBS_CSI_DRIVER_NAME) {
        r.status = Status::Pass;
        r.detail = "The Amazon EBS CSI driver is installed.".to_string();
        return r;
    }
    r.status = if opts.loki_object_storage {
        Status::Warn
    } else {
        Status::Fail
    };
    r.detail = "The Amazon EBS CSI driver is not installed. Volumes for Loki, container databases and service storage cannot be created.".to_string();
    r.fix =
        "Add the Amazon EBS CSI driver add-on to the EKS cluster, with its IAM role.".to_string();
    r
}

pub fn check_cluster_dns(dns: &ClusterDnsFacts) -> CheckResult {
    let mut r = CheckResult::new("CLUSTER_DNS_UNHEALTHY", "Cluster DNS");
    const LOOKUP: &str = "kubectl -n default run qovery-dns-preflight --rm -i --restart=Never --image=busybox:1.36 -- nslookup engine.qovery.com";
    if let Some(e) = &dns.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read the cluster DNS Service: {e}");
        r.fix = format!("Check name resolution from a Pod: {LOOKUP}");
    } else if !dns.service_found {
        r.status = Status::Warn;
        r.detail = "No kube-dns Service in namespace kube-system, so the cluster DNS could not be verified.".to_string();
        r.fix = format!(
            "Check name resolution from a Pod. The lookup must return an address: {LOOKUP}"
        );
    } else if dns.ready_endpoints == 0 {
        r.status = Status::Fail;
        r.detail = "The kube-dns Service has no ready endpoint. The Qovery agents cannot resolve Qovery and will not connect.".to_string();
        r.fix = format!(
            "Fix CoreDNS (kubectl -n kube-system get pods -l k8s-app=kube-dns) before installing, then check: {LOOKUP}"
        );
    } else {
        r.status = Status::Pass;
        r.detail = format!(
            "kube-dns has {} ready endpoint(s). Resolution of public names is not tested; run: {LOOKUP}",
            dns.ready_endpoints
        );
    }
    r
}

pub fn check_load_balancer(lbs: &LoadBalancerList, p: Provider) -> CheckResult {
    let mut r = CheckResult::new("LOAD_BALANCER_UNVERIFIED", "LoadBalancer Services");
    let mut fix = "The cluster must create a public load balancer for a Service of type LoadBalancer (on-premise, use MetalLB or an equivalent).".to_string();
    if p == Provider::Aws {
        fix.push_str(" On EKS, install the AWS Load Balancer Controller or use the in-tree provider, and tag the public subnets with kubernetes.io/role/elb=1.");
    }
    if let Some(e) = &lbs.err {
        r.status = Status::Skip;
        r.detail = format!("Could not read Services: {e}");
        r.fix = fix;
        return r;
    }
    let mut with_address: Vec<String> = Vec::new();
    let mut pending: Vec<String> = Vec::new();
    for lb in &lbs.items {
        let name = format!("{}/{}", lb.namespace, lb.name);
        if lb.has_address {
            with_address.push(name);
        } else {
            pending.push(name);
        }
    }
    if !with_address.is_empty() {
        r.status = Status::Pass;
        r.detail = format!(
            "{} LoadBalancer Service(s) received an address, for example {}.",
            with_address.len(),
            with_address[0]
        );
    } else if !pending.is_empty() {
        r.status = Status::Warn;
        r.detail = format!(
            "LoadBalancer Service(s) without an address: {}. The Service that exposes your Qovery applications would stay without an address too.",
            join_evidence(&pending)
        );
        r.fix = fix;
    } else {
        r.status = Status::Skip;
        r.detail = "No LoadBalancer Service on the cluster, so LoadBalancer support could not be confirmed.".to_string();
        r.fix = fix;
    }
    r
}

pub fn check_metrics_server(m: &MetricsApiFacts) -> CheckResult {
    let mut r = CheckResult::new("METRICS_SERVER_MISSING", "metrics-server");
    if m.available {
        r.status = Status::Pass;
        r.detail = "The metrics.k8s.io API is available.".to_string();
        return r;
    }
    r.status = Status::Warn;
    r.detail =
        "The metrics.k8s.io API is not available. Autoscaling of your services will not work."
            .to_string();
    if let Some(e) = &m.err {
        r.detail.push_str(&format!(" ({e})"));
    }
    r.fix = "Install metrics-server. On EKS, add the metrics-server add-on. GKE and Kapsule include it.".to_string();
    r
}

fn has_release(releases: &HelmReleaseList, name: &str) -> bool {
    releases.items.iter().any(|r| r.name == name)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn labels(pairs: &[(&str, &str)]) -> Labels {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect()
    }

    fn meta(namespace: &str, name: &str, l: &[(&str, &str)], a: &[(&str, &str)]) -> ObjectMeta {
        ObjectMeta {
            namespace: namespace.into(),
            name: name.into(),
            labels: labels(l),
            annotations: labels(a),
        }
    }

    fn node(name: &str, provider_id: &str) -> Node {
        Node {
            name: name.into(),
            provider_id: provider_id.into(),
            ..Default::default()
        }
    }

    /// The facts of an EKS cluster that is ready for Qovery.
    fn clean_eks() -> Facts {
        Facts {
            version: VersionFacts {
                raw: "v1.33.2-eks-1".into(),
                major: 1,
                minor: 33,
                err: None,
            },
            access: AccessFacts {
                cluster_admin: true,
                err: None,
            },
            nodes: NodeList {
                items: vec![Node {
                    name: "ip-10-0-1-1".into(),
                    ready: true,
                    os: "linux".into(),
                    arch: "amd64".into(),
                    provider_id: "aws:///eu-west-3a/i-0123".into(),
                    cpu_milli: 7910,
                    memory_bytes: 30 << 30,
                    labels: labels(&[("eks.amazonaws.com/nodegroup", "default")]),
                    ..Default::default()
                }],
                err: None,
            },
            storage_classes: StorageClassList {
                items: vec![StorageClass {
                    name: "gp3".into(),
                    default: true,
                    provisioner: EBS_CSI_DRIVER_NAME.into(),
                }],
                err: None,
            },
            csi_drivers: NameList {
                items: vec![EBS_CSI_DRIVER_NAME.into(), "efs.csi.aws.com".into()],
                err: None,
            },
            cluster_dns: ClusterDnsFacts {
                service_found: true,
                ready_endpoints: 2,
                err: None,
            },
            load_balancers: LoadBalancerList {
                items: vec![LoadBalancer {
                    namespace: "default".into(),
                    name: "web".into(),
                    has_address: true,
                }],
                err: None,
            },
            metrics_api: MetricsApiFacts {
                available: true,
                err: None,
            },
            ..Default::default()
        }
    }

    pub fn result_by_id(results: &[CheckResult], id: &str) -> CheckResult {
        results
            .iter()
            .find(|r| r.id == id)
            .cloned()
            .unwrap_or_else(|| panic!("no result with id {id}"))
    }

    #[test]
    fn evaluate_clean_cluster_passes() {
        let results = evaluate(&clean_eks(), Options::default());
        assert_eq!(results.len(), CHECK_IDS.len());
        for (i, r) in results.iter().enumerate() {
            assert_eq!(r.id, CHECK_IDS[i], "checks run in the documented order");
            assert_eq!(r.status, Status::Pass, "{}: {}", r.id, r.detail);
        }
        assert!(!failed(&results));
    }

    #[test]
    fn evaluate_unreachable_api_skips_everything_else() {
        let f = Facts {
            version: VersionFacts {
                err: Some("connection refused".into()),
                ..Default::default()
            },
            ..Default::default()
        };
        let results = evaluate(&f, Options::default());
        assert_eq!(results.len(), CHECK_IDS.len());
        assert_eq!(results[0].status, Status::Fail);
        for (i, r) in results[1..].iter().enumerate() {
            assert_eq!(r.id, CHECK_IDS[i + 1]);
            assert_eq!(r.status, Status::Skip, "{}", r.id);
            assert!(!r.title.is_empty(), "{}", r.id);
        }
    }

    #[test]
    fn check_titles_match_results() {
        for r in evaluate(&clean_eks(), Options::default()) {
            assert_eq!(check_title(&r.id), r.title, "{}", r.id);
        }
    }

    #[test]
    fn kubernetes_version() {
        let v = |raw: &str, major, minor| VersionFacts {
            raw: raw.into(),
            major,
            minor,
            err: None,
        };
        let cases = [
            (
                "unreachable API",
                VersionFacts {
                    err: Some("dial tcp: i/o timeout".into()),
                    ..Default::default()
                },
                Status::Fail,
            ),
            ("1.29 is too old", v("v1.29.10", 1, 29), Status::Fail),
            ("0.x is too old", v("v0.9.0", 0, 40), Status::Fail),
            ("1.30 warns", v("v1.30.4", 1, 30), Status::Warn),
            ("1.31 passes", v("v1.31.4+k3s1", 1, 31), Status::Pass),
            ("1.36 passes", v("v1.36.0", 1, 36), Status::Pass),
            ("1.37 is not validated", v("v1.37.0", 1, 37), Status::Warn),
        ];
        for (name, facts, status) in cases {
            let r = check_kubernetes_version(&facts);
            assert_eq!(r.status, status, "{name}: {}", r.detail);
            if facts.err.is_none() && facts.minor < 31 {
                assert!(r.detail.contains("1.31 or later"), "{name}");
            }
            if status != Status::Pass {
                assert!(!r.fix.is_empty(), "{name}");
            }
        }
    }

    #[test]
    fn cluster_admin() {
        assert_eq!(
            check_cluster_admin(&AccessFacts {
                cluster_admin: false,
                err: None
            })
            .status,
            Status::Fail
        );
        assert_eq!(
            check_cluster_admin(&AccessFacts {
                cluster_admin: false,
                err: Some("forbidden".into())
            })
            .status,
            Status::Skip
        );
        assert_eq!(
            check_cluster_admin(&AccessFacts {
                cluster_admin: true,
                err: None
            })
            .status,
            Status::Pass
        );
    }

    #[test]
    fn cloud_provider_never_fails() {
        type Case<'a> = (
            &'a str,
            &'a str,
            &'a [(&'a str, &'a str)],
            InstallPath,
            &'a str,
        );
        let cases: [Case; 7] = [
            (
                "EKS",
                "aws:///eu-west-3a/i-1",
                &[],
                InstallPath::Operator,
                "AWS",
            ),
            (
                "GKE",
                "gce://project/europe-west1-b/gke-node",
                &[],
                InstallPath::Operator,
                "GCP",
            ),
            (
                "Kapsule",
                "scaleway://instance/fr-par-1/abc",
                &[],
                InstallPath::Operator,
                "Scaleway",
            ),
            (
                "Kapsule by label",
                "",
                &[("k8s.scaleway.com/pool-name", "default")],
                InstallPath::Operator,
                "Scaleway",
            ),
            (
                "AKS",
                "azure:///subscriptions/x/resourceGroups/y",
                &[],
                InstallPath::Cli,
                "Azure",
            ),
            ("k3s", "k3s://node-1", &[], InstallPath::Cli, "k3s"),
            ("on-premise", "", &[], InstallPath::Cli, "on-premise"),
        ];
        for (name, provider_id, l, path, mention) in cases {
            let mut n = node("n1", provider_id);
            n.labels = labels(l);
            let nodes = NodeList {
                items: vec![n],
                err: None,
            };
            let f = Facts {
                nodes: nodes.clone(),
                ..Default::default()
            };
            let resolved = resolve_install_path(&f, InstallPath::Auto);
            assert_eq!(resolved, path, "{name}");
            let r =
                check_cloud_provider(&nodes, detect_provider(&nodes), InstallPath::Auto, resolved);
            assert_eq!(r.status, Status::Pass, "{name}");
            assert!(r.detail.contains(mention), "{name}");
            if path == InstallPath::Cli {
                assert!(r.detail.contains("qovery cluster install"), "{name}");
                assert!(r.detail.contains(BYOK_LEGACY_DOC_URL), "{name}");
            } else {
                assert!(r.detail.contains("Qovery Operator installation"), "{name}");
            }
        }
    }

    #[test]
    fn install_path_override() {
        let aks = NodeList {
            items: vec![node("n1", "azure:///subscriptions/x")],
            err: None,
        };
        let f = Facts {
            nodes: aks.clone(),
            ..Default::default()
        };
        let r = check_cloud_provider(
            &aks,
            detect_provider(&aks),
            InstallPath::Operator,
            resolve_install_path(&f, InstallPath::Operator),
        );
        assert_eq!(
            r.status,
            Status::Warn,
            "forcing the Operator on an unsupported provider warns, never fails"
        );
        assert!(r.fix.contains("--install-path cli"));

        let eks = clean_eks();
        assert_eq!(
            resolve_install_path(&eks, InstallPath::Cli),
            InstallPath::Cli
        );
        let r = check_cloud_provider(
            &eks.nodes,
            detect_provider(&eks.nodes),
            InstallPath::Cli,
            InstallPath::Cli,
        );
        assert_eq!(r.status, Status::Pass);
        assert!(r.detail.contains("also available"));

        let unreadable = Facts {
            nodes: NodeList {
                items: vec![],
                err: Some("forbidden".into()),
            },
            ..Default::default()
        };
        assert_eq!(
            resolve_install_path(&unreadable, InstallPath::Auto),
            InstallPath::Operator
        );

        for input in ["", "auto", "Operator", "cli"] {
            assert!(parse_install_path(input).is_ok(), "{input}");
        }
        assert!(parse_install_path("helm").is_err());
    }

    /// An on-premise cluster (k3s) that already runs cert-manager, ExternalDNS, Envoy Gateway and the Qovery
    /// release installed by qovery cluster install.
    fn on_premise() -> Facts {
        let mut f = clean_eks();
        f.nodes = NodeList {
            items: vec![Node {
                name: "k3s-1".into(),
                ready: true,
                os: "linux".into(),
                arch: "amd64".into(),
                provider_id: "k3s://k3s-1".into(),
                cpu_milli: 8000,
                memory_bytes: 32 << 30,
                ..Default::default()
            }],
            err: None,
        };
        f.helm_releases = HelmReleaseList {
            items: vec![HelmRelease {
                name: "qovery".into(),
                status: "deployed".into(),
            }],
            err: None,
        };
        f.crds.items.insert(
            CRD_CERT_MANAGER.into(),
            meta(
                "",
                CRD_CERT_MANAGER,
                &[],
                &[
                    (HELM_RELEASE_NAME_ANNOTATION, "cert-manager"),
                    (HELM_RELEASE_NAMESPACE_ANNOTATION, "cert-manager"),
                ],
            ),
        );
        f.crds
            .items
            .insert(CRD_GATEWAY_API.into(), meta("", CRD_GATEWAY_API, &[], &[]));
        f.deployments.items = vec![
            meta(
                "kube-system",
                "external-dns",
                &[("app.kubernetes.io/name", "external-dns")],
                &[],
            ),
            meta(
                "envoy-gateway-system",
                "envoy-gateway",
                &[("app.kubernetes.io/name", "gateway-helm")],
                &[],
            ),
        ];
        f.csi_drivers = NameList {
            items: vec!["driver.longhorn.io".into()],
            err: None,
        };
        f
    }

    #[test]
    fn cli_install_path_warns_on_existing_components() {
        let results = evaluate(&on_premise(), Options::default());
        assert!(
            !failed(&results),
            "an on-premise cluster on the CLI installation must not fail on existing components"
        );
        assert_eq!(
            result_by_id(&results, "CLOUD_PROVIDER").status,
            Status::Pass
        );
        let legacy = result_by_id(&results, "LEGACY_QOVERY_RELEASE");
        assert_eq!(legacy.status, Status::Pass);
        assert!(legacy.detail.contains("qovery cluster install"));
        for id in [
            "INCOMPATIBLE_CERT_MANAGER",
            "EXISTING_EXTERNAL_DNS",
            "EXISTING_ENVOY_GATEWAY",
        ] {
            let r = result_by_id(&results, id);
            assert_eq!(r.status, Status::Warn, "{id}");
            assert!(r.fix.contains("Helm values file"), "{id}");
        }
        assert_eq!(
            result_by_id(&results, "CRD_OWNERSHIP_CONFLICT").status,
            Status::Pass
        );

        let forced = evaluate(
            &on_premise(),
            Options {
                install_path: InstallPath::Operator,
                ..Default::default()
            },
        );
        assert!(failed(&forced));
        assert_eq!(result_by_id(&forced, "CLOUD_PROVIDER").status, Status::Warn);
        assert_eq!(
            result_by_id(&forced, "LEGACY_QOVERY_RELEASE").status,
            Status::Fail
        );
        assert_eq!(
            result_by_id(&forced, "INCOMPATIBLE_CERT_MANAGER").status,
            Status::Fail
        );
    }

    #[test]
    fn cli_install_path_clean_cluster_and_own_components() {
        let mut f = on_premise();
        f.helm_releases = HelmReleaseList::default();
        f.crds.items.clear();
        f.deployments.items.clear();
        let results = evaluate(&f, Options::default());
        for id in [
            "INCOMPATIBLE_CERT_MANAGER",
            "EXISTING_EXTERNAL_DNS",
            "EXISTING_ENVOY_GATEWAY",
            "CRD_OWNERSHIP_CONFLICT",
        ] {
            assert_eq!(result_by_id(&results, id).status, Status::Pass, "{id}");
        }
        f.helm_releases = HelmReleaseList {
            items: vec![HelmRelease {
                name: "qovery".into(),
                status: "deployed".into(),
            }],
            err: None,
        };
        f.deployments.items = vec![meta(
            "qovery",
            "cert-manager",
            &[("app.kubernetes.io/name", "cert-manager")],
            &[
                (HELM_RELEASE_NAME_ANNOTATION, "qovery"),
                (HELM_RELEASE_NAMESPACE_ANNOTATION, "qovery"),
            ],
        )];
        assert_eq!(
            result_by_id(
                &evaluate(&f, Options::default()),
                "INCOMPATIBLE_CERT_MANAGER"
            )
            .status,
            Status::Pass
        );
    }

    #[test]
    fn node_architecture_and_capacity() {
        let n = |name: &str, os: &str, arch: &str, cpu, mem| Node {
            name: name.into(),
            ready: true,
            os: os.into(),
            arch: arch.into(),
            cpu_milli: cpu,
            memory_bytes: mem,
            ..Default::default()
        };
        let only_s390x = NodeList {
            items: vec![n("z1", "linux", "s390x", 16000, 64 << 30)],
            err: None,
        };
        assert_eq!(check_node_architecture(&only_s390x).status, Status::Fail);

        let mixed = NodeList {
            items: vec![
                n("a1", "linux", "arm64", 0, 0),
                n("w1", "windows", "amd64", 0, 0),
            ],
            err: None,
        };
        let r = check_node_architecture(&mixed);
        assert_eq!(r.status, Status::Warn);
        assert!(r.detail.contains("w1 (windows/amd64)"));

        let small = NodeList {
            items: vec![n("s1", "linux", "amd64", 3920, 15 << 30)],
            err: None,
        };
        assert_eq!(check_node_capacity(&small).status, Status::Warn);

        let mut cordoned = n("b1", "linux", "amd64", 8000, 32 << 30);
        cordoned.unschedulable = true;
        assert_eq!(
            check_node_capacity(&NodeList {
                items: vec![cordoned],
                err: None
            })
            .status,
            Status::Warn
        );
    }

    #[test]
    fn qovery_namespace_and_legacy_release() {
        assert_eq!(
            check_qovery_namespace(&NamespaceFacts {
                exists: true,
                terminating: true,
                err: None
            })
            .status,
            Status::Fail
        );
        assert_eq!(
            check_qovery_namespace(&NamespaceFacts::default()).status,
            Status::Pass
        );

        let legacy_release = HelmReleaseList {
            items: vec![HelmRelease {
                name: "qovery".into(),
                status: "deployed".into(),
            }],
            err: None,
        };
        let legacy = check_legacy_qovery_release(&legacy_release, InstallPath::Operator);
        assert_eq!(legacy.status, Status::Fail);
        assert!(legacy.fix.contains("helm -n qovery uninstall qovery"));
        assert!(legacy.fix.contains("--install-path cli"));
        assert_eq!(
            check_legacy_qovery_release(&legacy_release, InstallPath::Cli).status,
            Status::Pass
        );

        let v2 = HelmReleaseList {
            items: vec![
                HelmRelease {
                    name: "qovery-operator".into(),
                    status: "deployed".into(),
                },
                HelmRelease {
                    name: "cert-manager".into(),
                    status: "deployed".into(),
                },
            ],
            err: None,
        };
        assert_eq!(
            check_legacy_qovery_release(&v2, InstallPath::Operator).status,
            Status::Pass
        );
    }

    #[test]
    fn cert_manager() {
        // foreign Helm release fails with its owner as evidence
        let mut f = clean_eks();
        f.crds.items.insert(
            CRD_CERT_MANAGER.into(),
            meta(
                "",
                CRD_CERT_MANAGER,
                &[],
                &[
                    (HELM_RELEASE_NAME_ANNOTATION, "cert-manager"),
                    (HELM_RELEASE_NAMESPACE_ANNOTATION, "cert-manager"),
                ],
            ),
        );
        f.deployments.items = vec![meta(
            "cert-manager",
            "cert-manager",
            &[("app.kubernetes.io/name", "cert-manager")],
            &[],
        )];
        let r = check_cert_manager(&f);
        assert_eq!(r.status, Status::Fail);
        assert!(r.detail.contains("Helm release cert-manager/cert-manager"));
        assert!(r.detail.contains("Deployment cert-manager/cert-manager"));
        assert!(failed(&evaluate(&f, Options::default())));

        // CRDs applied with kubectl are foreign on a fresh cluster
        let mut f = clean_eks();
        f.crds.items.insert(
            CRD_CERT_MANAGER.into(),
            meta("", CRD_CERT_MANAGER, &[], &[]),
        );
        let r = check_cert_manager(&f);
        assert_eq!(r.status, Status::Fail);
        assert!(r.detail.contains("not managed by Helm"));

        // Qovery's own cert-manager passes on a second run
        let mut f = clean_eks();
        f.helm_releases = HelmReleaseList {
            items: vec![HelmRelease {
                name: "cert-manager".into(),
                status: "deployed".into(),
            }],
            err: None,
        };
        f.crds.items.insert(
            CRD_CERT_MANAGER.into(),
            meta("", CRD_CERT_MANAGER, &[], &[]),
        );
        f.deployments.items = vec![meta(
            "qovery",
            "cert-manager",
            &[("app.kubernetes.io/name", "cert-manager")],
            &[
                (HELM_RELEASE_NAME_ANNOTATION, "cert-manager"),
                (HELM_RELEASE_NAMESPACE_ANNOTATION, "qovery"),
            ],
        )];
        assert_eq!(check_cert_manager(&f).status, Status::Pass);

        // unreadable cluster is skipped, never passed
        let mut f = clean_eks();
        f.crds = CrdList {
            items: Default::default(),
            err: Some("forbidden".into()),
        };
        f.deployments = DeploymentList {
            items: vec![],
            err: Some("forbidden".into()),
        };
        assert_eq!(check_cert_manager(&f).status, Status::Skip);
    }

    #[test]
    fn external_dns_and_envoy_gateway() {
        let mut f = clean_eks();
        f.deployments.items = vec![
            meta(
                "kube-system",
                "external-dns",
                &[("app.kubernetes.io/name", "external-dns")],
                &[],
            ),
            meta(
                "envoy-gateway-system",
                "envoy-gateway",
                &[("app.kubernetes.io/name", "gateway-helm")],
                &[],
            ),
            meta(
                "qovery",
                "external-dns",
                &[],
                &[
                    (HELM_RELEASE_NAME_ANNOTATION, "external-dns"),
                    (HELM_RELEASE_NAMESPACE_ANNOTATION, "qovery"),
                ],
            ),
        ];
        let dns = check_external_dns(&f.deployments);
        assert_eq!(dns.status, Status::Fail);
        assert!(dns.detail.contains("kube-system/external-dns"));
        assert!(!dns.detail.contains("qovery/external-dns"));

        let envoy = check_envoy_gateway(&f);
        assert_eq!(envoy.status, Status::Fail);
        assert!(envoy.detail.contains("envoy-gateway-system/envoy-gateway"));
    }

    #[test]
    fn gateway_api_crds() {
        let mut f = clean_eks();
        f.crds.items.insert(
            CRD_GATEWAY_API.into(),
            meta(
                "",
                CRD_GATEWAY_API,
                &[],
                &[
                    ("gateway.networking.k8s.io/channel", "standard"),
                    ("gateway.networking.k8s.io/bundle-version", "v1.2.1"),
                ],
            ),
        );
        let r = check_gateway_api_crds(&f);
        assert_eq!(r.status, Status::Warn);
        assert!(r.detail.contains("channel \"standard\""));
        assert!(r.detail.contains("older than v1.5"));

        let mut f = clean_eks();
        f.crds.items.insert(
            CRD_GATEWAY_API.into(),
            meta(
                "",
                CRD_GATEWAY_API,
                &[],
                &[
                    (HELM_RELEASE_NAME_ANNOTATION, "envoy-gateway-crd"),
                    (HELM_RELEASE_NAMESPACE_ANNOTATION, "qovery"),
                ],
            ),
        );
        assert_eq!(check_gateway_api_crds(&f).status, Status::Pass);

        assert!(gateway_bundle_before_15("v1.4"));
        assert!(gateway_bundle_before_15("v1.0.1"));
        assert!(!gateway_bundle_before_15("v1.5.0"));
        assert!(!gateway_bundle_before_15("v1.40"));
    }

    #[test]
    fn storage() {
        let none = StorageClassList {
            items: vec![StorageClass {
                name: "gp2".into(),
                ..Default::default()
            }],
            err: None,
        };
        let r = check_default_storage_class(&none, Options::default());
        assert_eq!(r.status, Status::Fail);
        assert!(r.detail.contains("gp2"));
        assert!(r.fix.contains("--loki-object-storage"));
        assert_eq!(
            check_default_storage_class(
                &none,
                Options {
                    loki_object_storage: true,
                    ..Default::default()
                }
            )
            .status,
            Status::Skip
        );

        let two = StorageClassList {
            items: vec![
                StorageClass {
                    name: "a".into(),
                    default: true,
                    ..Default::default()
                },
                StorageClass {
                    name: "b".into(),
                    default: true,
                    ..Default::default()
                },
            ],
            err: None,
        };
        assert_eq!(
            check_default_storage_class(&two, Options::default()).status,
            Status::Warn
        );

        let no_ebs = NameList {
            items: vec!["efs.csi.aws.com".into()],
            err: None,
        };
        assert_eq!(
            check_ebs_csi_driver(&no_ebs, Provider::Aws, Options::default()).status,
            Status::Fail
        );
        assert_eq!(
            check_ebs_csi_driver(
                &no_ebs,
                Provider::Aws,
                Options {
                    loki_object_storage: true,
                    ..Default::default()
                }
            )
            .status,
            Status::Warn
        );
        assert_eq!(
            check_ebs_csi_driver(&no_ebs, Provider::Gcp, Options::default()).status,
            Status::Skip
        );
    }

    #[test]
    fn network_checks() {
        let broken = check_cluster_dns(&ClusterDnsFacts {
            service_found: true,
            ready_endpoints: 0,
            err: None,
        });
        assert_eq!(broken.status, Status::Fail);
        assert!(broken.fix.contains("nslookup engine.qovery.com"));
        assert_eq!(
            check_cluster_dns(&ClusterDnsFacts::default()).status,
            Status::Warn
        );

        let pending = check_load_balancer(
            &LoadBalancerList {
                items: vec![LoadBalancer {
                    namespace: "default".into(),
                    name: "web".into(),
                    has_address: false,
                }],
                err: None,
            },
            Provider::Aws,
        );
        assert_eq!(pending.status, Status::Warn);
        assert!(pending.fix.contains("kubernetes.io/role/elb=1"));
        assert_eq!(
            check_load_balancer(&LoadBalancerList::default(), Provider::Gcp).status,
            Status::Skip
        );
        assert_eq!(
            check_metrics_server(&MetricsApiFacts::default()).status,
            Status::Warn
        );
    }

    #[test]
    fn evidence_is_bounded() {
        let items: Vec<String> = (0..10).map(|_| "x".to_string()).collect();
        let joined = join_evidence(&items);
        assert!(joined.ends_with("and 6 more"), "{joined}");
    }

    #[test]
    fn detect_provider_tie_break() {
        // equal counts: the greatest provider name wins ("Scaleway" > "AWS")
        let nodes = NodeList {
            items: vec![node("a", "aws:///x"), node("s", "scaleway://y")],
            err: None,
        };
        assert_eq!(detect_provider(&nodes), Provider::Scaleway);
        let nodes = NodeList {
            items: vec![node("a", "aws:///x"), node("u", "")],
            err: None,
        };
        assert_eq!(detect_provider(&nodes), Provider::Aws);
    }
}
