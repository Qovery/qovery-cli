//! `qovery audit-log` command. Port of `cmd/audit_log*.go`.
//!
//! [`service`] is the port of `pkg/auditlog`.

pub mod service;

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{auth, check_error, client, exit, resources, telemetry};
use serde_json::Value;

const EXPORT_LONG: &str = "> Description
-------------
This command provides an easy way to download audit logs.
Date parameters must follow the ISO-8601 format, i.e:
* 2025-10-02T01:04:45+12:00 is valid
* 2025-10-02T01:04:45Z is valid
* 2025-10-02 01:04:45Z is invalid (missing T separator)

> Examples 
----------
* Search from a specific date to now:
qovery audit-log export --from-date 2025-09-01T01:04:45+02:00

* Search between a range of dates
qovery audit-log export --from-date 2025-09-01T01:04:45Z --to-date 2025-09-02T02:00:00Z
";

/// The `audit-log` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("audit-log")
        .short("Interact with audit logs")
        .run(run_group)
        .subcommand(export())
}

fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

fn export() -> Command {
    Command::new("export")
        .short("Export audit logs")
        .long(EXPORT_LONG)
        .flag(Flag::string("organization", "Organization Name").short('o'))
        .flag(
            Flag::string(
                "from-date",
                "Start date for the search following ISO-8601 format",
            )
            .short('f')
            .required(),
        )
        .flag(
            Flag::string(
                "to-date",
                "End date for the search following ISO-8601 format (defaulted to 'now')",
            )
            .short('t'),
        )
        .run(run_export)
}

fn run_export(ctx: &Ctx) {
    let client = client::get_client_or_exit();
    let organization_id = check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));

    let org: Value = check_error(client.get(&format!("/organization/{organization_id}")));
    let days = org["organization_plan"]["audit_logs_retention_in_days"]
        .as_f64()
        .unwrap_or(0.0) as f32;
    qovery_log::println(format!(
        "Your organization plan provides  {:.0} days of audit log history",
        days as f64
    ));

    let (token_type, token) = check_error(auth::get_access_token(false));

    let options = service::DownloadOptions {
        organization_id,
        from_date: ctx.string("from-date"),
        to_date: ctx.string("to-date"),
        token_type: token_type.as_str().to_string(),
        token,
    };
    check_error(service::download_audit_logs(&options));
}
