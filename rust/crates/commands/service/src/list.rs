//! `qovery service list`. Port of `cmd/service_list.go` (`serviceListCmd`, `getServiceJsonOutput`,
//! `getMarkdownOutput`, `getJiraOutput`, `getApplicationPreviewUrl`, `getContainerPreviewUrl`).

use qovery_api::Client;
use qovery_api::models::{
    Application, Container, Database, Environment, EnvironmentStatuses, Helm, Job,
};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::status::{find_status, find_status_text_with_color};
use qovery_core::{check_error, client, exit_with_error, resources, telemetry};
use qovery_log::gofmt::go_json;
use qovery_services::{get_job_id, get_job_name};
use serde_json::{Value, json};
use std::io::Write;

pub fn command() -> Command {
    Command::new("list")
        .short("List services")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
        .flag(Flag::bool("markdown", "Markdown output"))
        .flag(Flag::bool("jira", "Atlassian Jira output"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn print_raw(s: &str) {
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(s.as_bytes());
    let _ = out.flush();
}

struct Services {
    apps: Vec<Application>,
    databases: Vec<Database>,
    containers: Vec<Container>,
    jobs: Vec<Job>,
    helms: Vec<Helm>,
}

fn job_type(job: &Job, cron: &'static str, lifecycle: &'static str) -> &'static str {
    if job.is_cron() { cron } else { lifecycle }
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = check_error(client::get_client());
    let (org_id, project_id, env_id) = check_error(resources::ids_from_flags(&client, ctx));

    let apps: Vec<Application> =
        check_error(client.list(&format!("/environment/{env_id}/application")));
    let databases: Vec<Database> =
        check_error(client.list(&format!("/environment/{env_id}/database")));
    let containers: Vec<Container> =
        check_error(client.list(&format!("/environment/{env_id}/container")));
    let jobs: Vec<Job> = check_error(client.list(&format!("/environment/{env_id}/job")));
    let helms: Vec<Helm> = check_error(client.list(&format!("/environment/{env_id}/helm")));
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{env_id}/statuses")));
    let s = Services {
        apps,
        databases,
        containers,
        jobs,
        helms,
    };

    if ctx.bool("markdown") {
        print_raw(&markdown_output(&client, &org_id, &project_id, &env_id, &s));
        return;
    }

    if ctx.bool("jira") {
        print_raw(&jira_output(&client, &org_id, &project_id, &env_id, &s));
        return;
    }

    if ctx.bool("json") {
        print_raw(&json_output(&statuses, &s));
        return;
    }

    let mut data: Vec<Vec<String>> = Vec::new();
    for app in &s.apps {
        data.push(vec![
            app.name.clone(),
            "Application".into(),
            find_status_text_with_color(&statuses.applications, &app.id),
        ]);
    }
    for c in &s.containers {
        data.push(vec![
            c.name.clone(),
            "Container".into(),
            find_status_text_with_color(&statuses.containers, &c.id),
        ]);
    }
    for job in &s.jobs {
        data.push(vec![
            get_job_name(job),
            job_type(job, "Cronjob", "Lifecycle").into(),
            find_status_text_with_color(&statuses.jobs, &get_job_id(job)),
        ]);
    }
    for db in &s.databases {
        data.push(vec![
            db.name.clone(),
            "Database".into(),
            find_status_text_with_color(&statuses.databases, &db.id),
        ]);
    }
    for helm in &s.helms {
        data.push(vec![
            helm.name.clone(),
            "Helm".into(),
            find_status_text_with_color(&statuses.helms, &helm.id),
        ]);
    }
    qovery_log::print_table(&["Name", "Type", "Status"], &data);
}

/// `getServiceJsonOutput`.
fn json_output(statuses: &EnvironmentStatuses, s: &Services) -> String {
    let mut results: Vec<Value> = Vec::new();
    for app in &s.apps {
        results.push(json!({"id": app.id, "name": app.name, "type": "application", "status": find_status(&statuses.applications, &app.id)}));
    }
    for c in &s.containers {
        results.push(json!({"id": c.id, "name": c.name, "type": "container", "status": find_status(&statuses.containers, &c.id)}));
    }
    for job in &s.jobs {
        let id = get_job_id(job);
        results.push(json!({
            "id": id,
            "name": get_job_name(job),
            "type": job_type(job, "cronjob", "lifecycle"),
            "status": find_status(&statuses.jobs, &id),
        }));
    }
    for helm in &s.helms {
        results.push(json!({"id": helm.id, "name": helm.name, "type": "helm", "status": find_status(&statuses.helms, &helm.id)}));
    }
    for db in &s.databases {
        results.push(json!({"id": db.id, "name": db.name, "type": "database", "status": find_status(&statuses.databases, &db.id)}));
    }
    if results.is_empty() {
        // json.Marshal of a nil slice
        return "null".to_string();
    }
    go_json(&Value::Array(results))
}

fn environment_name(client: &Client, env_id: &str) -> String {
    let env: Environment = match client.get(&format!("/environment/{env_id}")) {
        Ok(e) => e,
        Err(e) => exit_with_error(e),
    };
    env.name
}

/// `getApplicationPreviewUrl` / `getContainerPreviewUrl`: the url of the first link.
fn preview_url(client: &Client, kind: &str, id: &str) -> Option<String> {
    let links: Vec<Value> = check_error(client.list(&format!("/{kind}/{id}/link")));
    links.first().map(|l| {
        l.get("url")
            .and_then(|u| u.as_str())
            .unwrap_or_default()
            .to_string()
    })
}

fn console(org_id: &str, project_id: &str, env_id: &str) -> String {
    format!(
        "https://console.qovery.com/organization/{org_id}/project/{project_id}/environment/{env_id}"
    )
}

/// `getMarkdownOutput`.
fn markdown_output(
    client: &Client,
    org_id: &str,
    project_id: &str,
    env_id: &str,
    s: &Services,
) -> String {
    let env_name = environment_name(client, env_id);
    let base = console(org_id, project_id, env_id);
    let header = format!(
        "[![Qovery Preview](https://www.qovery.com/images/logo-white.svg)](https://www.qovery.com)\n---\n\nHere is the [{env_name}]({base}) environment services.\n\nClick on the links below to access the different services:\n"
    );
    let mut body =
        "\n| Service | Logs | Preview URL |\n|---------|------|-------------|".to_string();
    let footer = "\n---\n\nPowered by [Qovery](https://qovery.com).";
    let na = "N/A".to_string();

    let row = |body: &mut String, name: &str, kind: &str, id: &str, logs: &str, preview: &str| {
        body.push_str(&format!(
            "\n| [{name}]({base}/{kind}/{id}) | [Show logs]({base}/logs/{id}/{logs}) | {preview} |"
        ));
    };
    for app in &s.apps {
        let p = preview_url(client, "application", &app.id)
            .map(|u| format!("[Link]({u})"))
            .unwrap_or(na.clone());
        row(
            &mut body,
            &app.name,
            "application",
            &app.id,
            "live-logs",
            &p,
        );
    }
    for c in &s.containers {
        let p = preview_url(client, "container", &c.id)
            .map(|u| format!("[Link]({u})"))
            .unwrap_or(na.clone());
        row(&mut body, &c.name, "application", &c.id, "live-logs", &p);
    }
    for job in &s.jobs {
        row(
            &mut body,
            &get_job_name(job),
            "application",
            &get_job_id(job),
            "live-logs",
            &na,
        );
    }
    for db in &s.databases {
        row(
            &mut body,
            &db.name,
            "database",
            &db.id,
            "deployment-logs",
            &na,
        );
    }
    header + &body + footer
}

/// `getJiraOutput`.
fn jira_output(
    client: &Client,
    org_id: &str,
    project_id: &str,
    env_id: &str,
    s: &Services,
) -> String {
    let env_name = environment_name(client, env_id);
    let base = console(org_id, project_id, env_id);
    let header = format!(
        "[Qovery Preview|{base}]\n---\n\nHere is the [{env_name}|{base}] environment services.\n\nClick on the links below to access the different services:\n"
    );
    let mut body = "\n|| Service || Logs || Preview URL ||".to_string();
    let footer = "\n---\n\nPowered by [Qovery|https://qovery.com].";
    let na = "N/A".to_string();

    let row = |body: &mut String, name: &str, kind: &str, id: &str, logs: &str, preview: &str| {
        body.push_str(&format!(
            "\n|| [{name}|[Console|{base}/{kind}/{id}]] || [Logs|{base}/logs/{id}/{logs}] || {preview} ||"
        ));
    };
    for app in &s.apps {
        let p = preview_url(client, "application", &app.id)
            .map(|u| format!("[Link|{u}]"))
            .unwrap_or(na.clone());
        row(
            &mut body,
            &app.name,
            "application",
            &app.id,
            "live-logs",
            &p,
        );
    }
    for c in &s.containers {
        let p = preview_url(client, "container", &c.id)
            .map(|u| format!("[Link|{u}]"))
            .unwrap_or(na.clone());
        row(&mut body, &c.name, "application", &c.id, "live-logs", &p);
    }
    for job in &s.jobs {
        row(
            &mut body,
            &get_job_name(job),
            "application",
            &get_job_id(job),
            "live-logs",
            &na,
        );
    }
    for db in &s.databases {
        row(
            &mut body,
            &db.name,
            "database",
            &db.id,
            "deployment-logs",
            &na,
        );
    }
    header + &body + footer
}
