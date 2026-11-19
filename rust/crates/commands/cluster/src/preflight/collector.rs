//! Port of pkg/cluster/preflight/collector.go: read the facts the checks need.
//!
//! It only issues get, list and SelfSubjectAccessReview requests: it never creates, changes or deletes a
//! cluster object, and it reads Secrets and Deployments through the metadata API (no Secret data is fetched).
//! The requests (paths, `timeout` parameters, order) are the ones client-go sends.

use super::checks::{CRD_CERT_MANAGER, CRD_ENVOY_PROXY, CRD_GATEWAY_API, QOVERY_NAMESPACE};
use super::facts::*;
use serde_json::{Value, json};
use std::collections::BTreeMap;

/// `Accept` of the discovery requests.
pub const ACCEPT_DISCOVERY: &str = "application/json, */*";
/// `Accept` of the typed clientset requests (JSON only).
pub const ACCEPT_JSON: &str = "application/json";
pub const ACCEPT_METADATA: &str =
    "application/json;as=PartialObjectMetadata;g=meta.k8s.io;v=v1,application/json";
pub const ACCEPT_METADATA_LIST: &str =
    "application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1,application/json";

/// One request to the Kubernetes API.
#[derive(Debug, Clone)]
pub struct KubeRequest {
    pub method: &'static str,
    pub path: String,
    /// Query parameters (encoded sorted, like `url.Values`)
    pub query: Vec<(&'static str, String)>,
    pub body: Option<Value>,
    pub accept: &'static str,
    /// client-go `Resource()` (qualified group resource) and `Name()`, used in generic error messages
    pub resource: Option<(&'static str, String)>,
}

impl KubeRequest {
    fn get(path: impl Into<String>, accept: &'static str) -> Self {
        KubeRequest {
            method: "GET",
            path: path.into(),
            query: vec![],
            body: None,
            accept,
            resource: None,
        }
    }
    fn timeout(mut self) -> Self {
        self.query.push(("timeout", "20s".to_string()));
        self
    }
    fn label_selector(mut self, s: &str) -> Self {
        self.query.push(("labelSelector", s.to_string()));
        self
    }
    fn resource(mut self, r: &'static str, name: &str) -> Self {
        self.resource = Some((r, name.to_string()));
        self
    }
}

/// A Kubernetes API error, displayed like client-go's.
#[derive(Debug, Clone, PartialEq)]
pub struct KubeError {
    pub message: String,
    pub not_found: bool,
}

impl KubeError {
    pub fn new(message: impl Into<String>) -> Self {
        KubeError {
            message: message.into(),
            not_found: false,
        }
    }
}

/// Access to the Kubernetes API (the real client, or a fake in tests).
pub trait KubeApi {
    fn request(&self, req: &KubeRequest) -> Result<Value, KubeError>;
}

/// `leadingInt`: the leading decimal digits of `s` (`"31+"` -> 31).
pub fn leading_int(s: &str) -> i64 {
    let digits: String = s.chars().take_while(|c| c.is_ascii_digit()).collect();
    digits.parse().unwrap_or(0)
}

fn s<'a>(v: &'a Value, path: &[&str]) -> &'a str {
    let mut cur = v;
    for p in path {
        match cur.get(*p) {
            Some(x) => cur = x,
            None => return "",
        }
    }
    cur.as_str().unwrap_or_default()
}

fn string_map(v: Option<&Value>) -> Labels {
    v.and_then(|x| x.as_object())
        .map(|m| {
            m.iter()
                .map(|(k, v)| (k.clone(), v.as_str().unwrap_or_default().to_string()))
                .collect()
        })
        .unwrap_or_default()
}

fn items(v: &Value) -> Vec<Value> {
    v.get("items")
        .and_then(|i| i.as_array())
        .cloned()
        .unwrap_or_default()
}

fn object_meta(v: &Value) -> ObjectMeta {
    let m = v.get("metadata").cloned().unwrap_or(Value::Null);
    ObjectMeta {
        namespace: s(&m, &["namespace"]).to_string(),
        name: s(&m, &["name"]).to_string(),
        labels: string_map(m.get("labels")),
        annotations: string_map(m.get("annotations")),
    }
}

/// `collectWith`.
pub fn collect_with(api: &dyn KubeApi) -> Facts {
    let mut f = Facts {
        version: collect_version(api),
        ..Default::default()
    };
    if f.version.err.is_some() {
        // Without the API server, every other read fails the same way.
        return f;
    }
    f.access = collect_access(api);
    f.nodes = collect_nodes(api);
    f.qovery_ns = collect_qovery_namespace(api);
    f.helm_releases = collect_helm_releases(api, &f.qovery_ns);
    f.crds = collect_crds(api, &[CRD_CERT_MANAGER, CRD_ENVOY_PROXY, CRD_GATEWAY_API]);
    f.deployments = collect_deployments(api);
    f.storage_classes = collect_storage_classes(api);
    f.csi_drivers = collect_csi_drivers(api);
    f.cluster_dns = collect_cluster_dns(api);
    f.load_balancers = collect_load_balancers(api);
    f.metrics_api = collect_metrics_api(api);
    f
}

fn collect_version(api: &dyn KubeApi) -> VersionFacts {
    match api.request(&KubeRequest::get("/version", ACCEPT_DISCOVERY).timeout()) {
        Err(e) => VersionFacts {
            err: Some(e.message),
            ..Default::default()
        },
        // Managed providers report minors such as "31+".
        Ok(info) => VersionFacts {
            raw: s(&info, &["gitVersion"]).to_string(),
            major: leading_int(s(&info, &["major"])),
            minor: leading_int(s(&info, &["minor"])),
            err: None,
        },
    }
}

/// The SelfSubjectAccessReview client-go sends (`verb=*`, `group=*`, `resource=*`).
pub fn self_subject_access_review() -> Value {
    json!({
        "kind": "SelfSubjectAccessReview",
        "apiVersion": "authorization.k8s.io/v1",
        "metadata": {},
        "spec": {"resourceAttributes": {"verb": "*", "group": "*", "resource": "*"}},
        "status": {"allowed": false}
    })
}

fn collect_access(api: &dyn KubeApi) -> AccessFacts {
    let req = KubeRequest {
        method: "POST",
        path: "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews".to_string(),
        query: vec![("timeout", "20s".to_string())],
        body: Some(self_subject_access_review()),
        accept: ACCEPT_JSON,
        resource: Some((
            "selfsubjectaccessreviews.authorization.k8s.io",
            String::new(),
        )),
    };
    match api.request(&req) {
        Err(e) => AccessFacts {
            cluster_admin: false,
            err: Some(e.message),
        },
        Ok(res) => AccessFacts {
            cluster_admin: res
                .get("status")
                .and_then(|s| s.get("allowed"))
                .and_then(|a| a.as_bool())
                .unwrap_or(false),
            err: None,
        },
    }
}

fn collect_nodes(api: &dyn KubeApi) -> NodeList {
    let list =
        match api.request(&KubeRequest::get("/api/v1/nodes", ACCEPT_JSON).resource("nodes", "")) {
            Ok(l) => l,
            Err(e) => {
                return NodeList {
                    items: vec![],
                    err: Some(e.message),
                };
            }
        };
    let mut out = NodeList::default();
    for n in items(&list) {
        let labels = string_map(n.get("metadata").and_then(|m| m.get("labels")));
        let mut node = Node {
            name: s(&n, &["metadata", "name"]).to_string(),
            unschedulable: n
                .get("spec")
                .and_then(|x| x.get("unschedulable"))
                .and_then(|x| x.as_bool())
                .unwrap_or(false),
            os: s(&n, &["status", "nodeInfo", "operatingSystem"]).to_string(),
            arch: s(&n, &["status", "nodeInfo", "architecture"]).to_string(),
            provider_id: s(&n, &["spec", "providerID"]).to_string(),
            ready: false,
            cpu_milli: 0,
            memory_bytes: 0,
            labels,
        };
        if node.os.is_empty() {
            node.os = node
                .labels
                .get("kubernetes.io/os")
                .cloned()
                .unwrap_or_default();
        }
        if node.arch.is_empty() {
            node.arch = node
                .labels
                .get("kubernetes.io/arch")
                .cloned()
                .unwrap_or_default();
        }
        if let Some(conds) = n
            .get("status")
            .and_then(|x| x.get("conditions"))
            .and_then(|x| x.as_array())
        {
            for c in conds {
                if s(c, &["type"]) == "Ready" && s(c, &["status"]) == "True" {
                    node.ready = true;
                }
            }
        }
        if let Some(q) = n
            .get("status")
            .and_then(|x| x.get("allocatable"))
            .and_then(|x| x.get("cpu"))
        {
            node.cpu_milli = quantity_value(q, 1000);
        }
        if let Some(q) = n
            .get("status")
            .and_then(|x| x.get("allocatable"))
            .and_then(|x| x.get("memory"))
        {
            node.memory_bytes = quantity_value(q, 1);
        }
        out.items.push(node);
    }
    out
}

fn collect_qovery_namespace(api: &dyn KubeApi) -> NamespaceFacts {
    let req = KubeRequest::get(
        format!("/api/v1/namespaces/{QOVERY_NAMESPACE}"),
        ACCEPT_JSON,
    )
    .resource("namespaces", QOVERY_NAMESPACE)
    .timeout();
    match api.request(&req) {
        Err(e) if e.not_found => NamespaceFacts::default(),
        Err(e) => NamespaceFacts {
            err: Some(e.message),
            ..Default::default()
        },
        Ok(ns) => NamespaceFacts {
            exists: true,
            terminating: s(&ns, &["status", "phase"]) == "Terminating"
                || ns
                    .get("metadata")
                    .and_then(|m| m.get("deletionTimestamp"))
                    .is_some_and(|d| !d.is_null()),
            err: None,
        },
    }
}

/// `collectHelmReleases`: the Helm releases of namespace qovery, from the labels Helm puts on its storage Secrets
/// (owner=helm, name, status, version), keeping the latest revision.
fn collect_helm_releases(api: &dyn KubeApi, ns: &NamespaceFacts) -> HelmReleaseList {
    if ns.err.is_none() && !ns.exists {
        return HelmReleaseList::default();
    }
    let req = KubeRequest::get(
        format!("/api/v1/namespaces/{QOVERY_NAMESPACE}/secrets"),
        ACCEPT_METADATA_LIST,
    )
    .label_selector("owner=helm")
    .timeout();
    let list = match api.request(&req) {
        Ok(l) => l,
        Err(e) => {
            return HelmReleaseList {
                items: vec![],
                err: Some(e.message),
            };
        }
    };
    let mut latest: BTreeMap<String, ObjectMeta> = BTreeMap::new();
    for item in items(&list) {
        let m = object_meta(&item);
        let name = m.labels.get("name").cloned().unwrap_or_default();
        if name.is_empty() {
            continue;
        }
        let newer = match latest.get(&name) {
            None => true,
            Some(prev) => {
                leading_int(
                    m.labels
                        .get("version")
                        .map(|s| s.as_str())
                        .unwrap_or_default(),
                ) > leading_int(
                    prev.labels
                        .get("version")
                        .map(|s| s.as_str())
                        .unwrap_or_default(),
                )
            }
        };
        if newer {
            latest.insert(name, m);
        }
    }
    HelmReleaseList {
        items: latest
            .into_iter()
            .map(|(name, m)| HelmRelease {
                name,
                status: m.labels.get("status").cloned().unwrap_or_default(),
            })
            .collect(),
        err: None,
    }
}

fn collect_crds(api: &dyn KubeApi, names: &[&str]) -> CrdList {
    let mut out = CrdList::default();
    for name in names {
        let req = KubeRequest::get(
            format!("/apis/apiextensions.k8s.io/v1/customresourcedefinitions/{name}"),
            ACCEPT_METADATA,
        )
        .timeout();
        match api.request(&req) {
            Err(e) if e.not_found => continue,
            Err(e) => {
                return CrdList {
                    items: Default::default(),
                    err: Some(e.message),
                };
            }
            Ok(crd) => {
                out.items.insert(name.to_string(), object_meta(&crd));
            }
        }
    }
    out
}

fn collect_deployments(api: &dyn KubeApi) -> DeploymentList {
    match api
        .request(&KubeRequest::get("/apis/apps/v1/deployments", ACCEPT_METADATA_LIST).timeout())
    {
        Err(e) => DeploymentList {
            items: vec![],
            err: Some(e.message),
        },
        Ok(list) => DeploymentList {
            items: items(&list).iter().map(object_meta).collect(),
            err: None,
        },
    }
}

fn collect_storage_classes(api: &dyn KubeApi) -> StorageClassList {
    let req = KubeRequest::get("/apis/storage.k8s.io/v1/storageclasses", ACCEPT_JSON)
        .resource("storageclasses.storage.k8s.io", "");
    match api.request(&req) {
        Err(e) => StorageClassList {
            items: vec![],
            err: Some(e.message),
        },
        Ok(list) => StorageClassList {
            items: items(&list)
                .iter()
                .map(|sc| {
                    let m = object_meta(sc);
                    let is_default = m
                        .annotations
                        .get("storageclass.kubernetes.io/is-default-class")
                        .map(|s| s.as_str())
                        == Some("true")
                        || m.annotations
                            .get("storageclass.beta.kubernetes.io/is-default-class")
                            .map(|s| s.as_str())
                            == Some("true");
                    StorageClass {
                        name: m.name,
                        default: is_default,
                        provisioner: s(sc, &["provisioner"]).to_string(),
                    }
                })
                .collect(),
            err: None,
        },
    }
}

fn collect_csi_drivers(api: &dyn KubeApi) -> NameList {
    let req = KubeRequest::get("/apis/storage.k8s.io/v1/csidrivers", ACCEPT_JSON)
        .resource("csidrivers.storage.k8s.io", "");
    match api.request(&req) {
        Err(e) => NameList {
            items: vec![],
            err: Some(e.message),
        },
        Ok(list) => NameList {
            items: items(&list)
                .iter()
                .map(|d| s(d, &["metadata", "name"]).to_string())
                .collect(),
            err: None,
        },
    }
}

fn collect_cluster_dns(api: &dyn KubeApi) -> ClusterDnsFacts {
    let req = KubeRequest::get(
        "/api/v1/namespaces/kube-system/services/kube-dns",
        ACCEPT_JSON,
    )
    .resource("services", "kube-dns")
    .timeout();
    match api.request(&req) {
        Err(e) if e.not_found => return ClusterDnsFacts::default(),
        Err(e) => {
            return ClusterDnsFacts {
                err: Some(e.message),
                ..Default::default()
            };
        }
        Ok(_) => {}
    }
    let req = KubeRequest::get(
        "/apis/discovery.k8s.io/v1/namespaces/kube-system/endpointslices",
        ACCEPT_JSON,
    )
    .label_selector("kubernetes.io/service-name=kube-dns")
    .resource("endpointslices.discovery.k8s.io", "");
    let slices = match api.request(&req) {
        Ok(l) => l,
        Err(e) => {
            return ClusterDnsFacts {
                service_found: true,
                ready_endpoints: 0,
                err: Some(e.message),
            };
        }
    };
    let mut ready = 0;
    for slice in items(&slices) {
        for e in slice
            .get("endpoints")
            .and_then(|x| x.as_array())
            .cloned()
            .unwrap_or_default()
        {
            // A nil Ready condition means ready (discovery.k8s.io/v1 API contract).
            match e.get("conditions").and_then(|c| c.get("ready")) {
                None | Some(Value::Null) | Some(Value::Bool(true)) => ready += 1,
                _ => {}
            }
        }
    }
    ClusterDnsFacts {
        service_found: true,
        ready_endpoints: ready,
        err: None,
    }
}

fn collect_load_balancers(api: &dyn KubeApi) -> LoadBalancerList {
    let list = match api
        .request(&KubeRequest::get("/api/v1/services", ACCEPT_JSON).resource("services", ""))
    {
        Ok(l) => l,
        Err(e) => {
            return LoadBalancerList {
                items: vec![],
                err: Some(e.message),
            };
        }
    };
    let mut out = LoadBalancerList::default();
    for svc in items(&list) {
        if s(&svc, &["spec", "type"]) != "LoadBalancer" {
            continue;
        }
        let ingress = svc
            .get("status")
            .and_then(|x| x.get("loadBalancer"))
            .and_then(|x| x.get("ingress"))
            .and_then(|x| x.as_array())
            .cloned()
            .unwrap_or_default();
        let has_address = ingress
            .iter()
            .any(|i| !s(i, &["ip"]).is_empty() || !s(i, &["hostname"]).is_empty());
        let m = object_meta(&svc);
        out.items.push(LoadBalancer {
            namespace: m.namespace,
            name: m.name,
            has_address,
        });
    }
    out
}

fn collect_metrics_api(api: &dyn KubeApi) -> MetricsApiFacts {
    match api.request(&KubeRequest::get("/apis/metrics.k8s.io/v1beta1", ACCEPT_DISCOVERY).timeout())
    {
        Ok(_) => MetricsApiFacts {
            available: true,
            err: None,
        },
        Err(e) if e.not_found || e.message.contains("could not find the requested resource") => {
            MetricsApiFacts::default()
        }
        Err(e) => MetricsApiFacts {
            available: false,
            err: Some(e.message),
        },
    }
}

/// `resource.Quantity` value multiplied by `scale` (1 for `Value()`, 1000 for `MilliValue()`), rounded up.
pub fn quantity_value(q: &Value, scale: i128) -> i64 {
    match q {
        Value::Number(n) => {
            if let Some(i) = n.as_i64() {
                return (i as i128 * scale) as i64;
            }
            (n.as_f64().unwrap_or(0.0) * scale as f64).ceil() as i64
        }
        Value::String(s) => parse_quantity(s, scale).unwrap_or(0),
        _ => 0,
    }
}

fn parse_quantity(s: &str, scale: i128) -> Option<i64> {
    let s = s.trim();
    let (sign, s) = match s.strip_prefix('-') {
        Some(r) => (-1i128, r),
        None => (1i128, s.strip_prefix('+').unwrap_or(s)),
    };
    let num_end = s
        .find(|c: char| !(c.is_ascii_digit() || c == '.'))
        .unwrap_or(s.len());
    let (num, suffix) = s.split_at(num_end);
    let (int_part, frac_part) = num.split_once('.').unwrap_or((num, ""));
    let mantissa: i128 = format!("{int_part}{frac_part}").parse().ok()?;
    let mut num_scale: i32 = -(frac_part.len() as i32);
    let mut mult: i128 = 1;
    match suffix {
        "" => {}
        "Ki" => mult = 1 << 10,
        "Mi" => mult = 1 << 20,
        "Gi" => mult = 1 << 30,
        "Ti" => mult = 1 << 40,
        "Pi" => mult = 1 << 50,
        "Ei" => mult = 1 << 60,
        "n" => num_scale -= 9,
        "u" => num_scale -= 6,
        "m" => num_scale -= 3,
        "k" => num_scale += 3,
        "M" => num_scale += 6,
        "G" => num_scale += 9,
        "T" => num_scale += 12,
        "P" => num_scale += 15,
        "E" => num_scale += 18,
        other => {
            let exp = other.strip_prefix(['e', 'E'])?;
            num_scale += exp.parse::<i32>().ok()?;
        }
    }
    let mut value = mantissa * mult * scale;
    if num_scale >= 0 {
        value *= 10i128.checked_pow(num_scale as u32)?;
    } else {
        let div = 10i128.checked_pow((-num_scale) as u32)?;
        value = (value + div - 1) / div; // round up like resource.Quantity
    }
    Some((sign * value) as i64)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::preflight::checks::{Options, Status, evaluate, failed};
    use std::cell::RefCell;
    use std::collections::HashMap;

    /// Fake API: responses by path, request log.
    struct FakeApi {
        responses: HashMap<String, Result<Value, KubeError>>,
        log: RefCell<Vec<(String, String)>>,
    }

    impl KubeApi for FakeApi {
        fn request(&self, req: &KubeRequest) -> Result<Value, KubeError> {
            self.log
                .borrow_mut()
                .push((req.method.to_string(), req.path.clone()));
            self.responses.get(&req.path).cloned().unwrap_or_else(|| {
                Err(KubeError {
                    message: "the server could not find the requested resource".into(),
                    not_found: true,
                })
            })
        }
    }

    fn partial(namespace: &str, name: &str, labels: Value, annotations: Value) -> Value {
        json!({"metadata": {"namespace": namespace, "name": name, "labels": labels, "annotations": annotations}})
    }

    #[test]
    fn collect_reads_the_cluster() {
        let mut r: HashMap<String, Result<Value, KubeError>> = HashMap::new();
        r.insert(
            "/version".into(),
            Ok(json!({"major": "1", "minor": "33+", "gitVersion": "v1.33.5-gke.100"})),
        );
        r.insert(
            "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews".into(),
            Ok(json!({"status": {"allowed": true}})),
        );
        r.insert(
            "/api/v1/nodes".into(),
            Ok(json!({"items": [{
                "metadata": {"name": "node-1", "labels": {"kubernetes.io/os": "linux"}},
                "spec": {"providerID": "gce://p/europe-west1-b/node-1"},
                "status": {"nodeInfo": {"architecture": "arm64"}, "conditions": [{"type": "Ready", "status": "True"}],
                           "allocatable": {"cpu": "7910m", "memory": "29Gi"}}
            }]})),
        );
        r.insert(
            "/api/v1/namespaces/qovery".into(),
            Ok(json!({"metadata": {"name": "qovery"}, "status": {"phase": "Terminating"}})),
        );
        r.insert(
            "/api/v1/namespaces/qovery/secrets".into(),
            Ok(json!({"items": [
                partial("qovery", "sh.helm.release.v1.qovery.v1", json!({"owner": "helm", "name": "qovery", "status": "superseded", "version": "1"}), json!(null)),
                partial("qovery", "sh.helm.release.v1.qovery.v2", json!({"owner": "helm", "name": "qovery", "status": "deployed", "version": "2"}), json!(null)),
                partial("qovery", "unrelated", json!({"app": "x"}), json!(null)),
            ]})),
        );
        r.insert(
            format!("/apis/apiextensions.k8s.io/v1/customresourcedefinitions/{CRD_GATEWAY_API}"),
            Ok(partial(
                "",
                CRD_GATEWAY_API,
                json!(null),
                json!({"gateway.networking.k8s.io/channel": "standard"}),
            )),
        );
        r.insert(
            "/apis/apps/v1/deployments".into(),
            Ok(json!({"items": [partial("cert-manager", "cert-manager", json!({"app.kubernetes.io/name": "cert-manager"}), json!(null))]})),
        );
        r.insert(
            "/apis/storage.k8s.io/v1/storageclasses".into(),
            Ok(json!({"items": [{"metadata": {"name": "standard-rwo", "annotations": {"storageclass.kubernetes.io/is-default-class": "true"}}, "provisioner": "pd.csi.storage.gke.io"}]})),
        );
        r.insert(
            "/apis/storage.k8s.io/v1/csidrivers".into(),
            Ok(json!({"items": []})),
        );
        r.insert(
            "/api/v1/namespaces/kube-system/services/kube-dns".into(),
            Ok(json!({"metadata": {"name": "kube-dns"}})),
        );
        r.insert(
            "/apis/discovery.k8s.io/v1/namespaces/kube-system/endpointslices".into(),
            Ok(json!({"items": [{"endpoints": [{"conditions": {"ready": true}}, {"conditions": {"ready": false}}, {"conditions": {}}]}]})),
        );
        r.insert(
            "/api/v1/services".into(),
            Ok(json!({"items": [
                {"metadata": {"namespace": "default", "name": "web"}, "spec": {"type": "LoadBalancer"}, "status": {"loadBalancer": {"ingress": [{"ip": "34.1.2.3"}]}}},
                {"metadata": {"namespace": "default", "name": "internal"}, "spec": {"type": "ClusterIP"}}
            ]})),
        );
        let api = FakeApi {
            responses: r,
            log: RefCell::new(vec![]),
        };
        let f = collect_with(&api);

        assert!(f.version.err.is_none());
        assert_eq!((f.version.major, f.version.minor), (1, 33));
        assert!(f.access.cluster_admin);
        assert_eq!(f.nodes.items.len(), 1);
        let node = &f.nodes.items[0];
        assert_eq!(node.os, "linux");
        assert_eq!(node.arch, "arm64");
        assert!(node.ready);
        assert_eq!(node.cpu_milli, 7910);
        assert_eq!(node.memory_bytes, 29 << 30);
        assert!(f.qovery_ns.exists);
        assert!(f.qovery_ns.terminating);
        assert!(f.helm_releases.err.is_none());
        assert_eq!(
            f.helm_releases.items,
            vec![HelmRelease {
                name: "qovery".into(),
                status: "deployed".into()
            }]
        );
        assert!(f.crds.err.is_none());
        assert!(f.crds.items.contains_key(CRD_GATEWAY_API));
        assert!(!f.crds.items.contains_key(CRD_CERT_MANAGER));
        assert_eq!(f.deployments.items.len(), 1);
        assert_eq!(
            f.storage_classes.items,
            vec![StorageClass {
                name: "standard-rwo".into(),
                default: true,
                provisioner: "pd.csi.storage.gke.io".into()
            }]
        );
        assert_eq!(
            f.cluster_dns,
            ClusterDnsFacts {
                service_found: true,
                ready_endpoints: 2,
                err: None
            }
        );
        assert_eq!(
            f.load_balancers.items,
            vec![LoadBalancer {
                namespace: "default".into(),
                name: "web".into(),
                has_address: true
            }]
        );
        assert!(!f.metrics_api.available);
        assert!(f.metrics_api.err.is_none());

        // The collector never writes: the only non-read request is the access review.
        for (method, path) in api.log.borrow().iter() {
            if method == "POST" {
                assert_eq!(
                    path,
                    "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews"
                );
            } else {
                assert_eq!(method, "GET");
            }
        }

        let results = evaluate(&f, Options::default());
        let by_id = |id: &str| results.iter().find(|r| r.id == id).unwrap().status;
        assert!(failed(&results));
        assert_eq!(by_id("NAMESPACE_TERMINATING"), Status::Fail);
        assert_eq!(by_id("LEGACY_QOVERY_RELEASE"), Status::Fail);
        assert_eq!(by_id("INCOMPATIBLE_CERT_MANAGER"), Status::Fail);
        assert_eq!(by_id("CRD_OWNERSHIP_CONFLICT"), Status::Warn);
        assert_eq!(by_id("CLOUD_PROVIDER"), Status::Pass);
        assert_eq!(by_id("EBS_CSI_DRIVER_MISSING"), Status::Skip);
    }

    #[test]
    fn collect_stops_when_the_api_is_unreachable() {
        let mut r: HashMap<String, Result<Value, KubeError>> = HashMap::new();
        r.insert("/version".into(), Err(KubeError::new("connection refused")));
        let api = FakeApi {
            responses: r,
            log: RefCell::new(vec![]),
        };
        let f = collect_with(&api);
        assert!(f.version.err.is_some());
        // Only the version request was sent; nothing else is tried against an unreachable API.
        assert_eq!(api.log.borrow().len(), 1);
        assert_eq!(api.log.borrow()[0].1, "/version");
        assert_eq!(
            crate::preflight::checks::check_kubernetes_version(&f.version).status,
            Status::Fail
        );
    }

    #[test]
    fn quantities() {
        assert_eq!(quantity_value(&json!("7910m"), 1000), 7910);
        assert_eq!(quantity_value(&json!("4"), 1000), 4000);
        assert_eq!(quantity_value(&json!("1.5"), 1000), 1500);
        assert_eq!(quantity_value(&json!("29Gi"), 1), 29 << 30);
        assert_eq!(quantity_value(&json!("128974848"), 1), 128974848);
        assert_eq!(quantity_value(&json!("129e6"), 1), 129000000);
        assert_eq!(quantity_value(&json!("123M"), 1), 123000000);
        assert_eq!(quantity_value(&json!("100m"), 1), 1);
        assert_eq!(leading_int("31+"), 31);
        assert_eq!(leading_int(""), 0);
    }
}
