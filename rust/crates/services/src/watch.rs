//! Watching deployments: `utils.WatchEnvironment*`, the state helpers of `utils/qovery.go`, `utils/watch.go`
//! (`ServicesWatch`) and the id helpers of `cmd/watch.go`.

use crate::jobs::get_job_id;
use crate::util::path_escape;
use qovery_api::Client;
use qovery_api::models::{
    Application, Container, Database, EnvironmentStatuses, Helm, Job, Status, Terraform,
};
use qovery_core::status::status_text_with_color;
use qovery_log::gofmt::{duration_string, parse_duration};
use serde::{Deserialize, Deserializer};
use serde_json::Value;
use std::collections::HashMap;
use std::time::{Duration, Instant};

/// Go `STATEENUM_*` values used by the helpers.
pub mod state {
    pub const DEPLOYED: &str = "DEPLOYED";
    pub const RESTARTED: &str = "RESTARTED";
    pub const DELETED: &str = "DELETED";
    pub const STOPPED: &str = "STOPPED";
    pub const CANCELED: &str = "CANCELED";
    pub const READY: &str = "READY";
    pub const INVALID_CREDENTIALS: &str = "INVALID_CREDENTIALS";
}

const POLL_INTERVAL: Duration = Duration::from_secs(3);

/// Port of `utils.WatchEnvironment(envId, finalServiceState, client)`.
pub fn watch_environment(env_id: &str, final_service_state: &str, client: &Client) {
    watch_environment_with_options(env_id, final_service_state, client, false)
}

/// Port of `utils.WatchEnvironmentWithOptions(envId, finalServiceState, client, displaySimpleText)`: polls
/// `GET /environment/{id}/statuses` every 3 seconds and logs (logrus info)
/// `<last deployment state> (<n>/<total> services <⏳|✅> )` (or only the state with `display_simple_text`)
/// until the last deployment state is DEPLOYED / RESTARTED / DELETED / STOPPED / CANCELED. Exits 1 when it
/// ends with `ERROR`; returns silently on API error.
pub fn watch_environment_with_options(
    env_id: &str,
    final_service_state: &str,
    client: &Client,
    display_simple_text: bool,
) {
    loop {
        let Ok(statuses) = client
            .get::<EnvironmentStatuses>(&format!("/environment/{}/statuses", path_escape(env_id)))
        else {
            return;
        };
        let last = statuses
            .environment
            .last_deployment_state
            .clone()
            .unwrap_or_default();

        if display_simple_text {
            qovery_log::logrus::info(status_text_with_color(&last));
        } else {
            let count = count_status(&statuses.applications, final_service_state)
                + count_status(&statuses.databases, final_service_state)
                + count_status(&statuses.jobs, final_service_state)
                + count_status(&statuses.containers, final_service_state)
                + count_status(&statuses.helms, final_service_state)
                + count_status(&statuses.terraforms, final_service_state);
            let total = statuses.applications.len()
                + statuses.databases.len()
                + statuses.jobs.len()
                + statuses.containers.len()
                + statuses.helms.len()
                + statuses.terraforms.len();
            let icon = if count > 0 { "✅" } else { "⏳" };
            qovery_log::logrus::info(format!(
                "{} ({count}/{total} services {icon} )",
                status_text_with_color(&last)
            ));
        }

        if matches!(
            last.as_str(),
            state::DEPLOYED | state::RESTARTED | state::DELETED | state::STOPPED | state::CANCELED
        ) {
            return;
        }
        if last.ends_with("ERROR") {
            qovery_core::exit(1);
        }
        std::thread::sleep(POLL_INTERVAL);
    }
}

/// Port of `isFinalState(state)`: DEPLOYED, DELETED, STOPPED, CANCELED or RESTARTED.
pub fn is_final_state(state: &str) -> bool {
    matches!(
        state,
        state::DEPLOYED | state::DELETED | state::STOPPED | state::CANCELED | state::RESTARTED
    )
}

/// Port of `isErrorState(state)`: ends with `ERROR`.
pub fn is_error_state(state: &str) -> bool {
    state.ends_with("ERROR")
}

/// Port of `countStatus(statuses, state)`.
pub fn count_status(statuses: &[Status], state: &str) -> usize {
    statuses.iter().filter(|s| s.state == state).count()
}

/// Port of `utils.IsTerminalState(state)`: final states, READY and `*ERROR`.
pub fn is_terminal_state(state: &str) -> bool {
    matches!(
        state,
        state::DEPLOYED
            | state::DELETED
            | state::STOPPED
            | state::CANCELED
            | state::READY
            | state::RESTARTED
    ) || state.ends_with("ERROR")
}

/// Port of `utils.IsTerminalClusterState(state)`: like [`is_terminal_state`] plus INVALID_CREDENTIALS.
pub fn is_terminal_cluster_state(state: &str) -> bool {
    is_terminal_state(state) || state == state::INVALID_CREDENTIALS
}

// ---- utils/watch.go ---------------------------------------------------------------------------------------

/// `maxConsecutiveStatusErrors`: about 15 seconds of retries with the 3 seconds poll interval.
pub const MAX_CONSECUTIVE_STATUS_ERRORS: usize = 5;

/// `defaultWatchTimeout`: one hour, overridable with `QOVERY_CLI_WATCH_TIMEOUT` (a Go duration).
pub const DEFAULT_WATCH_TIMEOUT: Duration = Duration::from_secs(3600);

/// Port of `utils.Status` (`Continue`, `Stop`, `Err`).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WatchStatus {
    Continue,
    Stop,
    Err,
}

fn null_as_default<'de, D: Deserializer<'de>, T: Default + Deserialize<'de>>(
    d: D,
) -> Result<T, D::Error> {
    Ok(Option::<T>::deserialize(d)?.unwrap_or_default())
}

/// `qovery.DeploymentStageWithServicesStatuses`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct DeploymentStageWithServicesStatuses {
    #[serde(default, deserialize_with = "null_as_default")]
    pub applications: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub containers: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub jobs: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub databases: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub helms: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub terraforms: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub agentic_workflows: Vec<Status>,
    #[serde(default)]
    pub stage: Option<Value>,
}

/// `qovery.EnvironmentStatusesWithStages` (`GET /environment/{id}/statusesWithStages`).
#[derive(Debug, Clone, Default, Deserialize)]
pub struct EnvironmentStatusesWithStages {
    #[serde(default)]
    pub environment: Option<Value>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub stages: Vec<DeploymentStageWithServicesStatuses>,
    #[serde(default)]
    pub pre_check_stage: Option<Value>,
}

impl EnvironmentStatusesWithStages {
    /// Port of `allStatuses(statuses)`: every service status of every stage.
    pub fn all_statuses(&self) -> Vec<&Status> {
        let mut all = Vec::new();
        for stage in &self.stages {
            for list in [
                &stage.applications,
                &stage.containers,
                &stage.databases,
                &stage.jobs,
                &stage.helms,
                &stage.terraforms,
                &stage.agentic_workflows,
            ] {
                all.extend(list.iter());
            }
        }
        all
    }

    /// Port of `executionIds(statuses)`: execution id (`""` when none) of each service.
    pub fn execution_ids(&self) -> HashMap<String, String> {
        self.all_statuses()
            .into_iter()
            .map(|s| (s.id.clone(), s.execution_id.clone().unwrap_or_default()))
            .collect()
    }
}

/// Port of `utils.ServicesWatch`: follows the services targeted by one request until each of them is done
/// with it. A disabled watch (Go nil) does nothing on [`ServicesWatch::wait`].
///
/// It reads `/statusesWithStages` rather than `/statuses`: only the former reports a service as `*_QUEUED`
/// while a request for it waits in the environment queue.
#[derive(Clone)]
pub struct ServicesWatch {
    inner: Option<WatchInner>,
}

#[derive(Clone)]
struct WatchInner {
    client: Client,
    env_id: String,
    service_ids: Vec<String>,
    before: HashMap<String, String>,
}

/// Port of `utils.NewServicesWatch(client, envId, serviceIds, enabled)`: must be called **before** the request
/// is sent, it records the current execution of each service. When the statuses cannot be read
/// (5 attempts, 3 seconds apart) it prints
/// `Error: cannot watch the services, no request was sent: ...` on stderr and exits 1.
/// Returns a disabled watch when `enabled` is false.
pub fn new_services_watch(
    client: &Client,
    env_id: &str,
    service_ids: Vec<String>,
    enabled: bool,
) -> ServicesWatch {
    if !enabled {
        return ServicesWatch { inner: None };
    }
    let mut fetch = || fetch_statuses_with_stages(client, env_id);
    match snapshot_executions(&mut fetch, &mut || std::thread::sleep(POLL_INTERVAL)) {
        Ok(before) => ServicesWatch {
            inner: Some(WatchInner {
                client: client.clone(),
                env_id: env_id.to_string(),
                service_ids,
                before,
            }),
        },
        Err(e) => {
            qovery_log::println_error_to_stderr(format!(
                "cannot watch the services, no request was sent: {e}"
            ));
            qovery_core::exit(1)
        }
    }
}

impl ServicesWatch {
    /// A watch that does nothing (Go `nil` `*ServicesWatch`).
    pub fn disabled() -> Self {
        ServicesWatch { inner: None }
    }

    /// Whether the watch is enabled.
    pub fn is_enabled(&self) -> bool {
        self.inner.is_some()
    }

    /// Port of `(*ServicesWatch).Wait(finalServiceState)`: blocks until every service reached
    /// `final_service_state` for this request, logging `<done>/<total> services <STATE> <⏳|✅>` at each poll,
    /// and exits 1 if one of them fails, is canceled, ends in another state, disappears or if the watch
    /// times out (`QOVERY_CLI_WATCH_TIMEOUT`, default 1h).
    pub fn wait(&self, final_service_state: &str) {
        let Some(w) = &self.inner else {
            return;
        };
        let tracker = ServicesTracker::new(w.service_ids.clone(), w.before.clone());
        let deadline = Instant::now() + watch_timeout();
        let mut fetch = || fetch_statuses_with_stages(&w.client, &w.env_id);
        let status = watch_services(
            &mut fetch,
            &mut || std::thread::sleep(POLL_INTERVAL),
            &mut || Instant::now() > deadline,
            &tracker,
            final_service_state,
        );
        if status == WatchStatus::Err {
            qovery_core::exit(1);
        }
    }
}

/// `(*ServicesWatch).fetch`: `GET /environment/{id}/statusesWithStages`.
pub fn fetch_statuses_with_stages(
    client: &Client,
    env_id: &str,
) -> Result<EnvironmentStatusesWithStages, String> {
    client
        .get(&format!(
            "/environment/{}/statusesWithStages",
            path_escape(env_id)
        ))
        .map_err(|e| e.to_string())
}

/// Port of `snapshotExecutions(fetch, sleep)`.
pub fn snapshot_executions(
    fetch: &mut dyn FnMut() -> Result<EnvironmentStatusesWithStages, String>,
    sleep: &mut dyn FnMut(),
) -> Result<HashMap<String, String>, String> {
    let mut err = String::new();
    for attempt in 1..=MAX_CONSECUTIVE_STATUS_ERRORS {
        match fetch() {
            Ok(statuses) => return Ok(statuses.execution_ids()),
            Err(e) => err = e,
        }
        if attempt < MAX_CONSECUTIVE_STATUS_ERRORS {
            sleep();
        }
    }
    Err(format!(
        "cannot get the services status after {MAX_CONSECUTIVE_STATUS_ERRORS} attempts: {err}"
    ))
}

/// Port of `watchTimeout()`: `QOVERY_CLI_WATCH_TIMEOUT` or one hour.
pub fn watch_timeout() -> Duration {
    watch_timeout_from(&std::env::var("QOVERY_CLI_WATCH_TIMEOUT").unwrap_or_default())
}

/// [`watch_timeout`] for a given value: prints `Error: invalid QOVERY_CLI_WATCH_TIMEOUT "<v>", using 1h0m0s`
/// on stderr and uses the default when the value is not a positive Go duration.
pub fn watch_timeout_from(value: &str) -> Duration {
    if value.is_empty() {
        return DEFAULT_WATCH_TIMEOUT;
    }
    match parse_duration(value) {
        Ok(d) if !d.is_zero() => d,
        _ => {
            qovery_log::println_error_to_stderr(format!(
                "invalid QOVERY_CLI_WATCH_TIMEOUT {}, using {}",
                qovery_log::gofmt::quote(value),
                duration_string(DEFAULT_WATCH_TIMEOUT)
            ));
            DEFAULT_WATCH_TIMEOUT
        }
    }
}

/// Port of `watchServices(fetch, sleep, expired, tracker, finalServiceState)`.
pub fn watch_services(
    fetch: &mut dyn FnMut() -> Result<EnvironmentStatusesWithStages, String>,
    sleep: &mut dyn FnMut(),
    expired: &mut dyn FnMut() -> bool,
    tracker: &ServicesTracker,
    final_service_state: &str,
) -> WatchStatus {
    let mut consecutive_errors = 0;
    loop {
        if expired() {
            qovery_log::println_error_to_stderr(format!(
                "the services did not reach {final_service_state} before the watch timeout, set QOVERY_CLI_WATCH_TIMEOUT to wait longer"
            ));
            return WatchStatus::Err;
        }

        let statuses = match fetch() {
            Ok(s) => s,
            // a transient API error must not end the watch as a success
            Err(e) => {
                consecutive_errors += 1;
                if consecutive_errors >= MAX_CONSECUTIVE_STATUS_ERRORS {
                    qovery_log::println_error_to_stderr(format!(
                        "cannot get the services status after {consecutive_errors} attempts: {e}"
                    ));
                    return WatchStatus::Err;
                }
                sleep();
                continue;
            }
        };
        consecutive_errors = 0;

        let (status, done, err) = tracker.update(&statuses, final_service_state);
        let icon = if status == WatchStatus::Stop {
            "✅"
        } else {
            "⏳"
        };
        qovery_log::logrus::info(format!(
            "{done}/{} services {} {icon}",
            tracker.service_ids.len(),
            status_text_with_color(final_service_state)
        ));
        if let Some(e) = err {
            qovery_log::println_error_to_stderr(e);
        }
        if status != WatchStatus::Continue {
            return status;
        }
        sleep();
    }
}

/// Port of `servicesTracker`.
#[derive(Debug, Clone)]
pub struct ServicesTracker {
    pub service_ids: Vec<String>,
    /// execution id of each service when the watch started
    pub before: HashMap<String, String>,
}

impl ServicesTracker {
    /// Port of `newServicesTracker(serviceIds, before)`.
    pub fn new(service_ids: Vec<String>, before: HashMap<String, String>) -> Self {
        ServicesTracker {
            service_ids,
            before,
        }
    }

    /// Port of `(*servicesTracker).update`: `Err` with the reason as soon as one service failed, was canceled,
    /// ended in another state or disappeared, `Stop` once they all reached `final_service_state`, and how
    /// many did. A state only counts once it comes from a new execution.
    pub fn update(
        &self,
        statuses: &EnvironmentStatusesWithStages,
        final_service_state: &str,
    ) -> (WatchStatus, usize, Option<String>) {
        let by_id: HashMap<&str, &Status> = statuses
            .all_statuses()
            .into_iter()
            .map(|s| (s.id.as_str(), s))
            .collect();

        let mut done = 0;
        for id in &self.service_ids {
            let Some(s) = by_id.get(id.as_str()) else {
                if !self.before.contains_key(id) {
                    return (
                        WatchStatus::Err,
                        done,
                        Some(format!(
                            "service {id} was not in the environment when the watch started"
                        )),
                    );
                }
                // a deleted service disappears from the environment statuses
                if final_service_state == state::DELETED {
                    done += 1;
                    continue;
                }
                return (
                    WatchStatus::Err,
                    done,
                    Some(format!("service {id} no longer exists in the environment")),
                );
            };

            // queued, in progress, or still the state of a previous execution
            let execution_id = s.execution_id.as_deref().unwrap_or("");
            let before = self.before.get(id).map(|s| s.as_str()).unwrap_or("");
            if execution_id == before || (!is_final_state(&s.state) && !is_error_state(&s.state)) {
                continue;
            }

            if s.state == final_service_state {
                done += 1;
            } else if is_error_state(&s.state) || s.state == state::CANCELED {
                return (
                    WatchStatus::Err,
                    done,
                    Some(format!("service {id} is in state {}", s.state)),
                );
            } else {
                return (
                    WatchStatus::Err,
                    done,
                    Some(format!(
                        "service {id} ended in state {} instead of {final_service_state}",
                        s.state
                    )),
                );
            }
        }

        if done == self.service_ids.len() {
            return (WatchStatus::Stop, done, None);
        }
        (WatchStatus::Continue, done, None)
    }
}

// ---- cmd/watch.go -----------------------------------------------------------------------------------------

/// Port of `applicationIds(applications)` (`cmd/watch.go`): ids for [`new_services_watch`].
pub fn application_ids(applications: &[Application]) -> Vec<String> {
    applications.iter().map(|a| a.id.clone()).collect()
}

/// Port of `containerIds(containers)` (`cmd/watch.go`).
pub fn container_ids(containers: &[Container]) -> Vec<String> {
    containers.iter().map(|c| c.id.clone()).collect()
}

/// Port of `databaseIds(databases)` (`cmd/watch.go`).
pub fn database_ids(databases: &[Database]) -> Vec<String> {
    databases.iter().map(|d| d.id.clone()).collect()
}

/// Port of `jobIds(jobs)` (`cmd/watch.go`): `utils.GetJobId` of each job.
pub fn job_ids(jobs: &[Job]) -> Vec<String> {
    jobs.iter().map(get_job_id).collect()
}

/// Port of `helmIds(helms)` (`cmd/watch.go`).
pub fn helm_ids(helms: &[Helm]) -> Vec<String> {
    helms.iter().map(|h| h.id.clone()).collect()
}

/// Port of `terraformIds(terraforms)` (`cmd/watch.go`).
pub fn terraform_ids(terraforms: &[Terraform]) -> Vec<String> {
    terraforms.iter().map(|t| t.id.clone()).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::Cell;

    /// One service status of a trace: (state, execution id).
    type Poll = Vec<(&'static str, &'static str, &'static str)>;

    #[derive(Deserialize)]
    struct WatchTrace {
        #[allow(dead_code, reason = "field of the shared Go test trace format")]
        description: String,
        final_state: String,
        service_ids: Vec<String>,
        before: HashMap<String, String>,
        polls: Vec<TracePoll>,
    }

    #[derive(Deserialize)]
    struct TracePoll {
        t: f64,
        with_stages: HashMap<String, TraceServiceStatus>,
    }

    #[derive(Deserialize, Clone)]
    struct TraceServiceStatus {
        state: String,
        execution_id: String,
    }

    fn load_trace(name: &str) -> WatchTrace {
        let path = format!("{}/testdata/watch/{name}.json", env!("CARGO_MANIFEST_DIR"));
        serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap()
    }

    fn status(id: &str, state: &str, execution_id: &str) -> Status {
        Status {
            id: id.into(),
            state: state.into(),
            execution_id: Some(execution_id.into()),
            ..Default::default()
        }
    }

    fn with_stages(
        containers: &HashMap<String, TraceServiceStatus>,
    ) -> EnvironmentStatusesWithStages {
        let list = containers
            .iter()
            .map(|(id, s)| status(id, &s.state, &s.execution_id))
            .collect();
        EnvironmentStatusesWithStages {
            stages: vec![DeploymentStageWithServicesStatuses {
                containers: list,
                ..Default::default()
            }],
            ..Default::default()
        }
    }

    fn poll(p: &Poll) -> EnvironmentStatusesWithStages {
        let list = p.iter().map(|(id, st, ex)| status(id, st, ex)).collect();
        EnvironmentStatusesWithStages {
            stages: vec![DeploymentStageWithServicesStatuses {
                containers: list,
                ..Default::default()
            }],
            ..Default::default()
        }
    }

    fn before(pairs: &[(&str, &str)]) -> HashMap<String, String> {
        pairs
            .iter()
            .map(|(a, b)| (a.to_string(), b.to_string()))
            .collect()
    }

    #[test]
    fn services_tracker_on_real_traces() {
        let svc: Vec<String> = ["svc-06", "svc-07", "svc-08", "svc-09", "svc-10"]
            .iter()
            .map(|s| s.to_string())
            .collect();
        let tests: Vec<(&str, &str, Option<Vec<String>>, WatchStatus)> = vec![
            (
                "redeploy already deployed",
                "redeploy_already_deployed",
                None,
                WatchStatus::Stop,
            ),
            (
                "queued behind a running deployment",
                "queued_behind_running_deployment",
                None,
                WatchStatus::Stop,
            ),
            // every watched service waits behind a deployment of other services, and /statuses keeps
            // showing them with their previous execution id: DEPLOYED at 1.0s and 14.1s, queued in
            // between. Without the `execution_id == before` skip, the watch would stop at 1.0s
            // (and at 14.1s for the case above)
            (
                "only services queued behind another deployment",
                "queued_behind_running_deployment",
                Some(svc),
                WatchStatus::Stop,
            ),
            ("restart", "restart", None, WatchStatus::Stop),
            ("stop", "stop", None, WatchStatus::Stop),
            (
                "stop already stopped",
                "stop_already_stopped",
                None,
                WatchStatus::Stop,
            ),
            ("deploy error", "deploy_error", None, WatchStatus::Err),
        ];

        'tests: for (name, trace_name, only, expected) in tests {
            let mut trace = load_trace(trace_name);
            if let Some(o) = only {
                trace.service_ids = o;
            }
            let tracker = ServicesTracker::new(trace.service_ids.clone(), trace.before.clone());
            for (i, p) in trace.polls.iter().enumerate() {
                let (status, done, err) =
                    tracker.update(&with_stages(&p.with_stages), &trace.final_state);
                let all_done = trace.service_ids.iter().all(|id| {
                    let s = p.with_stages.get(id);
                    s.is_some_and(|s| {
                        s.state == trace.final_state
                            && Some(&s.execution_id) != trace.before.get(id)
                    })
                });
                assert!(
                    !(status == WatchStatus::Stop && !all_done),
                    "{name}: poll {i} at {:.1}s: Stop while services were not all done ({done}/{})",
                    p.t,
                    trace.service_ids.len()
                );
                if status != WatchStatus::Continue {
                    assert_eq!(
                        status, expected,
                        "{name}: poll {i} at {:.1}s (err {err:?})",
                        p.t
                    );
                    continue 'tests;
                }
            }
            panic!("{name}: the watch never ended, want {expected:?}");
        }
    }

    #[test]
    fn services_tracker() {
        type Case = (
            &'static str,
            HashMap<String, String>,
            &'static str,
            Vec<Poll>,
            WatchStatus,
            &'static str,
        );
        let tests: Vec<Case> = vec![
            (
                "previous final state with the same execution is not done",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![
                    vec![("a", "DEPLOYED", "env-1")],
                    vec![("a", "DEPLOYED", "env-1")],
                ],
                WatchStatus::Continue,
                "",
            ),
            (
                "new execution in the final state is done even if never seen in progress",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![vec![("a", "DEPLOYED", "env-2")]],
                WatchStatus::Stop,
                "",
            ),
            (
                "previous execution in progress then done is not done",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![
                    vec![("a", "DEPLOYING", "env-1")],
                    vec![("a", "DEPLOYED", "env-1")],
                ],
                WatchStatus::Continue,
                "",
            ),
            (
                "previous execution in progress, then new execution done",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![
                    vec![("a", "DEPLOYING", "env-1")],
                    vec![("a", "DEPLOYMENT_QUEUED", "env-1")],
                    vec![("a", "DEPLOYING", "env-2")],
                    vec![("a", "DEPLOYED", "env-2")],
                ],
                WatchStatus::Stop,
                "",
            ),
            (
                "later deploy of the same service is waited for",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![
                    vec![("a", "DEPLOYMENT_QUEUED", "env-2")],
                    vec![("a", "DEPLOYING", "env-2")],
                    vec![("a", "DEPLOYMENT_QUEUED", "env-3")],
                    vec![("a", "DEPLOYING", "env-3")],
                    vec![("a", "DEPLOYED", "env-3")],
                ],
                WatchStatus::Stop,
                "",
            ),
            (
                "later stop of the same service fails the deploy watch",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![
                    vec![("a", "DEPLOYMENT_QUEUED", "env-2")],
                    vec![("a", "DEPLOYING", "env-2")],
                    vec![("a", "STOP_QUEUED", "env-3")],
                    vec![("a", "STOPPING", "env-3")],
                    vec![("a", "STOPPED", "env-3")],
                ],
                WatchStatus::Err,
                "service a ended in state STOPPED instead of DEPLOYED",
            ),
            (
                "error of the previous execution is ignored",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![vec![("a", "DEPLOYMENT_ERROR", "env-1")]],
                WatchStatus::Continue,
                "",
            ),
            (
                "error of the new execution fails the watch",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![
                    vec![("a", "DEPLOYMENT_QUEUED", "env-1")],
                    vec![("a", "DEPLOYMENT_ERROR", "env-2")],
                ],
                WatchStatus::Err,
                "service a is in state DEPLOYMENT_ERROR",
            ),
            (
                "canceled fails the watch",
                before(&[("a", "env-1")]),
                "STOPPED",
                vec![
                    vec![("a", "STOP_QUEUED", "env-1")],
                    vec![("a", "CANCELED", "env-2")],
                ],
                WatchStatus::Err,
                "service a is in state CANCELED",
            ),
            (
                "another final state fails the watch",
                before(&[("a", "env-1")]),
                "STOPPED",
                vec![vec![("a", "DEPLOYED", "env-2")]],
                WatchStatus::Err,
                "service a ended in state DEPLOYED instead of STOPPED",
            ),
            (
                "deleted service disappears",
                before(&[("a", "env-1")]),
                "DELETED",
                vec![vec![("a", "DELETE_QUEUED", "env-1")], vec![]],
                WatchStatus::Stop,
                "",
            ),
            (
                "service missing when the watch started is not counted as deleted",
                before(&[]),
                "DELETED",
                vec![vec![]],
                WatchStatus::Err,
                "service a was not in the environment when the watch started",
            ),
            (
                "missing service fails the watch when not deleting",
                before(&[("a", "env-1")]),
                "DEPLOYED",
                vec![vec![]],
                WatchStatus::Err,
                "service a no longer exists in the environment",
            ),
        ];

        for (name, before, final_state, polls, expected, expected_error) in tests {
            let tracker = ServicesTracker::new(vec!["a".into()], before);
            let mut status = WatchStatus::Continue;
            let mut err = None;
            for p in &polls {
                (status, _, err) = tracker.update(&poll(p), final_state);
                if status != WatchStatus::Continue {
                    break;
                }
            }
            assert_eq!(status, expected, "{name}");
            assert_eq!(err.unwrap_or_default(), expected_error, "{name}");
        }
    }

    type Response = Result<EnvironmentStatusesWithStages, String>;

    #[test]
    fn watch_services_retries_api_errors() {
        let deployed = || Ok(poll(&vec![("a", "DEPLOYED", "env-2")]));
        let deploying = || Ok(poll(&vec![("a", "DEPLOYING", "env-2")]));
        let api_error = || Err("503 Service Unavailable".to_string());

        let tests: Vec<(&str, Vec<Response>, WatchStatus, usize)> = vec![
            (
                "transient errors are retried",
                vec![
                    api_error(),
                    api_error(),
                    deploying(),
                    api_error(),
                    deployed(),
                ],
                WatchStatus::Stop,
                5,
            ),
            (
                "errors in a row fail the watch",
                vec![
                    deploying(),
                    api_error(),
                    api_error(),
                    api_error(),
                    api_error(),
                    api_error(),
                    deployed(),
                ],
                WatchStatus::Err,
                1 + MAX_CONSECUTIVE_STATUS_ERRORS,
            ),
            (
                "a success resets the error count",
                vec![
                    api_error(),
                    api_error(),
                    api_error(),
                    api_error(),
                    deploying(),
                    api_error(),
                    api_error(),
                    api_error(),
                    api_error(),
                    deployed(),
                ],
                WatchStatus::Stop,
                10,
            ),
        ];

        for (name, responses, expected, expected_calls) in tests {
            let calls = Cell::new(0usize);
            let mut fetch = || {
                let i = calls.get();
                assert!(
                    i < responses.len(),
                    "{name}: watchServices fetched more than the {} prepared responses",
                    responses.len()
                );
                calls.set(i + 1);
                responses[i].clone()
            };
            let tracker = ServicesTracker::new(vec!["a".into()], before(&[("a", "env-1")]));
            let got = watch_services(&mut fetch, &mut || {}, &mut || false, &tracker, "DEPLOYED");
            assert_eq!(got, expected, "{name}");
            assert_eq!(calls.get(), expected_calls, "{name}");
        }
    }

    #[test]
    fn watch_services_timeout() {
        let calls = Cell::new(0usize);
        let mut fetch = || {
            calls.set(calls.get() + 1);
            Ok(poll(&vec![("a", "DEPLOYING", "env-2")]))
        };
        let tracker = ServicesTracker::new(vec!["a".into()], before(&[("a", "env-1")]));
        let got = watch_services(
            &mut fetch,
            &mut || {},
            &mut || calls.get() >= 3,
            &tracker,
            "DEPLOYED",
        );
        assert_eq!(
            got,
            WatchStatus::Err,
            "watchServices must fail once the deadline is passed"
        );
        assert_eq!(calls.get(), 3);
    }

    #[test]
    fn snapshot_executions_retries() {
        // transient errors are retried
        let mut calls = 0;
        let mut fetch = || {
            calls += 1;
            if calls < 3 {
                Err("503 Service Unavailable".to_string())
            } else {
                Ok(poll(&vec![("a", "DEPLOYED", "env-1")]))
            }
        };
        let before = snapshot_executions(&mut fetch, &mut || {}).unwrap();
        assert_eq!(before["a"], "env-1");

        // errors in a row fail before the request is sent
        let mut calls = 0;
        let mut fetch = || {
            calls += 1;
            Err("503 Service Unavailable".to_string())
        };
        let err = snapshot_executions(&mut fetch, &mut || {}).unwrap_err();
        assert_eq!(
            err,
            "cannot get the services status after 5 attempts: 503 Service Unavailable"
        );
        assert_eq!(calls, MAX_CONSECUTIVE_STATUS_ERRORS);
    }

    #[test]
    fn watch_timeout_values() {
        let tests = [
            ("", DEFAULT_WATCH_TIMEOUT),
            ("2h", Duration::from_secs(7200)),
            ("90m", Duration::from_secs(5400)),
            ("not-a-duration", DEFAULT_WATCH_TIMEOUT),
            ("-5m", DEFAULT_WATCH_TIMEOUT),
        ];
        for (value, expected) in tests {
            assert_eq!(
                watch_timeout_from(value),
                expected,
                "watchTimeout() with {value:?}"
            );
        }
    }

    #[test]
    fn state_helpers() {
        assert!(is_terminal_state("READY"));
        assert!(is_terminal_state("BUILD_ERROR"));
        assert!(!is_terminal_state("DEPLOYING"));
        assert!(is_terminal_cluster_state("INVALID_CREDENTIALS"));
        assert!(!is_terminal_state("INVALID_CREDENTIALS"));
        assert!(is_final_state("RESTARTED"));
        assert!(!is_final_state("READY"));
    }

    #[test]
    fn statuses_with_stages_decoding() {
        let v: EnvironmentStatusesWithStages = serde_json::from_str(
            r#"{"environment": {"id": "e"}, "stages": [{"applications": null, "containers": [{"id": "c", "state": "DEPLOYED", "execution_id": "x"}], "stage": {"id": "s"}}]}"#,
        )
        .unwrap();
        assert_eq!(v.execution_ids()["c"], "x");
    }
}
