//! Port of `cmd/demo_up_orchestrator.go` and of `demoDestroyOrchestrator` (`cmd/demo_destroy.go`).

use crate::api::{Credential, DemoApi, DemoDestroyApi, OperatorStatus};
use crate::failure::*;
use crate::local::{Bootstrap, DEMO_NODE_IP, DemoDestroyLocal, DemoLocal};
use crate::runner::{Cancel, Sink, write_to};
use chrono::{DateTime, TimeDelta, Utc};
use qovery_log::gofmt::{duration_string, quote};
use std::time::Duration;

const DEMO_OPERATOR_HEARTBEAT_FRESHNESS: TimeDelta = TimeDelta::minutes(2);
pub(crate) const DEMO_DEFAULT_OPERATOR_TIMEOUT: Duration = Duration::from_secs(10 * 60);
pub(crate) const DEMO_DEFAULT_DEPLOYMENT_TIMEOUT: Duration = Duration::from_secs(90 * 60);
pub(crate) const DEMO_DEFAULT_POLL_INTERVAL: Duration = Duration::from_secs(5);

pub(crate) const PHASE_LOCAL_DEPENDENCIES: &str = "local_dependencies";
pub(crate) const PHASE_CREDENTIALS_AND_CLUSTER: &str = "credentials_and_cluster";
pub(crate) const PHASE_LOCAL_CLUSTER: &str = "local_cluster";
pub(crate) const PHASE_LEGACY_RELEASE_CHECK: &str = "legacy_release_check";
pub(crate) const PHASE_OPERATOR_BOOTSTRAP: &str = "operator_bootstrap";
pub(crate) const PHASE_OPERATOR_HEARTBEAT: &str = "operator_heartbeat";
pub(crate) const PHASE_PLATFORM_CATALOG_DEPLOYMENT: &str = "platform_catalog_deployment";
pub(crate) const PHASE_WORKLOAD_VERIFICATION: &str = "workload_verification";
pub(crate) const PHASE_RESULT_SUCCEEDED: &str = "succeeded";
pub(crate) const PHASE_RESULT_FAILED: &str = "failed";
pub(crate) const PHASE_RESULT_CANCELED: &str = "canceled";

/// `demoClock`.
pub(crate) trait Clock {
    fn now(&mut self) -> DateTime<Utc>;
    fn sleep(&mut self, cancel: Cancel, duration: Duration) -> Result<(), DemoError>;
}

/// `demoSystemClock`.
pub(crate) struct SystemClock;

impl Clock for SystemClock {
    fn now(&mut self) -> DateTime<Utc> {
        Utc::now()
    }
    fn sleep(&mut self, cancel: Cancel, duration: Duration) -> Result<(), DemoError> {
        let deadline = std::time::Instant::now() + duration;
        loop {
            if let Some(err) = cancel.err() {
                return Err(err);
            }
            let now = std::time::Instant::now();
            if now >= deadline {
                return Ok(());
            }
            std::thread::sleep((deadline - now).min(Duration::from_millis(50)));
        }
    }
}

/// `demoPhaseExecution`.
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct PhaseExecution {
    pub phase: String,
    pub result: String,
    pub duration: Duration,
    pub error_code: String,
    pub safe_error_message: String,
}

/// `demoProgress`.
pub(crate) trait Progress {
    fn cluster_resolved(&mut self, cluster_id: &str);
    fn phase_finished(&mut self, execution: PhaseExecution);
}

/// No progress reporting (`progress: nil`).
impl Progress for () {
    fn cluster_resolved(&mut self, _: &str) {}
    fn phase_finished(&mut self, _: PhaseExecution) {}
}

/// `demoConfig`.
#[derive(Debug, Clone, Default)]
pub(crate) struct Config {
    pub organization_id: String,
    pub cluster_name: String,
    pub cpu_architecture: String,
    pub operator_timeout: Duration,
    pub deployment_timeout: Duration,
    pub poll_interval: Duration,
}

/// `demoOrchestrator`.
pub(crate) struct Orchestrator<A: DemoApi, L: DemoLocal, C: Clock, P: Progress> {
    pub api: A,
    pub local: L,
    pub clock: C,
    pub out: Sink,
    pub progress: P,
}

fn to_std(d: TimeDelta) -> Duration {
    d.to_std().unwrap_or_default()
}

fn from_std(d: Duration) -> TimeDelta {
    TimeDelta::from_std(d).unwrap_or(TimeDelta::MAX)
}

/// `time.Duration.Round(time.Second)` (half away from zero).
fn round_to_second(d: Duration) -> Duration {
    let nanos = d.as_nanos();
    Duration::from_secs(((nanos + 500_000_000) / 1_000_000_000) as u64)
}

/// `operatorStatusReady(status, now)`.
pub(crate) fn operator_status_ready(status: &OperatorStatus, now: DateTime<Utc>) -> bool {
    match (status.connected, status.last_heartbeat) {
        (true, Some(heartbeat)) => now - heartbeat <= DEMO_OPERATOR_HEARTBEAT_FRESHNESS,
        _ => false,
    }
}

/// `isDemoDeploymentInProgress(status)`.
pub(crate) fn is_demo_deployment_in_progress(status: &str) -> bool {
    status.ends_with("QUEUED") || status.ends_with("ING")
}

/// `isDemoDeploymentError(status)`.
pub(crate) fn is_demo_deployment_error(status: &str) -> bool {
    match status {
        "INVALID_CREDENTIALS" | "CANCELED" | "STOPPED" | "DELETED" => true,
        _ => status.ends_with("_ERROR"),
    }
}

impl<A: DemoApi, L: DemoLocal, C: Clock, P: Progress> Orchestrator<A, L, C, P> {
    pub fn up(&mut self, cancel: Cancel, mut cfg: Config) -> Result<(), DemoError> {
        if cfg.operator_timeout.is_zero() {
            cfg.operator_timeout = DEMO_DEFAULT_OPERATOR_TIMEOUT;
        }
        if cfg.deployment_timeout.is_zero() {
            cfg.deployment_timeout = DEMO_DEFAULT_DEPLOYMENT_TIMEOUT;
        }
        if cfg.poll_interval.is_zero() {
            cfg.poll_interval = DEMO_DEFAULT_POLL_INTERVAL;
        }

        self.run_phase(
            cancel,
            PHASE_LOCAL_DEPENDENCIES,
            "Checking local dependencies",
            |o| {
                o.local
                    .check_dependencies(cancel)
                    .map_err(|e| e.prefixed("local dependency check failed"))
            },
        )?;

        let mut cluster_id = String::new();
        let mut credential = Credential::default();
        let mut cluster_found = false;
        self.run_phase(
            cancel,
            PHASE_CREDENTIALS_AND_CLUSTER,
            "Resolving Qovery On-Premise credentials and cluster",
            |o| {
                credential = o
                    .api
                    .ensure_on_premise_credentials(&cfg.organization_id)
                    .map_err(|e| {
                        demo_qovery_api_failure(e.prefixed("cannot resolve On-Premise credentials"))
                    })?;
                match o.api.find_cluster(&cfg.organization_id, &cfg.cluster_name) {
                    Err(e) if e.not_a_demo => {
                        let message =
                            format!("{}; choose another name with --cluster-name", e.message);
                        Err(new_demo_failure(
                            ERROR_CLUSTER_NAME_NOT_A_DEMO,
                            "A Qovery cluster with this name exists and is not a demo cluster.",
                            e.wrap(message),
                        ))
                    }
                    Err(e) => Err(demo_qovery_api_failure(
                        e.prefixed("cannot look up Qovery cluster"),
                    )),
                    Ok((id, found)) => {
                        cluster_id = id;
                        cluster_found = found;
                        if found {
                            o.progress.cluster_resolved(&cluster_id);
                        }
                        Ok(())
                    }
                }
            },
        )?;

        self.run_phase(
            cancel,
            PHASE_LOCAL_CLUSTER,
            "Creating or starting the local k3d cluster",
            |o| {
                if let Err(e) = o.local.ensure_k3d_cluster(cancel, &cfg.cluster_name) {
                    if e.failure.is_some() {
                        return Err(e);
                    }
                    return Err(new_demo_failure(
                        ERROR_K3D_CLUSTER,
                        "The local k3d cluster could not be created or started.",
                        e.prefixed("cannot prepare local k3d cluster"),
                    ));
                }
                if let Err(e) = o.local.ensure_loopback(cancel) {
                    return Err(new_demo_failure(
                        ERROR_LOOPBACK,
                        "The local loopback address could not be configured.",
                        e.prefixed("cannot configure local loopback"),
                    ));
                }
                Ok(())
            },
        )?;

        self.run_phase(cancel, PHASE_LEGACY_RELEASE_CHECK, "Checking for an unsupported legacy Qovery release", |o| {
            match o.local.legacy_qovery_release_exists(cancel) {
                Err(e) => Err(new_demo_failure(
                    ERROR_HELM_LIST,
                    "Listing the local Helm releases failed.",
                    e.prefixed("cannot inspect Helm releases"),
                )),
                Ok(true) => Err(new_demo_failure(
                    ERROR_LEGACY_RELEASE,
                    "A legacy Qovery demo release exists on the local cluster.",
                    DemoError::new("legacy Helm release \"qovery\" exists in namespace \"qovery\"; adopting an old demo is not supported: destroy and recreate the local cluster before running `qovery demo up`"),
                )),
                Ok(false) => Ok(()),
            }
        })?;

        let mut bootstrap = Bootstrap::default();
        self.run_phase(cancel, PHASE_OPERATOR_BOOTSTRAP, "Bootstrapping and attaching the Qovery Operator", |o| {
            if !cluster_found {
                cluster_id = o
                    .api
                    .create_cluster(&cfg.organization_id, &cfg.cluster_name, &credential)
                    .map_err(|e| demo_qovery_api_failure(e.prefixed("cannot create Qovery cluster")))?;
                o.progress.cluster_resolved(&cluster_id);
            }
            o.api
                .configure_operator(&cfg.organization_id, &cluster_id, &cfg.cpu_architecture)
                .map_err(|e| demo_qovery_api_failure(e.prefixed("cannot configure the Qovery Operator for the local demo")))?;
            bootstrap = o
                .api
                .get_operator_bootstrap(&cfg.organization_id, &cluster_id)
                .map_err(|_| demo_qovery_api_failure(DemoError::new("cannot retrieve the Qovery Operator bootstrap")))?;
            o.api
                .attach_operator(&cfg.organization_id, &cluster_id)
                .map_err(|e| demo_qovery_api_failure(e.prefixed("cannot attach the cluster to the Qovery Operator path")))?;
            if o.local.install_operator(cancel, &bootstrap).is_err() {
                return Err(new_demo_failure(
                    ERROR_OPERATOR_INSTALL,
                    "The Qovery Operator Helm installation failed.",
                    DemoError::new("operator Helm installation failed; sensitive bootstrap values were redacted"),
                ));
            }
            Ok(())
        })?;

        self.run_phase(
            cancel,
            PHASE_OPERATOR_HEARTBEAT,
            "Waiting for a fresh Qovery Operator heartbeat",
            |o| o.wait_for_operator(cancel, &cfg, &cluster_id),
        )?;

        let mut status = String::new();
        self.run_phase(cancel, PHASE_PLATFORM_CATALOG_DEPLOYMENT, "Deploying the current self-managed platform catalog", |o| {
            let current_status = o
                .api
                .get_cluster_status(&cfg.organization_id, &cluster_id)
                .map_err(|e| demo_qovery_api_failure(e.prefixed("cannot read cluster deployment status")))?;
            let deadline = o.clock.now() + from_std(cfg.deployment_timeout);
            if is_demo_deployment_in_progress(&current_status) {
                write_to(
                    &o.out,
                    &format!("A cluster deployment is already in progress ({current_status}); waiting for it to finish before deploying.\n"),
                );
                o.wait_until_no_deployment_in_progress(cancel, &cfg, &cluster_id, deadline)?;
            }
            let initial_status = o
                .api
                .deploy_cluster(&cfg.organization_id, &cluster_id)
                .map_err(|e| demo_qovery_api_failure(e.prefixed("cannot trigger cluster deployment")))?;
            status = o.wait_for_deployment(cancel, &cfg, &cluster_id, initial_status, deadline)?;
            Ok(())
        })?;

        self.run_phase(
            cancel,
            PHASE_WORKLOAD_VERIFICATION,
            "Verifying Operator and Engine workloads",
            |o| {
                o.local
                    .validate_workloads(cancel, &bootstrap.namespace)
                    .map_err(|e| {
                        new_demo_failure(
                            ERROR_WORKLOAD_VERIFICATION,
                            "The Operator and Engine workloads are not healthy.",
                            e,
                        )
                    })
            },
        )?;
        write_to(
            &self.out,
            &format!(
                "\nQovery demo cluster is now installed !!!!\nThe kubeconfig is correctly set, so you can connect to it directly with kubectl or k9s from your local machine.\nTo stop or start your cluster, use `k3d cluster stop|start {0}`. To delete it, use `qovery demo destroy -c {0} -d`.\n\nGo to https://console.qovery.com to create your first environment on this cluster {1}.\nCluster deployment finished with status {2}.\n",
                cfg.cluster_name,
                quote(&cfg.cluster_name),
                status
            ),
        );
        Ok(())
    }

    fn run_phase(
        &mut self,
        cancel: Cancel,
        phase: &str,
        message: &str,
        run: impl FnOnce(&mut Self) -> Result<(), DemoError>,
    ) -> Result<(), DemoError> {
        self.phase(message);
        let started_at = self.clock.now();
        let result = match cancel.err() {
            Some(err) => Err(err),
            None => run(self),
        };
        let duration = to_std(self.clock.now() - started_at);
        if result.is_ok() {
            write_to(
                &self.out,
                &format!("✅ Done ({})\n", duration_string(round_to_second(duration))),
            );
        }
        let mut execution = PhaseExecution {
            phase: phase.to_string(),
            result: PHASE_RESULT_SUCCEEDED.to_string(),
            duration,
            ..Default::default()
        };
        if let Err(err) = &result {
            if cancel.is_canceled() {
                execution.result = PHASE_RESULT_CANCELED.to_string();
            } else {
                execution.result = PHASE_RESULT_FAILED.to_string();
                (execution.error_code, execution.safe_error_message) = demo_failure_details(err);
            }
        }
        self.progress.phase_finished(execution);
        result
    }

    fn wait_for_operator(
        &mut self,
        cancel: Cancel,
        cfg: &Config,
        cluster_id: &str,
    ) -> Result<(), DemoError> {
        let deadline = self.clock.now() + from_std(cfg.operator_timeout);
        loop {
            let status = self
                .api
                .get_operator_status(&cfg.organization_id, cluster_id)
                .map_err(|e| {
                    demo_qovery_api_failure(e.prefixed("cannot read Qovery Operator status"))
                })?;
            let now = self.clock.now();
            if operator_status_ready(&status, now) {
                return Ok(());
            }
            if self.clock.now() >= deadline {
                return Err(new_demo_failure(
                    ERROR_OPERATOR_HEARTBEAT_TIMEOUT,
                    "The Qovery Operator did not report a fresh heartbeat in time.",
                    DemoError::new(format!(
                        "timed out after {} waiting for a connected Qovery Operator with a fresh heartbeat",
                        duration_string(cfg.operator_timeout)
                    )),
                ));
            }
            self.clock.sleep(cancel, cfg.poll_interval)?;
        }
    }

    fn wait_for_deployment(
        &mut self,
        cancel: Cancel,
        cfg: &Config,
        cluster_id: &str,
        initial_status: String,
        deadline: DateTime<Utc>,
    ) -> Result<String, DemoError> {
        let mut status = initial_status;
        loop {
            if status == "DEPLOYED" || status == "RESTARTED" {
                return Ok(status);
            }
            if is_demo_deployment_error(&status) {
                return Err(new_demo_failure(
                    ERROR_CLUSTER_DEPLOYMENT,
                    format!("The cluster deployment finished with status {status}."),
                    DemoError::new(format!(
                        "cluster deployment finished with terminal status {status}"
                    )),
                ));
            }
            if self.clock.now() >= deadline {
                return Err(new_demo_failure(
                    ERROR_CLUSTER_DEPLOYMENT_TIMEOUT,
                    "The cluster deployment did not finish in time.",
                    DemoError::new(format!(
                        "timed out after {} waiting for cluster deployment; last status was {status}",
                        duration_string(cfg.deployment_timeout)
                    )),
                ));
            }
            self.clock.sleep(cancel, cfg.poll_interval)?;
            status = self
                .api
                .get_cluster_status(&cfg.organization_id, cluster_id)
                .map_err(|e| {
                    demo_qovery_api_failure(e.prefixed("cannot read cluster deployment status"))
                })?;
        }
    }

    fn wait_until_no_deployment_in_progress(
        &mut self,
        cancel: Cancel,
        cfg: &Config,
        cluster_id: &str,
        deadline: DateTime<Utc>,
    ) -> Result<(), DemoError> {
        loop {
            if self.clock.now() >= deadline {
                return Err(new_demo_failure(
                    ERROR_CLUSTER_DEPLOYMENT_TIMEOUT,
                    "The cluster deployment did not finish in time.",
                    DemoError::new(format!(
                        "timed out after {} waiting for the cluster deployment already in progress",
                        duration_string(cfg.deployment_timeout)
                    )),
                ));
            }
            self.clock.sleep(cancel, cfg.poll_interval)?;
            let status = self
                .api
                .get_cluster_status(&cfg.organization_id, cluster_id)
                .map_err(|e| {
                    demo_qovery_api_failure(e.prefixed("cannot read cluster deployment status"))
                })?;
            if !is_demo_deployment_in_progress(&status) {
                return Ok(());
            }
        }
    }

    fn phase(&self, message: &str) {
        const SEPARATOR: &str = r#""""""""""""""""""""""""""""""""""""""""""""""#;
        write_to(
            &self.out,
            &format!("\n{SEPARATOR}\n{message}\n{SEPARATOR}\n"),
        );
    }
}

// --- destroy ----------------------------------------------------------------------------------------------

/// `demoDestroyConfig`.
#[derive(Debug, Clone, Default)]
pub(crate) struct DestroyConfig {
    pub organization_id: String,
    pub cluster_name: String,
    pub delete_qovery_config: bool,
}

/// `demoDestroyOrchestrator`.
pub(crate) struct DestroyOrchestrator<A: DemoDestroyApi, L: DemoDestroyLocal> {
    pub api: Option<A>,
    pub local: L,
    pub out: Sink,
}

impl<A: DemoDestroyApi, L: DemoDestroyLocal> DestroyOrchestrator<A, L> {
    pub fn destroy(&mut self, cancel: Cancel, cfg: &DestroyConfig) -> Result<(), DemoError> {
        self.phase("Checking local dependencies");
        self.local
            .check_destroy_dependencies(cancel)
            .map_err(|e| e.prefixed("local dependency check failed"))?;

        let (mut cluster_id, mut found) = (String::new(), false);
        let mut lookup_err: Option<DemoError> = None;
        if cfg.delete_qovery_config {
            let Some(api) = self.api.as_mut() else {
                return Err(DemoError::new(
                    "qovery API is required to delete the remote configuration",
                ));
            };
            match api.find_cluster(&cfg.organization_id, &cfg.cluster_name) {
                Ok((id, f)) => (cluster_id, found) = (id, f),
                Err(e) if e.not_a_demo => return Err(e),
                Err(e) => lookup_err = Some(e),
            }
        }

        self.phase("Deleting the local k3d cluster and registry");
        self.local
            .delete_k3d_cluster(cancel, &cfg.cluster_name)
            .map_err(|e| e.prefixed("cannot delete local k3d resources"))?;
        if let Err(e) = self.local.teardown_loopback(cancel) {
            write_to(
                &self.out,
                &format!("Warning: cannot remove the local loopback address {DEMO_NODE_IP}: {e}\n"),
            );
        }

        if !cfg.delete_qovery_config {
            write_to(
                &self.out,
                &format!(
                    "Local demo cluster {} was deleted. Its Qovery configuration and Operator association were preserved.\n",
                    quote(&cfg.cluster_name)
                ),
            );
            return Ok(());
        }

        self.phase("Deleting the Qovery cluster configuration");
        if let Some(e) = lookup_err {
            let message = format!(
                "local demo cluster {} was deleted, but its Qovery cluster cannot be looked up; run the command again to delete it: {}",
                quote(&cfg.cluster_name),
                e.message
            );
            return Err(e.wrap(message));
        }
        if !found {
            write_to(
                &self.out,
                &format!(
                    "Local demo cluster {} was deleted. No Qovery cluster with this name exists in the current organization.\n",
                    quote(&cfg.cluster_name)
                ),
            );
            return Ok(());
        }
        if let Some(api) = self.api.as_mut() {
            api.delete_cluster_config(&cfg.organization_id, &cluster_id)
                .map_err(|e| e.prefixed("cannot delete Qovery cluster configuration"))?;
        }
        write_to(
            &self.out,
            &format!(
                "Demo cluster {} and its Qovery configuration were deleted.\n",
                quote(&cfg.cluster_name)
            ),
        );
        Ok(())
    }

    fn phase(&self, message: &str) {
        write_to(&self.out, &format!("\n==> {message}\n"));
    }
}

#[cfg(test)]
#[allow(clippy::type_complexity)]
mod tests {
    use super::*;
    use crate::local::tests::test_bootstrap;
    use crate::runner::tests::{buffer, text};
    use std::cell::RefCell;
    use std::rc::Rc;

    type Events = Rc<RefCell<Vec<String>>>;

    fn events() -> Events {
        Rc::new(RefCell::new(Vec::new()))
    }

    fn now() -> DateTime<Utc> {
        chrono::TimeZone::with_ymd_and_hms(&Utc, 2026, 8, 19, 10, 0, 0).unwrap()
    }

    fn ready(at: DateTime<Utc>) -> OperatorStatus {
        OperatorStatus {
            connected: true,
            last_heartbeat: Some(at),
        }
    }

    /// `fakeDemoAPI`.
    #[derive(Default)]
    struct FakeApi {
        events: Option<Events>,
        cluster_found: bool,
        find_err: Option<DemoError>,
        bootstrap: Option<Bootstrap>,
        operator_statuses: Vec<OperatorStatus>,
        cluster_statuses: Vec<&'static str>,
        ensure_credentials_calls: usize,
        operator_status_calls: usize,
        cluster_status_calls: usize,
        deploy_calls: usize,
        operator_cpu_architecture: String,
    }

    impl FakeApi {
        fn event(&self, v: &str) {
            if let Some(e) = &self.events {
                e.borrow_mut().push(v.to_string());
            }
        }
    }

    impl DemoApi for FakeApi {
        fn ensure_on_premise_credentials(&mut self, _: &str) -> Result<Credential, DemoError> {
            self.event("credentials");
            self.ensure_credentials_calls += 1;
            Ok(Credential {
                id: "credential-id".into(),
                name: "on-premise".into(),
            })
        }
        fn find_cluster(&mut self, _: &str, _: &str) -> Result<(String, bool), DemoError> {
            self.event("find-cluster");
            match &self.find_err {
                Some(e) => Err(e.clone()),
                None => Ok(("cluster-id".into(), self.cluster_found)),
            }
        }
        fn create_cluster(
            &mut self,
            _: &str,
            _: &str,
            _: &Credential,
        ) -> Result<String, DemoError> {
            self.event("create-cluster");
            Ok("cluster-id".into())
        }
        fn configure_operator(&mut self, _: &str, _: &str, cpu: &str) -> Result<(), DemoError> {
            self.event("operator-config");
            self.operator_cpu_architecture = cpu.to_string();
            Ok(())
        }
        fn get_operator_bootstrap(&mut self, _: &str, _: &str) -> Result<Bootstrap, DemoError> {
            self.event("bootstrap");
            Ok(self.bootstrap.clone().unwrap_or_else(test_bootstrap))
        }
        fn attach_operator(&mut self, _: &str, _: &str) -> Result<(), DemoError> {
            self.event("attach");
            Ok(())
        }
        fn get_operator_status(&mut self, _: &str, _: &str) -> Result<OperatorStatus, DemoError> {
            self.event("operator-status");
            let index = self.operator_status_calls;
            self.operator_status_calls += 1;
            if self.operator_statuses.is_empty() {
                return Ok(OperatorStatus::default());
            }
            Ok(self.operator_statuses[index.min(self.operator_statuses.len() - 1)].clone())
        }
        fn deploy_cluster(&mut self, _: &str, _: &str) -> Result<String, DemoError> {
            self.event("deploy");
            self.deploy_calls += 1;
            Ok("DEPLOYMENT_QUEUED".into())
        }
        fn get_cluster_status(&mut self, _: &str, _: &str) -> Result<String, DemoError> {
            self.event("cluster-status");
            let index = self.cluster_status_calls;
            self.cluster_status_calls += 1;
            if self.cluster_statuses.is_empty() {
                return Ok("DEPLOYED".into());
            }
            Ok(self.cluster_statuses[index.min(self.cluster_statuses.len() - 1)].to_string())
        }
    }

    /// `fakeDemoLocal`.
    #[derive(Default)]
    struct FakeLocal {
        events: Option<Events>,
        k3d_err: Option<DemoError>,
        legacy_release: bool,
        install_err: Option<DemoError>,
    }

    impl FakeLocal {
        fn event(&self, v: &str) {
            if let Some(e) = &self.events {
                e.borrow_mut().push(v.to_string());
            }
        }
    }

    impl DemoLocal for FakeLocal {
        fn check_dependencies(&mut self, _: Cancel) -> Result<(), DemoError> {
            self.event("dependencies");
            Ok(())
        }
        fn ensure_k3d_cluster(&mut self, _: Cancel, _: &str) -> Result<(), DemoError> {
            self.event("k3d");
            self.k3d_err.clone().map_or(Ok(()), Err)
        }
        fn ensure_loopback(&mut self, _: Cancel) -> Result<(), DemoError> {
            self.event("loopback");
            Ok(())
        }
        fn legacy_qovery_release_exists(&mut self, _: Cancel) -> Result<bool, DemoError> {
            self.event("legacy-release");
            Ok(self.legacy_release)
        }
        fn install_operator(&mut self, _: Cancel, _: &Bootstrap) -> Result<(), DemoError> {
            self.event("install-operator");
            self.install_err.clone().map_or(Ok(()), Err)
        }
        fn validate_workloads(&mut self, _: Cancel, _: &str) -> Result<(), DemoError> {
            self.event("validate-workloads");
            Ok(())
        }
    }

    /// `fakeDemoClock` (and `cancelingDemoClock` when `cancel_on_sleep` is set).
    struct FakeClock {
        now: DateTime<Utc>,
        cancel_on_sleep: Option<Cancel>,
    }

    impl Clock for FakeClock {
        fn now(&mut self) -> DateTime<Utc> {
            self.now
        }
        fn sleep(&mut self, cancel: Cancel, duration: Duration) -> Result<(), DemoError> {
            if let Some(c) = self.cancel_on_sleep {
                c.cancel();
            }
            if let Some(err) = cancel.err() {
                return Err(err);
            }
            self.now += from_std(duration);
            Ok(())
        }
    }

    fn clock(now: DateTime<Utc>) -> FakeClock {
        FakeClock {
            now,
            cancel_on_sleep: None,
        }
    }

    /// `recordingDemoProgress`.
    #[derive(Default)]
    struct RecordingProgress {
        cluster_id: String,
        resolved_cluster_ids: Vec<String>,
        phases: Vec<(PhaseExecution, String)>,
    }

    impl Progress for RecordingProgress {
        fn cluster_resolved(&mut self, cluster_id: &str) {
            self.cluster_id = cluster_id.to_string();
            self.resolved_cluster_ids.push(cluster_id.to_string());
        }
        fn phase_finished(&mut self, execution: PhaseExecution) {
            self.phases.push((execution, self.cluster_id.clone()));
        }
    }

    fn test_config() -> Config {
        Config {
            organization_id: "organization-id".into(),
            cluster_name: "local-demo-user".into(),
            cpu_architecture: "ARM64".into(),
            operator_timeout: Duration::from_secs(60),
            deployment_timeout: Duration::from_secs(60),
            poll_interval: Duration::from_secs(1),
        }
    }

    fn orchestrator<P: Progress>(
        api: FakeApi,
        local: FakeLocal,
        clock: FakeClock,
        progress: P,
    ) -> (
        Orchestrator<FakeApi, FakeLocal, FakeClock, P>,
        Rc<RefCell<Vec<u8>>>,
    ) {
        let (out_buffer, out) = buffer();
        (
            Orchestrator {
                api,
                local,
                clock,
                out,
                progress,
            },
            out_buffer,
        )
    }

    fn index_of(values: &[String], target: &str) -> Option<usize> {
        values.iter().position(|v| v == target)
    }

    #[test]
    fn fresh_creation_order() {
        let ev = events();
        let api = FakeApi {
            events: Some(ev.clone()),
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["READY", "DEPLOYED"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(
            api,
            FakeLocal {
                events: Some(ev.clone()),
                ..Default::default()
            },
            clock(now()),
            (),
        );
        o.up(Cancel::new(), test_config()).unwrap();
        assert_eq!(
            vec![
                "dependencies",
                "credentials",
                "find-cluster",
                "k3d",
                "loopback",
                "legacy-release",
                "create-cluster",
                "operator-config",
                "bootstrap",
                "attach",
                "install-operator",
                "operator-status",
                "cluster-status",
                "deploy",
                "cluster-status",
                "validate-workloads",
            ],
            *ev.borrow()
        );
        assert_eq!("ARM64", o.api.operator_cpu_architecture);
    }

    #[test]
    fn rerun_reuses_resources() {
        let ev = events();
        let api = FakeApi {
            events: Some(ev.clone()),
            cluster_found: true,
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["RESTARTED"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(
            api,
            FakeLocal {
                events: Some(ev.clone()),
                ..Default::default()
            },
            clock(now()),
            (),
        );
        o.up(Cancel::new(), test_config()).unwrap();
        let ev = ev.borrow();
        assert!(!ev.contains(&"create-cluster".to_string()));
        assert!(ev.contains(&"install-operator".to_string()));
        assert!(ev.contains(&"deploy".to_string()));
        assert_eq!(1, o.api.ensure_credentials_calls);
    }

    #[test]
    fn reports_every_phase_with_cluster_correlation() {
        let api = FakeApi {
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["DEPLOYED"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(
            api,
            FakeLocal::default(),
            clock(now()),
            RecordingProgress::default(),
        );
        o.up(Cancel::new(), test_config()).unwrap();
        assert_eq!(
            vec!["cluster-id".to_string()],
            o.progress.resolved_cluster_ids
        );
        let names: Vec<&str> = o
            .progress
            .phases
            .iter()
            .map(|(p, _)| p.phase.as_str())
            .collect();
        assert_eq!(
            vec![
                PHASE_LOCAL_DEPENDENCIES,
                PHASE_CREDENTIALS_AND_CLUSTER,
                PHASE_LOCAL_CLUSTER,
                PHASE_LEGACY_RELEASE_CHECK,
                PHASE_OPERATOR_BOOTSTRAP,
                PHASE_OPERATOR_HEARTBEAT,
                PHASE_PLATFORM_CATALOG_DEPLOYMENT,
                PHASE_WORKLOAD_VERIFICATION,
            ],
            names
        );
        for (p, _) in &o.progress.phases {
            assert_eq!(PHASE_RESULT_SUCCEEDED, p.result);
        }
        for (_, id) in &o.progress.phases[..4] {
            assert!(id.is_empty());
        }
        for (_, id) in &o.progress.phases[4..] {
            assert_eq!("cluster-id", id);
        }
    }

    #[test]
    fn reports_the_existing_cluster_from_the_lookup_phase() {
        let api = FakeApi {
            cluster_found: true,
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["DEPLOYED"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(
            api,
            FakeLocal::default(),
            clock(now()),
            RecordingProgress::default(),
        );
        o.up(Cancel::new(), test_config()).unwrap();
        assert!(o.progress.phases[0].1.is_empty());
        for (_, id) in &o.progress.phases[1..] {
            assert_eq!("cluster-id", id);
        }
    }

    #[test]
    fn does_not_create_the_qovery_cluster_when_the_local_setup_fails() {
        for local in [
            FakeLocal {
                k3d_err: Some(demo_subnet_in_use_error("k3d-local-demo-old")),
                ..Default::default()
            },
            FakeLocal {
                legacy_release: true,
                ..Default::default()
            },
        ] {
            let ev = events();
            let local = FakeLocal {
                events: Some(ev.clone()),
                ..local
            };
            let (mut o, _) = orchestrator(
                FakeApi {
                    events: Some(ev.clone()),
                    ..Default::default()
                },
                local,
                clock(now()),
                (),
            );
            assert!(o.up(Cancel::new(), test_config()).is_err());
            assert!(ev.borrow().contains(&"find-cluster".to_string()));
            assert!(!ev.borrow().contains(&"create-cluster".to_string()));
        }
    }

    #[test]
    fn reports_the_failed_phase_and_stops_the_timeline() {
        let (mut o, _) = orchestrator(
            FakeApi::default(),
            FakeLocal {
                legacy_release: true,
                ..Default::default()
            },
            clock(now()),
            RecordingProgress::default(),
        );
        assert!(o.up(Cancel::new(), test_config()).is_err());
        assert_eq!(4, o.progress.phases.len());
        assert_eq!(PHASE_LEGACY_RELEASE_CHECK, o.progress.phases[3].0.phase);
        assert_eq!(PHASE_RESULT_FAILED, o.progress.phases[3].0.result);
        assert!(o.progress.phases[3].1.is_empty());
    }

    #[test]
    fn rejects_legacy_qovery_release() {
        let ev = events();
        let (mut o, _) = orchestrator(
            FakeApi {
                events: Some(ev.clone()),
                ..Default::default()
            },
            FakeLocal {
                events: Some(ev.clone()),
                legacy_release: true,
                ..Default::default()
            },
            clock(Utc::now()),
            (),
        );
        let err = o.up(Cancel::new(), test_config()).unwrap_err();
        assert!(
            err.to_string()
                .contains("adopting an old demo is not supported")
        );
        assert!(!ev.borrow().contains(&"bootstrap".to_string()));
        assert!(!ev.borrow().contains(&"install-operator".to_string()));
    }

    #[test]
    fn times_out_waiting_for_operator() {
        let api = FakeApi {
            operator_statuses: vec![OperatorStatus::default()],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(api, FakeLocal::default(), clock(now()), ());
        let mut cfg = test_config();
        cfg.operator_timeout = Duration::from_secs(2);
        cfg.poll_interval = Duration::from_secs(1);
        let err = o.up(Cancel::new(), cfg).unwrap_err();
        assert!(err.to_string().contains("timed out"));
        assert_eq!(
            "timed out after 2s waiting for a connected Qovery Operator with a fresh heartbeat",
            err.to_string()
        );
        assert_eq!(0, o.api.deploy_calls);
    }

    #[test]
    fn does_not_deploy_before_heartbeat() {
        let ev = events();
        let api = FakeApi {
            events: Some(ev.clone()),
            operator_statuses: vec![
                OperatorStatus {
                    connected: true,
                    last_heartbeat: None,
                },
                ready(now() + TimeDelta::seconds(1)),
            ],
            cluster_statuses: vec!["DEPLOYED"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(
            api,
            FakeLocal {
                events: Some(ev.clone()),
                ..Default::default()
            },
            clock(now()),
            (),
        );
        o.up(Cancel::new(), test_config()).unwrap();
        let ev = ev.borrow();
        let first = index_of(&ev, "operator-status").unwrap();
        let second = index_of(&ev[first + 1..], "operator-status").unwrap() + first + 1;
        let deploy = index_of(&ev, "deploy").unwrap();
        assert!(deploy > second);
    }

    #[test]
    fn deployment_waits_for_a_successful_status() {
        let api = FakeApi {
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["READY", "READY", "DEPLOYING", "DEPLOYED"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(api, FakeLocal::default(), clock(now()), ());
        o.up(Cancel::new(), test_config()).unwrap();
        assert_eq!(4, o.api.cluster_status_calls);
    }

    #[test]
    fn deploys_once_the_deployment_in_progress_settles() {
        let ev = events();
        let api = FakeApi {
            events: Some(ev.clone()),
            cluster_found: true,
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["DEPLOYING", "DEPLOYING", "DEPLOYMENT_ERROR", "DEPLOYED"],
            ..Default::default()
        };
        let (mut o, out) = orchestrator(api, FakeLocal::default(), clock(now()), ());
        o.up(Cancel::new(), test_config()).unwrap();
        let ev = ev.borrow();
        let from = index_of(&ev, "cluster-status").unwrap();
        assert_eq!(
            vec![
                "cluster-status",
                "cluster-status",
                "cluster-status",
                "deploy",
                "cluster-status"
            ],
            ev[from..].to_vec()
        );
        assert!(text(&out).contains("A cluster deployment is already in progress (DEPLOYING); waiting for it to finish before deploying.\n"));
    }

    #[test]
    fn is_deployment_in_progress() {
        for s in [
            "BUILDING",
            "CANCELING",
            "DELETE_QUEUED",
            "DELETING",
            "DEPLOYING",
            "DEPLOYMENT_QUEUED",
            "QUEUED",
            "STOPPING",
            "STOP_QUEUED",
            "RESTART_QUEUED",
            "RESTARTING",
        ] {
            assert!(is_demo_deployment_in_progress(s), "{s}");
        }
        for s in [
            "READY",
            "DEPLOYED",
            "RESTARTED",
            "STOPPED",
            "DELETED",
            "CANCELED",
            "BUILD_ERROR",
            "DEPLOYMENT_ERROR",
            "DELETE_ERROR",
            "STOP_ERROR",
            "RESTART_ERROR",
            "INVALID_CREDENTIALS",
        ] {
            assert!(!is_demo_deployment_in_progress(s), "{s}");
        }
    }

    #[test]
    fn returns_terminal_deployment_error() {
        let api = FakeApi {
            operator_statuses: vec![ready(Utc::now())],
            cluster_statuses: vec!["READY", "DEPLOYING", "DEPLOYMENT_ERROR"],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(api, FakeLocal::default(), clock(Utc::now()), ());
        let err = o.up(Cancel::new(), test_config()).unwrap_err();
        assert!(err.to_string().contains("DEPLOYMENT_ERROR"));
    }

    #[test]
    fn redacts_values_from_errors() {
        let secret = "super-secret-cluster-jwt";
        let api = FakeApi {
            bootstrap: Some(Bootstrap {
                release_name: "qovery-operator".into(),
                chart_reference: "chart".into(),
                chart_version: "1".into(),
                namespace: "qovery".into(),
                values_yaml: format!("token: {secret}"),
            }),
            ..Default::default()
        };
        let local = FakeLocal {
            install_err: Some(DemoError::new(format!(
                "helm failed with values_yaml token: {secret}"
            ))),
            ..Default::default()
        };
        let (mut o, _) = orchestrator(api, local, clock(Utc::now()), ());
        let err = o.up(Cancel::new(), test_config()).unwrap_err().to_string();
        assert!(!err.contains(secret));
        assert!(!err.contains("values_yaml"));
        assert!(err.contains("redacted"));
    }

    #[test]
    fn reports_each_succeeded_phase() {
        let api = FakeApi {
            operator_statuses: vec![ready(now())],
            cluster_statuses: vec!["READY", "DEPLOYED"],
            ..Default::default()
        };
        let (mut o, out) = orchestrator(api, FakeLocal::default(), clock(now()), ());
        o.up(Cancel::new(), test_config()).unwrap();
        let out = text(&out);
        let sep = "\"".repeat(44);
        assert!(out.contains(&format!(
            "Waiting for a fresh Qovery Operator heartbeat\n{sep}\n✅ Done (0s)\n"
        )));
        assert!(out.contains(&format!(
            "Deploying the current self-managed platform catalog\n{sep}\n✅ Done (1s)\n"
        )));
        assert!(out.contains("To stop or start your cluster, use `k3d cluster stop|start local-demo-user`. To delete it, use `qovery demo destroy -c local-demo-user -d`.\n"));
    }

    #[test]
    fn does_not_report_a_failed_phase_as_done() {
        let (mut o, out) = orchestrator(
            FakeApi::default(),
            FakeLocal {
                legacy_release: true,
                ..Default::default()
            },
            clock(Utc::now()),
            (),
        );
        assert!(o.up(Cancel::new(), test_config()).is_err());
        let sep = "\"".repeat(44);
        assert!(text(&out).ends_with(&format!(
            "Checking for an unsupported legacy Qovery release\n{sep}\n"
        )));
    }

    #[test]
    fn suggests_another_name_when_the_cluster_is_not_a_demo() {
        let not_a_demo = crate::api::find_demo_cluster(
            &[serde_json::json!({"id": "prod-id", "name": "production"})],
            "production",
        )
        .unwrap_err();
        let (mut o, _) = orchestrator(
            FakeApi {
                find_err: Some(not_a_demo),
                ..Default::default()
            },
            FakeLocal::default(),
            clock(Utc::now()),
            (),
        );
        let err = o.up(Cancel::new(), test_config()).unwrap_err();
        assert_eq!(
            r#"cluster "production" in this organization is not a demo cluster; qovery demo only manages demo clusters; choose another name with --cluster-name"#,
            err.to_string()
        );
        assert_eq!(0, o.api.deploy_calls);
    }

    #[test]
    fn reports_an_interrupted_phase_as_canceled() {
        let cancel = Cancel::new();
        cancel.cancel();
        let ev = events();
        let api = FakeApi {
            events: Some(ev.clone()),
            operator_statuses: vec![OperatorStatus::default()],
            ..Default::default()
        };
        let (mut o, _) = orchestrator(
            api,
            FakeLocal {
                events: Some(ev.clone()),
                ..Default::default()
            },
            clock(Utc::now()),
            RecordingProgress::default(),
        );
        let err = o.up(cancel, test_config()).unwrap_err();
        assert!(err.canceled);
        assert_eq!(1, o.progress.phases.len());
        assert_eq!(PHASE_LOCAL_DEPENDENCIES, o.progress.phases[0].0.phase);
        assert_eq!(PHASE_RESULT_CANCELED, o.progress.phases[0].0.result);
        assert!(o.progress.phases[0].0.error_code.is_empty());
        assert!(ev.borrow().is_empty());
    }

    #[test]
    fn cancels_the_phase_running_when_interrupted() {
        let cancel = Cancel::new();
        let api = FakeApi {
            operator_statuses: vec![OperatorStatus::default()],
            ..Default::default()
        };
        let c = FakeClock {
            now: Utc::now(),
            cancel_on_sleep: Some(cancel),
        };
        let (mut o, _) = orchestrator(api, FakeLocal::default(), c, RecordingProgress::default());
        let err = o.up(cancel, test_config()).unwrap_err();
        assert!(err.canceled);
        let last = &o.progress.phases.last().unwrap().0;
        assert_eq!(PHASE_OPERATOR_HEARTBEAT, last.phase);
        assert_eq!(PHASE_RESULT_CANCELED, last.result);
        assert!(last.error_code.is_empty());
    }

    #[test]
    fn operator_status_ready_tolerates_clock_skew_but_rejects_stale_heartbeats() {
        let now = chrono::TimeZone::with_ymd_and_hms(&Utc, 2026, 11, 3, 12, 0, 0).unwrap();
        let at = |offset: TimeDelta| ready(now + offset);
        assert!(operator_status_ready(&at(TimeDelta::seconds(90)), now));
        assert!(operator_status_ready(&at(TimeDelta::minutes(-1)), now));
        assert!(!operator_status_ready(&at(TimeDelta::minutes(-3)), now));
        assert!(!operator_status_ready(
            &OperatorStatus {
                connected: false,
                last_heartbeat: Some(now)
            },
            now
        ));
    }

    // --- demo_up_failure_test.go ---

    #[test]
    fn reports_the_error_code_of_the_failed_phase() {
        let not_a_demo = {
            let mut e = DemoError::new(ERR_NOT_A_DEMO_CLUSTER);
            e.not_a_demo = true;
            e
        };
        let cases: Vec<(FakeApi, FakeLocal, &str, &str, String)> = vec![
            (
                FakeApi {
                    find_err: Some(not_a_demo),
                    ..Default::default()
                },
                FakeLocal::default(),
                PHASE_CREDENTIALS_AND_CLUSTER,
                ERROR_CLUSTER_NAME_NOT_A_DEMO,
                "A Qovery cluster with this name exists and is not a demo cluster.".into(),
            ),
            (
                FakeApi {
                    find_err: Some(DemoError::new("502 Bad Gateway")),
                    ..Default::default()
                },
                FakeLocal::default(),
                PHASE_CREDENTIALS_AND_CLUSTER,
                ERROR_QOVERY_API,
                "A Qovery API request failed.".into(),
            ),
            (
                FakeApi::default(),
                FakeLocal {
                    k3d_err: Some(demo_subnet_in_use_error("k3d-local-demo-old")),
                    ..Default::default()
                },
                PHASE_LOCAL_CLUSTER,
                ERROR_SUBNET_IN_USE,
                "Another Docker network already uses the demo subnet 172.42.0.0/16.".into(),
            ),
            (
                FakeApi::default(),
                FakeLocal {
                    legacy_release: true,
                    ..Default::default()
                },
                PHASE_LEGACY_RELEASE_CHECK,
                ERROR_LEGACY_RELEASE,
                "A legacy Qovery demo release exists on the local cluster.".into(),
            ),
            (
                FakeApi::default(),
                FakeLocal {
                    install_err: Some(DemoError::new("helm failed")),
                    ..Default::default()
                },
                PHASE_OPERATOR_BOOTSTRAP,
                ERROR_OPERATOR_INSTALL,
                "The Qovery Operator Helm installation failed.".into(),
            ),
            (
                FakeApi {
                    operator_statuses: vec![OperatorStatus::default()],
                    ..Default::default()
                },
                FakeLocal::default(),
                PHASE_OPERATOR_HEARTBEAT,
                ERROR_OPERATOR_HEARTBEAT_TIMEOUT,
                "The Qovery Operator did not report a fresh heartbeat in time.".into(),
            ),
            (
                FakeApi {
                    operator_statuses: vec![ready(now())],
                    cluster_statuses: vec!["READY", "DEPLOYMENT_ERROR"],
                    ..Default::default()
                },
                FakeLocal::default(),
                PHASE_PLATFORM_CATALOG_DEPLOYMENT,
                ERROR_CLUSTER_DEPLOYMENT,
                "The cluster deployment finished with status DEPLOYMENT_ERROR.".into(),
            ),
        ];
        for (api, local, phase, code, safe) in cases {
            let (mut o, _) = orchestrator(api, local, clock(now()), RecordingProgress::default());
            let mut cfg = test_config();
            cfg.operator_timeout = Duration::from_secs(2);
            cfg.poll_interval = Duration::from_secs(1);
            let err = o.up(Cancel::new(), cfg).unwrap_err();
            let last = &o.progress.phases.last().unwrap().0;
            assert_eq!(
                (phase, PHASE_RESULT_FAILED, code, safe.as_str()),
                (
                    last.phase.as_str(),
                    last.result.as_str(),
                    last.error_code.as_str(),
                    last.safe_error_message.as_str()
                )
            );
            assert_eq!((code.to_string(), safe.clone()), demo_failure_details(&err));
        }
    }

    // --- demo_destroy_test.go ---

    #[derive(Default)]
    struct FakeDestroyApi {
        cluster_found: bool,
        find_err: Option<DemoError>,
        deleted_cluster_id: String,
    }

    impl DemoDestroyApi for FakeDestroyApi {
        fn find_cluster(&mut self, _: &str, _: &str) -> Result<(String, bool), DemoError> {
            match &self.find_err {
                Some(e) => Err(e.clone()),
                None => Ok(("cluster-id".into(), self.cluster_found)),
            }
        }
        fn delete_cluster_config(&mut self, _: &str, cluster_id: &str) -> Result<(), DemoError> {
            self.deleted_cluster_id = cluster_id.to_string();
            Ok(())
        }
    }

    #[derive(Default)]
    struct FakeDestroyLocal {
        cluster_deleted: bool,
        loopback_removed: bool,
        loopback_err: Option<DemoError>,
    }

    impl DemoDestroyLocal for FakeDestroyLocal {
        fn check_destroy_dependencies(&mut self, _: Cancel) -> Result<(), DemoError> {
            Ok(())
        }
        fn delete_k3d_cluster(&mut self, _: Cancel, _: &str) -> Result<(), DemoError> {
            self.cluster_deleted = true;
            Ok(())
        }
        fn teardown_loopback(&mut self, _: Cancel) -> Result<(), DemoError> {
            self.loopback_removed = self.loopback_err.is_none();
            self.loopback_err.clone().map_or(Ok(()), Err)
        }
    }

    fn destroy_cfg(name: &str, delete: bool) -> DestroyConfig {
        DestroyConfig {
            organization_id: "organization-id".into(),
            cluster_name: name.into(),
            delete_qovery_config: delete,
        }
    }

    #[test]
    fn destroy_keeps_qovery_configuration_by_default() {
        let (out, sink) = buffer();
        let mut o = DestroyOrchestrator::<FakeDestroyApi, _> {
            api: None,
            local: FakeDestroyLocal::default(),
            out: sink,
        };
        o.destroy(
            Cancel::new(),
            &DestroyConfig {
                cluster_name: "local-demo-user".into(),
                ..Default::default()
            },
        )
        .unwrap();
        assert!(o.local.cluster_deleted);
        assert!(o.local.loopback_removed);
        assert!(text(&out).contains("Operator association were preserved"));
    }

    #[test]
    fn destroy_deletes_qovery_configuration_when_requested() {
        let (_, sink) = buffer();
        let mut o = DestroyOrchestrator {
            api: Some(FakeDestroyApi {
                cluster_found: true,
                ..Default::default()
            }),
            local: FakeDestroyLocal::default(),
            out: sink,
        };
        o.destroy(Cancel::new(), &destroy_cfg("local-demo-user", true))
            .unwrap();
        assert_eq!("cluster-id", o.api.unwrap().deleted_cluster_id);
    }

    #[test]
    fn destroy_keeps_local_cluster_when_the_qovery_cluster_is_refused() {
        let mut e = DemoError::new(format!(
            "cluster \"production\" in this organization {ERR_NOT_A_DEMO_CLUSTER}"
        ));
        e.not_a_demo = true;
        let (_, sink) = buffer();
        let mut o = DestroyOrchestrator {
            api: Some(FakeDestroyApi {
                find_err: Some(e),
                ..Default::default()
            }),
            local: FakeDestroyLocal::default(),
            out: sink,
        };
        assert!(
            o.destroy(Cancel::new(), &destroy_cfg("production", true))
                .is_err()
        );
        assert!(!o.local.cluster_deleted);
        assert!(!o.local.loopback_removed);
        assert!(o.api.unwrap().deleted_cluster_id.is_empty());
    }

    #[test]
    fn destroy_deletes_local_resources_when_the_qovery_lookup_fails() {
        let (_, sink) = buffer();
        let mut o = DestroyOrchestrator {
            api: Some(FakeDestroyApi {
                find_err: Some(DemoError::new("503 Service Unavailable")),
                ..Default::default()
            }),
            local: FakeDestroyLocal::default(),
            out: sink,
        };
        let err = o
            .destroy(Cancel::new(), &destroy_cfg("local-demo-user", true))
            .unwrap_err();
        assert!(err.to_string().contains("run the command again"));
        assert!(o.local.cluster_deleted);
        assert!(o.local.loopback_removed);
        assert!(o.api.unwrap().deleted_cluster_id.is_empty());
    }

    #[test]
    fn destroy_does_not_claim_a_remote_deletion_when_the_cluster_is_missing() {
        let (out, sink) = buffer();
        let mut o = DestroyOrchestrator {
            api: Some(FakeDestroyApi::default()),
            local: FakeDestroyLocal::default(),
            out: sink,
        };
        o.destroy(Cancel::new(), &destroy_cfg("local-demo-pierr", true))
            .unwrap();
        assert!(o.api.unwrap().deleted_cluster_id.is_empty());
        assert!(
            text(&out)
                .contains("No Qovery cluster with this name exists in the current organization.")
        );
        assert!(!text(&out).contains("Qovery configuration were deleted"));
    }

    #[test]
    fn destroy_deletes_the_qovery_cluster_when_the_loopback_removal_fails() {
        let (out, sink) = buffer();
        let mut o = DestroyOrchestrator {
            api: Some(FakeDestroyApi {
                cluster_found: true,
                ..Default::default()
            }),
            local: FakeDestroyLocal {
                loopback_err: Some(DemoError::new("sudo: a password is required")),
                ..Default::default()
            },
            out: sink,
        };
        o.destroy(Cancel::new(), &destroy_cfg("local-demo-user", true))
            .unwrap();
        assert_eq!("cluster-id", o.api.unwrap().deleted_cluster_id);
        assert!(text(&out).contains("Warning: cannot remove the local loopback address"));
    }
}
