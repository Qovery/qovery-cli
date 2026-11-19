//! Port of `demoLocalCommands` (`cmd/demo_up.go`, `cmd/demo_destroy.go`): docker, k3d, helm and kubectl calls.

use crate::failure::*;
use crate::runner::{Cancel, CommandRunner, ExecError};
use qovery_log::gofmt::quote;
use serde::Deserialize;
use std::collections::BTreeMap;
use std::io::Write;

pub(crate) const DEMO_K3S_IMAGE: &str = "docker.io/rancher/k3s:v1.36.4-k3s1";
pub(crate) const DEMO_SUBNET: &str = "172.42.0.0/16";
pub(crate) const DEMO_NODE_IP: &str = "172.42.0.3";
pub(crate) const DEMO_REGISTRY: &str = "qovery-registry.lan";

/// `demoBootstrap`.
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct Bootstrap {
    pub release_name: String,
    pub chart_reference: String,
    pub chart_version: String,
    pub namespace: String,
    pub values_yaml: String,
}

/// `demoLocal` (used by `demo up`).
pub(crate) trait DemoLocal {
    fn check_dependencies(&mut self, cancel: Cancel) -> Result<(), DemoError>;
    fn ensure_k3d_cluster(&mut self, cancel: Cancel, name: &str) -> Result<(), DemoError>;
    fn ensure_loopback(&mut self, cancel: Cancel) -> Result<(), DemoError>;
    fn legacy_qovery_release_exists(&mut self, cancel: Cancel) -> Result<bool, DemoError>;
    fn install_operator(&mut self, cancel: Cancel, bootstrap: &Bootstrap) -> Result<(), DemoError>;
    fn validate_workloads(&mut self, cancel: Cancel, namespace: &str) -> Result<(), DemoError>;
}

/// `demoDestroyLocal` (used by `demo destroy`).
pub(crate) trait DemoDestroyLocal {
    fn check_destroy_dependencies(&mut self, cancel: Cancel) -> Result<(), DemoError>;
    fn delete_k3d_cluster(&mut self, cancel: Cancel, name: &str) -> Result<(), DemoError>;
    fn teardown_loopback(&mut self, cancel: Cancel) -> Result<(), DemoError>;
}

/// `demoLocalCommands`.
pub(crate) struct LocalCommands<R: CommandRunner> {
    pub runner: R,
    pub goos: String,
    pub temp_dir: String,
    pub kube_context: String,
}

/// `demoKubeContext(clusterName)`.
pub(crate) fn demo_kube_context(cluster_name: &str) -> String {
    format!("k3d-{cluster_name}")
}

/// `commandFailed(operation, err)`.
pub(crate) fn command_failed(operation: &str, err: &ExecError) -> DemoError {
    match err.exit_code {
        Some(code) => DemoError::new(format!("{operation} failed with exit code {code}")),
        None => DemoError::new(format!("{operation} failed")),
    }
}

fn args(values: &[&str]) -> Vec<String> {
    values.iter().map(|s| s.to_string()).collect()
}

/// `loopbackHasAddress(output, ip)`.
pub(crate) fn loopback_has_address(output: &[u8], ip: &str) -> bool {
    let text = String::from_utf8_lossy(output);
    let fields: Vec<&str> = text.split_whitespace().collect();
    fields.iter().enumerate().any(|(i, f)| {
        *f == "inet" && i + 1 < fields.len() && fields[i + 1].split('/').next() == Some(ip)
    })
}

fn is_wsl() -> bool {
    std::fs::read("/proc/version")
        .map(|v| {
            String::from_utf8_lossy(&v)
                .to_lowercase()
                .contains("microsoft")
        })
        .unwrap_or(false)
}

/// `json.Unmarshal` into `[]struct{Name string \`json:"name"\`}`.
#[derive(Deserialize)]
struct Named {
    #[serde(default, alias = "Name", alias = "NAME")]
    name: Option<String>,
}

fn parse_names(output: &[u8]) -> Option<Vec<String>> {
    let values: Option<Vec<Option<Named>>> = serde_json::from_slice(output).ok()?;
    Some(
        values
            .unwrap_or_default()
            .into_iter()
            .map(|n| n.and_then(|n| n.name).unwrap_or_default())
            .collect(),
    )
}

// --- subnet overlap (`netip.Prefix.Overlaps`) -------------------------------------------------------------

#[derive(Debug, Clone, Copy, PartialEq)]
struct Prefix {
    v6: bool,
    addr: u128,
    bits: u32,
}

fn parse_prefix(s: &str) -> Option<Prefix> {
    let (ip, bits) = s.split_once('/')?;
    if bits.is_empty()
        || bits.len() > 3
        || !bits.bytes().all(|b| b.is_ascii_digit())
        || (bits.len() > 1 && bits.starts_with('0'))
    {
        return None;
    }
    let bits: u32 = bits.parse().ok()?;
    if let Ok(v4) = ip.parse::<std::net::Ipv4Addr>() {
        if bits > 32 {
            return None;
        }
        return Some(Prefix {
            v6: false,
            addr: u32::from(v4) as u128,
            bits,
        });
    }
    if ip.contains('%') {
        return None;
    }
    let v6 = ip.parse::<std::net::Ipv6Addr>().ok()?;
    if bits > 128 {
        return None;
    }
    Some(Prefix {
        v6: true,
        addr: u128::from(v6),
        bits,
    })
}

fn overlaps(a: Prefix, b: Prefix) -> bool {
    if a.v6 != b.v6 {
        return false;
    }
    let width = if a.v6 { 128 } else { 32 };
    let min = a.bits.min(b.bits);
    let shift = width - min;
    let mask = |x: u128| if shift >= 128 { 0 } else { x >> shift };
    mask(a.addr) == mask(b.addr)
}

// ---------------------------------------------------------------------------------------------------------

impl<R: CommandRunner> LocalCommands<R> {
    pub fn new(runner: R, goos: &str) -> Self {
        LocalCommands {
            runner,
            goos: goos.to_string(),
            temp_dir: String::new(),
            kube_context: String::new(),
        }
    }

    fn ensure_demo_subnet_is_free(
        &mut self,
        cancel: Cancel,
        cluster_name: &str,
    ) -> Result<(), DemoError> {
        let (ids, err) =
            self.runner
                .run_quiet(cancel, "docker", &args(&["network", "ls", "--quiet"]));
        let ids_text = String::from_utf8_lossy(&ids).to_string();
        let ids: Vec<&str> = ids_text.split_whitespace().collect();
        if err.is_err() || ids.is_empty() {
            return Ok(());
        }
        let mut inspect = args(&["network", "inspect"]);
        inspect.extend(ids.iter().map(|s| s.to_string()));
        let (output, err) = self.runner.run_unlogged(cancel, "docker", &inspect);
        if err.is_err() {
            return Ok(());
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Config {
            #[serde(alias = "subnet")]
            #[serde(rename = "Subnet")]
            subnet: Option<String>,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Ipam {
            #[serde(rename = "Config")]
            config: Option<Vec<Config>>,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Network {
            #[serde(rename = "Name")]
            name: Option<String>,
            #[serde(rename = "IPAM")]
            ipam: Option<Ipam>,
        }
        let Ok(networks) = serde_json::from_slice::<Option<Vec<Option<Network>>>>(&output) else {
            return Ok(());
        };
        let networks = networks
            .unwrap_or_default()
            .into_iter()
            .map(|n| n.unwrap_or_default());
        let demo_prefix = parse_prefix(DEMO_SUBNET).expect("valid demo subnet");
        for network in networks {
            let name = network.name.unwrap_or_default();
            if name == demo_kube_context(cluster_name) {
                continue;
            }
            for config in network.ipam.and_then(|i| i.config).unwrap_or_default() {
                if let Some(prefix) = parse_prefix(config.subnet.as_deref().unwrap_or(""))
                    && overlaps(prefix, demo_prefix)
                {
                    return Err(demo_subnet_in_use_error(&name));
                }
            }
        }
        Ok(())
    }

    fn ensure_demo_registry(
        &mut self,
        cancel: Cancel,
        cluster_name: &str,
    ) -> Result<(), DemoError> {
        let (container_name, found) = self.find_demo_registry(cancel)?;
        if found {
            return self.ensure_demo_registry_alias(cancel, cluster_name, &container_name);
        }
        let network = demo_kube_context(cluster_name);
        let (_, err) = self.runner.run(
            cancel,
            "k3d",
            &args(&[
                "registry",
                "create",
                DEMO_REGISTRY,
                "--default-network",
                &network,
                "--no-help",
            ]),
        );
        if let Err(e) = err {
            return Err(command_failed("k3d registry create", &e));
        }
        let (container_name, found) = self.find_demo_registry(cancel)?;
        if !found {
            return Err(DemoError::new(
                "k3d did not expose the created demo registry",
            ));
        }
        self.ensure_demo_registry_alias(cancel, cluster_name, &container_name)
    }

    pub(crate) fn find_demo_registry(
        &mut self,
        cancel: Cancel,
    ) -> Result<(String, bool), DemoError> {
        let (output, err) = self.runner.run_quiet(
            cancel,
            "k3d",
            &args(&["registry", "list", "--output", "json"]),
        );
        if let Err(e) = err {
            return Err(command_failed("k3d registry list", &e));
        }
        let Some(registries) = parse_names(&output) else {
            return Err(DemoError::new("k3d returned an invalid registry list"));
        };
        for name in registries {
            if name == DEMO_REGISTRY || name == format!("k3d-{DEMO_REGISTRY}") {
                return Ok((name, true));
            }
        }
        Ok((String::new(), false))
    }

    fn ensure_demo_registry_alias(
        &mut self,
        cancel: Cancel,
        cluster_name: &str,
        container_name: &str,
    ) -> Result<(), DemoError> {
        let network_name = demo_kube_context(cluster_name);
        let (output, err) =
            self.runner
                .run_quiet(cancel, "docker", &args(&["inspect", container_name]));
        if let Err(e) = err {
            return Err(command_failed("docker inspect demo registry", &e));
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Endpoint {
            #[serde(rename = "Aliases")]
            aliases: Option<Vec<String>>,
            #[serde(rename = "DNSNames")]
            dns_names: Option<Vec<String>>,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Settings {
            #[serde(rename = "Networks")]
            networks: Option<BTreeMap<String, Option<Endpoint>>>,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Container {
            #[serde(rename = "NetworkSettings")]
            network_settings: Option<Settings>,
        }
        let containers: Vec<Container> =
            match serde_json::from_slice::<Option<Vec<Option<Container>>>>(&output) {
                Ok(c) => c
                    .unwrap_or_default()
                    .into_iter()
                    .map(|c| c.unwrap_or_default())
                    .collect(),
                Err(_) => {
                    return Err(DemoError::new(
                        "docker returned invalid demo registry details",
                    ));
                }
            };
        if containers.len() != 1 {
            return Err(DemoError::new(
                "docker returned invalid demo registry details",
            ));
        }
        let networks = containers
            .into_iter()
            .next()
            .and_then(|c| c.network_settings)
            .and_then(|s| s.networks)
            .unwrap_or_default();
        let network = networks.get(&network_name);
        let connected = network.is_some();
        if let Some(endpoint) = network {
            let endpoint = endpoint.as_ref();
            let has = |v: Option<&Vec<String>>| {
                v.map(|v| v.iter().any(|s| s == DEMO_REGISTRY))
                    .unwrap_or(false)
            };
            if has(endpoint.and_then(|e| e.aliases.as_ref()))
                || has(endpoint.and_then(|e| e.dns_names.as_ref()))
            {
                return Ok(());
            }
        }
        if connected {
            let (_, err) = self.runner.run(
                cancel,
                "docker",
                &args(&["network", "disconnect", &network_name, container_name]),
            );
            if let Err(e) = err {
                return Err(command_failed(
                    "docker network disconnect demo registry",
                    &e,
                ));
            }
        }
        let (_, err) = self.runner.run(
            cancel,
            "docker",
            &args(&[
                "network",
                "connect",
                "--alias",
                DEMO_REGISTRY,
                &network_name,
                container_name,
            ]),
        );
        if let Err(e) = err {
            return Err(command_failed("docker network connect demo registry", &e));
        }
        Ok(())
    }

    fn demo_registry_used_outside_cluster(
        &mut self,
        cancel: Cancel,
        cluster_name: &str,
    ) -> Result<bool, DemoError> {
        let (container_name, found) = self.find_demo_registry(cancel)?;
        if !found {
            return Ok(false);
        }
        let (output, err) =
            self.runner
                .run_quiet(cancel, "docker", &args(&["inspect", &container_name]));
        if let Err(e) = err {
            return Err(command_failed("docker inspect demo registry", &e));
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Settings {
            #[serde(rename = "Networks")]
            networks: Option<BTreeMap<String, serde_json::Value>>,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Container {
            #[serde(rename = "NetworkSettings")]
            network_settings: Option<Settings>,
        }
        let containers: Vec<Container> =
            match serde_json::from_slice::<Option<Vec<Option<Container>>>>(&output) {
                Ok(c) => c
                    .unwrap_or_default()
                    .into_iter()
                    .map(|c| c.unwrap_or_default())
                    .collect(),
                Err(_) => {
                    return Err(DemoError::new(
                        "docker returned invalid demo registry details",
                    ));
                }
            };
        if containers.len() != 1 {
            return Err(DemoError::new(
                "docker returned invalid demo registry details",
            ));
        }
        let target = demo_kube_context(cluster_name);
        let networks = containers
            .into_iter()
            .next()
            .and_then(|c| c.network_settings)
            .and_then(|s| s.networks)
            .unwrap_or_default();
        Ok(networks
            .keys()
            .any(|n| n.starts_with("k3d-") && *n != target))
    }

    fn delete_demo_registry(&mut self, cancel: Cancel) -> Result<(), DemoError> {
        let (container_name, found) = self.find_demo_registry(cancel)?;
        if !found {
            return Ok(());
        }
        let (_, err) = self.runner.run(
            cancel,
            "k3d",
            &args(&["registry", "delete", &container_name]),
        );
        if let Err(e) = err {
            return Err(command_failed("k3d registry delete", &e));
        }
        Ok(())
    }

    fn powershell(&mut self) -> String {
        if self.runner.look_path("powershell.exe").is_ok() {
            "powershell.exe".to_string()
        } else {
            "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe".to_string()
        }
    }
}

/// `buildDemoHelmArgs(bootstrap, valuesPath, kubeContext)`.
pub(crate) fn build_demo_helm_args(
    bootstrap: &Bootstrap,
    values_path: &str,
    kube_context: &str,
) -> Result<Vec<String>, DemoError> {
    if bootstrap.release_name.trim().is_empty()
        || bootstrap.chart_reference.trim().is_empty()
        || bootstrap.chart_version.trim().is_empty()
        || bootstrap.namespace.trim().is_empty()
    {
        return Err(DemoError::new(
            "operator bootstrap is missing structured Helm fields",
        ));
    }
    if go_path_base(
        bootstrap
            .chart_reference
            .strip_suffix('/')
            .unwrap_or(&bootstrap.chart_reference),
    ) == "qovery"
    {
        return Err(DemoError::new(
            "refusing to install the legacy Qovery umbrella chart",
        ));
    }
    Ok(args(&[
        "upgrade",
        "--install",
        "--kube-context",
        kube_context,
        &bootstrap.release_name,
        &bootstrap.chart_reference,
        "--version",
        &bootstrap.chart_version,
        "--namespace",
        &bootstrap.namespace,
        "--values",
        values_path,
        "--create-namespace",
        "--atomic",
        "--wait",
        "--timeout",
        "15m",
    ]))
}

/// Go `path.Base`.
fn go_path_base(p: &str) -> &str {
    if p.is_empty() {
        return ".";
    }
    let trimmed = p.trim_end_matches('/');
    if trimmed.is_empty() {
        return "/";
    }
    trimmed.rsplit('/').next().unwrap_or(trimmed)
}

/// `file.Chmod(0600)`.
#[cfg(unix)]
fn chmod_owner_read_write(file: &std::fs::File) -> std::io::Result<()> {
    file.set_permissions(std::os::unix::fs::PermissionsExt::from_mode(0o600))
}

/// `file.Chmod(0600)`: on Windows Go only clears the read-only attribute.
#[cfg(not(unix))]
fn chmod_owner_read_write(file: &std::fs::File) -> std::io::Result<()> {
    let mut permissions = file.metadata()?.permissions();
    #[allow(clippy::permissions_set_readonly_false)]
    permissions.set_readonly(false);
    file.set_permissions(permissions)
}

/// `file.Close()`: unlike dropping a `File`, reports the error of close(2).
#[cfg(unix)]
fn close(file: std::fs::File) -> std::io::Result<()> {
    use std::os::fd::IntoRawFd;
    // SAFETY: the descriptor is owned by `file`, which `into_raw_fd` consumed: it is closed exactly once.
    if unsafe { libc::close(file.into_raw_fd()) } == 0 {
        Ok(())
    } else {
        Err(std::io::Error::last_os_error())
    }
}

/// `file.Close()` (Windows is refused before any file is written).
#[cfg(not(unix))]
fn close(file: std::fs::File) -> std::io::Result<()> {
    drop(file);
    Ok(())
}

/// `os.CreateTemp(dir, "qovery-demo-operator-values-*.yaml")` (mode 0600).
fn create_temp(dir: &str) -> std::io::Result<(std::fs::File, std::path::PathBuf)> {
    let dir = if dir.is_empty() {
        std::env::temp_dir()
    } else {
        std::path::PathBuf::from(dir)
    };
    let mut seed = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_nanos())
        .unwrap_or(0) as u64
        ^ ((std::process::id() as u64) << 32);
    for _ in 0..10000 {
        seed = seed
            .wrapping_mul(6364136223846793005)
            .wrapping_add(1442695040888963407);
        let path = dir.join(format!(
            "qovery-demo-operator-values-{}.yaml",
            (seed >> 33) as u32
        ));
        match crate::open_options_with_mode(0o600)
            .write(true)
            .create_new(true)
            .open(&path)
        {
            Ok(f) => return Ok((f, path)),
            Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => continue,
            Err(e) => return Err(e),
        }
    }
    Err(std::io::Error::other("too many attempts"))
}

impl<R: CommandRunner> DemoLocal for LocalCommands<R> {
    fn check_dependencies(&mut self, cancel: Cancel) -> Result<(), DemoError> {
        let missing: Vec<String> = ["docker", "k3d", "helm", "kubectl"]
            .iter()
            .filter(|d| self.runner.look_path(d).is_err())
            .map(|d| d.to_string())
            .collect();
        if !missing.is_empty() {
            return Err(missing_demo_dependencies_error(&self.goos, &missing));
        }
        if self
            .runner
            .run_quiet(cancel, "docker", &args(&["info"]))
            .1
            .is_err()
        {
            return Err(new_demo_failure(
                ERROR_DOCKER_NOT_RUNNING,
                "Docker is not running.",
                DemoError::new("docker is not running"),
            ));
        }
        Ok(())
    }

    fn ensure_k3d_cluster(&mut self, cancel: Cancel, name: &str) -> Result<(), DemoError> {
        let (output, err) = self.runner.run_unlogged(
            cancel,
            "k3d",
            &args(&["cluster", "list", "--output", "json"]),
        );
        if let Err(e) = err {
            return Err(command_failed("k3d cluster list", &e));
        }
        let Some(clusters) = parse_names(&output) else {
            return Err(DemoError::new("k3d returned an invalid cluster list"));
        };
        if clusters.iter().any(|c| c == name) {
            let (_, err) = self
                .runner
                .run(cancel, "k3d", &args(&["cluster", "start", name]));
            if let Err(e) = err {
                return Err(command_failed("k3d cluster start", &e));
            }
            return self.ensure_demo_registry(cancel, name);
        }
        self.ensure_demo_subnet_is_free(cancel, name)?;
        let node_ip = format!("--node-ip={DEMO_NODE_IP}@server:0");
        let create = args(&[
            "cluster",
            "create",
            name,
            "--image",
            DEMO_K3S_IMAGE,
            "--subnet",
            DEMO_SUBNET,
            "--k3s-arg",
            &node_ip,
            "--k3s-arg",
            "--disable=traefik@server:*",
            "--registry-create",
            DEMO_REGISTRY,
            "--port",
            "80:80@loadbalancer",
            "--port",
            "443:443@loadbalancer",
        ]);
        let (_, err) = self.runner.run(cancel, "k3d", &create);
        if let Err(e) = err {
            return Err(command_failed("k3d cluster create", &e));
        }
        self.ensure_demo_registry(cancel, name)
    }

    fn ensure_loopback(&mut self, cancel: Cancel) -> Result<(), DemoError> {
        match self.goos.as_str() {
            "darwin" => {
                let (output, err) = self.runner.run_quiet(cancel, "ifconfig", &args(&["lo0"]));
                if let Err(e) = err {
                    return Err(command_failed("macOS loopback inspection", &e));
                }
                if loopback_has_address(&output, DEMO_NODE_IP) {
                    return Ok(());
                }
                let alias = format!("{DEMO_NODE_IP}/32");
                let (_, err) = self.runner.run(
                    cancel,
                    "sudo",
                    &args(&["ifconfig", "lo0", "alias", &alias, "up"]),
                );
                if let Err(e) = err {
                    return Err(command_failed("macOS loopback configuration", &e));
                }
            }
            "linux" if is_wsl() => {
                let address = format!("{DEMO_NODE_IP}/32");
                let (output, err) = self.runner.run(
                    cancel,
                    "sudo",
                    &args(&["ip", "addr", "add", &address, "dev", "lo"]),
                );
                if let Err(e) = err
                    && !String::from_utf8_lossy(&output)
                        .to_lowercase()
                        .contains("exists")
                {
                    return Err(command_failed("WSL loopback configuration", &e));
                }
                let powershell = self.powershell();
                let command = format!(
                    "Start-Process netsh -Verb RunAs -Wait -ArgumentList \"interface ipv4 add address name='Loopback Pseudo-Interface 1' address={DEMO_NODE_IP} mask=255.255.255.255 skipassource=true\""
                );
                let (output, err) = self.runner.run(
                    cancel,
                    &powershell,
                    &args(&["-NoProfile", "-Command", &command]),
                );
                if let Err(e) = err
                    && !String::from_utf8_lossy(&output)
                        .to_lowercase()
                        .contains("exists")
                {
                    return Err(command_failed("Windows loopback configuration", &e));
                }
            }
            _ => {}
        }
        Ok(())
    }

    fn legacy_qovery_release_exists(&mut self, cancel: Cancel) -> Result<bool, DemoError> {
        // Helm 4 removed `helm list --all`; listing every status explicitly works on Helm 3 and 4.
        let kube_context = self.kube_context.clone();
        let (output, err) = self.runner.run_quiet(
            cancel,
            "helm",
            &args(&[
                "list",
                "--kube-context",
                &kube_context,
                "--namespace",
                "qovery",
                "--deployed",
                "--failed",
                "--pending",
                "--superseded",
                "--uninstalling",
                "--output",
                "json",
                "--filter",
                "^qovery$",
            ]),
        );
        if let Err(e) = err {
            return Err(command_failed("helm list", &e));
        }
        let Some(releases) = parse_names(&output) else {
            return Err(DemoError::new("helm returned an invalid release list"));
        };
        Ok(releases.iter().any(|r| r == "qovery"))
    }

    fn install_operator(&mut self, cancel: Cancel, bootstrap: &Bootstrap) -> Result<(), DemoError> {
        let Ok((mut file, path)) = create_temp(&self.temp_dir) else {
            return Err(DemoError::new(
                "cannot create protected temporary Operator values file",
            ));
        };
        struct Remove(std::path::PathBuf);
        impl Drop for Remove {
            fn drop(&mut self) {
                let _ = std::fs::remove_file(&self.0);
            }
        }
        let _remove = Remove(path.clone());
        {
            if chmod_owner_read_write(&file).is_err() {
                return Err(DemoError::new(
                    "cannot protect temporary Operator values file",
                ));
            }
        }
        if file.write_all(bootstrap.values_yaml.as_bytes()).is_err() {
            return Err(DemoError::new(
                "cannot write temporary Operator values file",
            ));
        }
        if close(file).is_err() {
            return Err(DemoError::new(
                "cannot close temporary Operator values file",
            ));
        }
        let path_text = path.to_string_lossy().to_string();
        let helm_args = build_demo_helm_args(bootstrap, &path_text, &self.kube_context)?;
        let (_, err) = self.runner.run(cancel, "helm", &helm_args);
        if let Err(e) = err {
            return Err(command_failed("helm upgrade --install", &e));
        }
        Ok(())
    }

    fn validate_workloads(&mut self, cancel: Cancel, namespace: &str) -> Result<(), DemoError> {
        let kube_context = self.kube_context.clone();
        let (output, err) = self.runner.run_quiet(
            cancel,
            "kubectl",
            &args(&[
                "--context",
                &kube_context,
                "--namespace",
                namespace,
                "get",
                "deployments",
                "--output",
                "json",
            ]),
        );
        if let Err(e) = err {
            return Err(command_failed("kubectl get deployments", &e));
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Metadata {
            name: Option<String>,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct Item {
            metadata: Metadata,
        }
        #[derive(Deserialize, Default)]
        #[serde(default)]
        struct List {
            items: Option<Vec<Item>>,
        }
        let Ok(deployments) = serde_json::from_slice::<List>(&output) else {
            return Err(DemoError::new(
                "kubectl returned an invalid deployment list",
            ));
        };
        let mut operator_found = false;
        for deployment in deployments.items.unwrap_or_default() {
            match deployment.metadata.name.as_deref().unwrap_or("") {
                "qovery-operator" => operator_found = true,
                "qovery-engine" => {
                    return Err(DemoError::new(format!(
                        "unexpected permanent Deployment qovery-engine exists in namespace {namespace}"
                    )));
                }
                _ => {}
            }
        }
        if !operator_found {
            return Err(DemoError::new(
                "deployment qovery-operator was not found after installation",
            ));
        }
        Ok(())
    }
}

impl<R: CommandRunner> DemoDestroyLocal for LocalCommands<R> {
    fn check_destroy_dependencies(&mut self, _cancel: Cancel) -> Result<(), DemoError> {
        for dependency in ["docker", "k3d"] {
            if self.runner.look_path(dependency).is_err() {
                return Err(DemoError::new(format!(
                    "required command {} is not installed",
                    quote(dependency)
                )));
            }
        }
        Ok(())
    }

    fn delete_k3d_cluster(&mut self, cancel: Cancel, name: &str) -> Result<(), DemoError> {
        let (output, err) = self.runner.run_unlogged(
            cancel,
            "k3d",
            &args(&["cluster", "list", "--output", "json"]),
        );
        if let Err(e) = err {
            return Err(command_failed("k3d cluster list", &e));
        }
        let Some(clusters) = parse_names(&output) else {
            return Err(DemoError::new("k3d returned an invalid cluster list"));
        };
        let registry_shared = self.demo_registry_used_outside_cluster(cancel, name)?;
        if clusters.iter().any(|c| c == name) {
            let (_, err) = self
                .runner
                .run(cancel, "k3d", &args(&["cluster", "delete", name]));
            if let Err(e) = err {
                return Err(command_failed("k3d cluster delete", &e));
            }
        }
        if registry_shared {
            return Ok(());
        }
        self.delete_demo_registry(cancel)
    }

    fn teardown_loopback(&mut self, cancel: Cancel) -> Result<(), DemoError> {
        if self.goos == "darwin" {
            let (output, err) = self.runner.run_quiet(cancel, "ifconfig", &args(&["lo0"]));
            if let Err(e) = err {
                return Err(command_failed("macOS loopback inspection", &e));
            }
            if !loopback_has_address(&output, DEMO_NODE_IP) {
                return Ok(());
            }
            let (_, err) = self.runner.run(
                cancel,
                "sudo",
                &args(&["ifconfig", "lo0", "-alias", DEMO_NODE_IP]),
            );
            if let Err(e) = err {
                return Err(command_failed("macOS loopback removal", &e));
            }
            return Ok(());
        }
        if self.goos != "linux" || !is_wsl() {
            return Ok(());
        }
        let (output, err) =
            self.runner
                .run_quiet(cancel, "ip", &args(&["addr", "show", "dev", "lo"]));
        if let Err(e) = err {
            return Err(command_failed("WSL loopback inspection", &e));
        }
        if loopback_has_address(&output, DEMO_NODE_IP) {
            let address = format!("{DEMO_NODE_IP}/32");
            let (_, err) = self.runner.run(
                cancel,
                "sudo",
                &args(&["ip", "addr", "del", &address, "dev", "lo"]),
            );
            if let Err(e) = err {
                return Err(command_failed("WSL loopback removal", &e));
            }
        }
        let powershell = self.powershell();
        let command = format!(
            "Start-Process netsh -Verb RunAs -Wait -ArgumentList \"interface ipv4 delete address name='Loopback Pseudo-Interface 1' address={DEMO_NODE_IP}\""
        );
        let (_, err) = self.runner.run(
            cancel,
            &powershell,
            &args(&["-NoProfile", "-Command", &command]),
        );
        if let Err(e) = err {
            return Err(command_failed("Windows loopback removal", &e));
        }
        Ok(())
    }
}

#[cfg(test)]
#[allow(clippy::type_complexity)]
pub(crate) mod tests {
    use super::*;
    use crate::runner::RunResult;

    /// `recordingDemoRunner`.
    #[derive(Default)]
    pub struct RecordingRunner {
        pub calls: Vec<Vec<String>>,
        pub unlogged: Vec<Vec<String>>,
        pub outputs: Vec<Option<&'static str>>,
        pub errors: Vec<Option<ExecError>>,
        pub missing: Vec<&'static str>,
    }

    impl RecordingRunner {
        pub fn with_outputs(outputs: Vec<Option<&'static str>>) -> Self {
            RecordingRunner {
                outputs,
                ..Default::default()
            }
        }
    }

    impl CommandRunner for RecordingRunner {
        fn look_path(&mut self, name: &str) -> Result<(), String> {
            if self.missing.contains(&name) {
                Err("executable file not found in $PATH".into())
            } else {
                Ok(())
            }
        }
        fn run(&mut self, _cancel: Cancel, name: &str, args: &[String]) -> RunResult {
            let mut call = vec![name.to_string()];
            call.extend(args.iter().cloned());
            self.calls.push(call);
            let index = self.calls.len() - 1;
            let output = self
                .outputs
                .get(index)
                .copied()
                .flatten()
                .unwrap_or("")
                .as_bytes()
                .to_vec();
            let err = self.errors.get(index).cloned().flatten();
            (
                output,
                match err {
                    Some(e) => Err(e),
                    None => Ok(()),
                },
            )
        }
        fn run_quiet(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult {
            self.run(cancel, name, args)
        }
        fn run_unlogged(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult {
            let mut call = vec![name.to_string()];
            call.extend(args.iter().cloned());
            self.unlogged.push(call);
            self.run(cancel, name, args)
        }
    }

    fn strs(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    fn exit_err() -> Option<ExecError> {
        Some(ExecError {
            exit_code: Some(1),
            message: "exit status 1".into(),
        })
    }

    fn local(runner: RecordingRunner, goos: &str) -> LocalCommands<RecordingRunner> {
        LocalCommands::new(runner, goos)
    }

    // --- demo_destroy_test.go ---

    #[test]
    fn destroy_deletes_registry_when_it_is_not_shared() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![
                Some(r#"[{"name":"local-demo-user"}]"#),
                Some(r#"[{"name":"k3d-qovery-registry.lan"}]"#),
                Some(r#"[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{}}}}]"#),
                None,
                Some(r#"[{"name":"k3d-qovery-registry.lan"}]"#),
                None,
            ]),
            "linux",
        );
        l.delete_k3d_cluster(Cancel::new(), "local-demo-user")
            .unwrap();
        assert_eq!(
            vec![strs(&["k3d", "cluster", "list", "--output", "json"])],
            l.runner.unlogged
        );
        assert_eq!(
            strs(&["k3d", "cluster", "delete", "local-demo-user"]),
            l.runner.calls[3]
        );
        assert_eq!(
            strs(&["k3d", "registry", "delete", "k3d-qovery-registry.lan"]),
            l.runner.calls[5]
        );
    }

    #[test]
    fn destroy_preserves_shared_registry() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![
                Some(r#"[{"name":"local-demo-user"}]"#),
                Some(r#"[{"name":"k3d-qovery-registry.lan"}]"#),
                Some(
                    r#"[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{},"k3d-other-demo":{}}}}]"#,
                ),
                None,
            ]),
            "linux",
        );
        l.delete_k3d_cluster(Cancel::new(), "local-demo-user")
            .unwrap();
        assert_eq!(4, l.runner.calls.len());
        assert_eq!(
            strs(&["k3d", "cluster", "delete", "local-demo-user"]),
            l.runner.calls[3]
        );
    }

    #[test]
    fn destroy_removes_macos_loopback_alias() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![Some("inet 172.42.0.3 netmask 0xffffffff"), None]),
            "darwin",
        );
        l.teardown_loopback(Cancel::new()).unwrap();
        assert_eq!(
            strs(&["sudo", "ifconfig", "lo0", "-alias", DEMO_NODE_IP]),
            l.runner.calls[1]
        );
    }

    // --- demo_up_failure_test.go ---

    #[test]
    fn check_dependencies_lists_every_missing_command_with_install_hints() {
        let mut l = local(
            RecordingRunner {
                missing: vec!["k3d", "helm"],
                ..Default::default()
            },
            "darwin",
        );
        let err = l.check_dependencies(Cancel::new()).unwrap_err();
        assert_eq!(
            "required commands are not installed: k3d, helm\n\
             Install them, then run `qovery demo up` again:\n  \
             brew install k3d helm\n  \
             k3d: https://k3d.io/stable/#installation\n  \
             helm: https://helm.sh/docs/intro/install/",
            err.to_string()
        );
        let (code, safe) = demo_failure_details(&err);
        assert_eq!(ERROR_MISSING_DEPENDENCY, code);
        assert_eq!("Required commands are not installed: k3d, helm.", safe);
        assert!(l.runner.calls.is_empty());
    }

    #[test]
    fn check_dependencies_only_suggests_brew_on_macos() {
        let mut l = local(
            RecordingRunner {
                missing: vec!["kubectl"],
                ..Default::default()
            },
            "linux",
        );
        let err = l.check_dependencies(Cancel::new()).unwrap_err().to_string();
        assert!(!err.contains("brew"));
        assert!(err.contains("kubectl: https://kubernetes.io/docs/tasks/tools/"));
    }

    #[test]
    fn check_dependencies_does_not_suggest_brew_for_docker() {
        let mut l = local(
            RecordingRunner {
                missing: vec!["docker"],
                ..Default::default()
            },
            "darwin",
        );
        let err = l.check_dependencies(Cancel::new()).unwrap_err().to_string();
        assert!(!err.contains("brew"));
        assert!(err.contains("docker: https://docs.docker.com/get-started/get-docker/"));
    }

    #[test]
    fn check_dependencies_reports_docker_not_running() {
        let mut l = local(
            RecordingRunner {
                errors: vec![exit_err()],
                ..Default::default()
            },
            "darwin",
        );
        let err = l.check_dependencies(Cancel::new()).unwrap_err();
        assert_eq!(vec![strs(&["docker", "info"])], l.runner.calls);
        let (code, safe) = demo_failure_details(&err);
        assert_eq!(ERROR_DOCKER_NOT_RUNNING, code);
        assert_eq!("Docker is not running.", safe);
    }

    #[test]
    fn ensure_k3d_cluster_refuses_a_subnet_already_in_use() {
        let cases: [(&'static str, &str); 3] = [
            (
                r#"{"Name":"k3d-local-demo-old","IPAM":{"Config":[{"Subnet":"172.42.0.0/16"}]}}"#,
                "the Docker network \"k3d-local-demo-old\" already uses the demo subnet 172.42.0.0/16\n\
                 k3d created it for the cluster \"local-demo-old\". If it is an old demo cluster, delete it with `k3d cluster delete local-demo-old`. \
                 If that cluster no longer exists, remove the network with `docker network rm k3d-local-demo-old`. Then run `qovery demo up` again.",
            ),
            (
                r#"{"Name":"k3d-x;touch pwned","IPAM":{"Config":[{"Subnet":"172.42.0.0/24"}]}}"#,
                "the Docker network \"k3d-x;touch pwned\" already uses the demo subnet 172.42.0.0/16\n\
                 k3d created it for the cluster \"x;touch pwned\". If it is an old demo cluster, delete it with `k3d cluster delete 'x;touch pwned'`. \
                 If that cluster no longer exists, remove the network with `docker network rm 'k3d-x;touch pwned'`. Then run `qovery demo up` again.",
            ),
            (
                r#"{"Name":"vpn","IPAM":{"Config":[{"Subnet":"172.40.0.0/13"}]}}"#,
                "the Docker network \"vpn\" already uses the demo subnet 172.42.0.0/16\n\
                 Remove this network with `docker network rm vpn`, then run `qovery demo up` again.",
            ),
        ];
        for (network, message) in cases {
            let inspect: &'static str =
                Box::leak(format!(r#"[{{"Name":"bridge","IPAM":{{"Config":[{{"Subnet":"172.17.0.0/16"}}]}}}},{network}]"#).into_boxed_str());
            let mut l = local(
                RecordingRunner::with_outputs(vec![
                    Some("[]"),
                    Some("bridge-id\nother-id\n"),
                    Some(inspect),
                ]),
                "darwin",
            );
            let err = l
                .ensure_k3d_cluster(Cancel::new(), "local-demo-user")
                .unwrap_err();
            assert_eq!(message, err.to_string());
            assert_eq!(ERROR_SUBNET_IN_USE, demo_failure_details(&err).0);
            assert_eq!(3, l.runner.calls.len());
        }
    }

    #[test]
    fn ensure_k3d_cluster_creates_the_cluster_when_the_subnet_check_cannot_decide() {
        let demo_network =
            r#"[{"Name":"k3d-local-demo-user","IPAM":{"Config":[{"Subnet":"172.42.0.0/16"}]}}]"#;
        let cases: Vec<(Vec<Option<&'static str>>, Vec<Option<ExecError>>)> = vec![
            (
                vec![Some("[]"), Some("demo-id\n"), Some(demo_network)],
                vec![],
            ),
            (vec![Some("[]"), None], vec![None, exit_err()]),
            (vec![Some("[]"), Some("")], vec![]),
            (
                vec![Some("[]"), Some("gone-id\n"), None],
                vec![None, None, exit_err()],
            ),
            (
                vec![
                    Some("[]"),
                    Some("bridge-id\n"),
                    Some("Emulate Docker CLI using podman.\n[]"),
                ],
                vec![],
            ),
        ];
        for (mut outputs, errors) in cases {
            let create_index = outputs.len();
            outputs.extend([
                None,
                Some(r#"[{"name":"qovery-registry.lan"}]"#),
                Some(r#"[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["qovery-registry.lan"]}}}}]"#),
            ]);
            let mut l = local(
                RecordingRunner {
                    outputs,
                    errors,
                    ..Default::default()
                },
                "darwin",
            );
            l.ensure_k3d_cluster(Cancel::new(), "local-demo-user")
                .unwrap();
            assert!(l.runner.calls.len() > create_index);
            assert_eq!(
                strs(&["k3d", "cluster", "create", "local-demo-user"]),
                l.runner.calls[create_index][..4].to_vec()
            );
        }
    }

    // --- demo_up_test.go ---

    #[test]
    fn build_demo_helm_args_uses_structured_bootstrap() {
        let args = build_demo_helm_args(
            &test_bootstrap(),
            "/tmp/protected-values.yaml",
            "k3d-local-demo-user",
        )
        .unwrap();
        assert_eq!(
            strs(&[
                "upgrade",
                "--install",
                "--kube-context",
                "k3d-local-demo-user",
                "qovery-operator",
                "oci://registry.example/qovery-operator",
                "--version",
                "1.2.3",
                "--namespace",
                "qovery",
                "--values",
                "/tmp/protected-values.yaml",
                "--create-namespace",
                "--atomic",
                "--wait",
                "--timeout",
                "15m",
            ]),
            args
        );
    }

    #[test]
    fn build_demo_helm_args_rejects_umbrella_chart() {
        let mut bootstrap = test_bootstrap();
        bootstrap.chart_reference = "oci://public.ecr.aws/example/charts/qovery".into();
        let err = build_demo_helm_args(
            &bootstrap,
            "/tmp/protected-values.yaml",
            "k3d-local-demo-user",
        )
        .unwrap_err();
        assert!(err.to_string().contains("umbrella chart"));
    }

    pub fn test_bootstrap() -> Bootstrap {
        Bootstrap {
            release_name: "qovery-operator".into(),
            chart_reference: "oci://registry.example/qovery-operator".into(),
            chart_version: "1.2.3".into(),
            namespace: "qovery".into(),
            values_yaml: "secretToken: highly-sensitive\n".into(),
        }
    }

    /// `inspectingDemoRunner`.
    #[derive(Default)]
    struct InspectingRunner {
        values_path: String,
    }

    impl CommandRunner for InspectingRunner {
        fn look_path(&mut self, _: &str) -> Result<(), String> {
            Ok(())
        }
        fn run(&mut self, _: Cancel, name: &str, args: &[String]) -> RunResult {
            assert_eq!("helm", name);
            let index = args.iter().position(|a| a == "--values").unwrap();
            self.values_path = args[index + 1].clone();
            let meta = std::fs::metadata(&self.values_path).unwrap();
            #[cfg(unix)]
            assert_eq!(
                0o600,
                std::os::unix::fs::PermissionsExt::mode(&meta.permissions()) & 0o777
            );
            assert_eq!(
                test_bootstrap().values_yaml,
                std::fs::read_to_string(&self.values_path).unwrap()
            );
            (Vec::new(), Ok(()))
        }
        fn run_quiet(&mut self, c: Cancel, n: &str, a: &[String]) -> RunResult {
            self.run(c, n, a)
        }
        fn run_unlogged(&mut self, c: Cancel, n: &str, a: &[String]) -> RunResult {
            self.run(c, n, a)
        }
    }

    #[test]
    fn operator_values_file_is_protected_and_removed() {
        let dir = std::env::temp_dir().join(format!("qovery-demo-test-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let mut l = LocalCommands::new(InspectingRunner::default(), "linux");
        l.temp_dir = dir.to_string_lossy().to_string();
        l.install_operator(Cancel::new(), &test_bootstrap())
            .unwrap();
        assert!(!l.runner.values_path.is_empty());
        assert!(!std::path::Path::new(&l.runner.values_path).exists());
        let _ = std::fs::remove_dir_all(dir);
    }

    #[test]
    fn ensure_k3d_cluster_uses_pinned_substrate() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![
                Some("[]"),
                Some("bridge-id\n"),
                Some(r#"[{"Name":"bridge","IPAM":{"Config":[{"Subnet":"172.17.0.0/16"}]}}]"#),
                None,
                Some(r#"[{"name":"qovery-registry.lan"}]"#),
                Some(
                    r#"[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["qovery-registry.lan"]}}}}]"#,
                ),
            ]),
            "linux",
        );
        l.ensure_k3d_cluster(Cancel::new(), "local-demo-user")
            .unwrap();
        assert_eq!(6, l.runner.calls.len());
        assert_eq!(
            vec![
                strs(&["k3d", "cluster", "list", "--output", "json"]),
                strs(&["docker", "network", "inspect", "bridge-id"])
            ],
            l.runner.unlogged
        );
        assert_eq!("k3d", l.runner.calls[3][0]);
        let joined = l.runner.calls[3][1..].join(" ");
        for expected in [
            DEMO_K3S_IMAGE,
            DEMO_SUBNET,
            "--node-ip=172.42.0.3@server:0",
            "--disable=traefik@server:*",
            DEMO_REGISTRY,
            "80:80@loadbalancer",
            "443:443@loadbalancer",
        ] {
            assert!(joined.contains(expected), "{expected}");
        }
    }

    #[test]
    fn ensure_k3d_cluster_starts_existing_cluster() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![
                Some(r#"[{"name":"local-demo-user"}]"#),
                None,
                Some(r#"[{"name":"k3d-qovery-registry.lan"}]"#),
                Some(
                    r#"[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["qovery-registry.lan"]}}}}]"#,
                ),
            ]),
            "linux",
        );
        l.ensure_k3d_cluster(Cancel::new(), "local-demo-user")
            .unwrap();
        assert_eq!(
            strs(&["k3d", "cluster", "start", "local-demo-user"]),
            l.runner.calls[1]
        );
        assert_eq!(
            strs(&["k3d", "registry", "list", "--output", "json"]),
            l.runner.calls[2]
        );
    }

    #[test]
    fn ensure_k3d_cluster_recreates_missing_registry() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![
                Some(r#"[{"name":"local-demo-user"}]"#),
                None,
                Some("[]"),
                None,
                Some(r#"[{"name":"k3d-qovery-registry.lan"}]"#),
                Some(
                    r#"[{"NetworkSettings":{"Networks":{"k3d-local-demo-user":{"DNSNames":["k3d-qovery-registry.lan"]}}}}]"#,
                ),
                None,
                None,
            ]),
            "linux",
        );
        l.ensure_k3d_cluster(Cancel::new(), "local-demo-user")
            .unwrap();
        assert_eq!(
            strs(&[
                "k3d",
                "registry",
                "create",
                DEMO_REGISTRY,
                "--default-network",
                "k3d-local-demo-user",
                "--no-help"
            ]),
            l.runner.calls[3]
        );
        assert_eq!(
            strs(&[
                "docker",
                "network",
                "connect",
                "--alias",
                DEMO_REGISTRY,
                "k3d-local-demo-user",
                "k3d-qovery-registry.lan"
            ]),
            l.runner.calls[7]
        );
    }

    #[test]
    fn validate_workloads_requires_operator_and_rejects_permanent_engine() {
        let mut l = local(
            RecordingRunner::with_outputs(vec![Some(
                r#"{"items":[{"metadata":{"name":"qovery-operator"}}]}"#,
            )]),
            "linux",
        );
        l.kube_context = "k3d-local-demo-user".into();
        l.validate_workloads(Cancel::new(), "qovery").unwrap();
        assert_eq!(
            strs(&[
                "kubectl",
                "--context",
                "k3d-local-demo-user",
                "--namespace",
                "qovery",
                "get",
                "deployments",
                "--output",
                "json"
            ]),
            l.runner.calls[0]
        );

        let mut l = local(
            RecordingRunner::with_outputs(vec![Some(
                r#"{"items":[{"metadata":{"name":"qovery-operator"}},{"metadata":{"name":"qovery-engine"}}]}"#,
            )]),
            "linux",
        );
        let err = l.validate_workloads(Cancel::new(), "qovery").unwrap_err();
        assert!(
            err.to_string()
                .contains("unexpected permanent Deployment qovery-engine")
        );
    }

    #[test]
    fn legacy_release_check_targets_the_demo_context() {
        let mut l = local(RecordingRunner::with_outputs(vec![Some("[]")]), "linux");
        l.kube_context = "k3d-local-demo-user".into();
        assert!(!l.legacy_qovery_release_exists(Cancel::new()).unwrap());
        assert_eq!(
            strs(&[
                "helm",
                "list",
                "--kube-context",
                "k3d-local-demo-user",
                "--namespace",
                "qovery",
                "--deployed",
                "--failed",
                "--pending",
                "--superseded",
                "--uninstalling",
                "--output",
                "json",
                "--filter",
                "^qovery$",
            ]),
            l.runner.calls[0]
        );
    }

    #[test]
    fn loopback_has_address_matches_the_exact_ip() {
        let mac_os = "lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> mtu 16384\n\tinet 127.0.0.1 netmask 0xff000000\n\tinet 172.42.0.30 netmask 0xffffffff\n";
        let linux = "1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536\n    inet 127.0.0.1/8 scope host lo\n    inet 172.42.0.3/32 scope global lo\n";
        assert!(!loopback_has_address(mac_os.as_bytes(), DEMO_NODE_IP));
        assert!(loopback_has_address(
            format!("{mac_os}\tinet 172.42.0.3 netmask 0xffffffff\n").as_bytes(),
            DEMO_NODE_IP
        ));
        assert!(loopback_has_address(linux.as_bytes(), DEMO_NODE_IP));
        assert!(!loopback_has_address(
            b"inet 172.42.0.31/32 scope global lo",
            DEMO_NODE_IP
        ));
    }

    #[test]
    fn prefix_overlaps() {
        let demo = parse_prefix(DEMO_SUBNET).unwrap();
        assert!(overlaps(parse_prefix("172.40.0.0/13").unwrap(), demo));
        assert!(overlaps(parse_prefix("172.42.0.0/24").unwrap(), demo));
        assert!(!overlaps(parse_prefix("172.17.0.0/16").unwrap(), demo));
        assert!(!overlaps(parse_prefix("::/0").unwrap(), demo));
        assert!(overlaps(parse_prefix("0.0.0.0/0").unwrap(), demo));
        assert!(parse_prefix("172.42.0.0").is_none());
    }
}
