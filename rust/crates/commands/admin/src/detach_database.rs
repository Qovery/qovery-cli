//! `qovery admin detach-database`. Port of `cmd/admin_detach_database.go` and `pkg/detach_database.go`.

use crate::goutil::path_escape;
use crate::http::{Req, authorization};
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::client::admin_url;
use qovery_core::exit_with_error;
use qovery_core::select::validate;
use qovery_log::fatih;
use qovery_log::gofmt::go_json;
use reqwest::Method;
use serde::Deserialize;
use serde_json::json;
use std::fmt::Write as _;

const BANNER: &str = "Cloud resources (instance, snapshots, subnet/parameter/security groups, KMS keys, IAM roles) will NOT be touched.\n\
Qovery will stop managing and tracking this database, and its built-in variables disappear from every service of the environment at its next deploy.";

pub fn command() -> Command {
    Command::new("detach-database")
        .short("Remove a managed database from Qovery only: no deployment, nothing touched on the cloud account")
        .long(
            "Remove a legacy managed database service and its built-in variables from Qovery, without any
action on the customer's cloud account: the instance, its snapshots and its network/IAM resources stay.

Runs a dry run by default. Refused when the database is a container or blueprint-backed one, when its
environment is deploying or has a queued request, or when an alias or override relies on its variables.",
        )
        .args(ArgsRule::NoArgs)
        .flag(Flag::string("database", "Database's id").short('d').required())
        .flag(
            Flag::string(
                "reason",
                "Why (support ticket, customer request): recorded in the customer's audit log",
            )
            .short('r')
            .required(),
        )
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .run(run)
}

fn run(ctx: &Ctx) {
    let database_id = ctx.string("database");
    let reason = ctx.string("reason");
    let execute = ctx.bool("disable-dry-run");
    if let Err(e) = detach_database(&database_id, &reason, execute) {
        exit_with_error(e);
    }
}

/// Go `pkg.DatabaseDetachReport`: absent and `null` fields read as zero values, like `json.Unmarshal`.
#[derive(Default, Deserialize)]
#[serde(default)]
struct Report {
    database_id: Option<String>,
    database_name: Option<String>,
    database_type: Option<String>,
    database_mode: Option<String>,
    database_version: Option<String>,
    environment_id: Option<String>,
    variables_to_delete: Option<Vec<String>>,
    blockers: Option<Vec<String>>,
    dry_run: Option<bool>,
    detached: Option<bool>,
    audit_logged: Option<bool>,
}

impl Report {
    fn id(&self) -> &str {
        self.database_id.as_deref().unwrap_or("")
    }
    fn blockers(&self) -> &[String] {
        self.blockers.as_deref().unwrap_or(&[])
    }
    fn is_dry_run(&self) -> bool {
        self.dry_run.unwrap_or(false)
    }
    fn is_detached(&self) -> bool {
        self.detached.unwrap_or(false)
    }
}

/// `pkg.DetachDatabase`.
fn detach_database(database_id: &str, reason: &str, execute: bool) -> Result<(), String> {
    if reason.trim().is_empty() {
        return Err("a reason is required (support ticket, customer request)".to_string());
    }
    let auth = authorization()?;
    let admin = admin_url();

    let preview = request(&admin, &auth, database_id, reason, true)?;
    print!("{}", format_report(&preview));
    if !preview.blockers().is_empty() {
        return Err(format!(
            "database {database_id} cannot be detached, fix the blockers above first"
        ));
    }

    qovery_log::dry_run_print(execute);
    if !execute {
        return Ok(());
    }
    if !validate("detach") {
        return Err("detach not confirmed".to_string());
    }

    let result = request(&admin, &auth, database_id, reason, false).map_err(|e| {
        format!("{e}\nThe detach may have been applied anyway: run the command again without -y to check the database before retrying")
    })?;
    println!(
        "Database {} ({}) detached from Qovery.",
        result.database_name.as_deref().unwrap_or(""),
        result.id()
    );
    if !result.audit_logged.unwrap_or(false) {
        return Err(format!(
            "database {} was detached but the audit log could not be written: record it by hand in the support ticket",
            result.id()
        ));
    }
    Ok(())
}

/// `pkg.requestDatabaseDetach`.
fn request(
    admin_url: &str,
    authorization: &str,
    database_id: &str,
    reason: &str,
    dry_run: bool,
) -> Result<Report, String> {
    // Go marshals the struct in field order (reason, dry_run), not sorted like a map.
    let body = format!(
        "{{\"reason\":{},\"dry_run\":{dry_run}}}",
        go_json(&json!(reason))
    );
    let url = format!(
        "{}/database/{}/detach",
        admin_url.strip_suffix('/').unwrap_or(admin_url),
        path_escape(database_id)
    );
    let res = Req::new(Method::POST, &url, authorization)
        .body(body)
        .send()?;
    if let Some(e) = res.read_err {
        return Err(e);
    }
    if res.status != 200 {
        return Err(format!(
            "detach of database {database_id} failed: {} {}",
            res.status_line(),
            res.text().trim()
        ));
    }
    // Go's syntax error text (`json.Unmarshal` runs the same check first).
    crate::gojson::check_valid(&res.body)
        .map_err(|e| format!("unexpected response for database {database_id}: {e}"))?;
    let value: serde_json::Value = serde_json::from_slice(&res.body)
        .map_err(|e| format!("unexpected response for database {database_id}: {e}"))?;
    let report: Report = if value.is_null() {
        Report::default()
    } else {
        serde_json::from_value(value)
            .map_err(|e| format!("unexpected response for database {database_id}: {e}"))?
    };
    check_report(&report, database_id, dry_run)?;
    Ok(report)
}

/// `pkg.checkDatabaseDetachReport`.
fn check_report(report: &Report, database_id: &str, dry_run: bool) -> Result<(), String> {
    if report.id() != database_id {
        return Err(format!(
            "unexpected response: it describes database {}, not {}",
            qovery_log::gofmt::quote(report.id()),
            qovery_log::gofmt::quote(database_id)
        ));
    }
    if report.is_dry_run() != dry_run {
        return Err(format!(
            "unexpected response for database {database_id}: dry_run is {}, expected {dry_run}",
            report.is_dry_run()
        ));
    }
    if dry_run && report.is_detached() {
        return Err(format!(
            "unexpected response for database {database_id}: a dry run reports the database as detached"
        ));
    }
    if !dry_run && (!report.is_detached() || !report.blockers().is_empty()) {
        return Err(format!(
            "database {database_id}: inconsistent detach outcome (detached: {}, blockers: {})",
            report.is_detached(),
            report.blockers().join("; ")
        ));
    }
    Ok(())
}

/// `pkg.formatDatabaseDetachReport`.
fn format_report(report: &Report) -> String {
    let field = |v: &Option<String>| v.clone().unwrap_or_default();
    let mut b = String::new();
    let _ = writeln!(
        b,
        "Database:    {} ({})",
        field(&report.database_name),
        report.id()
    );
    let _ = writeln!(
        b,
        "Type:        {} {}, mode {}",
        field(&report.database_type),
        field(&report.database_version),
        field(&report.database_mode)
    );
    let _ = writeln!(b, "Environment: {}", field(&report.environment_id));
    b.push_str("Built-in variables to delete:\n");
    let variables = report.variables_to_delete.as_deref().unwrap_or(&[]);
    if variables.is_empty() {
        b.push_str("  (none)\n");
    }
    for name in variables {
        let _ = writeln!(b, "  - {name}");
    }
    if !report.blockers().is_empty() {
        b.push_str(&fatih::red("Blockers:"));
        b.push('\n');
        for blocker in report.blockers() {
            let _ = writeln!(b, "  - {blocker}");
        }
    }
    b.push_str(&fatih::yellow(BANNER));
    b.push('\n');
    b
}

#[cfg(test)]
mod tests {
    use super::*;

    fn report(json: &str) -> Report {
        serde_json::from_str(json).unwrap()
    }

    #[test]
    fn rejects_responses_that_do_not_match_the_request() {
        let cases = [
            ("{}", true),
            ("{\"database_id\":\"db-2\",\"dry_run\":true}", true),
            ("{\"database_id\":\"db-1\",\"dry_run\":true}", false),
            (
                "{\"database_id\":\"db-1\",\"dry_run\":false,\"detached\":false}",
                false,
            ),
            (
                "{\"database_id\":\"db-1\",\"dry_run\":true,\"detached\":true}",
                true,
            ),
            (
                "{\"database_id\":\"db-1\",\"dry_run\":false,\"detached\":true,\"blockers\":[\"x\"]}",
                false,
            ),
        ];
        for (json, dry_run) in cases {
            assert!(
                check_report(&report(json), "db-1", dry_run).is_err(),
                "{json}"
            );
        }
    }

    #[test]
    fn accepts_a_matching_preview_and_execution() {
        let preview = report("{\"database_id\":\"db-1\",\"dry_run\":true,\"blockers\":null}");
        assert!(check_report(&preview, "db-1", true).is_ok());
        let done = report(
            "{\"database_id\":\"db-1\",\"dry_run\":false,\"detached\":true,\"blockers\":[]}",
        );
        assert!(check_report(&done, "db-1", false).is_ok());
    }

    #[test]
    fn report_shows_blockers_and_banner() {
        let out = format_report(&report(
            "{\"database_id\":\"db-1\",\"database_name\":\"orders-db\",\"variables_to_delete\":[\"QOVERY_POSTGRESQL_ZDB1_HOST\"],\"blockers\":[\"variable DATABASE_HOST (ALIAS, CONTAINER c-1) would be deleted with the built-in variables\"]}",
        ));
        for want in [
            "orders-db (db-1)",
            "QOVERY_POSTGRESQL_ZDB1_HOST",
            "DATABASE_HOST (ALIAS",
            "will NOT be touched",
        ] {
            assert!(out.contains(want), "missing {want}:\n{out}");
        }
    }
}
