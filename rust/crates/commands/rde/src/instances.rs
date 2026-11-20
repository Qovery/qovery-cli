//! `qovery rde info|list|start|start-all|status|stop|stop-all|urls`. Port of the matching `cmd/rde_*.go`.

use crate::helpers::*;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit_with_error, status, telemetry};
use qovery_log::gofmt::go_json;
use serde_json::{Value, json};
use std::time::Duration;

fn org_flag() -> Flag {
    Flag::string("organization", "Organization Name").short('o')
}

fn blueprint_filter_flag() -> Flag {
    Flag::string("blueprint", "Filter by Blueprint Project Name").short('b')
}

pub fn info() -> Command {
    Command::new("info")
        .short("Show RDE platform overview")
        .flag(org_flag())
        .run(run_info)
}

pub fn list() -> Command {
    Command::new("list")
        .short("List all RDE instances")
        .long(
            "List all Remote Development Environments, optionally filtered by blueprint.

Shows name, blueprint, status, uptime, and workspace URL for each RDE.",
        )
        .flag(blueprint_filter_flag())
        .flag(org_flag())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

pub fn start() -> Command {
    Command::new("start")
        .short("Start (deploy) an RDE")
        .flag(Flag::string("name", "RDE Name").short('n').required())
        .flag(org_flag())
        .flag(
            Flag::bool(
                "watch",
                "Watch deployment status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_start)
}

pub fn stop() -> Command {
    Command::new("stop")
        .short("Stop an RDE")
        .flag(Flag::string("name", "RDE Name").short('n').required())
        .flag(org_flag())
        .flag(
            Flag::bool(
                "watch",
                "Watch stop status until it completes or an error occurs",
            )
            .short('w'),
        )
        .run(run_stop)
}

pub fn start_all() -> Command {
    Command::new("start-all")
        .short("Start (deploy) all RDE environments")
        .flag(blueprint_filter_flag())
        .flag(org_flag())
        .run(run_start_all)
}

pub fn stop_all() -> Command {
    Command::new("stop-all")
        .short("Stop all RDE environments")
        .flag(blueprint_filter_flag())
        .flag(org_flag())
        .run(run_stop_all)
}

pub fn status() -> Command {
    Command::new("status")
        .short("Show detailed status of an RDE")
        .flag(Flag::string("name", "RDE Name").short('n').required())
        .flag(org_flag())
        .run(run_status)
}

pub fn urls() -> Command {
    Command::new("urls")
        .short("List workspace URLs for running RDEs")
        .flag(blueprint_filter_flag())
        .flag(org_flag())
        .run(run_urls)
}

fn run_info(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));

    let mut org_name = org.clone();
    if let Ok(orgs) = client.list::<Value>("/organization") {
        for o in &orgs {
            if o["id"].as_str() == Some(org.as_str()) {
                org_name = o["name"].as_str().unwrap_or_default().to_string();
                break;
            }
        }
    }

    let blueprints = list_blueprint_projects(&client, &org).unwrap_or_default();
    let all_children = list_all_children(&client, &org).unwrap_or_default();
    let (mut running, mut stopped, mut errors) = (0, 0, 0);
    for child in &all_children {
        if child.env_id.is_empty() {
            continue;
        }
        match get_env_status(&client, &child.env_id) {
            Err(_) => errors += 1,
            Ok(s) => match s.as_str() {
                "DEPLOYED" | "RESTARTED" => running += 1,
                "STOPPED" => stopped += 1,
                _ => errors += 1,
            },
        }
    }

    print_key_value_table(vec![
        vec!["Organization".to_string(), format!("{org_name} ({org})")],
        vec!["Blueprints".to_string(), blueprints.len().to_string()],
        vec![
            "RDEs".to_string(),
            format!(
                "{} total ({running} running, {stopped} stopped, {errors} error/other)",
                all_children.len()
            ),
        ],
    ]);

    if !blueprints.is_empty() {
        qovery_log::println("");
        let mut data = Vec::new();
        for bp in &blueprints {
            let mut st = "NO_ENV".to_string();
            if !bp.env_id.is_empty()
                && let Ok(s) = get_env_status(&client, &bp.env_id)
            {
                st = status::status_text_with_color(&s);
            }
            data.push(vec![
                bp.project_name.clone(),
                bp.env_name.clone(),
                st,
                bp.project_id.clone(),
            ]);
        }
        qovery_log::print_table(&["Blueprint", "Environment", "Status", "Project ID"], &data);
    }
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let children = children_from_flag(&client, ctx, &org);

    if children.is_empty() {
        qovery_log::println("No RDE instances found.");
        return;
    }

    if ctx.bool("json") {
        let mut results = Vec::new();
        for child in &children {
            let mut st = String::new();
            let mut url = String::new();
            if !child.env_id.is_empty() {
                let s = get_env_status(&client, &child.env_id);
                if let Ok(s) = &s {
                    st = s.clone();
                }
                if s.as_deref().unwrap_or_default() == "DEPLOYED" {
                    url = get_workspace_url(&client, &child.env_id);
                }
            }
            let bp_name = blueprint_name_for_project_id(&client, &child.blueprint_project_id);
            results.push(json!({
                "project_id": child.project_id,
                "project_name": child.project_name,
                "env_id": child.env_id,
                "env_name": child.env_name,
                "blueprint_id": child.blueprint_project_id,
                "blueprint_name": bp_name,
                "status": st,
                "owner": child.owner_email,
                "workspace_url": url,
            }));
        }
        qovery_log::println(go_json(&Value::Array(results)));
        return;
    }

    let (mut running, mut stopped, mut errors) = (0, 0, 0);
    let mut data = Vec::new();
    for child in &children {
        let mut st = "UNKNOWN".to_string();
        let mut uptime = "-".to_string();
        let mut url = "-".to_string();
        if !child.env_id.is_empty() {
            match get_env_status(&client, &child.env_id) {
                Ok(s) => {
                    st = status::status_text_with_color(&s);
                    match s.as_str() {
                        "DEPLOYED" | "RESTARTED" => {
                            running += 1;
                            url = get_workspace_url(&client, &child.env_id);
                            if url.is_empty() {
                                url = "-".to_string();
                            }
                            uptime = format_uptime(get_last_deploy_time(&client, &child.env_id));
                        }
                        "STOPPED" => stopped += 1,
                        _ => errors += 1,
                    }
                }
                Err(_) => errors += 1,
            }
        }
        let bp_name = blueprint_name_for_project_id(&client, &child.blueprint_project_id);
        let owner = if child.owner_email.is_empty() {
            "-".to_string()
        } else {
            child.owner_email.clone()
        };
        data.push(vec![
            child.project_name.clone(),
            bp_name,
            st,
            uptime,
            owner,
            url,
        ]);
    }
    qovery_log::print_table(
        &[
            "Name",
            "Blueprint",
            "Status",
            "Uptime",
            "Owner",
            "Workspace URL",
        ],
        &data,
    );
    qovery_log::println(format!(
        "\nTotal: {} RDE(s) ({running} running, {stopped} stopped, {errors} error/other)",
        children.len()
    ));
}

/// Common start of start / stop / status: resolve the `rde-<name>` child.
fn resolve_child(ctx: &Ctx) -> (qovery_api::Client, String, ChildInfo) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let child = match find_child_by_name(&client, &org, &format!("rde-{}", ctx.string("name"))) {
        Ok(c) => c,
        Err(e) => exit_with_error(e),
    };
    (client, org, child)
}

fn run_start(ctx: &Ctx) {
    let (client, _, child) = resolve_child(ctx);
    if let Err(e) = deploy_environment(&client, &child.env_id) {
        exit_with_error(format!("deploy failed: {e}"));
    }
    qovery_log::println(format!(
        "Request to start RDE {} has been queued..",
        blue(&ctx.string("name"))
    ));
    if ctx.bool("watch") {
        std::thread::sleep(Duration::from_secs(3));
        qovery_services::watch_environment(&child.env_id, "DEPLOYED", &client);
    }
}

fn run_stop(ctx: &Ctx) {
    let (client, _, child) = resolve_child(ctx);
    if let Err(e) = stop_environment(&client, &child.env_id) {
        exit_with_error(format!("stop failed: {e}"));
    }
    qovery_log::println(format!(
        "Request to stop RDE {} has been queued..",
        blue(&ctx.string("name"))
    ));
    if ctx.bool("watch") {
        std::thread::sleep(Duration::from_secs(3));
        qovery_services::watch_environment(&child.env_id, "STOPPED", &client);
    }
}

fn run_status(ctx: &Ctx) {
    let (client, org, child) = resolve_child(ctx);
    let bp_name = blueprint_name_for_project_id(&client, &child.blueprint_project_id);
    let mut rows = vec![
        vec!["RDE".to_string(), blue(&child.project_name)],
        vec!["Project".to_string(), child.project_id.clone()],
        vec![
            "Environment".to_string(),
            format!("{} ({})", child.env_name, child.env_id),
        ],
        vec![
            "Blueprint".to_string(),
            format!("{bp_name} ({})", child.blueprint_project_id),
        ],
    ];
    if !child.owner_email.is_empty() {
        rows.push(vec!["Owner".to_string(), child.owner_email.clone()]);
    }
    let st = get_env_status(&client, &child.env_id);
    if let Ok(s) = &st {
        rows.push(vec![
            "Status".to_string(),
            status::status_text_with_color(s),
        ]);
    }
    let s = st.unwrap_or_default();
    if s == "DEPLOYED" || s == "RESTARTED" {
        let url = get_workspace_url(&client, &child.env_id);
        if !url.is_empty() {
            rows.push(vec!["Workspace".to_string(), url]);
        }
    }
    let uptime = format_uptime(get_last_deploy_time(&client, &child.env_id));
    rows.push(vec!["Uptime".to_string(), uptime]);
    rows.push(vec![
        "Console".to_string(),
        console_url(&org, &child.project_id, &child.env_id),
    ]);
    print_key_value_table(rows);

    qovery_log::println("");
    print_env_services(&client, &child.env_id);
}

fn run_start_all(ctx: &Ctx) {
    run_all(ctx, true);
}

fn run_stop_all(ctx: &Ctx) {
    run_all(ctx, false);
}

fn run_all(ctx: &Ctx, start: bool) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let children = children_from_flag(&client, ctx, &org);

    if children.is_empty() {
        qovery_log::println("No RDE instances found.");
        return;
    }

    let (verb, verb_ing) = if start {
        ("start", "Starting")
    } else {
        ("stop", "Stopping")
    };
    qovery_log::println(format!(
        "{verb_ing} {} RDE environment(s)...",
        children.len()
    ));
    for child in &children {
        if child.env_id.is_empty() {
            continue;
        }
        let r = if start {
            deploy_environment(&client, &child.env_id)
        } else {
            stop_environment(&client, &child.env_id)
        };
        match r {
            Err(e) => qovery_log::println(format!(
                "  Failed to {verb}: {} ({e})",
                blue(&child.project_name)
            )),
            Ok(()) => qovery_log::println(format!(
                "  Request to {verb} {} has been queued..",
                blue(&child.project_name)
            )),
        }
    }
    qovery_log::println("Done.");
}

fn run_urls(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let org = check_error(org_id(&client, ctx));
    let children = children_from_flag(&client, ctx, &org);

    if children.is_empty() {
        qovery_log::println("No RDE instances found.");
        return;
    }

    let mut data = Vec::new();
    for child in &children {
        if child.env_id.is_empty() {
            continue;
        }
        let Ok(s) = get_env_status(&client, &child.env_id) else {
            continue;
        };
        if s == "DEPLOYED" || s == "RESTARTED" {
            let mut url = get_workspace_url(&client, &child.env_id);
            if url.is_empty() {
                url = "-".to_string();
            }
            data.push(vec![child.project_name.clone(), url]);
        }
    }

    if data.is_empty() {
        qovery_log::println("No running RDEs with workspace URLs found.");
        return;
    }
    qovery_log::print_table(&["Name", "Workspace URL"], &data);
    qovery_log::println(format!(
        "\n{} running RDE(s) with workspace URLs.",
        data.len()
    ));
}
