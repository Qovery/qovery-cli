//! `qovery admin cluster profile [get|set]`. Port of `cmd/admin_cluster_profile.go`.

use crate::common::{Method, http_request, validate};
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_log::gofmt::{go_json, quote};
use serde_json::Value;
use std::time::Duration;

const RESOURCE_PROFILES: &[&str] = &["LOW", "NORMAL", "HIGH"];
const KEDA_AVAILABILITY_PROFILES: &[&str] = &["NORMAL", "HIGH"];
const OBSERVABILITY_RESOURCE_FLAG: &str = "observability-resource-profile";
const OBSERVABILITY_HA_FLAG: &str = "observability-high-availability";
const KEDA_RESOURCE_FLAG: &str = "keda-resource-profile";
const KEDA_AVAILABILITY_FLAG: &str = "keda-availability-profile";

pub fn command() -> Command {
    Command::new("profile")
        .short("Read or change the observability and KEDA profiles of a cluster")
        .args(ArgsRule::NoArgs)
        .run(|ctx| {
            qovery_core::telemetry::capture(ctx);
            ctx.help();
            qovery_core::exit(0)
        })
        .subcommand(with_target_flags(
            Command::new("get").short("Show the observability and KEDA profiles of a cluster").args(ArgsRule::NoArgs).run(run_get),
        ))
        .subcommand(
            with_target_flags(
                Command::new("set")
                    .short("Change the observability and KEDA profiles of a cluster")
                    .long(
                        "Change the observability and KEDA profiles of a cluster.

Only the profiles given as flags change. The profile of an add-on that is not enabled cannot be changed.
The new profiles apply at the next cluster deployment: use --deploy to trigger it.

Examples:
qovery admin cluster profile set --organization-id <org-id> --cluster-id <cluster-id> --observability-resource-profile NORMAL
qovery admin cluster profile set --organization-id <org-id> --cluster-id <cluster-id> --observability-resource-profile NORMAL --observability-high-availability=false --deploy
qovery admin cluster profile set --organization-id <org-id> --cluster-id <cluster-id> --keda-resource-profile HIGH --keda-availability-profile HIGH",
                    )
                    .args(ArgsRule::NoArgs)
                    .run(run_set),
            )
            .flag(Flag::string(OBSERVABILITY_RESOURCE_FLAG, &format!("Observability resource profile: {}", RESOURCE_PROFILES.join(", "))))
            .flag(Flag::bool(OBSERVABILITY_HA_FLAG, "Run the observability stack in high availability"))
            .flag(Flag::string(KEDA_RESOURCE_FLAG, &format!("KEDA resource profile: {}", RESOURCE_PROFILES.join(", "))))
            .flag(Flag::string(
                KEDA_AVAILABILITY_FLAG,
                &format!("KEDA availability profile: {}", KEDA_AVAILABILITY_PROFILES.join(", ")),
            ))
            .flag(Flag::bool("deploy", "Deploy the cluster after the update"))
            .flag(Flag::bool("no-confirm", "Do not ask for confirmation").short('c')),
        )
}

fn with_target_flags(c: Command) -> Command {
    c.flag(Flag::string("organization-id", "The organization of the cluster").required())
        .flag(Flag::string("cluster-id", "The cluster to target").required())
}

/// `parseProfile`.
pub fn parse_profile(flag: &str, value: &str, allowed: &[&str]) -> Result<String, String> {
    let profile = value.to_uppercase();
    if !allowed.contains(&profile.as_str()) {
        return Err(format!(
            "invalid --{flag} {}: must be one of {}",
            quote(value),
            allowed.join(", ")
        ));
    }
    Ok(profile)
}

/// `clusterProfileChanges`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Changes {
    pub observability_resource_profile: Option<String>,
    pub observability_high_availability: Option<bool>,
    pub keda_resource_profile: Option<String>,
    pub keda_availability_profile: Option<String>,
}

impl Changes {
    fn is_empty(&self) -> bool {
        *self == Changes::default()
    }

    /// `savedIn`.
    fn saved_in(&self, p: &Profiles) -> bool {
        if let Some(v) = &self.observability_resource_profile
            && *v != p.observability_resource_profile
        {
            return false;
        }
        if let Some(v) = self.observability_high_availability
            && p.observability_high_availability != Some(v)
        {
            return false;
        }
        if let Some(v) = &self.keda_resource_profile
            && *v != p.keda_resource_profile
        {
            return false;
        }
        if let Some(v) = &self.keda_availability_profile
            && *v != p.keda_availability_profile
        {
            return false;
        }
        true
    }
}

/// `clusterProfiles`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Profiles {
    pub observability_enabled: bool,
    pub observability_resource_profile: String,
    pub observability_high_availability: Option<bool>,
    pub keda_enabled: bool,
    pub keda_resource_profile: String,
    pub keda_availability_profile: String,
}

impl Profiles {
    pub fn rows(&self) -> Vec<Vec<String>> {
        let ha = self
            .observability_high_availability
            .map(crate::common::format_bool)
            .unwrap_or_default();
        vec![
            vec![
                "observability.enabled".into(),
                crate::common::format_bool(self.observability_enabled),
            ],
            vec![
                "observability.resource_profile".into(),
                self.observability_resource_profile.clone(),
            ],
            vec!["observability.high_availability".into(), ha],
            vec![
                "keda.enabled".into(),
                crate::common::format_bool(self.keda_enabled),
            ],
            vec![
                "keda.resource_profile".into(),
                self.keda_resource_profile.clone(),
            ],
            vec![
                "keda.availability_profile".into(),
                self.keda_availability_profile.clone(),
            ],
        ]
    }
}

/// `jsonObject(parent, key)`: the object at `key` (or nothing).
fn object<'a>(parent: Option<&'a Value>, key: &str) -> Option<&'a Value> {
    parent
        .and_then(|p| p.as_object())
        .and_then(|o| o.get(key))
        .filter(|v| v.is_object())
}

/// `readClusterProfiles`.
pub fn read_profiles(cluster: &Value) -> Profiles {
    let metrics = object(Some(cluster), "metrics_parameters");
    let configuration = object(metrics, "configuration");
    let keda = object(Some(cluster), "keda");
    let s = |o: Option<&Value>, k: &str| {
        o.and_then(|o| o.get(k))
            .and_then(|v| v.as_str())
            .unwrap_or_default()
            .to_string()
    };
    let b = |o: Option<&Value>, k: &str| o.and_then(|o| o.get(k)).and_then(|v| v.as_bool());
    Profiles {
        observability_enabled: b(metrics, "enabled").unwrap_or(false),
        observability_resource_profile: s(configuration, "resource_profile"),
        observability_high_availability: b(configuration, "high_availability"),
        keda_enabled: b(keda, "enabled").unwrap_or(false),
        keda_resource_profile: s(keda, "resource_profile"),
        keda_availability_profile: s(keda, "availability_profile"),
    }
}

/// `applyClusterProfileChanges`.
pub fn apply_changes(cluster: &mut Value, changes: &Changes) -> Result<(), String> {
    let current = read_profiles(cluster);
    if changes.observability_resource_profile.is_some()
        || changes.observability_high_availability.is_some()
    {
        let has_configuration =
            object(object(Some(cluster), "metrics_parameters"), "configuration").is_some();
        if !current.observability_enabled || !has_configuration {
            return Err(
                "observability is not enabled on this cluster: its profile cannot be changed"
                    .to_string(),
            );
        }
        let configuration = &mut cluster["metrics_parameters"]["configuration"];
        if let Some(p) = &changes.observability_resource_profile {
            configuration["resource_profile"] = Value::String(p.clone());
        }
        if let Some(ha) = changes.observability_high_availability {
            configuration["high_availability"] = Value::Bool(ha);
        }
    }
    if changes.keda_resource_profile.is_some() || changes.keda_availability_profile.is_some() {
        if !current.keda_enabled {
            return Err(
                "KEDA is not enabled on this cluster: its profile cannot be changed".to_string(),
            );
        }
        let keda = &mut cluster["keda"];
        if let Some(p) = &changes.keda_resource_profile {
            keda["resource_profile"] = Value::String(p.clone());
        }
        if let Some(p) = &changes.keda_availability_profile {
            keda["availability_profile"] = Value::String(p.clone());
        }
    }
    Ok(())
}

/// The calls of `clusterProfileAPI`.
pub trait ProfileApi {
    fn cluster(&self, organization_id: &str, cluster_id: &str) -> Result<Value, String>;
    fn edit_cluster(
        &self,
        organization_id: &str,
        cluster_id: &str,
        cluster: &Value,
    ) -> Result<(), String>;
    fn deploy_cluster(&self, organization_id: &str, cluster_id: &str) -> Result<(), String>;
}

/// `clusterProfileAPI` (raw HTTP on the Qovery API).
pub struct HttpProfileApi {
    base_url: String,
    authorization: String,
}

impl HttpProfileApi {
    fn new() -> Result<Self, String> {
        Ok(HttpProfileApi {
            base_url: qovery_api::api_base_url(),
            authorization: crate::common::authorization()?,
        })
    }

    fn call(&self, method: Method, path: &str, body: Option<&Value>) -> Result<Vec<u8>, String> {
        let mut headers = vec![("Authorization", self.authorization.as_str())];
        if body.is_some() {
            headers.push(("Content-Type", "application/json"));
        }
        let res = http_request(
            method.clone(),
            &format!("{}{}", self.base_url, path),
            &headers,
            body.map(|b| go_json(b).into_bytes()),
            Some(Duration::from_secs(60)),
        )?;
        if !(200..300).contains(&res.status) {
            return Err(format!(
                "{} {}: {}: {}",
                method.as_str(),
                path,
                res.status_line(),
                res.text().trim()
            ));
        }
        Ok(res.body)
    }
}

impl ProfileApi for HttpProfileApi {
    fn cluster(&self, organization_id: &str, cluster_id: &str) -> Result<Value, String> {
        let content = self.call(
            Method::GET,
            &format!("/organization/{organization_id}/cluster"),
            None,
        )?;
        let list: Value = serde_json::from_slice(&content).map_err(|e| e.to_string())?;
        find_cluster(&list, organization_id, cluster_id)
    }

    fn edit_cluster(
        &self,
        organization_id: &str,
        cluster_id: &str,
        cluster: &Value,
    ) -> Result<(), String> {
        self.call(
            Method::PUT,
            &format!("/organization/{organization_id}/cluster/{cluster_id}"),
            Some(cluster),
        )
        .map(|_| ())
    }

    fn deploy_cluster(&self, organization_id: &str, cluster_id: &str) -> Result<(), String> {
        self.call(
            Method::POST,
            &format!("/organization/{organization_id}/cluster/{cluster_id}/deploy"),
            None,
        )
        .map(|_| ())
    }
}

fn find_cluster(list: &Value, organization_id: &str, cluster_id: &str) -> Result<Value, String> {
    if let Some(results) = list.get("results").and_then(|r| r.as_array()) {
        for c in results {
            if c.get("id").and_then(|i| i.as_str()) == Some(cluster_id) {
                return Ok(c.clone());
            }
        }
    }
    Err(format!(
        "cluster {cluster_id} not found in organization {organization_id}"
    ))
}

/// `updateClusterProfiles`.
pub fn update_profiles(
    api: &dyn ProfileApi,
    organization_id: &str,
    cluster_id: &str,
    confirmed: &Profiles,
    changes: &Changes,
) -> Result<(), String> {
    let mut cluster = api.cluster(organization_id, cluster_id)?;
    if read_profiles(&cluster).rows() != confirmed.rows() {
        return Err("the cluster profiles changed since the confirmation: nothing was updated, run the command again".to_string());
    }
    apply_changes(&mut cluster, changes)?;
    api.edit_cluster(organization_id, cluster_id, &cluster)?;
    let saved = api.cluster(organization_id, cluster_id)?;
    if !changes.saved_in(&read_profiles(&saved)) {
        return Err("the API accepted the update but did not save the new profiles".to_string());
    }
    Ok(())
}

/// `clusterProfileChangeRows`.
pub fn change_rows(before: &Profiles, after: &Profiles) -> Vec<Vec<String>> {
    let mut rows = before.rows();
    for (i, row) in after.rows().into_iter().enumerate() {
        rows[i].push(row[1].clone());
    }
    rows
}

fn changes_from_flags(ctx: &Ctx) -> Result<Changes, String> {
    let mut changes = Changes::default();
    if ctx.changed(OBSERVABILITY_RESOURCE_FLAG) {
        changes.observability_resource_profile = Some(parse_profile(
            OBSERVABILITY_RESOURCE_FLAG,
            &ctx.string(OBSERVABILITY_RESOURCE_FLAG),
            RESOURCE_PROFILES,
        )?);
    }
    if ctx.changed(OBSERVABILITY_HA_FLAG) {
        changes.observability_high_availability = Some(ctx.bool(OBSERVABILITY_HA_FLAG));
    }
    if ctx.changed(KEDA_RESOURCE_FLAG) {
        changes.keda_resource_profile = Some(parse_profile(
            KEDA_RESOURCE_FLAG,
            &ctx.string(KEDA_RESOURCE_FLAG),
            RESOURCE_PROFILES,
        )?);
    }
    if ctx.changed(KEDA_AVAILABILITY_FLAG) {
        changes.keda_availability_profile = Some(parse_profile(
            KEDA_AVAILABILITY_FLAG,
            &ctx.string(KEDA_AVAILABILITY_FLAG),
            KEDA_AVAILABILITY_PROFILES,
        )?);
    }
    if changes.is_empty() {
        return Err("nothing to change: set at least one profile flag".to_string());
    }
    Ok(changes)
}

fn check<T>(r: Result<T, String>) -> T {
    qovery_core::check_error(r)
}

fn run_get(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    let api = check(HttpProfileApi::new());
    let cluster = check(api.cluster(&ctx.string("organization-id"), &ctx.string("cluster-id")));
    qovery_log::print_table(&["Setting", "Value"], &read_profiles(&cluster).rows());
}

fn run_set(ctx: &Ctx) {
    qovery_core::telemetry::capture(ctx);
    let changes = check(changes_from_flags(ctx));
    let organization_id = ctx.string("organization-id");
    let cluster_id = ctx.string("cluster-id");
    let api = check(HttpProfileApi::new());
    let mut cluster = check(api.cluster(&organization_id, &cluster_id));
    let before = read_profiles(&cluster);
    check(apply_changes(&mut cluster, &changes));
    qovery_log::print_table(
        &["Setting", "Current", "New"],
        &change_rows(&before, &read_profiles(&cluster)),
    );

    if !ctx.bool("no-confirm") && !validate("profile update") {
        qovery_log::println_info("profile update cancelled");
        return;
    }
    check(update_profiles(
        &api,
        &organization_id,
        &cluster_id,
        &before,
        &changes,
    ));
    qovery_log::println_info("profiles updated");
    if !ctx.bool("deploy") {
        qovery_log::println_info("the new profiles apply at the next cluster deployment");
        return;
    }
    check(api.deploy_cluster(&organization_id, &cluster_id));
    qovery_log::println_info("cluster deployment triggered");
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::RefCell;

    fn cluster_json() -> Value {
        serde_json::json!({
            "id": "cluster-1",
            "name": "my-cluster",
            "max_running_nodes": 10,
            "metrics_parameters": {"enabled": true, "configuration": {"kind": "MANAGED_BY_QOVERY", "resource_profile": "LOW", "high_availability": true, "cloud_watch_export_config": {"enabled": false}}},
            "keda": {"enabled": true, "resource_profile": "NORMAL", "availability_profile": "NORMAL"}
        })
    }

    #[test]
    fn parse_profiles() {
        assert_eq!(
            parse_profile("keda-resource-profile", "high", RESOURCE_PROFILES).unwrap(),
            "HIGH"
        );
        assert_eq!(
            parse_profile(
                "keda-availability-profile",
                "low",
                KEDA_AVAILABILITY_PROFILES
            )
            .unwrap_err(),
            "invalid --keda-availability-profile \"low\": must be one of NORMAL, HIGH"
        );
    }

    #[test]
    fn read_cluster_profiles() {
        let p = read_profiles(&cluster_json());
        assert_eq!(
            p,
            Profiles {
                observability_enabled: true,
                observability_resource_profile: "LOW".into(),
                observability_high_availability: Some(true),
                keda_enabled: true,
                keda_resource_profile: "NORMAL".into(),
                keda_availability_profile: "NORMAL".into(),
            }
        );
        let empty = read_profiles(&serde_json::json!({"id": "cluster-1"}));
        assert_eq!(empty, Profiles::default());
        assert_eq!(empty.rows()[2][1], "");
    }

    #[test]
    fn apply_only_touches_requested_fields() {
        let mut c = cluster_json();
        let changes = Changes {
            observability_resource_profile: Some("NORMAL".into()),
            keda_availability_profile: Some("HIGH".into()),
            ..Default::default()
        };
        apply_changes(&mut c, &changes).unwrap();
        let mut expected = cluster_json();
        expected["metrics_parameters"]["configuration"]["resource_profile"] = "NORMAL".into();
        expected["keda"]["availability_profile"] = "HIGH".into();
        assert_eq!(c, expected);
    }

    #[test]
    fn apply_refuses_disabled_addons() {
        let mut c = serde_json::json!({"metrics_parameters": {"enabled": false}, "keda": {"enabled": false}});
        let e = apply_changes(
            &mut c,
            &Changes {
                observability_high_availability: Some(false),
                ..Default::default()
            },
        )
        .unwrap_err();
        assert!(e.contains("observability is not enabled"));
        let e = apply_changes(
            &mut c,
            &Changes {
                keda_resource_profile: Some("HIGH".into()),
                ..Default::default()
            },
        )
        .unwrap_err();
        assert!(e.contains("KEDA is not enabled"));
    }

    struct FakeApi {
        gets: RefCell<Vec<Value>>,
        puts: RefCell<Vec<Value>>,
        save: bool,
    }

    impl ProfileApi for FakeApi {
        fn cluster(&self, _: &str, _: &str) -> Result<Value, String> {
            let mut gets = self.gets.borrow_mut();
            Ok(if gets.len() > 1 {
                gets.remove(0)
            } else {
                gets[0].clone()
            })
        }
        fn edit_cluster(&self, _: &str, _: &str, cluster: &Value) -> Result<(), String> {
            self.puts.borrow_mut().push(cluster.clone());
            if self.save {
                let mut gets = self.gets.borrow_mut();
                gets.clear();
                gets.push(cluster.clone());
            }
            Ok(())
        }
        fn deploy_cluster(&self, _: &str, _: &str) -> Result<(), String> {
            Ok(())
        }
    }

    #[test]
    fn update_sends_the_whole_cluster_and_checks_the_result() {
        let api = FakeApi {
            gets: RefCell::new(vec![cluster_json()]),
            puts: RefCell::new(vec![]),
            save: true,
        };
        let changes = Changes {
            observability_resource_profile: Some("HIGH".into()),
            ..Default::default()
        };
        update_profiles(
            &api,
            "org-1",
            "cluster-1",
            &read_profiles(&cluster_json()),
            &changes,
        )
        .unwrap();
        let put = &api.puts.borrow()[0];
        assert_eq!(put["max_running_nodes"], 10);
        assert_eq!(
            put["metrics_parameters"]["configuration"]["resource_profile"],
            "HIGH"
        );
        assert_eq!(
            put["metrics_parameters"]["configuration"]["cloud_watch_export_config"]["enabled"],
            false
        );
    }

    #[test]
    fn update_refuses_profiles_changed_since_confirmation() {
        let mut changed = cluster_json();
        changed["keda"]["resource_profile"] = "HIGH".into();
        let api = FakeApi {
            gets: RefCell::new(vec![changed]),
            puts: RefCell::new(vec![]),
            save: true,
        };
        let changes = Changes {
            observability_resource_profile: Some("HIGH".into()),
            ..Default::default()
        };
        let e =
            update_profiles(&api, "o", "c", &read_profiles(&cluster_json()), &changes).unwrap_err();
        assert!(e.contains("changed since the confirmation"));
        assert!(api.puts.borrow().is_empty());
    }

    #[test]
    fn update_fails_when_the_api_does_not_save() {
        let api = FakeApi {
            gets: RefCell::new(vec![cluster_json()]),
            puts: RefCell::new(vec![]),
            save: false,
        };
        let changes = Changes {
            observability_resource_profile: Some("HIGH".into()),
            ..Default::default()
        };
        let e =
            update_profiles(&api, "o", "c", &read_profiles(&cluster_json()), &changes).unwrap_err();
        assert_eq!(
            e,
            "the API accepted the update but did not save the new profiles"
        );
    }

    #[test]
    fn change_rows_add_the_new_column() {
        let before = read_profiles(&cluster_json());
        let mut after = before.clone();
        after.keda_resource_profile = "HIGH".into();
        let rows = change_rows(&before, &after);
        assert_eq!(rows[4], vec!["keda.resource_profile", "NORMAL", "HIGH"]);
        assert_eq!(rows[0], vec!["observability.enabled", "true", "true"]);
    }

    #[test]
    fn find_cluster_in_list() {
        let list = serde_json::json!({"results": [{"id": "a"}, {"id": "cluster-1", "name": "x"}]});
        assert_eq!(find_cluster(&list, "o", "cluster-1").unwrap()["name"], "x");
        assert_eq!(
            find_cluster(&list, "o", "nope").unwrap_err(),
            "cluster nope not found in organization o"
        );
    }
}
