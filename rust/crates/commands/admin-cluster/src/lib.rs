//! `qovery admin cluster ...` and the other cluster related `qovery admin` sub-commands.
//! Port of `cmd/admin_cluster*.go` & co. Assembled under `admin` by the `qovery-cli` crate.

mod common;
mod credentials;
mod dns;
mod list_deploy;
mod maintenance;
mod operator;
mod profile;
mod status;

use qovery_cobra::Command;

/// Sub-commands of `qovery admin` owned by this crate.
pub fn commands() -> Vec<Command> {
    vec![
        cluster(),
        maintenance::delete_cluster_undeployed_in_error(),
        maintenance::force_delete_cluster(),
        maintenance::force_delete_old_invalid_credentials_clusters(),
        credentials::k9s(),
        credentials::load_aws_credentials(),
        credentials::load_credentials(),
        maintenance::locked(),
        maintenance::notify_users_cluster_failure(),
        maintenance::update(),
        maintenance::update_bulk(),
    ]
}

/// `qovery admin cluster` (`cmd/admin_cluster.go`).
fn cluster() -> Command {
    Command::new("cluster")
        .short("Manage clusters")
        .run(|ctx| {
            qovery_core::telemetry::capture(ctx);
            if ctx.args().is_empty() {
                ctx.help();
                qovery_core::exit(0);
            }
        })
        .subcommand(list_deploy::deploy())
        .subcommand(list_deploy::list())
        .subcommand(operator::command())
        .subcommand(profile::command())
        .subcommand(status::command())
        .subcommand(dns::update_dns_provider())
        .subcommand(dns::update_domain())
        .subcommand(dns::kubeconfig())
}
