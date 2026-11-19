//! `qovery webhook` command. Port of `cmd/webhook*.go`.

use qovery_api::path_escape;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit, resources, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use serde_json::{Map, Value};

/// The `webhook` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("webhook")
        .short("Manage webhooks")
        .run(run_webhook)
        .subcommand(list())
        .subcommand(list_event())
}

fn run_webhook(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

fn list() -> Command {
    Command::new("list")
        .short("List webhooks")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn list_event() -> Command {
    Command::new("list-event")
        .short("List webhook events")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("webhook-id", "Webhook ID (UUID)").required())
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list_event)
}

fn results(v: &Value) -> Vec<Value> {
    v["results"].as_array().cloned().unwrap_or_default()
}

fn str_of(v: &Value) -> String {
    v.as_str().unwrap_or_default().to_string()
}

/// A non-null optional property (Go pointer field set).
fn opt<'a>(v: &'a Value, key: &str) -> Option<&'a Value> {
    v.get(key).filter(|x| !x.is_null())
}

/// A non-empty array property (Go `len(slice) > 0`).
fn non_empty_array<'a>(v: &'a Value, key: &str) -> Option<&'a Vec<Value>> {
    v.get(key)
        .and_then(|x| x.as_array())
        .filter(|a| !a.is_empty())
}

/// Go `json.Marshal` of a nil `[]interface{}` is `null`.
fn json_array(items: Vec<Value>) -> String {
    if items.is_empty() {
        "null".to_string()
    } else {
        go_json(&Value::Array(items))
    }
}

/// Go `time.Time.String()` of a decoded RFC 3339 field (zero time when missing).
fn time_string(v: &Value) -> String {
    match v.as_str() {
        Some(s) => time_string_from_rfc3339(s),
        None => qovery_log::gofmt::ZERO_TIME_STRING.to_string(),
    }
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = client::get_client_or_exit();
    let organization_id = check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let webhooks: Value = check_error(client.get(&format!(
        "/organization/{}/webhook",
        path_escape(&organization_id)
    )));

    if ctx.bool("json") {
        qovery_log::println(webhook_list_json(&webhooks));
        return;
    }

    let data: Vec<Vec<String>> = results(&webhooks)
        .iter()
        .map(|w| {
            let enabled = if opt(w, "enabled").and_then(|e| e.as_bool()).unwrap_or(false) {
                "true"
            } else {
                "false"
            };
            let events = non_empty_array(w, "events")
                .map(|a| a.iter().map(str_of).collect::<Vec<_>>().join(", "))
                .unwrap_or_default();
            vec![
                str_of(&w["id"]),
                opt(w, "description").map(str_of).unwrap_or_default(),
                opt(w, "kind").map(str_of).unwrap_or_default(),
                opt(w, "target_url").map(str_of).unwrap_or_default(),
                enabled.to_string(),
                events,
            ]
        })
        .collect();
    qovery_log::print_table(
        &[
            "ID",
            "Description",
            "Kind",
            "Target URL",
            "Enabled",
            "Events",
        ],
        &data,
    );
}

fn webhook_list_json(webhooks: &Value) -> String {
    let items = results(webhooks)
        .iter()
        .map(|w| {
            let mut m = Map::new();
            m.insert("id".into(), Value::String(str_of(&w["id"])));
            m.insert(
                "created_at".into(),
                Value::String(time_string(&w["created_at"])),
            );
            if let Some(u) = opt(w, "updated_at") {
                m.insert("updated_at".into(), Value::String(time_string(u)));
            }
            for key in ["description", "kind", "target_url"] {
                if let Some(x) = opt(w, key) {
                    m.insert(key.into(), Value::String(str_of(x)));
                }
            }
            if let Some(e) = opt(w, "enabled") {
                m.insert("enabled".into(), Value::Bool(e.as_bool().unwrap_or(false)));
            }
            for key in ["events", "project_names_filter", "environment_types_filter"] {
                if let Some(a) = non_empty_array(w, key) {
                    m.insert(
                        key.into(),
                        Value::Array(a.iter().map(|x| Value::String(str_of(x))).collect()),
                    );
                }
            }
            Value::Object(m)
        })
        .collect();
    json_array(items)
}

fn run_list_event(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = client::get_client_or_exit();
    let organization_id = check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let webhook_id = ctx.string("webhook-id");
    let events: Value = check_error(client.get(&format!(
        "/organization/{}/webhook/{}/event",
        path_escape(&organization_id),
        path_escape(&webhook_id)
    )));

    if ctx.bool("json") {
        qovery_log::println(webhook_event_json(&events));
        return;
    }

    let mut data: Vec<Vec<String>> = Vec::new();
    for e in results(&events) {
        let body = match opt(&e, "target_response_body") {
            Some(b) => str_of(b),
            // Go dereferences a nil *string here
            None => nil_pointer_panic(),
        };
        data.push(vec![
            str_of(&e["id"]),
            time_string(&e["created_at"]),
            str_of(&e["matched_event"]),
            str_of(&e["kind"]),
            str_of(&e["target_url_used"]),
            e["target_response_status_code"]
                .as_i64()
                .unwrap_or(0)
                .to_string(),
            body,
        ]);
    }
    qovery_log::print_table(
        &[
            "ID",
            "Created At",
            "Event",
            "Kind",
            "Target URL",
            "Response Status Code",
            "Response Body",
        ],
        &data,
    );
}

fn webhook_event_json(events: &Value) -> String {
    let items = results(events)
        .iter()
        .map(|e| {
            let mut m = Map::new();
            m.insert("id".into(), Value::String(str_of(&e["id"])));
            m.insert(
                "matched_event".into(),
                Value::String(str_of(&e["matched_event"])),
            );
            m.insert("kind".into(), Value::String(str_of(&e["kind"])));
            m.insert(
                "target_url_used".into(),
                Value::String(str_of(&e["target_url_used"])),
            );
            m.insert(
                "target_response_status_code".into(),
                Value::from(e["target_response_status_code"].as_i64().unwrap_or(0)),
            );
            m.insert(
                "target_response_body".into(),
                opt(e, "target_response_body")
                    .map(|b| Value::String(str_of(b)))
                    .unwrap_or(Value::Null),
            );
            m.insert(
                "created_at".into(),
                Value::String(time_string(&e["created_at"])),
            );
            m.insert(
                "payload".into(),
                e.get("request")
                    .filter(|r| r.is_object())
                    .cloned()
                    .unwrap_or(Value::Null),
            );
            Value::Object(m)
        })
        .collect();
    json_array(items)
}

use qovery_core::gosys::go_nil_pointer_panic as nil_pointer_panic;

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn list_json_keeps_only_set_fields() {
        let v = json!({"results": [{"id": "w1", "created_at": "2024-05-01T10:00:00Z", "description": null,
            "kind": "STANDARD", "enabled": false, "events": [], "project_names_filter": ["a*"]}]});
        assert_eq!(
            webhook_list_json(&v),
            r#"[{"created_at":"2024-05-01 10:00:00 +0000 UTC","enabled":false,"id":"w1","kind":"STANDARD","project_names_filter":["a*"]}]"#
        );
        assert_eq!(webhook_list_json(&json!({"results": []})), "null");
    }
}
