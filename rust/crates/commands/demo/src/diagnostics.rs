//! Port of `cmd/demo_up_diagnostics.go`: the Kubernetes diagnostic appended to the debug log after a failed
//! phase, its redaction, and the node disk pressure read.

use crate::api::DEMO_REDACTED_VALUE;
use crate::failure::DemoError;
use crate::local::{LocalCommands, command_failed};
use crate::runner::{Cancel, CommandRunner, ExecError, write_to};
use chrono::{DateTime, Utc};
use regex::Regex;
use std::sync::LazyLock;
use std::time::{Duration, Instant};

pub(crate) const DEMO_DIAGNOSTIC_REQUEST_TIMEOUT: &str = "10s";
pub(crate) const DEMO_DIAGNOSTIC_COMMAND_TIMEOUT: Duration = Duration::from_secs(20);
pub(crate) const DEMO_DIAGNOSTIC_BUDGET: Duration = Duration::from_secs(60);
pub(crate) const DEMO_DIAGNOSTIC_MAX_BYTES: usize = 32 * 1024;
pub(crate) const DEMO_DISK_PRESSURE_TAINT: &str = "node.kubernetes.io/disk-pressure";
pub(crate) const DEMO_OPERATOR_NAMESPACE: &str = "qovery";

/// `demoSecretQuoted`.
const SECRET_QUOTED: &str = concat!(
    // In a JSON string containing JSON, an inner escape starts with two backslashes.
    // Match it as a unit so an escaped inner quote does not end the value.
    r#"(?P<jsonOpen>\\")(?:[^"\\\r\n]|\\\\(?:\\["\\]|[^"\\\r\n])|\\[^"\\\r\n])*(?P<jsonClose>\\")"#,
    r#"|(?P<doubleOpen>")(?:[^"\\]|\\.)*(?P<doubleClose>")"#,
    r"|(?P<singleOpen>')(?:[^'\\]|\\.)*(?P<singleClose>')",
    // A byte limit can leave a quote unclosed. Mask the rest of that line.
    r#"|(?P<unclosed>\\"|["'])[^\r\n]*"#,
);
/// `demoSecretWords`.
const SECRET_WORDS: &str = r"token|secret|password|passwd|jwt|credentials?|api[_-]?key|access[_-]?key|secret[_-]?key|private[_-]?key";
const WS: &str = r"[\t\n\f\r ]";

/// `demoSecretPatterns`.
static SECRET_PATTERNS: LazyLock<Vec<(Regex, String)>> = LazyLock::new(|| {
    let name = format!(r#"[A-Za-z0-9_.-]*(?:{SECRET_WORDS})(?:\\?["'])?"#);
    let quoted = format!(
        "${{1}}${{jsonOpen}}${{doubleOpen}}${{singleOpen}}${{unclosed}}{DEMO_REDACTED_VALUE}${{jsonClose}}${{doubleClose}}${{singleClose}}"
    );
    let re = |pattern: String| Regex::new(&pattern).unwrap();
    vec![
        (
            re(r"eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*".into()),
            DEMO_REDACTED_VALUE.to_string(),
        ),
        (
            re(r#"([A-Za-z][A-Za-z0-9+.-]*://)[^/?#@\t\n\f\r "<>]+@"#.into()),
            format!("${{1}}{DEMO_REDACTED_VALUE}@"),
        ),
        (
            re(format!(
                r#"(?i)((?:authorization|cookie|set-cookie|(?:x-)?api-key)(?:\\?["'])?{WS}*:{WS}*)(?:{SECRET_QUOTED}|[^\r\n]+)"#
            )),
            quoted.clone(),
        ),
        (
            re(format!(r"(?i)(bearer{WS}+)[A-Za-z0-9._~+/=-]+")),
            format!("${{1}}{DEMO_REDACTED_VALUE}"),
        ),
        (
            re(format!(
                r#"(?i)(--[A-Za-z0-9-]*(?:{SECRET_WORDS})(?:=|{WS}+))(?:{SECRET_QUOTED}|[^\t\n\f\r "']+)"#
            )),
            quoted.clone(),
        ),
        (
            re(format!(
                r#"(?i)({name}{WS}*[:=]{WS}*)(?:{SECRET_QUOTED}|[^\t\n\f\r "',;}}]+)"#
            )),
            quoted,
        ),
    ]
});

/// `redactDemoDiagnostic(text)`.
pub(crate) fn redact_demo_diagnostic(text: &str) -> String {
    let mut text = text.to_string();
    for (pattern, replacement) in SECRET_PATTERNS.iter() {
        text = pattern
            .replace_all(&text, replacement.as_str())
            .into_owned();
    }
    text
}

/// `demoValidUTF8(output)`: each invalid byte becomes U+FFFD, as the JSON encoding of the Go upload does.
pub(crate) fn demo_valid_utf8(output: &[u8]) -> String {
    let mut text = String::with_capacity(output.len());
    for chunk in output.utf8_chunks() {
        text.push_str(chunk.valid());
        for _ in chunk.invalid() {
            text.push(char::REPLACEMENT_CHARACTER);
        }
    }
    text
}

/// `demoDiagnosticTail(output, maxLines, maxBytes)`: the last `max_lines` lines of the redacted output, then the
/// last whole lines that fit in `max_bytes` (the end of a single longer line), saying how many bytes were cut.
pub(crate) fn demo_diagnostic_tail(output: &[u8], max_lines: usize, max_bytes: usize) -> String {
    let redacted = redact_demo_diagnostic(&demo_valid_utf8(output));
    let mut lines: Vec<&str> = redacted.trim_end_matches('\n').split('\n').collect();
    if lines.len() > max_lines {
        lines.drain(..lines.len() - max_lines);
    }
    let mut text = lines.join("\n");
    let full = text.len();
    while lines.len() > 1 && text.len() > max_bytes {
        lines.remove(0);
        text = lines.join("\n");
    }
    if text.len() > max_bytes {
        let mut cut = text.len() - max_bytes;
        while cut < text.len() && !text.is_char_boundary(cut) {
            cut += 1;
        }
        text = text[cut..].to_string();
    }
    if full > text.len() {
        text = format!("[truncated: {} bytes cut]\n{text}", full - text.len());
    }
    format!("{text}\n")
}

/// `demoOperatorPod`.
#[derive(Debug, PartialEq)]
struct OperatorPod {
    name: String,
    restarts: i64,
}

/// `demoOperatorPods(pods, namespace)`: reads `kubectl get pods -o wide` lines (NAMESPACE NAME READY STATUS RESTARTS ...).
fn demo_operator_pods(pods: &[u8], namespace: &str) -> Vec<OperatorPod> {
    demo_valid_utf8(pods)
        .split('\n')
        .filter_map(|line| {
            let fields: Vec<&str> = line.split_whitespace().collect();
            (fields.len() > 4
                && fields[0] == namespace
                && fields[1].starts_with("qovery-operator-"))
            .then(|| OperatorPod {
                name: fields[1].to_string(),
                restarts: fields[4].parse().unwrap_or(0),
            })
        })
        .collect()
}

/// `demoUnhealthyPods(pods, namespace)`.
fn demo_unhealthy_pods(pods: &[u8], namespace: &str) -> Vec<u8> {
    let text = demo_valid_utf8(pods);
    text.trim_end_matches('\n')
        .split('\n')
        .enumerate()
        .filter(|(index, line)| {
            let fields: Vec<&str> = line.split_whitespace().collect();
            *index == 0
                || fields.len() < 4
                || fields[0] == namespace
                || (fields[3] != "Running" && fields[3] != "Completed")
                || (fields[3] != "Completed" && demo_pod_not_ready(fields[2]))
        })
        .map(|(_, line)| line)
        .collect::<Vec<_>>()
        .join("\n")
        .into_bytes()
}

/// `demoPodNotReady(column)`: `a/b` with a < b.
fn demo_pod_not_ready(column: &str) -> bool {
    let Some((ready, total)) = column.split_once('/') else {
        return false;
    };
    matches!((ready.parse::<i64>(), total.parse::<i64>()), (Ok(r), Ok(t)) if r < t)
}

/// Outcome of a diagnostic kubectl: output, whether its own deadline ended it, result.
type KubectlResult = (Vec<u8>, bool, Result<(), ExecError>);

impl<R: CommandRunner> LocalCommands<R> {
    /// `kubectl(ctx, timeout, args...)`.
    fn kubectl(&mut self, cancel: Cancel, timeout: Duration, extra: &[&str]) -> KubectlResult {
        let mut args = self.kubectl_target();
        args.extend([
            "--request-timeout".to_string(),
            DEMO_DIAGNOSTIC_REQUEST_TIMEOUT.to_string(),
        ]);
        args.extend(extra.iter().map(|a| a.to_string()));
        let started_at = Instant::now();
        let (output, result) = self.runner.run_silent(cancel, timeout, "kubectl", &args);
        let timed_out = result.is_err() && !cancel.is_canceled() && started_at.elapsed() >= timeout;
        (output, timed_out, result)
    }

    /// `collectDiagnostics(ctx, namespace, budget)`: appends a bounded, redacted view of the cluster to the
    /// debug log; it never fails.
    pub(crate) fn collect_kubernetes_diagnostics(
        &mut self,
        cancel: Cancel,
        namespace: &str,
        budget: Duration,
    ) {
        let deadline = Instant::now() + budget;
        let mut remaining_bytes = DEMO_DIAGNOSTIC_MAX_BYTES;
        let mut stopped = false;
        let log = self.debug_log.clone();
        write_to(&log, "\n===== Kubernetes diagnostics =====\n");
        let mut run = |l: &mut Self,
                       title: &str,
                       max_lines: usize,
                       keep: &dyn Fn(&[u8]) -> Vec<u8>,
                       args: &[&str]| {
            let remaining = deadline.saturating_duration_since(Instant::now());
            if stopped {
                return Vec::new();
            }
            if !cancel.is_canceled() && remaining.is_zero() {
                stopped = true;
                write_to(&log, "diagnostic time budget exhausted\n");
                return Vec::new();
            }
            if remaining_bytes == 0 {
                stopped = true;
                write_to(&log, "diagnostic size limit reached\n");
                return Vec::new();
            }
            let (output, timed_out, result) =
                l.kubectl(cancel, remaining.min(DEMO_DIAGNOSTIC_COMMAND_TIMEOUT), args);
            if cancel.is_canceled() {
                stopped = true;
                write_to(&log, "diagnostic interrupted\n");
                return Vec::new();
            }
            if timed_out {
                write_to(&log, &format!("{title} timed out\n"));
                return Vec::new();
            }
            if let Err(e) = result {
                write_to(&log, &format!("{}\n", command_failed(title, &e)));
                return Vec::new();
            }
            let section = demo_diagnostic_tail(&keep(&output), max_lines, remaining_bytes);
            remaining_bytes = remaining_bytes.saturating_sub(section.len());
            write_to(&log, &format!("--- {title} ---\n{section}"));
            output
        };
        let all = |output: &[u8]| output.to_vec();
        run(
            self,
            "kubectl get nodes",
            20,
            &all,
            &[
                "get",
                "nodes",
                "--output",
                r#"custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,DISK_PRESSURE:.status.conditions[?(@.type=="DiskPressure")].status,MEMORY_PRESSURE:.status.conditions[?(@.type=="MemoryPressure")].status,PID_PRESSURE:.status.conditions[?(@.type=="PIDPressure")].status,TAINTS:.spec.taints[*].key"#,
            ],
        );
        let pods = run(
            self,
            "kubectl get pods",
            100,
            &|output| demo_unhealthy_pods(output, namespace),
            &["get", "pods", "--all-namespaces", "--output", "wide"],
        );
        run(
            self,
            "kubectl get events",
            60,
            &all,
            &[
                "--namespace",
                namespace,
                "get",
                "events",
                "--sort-by",
                ".metadata.creationTimestamp",
            ],
        );
        for pod in demo_operator_pods(&pods, namespace) {
            let logs = [
                "--namespace",
                namespace,
                "logs",
                &pod.name,
                "--tail=100",
                "--limit-bytes=32768",
            ];
            run(
                self,
                &format!("kubectl logs {}", pod.name),
                100,
                &all,
                &logs,
            );
            if pod.restarts > 0 {
                let previous = [&logs[..], &["--previous"]].concat();
                run(
                    self,
                    &format!("kubectl logs {} --previous", pod.name),
                    100,
                    &all,
                    &previous,
                );
            }
        }
        write_to(&log, "===== end of Kubernetes diagnostics =====\n");
    }

    /// `NodeDiskPressure(ctx, timeout)`: the first node under disk pressure and the creation time of the newest
    /// node.
    pub(crate) fn read_node_disk_pressure(
        &mut self,
        cancel: Cancel,
        timeout: Duration,
    ) -> Result<(String, DateTime<Utc>), DemoError> {
        let (output, timed_out, result) = self.kubectl(cancel, timeout, &[
            "get",
            "nodes",
            "--output",
            r#"jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.creationTimestamp}{"\t"}{.spec.taints[*].key}{"\t"}{.status.conditions[?(@.type=="DiskPressure")].status}{"\n"}{end}"#,
        ]);
        let failure = match (timed_out, result) {
            (true, _) => Some(DemoError::new("kubectl get nodes timed out")),
            (false, Err(e)) => Some(command_failed("kubectl get nodes", &e)),
            (false, Ok(())) => None,
        };
        if let Some(err) = failure {
            write_to(&self.debug_log, &format!("{err}\n"));
            return Err(err);
        }
        let (mut pressure, mut newest, mut read) = (String::new(), DateTime::<Utc>::MIN_UTC, false);
        for line in demo_valid_utf8(&output).split('\n') {
            let fields: Vec<&str> = line.split('\t').collect();
            if fields.len() != 4 {
                continue;
            }
            let Ok(created_at) = DateTime::parse_from_rfc3339(fields[1]) else {
                continue;
            };
            read = true;
            newest = newest.max(created_at.with_timezone(&Utc));
            let tainted = fields[2]
                .split_whitespace()
                .any(|t| t == DEMO_DISK_PRESSURE_TAINT);
            if pressure.is_empty() && (fields[3] == "True" || tainted) {
                pressure = fields[0].to_string();
            }
        }
        if !read {
            let err = DemoError::new("kubectl get nodes returned no readable node");
            write_to(&self.debug_log, &format!("{err}\n"));
            return Err(err);
        }
        Ok((pressure, newest))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runner::RunResult;
    use crate::runner::tests::{buffer, text};
    use std::cell::RefCell;
    use std::rc::Rc;

    const JWT: &str = "eyJhbGciOiJIUzI1NiJ9.eyJjbHVzdGVyIjoiZGVtbyJ9.c2lnbmF0dXJl";
    const KUBECTL: &str = "silent: kubectl --context k3d-local-demo-user --request-timeout 10s ";

    type Respond = Box<dyn Fn(Cancel, Duration, &str) -> RunResult>;

    /// `scriptedDemoRunner`.
    struct ScriptedRunner {
        calls: Rc<RefCell<Vec<String>>>,
        timeouts: Rc<RefCell<Vec<Duration>>>,
        respond: Respond,
    }

    impl ScriptedRunner {
        fn record(
            &mut self,
            cancel: Cancel,
            timeout: Duration,
            mode: &str,
            name: &str,
            args: &[String],
        ) -> RunResult {
            let command = std::iter::once(name.to_string())
                .chain(args.iter().cloned())
                .collect::<Vec<_>>()
                .join(" ");
            self.calls.borrow_mut().push(format!("{mode}: {command}"));
            (self.respond)(cancel, timeout, &command)
        }
    }

    impl CommandRunner for ScriptedRunner {
        fn look_path(&mut self, _: &str) -> Result<(), String> {
            Ok(())
        }
        fn run(&mut self, c: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record(c, Duration::ZERO, "run", name, args)
        }
        fn run_quiet(&mut self, c: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record(c, Duration::ZERO, "quiet", name, args)
        }
        fn run_unlogged(&mut self, c: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record(c, Duration::ZERO, "unlogged", name, args)
        }
        fn run_silent(
            &mut self,
            c: Cancel,
            timeout: Duration,
            name: &str,
            args: &[String],
        ) -> RunResult {
            self.timeouts.borrow_mut().push(timeout);
            self.record(c, timeout, "silent", name, args)
        }
    }

    struct Test {
        local: LocalCommands<ScriptedRunner>,
        calls: Rc<RefCell<Vec<String>>>,
        timeouts: Rc<RefCell<Vec<Duration>>>,
        log: Rc<RefCell<Vec<u8>>>,
    }

    fn local(respond: impl Fn(Cancel, Duration, &str) -> RunResult + 'static) -> Test {
        let calls = Rc::new(RefCell::new(Vec::new()));
        let timeouts = Rc::new(RefCell::new(Vec::new()));
        let (log, sink) = buffer();
        let mut local = LocalCommands::new(
            ScriptedRunner {
                calls: calls.clone(),
                timeouts: timeouts.clone(),
                respond: Box::new(respond),
            },
            "linux",
            sink,
        );
        local.kube_context = "k3d-local-demo-user".into();
        Test {
            local,
            calls,
            timeouts,
            log,
        }
    }

    fn ok(output: &str) -> RunResult {
        (output.as_bytes().to_vec(), Ok(()))
    }

    fn failed(output: &str) -> RunResult {
        (
            output.as_bytes().to_vec(),
            Err(ExecError {
                exit_code: Some(1),
                message: "exit status 1".into(),
            }),
        )
    }

    const PODS: &str = concat!(
        "NAMESPACE     NAME                             READY   STATUS             RESTARTS      AGE   IP          NODE\n",
        "kube-system   coredns-1                        1/1     Running            0             1h    10.0.0.2    server-0\n",
        "ingress       ingress-1                        0/1     ImagePullBackOff   0             1h    10.0.0.3    server-0\n",
        "ingress       ingress-2                        1/2     Running            0             1h    10.0.0.7    server-0\n",
        "jobs          migrate-1                        0/1     Completed          0             1h    10.0.0.4    server-0\n",
        "qovery-ops    qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4 (30s ago)   9m    10.0.0.5    server-0\n",
        "qovery-ops    qovery-helper-1                  1/1     Running            0             9m    10.0.0.6    server-0\n",
    );

    #[test]
    fn collect_diagnostics_writes_a_redacted_bounded_view() {
        let mut t = local(|_, _, command| {
            if command.contains(" get nodes ") {
                ok(
                    "NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS\nserver-0   True    True            False             False          node.kubernetes.io/disk-pressure\n",
                )
            } else if command.contains(" get pods ") {
                ok(PODS)
            } else if command.contains(" get events ") {
                ok(
                    "LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE\n10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container\n",
                )
            } else if command.ends_with(" --previous") {
                ok("panic: password=abc\\def\n")
            } else if command.contains(" logs ") {
                ok(&format!(
                    "starting\nconnecting with Authorization: Bearer opaque-value\njwt={JWT}\n"
                ))
            } else {
                ok("")
            }
        });

        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery-ops", DEMO_DIAGNOSTIC_BUDGET);

        assert_eq!(
            vec![
                format!(
                    "{KUBECTL}get nodes --output custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type==\"Ready\")].status,DISK_PRESSURE:.status.conditions[?(@.type==\"DiskPressure\")].status,MEMORY_PRESSURE:.status.conditions[?(@.type==\"MemoryPressure\")].status,PID_PRESSURE:.status.conditions[?(@.type==\"PIDPressure\")].status,TAINTS:.spec.taints[*].key"
                ),
                format!("{KUBECTL}get pods --all-namespaces --output wide"),
                format!(
                    "{KUBECTL}--namespace qovery-ops get events --sort-by .metadata.creationTimestamp"
                ),
                format!(
                    "{KUBECTL}--namespace qovery-ops logs qovery-operator-7d9c-x2x --tail=100 --limit-bytes=32768"
                ),
                format!(
                    "{KUBECTL}--namespace qovery-ops logs qovery-operator-7d9c-x2x --tail=100 --limit-bytes=32768 --previous"
                ),
            ],
            *t.calls.borrow()
        );
        assert_eq!(5, t.timeouts.borrow().len());
        for timeout in t.timeouts.borrow().iter() {
            assert!(*timeout <= DEMO_DIAGNOSTIC_COMMAND_TIMEOUT);
            assert!(*timeout > DEMO_DIAGNOSTIC_COMMAND_TIMEOUT - Duration::from_secs(1));
        }
        assert_eq!(
            r#"
===== Kubernetes diagnostics =====
--- kubectl get nodes ---
NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS
server-0   True    True            False             False          node.kubernetes.io/disk-pressure
--- kubectl get pods ---
NAMESPACE     NAME                             READY   STATUS             RESTARTS      AGE   IP          NODE
ingress       ingress-1                        0/1     ImagePullBackOff   0             1h    10.0.0.3    server-0
ingress       ingress-2                        1/2     Running            0             1h    10.0.0.7    server-0
qovery-ops    qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4 (30s ago)   9m    10.0.0.5    server-0
qovery-ops    qovery-helper-1                  1/1     Running            0             9m    10.0.0.6    server-0
--- kubectl get events ---
LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE
10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container
--- kubectl logs qovery-operator-7d9c-x2x ---
starting
connecting with Authorization: <redacted>
jwt=<redacted>
--- kubectl logs qovery-operator-7d9c-x2x --previous ---
panic: password=<redacted>
===== end of Kubernetes diagnostics =====
"#,
            text(&t.log)
        );
    }

    #[test]
    fn collect_diagnostics_logs_failures_and_skips_operator_logs_without_an_operator_pod() {
        let mut t = local(|_, _, command| {
            if command.contains(" get nodes ") {
                failed("The connection to the server was refused")
            } else {
                ok(&"line\n".repeat(500))
            }
        });

        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery", DEMO_DIAGNOSTIC_BUDGET);

        assert_eq!(3, t.calls.borrow().len());
        assert_eq!(
            format!(
                "\n===== Kubernetes diagnostics =====\nkubectl get nodes failed with exit code 1\n--- kubectl get pods ---\n{}--- kubectl get events ---\n{}===== end of Kubernetes diagnostics =====\n",
                "line\n".repeat(100),
                "line\n".repeat(60)
            ),
            text(&t.log)
        );
    }

    #[test]
    fn collect_diagnostics_stays_within_its_budget() {
        let mut t = local(|_, timeout, _| {
            std::thread::sleep(timeout);
            failed("")
        });

        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery", Duration::from_millis(100));

        assert_eq!(1, t.calls.borrow().len());
        assert!(t.timeouts.borrow()[0] <= Duration::from_millis(100));
        assert_eq!(
            "\n===== Kubernetes diagnostics =====\nkubectl get nodes timed out\ndiagnostic time budget exhausted\n===== end of Kubernetes diagnostics =====\n",
            text(&t.log)
        );

        t.calls.borrow_mut().clear();
        t.log.borrow_mut().clear();
        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery", Duration::ZERO);
        assert!(t.calls.borrow().is_empty());
        assert!(text(&t.log).contains("\ndiagnostic time budget exhausted\n===== end"));
    }

    #[test]
    fn collect_diagnostics_stops_cleanly_when_interrupted() {
        let mut t = local(|cancel, _, command| {
            if command.contains(" get pods ") {
                cancel.cancel();
                failed("")
            } else {
                ok("ok\n")
            }
        });

        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery", DEMO_DIAGNOSTIC_BUDGET);

        assert_eq!(2, t.calls.borrow().len());
        assert_eq!(
            "\n===== Kubernetes diagnostics =====\n--- kubectl get nodes ---\nok\ndiagnostic interrupted\n===== end of Kubernetes diagnostics =====\n",
            text(&t.log)
        );
    }

    #[test]
    fn tail_caps_lines_and_bytes_after_redaction() {
        assert_eq!("c\nd\n", demo_diagnostic_tail(b"a\nb\nc\nd\n", 2, 100));
        assert_eq!(
            "[truncated: 4 bytes cut]\nc\nd\n",
            demo_diagnostic_tail(b"a\nb\nc\nd\n", 10, 3)
        );
        assert_eq!(
            format!("[truncated: 253952 bytes cut]\n{}\n", "y".repeat(32 * 1024)),
            demo_diagnostic_tail("y".repeat(280 * 1024).as_bytes(), 100, 32 * 1024)
        );
        assert_eq!(
            format!("[truncated: 3 bytes cut]\n{}\n", "y".repeat(99)),
            demo_diagnostic_tail(format!("aé{}", "y".repeat(99)).as_bytes(), 1, 100)
        );
        assert_eq!(
            "token=<redacted>\n",
            demo_diagnostic_tail(format!("token={}", "s".repeat(1000)).as_bytes(), 1, 100)
        );
    }

    #[test]
    fn tail_replaces_each_invalid_byte() {
        assert_eq!(
            "a\u{FFFD}\u{FFFD}b\u{FFFD}c\u{FFFD}\u{FFFD}\u{FFFD}d\n",
            demo_diagnostic_tail(b"a\xe2\x82b\xffc\xed\xa0\x80d", 10, 100)
        );
    }

    #[test]
    fn collect_diagnostics_shares_one_size_limit() {
        let line = "x".repeat(299);
        let mut t = local(move |_, _, command| {
            let mut output = format!("{line}\n").repeat(200);
            if command.contains(" get pods ") {
                output.push_str("qovery qovery-operator-1 0/1 CrashLoopBackOff 3 1m\n");
            }
            ok(&output)
        });

        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery", DEMO_DIAGNOSTIC_BUDGET);

        assert_eq!(2, t.calls.borrow().len());
        assert!(text(&t.log).len() <= DEMO_DIAGNOSTIC_MAX_BYTES + 512);
        assert!(text(&t.log).contains("--- kubectl get pods ---\n[truncated: 3000 bytes cut]\n"));
        assert!(text(&t.log).ends_with(
            "\ndiagnostic size limit reached\n===== end of Kubernetes diagnostics =====\n"
        ));
    }

    #[test]
    fn redacts_secrets() {
        for (input, expected) in [
            (format!("jwt {JWT} end"), "jwt <redacted> end"),
            (
                "Authorization: Bearer abc.DEF-123".into(),
                "Authorization: <redacted>",
            ),
            (
                "authorization: Basic dXNlcjpwYXNz trailing".into(),
                "authorization: <redacted>",
            ),
            (
                r#"{"Authorization":"Token abc","level":"info"}"#.into(),
                r#"{"Authorization":"<redacted>","level":"info"}"#,
            ),
            ("X-Api-Key: abc def".into(), "X-Api-Key: <redacted>"),
            ("Api-Key: abc".into(), "Api-Key: <redacted>"),
            (
                "Cookie: session=abc; theme=dark".into(),
                "Cookie: <redacted>",
            ),
            (
                "calling with Bearer abc.def".into(),
                "calling with Bearer <redacted>",
            ),
            (
                "pulling https://robot:s3cret@registry.example/v2/ failed".into(),
                "pulling https://<redacted>@registry.example/v2/ failed",
            ),
            (
                "redis://:opaque-password@cache.example:6379/0".into(),
                "redis://<redacted>@cache.example:6379/0",
            ),
            (
                "https://opaque-token@api.example/path".into(),
                "https://<redacted>@api.example/path",
            ),
            (
                "https://api.example/path/user@example".into(),
                "https://api.example/path/user@example",
            ),
            (
                "kubectl --token abc get pods".into(),
                "kubectl --token <redacted> get pods",
            ),
            (
                "kubectl --token \"opaque-token".into(),
                "kubectl --token \"<redacted>",
            ),
            (
                "helm --password='opaque-password\r\nnext entry".into(),
                "helm --password='<redacted>\r\nnext entry",
            ),
            (
                r#"helm --password "a b" --namespace x"#.into(),
                r#"helm --password "<redacted>" --namespace x"#,
            ),
            ("--api-key=k1 next".into(), "--api-key=<redacted> next"),
            (
                r#"{"token":"abc","level":"info"}"#.into(),
                r#"{"token":"<redacted>","level":"info"}"#,
            ),
            (
                r#"{\"token\":\"abc def\",\"level\":\"info\"}"#.into(),
                r#"{\"token\":\"<redacted>\",\"level\":\"info\"}"#,
            ),
            (
                r#"{"message":"{\"password\":\"abc\\\"opaque-suffix\"}","level":"info"}"#.into(),
                r#"{"message":"{\"password\":\"<redacted>\"}","level":"info"}"#,
            ),
            (
                r#"{"message":"{\"password\":\"abc\\\\\"}","level":"info"}"#.into(),
                r#"{"message":"{\"password\":\"<redacted>\"}","level":"info"}"#,
            ),
            (
                r#"password="abc\"SECRET" user=admin"#.into(),
                r#"password="<redacted>" user=admin"#,
            ),
            (r"password=abc\def next".into(), "password=<redacted> next"),
            (r#"token="abc def"#.into(), r#"token="<redacted>"#),
            (
                r#"password="abc'opaque-suffix"#.into(),
                r#"password="<redacted>"#,
            ),
            (
                r#"secretToken: "my pass word" next"#.into(),
                r#"secretToken: "<redacted>" next"#,
            ),
            ("password: hunter2 next".into(), "password: <redacted> next"),
            (
                "refresh token: context deadline exceeded".into(),
                "refresh token: <redacted> deadline exceeded",
            ),
            ("token:abc next".into(), "token:<redacted> next"),
            (
                "AWS_SECRET_ACCESS_KEY=AKIA/def+1 next".into(),
                "AWS_SECRET_ACCESS_KEY=<redacted> next",
            ),
            ("api-key = k1; next".into(), "api-key = <redacted>; next"),
            (
                "private_key='-----BEGIN KEY-----'".into(),
                "private_key='<redacted>'",
            ),
            (
                "client_credentials=xyz".into(),
                "client_credentials=<redacted>",
            ),
            (
                "secret \"qovery-operator\" not found".into(),
                "secret \"qovery-operator\" not found",
            ),
            (
                "tokenRefreshInterval=5s monkey=3".into(),
                "tokenRefreshInterval=5s monkey=3",
            ),
        ] {
            assert_eq!(expected, redact_demo_diagnostic(&input), "{input}");
        }
    }

    #[test]
    fn node_disk_pressure_reads_taints_and_conditions() {
        let created = chrono::TimeZone::with_ymd_and_hms(&Utc, 2026, 8, 19, 9, 0, 0).unwrap();
        for (output, node) in [
            (
                "server-0\t2026-08-19T09:00:00Z\tnode.kubernetes.io/disk-pressure\tFalse\n",
                "server-0",
            ),
            (
                "agent-0\t2026-08-19T08:00:00Z\t\tFalse\nserver-0\t2026-08-19T09:00:00Z\tnode-role.kubernetes.io/control-plane\tTrue\n",
                "server-0",
            ),
            (
                "W1123 warning on stderr\nserver-0\t2026-08-19T09:00:00Z\tnode.kubernetes.io/memory-pressure\tFalse\n",
                "",
            ),
        ] {
            let mut t = local(move |_, _, _| ok(output));
            assert_eq!(
                (node.to_string(), created),
                t.local
                    .read_node_disk_pressure(Cancel::new(), Duration::from_secs(7))
                    .unwrap()
            );
            assert_eq!(
                vec![format!(
                    "{KUBECTL}get nodes --output jsonpath={{range .items[*]}}{{.metadata.name}}{{\"\\t\"}}{{.metadata.creationTimestamp}}{{\"\\t\"}}{{.spec.taints[*].key}}{{\"\\t\"}}{{.status.conditions[?(@.type==\"DiskPressure\")].status}}{{\"\\n\"}}{{end}}"
                )],
                *t.calls.borrow()
            );
            assert_eq!(vec![Duration::from_secs(7)], *t.timeouts.borrow());
        }
        let mut t = local(|_, _, _| failed("refused"));
        assert!(
            t.local
                .read_node_disk_pressure(Cancel::new(), Duration::from_secs(1))
                .is_err()
        );
        assert_eq!("kubectl get nodes failed with exit code 1\n", text(&t.log));

        let mut t = local(|_, timeout, _| {
            std::thread::sleep(timeout);
            failed("")
        });
        assert!(
            t.local
                .read_node_disk_pressure(Cancel::new(), Duration::from_millis(50))
                .is_err()
        );
        assert_eq!("kubectl get nodes timed out\n", text(&t.log));
    }

    #[test]
    fn collect_diagnostics_targets_the_given_kubeconfig() {
        let mut t = local(|_, _, _| ok(""));
        t.local.kube_context = "vcluster_qls-a".into();
        t.local.kubeconfig = "/kube/qls-a.yaml".into();

        t.local
            .collect_kubernetes_diagnostics(Cancel::new(), "qovery", DEMO_DIAGNOSTIC_BUDGET);

        assert!(!t.calls.borrow().is_empty());
        for call in t.calls.borrow().iter() {
            assert!(
                call.starts_with("silent: kubectl --context vcluster_qls-a --kubeconfig /kube/qls-a.yaml --request-timeout 10s "),
                "{call}"
            );
        }
    }
}
