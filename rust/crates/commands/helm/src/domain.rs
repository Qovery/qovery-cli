//! `helm domain ...`: helm_domain.go, helm_domain_list.go, helm_container_create.go (`domain create`),
//! helm_domain_edit.go, hem_domain_delete.go.

use crate::{context_flags, run_group};
use qovery_api::models::CustomDomain;
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::find_by_custom_domain_name;
use qovery_core::{check_error, exit_with_error, telemetry};
use qovery_log::gofmt::go_json;
use qovery_log::pterm::FG_BLUE;
use qovery_services::path_escape;
use serde_json::{Value, json};
use std::collections::HashSet;

pub(crate) fn command() -> Command {
    Command::new("domain")
        .short("Manage helm domains")
        .run(run_group)
        .subcommand(create())
        .subcommand(delete())
        .subcommand(edit())
        .subcommand(list())
}

fn helm_and_domain_flags(cmd: Command) -> Command {
    context_flags(cmd)
        .flag(Flag::string("helm", "helm Name").short('n').required())
        .flag(Flag::string("domain", "Custom Domain <subdomain.domain.tld>").required())
}

fn certificate_flags(cmd: Command) -> Command {
    cmd.flag(Flag::bool(
        "do-not-generate-certificate",
        "Do Not Generate Certificate",
    ))
    .flag(Flag::bool(
        "is-behind-a-cdn",
        "Custom Domain is behind a CDN",
    ))
}

fn list_custom_domains(client: &Client, helm_id: &str) -> Vec<CustomDomain> {
    check_error(client.list(&format!("/helm/{}/customDomain", path_escape(helm_id))))
}

/// Go `*customDomain.ValidationDomain` (nil pointer panic when missing).
fn validation_domain(d: &CustomDomain) -> String {
    match &d.validation_domain {
        Some(v) => v.clone(),
        None => qovery_envvars::go_nil_pointer_panic(),
    }
}

/// `qovery.CustomDomainRequest` built from `--domain`, `--do-not-generate-certificate`, `--is-behind-a-cdn`.
fn custom_domain_request(ctx: &Ctx) -> Value {
    json!({
        "domain": ctx.string("domain"),
        "generate_certificate": !ctx.bool("do-not-generate-certificate"),
        "use_cdn": ctx.bool("is-behind-a-cdn"),
    })
}

/// `Custom domain X has been <verb> (generate certificate: B)` from the API response.
fn print_domain_result(verb: &str, res: &Value) {
    let domain = res
        .get("domain")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let generate = res
        .get("generate_certificate")
        .and_then(Value::as_bool)
        .unwrap_or_default();
    qovery_log::println(format!(
        "Custom domain {} has been {verb} (generate certificate: {})",
        FG_BLUE.sprint(domain),
        FG_BLUE.sprint(generate.to_string())
    ));
}

// ---------------------------------------------------------------------------------------------------------

fn list() -> Command {
    context_flags(Command::new("list").short("List helm domains"))
        .flag(Flag::string("helm", "helm Name").short('n').required())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    let custom_domains = list_custom_domains(&client, &helm.id);

    let mut custom_domains_set = HashSet::new();
    let mut data: Vec<Vec<String>> = Vec::new();
    for d in &custom_domains {
        custom_domains_set.insert(d.domain.clone());
        data.push(vec![
            d.id.clone(),
            "CUSTOM_DOMAIN".to_string(),
            d.domain.clone(),
            validation_domain(d),
            d.generate_certificate.unwrap_or_default().to_string(),
        ]);
    }

    let links: Vec<Value> =
        check_error(client.list(&format!("/helm/{}/link", path_escape(&helm.id))));
    let link_url = |l: &Value| {
        l.get("url")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .replace("https://", "")
    };

    if ctx.bool("json") {
        let mut results = Vec::new();
        for l in &links {
            results.push(json!({"id": null, "type": "BUILT_IN_DOMAIN", "domain": link_url(l), "validation_domain": null}));
        }
        for d in &custom_domains {
            results.push(json!({
                "id": d.id,
                "type": "CUSTOM_DOMAIN",
                "domain": d.domain,
                "validation_domain": validation_domain(d),
            }));
        }
        let v = if results.is_empty() {
            Value::Null
        } else {
            Value::Array(results)
        };
        qovery_log::println(go_json(&v));
        return;
    }

    for l in &links {
        let domain = link_url(l);
        if !custom_domains_set.contains(&domain) {
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

// ---------------------------------------------------------------------------------------------------------

fn create() -> Command {
    certificate_flags(helm_and_domain_flags(
        Command::new("create").short("Create helm custom domain"),
    ))
    .run(run_create)
}

fn run_create(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    let custom_domains = list_custom_domains(&client, &helm.id);
    let domain = ctx.string("domain");
    if find_by_custom_domain_name(&custom_domains, &domain).is_some() {
        exit_with_error(format!("custom domain {domain} already exists"));
    }
    let res: Value = check_error(client.post(
        &format!("/helm/{}/customDomain", path_escape(&helm.id)),
        &custom_domain_request(ctx),
    ));
    print_domain_result("created", &res);
}

// ---------------------------------------------------------------------------------------------------------

fn edit() -> Command {
    certificate_flags(helm_and_domain_flags(
        Command::new("edit").short("Edit helm custom domain"),
    ))
    .run(run_edit)
}

/// The custom domain `--domain` of the helm, or `custom domain X does not exist` (exit 1).
fn existing_domain(client: &Client, helm_id: &str, domain: &str) -> CustomDomain {
    let custom_domains = list_custom_domains(client, helm_id);
    match find_by_custom_domain_name(&custom_domains, domain) {
        Some(d) => d.clone(),
        None => exit_with_error(format!("custom domain {domain} does not exist")),
    }
}

fn run_edit(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    let custom_domain = existing_domain(&client, &helm.id, &ctx.string("domain"));
    let res: Value = check_error(client.put(
        &format!(
            "/helm/{}/customDomain/{}",
            path_escape(&helm.id),
            path_escape(&custom_domain.id)
        ),
        &custom_domain_request(ctx),
    ));
    print_domain_result("edited", &res);
}

// ---------------------------------------------------------------------------------------------------------

fn delete() -> Command {
    helm_and_domain_flags(Command::new("delete").short("Delete helm custom domain")).run(run_delete)
}

fn run_delete(ctx: &Ctx) {
    telemetry::capture(ctx);
    let (client, _, helm) = crate::resolve_helm(ctx);
    let domain = ctx.string("domain");
    let custom_domain = existing_domain(&client, &helm.id, &domain);
    check_error(client.execute(
        Method::DELETE,
        &format!(
            "/helm/{}/customDomain/{}",
            path_escape(&helm.id),
            path_escape(&custom_domain.id)
        ),
        &[],
        None,
    ));
    qovery_log::println(format!(
        "Custom domain {} has been deleted",
        FG_BLUE.sprint(&domain)
    ));
}
