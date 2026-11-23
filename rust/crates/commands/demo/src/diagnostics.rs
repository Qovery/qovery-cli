//! Port of `cmd/demo_up_diagnostics.go`: the Kubernetes diagnostic appended to the debug log after a failed
//! phase, its redaction, and the node disk pressure read.

use crate::api::DEMO_REDACTED_VALUE;
use crate::failure::DemoError;
use crate::local::{LocalCommands, command_failed};
use crate::runner::{Cancel, CommandRunner, write_to};
use chrono::{DateTime, Utc};
use regex::Regex;
use std::sync::LazyLock;
use std::time::{Duration, Instant};

pub(crate) const DEMO_DIAGNOSTIC_REQUEST_TIMEOUT: &str = "10s";
pub(crate) const DEMO_DIAGNOSTIC_BUDGET: Duration = Duration::from_secs(60);
pub(crate) const DEMO_DIAGNOSTIC_MAX_BYTES: usize = 32 * 1024;
pub(crate) const DEMO_DISK_PRESSURE_TAINT: &str = "node.kubernetes.io/disk-pressure";

/// `demoSecretQuoted` (Go's `\s` is `[\t\n\f\r ]`).
const SECRET_QUOTED: &str = r#"(\\")(?:[^"\\]|\\[^"])*\\"|(")(?:[^"\\]|\\.)*"|(')(?:[^'\\]|\\.)*'"#;
/// `demoSecretName`.
const SECRET_NAME: &str = r#"[A-Za-z0-9_.-]*(?:token|secret|password|passwd|jwt|credentials?|api[_-]?key|access[_-]?key|secret[_-]?key|private[_-]?key)(?:\\?["'])?"#;
const WS: &str = r"[\t\n\f\r ]";

/// `demoSecretPatterns`.
static SECRET_PATTERNS: LazyLock<Vec<(Regex, String)>> = LazyLock::new(|| {
    let quoted = format!("${{1}}${{2}}${{3}}${{4}}{DEMO_REDACTED_VALUE}${{2}}${{3}}${{4}}");
    let re = |pattern: String| Regex::new(&pattern).unwrap();
    vec![
        (
            re(r"eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*".into()),
            DEMO_REDACTED_VALUE.to_string(),
        ),
        (
            re(r"([A-Za-z][A-Za-z0-9+.-]*://[^/\t\n\f\r :@]+:)[^/\t\n\f\r @]+@".into()),
            format!("${{1}}{DEMO_REDACTED_VALUE}@"),
        ),
        (
            re(format!(
                r#"(?i)(authorization(?:\\?["'])?{WS}*[:=]{WS}*)(?:{SECRET_QUOTED}|[^\r\n]+)"#
            )),
            quoted.clone(),
        ),
        (
            re(format!(r"(?i)(bearer{WS}+)[A-Za-z0-9._~+/=-]+")),
            format!("${{1}}{DEMO_REDACTED_VALUE}"),
        ),
        (
            re(format!(
                r#"(?i)({SECRET_NAME}{WS}*(?:={WS}*|:))(?:{SECRET_QUOTED}|[^\t\n\f\r "',;}}\\]+)"#
            )),
            quoted.clone(),
        ),
        (
            re(format!(
                r"(?i)({SECRET_NAME}{WS}*:{WS}+)(?:{SECRET_QUOTED})"
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

/// `demoDiagnosticTail(output, maxLines)`.
pub(crate) fn demo_diagnostic_tail(output: &[u8], max_lines: usize) -> String {
    let redacted = redact_demo_diagnostic(&String::from_utf8_lossy(output));
    let mut lines: Vec<&str> = redacted.trim_end_matches('\n').split('\n').collect();
    if lines.len() > max_lines {
        lines.drain(..lines.len() - max_lines);
    }
    while lines.len() > 1 && lines.join("\n").len() > DEMO_DIAGNOSTIC_MAX_BYTES {
        lines.remove(0);
    }
    format!("{}\n", lines.join("\n"))
}

/// Keeps part of a command output before it is logged.
type OutputFilter = fn(&[u8]) -> Vec<u8>;

/// `demoOperatorPodExists(pods)`.
fn demo_operator_pod_exists(pods: &[u8]) -> bool {
    String::from_utf8_lossy(pods).split('\n').any(|line| {
        let fields: Vec<&str> = line.split_whitespace().collect();
        fields.len() > 1 && fields[0] == "qovery" && fields[1].starts_with("qovery-operator-")
    })
}

/// `demoUnhealthyPods(pods)`.
fn demo_unhealthy_pods(pods: &[u8]) -> Vec<u8> {
    let text = String::from_utf8_lossy(pods);
    text.trim_end_matches('\n')
        .split('\n')
        .enumerate()
        .filter(|(index, line)| {
            let fields: Vec<&str> = line.split_whitespace().collect();
            *index == 0
                || fields.len() < 4
                || fields[0] == "qovery"
                || (fields[3] != "Running" && fields[3] != "Completed")
        })
        .map(|(_, line)| line)
        .collect::<Vec<_>>()
        .join("\n")
        .into_bytes()
}

impl<R: CommandRunner> LocalCommands<R> {
    fn kubectl(&mut self, cancel: Cancel, extra: &[&str]) -> crate::runner::RunResult {
        let mut args = vec![
            "--context".to_string(),
            self.kube_context.clone(),
            "--request-timeout".to_string(),
            DEMO_DIAGNOSTIC_REQUEST_TIMEOUT.to_string(),
        ];
        args.extend(extra.iter().map(|a| a.to_string()));
        self.runner.run_silent(cancel, "kubectl", &args)
    }

    /// `collectDiagnostics(ctx, budget)`: appends a bounded, redacted view of the cluster to the debug log.
    pub(crate) fn collect_kubernetes_diagnostics(&mut self, cancel: Cancel, budget: Duration) {
        let started_at = Instant::now();
        let log = self.debug_log.clone();
        write_to(&log, "\n===== Kubernetes diagnostics =====\n");
        let run = |l: &mut Self,
                   title: &str,
                   max_lines: usize,
                   keep: Option<OutputFilter>,
                   args: &[&str]| {
            if started_at.elapsed() >= budget {
                write_to(
                    &log,
                    &format!("{title} skipped: diagnostic time budget exhausted\n"),
                );
                return Vec::new();
            }
            let (mut output, err) = l.kubectl(cancel, args);
            if let Err(e) = err {
                write_to(&log, &format!("{}\n", command_failed(title, &e)));
                return Vec::new();
            }
            if let Some(keep) = keep {
                output = keep(&output);
            }
            write_to(
                &log,
                &format!(
                    "--- {title} ---\n{}",
                    demo_diagnostic_tail(&output, max_lines)
                ),
            );
            output
        };
        run(
            self,
            "kubectl get nodes",
            20,
            None,
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
            Some(demo_unhealthy_pods),
            &["get", "pods", "--all-namespaces", "--output", "wide"],
        );
        run(
            self,
            "kubectl get events",
            60,
            None,
            &[
                "--namespace",
                "qovery",
                "get",
                "events",
                "--sort-by",
                ".metadata.creationTimestamp",
            ],
        );
        if demo_operator_pod_exists(&pods) {
            run(
                self,
                "kubectl logs",
                100,
                None,
                &[
                    "--namespace",
                    "qovery",
                    "logs",
                    "deploy/qovery-operator",
                    "--tail=100",
                    "--limit-bytes=32768",
                ],
            );
        }
        write_to(&log, "===== end of Kubernetes diagnostics =====\n");
    }

    /// `NodeDiskPressure(ctx)`: the first node under disk pressure and the creation time of the newest node.
    pub(crate) fn read_node_disk_pressure(
        &mut self,
        cancel: Cancel,
    ) -> Result<(String, DateTime<Utc>), DemoError> {
        let (output, err) = self.kubectl(cancel, &[
            "get",
            "nodes",
            "--output",
            r#"jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.creationTimestamp}{"\t"}{.spec.taints[*].key}{"\t"}{.status.conditions[?(@.type=="DiskPressure")].status}{"\n"}{end}"#,
        ]);
        if let Err(e) = err {
            let err = command_failed("kubectl get nodes", &e);
            write_to(&self.debug_log, &format!("{err}\n"));
            return Err(err);
        }
        let (mut pressure, mut newest, mut read) = (String::new(), DateTime::<Utc>::MIN_UTC, false);
        for line in String::from_utf8_lossy(&output).split('\n') {
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
    use crate::runner::tests::{buffer, text};
    use crate::runner::{ExecError, RunResult};
    use std::cell::RefCell;
    use std::rc::Rc;

    const JWT: &str = "eyJhbGciOiJIUzI1NiJ9.eyJjbHVzdGVyIjoiZGVtbyJ9.c2lnbmF0dXJl";
    const KUBECTL: &str = "silent: kubectl --context k3d-local-demo-user --request-timeout 10s ";

    /// `scriptedDemoRunner`.
    struct ScriptedRunner {
        calls: Rc<RefCell<Vec<String>>>,
        respond: Box<dyn Fn(&str) -> RunResult>,
    }

    impl ScriptedRunner {
        fn record(&mut self, mode: &str, name: &str, args: &[String]) -> RunResult {
            let command = std::iter::once(name.to_string())
                .chain(args.iter().cloned())
                .collect::<Vec<_>>()
                .join(" ");
            self.calls.borrow_mut().push(format!("{mode}: {command}"));
            (self.respond)(&command)
        }
    }

    impl CommandRunner for ScriptedRunner {
        fn look_path(&mut self, _: &str) -> Result<(), String> {
            Ok(())
        }
        fn run(&mut self, _: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record("run", name, args)
        }
        fn run_quiet(&mut self, _: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record("quiet", name, args)
        }
        fn run_unlogged(&mut self, _: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record("unlogged", name, args)
        }
        fn run_silent(&mut self, _: Cancel, name: &str, args: &[String]) -> RunResult {
            self.record("silent", name, args)
        }
    }

    type TestLocal = (
        LocalCommands<ScriptedRunner>,
        Rc<RefCell<Vec<String>>>,
        Rc<RefCell<Vec<u8>>>,
    );

    fn local(respond: impl Fn(&str) -> RunResult + 'static) -> TestLocal {
        let calls = Rc::new(RefCell::new(Vec::new()));
        let (log, sink) = buffer();
        let mut l = LocalCommands::new(
            ScriptedRunner {
                calls: calls.clone(),
                respond: Box::new(respond),
            },
            "linux",
            sink,
        );
        l.kube_context = "k3d-local-demo-user".into();
        (l, calls, log)
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

    #[test]
    fn collect_diagnostics_writes_a_redacted_bounded_view() {
        let (mut l, calls, log) = local(|command| {
            if command.contains(" get nodes ") {
                ok(
                    "NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS\nserver-0   True    True            False             False          node.kubernetes.io/disk-pressure\n",
                )
            } else if command.contains(" get pods ") {
                ok(concat!(
                    "NAMESPACE     NAME                             READY   STATUS             RESTARTS   AGE   IP          NODE\n",
                    "kube-system   coredns-1                        1/1     Running            0          1h    10.0.0.2    server-0\n",
                    "ingress       ingress-1                        0/1     ImagePullBackOff   0          1h    10.0.0.3    server-0\n",
                    "jobs          migrate-1                        0/1     Completed          0          1h    10.0.0.4    server-0\n",
                    "qovery        qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4          9m    10.0.0.5    server-0\n",
                    "qovery        qovery-helper-1                  1/1     Running            0          9m    10.0.0.6    server-0\n",
                ))
            } else if command.contains(" get events ") {
                ok(
                    "LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE\n10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container\n",
                )
            } else if command.contains(" logs ") {
                ok(&format!(
                    "starting\nconnecting with Authorization: Bearer opaque-value\njwt={JWT}\npanic: invalid token: expired\n"
                ))
            } else {
                ok("")
            }
        });

        l.collect_kubernetes_diagnostics(Cancel::new(), DEMO_DIAGNOSTIC_BUDGET);

        assert_eq!(
            vec![
                format!(
                    "{KUBECTL}get nodes --output custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type==\"Ready\")].status,DISK_PRESSURE:.status.conditions[?(@.type==\"DiskPressure\")].status,MEMORY_PRESSURE:.status.conditions[?(@.type==\"MemoryPressure\")].status,PID_PRESSURE:.status.conditions[?(@.type==\"PIDPressure\")].status,TAINTS:.spec.taints[*].key"
                ),
                format!("{KUBECTL}get pods --all-namespaces --output wide"),
                format!(
                    "{KUBECTL}--namespace qovery get events --sort-by .metadata.creationTimestamp"
                ),
                format!(
                    "{KUBECTL}--namespace qovery logs deploy/qovery-operator --tail=100 --limit-bytes=32768"
                ),
            ],
            *calls.borrow()
        );
        assert_eq!(
            r#"
===== Kubernetes diagnostics =====
--- kubectl get nodes ---
NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS
server-0   True    True            False             False          node.kubernetes.io/disk-pressure
--- kubectl get pods ---
NAMESPACE     NAME                             READY   STATUS             RESTARTS   AGE   IP          NODE
ingress       ingress-1                        0/1     ImagePullBackOff   0          1h    10.0.0.3    server-0
qovery        qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4          9m    10.0.0.5    server-0
qovery        qovery-helper-1                  1/1     Running            0          9m    10.0.0.6    server-0
--- kubectl get events ---
LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE
10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container
--- kubectl logs ---
starting
connecting with Authorization: <redacted>
jwt=<redacted>
panic: invalid token: expired
===== end of Kubernetes diagnostics =====
"#,
            text(&log)
        );
    }

    #[test]
    fn collect_diagnostics_logs_failures_skips_operator_logs_and_keeps_to_its_budget() {
        let (mut l, calls, log) = local(|command| {
            if command.contains(" get nodes ") {
                failed("The connection to the server was refused")
            } else {
                ok(&"line\n".repeat(500))
            }
        });

        l.collect_kubernetes_diagnostics(Cancel::new(), DEMO_DIAGNOSTIC_BUDGET);

        assert_eq!(3, calls.borrow().len());
        let lines = |n| "line\n".repeat(n);
        assert!(text(&log).contains(&format!(
            "\nkubectl get nodes failed with exit code 1\n--- kubectl get pods ---\n{}--- kubectl get events ---\n{}=====",
            lines(100),
            lines(60)
        )));
        assert!(!text(&log).contains("connection to the server"));

        calls.borrow_mut().clear();
        log.borrow_mut().clear();
        l.collect_kubernetes_diagnostics(Cancel::new(), Duration::ZERO);
        assert!(calls.borrow().is_empty());
        assert!(
            text(&log).contains("kubectl get nodes skipped: diagnostic time budget exhausted\n")
        );
    }

    #[test]
    fn tail_caps_lines_and_bytes() {
        assert_eq!("c\nd\n", demo_diagnostic_tail(b"a\nb\nc\nd\n", 2));
        let line = "x".repeat(1000);
        let tail = demo_diagnostic_tail(format!("{line}\n").repeat(100).as_bytes(), 100);
        assert!(tail.len() <= DEMO_DIAGNOSTIC_MAX_BYTES + 1);
        assert!(tail.starts_with(&format!("{line}\n")));
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
            (
                "calling with Bearer abc.def".into(),
                "calling with Bearer <redacted>",
            ),
            (
                "pulling https://robot:s3cret@registry.example/v2/ failed".into(),
                "pulling https://robot:<redacted>@registry.example/v2/ failed",
            ),
            (
                r#"{"token":"abc","level":"info"}"#.into(),
                r#"{"token":"<redacted>","level":"info"}"#,
            ),
            (
                r#"{\"token\":\"abc def\",\"level\":\"info\"}"#.into(),
                r#"{\"token\":\"<redacted>\",\"level\":\"info\"}"#,
            ),
            (
                r#"password="abc\"SECRET" user=admin"#.into(),
                r#"password="<redacted>" user=admin"#,
            ),
            (
                r#"secretToken: "my pass word" next"#.into(),
                r#"secretToken: "<redacted>" next"#,
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
                "refresh token: context deadline exceeded".into(),
                "refresh token: context deadline exceeded",
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
            let (mut l, calls, _) = local(move |_| ok(output));
            assert_eq!(
                (node.to_string(), created),
                l.read_node_disk_pressure(Cancel::new()).unwrap()
            );
            assert_eq!(
                vec![format!(
                    "{KUBECTL}get nodes --output jsonpath={{range .items[*]}}{{.metadata.name}}{{\"\\t\"}}{{.metadata.creationTimestamp}}{{\"\\t\"}}{{.spec.taints[*].key}}{{\"\\t\"}}{{.status.conditions[?(@.type==\"DiskPressure\")].status}}{{\"\\n\"}}{{end}}"
                )],
                *calls.borrow()
            );
        }
        let (mut l, _, log) = local(|_| failed("refused"));
        assert!(l.read_node_disk_pressure(Cancel::new()).is_err());
        assert_eq!("kubectl get nodes failed with exit code 1\n", text(&log));
    }
}
