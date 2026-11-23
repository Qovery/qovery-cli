//! Port of `cmd/demo_up_failure.go`: classified demo failures, plus the Go `error` chain semantics the demo
//! commands rely on (`errors.Is` / `errors.As` on wrapped errors).

use qovery_log::gofmt::quote;

pub(crate) const DEMO_UNKNOWN_ERROR_CODE: &str = "UNKNOWN_FAILURE";
pub(crate) const DEMO_SAFE_ERROR_MESSAGE: &str = "The demo installation failed.";

pub(crate) const ERROR_UNSUPPORTED_OS: &str = "UNSUPPORTED_OS";
pub(crate) const ERROR_AUTHENTICATION: &str = "AUTHENTICATION_FAILED";
pub(crate) const ERROR_ORGANIZATION: &str = "ORGANIZATION_NOT_RESOLVED";
pub(crate) const ERROR_INVALID_CLUSTER_NAME: &str = "INVALID_CLUSTER_NAME";
pub(crate) const ERROR_MISSING_DEPENDENCY: &str = "MISSING_DEPENDENCY";
pub(crate) const ERROR_DOCKER_NOT_RUNNING: &str = "DOCKER_NOT_RUNNING";
pub(crate) const ERROR_QOVERY_API: &str = "QOVERY_API_ERROR";
pub(crate) const ERROR_CLUSTER_NAME_NOT_A_DEMO: &str = "CLUSTER_NAME_NOT_A_DEMO";
pub(crate) const ERROR_K3D_CLUSTER: &str = "K3D_CLUSTER_FAILED";
pub(crate) const ERROR_SUBNET_IN_USE: &str = "DEMO_SUBNET_IN_USE";
pub(crate) const ERROR_LOOPBACK: &str = "LOOPBACK_SETUP_FAILED";
pub(crate) const ERROR_DOCKER_DISK_PRESSURE: &str = "DOCKER_DISK_PRESSURE";
pub(crate) const ERROR_HELM_LIST: &str = "HELM_LIST_FAILED";
pub(crate) const ERROR_LEGACY_RELEASE: &str = "LEGACY_RELEASE_EXISTS";
pub(crate) const ERROR_OPERATOR_INSTALL: &str = "OPERATOR_INSTALL_FAILED";
pub(crate) const ERROR_OPERATOR_HEARTBEAT_TIMEOUT: &str = "OPERATOR_HEARTBEAT_TIMEOUT";
pub(crate) const ERROR_CLUSTER_DEPLOYMENT: &str = "CLUSTER_DEPLOYMENT_FAILED";
pub(crate) const ERROR_CLUSTER_DEPLOYMENT_TIMEOUT: &str = "CLUSTER_DEPLOYMENT_TIMEOUT";
pub(crate) const ERROR_WORKLOAD_VERIFICATION: &str = "WORKLOAD_VERIFICATION_FAILED";

/// `errNotADemoCluster`.
pub(crate) const ERR_NOT_A_DEMO_CLUSTER: &str =
    "is not a demo cluster; qovery demo only manages demo clusters";
/// `errDemoInterrupted`.
pub(crate) const ERR_DEMO_INTERRUPTED: &str = "demo installation interrupted";
/// `context.Canceled`.
pub(crate) const CONTEXT_CANCELED: &str = "context canceled";

/// A Go error value as used by the demo commands. Wrapping (`fmt.Errorf("...: %w", err)`) keeps what
/// `errors.Is` / `errors.As` can find in the chain.
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct DemoError {
    pub message: String,
    /// The outermost `*demoFailure` of the chain: (code, safe message).
    pub failure: Option<(String, String)>,
    /// `errors.Is(err, errNotADemoCluster)`
    pub not_a_demo: bool,
    /// `errors.Is(err, errDemoInterrupted)`
    pub interrupted: bool,
    /// `errors.Is(err, context.Canceled)`
    pub canceled: bool,
    /// HTTP status of the API response that caused the error, if any.
    pub api_status: Option<u16>,
    /// Body of that API response (`GenericOpenAPIError.Body()`).
    pub api_body: String,
}

impl std::fmt::Display for DemoError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl DemoError {
    /// `errors.New(message)`.
    pub fn new(message: impl Into<String>) -> Self {
        DemoError {
            message: message.into(),
            ..Default::default()
        }
    }

    /// `fmt.Errorf(format, ..., err)` with `%w`: the new message, the same chain.
    pub fn wrap(mut self, message: impl Into<String>) -> Self {
        self.message = message.into();
        self
    }

    /// `fmt.Errorf("<prefix>: %w", err)`.
    pub fn prefixed(self, prefix: &str) -> Self {
        let message = format!("{prefix}: {}", self.message);
        self.wrap(message)
    }

    /// `context.Canceled`.
    pub fn canceled() -> Self {
        DemoError {
            message: CONTEXT_CANCELED.to_string(),
            canceled: true,
            ..Default::default()
        }
    }
}

/// `newDemoFailure(code, safeMessage, err)`.
pub(crate) fn new_demo_failure(
    code: &str,
    safe_message: impl Into<String>,
    mut err: DemoError,
) -> DemoError {
    err.failure = Some((code.to_string(), safe_message.into()));
    err
}

/// `demoQoveryAPIFailure(err)`.
pub(crate) fn demo_qovery_api_failure(err: DemoError) -> DemoError {
    new_demo_failure(ERROR_QOVERY_API, "A Qovery API request failed.", err)
}

/// `demoFailureDetails(err)`.
pub(crate) fn demo_failure_details(err: &DemoError) -> (String, String) {
    match &err.failure {
        Some((code, safe)) => (code.clone(), safe.clone()),
        None => (
            DEMO_UNKNOWN_ERROR_CODE.to_string(),
            DEMO_SAFE_ERROR_MESSAGE.to_string(),
        ),
    }
}

/// `demoSubnetInUseError(networkName)`.
/// `demoDockerDiskPressureError(nodeName, waited)`.
pub(crate) fn demo_docker_disk_pressure_error(
    node_name: &str,
    waited: std::time::Duration,
) -> DemoError {
    new_demo_failure(
        ERROR_DOCKER_DISK_PRESSURE,
        "Docker is running out of disk space: Kubernetes cannot start new pods.",
        DemoError::new(format!(
            "node {node_name} of the local cluster is still under disk pressure ({}) after {}: Docker is running out of disk space. Free space with `docker builder prune -f` and `docker image prune -a -f`, or increase the disk size in Docker Desktop (Settings > Resources), then run `qovery demo up` again",
            crate::diagnostics::DEMO_DISK_PRESSURE_TAINT,
            qovery_log::gofmt::duration_string(waited)
        )),
    )
}

pub(crate) fn demo_subnet_in_use_error(network_name: &str) -> DemoError {
    let mut hint = format!(
        "Remove this network with `docker network rm {}`, then run `qovery demo up` again.",
        demo_shell_quote(network_name)
    );
    if let Some(cluster_name) = network_name.strip_prefix("k3d-") {
        hint = format!(
            "k3d created it for the cluster {}. If it is an old demo cluster, delete it with `k3d cluster delete {}`. If that cluster no longer exists, remove the network with `docker network rm {}`. Then run `qovery demo up` again.",
            quote(cluster_name),
            demo_shell_quote(cluster_name),
            demo_shell_quote(network_name),
        );
    }
    new_demo_failure(
        ERROR_SUBNET_IN_USE,
        format!(
            "Another Docker network already uses the demo subnet {}.",
            crate::local::DEMO_SUBNET
        ),
        DemoError::new(format!(
            "the Docker network {} already uses the demo subnet {}\n{}",
            quote(network_name),
            crate::local::DEMO_SUBNET,
            hint
        )),
    )
}

/// `demoShellQuote(word)`: `^[A-Za-z0-9_.-]+$` words are kept, others single-quoted.
pub(crate) fn demo_shell_quote(word: &str) -> String {
    let safe = !word.is_empty()
        && word
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || matches!(b, b'_' | b'.' | b'-'));
    if safe {
        return word.to_string();
    }
    format!("'{}'", word.replace('\'', r"'\''"))
}

fn demo_dependency_install_doc(dependency: &str) -> &'static str {
    match dependency {
        "docker" => "https://docs.docker.com/get-started/get-docker/",
        "k3d" => "https://k3d.io/stable/#installation",
        "helm" => "https://helm.sh/docs/intro/install/",
        "kubectl" => "https://kubernetes.io/docs/tasks/tools/",
        _ => "",
    }
}

/// `missingDemoDependenciesError(goos, missing)`.
pub(crate) fn missing_demo_dependencies_error(goos: &str, missing: &[String]) -> DemoError {
    let mut message = format!(
        "required commands are not installed: {}\nInstall them, then run `qovery demo up` again:",
        missing.join(", ")
    );
    let brew_installable: Vec<&str> = missing
        .iter()
        .map(|s| s.as_str())
        .filter(|d| *d != "docker")
        .collect();
    if goos == "darwin" && !brew_installable.is_empty() {
        message.push_str(&format!("\n  brew install {}", brew_installable.join(" ")));
    }
    for dependency in missing {
        message.push_str(&format!(
            "\n  {}: {}",
            dependency,
            demo_dependency_install_doc(dependency)
        ));
    }
    new_demo_failure(
        ERROR_MISSING_DEPENDENCY,
        format!(
            "Required commands are not installed: {}.",
            missing.join(", ")
        ),
        DemoError::new(message),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn demo_failure_details_falls_back_to_unknown_for_unclassified_errors() {
        let (code, safe_message) = demo_failure_details(&DemoError::new("boom"));
        assert_eq!(DEMO_UNKNOWN_ERROR_CODE, code);
        assert_eq!(DEMO_SAFE_ERROR_MESSAGE, safe_message);
    }

    #[test]
    fn demo_shell_quote_cases() {
        assert_eq!("k3d-local-demo_1.2", demo_shell_quote("k3d-local-demo_1.2"));
        assert_eq!("'a b'", demo_shell_quote("a b"));
        assert_eq!("'$(id)'", demo_shell_quote("$(id)"));
        assert_eq!(r"'it'\''s'", demo_shell_quote("it's"));
        assert_eq!("''", demo_shell_quote(""));
    }
}
