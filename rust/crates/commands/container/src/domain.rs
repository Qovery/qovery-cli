//! `qovery container domain list|create|edit|delete`. Port of `cmd/container_domain*.go`.

use crate::{
    client_or_exit, container_flag, fail, find_container_or_exit, ids_or_exit, run_group_help,
    with_context_flags,
};
use qovery_api::models::{Container, CustomDomain};
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::find_by_custom_domain_name;
use qovery_core::{check_error, telemetry};
use qovery_log::gofmt::go_json;
use qovery_log::pterm::FG_BLUE;
use serde_json::{Value, json};
use std::collections::HashSet;

const DOMAIN_USAGE: &str = "Custom Domain <subdomain.domain.tld>";

pub fn command() -> Command {
    Command::new("domain")
        .short("Manage container domains")
        .run(run_group_help)
        .subcommand(create())
        .subcommand(delete())
        .subcommand(edit())
        .subcommand(list())
}

fn list() -> Command {
    with_context_flags(Command::new("list").short("List container domains"))
        .flag(container_flag().required())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn create() -> Command {
    with_context_flags(Command::new("create").short("Create container custom domain"))
        .flag(container_flag().required())
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

fn edit() -> Command {
    with_context_flags(Command::new("edit").short("Edit container custom domain"))
        .flag(container_flag().required())
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

fn delete() -> Command {
    with_context_flags(Command::new("delete").short("Delete container custom domain"))
        .flag(container_flag().required())
        .flag(Flag::string("domain", DOMAIN_USAGE).required())
        .run(run_delete)
}

/// Common prologue: client, container (by `--container`) and its custom domains.
fn container_and_domains(ctx: &Ctx) -> (Client, Container, Vec<CustomDomain>) {
    telemetry::capture(ctx);
    let client = client_or_exit();
    let (_, _, env_id) = ids_or_exit(&client, ctx);
    let container = find_container_or_exit(&client, &env_id, &ctx.string("container"));
    let domains: Vec<CustomDomain> =
        check_error(client.list(&format!("/container/{}/customDomain", container.id)));
    (client, container, domains)
}

/// Go `*customDomain.ValidationDomain` (panics on nil like Go).
fn validation_domain(d: &CustomDomain) -> String {
    match &d.validation_domain {
        Some(v) => v.clone(),
        None => qovery_envvars::go_nil_pointer_panic(),
    }
}

fn run_list(ctx: &Ctx) {
    let (client, container, domains) = container_and_domains(ctx);

    let mut custom_domains_set = HashSet::new();
    let mut data: Vec<Vec<String>> = Vec::new();
    for d in &domains {
        custom_domains_set.insert(d.domain.clone());
        data.push(vec![
            d.id.clone(),
            "CUSTOM_DOMAIN".to_string(),
            d.domain.clone(),
            validation_domain(d),
            d.generate_certificate.unwrap_or(false).to_string(),
        ]);
    }

    let links: Vec<Value> = check_error(client.list(&format!("/container/{}/link", container.id)));
    let link_domain = |l: &Value| {
        l.get("url")
            .and_then(|u| u.as_str())
            .unwrap_or("")
            .replace("https://", "")
    };

    if ctx.bool("json") {
        let mut results: Vec<Value> = links
            .iter()
            .map(|l| json!({"id": null, "type": "BUILT_IN_DOMAIN", "domain": link_domain(l), "validation_domain": null}))
            .collect();
        for d in &domains {
            results.push(json!({
                "id": d.id,
                "type": "CUSTOM_DOMAIN",
                "domain": d.domain,
                "validation_domain": validation_domain(d),
            }));
        }
        let out = if results.is_empty() {
            Value::Null
        } else {
            Value::Array(results)
        };
        qovery_log::println(go_json(&out));
        return;
    }

    for l in &links {
        let domain = link_domain(l);
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

/// `qovery.CustomDomainRequest` of `create` / `edit`.
fn domain_request(ctx: &Ctx) -> Value {
    json!({
        "domain": ctx.string("domain"),
        "generate_certificate": !ctx.bool("do-not-generate-certificate"),
        "use_cdn": ctx.bool("is-behind-a-cdn"),
    })
}

fn run_create(ctx: &Ctx) {
    let (client, container, domains) = container_and_domains(ctx);
    let domain = ctx.string("domain");
    if find_by_custom_domain_name(&domains, &domain).is_some() {
        fail(format!("custom domain {domain} already exists"));
    }
    let created: CustomDomain = check_error(client.post(
        &format!("/container/{}/customDomain", container.id),
        &domain_request(ctx),
    ));
    qovery_log::println(format!(
        "Custom domain {} has been created (generate certificate: {})",
        FG_BLUE.sprint(&created.domain),
        FG_BLUE.sprint(created.generate_certificate.unwrap_or(false).to_string())
    ));
}

fn run_edit(ctx: &Ctx) {
    let (client, container, domains) = container_and_domains(ctx);
    let domain = ctx.string("domain");
    let Some(custom_domain) = find_by_custom_domain_name(&domains, &domain) else {
        fail(format!("custom domain {domain} does not exist"));
    };
    let edited: CustomDomain = check_error(client.put(
        &format!(
            "/container/{}/customDomain/{}",
            container.id, custom_domain.id
        ),
        &domain_request(ctx),
    ));
    qovery_log::println(format!(
        "Custom domain {} has been edited (generate certificate: {})",
        FG_BLUE.sprint(&edited.domain),
        FG_BLUE.sprint(edited.generate_certificate.unwrap_or(false).to_string())
    ));
}

fn run_delete(ctx: &Ctx) {
    let (client, container, domains) = container_and_domains(ctx);
    let domain = ctx.string("domain");
    let Some(custom_domain) = find_by_custom_domain_name(&domains, &domain) else {
        fail(format!("custom domain {domain} does not exist"));
    };
    check_error(client.execute(
        Method::DELETE,
        &format!(
            "/container/{}/customDomain/{}",
            container.id, custom_domain.id
        ),
        &[],
        None,
    ));
    qovery_log::println(format!(
        "Custom domain {} has been deleted",
        FG_BLUE.sprint(&domain)
    ));
}
