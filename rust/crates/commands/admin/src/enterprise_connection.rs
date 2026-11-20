//! `qovery admin enterprise-connection ...`. Port of `cmd/admin_enterprise_connection*.go`.

use crate::goutil::{path_escape, query_escape};
use crate::http::{Req, authorization};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::check_error;
use qovery_core::client::admin_url;
use qovery_log::gofmt::go_json;
use reqwest::Method;
use serde_json::{Value, json};

const CONNECTION_NAME_USAGE: &str =
    "The connection name configured on Auth0 side for the target client";
const ORGANIZATION_ID_USAGE: &str = "The organization of the target client";

pub fn command() -> Command {
    Command::new("enterprise-connection")
        .short("Manage enterprise connections")
        .run(crate::run_group_help)
        .subcommand(
            Command::new("create")
                .short("Create a new enterprise connection")
                .flag(
                    Flag::string("connection-name", CONNECTION_NAME_USAGE)
                        .short('c')
                        .required(),
                )
                .flag(
                    Flag::string("organization-id", ORGANIZATION_ID_USAGE)
                        .short('o')
                        .required(),
                )
                .run(run_create),
        )
        .subcommand(
            Command::new("delete")
                .short("Delete an enterprise connection")
                .flag(
                    Flag::string("connection-name", CONNECTION_NAME_USAGE)
                        .short('c')
                        .required(),
                )
                .flag(
                    Flag::string("organization-id", ORGANIZATION_ID_USAGE)
                        .short('o')
                        .required(),
                )
                .run(run_delete),
        )
        .subcommand(
            Command::new("list")
                .short("List enterprise connections by connection name")
                .flag(
                    Flag::string("connection-name", CONNECTION_NAME_USAGE)
                        .short('c')
                        .required(),
                )
                .run(run_list),
        )
}

fn row(ec: &Value) -> Vec<String> {
    ["organization_id", "connection_name", "default_role"]
        .iter()
        .map(|k| {
            ec.get(*k)
                .and_then(|v| v.as_str())
                .unwrap_or_default()
                .to_string()
        })
        .collect()
}

const HEADERS: [&str; 3] = ["Organization ID", "Connection Name", "Default Role"];

/// `json.Unmarshal(body, &v)` error: only syntax errors are reproduced.
fn decode(body: &[u8]) -> Result<Value, String> {
    crate::gojson::check_valid(body)?;
    serde_json::from_slice(body).map_err(|e| e.to_string())
}

fn run_create(ctx: &Ctx) {
    let auth = check_error(authorization());
    let payload = go_json(&json!({
        "organization_id": ctx.string("organization-id"),
        "connection_name": ctx.string("connection-name"),
    }));
    let url = format!("{}/enterpriseconnection", admin_url());
    let res = check_error(Req::new(Method::POST, &url, &auth).body(payload).send());
    if res.status != 201 {
        qovery_log::println_error(res.text());
        return;
    }
    let created = match decode(&res.body) {
        Ok(v) => v,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };
    qovery_log::print_table(&HEADERS, &[row(&created)]);
}

fn run_delete(ctx: &Ctx) {
    let auth = check_error(authorization());
    let url = format!(
        "{}/enterpriseconnection/{}?organization_id={}",
        admin_url(),
        path_escape(&ctx.string("connection-name")),
        query_escape(&ctx.string("organization-id"))
    );
    let res = check_error(Req::new(Method::DELETE, &url, &auth).send());
    if res.status != 202 {
        qovery_log::println_error(res.text());
    }
}

fn run_list(ctx: &Ctx) {
    let auth = check_error(authorization());
    let url = format!(
        "{}/enterpriseconnection/{}",
        admin_url(),
        path_escape(&ctx.string("connection-name"))
    );
    let res = check_error(Req::new(Method::GET, &url, &auth).send());
    if res.status != 200 {
        qovery_log::println_error(res.text());
        return;
    }
    let wrapped = match decode(&res.body) {
        Ok(v) => v,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };
    let rows: Vec<Vec<String>> = wrapped
        .get("results")
        .and_then(|r| r.as_array())
        .map(|a| a.iter().map(row).collect())
        .unwrap_or_default();
    qovery_log::print_table(&HEADERS, &rows);
}
