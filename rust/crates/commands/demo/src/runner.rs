//! Local command execution of `qovery demo` (`demoCommandRunner`, `demoExecRunner`), output sinks and the
//! interruption context (`signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`).

use std::cell::RefCell;
use std::io::{Read, Write};
use std::process::{Command, ExitStatus, Stdio};
use std::rc::Rc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc;
use std::time::{Duration, Instant};

/// `demoCommandInterruptGrace`: leaves time for helm --atomic to roll back after an interrupt.
pub(crate) const DEMO_COMMAND_INTERRUPT_GRACE: Duration = Duration::from_secs(120);

// ---------------------------------------------------------------------------------------------------------
// Cancellation

/// The Go `context.Context` of a demo run: only its cancellation matters.
#[derive(Clone, Copy)]
pub(crate) struct Cancel(&'static AtomicBool);

impl Cancel {
    /// A context that is never canceled unless [`Cancel::cancel`] is called (tests).
    #[cfg(test)]
    pub fn new() -> Self {
        Cancel(Box::leak(Box::new(AtomicBool::new(false))))
    }

    #[cfg(test)]
    pub fn cancel(&self) {
        self.0.store(true, Ordering::SeqCst);
    }

    pub fn is_canceled(&self) -> bool {
        self.0.load(Ordering::SeqCst)
    }

    /// `ctx.Err()`.
    pub fn err(&self) -> Option<crate::failure::DemoError> {
        self.is_canceled().then(crate::failure::DemoError::canceled)
    }
}

static SIGNALED: AtomicBool = AtomicBool::new(false);

extern "C" fn on_signal(_: libc::c_int) {
    SIGNALED.store(true, Ordering::SeqCst);
    // `context.AfterFunc(ctx, stop)`: a second signal gets the default behavior.
    unsafe {
        libc::signal(libc::SIGINT, libc::SIG_DFL);
        libc::signal(libc::SIGTERM, libc::SIG_DFL);
    }
}

/// `signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)`.
pub(crate) fn notify_context() -> Cancel {
    unsafe {
        libc::signal(libc::SIGINT, on_signal as *const () as libc::sighandler_t);
        libc::signal(libc::SIGTERM, on_signal as *const () as libc::sighandler_t);
    }
    Cancel(&SIGNALED)
}

/// `stop()` of `signal.NotifyContext`.
pub(crate) fn stop_notify() {
    unsafe {
        libc::signal(libc::SIGINT, libc::SIG_DFL);
        libc::signal(libc::SIGTERM, libc::SIG_DFL);
    }
}

// ---------------------------------------------------------------------------------------------------------
// Output sinks

/// A shared `io.Writer`.
pub(crate) type Sink = Rc<RefCell<dyn Write>>;

/// `cmd.OutOrStdout()`: unbuffered stdout.
pub(crate) struct StdoutWriter;

impl Write for StdoutWriter {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        let mut out = std::io::stdout().lock();
        out.write_all(buf)?;
        out.flush()?;
        Ok(buf.len())
    }
    fn flush(&mut self) -> std::io::Result<()> {
        std::io::stdout().flush()
    }
}

/// `io.MultiWriter(...)`.
pub(crate) struct MultiWriter(pub Vec<Sink>);

impl Write for MultiWriter {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        for w in &self.0 {
            w.borrow_mut().write_all(buf)?;
        }
        Ok(buf.len())
    }
    fn flush(&mut self) -> std::io::Result<()> {
        for w in &self.0 {
            w.borrow_mut().flush()?;
        }
        Ok(())
    }
}

/// `fmt.Fprint(w, s)`, errors ignored.
pub(crate) fn write_to(w: &Sink, s: &str) {
    let _ = w.borrow_mut().write_all(s.as_bytes());
}

// ---------------------------------------------------------------------------------------------------------
// Runner

/// Error of a local command (`*exec.ExitError` when it exited with a status, `*exec.Error` & co otherwise).
#[derive(Debug, Clone, PartialEq)]
pub(crate) struct ExecError {
    /// `ExitError.ExitCode()` (-1 when killed by a signal); `None` when the command did not run to completion.
    pub exit_code: Option<i32>,
    pub message: String,
}

impl ExecError {
    pub fn new(message: impl Into<String>) -> Self {
        ExecError {
            exit_code: None,
            message: message.into(),
        }
    }
}

/// Output (stdout and stderr combined) and outcome of a command.
pub(crate) type RunResult = (Vec<u8>, Result<(), ExecError>);

/// `demoCommandRunner`.
pub(crate) trait CommandRunner {
    fn look_path(&mut self, name: &str) -> Result<(), String>;
    fn run(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult;
    fn run_quiet(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult;
    /// Keeps the output out of the debug log, which is uploaded on failure.
    fn run_unlogged(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult;
    /// Logs only the command line and never writes to the terminal: the caller logs the output it keeps.
    fn run_silent(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult;
}

/// `demoExecRunner`.
pub(crate) struct ExecRunner {
    pub out: Option<Sink>,
    pub log: Option<Sink>,
    pub debug: bool,
}

/// `exec.LookPath(name)`.
pub(crate) use qovery_core::gosys::look_path;

impl CommandRunner for ExecRunner {
    fn look_path(&mut self, name: &str) -> Result<(), String> {
        look_path(name).map(|_| ())
    }
    fn run(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult {
        self.execute(cancel, true, true, name, args)
    }
    fn run_quiet(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult {
        self.execute(cancel, false, true, name, args)
    }
    fn run_unlogged(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult {
        self.execute(cancel, false, false, name, args)
    }
    fn run_silent(&mut self, cancel: Cancel, name: &str, args: &[String]) -> RunResult {
        if let Some(log) = &self.log {
            let mut parts = vec![name.to_string()];
            parts.extend(args.iter().cloned());
            write_to(log, &format!("$ {}\n", parts.join(" ")));
        }
        run_command(cancel, name, args, &[])
    }
}

impl ExecRunner {
    fn execute(
        &mut self,
        cancel: Cancel,
        visible: bool,
        log_output: bool,
        name: &str,
        args: &[String],
    ) -> RunResult {
        let mut parts = vec![name.to_string()];
        parts.extend(args.iter().cloned());
        let command_line = parts.join(" ");
        if let Some(log) = &self.log {
            write_to(log, &format!("$ {command_line}\n"));
        }
        let show_command = visible || self.debug;
        let stream_output = visible || (self.debug && log_output);
        if show_command && let Some(out) = &self.out {
            write_to(out, &format!("$ {command_line}\n"));
        }

        let mut writers: Vec<Sink> = Vec::new();
        if log_output && let Some(log) = &self.log {
            writers.push(log.clone());
        }
        if stream_output && let Some(out) = &self.out {
            writers.push(out.clone());
        }
        let (output, result) = run_command(cancel, name, args, &writers);
        if result.is_err()
            && !stream_output
            && let Some(out) = &self.out
        {
            if !show_command {
                write_to(out, &format!("$ {command_line}\n"));
            }
            let _ = out.borrow_mut().write_all(&output);
        }
        (output, result)
    }
}

fn exit_error(status: ExitStatus) -> ExecError {
    match status.code() {
        Some(code) => ExecError {
            exit_code: Some(code),
            message: format!("exit status {code}"),
        },
        None => ExecError {
            exit_code: Some(-1),
            message: format!("signal: {}", exit_signal(status)),
        },
    }
}

#[cfg(unix)]
fn exit_signal(status: ExitStatus) -> String {
    use std::os::unix::process::ExitStatusExt;
    status.signal().map(|s| s.to_string()).unwrap_or_default()
}

/// A Windows process always has an exit code.
#[cfg(not(unix))]
fn exit_signal(_status: ExitStatus) -> String {
    String::new()
}

/// `command.Process.Signal(os.Interrupt)`.
#[cfg(unix)]
fn interrupt(child: &mut std::process::Child) {
    // SAFETY: plain kill(2) on the pid of a child we own and have not reaped yet.
    unsafe {
        libc::kill(child.id() as libc::pid_t, libc::SIGINT);
    }
}

/// `command.Process.Signal(os.Interrupt)` is not supported on Windows: Go's `Cancel` fails and the command is
/// killed instead.
#[cfg(not(unix))]
fn interrupt(child: &mut std::process::Child) {
    let _ = child.kill();
}

/// `exec.CommandContext(ctx, name, args...)` with `Cancel` sending SIGINT and `WaitDelay` set to the interrupt
/// grace; stdout and stderr go, combined, to the returned buffer and to `writers`.
fn run_command(cancel: Cancel, name: &str, args: &[String], writers: &[Sink]) -> RunResult {
    run_command_with_wait_delay(cancel, name, args, writers, DEMO_COMMAND_INTERRUPT_GRACE)
}

fn run_command_with_wait_delay(
    cancel: Cancel,
    name: &str,
    args: &[String],
    writers: &[Sink],
    wait_delay: Duration,
) -> RunResult {
    if cancel.is_canceled() {
        return (
            Vec::new(),
            Err(ExecError::new(crate::failure::CONTEXT_CANCELED)),
        );
    }
    let program = match look_path(name) {
        Ok(p) => p,
        Err(e) => return (Vec::new(), Err(ExecError::new(e))),
    };
    let (mut reader, writer) = match std::io::pipe() {
        Ok(p) => p,
        Err(e) => return (Vec::new(), Err(ExecError::new(e.to_string()))),
    };
    let writer2 = match writer.try_clone() {
        Ok(w) => w,
        Err(e) => return (Vec::new(), Err(ExecError::new(e.to_string()))),
    };
    let mut command = Command::new(program);
    #[cfg(unix)]
    std::os::unix::process::CommandExt::arg0(&mut command, name);
    command
        .args(args)
        .stdin(Stdio::null())
        .stdout(writer)
        .stderr(writer2);
    let spawned = command.spawn();
    drop(command);
    let mut child = match spawned {
        Ok(c) => c,
        Err(e) => {
            return (
                Vec::new(),
                Err(ExecError::new(format!("fork/exec {name}: {e}"))),
            );
        }
    };

    let (tx, rx) = mpsc::channel::<Vec<u8>>();
    std::thread::spawn(move || {
        let mut buf = [0u8; 8192];
        loop {
            match reader.read(&mut buf) {
                Ok(0) | Err(_) => break,
                Ok(n) => {
                    if tx.send(buf[..n].to_vec()).is_err() {
                        break;
                    }
                }
            }
        }
    });

    // Returning drops `rx`: what an orphaned grandchild may still write past `WaitDelay` is discarded.
    supervise(cancel, &mut child, &rx, writers, wait_delay)
}

/// `exec.ErrWaitDelay`.
const ERR_WAIT_DELAY: &str = "exec: WaitDelay expired before I/O complete";

/// How often the child, its output and the context are polled.
const POLL_INTERVAL: Duration = Duration::from_millis(20);

/// `command.Wait()` of a command started with `Cancel` sending SIGINT and `WaitDelay` set to `wait_delay`
/// (`os/exec`'s `watchCtx` and `awaitGoroutines`):
/// - when the context is done while the process runs, it is interrupted;
/// - `wait_delay` after the process exited or the context was done, whichever comes first, a still running
///   process is killed and the output pipe is abandoned;
/// - the error is the exit error of an unsuccessful process, else `context canceled` once interrupted, else
///   `ErrWaitDelay` when the pipe had to be abandoned.
fn supervise(
    cancel: Cancel,
    child: &mut std::process::Child,
    rx: &mpsc::Receiver<Vec<u8>>,
    writers: &[Sink],
    wait_delay: Duration,
) -> RunResult {
    let mut output = Vec::new();
    let mut pipe_open = true;
    let mut exited: Option<ExitStatus> = None;
    let mut interrupted = false;
    let mut killed = false;
    let mut wait_delay_start: Option<Instant> = None;
    let mut wait_delay_expired = false;
    loop {
        if pipe_open {
            match rx.recv_timeout(POLL_INTERVAL) {
                Ok(chunk) => {
                    output.extend_from_slice(&chunk);
                    for w in writers {
                        let _ = w.borrow_mut().write_all(&chunk);
                    }
                }
                Err(mpsc::RecvTimeoutError::Disconnected) => pipe_open = false,
                Err(mpsc::RecvTimeoutError::Timeout) => {}
            }
        } else if exited.is_none() {
            std::thread::sleep(POLL_INTERVAL);
        }
        if exited.is_none() {
            match child.try_wait() {
                Ok(Some(status)) => {
                    exited = Some(status);
                    wait_delay_start.get_or_insert_with(Instant::now);
                }
                Ok(None) => {}
                Err(e) => return (output, Err(ExecError::new(e.to_string()))),
            }
        }
        if exited.is_none() && !interrupted && cancel.is_canceled() {
            interrupt(child);
            interrupted = true;
            wait_delay_start.get_or_insert_with(Instant::now);
        }
        if wait_delay_start.is_some_and(|t| t.elapsed() > wait_delay) {
            if exited.is_none() && !killed {
                let _ = child.kill();
                killed = true;
            }
            if pipe_open {
                pipe_open = false;
                wait_delay_expired = true;
            }
        }
        if let Some(status) = exited
            && !pipe_open
        {
            if !status.success() {
                return (output, Err(exit_error(status)));
            }
            if interrupted {
                return (
                    output,
                    Err(ExecError::new(crate::failure::CONTEXT_CANCELED)),
                );
            }
            if wait_delay_expired {
                return (output, Err(ExecError::new(ERR_WAIT_DELAY)));
            }
            return (output, Ok(()));
        }
    }
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;

    pub fn buffer() -> (Rc<RefCell<Vec<u8>>>, Sink) {
        let b = Rc::new(RefCell::new(Vec::new()));
        let s: Sink = b.clone();
        (b, s)
    }

    pub fn text(b: &Rc<RefCell<Vec<u8>>>) -> String {
        String::from_utf8_lossy(&b.borrow()).to_string()
    }

    fn sh(script: &str) -> Vec<String> {
        vec!["-c".to_string(), script.to_string()]
    }

    #[test]
    fn exec_runner_keeps_command_output_in_the_debug_log() {
        let (terminal, out) = buffer();
        let (log, log_sink) = buffer();
        let mut runner = ExecRunner {
            out: Some(out),
            log: Some(log_sink),
            debug: false,
        };

        let (_, err) = runner.run_quiet(
            Cancel::new(),
            "/bin/sh",
            &sh("printf diagnostic >&2; exit 7"),
        );

        assert!(err.is_err());
        assert!(text(&terminal).contains("diagnostic"));
        assert!(text(&log).contains("$ /bin/sh -c"));
        assert!(text(&log).contains("diagnostic"));
    }

    #[test]
    fn exec_runner_streams_successful_commands_in_debug_mode() {
        let (terminal, out) = buffer();
        let (log, log_sink) = buffer();
        let mut runner = ExecRunner {
            out: Some(out),
            log: Some(log_sink),
            debug: true,
        };

        let (_, err) = runner.run_quiet(Cancel::new(), "/bin/sh", &sh("printf diagnostic"));

        assert!(err.is_ok());
        assert!(text(&terminal).contains("$ /bin/sh -c"));
        assert!(text(&terminal).contains("diagnostic"));
        assert!(text(&log).contains("diagnostic"));
    }

    #[test]
    fn exec_runner_keeps_unlogged_output_out_of_the_debug_log() {
        let (terminal, out) = buffer();
        let (log, log_sink) = buffer();
        let mut runner = ExecRunner {
            out: Some(out),
            log: Some(log_sink),
            debug: true,
        };

        let (output, err) = runner.run_unlogged(
            Cancel::new(),
            "/bin/sh",
            &sh("printf 'K3S_%s=%s' TOKEN ok42"),
        );
        assert!(err.is_ok());
        let (_, err) = runner.run_unlogged(
            Cancel::new(),
            "/bin/sh",
            &sh("printf 'K3S_%s=%s' TOKEN ko42 >&2; exit 7"),
        );
        assert_eq!(Some(7), err.unwrap_err().exit_code);

        assert_eq!("K3S_TOKEN=ok42", String::from_utf8_lossy(&output));
        assert!(text(&log).contains("$ /bin/sh -c printf 'K3S_%s=%s' TOKEN ok42\n"));
        assert!(!text(&log).contains("K3S_TOKEN="));
        assert!(!text(&terminal).contains("K3S_TOKEN=ok42"));
        assert!(text(&terminal).contains("K3S_TOKEN=ko42"));
    }

    #[test]
    fn exec_runner_interrupts_commands_on_cancellation() {
        let cancel = Cancel::new();
        let mut runner = ExecRunner {
            out: None,
            log: None,
            debug: false,
        };
        let flag = cancel.0;
        std::thread::spawn(move || {
            std::thread::sleep(Duration::from_millis(200));
            flag.store(true, Ordering::SeqCst);
        });

        let (output, _) = runner.run_quiet(
            cancel,
            "/bin/sh",
            &sh("trap 'echo interrupted; exit 0' INT; sleep 5 >/dev/null 2>&1 & wait"),
        );

        assert_eq!("interrupted\n", String::from_utf8_lossy(&output));
    }

    /// A context canceled 200ms from now.
    fn cancel_soon() -> Cancel {
        let cancel = Cancel::new();
        let flag = cancel.0;
        std::thread::spawn(move || {
            std::thread::sleep(Duration::from_millis(200));
            flag.store(true, Ordering::SeqCst);
        });
        cancel
    }

    #[test]
    fn run_command_interrupts_commands_that_keep_writing() {
        let started = Instant::now();
        let (output, err) = run_command(
            cancel_soon(),
            "/bin/sh",
            &sh("trap 'exit 0' INT; while :; do echo spam; sleep 0.005; done"),
            &[],
        );

        assert!(!output.is_empty());
        assert_eq!(Err(ExecError::new(crate::failure::CONTEXT_CANCELED)), err);
        assert!(started.elapsed() < Duration::from_secs(5));
    }

    #[test]
    fn run_command_interrupts_commands_that_closed_their_output() {
        let started = Instant::now();
        let (_, err) = run_command(
            cancel_soon(),
            "/bin/sh",
            &sh("exec >/dev/null 2>&1; trap 'exit 0' INT; while :; do sleep 0.05; done"),
            &[],
        );

        assert_eq!(Err(ExecError::new(crate::failure::CONTEXT_CANCELED)), err);
        assert!(started.elapsed() < Duration::from_secs(5));
    }

    #[test]
    fn run_command_kills_commands_still_running_after_wait_delay() {
        let (_, err) = run_command_with_wait_delay(
            cancel_soon(),
            "/bin/sh",
            &sh("trap '' INT; sleep 5"),
            &[],
            Duration::from_millis(200),
        );

        assert_eq!(Some(-1), err.unwrap_err().exit_code);
    }

    #[test]
    fn run_command_reports_wait_delay_when_output_outlives_a_successful_command() {
        let started = Instant::now();
        let (output, err) = run_command_with_wait_delay(
            Cancel::new(),
            "/bin/sh",
            &sh("echo done; sleep 5 & exit 0"),
            &[],
            Duration::from_millis(200),
        );

        assert_eq!("done\n", String::from_utf8_lossy(&output));
        assert_eq!(Err(ExecError::new(ERR_WAIT_DELAY)), err);
        assert!(started.elapsed() < Duration::from_secs(4));
    }

    #[test]
    fn run_command_prefers_the_exit_error_over_wait_delay() {
        let (_, err) = run_command_with_wait_delay(
            Cancel::new(),
            "/bin/sh",
            &sh("sleep 5 & exit 3"),
            &[],
            Duration::from_millis(200),
        );

        assert_eq!(Some(3), err.unwrap_err().exit_code);
    }

    #[test]
    fn exec_runner_run_silent_keeps_the_terminal_and_the_log_clean() {
        let (terminal, out) = buffer();
        let (log, log_sink) = buffer();
        let mut runner = ExecRunner {
            out: Some(out),
            log: Some(log_sink),
            debug: true,
        };

        let (output, err) = runner.run_silent(
            Cancel::new(),
            "/bin/sh",
            &sh("printf out; printf err >&2; exit 7"),
        );

        assert_eq!(Some(7), err.unwrap_err().exit_code);
        assert_eq!("outerr", String::from_utf8_lossy(&output));
        assert_eq!("", text(&terminal));
        assert_eq!(
            "$ /bin/sh -c printf out; printf err >&2; exit 7\n",
            text(&log)
        );
    }

    #[test]
    fn exec_runner_streams_action_commands() {
        let (terminal, out) = buffer();
        let (log, log_sink) = buffer();
        let mut runner = ExecRunner {
            out: Some(out),
            log: Some(log_sink),
            debug: false,
        };

        let (_, err) = runner.run(Cancel::new(), "/bin/sh", &sh("printf diagnostic"));

        assert!(err.is_ok());
        assert!(text(&terminal).contains("$ /bin/sh -c"));
        assert!(text(&terminal).contains("diagnostic"));
        assert!(text(&log).contains("diagnostic"));
    }
}
