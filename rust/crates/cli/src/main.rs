//! `qovery` binary: the root command (port of `main.go` and `cmd/root.go`).
//!
//! Every top-level command lives in its own crate (`crates/commands/*`), enabled by a `cmd-<name>` feature.

#[allow(unused_imports, reason = "unused when the cmd-* features are disabled")]
use qovery_cobra::{App, Command, Flag};

#[allow(
    unused_mut,
    reason = "not mutated when the cmd-* features are disabled"
)]
fn root() -> Command {
    let mut root = Command::new("qovery")
        .short("A Command-line Interface of the Qovery platform")
        .flag(Flag::bool("verbose", "Verbose output").persistent());

    #[cfg(any(feature = "cmd-admin", feature = "cmd-admin-cluster"))]
    {
        #[cfg(feature = "cmd-admin")]
        let mut admin = qovery_cmd_admin::command();
        #[cfg(not(feature = "cmd-admin"))]
        let mut admin = Command::new("admin").hidden();
        #[cfg(feature = "cmd-admin-cluster")]
        admin.children.extend(qovery_cmd_admin_cluster::commands());
        root = root.subcommand(admin);
    }
    #[cfg(feature = "cmd-api")]
    {
        root = root.subcommand(qovery_cmd_api::command());
    }
    #[cfg(feature = "cmd-application")]
    {
        root = root.subcommand(qovery_cmd_application::command());
    }
    #[cfg(feature = "cmd-audit-log")]
    {
        root = root.subcommand(qovery_cmd_audit_log::command());
    }
    #[cfg(feature = "cmd-auth")]
    {
        root = root.subcommand(qovery_cmd_auth::command());
    }
    #[cfg(feature = "cmd-blueprint")]
    {
        root = root.subcommand(qovery_cmd_blueprint::command());
    }
    #[cfg(feature = "cmd-cluster")]
    {
        root = root.subcommand(qovery_cmd_cluster::command());
    }
    #[cfg(feature = "cmd-console")]
    {
        root = root.subcommand(qovery_cmd_console::command());
    }
    #[cfg(feature = "cmd-container")]
    {
        root = root.subcommand(qovery_cmd_container::command());
    }
    #[cfg(feature = "cmd-context")]
    {
        root = root.subcommand(qovery_cmd_context::command());
    }
    #[cfg(feature = "cmd-cronjob")]
    {
        root = root.subcommand(qovery_cmd_cronjob::command());
    }
    #[cfg(feature = "cmd-database")]
    {
        root = root.subcommand(qovery_cmd_database::command());
    }
    #[cfg(feature = "cmd-demo")]
    {
        root = root.subcommand(qovery_cmd_demo::command());
    }
    #[cfg(feature = "cmd-enterprise-connection")]
    {
        root = root.subcommand(qovery_cmd_enterprise_connection::command());
    }
    #[cfg(feature = "cmd-env")]
    {
        root = root.subcommand(qovery_cmd_env::command());
    }
    #[cfg(feature = "cmd-environment")]
    {
        root = root.subcommand(qovery_cmd_environment::command());
    }
    #[cfg(feature = "cmd-helm")]
    {
        root = root.subcommand(qovery_cmd_helm::command());
    }
    #[cfg(feature = "cmd-lifecycle")]
    {
        root = root.subcommand(qovery_cmd_lifecycle::command());
    }
    #[cfg(feature = "cmd-list-commands")]
    {
        root = root.subcommand(qovery_cmd_list_commands::command());
    }
    #[cfg(feature = "cmd-list-pods")]
    {
        root = root.subcommand(qovery_cmd_list_pods::command());
    }
    #[cfg(feature = "cmd-log")]
    {
        root = root.subcommand(qovery_cmd_log::command());
    }
    #[cfg(feature = "cmd-organization")]
    {
        root = root.subcommand(qovery_cmd_organization::command());
    }
    #[cfg(feature = "cmd-port-forward")]
    {
        root = root.subcommand(qovery_cmd_port_forward::command());
    }
    #[cfg(feature = "cmd-project")]
    {
        root = root.subcommand(qovery_cmd_project::command());
    }
    #[cfg(feature = "cmd-rde")]
    {
        root = root.subcommand(qovery_cmd_rde::command());
    }
    #[cfg(feature = "cmd-service")]
    {
        root = root.subcommand(qovery_cmd_service::command());
    }
    #[cfg(feature = "cmd-shell")]
    {
        root = root.subcommand(qovery_cmd_shell::command());
    }
    #[cfg(feature = "cmd-skills")]
    {
        root = root.subcommand(qovery_cmd_skills::command());
    }
    #[cfg(feature = "cmd-status")]
    {
        root = root.subcommand(qovery_cmd_status::command());
    }
    #[cfg(feature = "cmd-terraform")]
    {
        root = root.subcommand(qovery_cmd_terraform::command());
    }
    #[cfg(feature = "cmd-token")]
    {
        root = root.subcommand(qovery_cmd_token::command());
    }
    // `cmd/upgrade.go` is `//go:build !windows`
    #[cfg(all(feature = "cmd-upgrade", not(windows)))]
    {
        root = root.subcommand(qovery_cmd_upgrade::command());
    }
    #[cfg(feature = "cmd-version")]
    {
        root = root.subcommand(qovery_cmd_version::command());
    }
    #[cfg(feature = "cmd-webhook")]
    {
        root = root.subcommand(qovery_cmd_webhook::command());
    }
    #[cfg(feature = "cmd-completion")]
    {
        root = root.subcommands(qovery_cmd_completion::commands());
    }
    root
}

/// `initConfig`: create `~/.qovery/context.json` when missing.
fn init_config() {
    if !qovery_core::context::qovery_context_exists()
        && let Err(e) = qovery_core::context::initialize_qovery_context()
    {
        qovery_log::println_error(e);
        qovery_core::exit(0);
    }
}

fn main() {
    // Several TLS stacks are linked (reqwest, kube, tokio-tungstenite, the AWS SDK): with more than one
    // rustls crypto backend compiled in, rustls cannot pick one on its own. Select ring for the process.
    let _ = rustls::crypto::ring::default_provider().install_default();
    // Go programs die silently on SIGPIPE when writing to a closed stdout/stderr (`qovery ... | head`);
    // Rust ignores it by default (and `println!` would panic): restore the default disposition.
    #[cfg(unix)]
    unsafe {
        libc::signal(libc::SIGPIPE, libc::SIG_DFL);
    }
    // `utils.Capture(rootCmd)`
    qovery_core::telemetry::capture_with_event_and_properties(
        &qovery_core::telemetry::CommandInfo::root(),
        qovery_core::telemetry::DEFAULT_EVENT_NAME,
        Default::default(),
    );
    qovery_cobra::execute(App::new(root()).on_initialize(init_config));
}
