//! `qovery admin` command. Port of `cmd/admin*.go` (except the cluster related sub-commands, ported in
//! the `qovery-cmd-admin-cluster` crate and assembled under `admin` by `qovery-cli`).
//!
//! Sub-commands owned by this crate: delete, demo, deployment-restriction, download-s3-archive,
//! enable-user-connect, encrypt-secret, enterprise-connection, force-delete-project,
//! force-failed-deployments-to-internal-error, get-organization-plan, jwt, jwt-qovery-usage,
//! publish-environment-deployment-rules, require-extended-credentials, transfer-ownership,
//! update-billing-external-id, update-organization-plan.

mod demo;
mod enterprise_connection;
mod gojson;
mod goutil;
mod http;
mod jwt;
mod misc;
mod organization;
mod s3_archive;

use qovery_cobra::{Command, Ctx};

/// The `admin` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("admin").hidden().subcommands([
        organization::delete(),
        demo::command(),
        organization::deployment_restriction(),
        s3_archive::command(),
        misc::enable_user_connect(),
        misc::encrypt_secret(),
        enterprise_connection::command(),
        misc::force_delete_project(),
        misc::force_failed_deployments_to_internal_error(),
        organization::get_organization_plan(),
        jwt::jwt(),
        jwt::jwt_qovery_usage(),
        misc::publish_environment_deployment_rules(),
        misc::require_extended_credentials(),
        organization::transfer_ownership(),
        organization::update_billing_external_id(),
        organization::update_organization_plan(),
    ])
}

/// Go groups `Run: func(cmd, args) { utils.Capture(cmd); if len(args) == 0 { _ = cmd.Help(); os.Exit(0) } }`.
pub(crate) fn run_group_help(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        qovery_core::exit(0);
    }
}
