//! `qovery blueprint` command. Port of `cmd/blueprint*.go`.

mod decode;

use qovery_api::{ApiError, path_escape};
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::{check_error, client, exit, resources, telemetry};
use qovery_log::pterm;
use serde_json::{Value, json};

const UPGRADE_MAJOR_LONG: &str = "Point a blueprint service at the latest tag of a newer major version (e.g. PostgreSQL 14 to 18).

Without --major, lists the newer majors and what moving to each one does:
  RELABEL      the service already runs that major (e.g. upgraded on AWS): only Qovery's records change
  UPGRADE      the service runs an older major: the next deploy upgrades it in place (downtime for databases)
  UNSUPPORTED  the move is refused, with the reason";

/// The `blueprint` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("blueprint")
        .short("Manage blueprint services")
        .args(ArgsRule::NoArgs)
        .run(run_blueprint)
        .subcommand(upgrade_major())
}

fn run_blueprint(ctx: &Ctx) {
    telemetry::capture(ctx);
    ctx.help();
}

fn upgrade_major() -> Command {
    Command::new("upgrade-major")
        .short("Point a blueprint at the latest tag of a newer major version")
        .long(UPGRADE_MAJOR_LONG)
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
        .flag(
            Flag::string("name", "Blueprint service name")
                .short('n')
                .required(),
        )
        .flag(Flag::string(
            "major",
            "Major version to move to (e.g. 18). Without it, lists the available majors",
        ))
        .flag(Flag::bool("yes", "Skip the confirmation prompt").short('y'))
        .flag(
            Flag::bool(
                "watch",
                "Watch the service status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_upgrade_major)
}

/// `qovery.BlueprintUpdateNewMajorVersion`.
#[derive(Debug, Clone, Default, PartialEq)]
struct NewMajor {
    service_version: String,
    latest_tag: String,
    mode: String,
    unsupported_reason: String,
}

impl NewMajor {
    fn from_value(v: &Value) -> Self {
        let s = |k: &str| v[k].as_str().unwrap_or_default().to_string();
        NewMajor {
            service_version: s("service_version"),
            latest_tag: s("latest_tag"),
            mode: s("mode"),
            unsupported_reason: s("unsupported_reason"),
        }
    }
}

/// A blueprint materializes as a Terraform or a Helm service; both carry the blueprint they come from.
#[derive(Debug, Clone, Default, PartialEq)]
struct BlueprintService {
    service_id: String,
    name: String,
    blueprint_id: String,
    icon_uri: String,
}

fn run_upgrade_major(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = client::get_client_or_exit();
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);
    let service_name = ctx.string("name");

    let service = check_error(find_blueprint_service(&client, &env_id, &service_name));

    let availability = check_error(check_blueprint_update(&client, &service.blueprint_id));
    let current_tag = availability["current_tag"]
        .as_str()
        .unwrap_or_default()
        .to_string();
    let majors: Vec<NewMajor> = availability["new_major_versions"]
        .as_array()
        .map(|a| a.iter().map(NewMajor::from_value).collect())
        .unwrap_or_default();

    let major = ctx.string("major");
    if major.is_empty() {
        print_new_majors(&service_name, &current_tag, &majors);
        return;
    }

    let target = check_error(pick_major(&majors, &major));
    if let Err(e) = refused_switch(&service_name, &target) {
        qovery_core::exit_with_error(e);
    }

    qovery_log::println(confirmation_text(
        &target.mode,
        major_of_tag(&current_tag),
        &target.service_version,
    ));
    if !ctx.bool("yes") && !confirmed("Do you want to proceed? [Yes/No]") {
        qovery_log::println("Upgrade aborted");
        exit(1);
    }

    let blueprint: Value = check_error(
        client
            .get(&format!(
                "/blueprint/{}",
                path_escape(&service.blueprint_id)
            ))
            .map_err(api_error),
    );
    let name = blueprint["name"].as_str().unwrap_or_default();

    let request = build_update_request(name, &target.latest_tag, &service.icon_uri);
    let _: Value = check_error(
        client
            .patch(
                &format!("/blueprint/{}", path_escape(&service.blueprint_id)),
                &request,
            )
            .map_err(api_error),
    );

    let w = qovery_services::new_services_watch(
        &client,
        &env_id,
        vec![service.service_id.clone()],
        ctx.bool("watch"),
    );
    let ack: Value = check_error(
        client
            .post_empty(&format!(
                "/blueprint/{}/deploy",
                path_escape(&service.blueprint_id)
            ))
            .map_err(api_error),
    );

    qovery_log::println(format!(
        "Request to point {} at {} has been queued (deployment {})",
        pterm::FG_BLUE.sprint(&service_name),
        pterm::FG_BLUE.sprint(&target.latest_tag),
        ack["deployment_id"].as_str().unwrap_or_default()
    ));
    w.wait("DEPLOYED");
}

/// `CheckBlueprintUpdate`: decoded like the Go client (see [`decode`]); a decoding error carries the body.
fn check_blueprint_update(
    client: &qovery_api::Client,
    blueprint_id: &str,
) -> Result<Value, String> {
    let res = client
        .execute(
            qovery_api::Method::GET,
            &format!("/blueprint/{}/update", path_escape(blueprint_id)),
            &[],
            None,
        )
        .map_err(api_error)?;
    let content_type = res
        .headers
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default();
    decode::check_update_response(&res.body, content_type).map_err(|message| {
        api_error(ApiError {
            status: Some(res.status),
            message,
            body: res.text(),
        })
    })?;
    Ok(serde_json::from_slice(&res.body).unwrap_or(Value::Null))
}

fn find_blueprint_service(
    client: &qovery_api::Client,
    env_id: &str,
    name: &str,
) -> Result<BlueprintService, String> {
    let terraforms: Vec<Value> = client
        .list(&format!("/environment/{}/terraform", path_escape(env_id)))
        .map_err(|e| e.to_string())?;
    let helms: Vec<Value> = client
        .list(&format!("/environment/{}/helm", path_escape(env_id)))
        .map_err(|e| e.to_string())?;
    let services: Vec<BlueprintService> = terraforms
        .iter()
        .chain(helms.iter())
        .map(|s| {
            let f = |k: &str| s[k].as_str().unwrap_or_default().to_string();
            BlueprintService {
                service_id: f("id"),
                name: f("name"),
                blueprint_id: f("blueprint_id"),
                icon_uri: f("icon_uri"),
            }
        })
        .collect();
    pick_blueprint_service(&services, name)
}

fn pick_blueprint_service(
    services: &[BlueprintService],
    name: &str,
) -> Result<BlueprintService, String> {
    for service in services {
        if service.name != name {
            continue;
        }
        if service.blueprint_id.is_empty() {
            return Err(format!("{name} is not a blueprint service"));
        }
        return Ok(service.clone());
    }
    Err(format!(
        "no blueprint service named {name} in this environment"
    ))
}

fn print_new_majors(service_name: &str, current_tag: &str, majors: &[NewMajor]) {
    if majors.is_empty() {
        qovery_log::println(format!(
            "{service_name} is already on the latest major ({current_tag})"
        ));
        return;
    }
    qovery_log::println(format!("{service_name} runs {current_tag}. Newer majors:"));
    let data: Vec<Vec<String>> = majors
        .iter()
        .map(|m| {
            vec![
                m.service_version.clone(),
                m.latest_tag.clone(),
                m.mode.clone(),
                m.unsupported_reason.clone(),
            ]
        })
        .collect();
    qovery_log::print_table(&["Major", "Latest tag", "Mode", "Reason"], &data);
    qovery_log::println("Run again with --major <major> to move to one of them.");
}

fn pick_major(majors: &[NewMajor], major: &str) -> Result<NewMajor, String> {
    if let Some(m) = majors.iter().find(|m| m.service_version == major) {
        return Ok(m.clone());
    }
    let available: Vec<&str> = majors.iter().map(|m| m.service_version.as_str()).collect();
    if available.is_empty() {
        return Err(format!(
            "major {major} is not available: the blueprint is already on the latest major"
        ));
    }
    Err(format!(
        "major {major} is not available, choose one of: {}",
        available.join(", ")
    ))
}

fn confirmation_text(mode: &str, current: &str, target: &str) -> String {
    match mode {
        "RELABEL" => format!(
            "The service already runs {target}: only Qovery's records change, the service is not touched."
        ),
        "UPGRADE" => format!(
            "This upgrades the service from {current} to {target} in place on the next deploy (downtime for databases). Review the preview in the Console first if needed."
        ),
        _ => format!("Moving the service from {current} to {target} is not supported."),
    }
}

/// Name and icon are required by the API; variables and spec overrides are left out so they stay unchanged.
fn build_update_request(name: &str, tag: &str, icon: &str) -> Value {
    json!({"name": name, "tag": tag, "icon": icon})
}

/// Catalog tags look like AWS/postgres/14/4.6.0; the third segment is the major.
fn major_of_tag(tag: &str) -> &str {
    let segments: Vec<&str> = tag.split('/').collect();
    if segments.len() < 3 {
        return tag;
    }
    segments[2]
}

/// promptui `Select{Label, Items: [Yes, No]}`; a prompt error prints `Error: <err>` and exits 1.
fn confirmed(label: &str) -> bool {
    let items = ["Yes".to_string(), "No".to_string()];
    let result = qovery_core::select::prompt_select_with_label(label, &items, None).map(|(_, v)| v);
    check_error(result).trim().eq_ignore_ascii_case("yes")
}

/// Surfaces the API's own message (e.g. why a tag change is refused) instead of the bare status line.
fn api_error(e: ApiError) -> String {
    if e.body.is_empty() {
        e.message
    } else {
        format!("{}: {}", e.message, e.body)
    }
}

/// refusedSwitch lets only RELABEL and UPGRADE through: UNSUPPORTED, an empty mode, or one this CLI
/// version doesn't know yet must never reach the update and the deploy.
fn refused_switch(service_name: &str, target: &NewMajor) -> Result<(), String> {
    match target.mode.as_str() {
        "RELABEL" | "UPGRADE" => Ok(()),
        "UNSUPPORTED" => Err(format!(
            "cannot move {service_name} to major {}: {}",
            target.service_version, target.unsupported_reason
        )),
        other => Err(format!(
            "cannot move {service_name} to major {}: unknown switch mode {}, update the Qovery CLI",
            target.service_version,
            qovery_log::gofmt::quote(other)
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn new_major(version: &str, mode: &str) -> NewMajor {
        NewMajor {
            service_version: version.to_string(),
            latest_tag: format!("AWS/postgres/{version}/1.5.0"),
            mode: mode.to_string(),
            unsupported_reason: String::new(),
        }
    }

    #[test]
    fn pick_major_returns_the_requested_major() {
        let majors = vec![new_major("17", "UPGRADE"), new_major("18", "RELABEL")];
        let picked = pick_major(&majors, "18").unwrap();
        assert_eq!(picked.latest_tag, "AWS/postgres/18/1.5.0");
        assert_eq!(picked.mode, "RELABEL");
    }

    #[test]
    fn pick_major_lists_the_available_majors_when_missing() {
        let majors = vec![new_major("17", "UPGRADE"), new_major("18", "UPGRADE")];
        assert_eq!(
            pick_major(&majors, "19").unwrap_err(),
            "major 19 is not available, choose one of: 17, 18"
        );
    }

    #[test]
    fn pick_major_says_when_already_on_the_latest_major() {
        assert_eq!(
            pick_major(&[], "18").unwrap_err(),
            "major 18 is not available: the blueprint is already on the latest major"
        );
    }

    #[test]
    fn confirmation_text_per_mode() {
        assert_eq!(
            confirmation_text("RELABEL", "14", "18"),
            "The service already runs 18: only Qovery's records change, the service is not touched."
        );
        assert_eq!(
            confirmation_text("UPGRADE", "14", "18"),
            "This upgrades the service from 14 to 18 in place on the next deploy (downtime for databases). Review the preview in the Console first if needed."
        );
    }

    #[test]
    fn upgrade_request_leaves_variables_and_overrides_unchanged() {
        let request = build_update_request(
            "orders-db",
            "AWS/postgres/18/1.5.0",
            "app://qovery-console/postgresql",
        );
        assert_eq!(
            qovery_log::gofmt::go_json(&request),
            r#"{"icon":"app://qovery-console/postgresql","name":"orders-db","tag":"AWS/postgres/18/1.5.0"}"#
        );
    }

    #[test]
    fn major_of_tag_works() {
        assert_eq!(major_of_tag("AWS/postgres/14/4.6.0"), "14");
        assert_eq!(major_of_tag("weird"), "weird");
    }

    #[test]
    fn api_error_keeps_plain_errors() {
        assert_eq!(api_error(ApiError::new("boom")), "boom");
        let e = ApiError {
            status: Some(400),
            message: "400 Bad Request".into(),
            body: "{\"m\":1}".into(),
        };
        assert_eq!(api_error(e), "400 Bad Request: {\"m\":1}");
    }

    #[test]
    fn pick_blueprint_service_finds_terraform_and_helm_blueprints() {
        let services = vec![
            BlueprintService {
                service_id: "tf-1".into(),
                name: "orders-db".into(),
                blueprint_id: "bp-1".into(),
                icon_uri: "x".into(),
            },
            BlueprintService {
                service_id: "helm-1".into(),
                name: "cache".into(),
                blueprint_id: "bp-2".into(),
                icon_uri: "y".into(),
            },
        ];
        for name in ["orders-db", "cache"] {
            assert_eq!(pick_blueprint_service(&services, name).unwrap().name, name);
        }
    }

    #[test]
    fn pick_blueprint_service_rejects_a_service_without_blueprint() {
        let services = vec![BlueprintService {
            service_id: "tf-1".into(),
            name: "plain-terraform".into(),
            ..Default::default()
        }];
        assert_eq!(
            pick_blueprint_service(&services, "plain-terraform").unwrap_err(),
            "plain-terraform is not a blueprint service"
        );
    }

    #[test]
    fn pick_blueprint_service_reports_an_unknown_name() {
        assert_eq!(
            pick_blueprint_service(&[], "missing").unwrap_err(),
            "no blueprint service named missing in this environment"
        );
    }

    #[test]
    fn refused_switch_lets_only_relabel_and_upgrade_through() {
        for mode in ["RELABEL", "UPGRADE"] {
            assert!(refused_switch("db", &new_major("18", mode)).is_ok());
        }
        let mut unsupported = new_major("13", "UNSUPPORTED");
        unsupported.unsupported_reason =
            "the target major is lower than the version running".into();
        assert_eq!(
            refused_switch("db", &unsupported).unwrap_err(),
            "cannot move db to major 13: the target major is lower than the version running"
        );
        for mode in ["", "SOMETHING_NEW"] {
            assert!(refused_switch("db", &new_major("18", mode)).is_err());
        }
    }
}
