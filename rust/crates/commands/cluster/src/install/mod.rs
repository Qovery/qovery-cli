//! `cluster install` (cmd/cluster_install.go, pkg/cluster/selfmanaged/install_self_managed_cluster_service.go).

pub mod goyaml;
pub mod services;

use crate::manage::{ClusterService, ClusterServiceImpl, find_cluster_by_name, str_field};
use crate::prompt::{PromptUiFactory, PromptUiFactoryImpl};
use goyaml::GoYaml;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, exit, exit_with_error, telemetry};
use serde_json::Value;
use services::*;
use std::path::Path;

pub fn command() -> Command {
    Command::new("install")
        .short("Install Qovery on your cluster.")
        .flag(Flag::string(
            "base-values-file",
            "Local Helm values base file to use instead of downloading values-demo-<provider>.yaml from qovery-chart",
        ))
        .run(run)
}

/// `expandPath`: `~`, `~/...` and `~<separator>...` (`~\\...` on Windows) relative to `os.UserHomeDir()`.
pub fn expand_path(path: &str) -> Result<String, String> {
    expand_path_with(path, std::path::MAIN_SEPARATOR, || {
        qovery_core::gosys::user_home_dir().map(|h| h.to_string_lossy().into_owned())
    })
}

fn expand_path_with(
    path: &str,
    separator: char,
    home: impl Fn() -> Result<String, String>,
) -> Result<String, String> {
    if path.is_empty() {
        return Ok(String::new());
    }
    if path == "~" {
        return home();
    }
    let tilde_sep = format!("~{separator}");
    if path.starts_with("~/") || path.starts_with(&tilde_sep) {
        let home = home()?;
        // strings.TrimPrefix(strings.TrimPrefix(path, "~/"), "~"+string(filepath.Separator))
        let rest = path.strip_prefix("~/").unwrap_or(path);
        let rest = rest.strip_prefix(&tilde_sep).unwrap_or(rest);
        // filepath.Join(homeDir, rest)
        let windows = separator == '\\';
        let joined = match (home.is_empty(), rest.is_empty()) {
            (true, true) => return Ok(String::new()),
            (true, false) => rest.to_string(),
            (false, true) => home,
            (false, false) => format!("{home}{separator}{rest}"),
        };
        return Ok(go_clean_str(&joined, windows));
    }
    Ok(path.to_string())
}

/// Go `filepath.Clean` (lexical) for the current platform.
fn go_clean(p: &Path) -> String {
    go_clean_str(&p.to_string_lossy(), cfg!(windows))
}

/// Go `filepath.Clean`: Unix, or Windows (`/` and `\\` separators, drive letter or UNC volume kept, `\\` output).
fn go_clean_str(s: &str, windows: bool) -> String {
    let is_sep = |c: char| c == '/' || (windows && c == '\\');
    let separator = if windows { "\\" } else { "/" };
    let vol_len = if windows { windows_volume_len(s) } else { 0 };
    let (volume, path) = s.split_at(vol_len);
    if path.is_empty() {
        if vol_len > 1 && s.starts_with(is_sep) && s[1..].starts_with(is_sep) {
            return volume.replace('/', "\\");
        }
        return format!("{s}.");
    }
    let rooted = path.starts_with(is_sep);
    let mut parts: Vec<&str> = Vec::new();
    for c in path.split(is_sep) {
        match c {
            "" | "." => {}
            ".." => {
                if parts.last().is_some_and(|l| *l != "..") {
                    parts.pop();
                } else if !rooted {
                    parts.push("..");
                }
            }
            c => parts.push(c),
        }
    }
    let joined = parts.join(separator);
    let out = if rooted {
        format!("{separator}{joined}")
    } else if joined.is_empty() {
        ".".to_string()
    } else {
        joined
    };
    let volume = if windows {
        volume.replace('/', "\\")
    } else {
        volume.to_string()
    };
    format!("{volume}{out}")
}

/// Go `filepath.VolumeName` length on Windows: `C:` or a UNC `\\\\host\\share` prefix.
fn windows_volume_len(s: &str) -> usize {
    let b = s.as_bytes();
    let is_sep = |c: u8| c == b'/' || c == b'\\';
    if b.len() >= 2 && b[1] == b':' && b[0].is_ascii_alphabetic() {
        return 2;
    }
    if b.len() >= 3 && is_sep(b[0]) && is_sep(b[1]) && !is_sep(b[2]) {
        // \\host\share: up to the separator following the share name
        let Some(host_end) = b[2..].iter().position(|c| is_sep(*c)).map(|i| i + 2) else {
            return b.len();
        };
        return b[host_end + 1..]
            .iter()
            .position(|c| is_sep(*c))
            .map_or(b.len(), |i| i + host_end + 1);
    }
    0
}

/// `validateClusterInstallBaseValuesFile`: the absolute path of an existing regular file.
pub fn validate_cluster_install_base_values_file(path: &str) -> Result<String, String> {
    if path.is_empty() {
        return Ok(String::new());
    }
    let expanded = expand_path(path).map_err(|e| format!("expand base values file path: {e}"))?;
    let abs = if Path::new(&expanded).is_absolute() {
        go_clean(Path::new(&expanded))
    } else {
        let cwd =
            std::env::current_dir().map_err(|e| format!("resolve base values file path: {e}"))?;
        go_clean(&cwd.join(&expanded))
    };
    let meta = std::fs::metadata(&abs).map_err(|e| {
        format!(
            "read base values file path {}: {}",
            qovery_log::gofmt::quote(&abs),
            qovery_core::context::go_path_error("stat", Path::new(&abs), &e)
        )
    })?;
    if !meta.is_file() {
        return Err(format!(
            "base values file path {} must be a file",
            qovery_log::gofmt::quote(&abs)
        ));
    }
    Ok(abs)
}

fn run(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    let base_values_file =
        match validate_cluster_install_base_values_file(&ctx.string("base-values-file")) {
            Ok(p) => p,
            Err(e) => exit_with_error(e),
        };
    let prompt = PromptUiFactoryImpl;
    let organization_service = OrganizationServiceImpl {
        client: &client,
        prompt: &prompt,
    };
    let cluster_service = ClusterServiceImpl::new(&client, &prompt);
    let credentials_service = ClusterCredentialsServiceImpl {
        client: &client,
        prompt: &prompt,
    };
    let registry_service = ClusterContainerRegistryServiceImpl {
        client: &client,
        prompt: &prompt,
    };
    let self_managed_service = SelfManagedClusterServiceImpl {
        client: &client,
        cluster_service: &cluster_service,
        credentials_service: &credentials_service,
        container_registry_service: &registry_service,
        prompt: &prompt,
        local_base_helm_values_path: base_values_file,
    };
    let file_writer = FileWriterServiceImpl;
    let service = InstallSelfManagedClusterService {
        organization_service: &organization_service,
        self_managed_service: &self_managed_service,
        cluster_service: &cluster_service,
        file_writer: &file_writer,
        prompt: &prompt,
    };
    match service.install_cluster() {
        Err(e) => exit_with_error(e),
        Ok(Some(message)) => {
            qovery_log::println(format!("{message}\n"));
            exit(0);
        }
        Ok(None) => {}
    }
}

/// `InstallSelfManagedClusterService`.
pub struct InstallSelfManagedClusterService<'a> {
    pub organization_service: &'a dyn OrganizationService,
    pub self_managed_service: &'a dyn SelfManagedClusterService,
    pub cluster_service: &'a dyn ClusterService,
    pub file_writer: &'a dyn FileWriterService,
    pub prompt: &'a dyn PromptUiFactory,
}

const CLOUD_PROVIDERS: [(&str, &str); 12] = [
    ("Your AWS EKS cluster", "AWS"),
    ("Your GCP GKE cluster", "GCP"),
    ("Your Scaleway Kapsule cluster", "SCW"),
    ("Your Azure AKS cluster", "AZURE"),
    ("Your OVH kube cluster", "OVH"),
    ("Your Digital Ocean kube cluster", "DO"),
    ("Your Oracle Cloud kube cluster", "ORACLE"),
    ("Your Hetzner kube cluster", "HETZNER"),
    ("Your IBM Cloud kube cluster", "IBM"),
    ("Your Civo K3S cluster", "CIVO"),
    ("Your Local Machine", "ON_PREMISE"),
    ("Other", "ON_PREMISE"),
];

/// `getCloudVendor`.
fn get_cloud_vendor(kubernetes_type: &str) -> &'static str {
    CLOUD_PROVIDERS
        .iter()
        .find(|(n, _)| *n == kubernetes_type)
        .map(|(_, v)| *v)
        .unwrap_or("ON_PREMISE")
}

impl InstallSelfManagedClusterService<'_> {
    /// `InstallCluster`: returns either an error or an indication printed by the caller.
    pub fn install_cluster(&self) -> Result<Option<String>, String> {
        qovery_log::println("");
        qovery_log::println_info(
            "The following procedure allows you to generate the values files and the helm command necessary to install Qovery on your cluster. You can find more information on our public documentation: https://hub.qovery.com/docs/getting-started/install-qovery/kubernetes/quickstart/",
        );
        qovery_log::println("Cluster Type:");
        let keys: Vec<String> = CLOUD_PROVIDERS.iter().map(|(n, _)| n.to_string()).collect();
        let (_, kubernetes_type) = self.prompt.run_select_with_size(
            "Select where you want to install Qovery on",
            &keys,
            keys.len(),
        )?;
        if kubernetes_type.contains("Local Machine") {
            return Ok(Some(
                "Please use `qovery demo up` to create a demo cluster on your local machine"
                    .to_string(),
            ));
        }
        let cloud_vendor = get_cloud_vendor(&kubernetes_type);
        let organization = self
            .organization_service
            .ask_user_to_select_organization()?
            .ok_or_else(|| {
                "organization not found, please create one on https://console.qovery.com"
                    .to_string()
            })?;

        let clusters = self.cluster_service.list_clusters(&organization.id)?;
        let self_managed: Vec<Value> = clusters
            .into_iter()
            .filter(|c| {
                str_field(c, "kubernetes") == "SELF_MANAGED"
                    && str_field(c, "cloud_provider") == cloud_vendor
            })
            .collect();

        let mut cluster: Option<Value> = None;
        if !self_managed.is_empty() {
            qovery_log::println("You already have self-managed clusters in your organization.");
            qovery_log::println("Do you want to reuse one of them or create a new one?");
            let choices = vec![
                "Reuse a Cluster".to_string(),
                "Create a new cluster".to_string(),
            ];
            let (_, reuse) = self
                .prompt
                .run_select("Reuse or Create a new cluster?", &choices)?;
            if reuse == "Reuse a Cluster" {
                qovery_log::println("Select the cluster you want to reuse:");
                let names: Vec<String> = self_managed
                    .iter()
                    .map(|c| str_field(c, "name").to_string())
                    .collect();
                let (_, name) = self.prompt.run_select_with_size(
                    "Select the cluster you want to reuse:",
                    &names,
                    10,
                )?;
                cluster = find_cluster_by_name(&self_managed, &name).cloned();
            }
        }

        let cluster = match cluster {
            Some(c) => c,
            None => {
                let created = self
                    .self_managed_service
                    .create(&organization.id, cloud_vendor)?;
                self.self_managed_service.configure(&created)?;
                created
            }
        };

        qovery_log::println("Contact email for Let's Encrypt certificate:");
        let email = self.prompt.run_prompt(
            "Enter your email address to receive expiration notification from Let's Encrypt",
            "acme@qovery.com",
        )?;

        let mut helm_values = self
            .self_managed_service
            .get_installation_helm_values(&organization.id, str_field(&cluster, "id"))?;
        helm_values = helm_values.replace("acme@qovery.com", &email);
        helm_values = format!("{helm_values}\n");

        let base = self
            .self_managed_service
            .get_base_helm_values_content(map_cloud_vendor_to_cloud_provider_type(cloud_vendor))?;
        helm_values.push_str(&strip_qovery_section(&base));

        if kubernetes_type.contains("Azure") {
            helm_values = inject_azure_aks_values(&helm_values)?;
        }
        helm_values = inject_qovery_cluster_gateway_domain(&helm_values)?;
        helm_values = inject_external_dns_gateway_sources(&helm_values)?;
        helm_values = inject_envoy_ingress_services(&helm_values)?;

        let file_name = format!("values-{}.yaml", str_field(&cluster, "name").to_lowercase());
        let dir = std::env::current_dir().map_err(|e| e.to_string())?;
        let default_path = go_clean(&dir.join(file_name));
        let file_name = self
            .prompt
            .run_prompt("File path to save Helm Values to", &default_path)?;
        self.file_writer
            .write_file(&file_name, helm_values.as_bytes(), 0o644)?;
        output_commands_to_install_qovery_on_cluster(&file_name);
        Ok(None)
    }
}

/// `stripQoverySection`: remove the `qovery:` section of the base values (kept as text: the file uses anchors).
pub fn strip_qovery_section(values: &str) -> String {
    let re = regex::Regex::new(r"(?m)^qovery:\n( .*\n)+").expect("regex");
    re.replace_all(values, "").to_string()
}

fn output_commands_text(helm_values_file_name: &str) -> String {
    let mut out = String::new();
    let mut println = |s: &str| {
        out.push_str(s);
        out.push('\n');
    };
    println("");
    println("////////////////////////////////////////////////////////////////////////////////////");
    println("////              Follow these instructions to install your cluster             ////");
    println("////////////////////////////////////////////////////////////////////////////////////");
    println("\n# Add the Qovery Helm repository\nhelm repo add qovery https://helm.qovery.com");
    println("helm repo update");
    println(&format!(
        "\n# Verify the helm values\nQovery provides you with a default configuration that can be customized based on your needs. More information here: https://hub.qovery.com/docs/getting-started/install-qovery/kubernetes/byok-config\nHelm values location: {helm_values_file_name}\n\t"
    ));
    println(
        "\n# Pre-apply Gateway API and Envoy CRDs before the main Helm release.
# The CRD bundle is too large to fit reliably in Helm's release Secret storage.
helm pull qovery/qovery --untar --untardir /tmp/qovery-helm-chart
helm template qovery-gateway-crds /tmp/qovery-helm-chart/qovery/charts/gateway-crds-helm \\
\t --set crds.gatewayAPI.enabled=true \\
\t --set crds.gatewayAPI.channel=standard \\
\t --set crds.envoyGateway.enabled=true | kubectl apply --server-side -f -
kubectl wait --for=condition=Established --timeout=180s crd/gateways.gateway.networking.k8s.io
kubectl wait --for=condition=Established --timeout=180s crd/envoyproxies.gateway.envoyproxy.io",
    );
    println(
        "\n# Note: --rollback-on-failure requires Helm >= 3.15.0 (replaces the deprecated --atomic flag).
# Check your version with: helm version",
    );
    println(&format!(
        "\n# Install Qovery on your cluster first, without some services to avoid circular dependency errors.
helm upgrade --install --create-namespace -n qovery -f \"{helm_values_file_name}\" --rollback-on-failure \\
\t --set services.certificates.cert-manager-configs.enabled=false \\
\t --set services.certificates.qovery-cert-manager-webhook.enabled=false \\
\t --set services.ingress.envoy-gateway-crd.enabled=false \\
\t --set qovery-cluster-gateway.metrics.enabled=false \\
\t --set qovery-cluster-gateway.metrics.podMonitor.enabled=false \\
\t --set services.ingress.envoy-gateway.enabled=false \\
\t --set services.ingress.qovery-gateway-class.enabled=false \\
\t --set services.ingress.qovery-cluster-gateway.enabled=false \\
\t --set services.qovery.qovery-cluster-agent.enabled=false \\
\t --set services.qovery.qovery-engine.enabled=false \\
\t --set services.qovery.qovery-operator.enabled=false \\
\t qovery qovery/qovery"
    ));
    println(&format!(
        "\n# Then, re-apply the Qovery installation with the remaining services
helm upgrade --install --create-namespace -n qovery -f \"{helm_values_file_name}\" --wait --rollback-on-failure \\
\t --set services.ingress.envoy-gateway-crd.enabled=false \\
\t --set qovery-cluster-gateway.metrics.enabled=false \\
\t --set qovery-cluster-gateway.metrics.podMonitor.enabled=false \\
\t --set services.qovery.qovery-operator.enabled=false \\
\t qovery qovery/qovery\n"
    ));
    println("////////////////////////////////////////////////////////////////////////////////////");
    out
}

/// `outputCommandsToInstallQoveryOnCluster`.
fn output_commands_to_install_qovery_on_cluster(helm_values_file_name: &str) {
    use std::io::Write;
    let text = output_commands_text(helm_values_file_name);
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(text.as_bytes());
    let _ = out.flush();
    drop(out);
    qovery_log::println_info(
        "Please note that the installation process may take a few minutes to complete.",
    );
}

fn map_entry<'a>(m: &'a mut GoYaml, key: &str) -> Option<&'a mut GoYaml> {
    m.get_mut(key).filter(|v| v.is_map())
}

/// `injectAzureAKSValues`.
pub fn inject_azure_aks_values(content: &str) -> Result<String, String> {
    let mut values = goyaml::unmarshal(content)?;
    let not_map = |v: Option<&GoYaml>| match v {
        None | Some(GoYaml::Null) => {
            "interface conversion: interface {} is nil, not map[string]interface {}".to_string()
        }
        Some(_) => "interface conversion: interface {} is not map[string]interface {}".to_string(),
    };
    if map_entry(&mut values, "ingress-nginx").is_none() {
        return Err(not_map(values.get("ingress-nginx")));
    }
    let ingress = values.get_mut("ingress-nginx").expect("checked");
    if map_entry(ingress, "controller").is_none() {
        return Err(not_map(ingress.get("controller")));
    }
    let controller = ingress.get_mut("controller").expect("checked");
    let annotation = || {
        GoYaml::Map(vec![(
            "service.beta.kubernetes.io/azure-load-balancer-internal".into(),
            GoYaml::Str("false".into()),
        )])
    };
    match controller.get("service") {
        None | Some(GoYaml::Null) => controller.set(
            "service",
            GoYaml::Map(vec![
                ("externalTrafficPolicy".into(), GoYaml::Str("Local".into())),
                ("annotations".into(), annotation()),
            ]),
        ),
        Some(GoYaml::Map(_)) => {
            let service = controller.get_mut("service").expect("checked");
            service.set("externalTrafficPolicy", GoYaml::Str("Local".into()));
            match service.get("annotations") {
                None | Some(GoYaml::Null) => service.set("annotations", annotation()),
                Some(GoYaml::Map(_)) => service.get_mut("annotations").expect("checked").set(
                    "service.beta.kubernetes.io/azure-load-balancer-internal",
                    GoYaml::Str("false".into()),
                ),
                Some(_) => {
                    return Err(
                        "interface conversion: interface {} is not map[string]interface {}"
                            .to_string(),
                    );
                }
            }
        }
        Some(_) => {
            return Err(
                "interface conversion: interface {} is not map[string]interface {}".to_string(),
            );
        }
    }
    Ok(goyaml::marshal(&values))
}

/// `injectQoveryClusterGatewayDomain`.
pub fn inject_qovery_cluster_gateway_domain(content: &str) -> Result<String, String> {
    let Ok(mut values) = goyaml::unmarshal(content) else {
        return Ok(content.to_string());
    };
    let Some(domain) = values
        .get("qovery")
        .filter(|q| q.is_map())
        .and_then(|q| q.get("domain"))
        .and_then(|d| d.as_str())
        .map(|s| s.to_string())
    else {
        return Ok(content.to_string());
    };
    if domain.trim().is_empty() {
        return Ok(content.to_string());
    }
    if map_entry(&mut values, "qovery-cluster-gateway").is_none() {
        values.set("qovery-cluster-gateway", GoYaml::Map(vec![]));
    }
    let gateway = values.get_mut("qovery-cluster-gateway").expect("set");
    if map_entry(gateway, "dns").is_none() {
        gateway.set("dns", GoYaml::Map(vec![]));
    }
    let dns = gateway.get_mut("dns").expect("set");
    if dns
        .get("domain")
        .and_then(|d| d.as_str())
        .is_some_and(|d| !d.trim().is_empty())
    {
        return Ok(content.to_string());
    }
    dns.set("domain", GoYaml::Str(domain));
    Ok(goyaml::marshal(&values))
}

/// `injectExternalDNSGatewaySources`.
pub fn inject_external_dns_gateway_sources(content: &str) -> Result<String, String> {
    let Ok(mut values) = goyaml::unmarshal(content) else {
        return Ok(content.to_string());
    };
    let Some(external_dns) = map_entry(&mut values, "external-dns") else {
        return Ok(content.to_string());
    };
    if !external_dns.contains_key("sources") {
        external_dns.set(
            "sources",
            GoYaml::Seq(
                [
                    "service",
                    "ingress",
                    "gateway-httproute",
                    "gateway-grpcroute",
                ]
                .iter()
                .map(|s| GoYaml::Str(s.to_string()))
                .collect(),
            ),
        );
    }
    if !external_dns.contains_key("enableGatewayListenerSets") {
        external_dns.set("enableGatewayListenerSets", GoYaml::Bool(true));
    }
    Ok(goyaml::marshal(&values))
}

/// `injectEnvoyIngressServices`.
pub fn inject_envoy_ingress_services(content: &str) -> Result<String, String> {
    let Ok(mut values) = goyaml::unmarshal(content) else {
        return Ok(content.to_string());
    };
    let Some(services) = map_entry(&mut values, "services") else {
        return Ok(content.to_string());
    };
    if map_entry(services, "ingress").is_none() {
        services.set("ingress", GoYaml::Map(vec![]));
    }
    let ingress = services.get_mut("ingress").expect("set");
    for (name, enabled) in [
        ("ingress-nginx", false),
        ("envoy-gateway-crd", false),
        ("envoy-gateway", true),
        ("qovery-gateway-class", true),
        ("qovery-cluster-gateway", true),
    ] {
        if map_entry(ingress, name).is_none() {
            ingress.set(name, GoYaml::Map(vec![]));
        }
        ingress
            .get_mut(name)
            .expect("set")
            .set("enabled", GoYaml::Bool(enabled));
    }
    Ok(goyaml::marshal(&values))
}

#[cfg(test)]
mod tests;
