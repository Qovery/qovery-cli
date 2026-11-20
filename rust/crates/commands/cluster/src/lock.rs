//! `cluster lock`, `cluster unlock`, `cluster locked` (cmd/cluster_lock.go, cluster_unlock.go, cluster_locked.go)
//! and `pkg.LockedClusters` (pkg/lock.go, used by `qovery admin locked`).

use crate::prompt::validate;
use crate::util::{self, ZERO_TIME_RFC1123, format_rfc1123, log_detail, path_escape, tabwriter};
use qovery_api::Method;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, exit, exit_with_error};
use serde_json::{Value, json};

pub fn lock_command() -> Command {
    Command::new("lock")
        .short("Lock a cluster")
        .flag(
            Flag::string("cluster-id", "Cluster ID")
                .short('c')
                .required(),
        )
        .flag(Flag::string("reason", "Reason").short('r').required())
        .flag(
            Flag::int32(
                "ttl-in-days",
                " Time-to-live (TTL) for the lock in days (1 to 5 days)",
            )
            .short('d')
            .default_value("-1"),
        )
        .run(run_lock)
}

pub fn unlock_command() -> Command {
    Command::new("unlock")
        .short("Unlock a cluster")
        .flag(Flag::string("cluster-id", "Cluster ID").short('c'))
        .run(run_unlock)
}

pub fn locked_command() -> Command {
    Command::new("locked")
        .short("List locked clusters")
        .flag(
            Flag::string("organization-id", "Organization ID")
                .short('o')
                .required(),
        )
        .run(run_locked)
}

/// Required properties of the `ClusterLock` model.
const CLUSTER_LOCK_REQUIRED: [&str; 4] = ["reason", "cluster_id", "locked_at", "owner_name"];

fn missing_required(v: &Value, required: &[&str]) -> Option<String> {
    let obj = v.as_object()?;
    required
        .iter()
        .find(|p| !obj.contains_key(**p))
        .map(|p| format!("no value given for required property {p}"))
}

fn run_lock(ctx: &Ctx) {
    let ttl = ctx.int32("ttl-in-days");
    if !validate("lock") {
        return;
    }
    let client = qovery_core::check_error(client::get_client());
    let mut body = json!({"reason": ctx.string("reason")});
    if ttl != -1 {
        body["ttl_in_days"] = json!(ttl);
    }
    let path = format!("/cluster/{}/lock", path_escape(&ctx.string("cluster-id")));
    match client.send(Method::POST, &path, &[], Some(&body)) {
        Err(e) => exit_with_error(e),
        Ok(r) => {
            let err = if r.status >= 300 {
                Some(r.status_line())
            } else {
                serde_json::from_slice::<Value>(&r.body)
                    .ok()
                    .and_then(|v| missing_required(&v, &CLUSTER_LOCK_REQUIRED))
            };
            if let Some(e) = err {
                qovery_log::println_error(e);
                log_detail(&r.body);
                exit(1);
            }
        }
    }
    println!("Cluster locked.");
}

fn run_unlock(ctx: &Ctx) {
    if !validate("unlock") {
        return;
    }
    let client = qovery_core::check_error(client::get_client());
    let path = format!("/cluster/{}/lock", path_escape(&ctx.string("cluster-id")));
    match client.send(Method::DELETE, &path, &[], None) {
        Err(e) => exit_with_error(e),
        Ok(r) if r.status >= 300 => {
            qovery_log::println_error(r.status_line());
            log_detail(&r.body);
            exit(1);
        }
        Ok(_) => {}
    }
    println!("Cluster unlocked.");
}

fn locked_at(lock: &Value) -> String {
    lock.get("locked_at")
        .and_then(|v| v.as_str())
        .and_then(util::parse_time)
        .map(|t| format_rfc1123(&t))
        .unwrap_or_else(|| ZERO_TIME_RFC1123.to_string())
}

fn ttl_in_days(lock: &Value) -> String {
    match lock.get("ttl_in_days").and_then(|v| v.as_i64()) {
        Some(n) => n.to_string(),
        None => "infinite".to_string(),
    }
}

fn line(cells: [&str; 6]) -> Vec<String> {
    vec![
        cells[0].to_string(),
        format!(" | {}", cells[1]),
        format!(" | {}", cells[2]),
        format!(" | {}", cells[3]),
        format!(" | {}", cells[4]),
        format!(" | {}\n", cells[5]),
    ]
}

fn print_stdout(s: &str) {
    use std::io::Write;
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(s.as_bytes());
    let _ = out.flush();
}

/// Table of `cluster locked`.
fn locked_table(locks: &[Value]) -> String {
    let mut lines = vec![line([
        "",
        "cluster_id",
        "locked_at",
        "ttl_in_days",
        "locked_by",
        "reason",
    ])];
    for (idx, lock) in locks.iter().enumerate() {
        let s = |k: &str| {
            lock.get(k)
                .and_then(|v| v.as_str())
                .unwrap_or_default()
                .to_string()
        };
        lines.push(line([
            &(idx + 1).to_string(),
            &s("cluster_id"),
            &locked_at(lock),
            &ttl_in_days(lock),
            &s("owner_name"),
            &s("reason"),
        ]));
    }
    tabwriter(&lines, 1, 1)
}

fn run_locked(ctx: &Ctx) {
    let client = qovery_core::check_error(client::get_client());
    let path = format!(
        "/organization/{}/lock",
        path_escape(&ctx.string("organization-id"))
    );
    let res = match client.send(Method::GET, &path, &[], None) {
        Ok(r) => r,
        Err(e) => qovery_log::logrus::fatal(e.to_string()),
    };
    if res.status != 200 {
        qovery_log::logrus::error(format!(
            "Could not list locked clusters : {}. {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let v: Value = serde_json::from_slice(&res.body).unwrap_or(Value::Null);
    let locks: Vec<Value> = v
        .get("results")
        .and_then(|r| r.as_array())
        .cloned()
        .unwrap_or_default();
    if let Some(e) = locks
        .iter()
        .find_map(|l| missing_required(l, &CLUSTER_LOCK_REQUIRED))
    {
        qovery_log::logrus::fatal(e);
    }
    print_stdout(&locked_table(&locks));
}

/// Table of `pkg.LockedClusters` (columns in another order than `cluster locked`).
fn admin_locked_table(locks: &[Value]) -> String {
    let mut lines = vec![line([
        "",
        "cluster_id",
        "locked_at",
        "locked_by",
        "reason",
        "ttl_in_days",
    ])];
    for (idx, lock) in locks.iter().enumerate() {
        let s = |k: &str| {
            lock.get(k)
                .and_then(|v| v.as_str())
                .unwrap_or_default()
                .to_string()
        };
        lines.push(line([
            &(idx + 1).to_string(),
            &s("cluster_id"),
            &locked_at(lock),
            &s("owner_name"),
            &s("reason"),
            &ttl_in_days(lock),
        ]));
    }
    tabwriter(&lines, 1, 1)
}

/// Port of `pkg.LockedClusters()` (pkg/lock.go): list the locked clusters through the admin API (`ADMIN_URL`).
pub fn admin_locked_clusters() {
    client::admin_url();
    let client = match client::get_client() {
        Ok(c) => c,
        Err(e) => {
            qovery_log::println_error(e);
            exit(0);
        }
    };
    let url = format!("{}/cluster/lock", client::admin_url());
    let res = match client.send(Method::GET, &url, &[], None) {
        Ok(r) => r,
        Err(e) => qovery_log::logrus::fatal(e.to_string()),
    };
    if res.status != 200 {
        qovery_log::logrus::error(format!(
            "Could not list locked clusters : {}. {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let v: Value = match serde_json::from_slice(&res.body) {
        Ok(v) => v,
        Err(e) => qovery_log::logrus::fatal(e.to_string()),
    };
    let locks: Vec<Value> = v
        .get("results")
        .and_then(|r| r.as_array())
        .cloned()
        .unwrap_or_default();
    print_stdout(&admin_locked_table(&locks));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn locked_tables() {
        let locks = vec![
            json!({"cluster_id": "c-1", "locked_at": "2024-01-02T03:04:05Z", "owner_name": "me", "reason": "maint", "ttl_in_days": 2}),
            json!({"cluster_id": "cluster-22", "locked_at": "2024-01-02T03:04:05Z", "owner_name": "someone", "reason": "r"}),
        ];
        let expected = concat!(
            "   | cluster_id  | locked_at                      | ttl_in_days  | locked_by  | reason\n",
            "1  | c-1         | Tue, 02 Jan 2024 03:04:05 UTC  | 2            | me         | maint\n",
            "2  | cluster-22  | Tue, 02 Jan 2024 03:04:05 UTC  | infinite     | someone    | r\n",
        );
        assert_eq!(locked_table(&locks), expected);
        assert!(admin_locked_table(&locks).starts_with("   | cluster_id  | locked_at                      | locked_by  | reason  | ttl_in_days\n"));
    }
}
