//! `application domain create|delete|edit|list`.

use crate::{application_flag, context_flags, find_application_or_exit, run_help};
use qovery_api::models::CustomDomain;
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::find_by_custom_domain_name;
use qovery_core::{check_error, client, exit_with_error, resources, telemetry};
use qovery_log::gofmt::go_json;
use qovery_log::pterm::FG_BLUE;
use qovery_services::path_escape;
use serde_json::{Value, json};

const DOMAIN_USAGE: &str = "Custom Domain <subdomain.domain.tld>";

pub(crate) fn command() -> Command {
    Command::new("domain")
        .short("Manage application domains")
        .run(run_help)
        .subcommand(create())
        .subcommand(delete())
        .subcommand(edit())
        .subcommand(list())
}

fn create() -> Command {
    context_flags(Command::new("create").short("Create application custom domain"))
        .flag(application_flag().required())
        .flag(Flag::string("domain", DOMAIN_USAGE).required())
        .flag(Flag::bool(
            "do-not-generate-certificate",
            "Do Not Generate Certificate",
        ))
        .flag(Flag::bool(
            "is-behind-a-cdn",
            "Custom Domain is behind a CDN",
        ))
        .run(run_create)
}

fn delete() -> Command {
    context_flags(Command::new("delete").short("Delete application custom domain"))
        .flag(application_flag().required())
        .flag(Flag::string("domain", DOMAIN_USAGE).required())
        .run(run_delete)
}

fn edit() -> Command {
    context_flags(Command::new("edit").short("Edit application custom domain"))
        .flag(application_flag().required())
        .flag(Flag::string("domain", DOMAIN_USAGE).required())
        .flag(Flag::bool(
            "do-not-generate-certificate",
            "Do Not Generate Certificate",
        ))
        .flag(Flag::bool(
            "is-behind-a-cdn",
            "Custom Domain is behind a CDN",
        ))
        .run(run_edit)
}

fn list() -> Command {
    context_flags(Command::new("list").short("List application domains"))
        .flag(application_flag().required())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

/// Common start: client, ids, application lookup and its custom domains.
fn start(ctx: &Ctx) -> (Client, String, Vec<CustomDomain>) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let application = find_application_or_exit(&client, &env_id, &ctx.string("application"));
    let domains: Vec<CustomDomain> = check_error(client.list(&format!(
        "/application/{}/customDomain",
        path_escape(&application.id)
    )));
    (client, application.id, domains)
}

/// `qovery.CustomDomainRequest`.
fn custom_domain_request(ctx: &Ctx) -> Value {
    json!({
        "domain": ctx.string("domain"),
        "generate_certificate": !ctx.bool("do-not-generate-certificate"),
        "use_cdn": ctx.bool("is-behind-a-cdn"),
    })
}

/// The `*CustomDomain` returned by a create / edit call, decoded like the Go client (error texts of the envelope,
/// required properties and field checks); Go dereferences it, so an empty or `null` body is a nil pointer panic.
fn decode_domain(res: qovery_api::Response) -> CustomDomain {
    match crate::go_decode::go_decode_model(
        &res,
        &["id", "created_at", "domain", "generate_certificate"],
    ) {
        Ok(Some(mut map)) => {
            if let Some(e) = custom_domain_type_error(&map) {
                exit_with_error(e);
            }
            // not a field of the Go model (an additional property, never a decoding error)
            map.remove("name");
            serde_json::from_value(Value::Object(map))
                .unwrap_or_else(|e| exit_with_error(e.to_string()))
        }
        Ok(None) => qovery_envvars::go_nil_pointer_panic(),
        Err(e) => exit_with_error(e),
    }
}

/// Go's decoding error for the fields of `_CustomDomain`, checked in the struct order (`null` keeps the zero
/// value): scalar type errors, `time.Time` parsing and `CustomDomainStatusEnum.UnmarshalJSON` (a plain string
/// decoding, then the allowed values).
fn custom_domain_type_error(map: &serde_json::Map<String, Value>) -> Option<String> {
    const STATUS: &str = "CustomDomainStatusEnum";
    const FIELDS: [(&str, &str); 8] = [
        ("id", "string"),
        ("created_at", "time.Time"),
        ("updated_at", "time.Time"),
        ("domain", "string"),
        ("generate_certificate", "bool"),
        ("use_cdn", "bool"),
        ("validation_domain", "string"),
        ("status", STATUS),
    ];
    FIELDS.iter().find_map(|(field, go_type)| {
        let kind = match map.get(*field)? {
            Value::Null => return None,
            Value::String(_) if *go_type == "string" => return None,
            Value::String(s) if *go_type == "time.Time" => {
                return qovery_log::gotime::parse_rfc3339(s).err();
            }
            Value::String(s) if *go_type == STATUS => {
                return (s != "VALIDATION_PENDING").then(|| format!("{s} is not a valid {STATUS}"));
            }
            Value::Bool(_) if *go_type == "bool" => return None,
            Value::String(_) => "string",
            Value::Bool(_) => "bool",
            Value::Number(_) => "number",
            Value::Array(_) => "array",
            Value::Object(_) => "object",
        };
        if *go_type == STATUS {
            return Some(format!(
                "json: cannot unmarshal {kind} into Go value of type string"
            ));
        }
        Some(format!(
            "json: cannot unmarshal {kind} into Go struct field _CustomDomain.{field} of type {go_type}"
        ))
    })
}

fn run_create(ctx: &Ctx) {
    let (client, application_id, domains) = start(ctx);
    let domain = ctx.string("domain");
    if find_by_custom_domain_name(&domains, &domain).is_some() {
        exit_with_error(format!("custom domain {domain} already exists"));
    }
    let res = check_error(client.execute(
        Method::POST,
        &format!("/application/{}/customDomain", path_escape(&application_id)),
        &[],
        Some(&custom_domain_request(ctx)),
    ));
    let created = decode_domain(res);
    qovery_log::println(format!(
        "Custom domain {} has been created (generate certificate: {})",
        FG_BLUE.sprint(&created.domain),
        FG_BLUE.sprint(created.generate_certificate.unwrap_or(false).to_string())
    ));
}

fn run_delete(ctx: &Ctx) {
    let (client, application_id, domains) = start(ctx);
    let domain = ctx.string("domain");
    let Some(custom_domain) = find_by_custom_domain_name(&domains, &domain) else {
        exit_with_error(format!("custom domain {domain} does not exist"));
    };
    check_error(client.delete(&format!(
        "/application/{}/customDomain/{}",
        path_escape(&application_id),
        path_escape(&custom_domain.id)
    )));
    qovery_log::println(format!(
        "Custom domain {} has been deleted",
        FG_BLUE.sprint(&domain)
    ));
}

fn run_edit(ctx: &Ctx) {
    let (client, application_id, domains) = start(ctx);
    let domain = ctx.string("domain");
    let Some(custom_domain) = find_by_custom_domain_name(&domains, &domain) else {
        exit_with_error(format!("custom domain {domain} does not exist"));
    };
    let res = check_error(client.execute(
        Method::PUT,
        &format!(
            "/application/{}/customDomain/{}",
            path_escape(&application_id),
            path_escape(&custom_domain.id)
        ),
        &[],
        Some(&custom_domain_request(ctx)),
    ));
    let edited = decode_domain(res);
    qovery_log::println(format!(
        "Custom domain {} has been edited (generate certificate: {})",
        FG_BLUE.sprint(&edited.domain),
        FG_BLUE.sprint(edited.generate_certificate.unwrap_or(false).to_string())
    ));
}

fn run_list(ctx: &Ctx) {
    let (client, application_id, domains) = start(ctx);
    let links: Vec<Value> = check_error(client.list(&format!(
        "/application/{}/link",
        path_escape(&application_id)
    )));
    let link_url = |l: &Value| {
        l.get("url")
            .and_then(|u| u.as_str())
            .unwrap_or_default()
            .replace("https://", "")
    };

    if ctx.bool("json") {
        let mut results: Vec<Value> = Vec::new();
        for link in &links {
            results.push(json!({
                "id": null,
                "type": "BUILT_IN_DOMAIN",
                "domain": link_url(link),
                "validation_domain": null,
            }));
        }
        for d in &domains {
            results.push(json!({
                "id": d.id,
                "type": "CUSTOM_DOMAIN",
                "domain": d.domain,
                "validation_domain": validation_domain(d),
            }));
        }
        // a nil slice marshals as null
        let out = if results.is_empty() {
            "null".to_string()
        } else {
            go_json(&Value::Array(results))
        };
        qovery_log::println(out);
        return;
    }

    let mut data: Vec<Vec<String>> = Vec::new();
    let mut custom_domains = std::collections::HashSet::new();
    for d in &domains {
        custom_domains.insert(d.domain.clone());
        data.push(vec![
            d.id.clone(),
            "CUSTOM_DOMAIN".to_string(),
            d.domain.clone(),
            validation_domain(d),
            d.generate_certificate.unwrap_or(false).to_string(),
        ]);
    }
    for link in &links {
        let domain = link_url(link);
        if !custom_domains.contains(&domain) {
            data.push(vec![
                "N/A".into(),
                "BUILT_IN_DOMAIN".into(),
                domain,
                "N/A".into(),
                "N/A".into(),
            ]);
        }
    }
    qovery_log::print_table(
        &[
            "Id",
            "Type",
            "Domain",
            "Validation Domain",
            "Generate Certificate",
        ],
        &data,
    );
}

/// `*customDomain.ValidationDomain`: Go panics on a nil pointer (nil pointer dereference, exit 2).
fn validation_domain(d: &CustomDomain) -> String {
    match &d.validation_domain {
        Some(v) => v.clone(),
        None => qovery_envvars::go_nil_pointer_panic(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn error(body: Value) -> Option<String> {
        custom_domain_type_error(body.as_object().unwrap())
    }

    #[test]
    fn custom_domain_field_errors() {
        let ok = json!({"id": "d", "created_at": "2024-05-01T10:00:00Z", "domain": "x", "generate_certificate": true,
            "updated_at": null, "status": "VALIDATION_PENDING", "name": 5});
        assert_eq!(error(ok), None);
        assert_eq!(
            error(json!({"created_at": "yesterday"})).as_deref(),
            Some(
                r#"parsing time "yesterday" as "2006-01-02T15:04:05Z07:00": cannot parse "yesterday" as "2006""#
            )
        );
        assert_eq!(
            error(json!({"updated_at": 5})).as_deref(),
            Some(
                "json: cannot unmarshal number into Go struct field _CustomDomain.updated_at of type time.Time"
            )
        );
        assert_eq!(
            error(json!({"status": "FOO"})).as_deref(),
            Some("FOO is not a valid CustomDomainStatusEnum")
        );
        assert_eq!(
            error(json!({"status": false})).as_deref(),
            Some("json: cannot unmarshal bool into Go value of type string")
        );
    }
}
