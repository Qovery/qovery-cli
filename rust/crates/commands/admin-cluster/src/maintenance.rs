//! `qovery admin delete-cluster-undeployed-in-error`, `force-delete-cluster`,
//! `force-delete-old-invalid-credentials-clusters`, `locked`, `notify-users-cluster-failure`, `update`, `update-bulk`.
//! Port of the corresponding `cmd/admin_*.go` files and of `pkg/delete_cluster.go`, `pkg/lock.go`, `pkg/update.go`,
//! `pkg/admin_notify_users_cluster_failure.go`.

use crate::common::{
    HttpResponse, Method, admin_request, admin_url, authorization, tabwriter, validate,
};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_log::{logrus, println_error};
use serde_json::Value;

pub fn delete_cluster_undeployed_in_error() -> Command {
    Command::new("delete-cluster-undeployed-in-error")
        .short("Trigger deletion of all clusters not deployed once and that are in error")
        .run(|_| {
            if validate("delete") {
                let res = delete_with_body(
                    &format!("{}/cluster/deleteNotDeployedInErrorClusters", admin_url()),
                    Method::POST,
                    true,
                    None,
                )
                .expect("dry run disabled");
                if !res.status_line().contains("200") {
                    logrus::error(format!(
                        "Could not delete all clusters undeployed and in error : {}. {}",
                        res.status_line(),
                        res.text()
                    ));
                } else {
                    println!("Clusters deleted: {}", res.text());
                }
            }
        })
}

pub fn force_delete_cluster() -> Command {
    Command::new("force-delete-cluster")
        .short("Force delete cluster by id (only Qovery DB side, without calling the engine)")
        .flag(
            Flag::string("cluster", "Cluster's id")
                .short('c')
                .required(),
        )
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .run(|ctx| {
            let cluster_id = ctx.string("cluster");
            let dry_run_disabled = ctx.bool("disable-dry-run");
            qovery_log::dry_run_print(dry_run_disabled);
            if validate("delete") {
                let res = delete_with_body(
                    &format!("{}/cluster/{}", admin_url(), cluster_id),
                    Method::DELETE,
                    dry_run_disabled,
                    None,
                );
                match res {
                    None => println!("Cluster with id {cluster_id} deletable."),
                    Some(res) if !res.status_line().contains("200") => logrus::error(format!(
                        "Could not delete cluster with id {cluster_id} : {}. {}",
                        res.status_line(),
                        res.text()
                    )),
                    Some(_) => println!("Cluster with id {cluster_id} deleted."),
                }
            }
        })
}

pub fn force_delete_old_invalid_credentials_clusters() -> Command {
    Command::new("force-delete-old-invalid-credentials-clusters")
        .short("Force delete clusters with invalid credentials with last updated date more thant n days")
        .flag(Flag::int("cluster-last-update-in-days", "cluster last update in days").short('d').default_value("30"))
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .run(|ctx| {
            let dry_run_disabled = ctx.bool("disable-dry-run");
            if validate("delete") {
                let mut params = serde_json::Map::new();
                params.insert("last_update_in_days".into(), Value::from(ctx.int("cluster-last-update-in-days")));
                params.insert("dry_run".into(), Value::Bool(!dry_run_disabled));
                let body = qovery_log::gofmt::go_json(&Value::Object(params)).into_bytes();
                let res = delete_with_body(
                    &format!("{}/cluster/deleteOldClustersWithInvalidCredentials", admin_url()),
                    Method::POST,
                    true,
                    Some(body),
                )
                .expect("dry run disabled");
                if !res.status_line().contains("200") {
                    logrus::error(format!(
                        "Could not delete all clusters with invalid credentials : {}. {}",
                        res.status_line(),
                        res.text()
                    ));
                } else if dry_run_disabled {
                    println!("Clusters deleted: {}", res.text());
                } else {
                    println!("Clusters that will be deleted: {}", res.text());
                }
            }
        })
}

/// `pkg.deleteWithBody` (`httpDelete` without body): `None` in dry run.
fn delete_with_body(
    url: &str,
    method: Method,
    dry_run_disabled: bool,
    body: Option<Vec<u8>>,
) -> Option<HttpResponse> {
    let auth = token_or_exit_0();
    if !dry_run_disabled {
        return None;
    }
    Some(admin_request(method, url, &auth, body).unwrap_or_else(|e| logrus::fatal(e)))
}

/// `utils.GetAccessToken(false)`: on error `utils.PrintlnError(err); os.Exit(0)`.
fn token_or_exit_0() -> String {
    match authorization() {
        Ok(a) => a,
        Err(e) => {
            println_error(e);
            qovery_core::exit(0)
        }
    }
}

pub fn locked() -> Command {
    Command::new("locked")
        .short("List locked clusters")
        .run(|_| locked_clusters())
}

/// `pkg.LockedClusters`.
fn locked_clusters() {
    admin_url();
    let auth = token_or_exit_0();
    let res = admin_request(
        Method::GET,
        &format!("{}/cluster/lock", admin_url()),
        &auth,
        None,
    )
    .unwrap_or_else(|e| logrus::fatal(e));
    if res.status != 200 {
        logrus::error(format!(
            "Could not list locked clusters : {}. {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let v: Value =
        serde_json::from_slice(&res.body).unwrap_or_else(|e| logrus::fatal(e.to_string()));
    let format_line = |cells: [&str; 6]| {
        format!(
            "{}\t | {}\t | {}\t | {}\t | {}\t | {}\n",
            cells[0], cells[1], cells[2], cells[3], cells[4], cells[5]
        )
    };
    let mut lines = vec![format_line([
        "",
        "cluster_id",
        "locked_at",
        "locked_by",
        "reason",
        "ttl_in_days",
    ])];
    let results = crate::common::go_field(&v, "results")
        .and_then(|r| r.as_array())
        .cloned()
        .unwrap_or_default();
    for (idx, lock) in results.iter().enumerate() {
        let s = |k: &str| {
            crate::common::go_field(lock, k)
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string()
        };
        let ttl = crate::common::go_field(lock, "ttl_in_days")
            .and_then(|t| t.as_i64())
            .map(|t| t.to_string())
            .unwrap_or_else(|| "infinite".to_string());
        let locked_at = crate::common::go_field(lock, "locked_at")
            .and_then(|t| t.as_str())
            .and_then(|t| chrono::DateTime::parse_from_rfc3339(t).ok())
            .map(|t| crate::common::rfc1123(&t))
            .unwrap_or_else(|| "Mon, 01 Jan 0001 00:00:00 UTC".to_string());
        let n = (idx + 1).to_string();
        lines.push(format_line([
            &n,
            &s("cluster_id"),
            &locked_at,
            &s("owner_name"),
            &s("reason"),
            &ttl,
        ]));
    }
    print!("{}", tabwriter(&lines, 1, 1));
}

pub fn notify_users_cluster_failure() -> Command {
    Command::new("notify-users-cluster-failure")
        .short("Notify users of a cluster failure")
        .long(
            "Notify users by email of a cluster having FAILED status.
- (Default) With --cluster-id, only admins of the cluster with the given id will be notified.
- Without --cluster-id, admins of all clusters with FAILED status will be notified.
",
        )
        .flag(Flag::string("cluster-id", "Cluster ID").short('c'))
        .run(run_notify)
}

fn run_notify(ctx: &Ctx) {
    admin_url();
    // the cluster id pointer given by the command is never nil
    let body = format!("{{\"cluster_ids\": [\"{}\"]}}", ctx.string("cluster-id"));
    let notify = || -> Result<(), String> {
        let auth = authorization()?;
        let res = admin_request(
            Method::POST,
            &format!("{}/cluster/notifyFailedClustersAdmins", admin_url()),
            &auth,
            Some(body.into_bytes()),
        )?;
        if !res.status_line().contains("200") {
            return Err(format!(
                "could not notify (error {}: {})",
                res.status_line(),
                res.text()
            ));
        }
        qovery_log::println(format!(
            "Notification sent for admins of these clusters {}",
            res.text()
        ));
        Ok(())
    };
    if let Err(e) = notify() {
        qovery_core::exit_with_error(e);
    }
}

pub fn update() -> Command {
    Command::new("update")
        .short("Update cluster with its Id to a specific version")
        .flag(
            Flag::string("cluster", "Cluster's id")
                .short('c')
                .required(),
        )
        .flag(
            Flag::string("version", "Targeted version")
                .short('v')
                .required(),
        )
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .run(|ctx| {
            let cluster_id = ctx.string("cluster");
            let dry_run_disabled = ctx.bool("disable-dry-run");
            qovery_log::dry_run_print(dry_run_disabled);
            if validate("update") {
                let res = update_request(
                    &format!("{}/cluster/update/{}", admin_url(), cluster_id),
                    dry_run_disabled,
                    &ctx.string("version"),
                    "",
                    0,
                );
                if !res.status_line().contains("200") {
                    logrus::error(format!(
                        "Could not update cluster : {}. {}",
                        res.status_line(),
                        res.text()
                    ));
                } else if !dry_run_disabled {
                    println!("Cluster {cluster_id} updatable.");
                } else {
                    println!("Cluster {cluster_id} updating.");
                }
            }
        })
}

pub fn update_bulk() -> Command {
    Command::new("update-bulk")
        .short("Update an amount of clusters to a specific version based on cloud provider kind.")
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .flag(
            Flag::string("version", "Targeted version")
                .short('v')
                .required(),
        )
        .flag(
            Flag::string(
                "provider-kind",
                "Provider to upgrade. Can be : AWS, AZURE, GCP or SCW",
            )
            .short('k')
            .required(),
        )
        .flag(
            Flag::int("parallel-run", "Number of parallel upgrades. Max is 20.")
                .short('p')
                .default_value("1"),
        )
        .run(|ctx| {
            let parallel_run = ctx.int("parallel-run");
            if parallel_run > 20 {
                logrus::error("Can't update more than 20 clusters");
                return;
            }
            let dry_run_disabled = ctx.bool("disable-dry-run");
            qovery_log::dry_run_print(dry_run_disabled);
            if validate("update") {
                let res = update_request(
                    &format!("{}/cluster/update", admin_url()),
                    dry_run_disabled,
                    &ctx.string("version"),
                    &ctx.string("provider-kind"),
                    parallel_run,
                );
                let status = res.status_line();
                if status.contains("40") || status.contains("50") {
                    logrus::error(format!(
                        "Could not update clusters : {}. {}",
                        status,
                        res.text()
                    ));
                } else {
                    let depl = if dry_run_disabled {
                        "Deploying"
                    } else {
                        "Deployable"
                    };
                    logrus::info(format!("{depl} clusters: {}", res.text()));
                }
            }
        })
}

/// `pkg.update`.
fn update_request(
    url: &str,
    dry_run_disabled: bool,
    version: &str,
    provider_kind: &str,
    parallel_run: i64,
) -> HttpResponse {
    let auth = token_or_exit_0();
    let content = format!(
        r#"{{ "metadata": {{ "dry_run_deploy": {}, "target_version": "{}", "provider_kind": "{}", "parallel_run": {} }} }}"#,
        !dry_run_disabled, version, provider_kind, parallel_run
    );
    admin_request(Method::POST, url, &auth, Some(content.into_bytes()))
        .unwrap_or_else(|e| logrus::fatal(e))
}
