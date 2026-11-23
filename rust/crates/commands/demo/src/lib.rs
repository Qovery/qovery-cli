//! `qovery demo` command. Port of `cmd/demo.go`, `cmd/demo_up*.go` and `cmd/demo_destroy.go`.
//!
//! * [`failure`]: classified failures (`demo_up_failure.go`) and the Go error-chain semantics
//! * [`runner`]: local command execution, output sinks, interruption (`demoExecRunner`)
//! * [`local`]: docker / k3d / helm / kubectl operations (`demoLocalCommands`)
//! * [`api`]: Qovery API calls (`demoQoveryAPI`)
//! * [`orchestrator`]: `demo up` / `demo destroy` workflows
//! * [`upload`]: upload of the debug log after a failure

mod api;
mod diagnostics;
mod failure;
mod local;
mod orchestrator;
mod runner;
mod upload;

use failure::*;
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::telemetry::{self, CommandExecutionProperties};
use qovery_log::gofmt::quote;
use runner::{Cancel, MultiWriter, Sink, StdoutWriter};
use std::cell::RefCell;
use std::io::Write;
use std::rc::Rc;

const DEMO_WORKFLOW_TYPE: &str = "demo_installation";
const DEMO_IMPLEMENTATION: &str = "engine_v2";

/// The `demo` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("demo")
        .short("Try Qovery on your local machine")
        .long("Try Qovery on your local machine.\n\nqovery demo up creates a local k3d cluster, installs the Qovery Operator on it and lets Qovery\ndeploy the self-managed platform catalog with Engine v2.")
        .run(run_demo)
        .subcommand(destroy())
        .subcommand(up())
}

fn up() -> Command {
    Command::new("up")
        .short("Create a local k3d cluster running the Qovery Operator and Engine v2")
        .args(ArgsRule::NoArgs)
        .flag(
            Flag::string("cluster-name", "The name of the local demo cluster")
                .short('c')
                .default_value(format!("local-demo-{}", demo_safe_username())),
        )
        .flag(Flag::bool("debug", "Enable debug mode"))
        .run(run_up)
}

fn destroy() -> Command {
    Command::new("destroy")
        .short("Remove the local demo cluster")
        .args(ArgsRule::NoArgs)
        .flag(
            Flag::string("cluster-name", "The name of the local demo cluster")
                .short('c')
                .default_value(format!("local-demo-{}", demo_safe_username())),
        )
        .flag(
            Flag::bool(
                "delete-qovery-config",
                "Also delete the Qovery cluster, its environments, and its Operator association",
            )
            .short('d'),
        )
        .run(run_destroy)
}

fn run_demo(ctx: &Ctx) {
    telemetry::capture(ctx);
    ctx.help();
    if let Some(arg) = ctx.args().first() {
        qovery_log::println_error(format!(
            "unknown command {} for {}",
            quote(arg),
            quote(&ctx.command_path())
        ));
        qovery_core::exit(1);
    }
}

// --- helpers ----------------------------------------------------------------------------------------------

pub(crate) use qovery_core::gosys::{go_arch, go_os};

/// `detectArchitecture()`: only needed due to macOS when rosetta (x86_64 emulation on ARM64) is turned on.
pub(crate) fn detect_architecture() -> String {
    if go_os() != "darwin" {
        return go_arch().to_uppercase();
    }
    rosetta_native_arch().to_uppercase()
}

#[cfg(target_os = "macos")]
fn rosetta_native_arch() -> &'static str {
    let mut value: libc::c_int = 0;
    let mut size = std::mem::size_of::<libc::c_int>();
    let name = c"sysctl.proc_translated";
    let rc = unsafe {
        libc::sysctlbyname(
            name.as_ptr(),
            &mut value as *mut _ as *mut libc::c_void,
            &mut size,
            std::ptr::null_mut(),
            0,
        )
    };
    if rc == 0 && value == 1 {
        "arm64"
    } else {
        go_arch()
    }
}

#[cfg(not(target_os = "macos"))]
fn rosetta_native_arch() -> &'static str {
    go_arch()
}

use qovery_core::gosys::current_username;

/// `demoSafeUsername()`.
fn demo_safe_username() -> String {
    let Some(username) = current_username() else {
        return "qovery".to_string();
    };
    safe_username(&username)
}

fn safe_username(username: &str) -> String {
    let lower = username.to_lowercase();
    // regexp `[^a-z0-9-]+` replaced by "-"
    let mut value = String::with_capacity(lower.len());
    let mut in_run = false;
    for c in lower.chars() {
        if c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-' {
            value.push(c);
            in_run = false;
        } else if !in_run {
            value.push('-');
            in_run = true;
        }
    }
    let value = value.trim_matches('-');
    if value.is_empty() {
        "qovery".to_string()
    } else {
        value.to_string()
    }
}

/// `validateDemoClusterName(name)`: `^[a-zA-Z][-a-zA-Z0-9]*[a-zA-Z0-9]$`.
fn validate_demo_cluster_name(name: &str) -> Result<(), DemoError> {
    let b = name.as_bytes();
    let valid = b.len() >= 2
        && b[0].is_ascii_alphabetic()
        && b[b.len() - 1].is_ascii_alphanumeric()
        && b[1..b.len() - 1]
            .iter()
            .all(|c| c.is_ascii_alphanumeric() || *c == b'-');
    if !valid {
        return Err(DemoError::new(format!(
            "cluster name must start with a letter, end with a letter or digit, and contain only letters, digits, or hyphens: got {}",
            quote(name)
        )));
    }
    Ok(())
}

// --- demo up ----------------------------------------------------------------------------------------------

/// `demoAttempt`.
struct Attempt {
    id: String,
    started_at: std::time::Instant,
    cluster_id: String,
    failed_phase: String,
    canceled: bool,
}

impl Attempt {
    /// `newDemoAttempt()`.
    fn new() -> Self {
        Attempt {
            id: uuid::Uuid::now_v7().to_string(),
            started_at: std::time::Instant::now(),
            cluster_id: String::new(),
            failed_phase: String::new(),
            canceled: false,
        }
    }

    fn capture(&self, ctx: &Ctx, event: &str, mut execution: CommandExecutionProperties) {
        execution.attempt_id = self.id.clone();
        execution.workflow_type = DEMO_WORKFLOW_TYPE.to_string();
        execution.implementation = DEMO_IMPLEMENTATION.to_string();
        execution.cluster_id = self.cluster_id.clone();
        if !execution.result.is_empty() && execution.duration_millis <= 0 {
            execution.duration_millis = (self.started_at.elapsed().as_millis() as i64).max(1);
        }
        telemetry::capture_command_execution(ctx, event, &execution);
    }

    fn print_failure_summary(&self, safe_message: &str) {
        let mut phase = self.failed_phase.replace('_', " ");
        if phase.is_empty() {
            phase = "an unknown phase".to_string();
        }
        let _ = write!(
            std::io::stderr(),
            "\nDemo installation failed during {phase}.\n{safe_message}\nDiagnostic ID: {}\n",
            self.id
        );
    }
}

/// `demoAttemptTelemetry`.
struct AttemptTelemetry<'a, 'b> {
    attempt: &'a mut Attempt,
    ctx: &'a Ctx<'b>,
}

impl orchestrator::Progress for AttemptTelemetry<'_, '_> {
    fn cluster_resolved(&mut self, cluster_id: &str) {
        self.attempt.cluster_id = cluster_id.to_string();
    }

    fn phase_finished(&mut self, execution: orchestrator::PhaseExecution) {
        if execution.result != orchestrator::PHASE_RESULT_SUCCEEDED {
            self.attempt.failed_phase = execution.phase.clone();
            self.attempt.canceled = execution.result == orchestrator::PHASE_RESULT_CANCELED;
        }
        let mut properties = CommandExecutionProperties {
            phase: execution.phase.clone(),
            result: execution.result.clone(),
            duration_millis: (execution.duration.as_millis() as i64).max(1),
            ..Default::default()
        };
        if execution.result == orchestrator::PHASE_RESULT_FAILED {
            properties.error_code = execution.error_code;
            properties.safe_error_message = execution.safe_error_message;
        }
        self.attempt
            .capture(self.ctx, telemetry::PHASE_FINISHED_EVENT_NAME, properties);
    }
}

fn run_up(ctx: &Ctx) {
    let mut attempt = Attempt::new();
    attempt.capture(
        ctx,
        telemetry::DEFAULT_EVENT_NAME,
        CommandExecutionProperties::default(),
    );

    let cancel = runner::notify_context();
    let id = attempt.id.clone();
    let result = run_demo_up(
        cancel,
        ctx,
        &mut AttemptTelemetry {
            attempt: &mut attempt,
            ctx,
        },
        &id,
    );
    runner::stop_notify();
    let Err(err) = result else {
        attempt.capture(
            ctx,
            telemetry::END_OF_EXECUTION_EVENT_NAME,
            CommandExecutionProperties {
                result: "succeeded".into(),
                ..Default::default()
            },
        );
        return;
    };
    if attempt.canceled || err.interrupted {
        attempt.capture(
            ctx,
            telemetry::END_OF_EXECUTION_EVENT_NAME,
            CommandExecutionProperties {
                result: orchestrator::PHASE_RESULT_CANCELED.into(),
                phase: attempt.failed_phase.clone(),
                ..Default::default()
            },
        );
        let _ = write!(std::io::stderr(), "\nDemo installation interrupted.\n");
        qovery_core::exit(130);
    }
    let (error_code, safe_message) = demo_failure_details(&err);
    attempt.capture(
        ctx,
        telemetry::END_OF_EXECUTION_ERROR_EVENT_NAME,
        CommandExecutionProperties {
            result: "failed".into(),
            phase: attempt.failed_phase.clone(),
            error_code,
            safe_error_message: safe_message.clone(),
            ..Default::default()
        },
    );
    attempt.print_failure_summary(&safe_message);
    qovery_core::exit_with_error(err);
}

/// Go `os.MkdirAll(path, perm)` with its error reporting.
fn mkdir_all(path: &std::path::Path, mode: u32) -> Result<(), String> {
    match std::fs::metadata(path) {
        Ok(m) if m.is_dir() => return Ok(()),
        Ok(_) => {
            return Err(qovery_core::context::go_path_error(
                "mkdir",
                path,
                &not_a_directory(),
            ));
        }
        Err(_) => {}
    }
    if let Some(parent) = path.parent()
        && !parent.as_os_str().is_empty()
    {
        mkdir_all(parent, mode)?;
    }
    #[cfg_attr(not(unix), allow(unused_mut))]
    let mut builder = std::fs::DirBuilder::new();
    // Go ignores the permission bits on Windows
    #[cfg(unix)]
    std::os::unix::fs::DirBuilderExt::mode(&mut builder, mode);
    #[cfg(not(unix))]
    let _ = mode;
    if let Err(e) = builder.create(path) {
        if std::fs::metadata(path).map(|m| m.is_dir()).unwrap_or(false) {
            return Ok(());
        }
        return Err(qovery_core::context::go_path_error("mkdir", path, &e));
    }
    Ok(())
}

/// `syscall.ENOTDIR`.
#[cfg(unix)]
fn not_a_directory() -> std::io::Error {
    std::io::Error::from_raw_os_error(libc::ENOTDIR)
}

/// `syscall.ENOTDIR` (an invented errno on Windows, "not a directory").
#[cfg(not(unix))]
fn not_a_directory() -> std::io::Error {
    std::io::Error::other("not a directory")
}

/// `os.OpenFile(path, O_CREATE|O_TRUNC|O_WRONLY, mode)`; Go ignores the permission bits on Windows.
fn open_options_with_mode(mode: u32) -> std::fs::OpenOptions {
    #[cfg_attr(not(unix), allow(unused_mut))]
    let mut options = std::fs::OpenOptions::new();
    #[cfg(unix)]
    std::os::unix::fs::OpenOptionsExt::mode(&mut options, mode);
    #[cfg(not(unix))]
    let _ = mode;
    options
}

/// `openDemoDebugLog()`.
fn open_demo_debug_log() -> Result<(std::fs::File, String), DemoError> {
    let directory = std::env::temp_dir().join("qovery-demo");
    mkdir_all(&directory, 0o700)
        .map_err(|e| DemoError::new(format!("cannot create demo log directory: {e}")))?;
    let log_path = directory.join("qovery-demo.log");
    let file = open_options_with_mode(0o600)
        .create(true)
        .truncate(true)
        .write(true)
        .open(&log_path)
        .map_err(|e| {
            DemoError::new(format!(
                "cannot create demo log file: {}",
                qovery_core::context::go_path_error("open", &log_path, &e)
            ))
        })?;
    Ok((file, log_path.to_string_lossy().to_string()))
}

/// Error of `qovery demo up` and `qovery demo destroy` on Windows.
const UNSUPPORTED_ON_WINDOWS: &str = "qovery demo is not supported directly on Windows; use WSL";

/// `runDemoUp(ctx, cmd, progress, diagnosticID)`.
fn run_demo_up(
    cancel: Cancel,
    ctx: &Ctx,
    progress: &mut AttemptTelemetry,
    diagnostic_id: &str,
) -> Result<(), DemoError> {
    if go_os() == "windows" {
        return Err(new_demo_failure(
            ERROR_UNSUPPORTED_OS,
            "Windows is not supported directly; use WSL.",
            DemoError::new(UNSUPPORTED_ON_WINDOWS),
        ));
    }
    let (mut token_type, mut token) = qovery_core::auth::get_access_token(false).map_err(|e| {
        new_demo_failure(
            ERROR_AUTHENTICATION,
            "Authentication failed.",
            DemoError::new(format!(
                "authentication failed; run `qovery auth` first: {e}"
            )),
        )
    })?;
    let organization = qovery_core::context::current_organization(true).map_err(|e| {
        new_demo_failure(
            ERROR_ORGANIZATION,
            "The current organization could not be resolved.",
            DemoError::new(format!("cannot resolve the current organization: {e}")),
        )
    })?;
    let cluster_name = ctx.string("cluster-name");
    validate_demo_cluster_name(&cluster_name).map_err(|e| {
        new_demo_failure(
            ERROR_INVALID_CLUSTER_NAME,
            "The cluster name is invalid.",
            e,
        )
    })?;
    let (debug_file, debug_logs_path) = open_demo_debug_log()?;
    let debug_log: Sink = Rc::new(RefCell::new(debug_file));
    runner::write_to(&debug_log, &format!("Diagnostic ID: {diagnostic_id}\n"));

    let terminal_output: Sink = Rc::new(RefCell::new(StdoutWriter));
    let exec_runner = runner::ExecRunner {
        out: Some(terminal_output.clone()),
        log: Some(debug_log.clone()),
        debug: ctx.bool("debug"),
    };
    let temp_dir = std::path::Path::new(&debug_logs_path)
        .parent()
        .map(|p| p.to_string_lossy().to_string())
        .unwrap_or_default();
    let mut orchestrator = orchestrator::Orchestrator {
        api: api::QoveryApi::new(qovery_core::client::client_for(token_type, &token)),
        local: local::LocalCommands {
            runner: exec_runner,
            goos: go_os().to_string(),
            temp_dir,
            kube_context: local::demo_kube_context(&cluster_name),
            debug_log: debug_log.clone(),
        },
        clock: orchestrator::SystemClock,
        out: Rc::new(RefCell::new(MultiWriter(vec![
            terminal_output,
            debug_log.clone(),
        ]))),
        progress,
    };
    let result = orchestrator.up(
        cancel,
        orchestrator::Config {
            organization_id: organization.id.clone(),
            cluster_name: cluster_name.clone(),
            cpu_architecture: detect_architecture(),
            ..Default::default()
        },
    );
    drop(orchestrator);
    if let Err(err) = result {
        runner::write_to(&debug_log, &format!("\nERROR: {err}\n"));
        let _ = debug_log.borrow_mut().flush();
        if err.interrupted {
            return Err(err);
        }
        if let Ok((t, k)) = qovery_core::auth::get_access_token(false) {
            (token_type, token) = (t, k);
        }
        upload::upload_error_logs(
            token_type,
            &token,
            &organization.id,
            &cluster_name,
            &debug_logs_path,
        );
        return Err(err);
    }
    Ok(())
}

impl<T: orchestrator::Progress + ?Sized> orchestrator::Progress for &mut T {
    fn cluster_resolved(&mut self, cluster_id: &str) {
        (**self).cluster_resolved(cluster_id)
    }
    fn phase_finished(&mut self, execution: orchestrator::PhaseExecution) {
        (**self).phase_finished(execution)
    }
}

// --- demo destroy -----------------------------------------------------------------------------------------

fn run_destroy(ctx: &Ctx) {
    telemetry::capture(ctx);
    let cancel = runner::notify_context();
    let result = run_demo_destroy(cancel, ctx);
    runner::stop_notify();
    if let Err(err) = result {
        telemetry::capture_with_event(ctx, telemetry::END_OF_EXECUTION_ERROR_EVENT_NAME);
        qovery_core::exit_with_error(err);
    }
    telemetry::capture_with_event(ctx, telemetry::END_OF_EXECUTION_EVENT_NAME);
}

/// `runDemoDestroy(ctx, cmd)`.
fn run_demo_destroy(cancel: Cancel, ctx: &Ctx) -> Result<(), DemoError> {
    if go_os() == "windows" {
        return Err(DemoError::new(UNSUPPORTED_ON_WINDOWS));
    }
    let cluster_name = ctx.string("cluster-name");
    validate_demo_cluster_name(&cluster_name)?;
    let delete_qovery_config = ctx.bool("delete-qovery-config");

    let mut api = None;
    let mut organization_id = String::new();
    if delete_qovery_config {
        let (token_type, token) = qovery_core::auth::get_access_token(false).map_err(|e| {
            DemoError::new(format!(
                "authentication failed; run `qovery auth` first: {e}"
            ))
        })?;
        let organization = qovery_core::context::current_organization(true)
            .map_err(|e| DemoError::new(format!("cannot resolve the current organization: {e}")))?;
        organization_id = organization.id;
        api = Some(api::QoveryApi::new(qovery_core::client::client_for(
            token_type, &token,
        )));
    }

    let terminal_output: Sink = Rc::new(RefCell::new(StdoutWriter));
    let mut orchestrator = orchestrator::DestroyOrchestrator {
        api,
        local: local::LocalCommands::new(
            runner::ExecRunner {
                out: Some(terminal_output.clone()),
                log: None,
                debug: false,
            },
            go_os(),
            Rc::new(RefCell::new(std::io::sink())),
        ),
        out: terminal_output,
    };
    orchestrator.destroy(
        cancel,
        &orchestrator::DestroyConfig {
            organization_id,
            cluster_name,
            delete_qovery_config,
        },
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn new_attempt_uses_uuid_v7() {
        let attempt = Attempt::new();
        let id = uuid::Uuid::parse_str(&attempt.id).unwrap();
        assert_eq!(7, id.get_version_num());
    }

    #[test]
    fn safe_username_cases() {
        assert_eq!("erebe", safe_username("erebe"));
        assert_eq!("john-doe", safe_username("John.Doe"));
        assert_eq!("a-b", safe_username("__a@@b__"));
        assert_eq!("qovery", safe_username("___"));
        assert_eq!("pi-rre", safe_username("Piérre"));
    }

    #[test]
    fn validate_cluster_name_cases() {
        assert!(validate_demo_cluster_name("local-demo-user").is_ok());
        assert!(validate_demo_cluster_name("a1").is_ok());
        for bad in ["a", "1abc", "abc-", "ab_c", "", "é-a"] {
            assert!(validate_demo_cluster_name(bad).is_err(), "{bad}");
        }
        assert_eq!(
            "cluster name must start with a letter, end with a letter or digit, and contain only letters, digits, or hyphens: got \"a\"",
            validate_demo_cluster_name("a").unwrap_err().to_string()
        );
    }
}
