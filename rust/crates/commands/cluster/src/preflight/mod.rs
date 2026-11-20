//! `cluster preflight` (cmd/cluster_preflight.go, pkg/cluster/preflight).

pub mod checks;
pub mod collector;
pub mod facts;
pub mod kube;

use checks::{CheckResult, InstallPath, Options, Status};
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::{exit, telemetry};
use serde::Serialize;

const LONG: &str = "Check that your Kubernetes cluster is ready before you install Qovery on it.

Qovery runs on any Kubernetes cluster, with one of two installations:
  - EKS (AWS), GKE (GCP) and Scaleway Kapsule: the Qovery Operator installation, the Helm
    command shown in the Qovery Console.
  - Every other cluster (AKS, k3s, RKE2, OpenShift, on-premise, ...): the installation with
    the Qovery CLI, qovery cluster install.
    https://www.qovery.com/docs/configuration/integrations/kubernetes/byok-legacy

The preflight detects the provider of your nodes, tells you which installation to use and
runs the checks of that installation. Use --install-path to force one. The checks for an
existing cert-manager, ExternalDNS, Envoy Gateway or Gateway API CRDs fail on the Operator
installation, which does not share them. Both installations install these components, so on
the CLI installation they warn: turn off the Qovery copy in the Helm values file that
qovery cluster install generates, or remove the existing one. An existing qovery Helm release
fails only on the Operator installation; on the CLI installation it is Qovery itself.

The preflight runs against the current context of your kubeconfig, or the one given with
--kubeconfig and --context. It only reads from the cluster: it creates, changes or deletes
nothing, and it reads no Secret data. It needs no Qovery account.

It exits with status 1 when at least one check fails. A failed check would make the
installation or the first deployment of the cluster fail; fix it before installing.
Warnings do not block the installation but can break a feature.";

pub fn command() -> Command {
    Command::new("preflight")
        .short("Check that your Kubernetes cluster is ready before you install Qovery on it")
        .long(LONG)
        .args(ArgsRule::NoArgs)
        .flag(Flag::string("kubeconfig", "Path to the kubeconfig file (default: $KUBECONFIG or ~/.kube/config)"))
        .flag(Flag::string("context", "Kubeconfig context to check (default: the current context)"))
        .flag(Flag::bool(
            "loki-object-storage",
            "Loki will store logs in object storage (s3, gcs or s3Compatible), so no default StorageClass is needed",
        ))
        .flag(Flag::bool("json", "JSON output"))
        .flag(
            Flag::string(
                "install-path",
                "Installation to check for: operator (EKS, GKE, Scaleway Kapsule), cli (qovery cluster install, any other cluster) or auto (picked from the provider of the nodes)",
            )
            .default_value("auto"),
        )
        .run(run)
}

fn fail(e: impl std::fmt::Display) -> ! {
    qovery_log::println_error_to_stderr(e);
    exit(1)
}

/// JSON output (keys of the Go map, sorted).
#[derive(Serialize)]
struct JsonOutput<'a> {
    checks: &'a [CheckResult],
    context: &'a str,
    install_path: &'a str,
    passed: bool,
    server: &'a str,
}

fn run(ctx: &Ctx) {
    telemetry::capture(ctx);
    let install_path =
        checks::parse_install_path(&ctx.string("install-path")).unwrap_or_else(|e| fail(e));

    let loaded = kube::load_raw_config(&ctx.string("kubeconfig"))
        .unwrap_or_else(|e| fail(format!("read kubeconfig: {e}")));
    let context_override = ctx.string("context");
    let context_name = if context_override.is_empty() {
        loaded.current_context()
    } else {
        context_override.clone()
    };
    let server = loaded.server(&context_override).unwrap_or_else(|e| {
        fail(format!(
            "load kubeconfig context {}: {e}",
            qovery_log::gofmt::quote(&context_name)
        ))
    });
    let api = kube::RealKubeApi::new(&loaded, &context_name, &server).unwrap_or_else(|e| {
        fail(format!(
            "load kubeconfig context {}: {e}",
            qovery_log::gofmt::quote(&context_name)
        ))
    });

    let json = ctx.bool("json");
    if !json {
        qovery_log::println_info(format!(
            "Checking context {context_name} ({server}). Nothing is changed on the cluster."
        ));
    }

    let facts = collector::collect_with(&api);
    let opts = Options {
        loki_object_storage: ctx.bool("loki-object-storage"),
        install_path,
    };
    let results = checks::evaluate(&facts, opts);
    let resolved_path = checks::resolve_install_path(&facts, install_path);
    let failed = checks::failed(&results);

    if json {
        let out = JsonOutput {
            checks: &results,
            context: &context_name,
            install_path: resolved_path.as_str(),
            passed: !failed,
            server: &server,
        };
        qovery_log::println(qovery_log::gofmt::go_marshal_indent(&out, "  "));
    } else {
        print_preflight_results(&results, failed, resolved_path);
    }
    if failed {
        exit(1);
    }
}

fn preflight_status_label(status: Status) -> String {
    match status {
        Status::Pass => qovery_log::fatih::green(status.as_str()),
        Status::Warn => qovery_log::fatih::yellow(status.as_str()),
        Status::Fail => qovery_log::fatih::red(status.as_str()),
        Status::Skip => status.as_str().to_string(),
    }
}

fn print_preflight_results(results: &[CheckResult], failed: bool, path: InstallPath) {
    let data: Vec<Vec<String>> = results
        .iter()
        .map(|r| {
            vec![
                preflight_status_label(r.status),
                r.title.clone(),
                r.detail.clone(),
            ]
        })
        .collect();
    qovery_log::print_table(&["Status", "Check", "Detail"], &data);

    let to_fix: Vec<&CheckResult> = results
        .iter()
        .filter(|r| !r.fix.is_empty() && (r.status == Status::Fail || r.status == Status::Warn))
        .collect();
    if !to_fix.is_empty() {
        qovery_log::println("\nHow to fix:");
        for r in to_fix {
            qovery_log::println(format!(
                "- [{}] {} ({}): {}",
                preflight_status_label(r.status),
                r.title,
                r.id,
                r.fix
            ));
        }
    }

    qovery_log::println("");
    if failed {
        qovery_log::println_error(
            "the cluster is not ready for Qovery. Fix the failed checks, then run qovery cluster preflight again",
        );
        return;
    }
    if path == InstallPath::Cli {
        qovery_log::println_info(
            "No blocking issue found. Install Qovery with: qovery cluster install",
        );
        return;
    }
    qovery_log::println_info(
        "No blocking issue found. You can run the Helm command of the Qovery Operator shown in the Qovery Console.",
    );
}
