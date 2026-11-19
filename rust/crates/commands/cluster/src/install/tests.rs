//! Port of pkg/cluster/selfmanaged/*_test.go and cmd/cluster_install_test.go.

use super::*;
use crate::prompt::PromptUiFactoryMock;
use crate::testutil::{MockServer, Route, cluster_json};
use serde_json::json;
use std::cell::RefCell;

struct OrganizationServiceMock;

impl OrganizationService for OrganizationServiceMock {
    fn ask_user_to_select_organization(&self) -> Result<Option<OrganizationDto>, String> {
        Ok(Some(OrganizationDto {
            id: "org-test".into(),
            name: "TestOrganization".into(),
        }))
    }
}

struct SelfManagedClusterServiceMock {
    create_error: bool,
    configure_error: bool,
    installation_values: String,
    base_values: String,
}

impl SelfManagedClusterService for SelfManagedClusterServiceMock {
    fn create(&self, organization_id: &str, cloud_vendor: &str) -> Result<Value, String> {
        if self.create_error {
            return Err("should not create self managed cluster".into());
        }
        Ok(cluster_json(
            "c-new",
            "TestCluster",
            organization_id,
            cloud_vendor,
            "SELF_MANAGED",
        ))
    }
    fn configure(&self, _cluster: &Value) -> Result<(), String> {
        if self.configure_error {
            return Err("should not configure an existing self managed cluster".into());
        }
        Ok(())
    }
    fn get_installation_helm_values(
        &self,
        _organization_id: &str,
        _cluster_id: &str,
    ) -> Result<String, String> {
        Ok(self.installation_values.clone())
    }
    fn get_base_helm_values_content(&self, _kubernetes_type: &str) -> Result<String, String> {
        Ok(self.base_values.clone())
    }
}

struct ClusterServiceMock {
    clusters: Vec<Value>,
}

impl ClusterService for ClusterServiceMock {
    fn list_clusters(&self, _organization_id: &str) -> Result<Vec<Value>, String> {
        Ok(self.clusters.clone())
    }
    fn list_cluster_regions(&self, _cloud_provider_type: &str) -> Result<Vec<Value>, String> {
        Ok(vec![])
    }
    fn ask_to_edit_storage_class(&self, _cluster: &Value) -> Result<(), String> {
        Ok(())
    }
}

#[derive(Default)]
struct FileWriterServiceMock {
    content: RefCell<String>,
}

impl FileWriterService for FileWriterServiceMock {
    fn write_file(&self, _name: &str, data: &[u8], _perm: u32) -> Result<(), String> {
        *self.content.borrow_mut() = String::from_utf8_lossy(data).to_string();
        Ok(())
    }
}

const TYPE: &str = "Select where you want to install Qovery on";
const EMAIL: &str =
    "Enter your email address to receive expiration notification from Let's Encrypt";

fn run_install(
    clusters: Vec<Value>,
    self_managed: &SelfManagedClusterServiceMock,
    forced: &[&str],
    answers: &[(&str, &str)],
) -> (Result<Option<String>, String>, String) {
    let organization_service = OrganizationServiceMock;
    let cluster_service = ClusterServiceMock { clusters };
    let file_writer = FileWriterServiceMock::default();
    let prompt = PromptUiFactoryMock::new(forced, answers);
    let service = InstallSelfManagedClusterService {
        organization_service: &organization_service,
        self_managed_service: self_managed,
        cluster_service: &cluster_service,
        file_writer: &file_writer,
        prompt: &prompt,
    };
    let res = service.install_cluster();
    let content = file_writer.content.borrow().clone();
    (res, content)
}

fn self_managed(installation: &str, base: &str) -> SelfManagedClusterServiceMock {
    SelfManagedClusterServiceMock {
        create_error: false,
        configure_error: false,
        installation_values: installation.to_string(),
        base_values: base.to_string(),
    }
}

#[test]
fn install_new_cluster() {
    let sm = self_managed(
        "<helm_values_content_fetched_from_core>",
        "<helm_base_helm_values_content_fetched_from_core",
    );
    let (res, _) = run_install(vec![], &sm, &[], &[(TYPE, "Your Local Machine")]);
    assert_eq!(
        res,
        Ok(Some(
            "Please use `qovery demo up` to create a demo cluster on your local machine"
                .to_string()
        ))
    );

    let (res, content) = run_install(
        vec![],
        &sm,
        &["Reuse or Create a new cluster?"],
        &[(TYPE, "Your AWS EKS cluster"), (EMAIL, "email@test.com")],
    );
    assert_eq!(res, Ok(None));
    assert_eq!(
        content,
        "<helm_values_content_fetched_from_core>\n<helm_base_helm_values_content_fetched_from_core"
    );
}

#[test]
fn install_azure_cluster() {
    let bases = [
        "\ningress-nginx:\n    controller:\n        useComponentLabel: true\n    fullnameOverride: ingress-nginx\n",
        "\ningress-nginx:\n    controller:\n        service:\n            externalTrafficPolicy: Local\n",
        "\ningress-nginx:\n    controller:\n        service:\n            annotations:\n            externalTrafficPolicy: Local\n",
    ];
    for base in bases {
        let sm = self_managed("", base);
        let (res, content) = run_install(
            vec![],
            &sm,
            &[],
            &[(TYPE, "Your Azure EKS cluster"), (EMAIL, "email@test.com")],
        );
        assert_eq!(res, Ok(None));
        assert!(
            content.contains("service.beta.kubernetes.io/azure-load-balancer-internal: \"false\""),
            "{content}"
        );
        assert!(
            content.contains("externalTrafficPolicy: Local"),
            "{content}"
        );
    }
    // exact yaml.v3 output of the first case
    let sm = self_managed("", bases[0]);
    let (_, content) = run_install(
        vec![],
        &sm,
        &[],
        &[(TYPE, "Your Azure EKS cluster"), (EMAIL, "email@test.com")],
    );
    assert_eq!(
        content,
        "ingress-nginx:\n    controller:\n        service:\n            annotations:\n                service.beta.kubernetes.io/azure-load-balancer-internal: \"false\"\n            externalTrafficPolicy: Local\n        useComponentLabel: true\n    fullnameOverride: ingress-nginx\n"
    );
}

#[test]
fn inject_qovery_cluster_gateway_domain_cases() {
    let input = "\nqovery:\n  domain: zf37fd40f.xmx.sh\nqovery-cluster-gateway:\n  dns: {}\n";
    let result = inject_qovery_cluster_gateway_domain(input).unwrap();
    assert!(result.contains("qovery-cluster-gateway:"));
    assert!(result.contains("domain: zf37fd40f.xmx.sh"));
    assert_eq!(
        result,
        "qovery:\n    domain: zf37fd40f.xmx.sh\nqovery-cluster-gateway:\n    dns:\n        domain: zf37fd40f.xmx.sh\n"
    );

    let input = "\nqovery:\n  domain: zf37fd40f.xmx.sh\nqovery-cluster-gateway:\n  dns:\n    domain: custom.example.com\n";
    let result = inject_qovery_cluster_gateway_domain(input).unwrap();
    let values = goyaml::unmarshal(&result).unwrap();
    assert_eq!(
        values
            .get("qovery-cluster-gateway")
            .and_then(|g| g.get("dns"))
            .and_then(|d| d.get("domain"))
            .and_then(|d| d.as_str()),
        Some("custom.example.com")
    );
}

#[test]
fn inject_external_dns_gateway_sources_cases() {
    let result =
        inject_external_dns_gateway_sources("\nexternal-dns:\n  provider:\n    name: pdns\n")
            .unwrap();
    let values = goyaml::unmarshal(&result).unwrap();
    let dns = values.get("external-dns").unwrap();
    assert_eq!(
        dns.get("enableGatewayListenerSets"),
        Some(&GoYaml::Bool(true))
    );
    assert_eq!(
        dns.get("sources"),
        Some(&GoYaml::Seq(
            [
                "service",
                "ingress",
                "gateway-httproute",
                "gateway-grpcroute"
            ]
            .iter()
            .map(|s| GoYaml::Str(s.to_string()))
            .collect()
        ))
    );

    let result = inject_external_dns_gateway_sources(
        "\nexternal-dns:\n  enableGatewayListenerSets: false\n  sources:\n    - service\n    - gateway-httproute\n",
    )
    .unwrap();
    let values = goyaml::unmarshal(&result).unwrap();
    let dns = values.get("external-dns").unwrap();
    assert_eq!(
        dns.get("enableGatewayListenerSets"),
        Some(&GoYaml::Bool(false))
    );
    assert_eq!(
        dns.get("sources"),
        Some(&GoYaml::Seq(vec![
            GoYaml::Str("service".into()),
            GoYaml::Str("gateway-httproute".into())
        ]))
    );
}

#[test]
fn inject_envoy_ingress_services_cases() {
    let result = inject_envoy_ingress_services(
        "\nservices:\n  ingress:\n    ingress-nginx:\n      enabled: true\n",
    )
    .unwrap();
    let values = goyaml::unmarshal(&result).unwrap();
    let ingress = values
        .get("services")
        .and_then(|s| s.get("ingress"))
        .unwrap();
    let enabled = |n: &str| ingress.get(n).and_then(|x| x.get("enabled")).cloned();
    assert_eq!(enabled("ingress-nginx"), Some(GoYaml::Bool(false)));
    assert_eq!(enabled("envoy-gateway-crd"), Some(GoYaml::Bool(false)));
    assert_eq!(enabled("envoy-gateway"), Some(GoYaml::Bool(true)));
    assert_eq!(enabled("qovery-gateway-class"), Some(GoYaml::Bool(true)));
    assert_eq!(enabled("qovery-cluster-gateway"), Some(GoYaml::Bool(true)));
}

#[test]
fn reuse_existing_cluster() {
    let sm = SelfManagedClusterServiceMock {
        create_error: true,
        configure_error: true,
        installation_values: "<helm_values_content_fetched_from_core>".into(),
        base_values: "<helm_base_helm_values_content_fetched_from_core".into(),
    };
    let existing = cluster_json("c-1", "MySelfManaged", "org-test", "AWS", "SELF_MANAGED");
    let (res, _) = run_install(
        vec![existing],
        &sm,
        &[],
        &[
            (TYPE, "Your AWS EKS cluster"),
            ("Reuse or Create a new cluster?", "Reuse a Cluster"),
            ("Select the cluster you want to reuse:", "MySelfManaged"),
            (EMAIL, "email@test.com"),
        ],
    );
    assert_eq!(res, Ok(None));
}

#[test]
fn install_cluster_injects_values() {
    let installation = "qovery:\n  domain: zf37fd40f.xmx.sh\n  acmeEmailAddr: acme@qovery.com\nexternal-dns:\n  provider:\n    name: pdns\nservices:\n  ingress:\n    ingress-nginx:\n      enabled: true\n";
    let sm = self_managed(installation, "");
    let (res, content) = run_install(
        vec![],
        &sm,
        &[],
        &[(TYPE, "Your AWS EKS cluster"), (EMAIL, "email@test.com")],
    );
    assert_eq!(res, Ok(None));
    assert_eq!(
        content,
        "external-dns:
    enableGatewayListenerSets: true
    provider:
        name: pdns
    sources:
        - service
        - ingress
        - gateway-httproute
        - gateway-grpcroute
qovery:
    acmeEmailAddr: email@test.com
    domain: zf37fd40f.xmx.sh
qovery-cluster-gateway:
    dns:
        domain: zf37fd40f.xmx.sh
services:
    ingress:
        envoy-gateway:
            enabled: true
        envoy-gateway-crd:
            enabled: false
        ingress-nginx:
            enabled: false
        qovery-cluster-gateway:
            enabled: true
        qovery-gateway-class:
            enabled: true
"
    );
}

#[test]
fn strip_qovery_section_like_go() {
    let values = "services:\n  qovery:\n    qovery-engine:\n      enabled: true\nqovery:\n  clusterId: &clusterId set-by-customer\n  jwtToken: &jwtToken set-by-customer\nqovery-cluster-agent:\n  environmentVariables:\n    CLUSTER_ID: *clusterId\n";
    assert_eq!(
        strip_qovery_section(values),
        "services:\n  qovery:\n    qovery-engine:\n      enabled: true\nqovery-cluster-agent:\n  environmentVariables:\n    CLUSTER_ID: *clusterId\n"
    );
}

#[test]
fn output_commands_to_install_qovery_on_cluster() {
    let output = output_commands_text("/tmp/values-test.yaml");
    for expected in [
        "helm pull qovery/qovery --untar --untardir /tmp/qovery-helm-chart",
        "helm template qovery-gateway-crds /tmp/qovery-helm-chart/qovery/charts/gateway-crds-helm",
        "kubectl apply --server-side -f -",
        "kubectl wait --for=condition=Established --timeout=180s crd/gateways.gateway.networking.k8s.io",
        "kubectl wait --for=condition=Established --timeout=180s crd/envoyproxies.gateway.envoyproxy.io",
        "--set services.ingress.envoy-gateway-crd.enabled=false",
        "--set qovery-cluster-gateway.metrics.enabled=false",
        "--set qovery-cluster-gateway.metrics.podMonitor.enabled=false",
        "--set services.ingress.envoy-gateway.enabled=false",
        "--set services.ingress.qovery-gateway-class.enabled=false",
        "--set services.ingress.qovery-cluster-gateway.enabled=false",
    ] {
        assert!(output.contains(expected), "{expected}");
    }
}

#[test]
fn validate_base_values_file() {
    let dir = tempfile::tempdir().unwrap();
    let file = dir.path().join("values-scaleway.yaml");
    std::fs::write(&file, "services: {}\n").unwrap();
    assert_eq!(
        validate_cluster_install_base_values_file(file.to_str().unwrap()),
        Ok(file.to_string_lossy().to_string())
    );
    assert!(validate_cluster_install_base_values_file(dir.path().to_str().unwrap()).is_err());
    assert_eq!(
        validate_cluster_install_base_values_file(""),
        Ok(String::new())
    );
}

#[test]
fn self_managed_service_create_and_values() {
    let region =
        json!({"name": "eu-west-3", "country_code": "FR", "country": "France", "city": "Paris"});
    let server = MockServer::start(vec![
        Route::get("/aws/region", json!({"results": [region]})),
        Route::get(
            "/organization/org/aws/credentials",
            json!({"results": [{"id": "cred-1", "name": "creds"}]}),
        ),
        Route::get(
            "/organization/org/onPremise/credentials",
            json!({"results": [{"id": "cred-op", "name": "on-premise"}]}),
        ),
        Route::new(
            "POST",
            "/organization/org/cluster",
            201,
            cluster_json("c-1", "my-cluster", "org", "AWS", "SELF_MANAGED"),
        ),
        Route::raw(
            "GET",
            "/organization/org/cluster/c-1/installationHelmValues",
            200,
            "qovery:\n  clusterId: c-1\n",
        ),
    ]);
    let client = server.client();
    let prompt = PromptUiFactoryMock::new(
        &[],
        &[
            ("Give a name to your new cluster", "my-cluster"),
            (
                "Which credentials do you want to use for the container registry ? A container registry is necessary to build and mirror the images deployed on your cluster.",
                "creds",
            ),
        ],
    );
    let cluster_service = ClusterServiceImpl::new(&client, &prompt);
    let credentials_service = ClusterCredentialsServiceImpl {
        client: &client,
        prompt: &prompt,
    };
    let registry_service = ClusterContainerRegistryServiceImpl {
        client: &client,
        prompt: &prompt,
    };
    let dir = tempfile::tempdir().unwrap();
    let base = dir.path().join("values.yaml");
    std::fs::write(&base, "services: {}\n").unwrap();
    let service = SelfManagedClusterServiceImpl {
        client: &client,
        cluster_service: &cluster_service,
        credentials_service: &credentials_service,
        container_registry_service: &registry_service,
        prompt: &prompt,
        local_base_helm_values_path: base.to_string_lossy().to_string(),
    };
    let cluster = service.create("org", "AWS").unwrap();
    assert_eq!(str_field(&cluster, "id"), "c-1");
    let post = server
        .requests()
        .into_iter()
        .find(|r| r.method == "POST")
        .unwrap();
    assert_eq!(
        post.body,
        json!({
            "name": "my-cluster", "region": "eu-west-3", "cloud_provider": "AWS", "kubernetes": "SELF_MANAGED",
            "cloud_provider_credentials": {"cloud_provider": "AWS", "credentials": {"id": "cred-1", "name": "creds"}, "region": "eu-west-3"},
            "features": []
        })
    );
    // On premise: no region nor credentials prompt
    service.create("org", "ON_PREMISE").unwrap();
    let post = server
        .requests()
        .into_iter()
        .filter(|r| r.method == "POST")
        .nth(1)
        .unwrap();
    assert_eq!(post.body["region"], "on-premise");
    assert_eq!(
        post.body["cloud_provider_credentials"]["credentials"]["id"],
        "cred-op"
    );

    assert_eq!(
        service.get_installation_helm_values("org", "c-1").unwrap(),
        "qovery:\n  clusterId: c-1\n"
    );
    assert_eq!(
        service.get_base_helm_values_content("AWS").unwrap(),
        "services: {}\n"
    );
    // configure is a no-op for non on-premise clusters
    assert_eq!(service.configure(&cluster), Ok(()));
}

#[test]
fn expand_path_like_go() {
    let home = |h: &'static str| move || Ok::<String, String>(h.to_string());
    let no_home = || Err::<String, String>("$HOME is not defined".to_string());
    // Unix (separator '/')
    assert_eq!(super::expand_path_with("", '/', no_home).unwrap(), "");
    assert_eq!(
        super::expand_path_with("~", '/', home("/home/u/")).unwrap(),
        "/home/u/"
    );
    assert_eq!(
        super::expand_path_with("~/a/../b", '/', home("/home/u")).unwrap(),
        "/home/u/b"
    );
    // strings.TrimPrefix(strings.TrimPrefix(path, "~/"), "~/")
    assert_eq!(
        super::expand_path_with("~/~/x", '/', home("/home/u")).unwrap(),
        "/home/u/x"
    );
    assert_eq!(
        super::expand_path_with("~/", '/', home("/home/u/")).unwrap(),
        "/home/u"
    );
    assert_eq!(super::expand_path_with("~x", '/', no_home).unwrap(), "~x");
    assert_eq!(
        super::expand_path_with("~\\x", '/', no_home).unwrap(),
        "~\\x"
    );
    assert_eq!(
        super::expand_path_with("~/x", '/', no_home).unwrap_err(),
        "$HOME is not defined"
    );
    // Windows (separator '\')
    assert_eq!(
        super::expand_path_with("~\\vals\\v.yaml", '\\', home("C:\\Users\\u")).unwrap(),
        "C:\\Users\\u\\vals\\v.yaml"
    );
    assert_eq!(
        super::expand_path_with("~/vals/../v.yaml", '\\', home("C:\\Users\\u")).unwrap(),
        "C:\\Users\\u\\v.yaml"
    );
    assert_eq!(
        super::expand_path_with("~\\x", '\\', || Err("%userprofile% is not defined".into()))
            .unwrap_err(),
        "%userprofile% is not defined"
    );
}

#[test]
fn go_clean_windows() {
    for (input, go) in [
        ("C:\\a\\..\\b/c", "C:\\b\\c"),
        ("C:", "C:."),
        ("\\\\host\\share\\x\\..", "\\\\host\\share\\"),
        ("a/./b/", "a\\b"),
        ("..\\..\\a", "..\\..\\a"),
        ("", "."),
    ] {
        assert_eq!(super::go_clean_str(input, true), go, "{input}");
    }
}
