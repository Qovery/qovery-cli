//! Compare the API helpers with the Go implementation: same requests (method, path, query, JSON body) and
//! same results / error strings. `tests/data/go_api_expected.json` was produced by calling the Go functions
//! against an `httptest` server implementing the routes of [`route`] (throw-away Go test).

use qovery_core::context::ServiceType;
use qovery_envvars::*;
use serde_json::{Value, json};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{Arc, Mutex};

const VAR_TPL: &str = r#"{"id":"ID","created_at":"2024-01-02T15:04:05Z","updated_at":"2024-01-02T15:04:05Z","key":"KEY","value":"v","scope":"APPLICATION","variable_type":"VALUE","is_secret":false}"#;

fn var(id: &str, key: &str) -> String {
    VAR_TPL
        .replace("\"ID\"", &format!("\"{id}\""))
        .replace("\"KEY\"", &format!("\"{key}\""))
}

/// Same routes as the Go `httptest` handler.
fn route(method: &str, path: &str, query: &str, body: &Value) -> (u16, String) {
    let parent = query
        .split('&')
        .find_map(|kv| kv.strip_prefix("parent_id="))
        .unwrap_or_default();
    match (method, path) {
        ("GET", "/variable") => match parent {
            "err500" => (500, r#"{"detail":"boom"}"#.into()),
            "empty" => (200, String::new()),
            "null" => (200, "null".into()),
            _ => (
                200,
                format!(
                    r#"{{"results":[{},{},{}]}}"#,
                    var("var-1", "KEY1"),
                    var("var-2", "KEY2"),
                    var("var-1b", "KEY1")
                ),
            ),
        },
        ("POST", "/variable") => match body["key"].as_str().unwrap() {
            "CONFLICT" => (
                409,
                r#"{"status":409,"detail":"Key already exists at ENVIRONMENT scope"}"#.into(),
            ),
            "RAW" => (400, "plain text error".into()),
            "NODETAIL" => (400, r#"{"message":"nope"}"#.into()),
            "EMPTYBODY" => (409, String::new()),
            "HTML" => (409, r#"{"detail":"<b>&</b> \"q\""}"#.into()),
            "NUMDETAIL" => (400, r#"{"detail":12}"#.into()),
            "CASE" => (400, r#"{"Detail":"case insensitive"}"#.into()),
            "NULLDETAIL" => (400, r#"{"detail":null}"#.into()),
            key => (201, var("new", key)),
        },
        ("PUT", p) if p.starts_with("/variable/") => {
            if p == "/variable/var-2" {
                (500, r#"{"detail":"edit failed"}"#.into())
            } else {
                (200, var("var-1", "KEY1"))
            }
        }
        ("DELETE", p) if p.starts_with("/variable/") => {
            if p == "/variable/var-2" {
                (403, r#"{"detail":"forbidden"}"#.into())
            } else {
                (204, String::new())
            }
        }
        ("POST", p) if p.ends_with("/alias") || p.ends_with("/override") => {
            if body.get("key").and_then(|k| k.as_str()) == Some("BADALIAS") {
                (400, r#"{"detail":"bad alias"}"#.into())
            } else {
                (201, var("new", "X"))
            }
        }
        ("GET", p) if p.starts_with("/environment/") => {
            let id = p.trim_start_matches("/environment/");
            if id == "env-err" {
                return (404, r#"{"detail":"no env"}"#.into());
            }
            let cl = if id == "env-nocluster" {
                "cluster-x"
            } else {
                "cluster-1"
            };
            (
                200,
                json!({"id": id, "created_at": "2024-01-02T15:04:05Z", "name": "e", "organization": {"id": "org-1"},
                    "project": {"id": "proj-1"}, "cloud_provider": {"provider": "AWS", "cluster": "c"},
                    "mode": "DEVELOPMENT", "cluster_id": cl})
                .to_string(),
            )
        }
        ("GET", p) if p.starts_with("/organization/") => {
            if p == "/organization/org-err/cluster" {
                return (403, r#"{"detail":"no"}"#.into());
            }
            let sma = |id: &str, name: &str| {
                json!({"id": id, "name": name, "created_at": "2024-01-02T15:04:05Z", "updated_at": "2024-01-02T15:04:05Z",
                    "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-1"},
                    "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}})
            };
            let cluster = |id: &str, name: &str| {
                json!({"id": id, "created_at": "2024-01-02T15:04:05Z", "organization": {"id": "org-1"}, "name": name,
                    "region": "us-east-1", "cloud_provider": "AWS"})
            };
            let mut c1 = cluster("cluster-1", "my-cluster");
            c1["secret_manager_accesses"] = json!([sma("sma-1", "aws-sm"), sma("sma-2", "gcp-sm")]);
            (
                200,
                json!({"results": [cluster("cluster-0", "other"), c1]}).to_string(),
            )
        }
        _ => (404, String::new()),
    }
}

type Records = Arc<Mutex<Vec<Value>>>;

fn start_server(records: Records) -> u16 {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let port = listener.local_addr().unwrap().port();
    std::thread::spawn(move || {
        for stream in listener.incoming() {
            let mut stream = stream.unwrap();
            let mut reader = BufReader::new(stream.try_clone().unwrap());
            let mut request_line = String::new();
            reader.read_line(&mut request_line).unwrap();
            let mut content_length = 0;
            loop {
                let mut line = String::new();
                reader.read_line(&mut line).unwrap();
                if line == "\r\n" || line.is_empty() {
                    break;
                }
                if let Some((k, v)) = line.split_once(':')
                    && k.eq_ignore_ascii_case("content-length")
                {
                    content_length = v.trim().parse().unwrap();
                }
            }
            let mut body = vec![0u8; content_length];
            reader.read_exact(&mut body).unwrap();
            let mut parts = request_line.split_whitespace();
            let method = parts.next().unwrap().to_string();
            let target = parts.next().unwrap().to_string();
            let (path, query) = target.split_once('?').unwrap_or((&target, ""));
            let body: Value = if body.is_empty() {
                Value::Null
            } else {
                serde_json::from_slice(&body).unwrap()
            };
            records
                .lock()
                .unwrap()
                .push(json!({"method": method, "path": path, "query": query, "body": body}));
            let (code, resp) = route(&method, path, query, &body);
            let reason = match code {
                200 => "OK",
                201 => "Created",
                204 => "No Content",
                400 => "Bad Request",
                403 => "Forbidden",
                404 => "Not Found",
                409 => "Conflict",
                _ => "Internal Server Error",
            };
            let head = format!(
                "HTTP/1.1 {code} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                resp.len()
            );
            stream.write_all(head.as_bytes()).unwrap();
            stream.write_all(resp.as_bytes()).unwrap();
        }
    });
    port
}

type Scenario = Box<dyn Fn(&qovery_api::Client) -> Result<String, String>>;

fn unit(r: Result<(), String>) -> Result<String, String> {
    r.map(|_| String::new())
}

fn scenarios() -> Vec<(&'static str, Scenario)> {
    use ServiceType::*;
    vec![
        (
            "create_service_variable",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "APPLICATION",
                    "K1",
                    "v1",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_project_secret_override",
            Box::new(|c| {
                unit(create_service_variable(
                    c, "proj-1", "env-1", "svc-1", "PROJECT", "K1", "v1", true, true,
                ))
            }),
        ),
        (
            "create_service_variable_bad_scope",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "application",
                    "K1",
                    "v1",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_conflict",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "ENVIRONMENT",
                    "CONFLICT",
                    "v",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_raw",
            Box::new(|c| {
                unit(create_service_variable(
                    c, "proj-1", "env-1", "svc-1", "HELM", "RAW", "v", false, false,
                ))
            }),
        ),
        (
            "create_service_variable_nodetail",
            Box::new(|c| {
                unit(create_service_variable(
                    c, "proj-1", "env-1", "svc-1", "JOB", "NODETAIL", "v", false, false,
                ))
            }),
        ),
        (
            "create_service_variable_emptybody",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "CONTAINER",
                    "EMPTYBODY",
                    "v",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_html",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "TERRAFORM",
                    "HTML",
                    "v",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_numdetail",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "APPLICATION",
                    "NUMDETAIL",
                    "v",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_case",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "APPLICATION",
                    "CASE",
                    "v",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_variable_nulldetail",
            Box::new(|c| {
                unit(create_service_variable(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "APPLICATION",
                    "NULLDETAIL",
                    "v",
                    false,
                    false,
                ))
            }),
        ),
        (
            "create_service_external_secret",
            Box::new(|c| {
                unit(create_service_external_secret(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "CONTAINER",
                    "ES",
                    "ref/x",
                    "sma-1",
                    "",
                ))
            }),
        ),
        (
            "create_service_external_secret_mount",
            Box::new(|c| {
                unit(create_service_external_secret(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    "ENVIRONMENT",
                    "ES",
                    "ref/x",
                    "sma-1",
                    "/etc/x",
                ))
            }),
        ),
        (
            "create_service_external_secret_conflict",
            Box::new(|c| {
                unit(create_service_external_secret(
                    c, "proj-1", "env-1", "svc-1", "PROJECT", "CONFLICT", "ref/x", "sma-1", "",
                ))
            }),
        ),
        (
            "update_service_external_secret",
            Box::new(|c| {
                unit(update_service_external_secret(
                    c, "KEY1", "ref/y", "sma-2", "svc-1", Container,
                ))
            }),
        ),
        (
            "update_service_external_secret_empty",
            Box::new(|c| {
                unit(update_service_external_secret(
                    c, "KEY1", "", "", "svc-1", Job,
                ))
            }),
        ),
        (
            "update_service_external_secret_missing",
            Box::new(|c| {
                unit(update_service_external_secret(
                    c,
                    "NOPE",
                    "r",
                    "s",
                    "svc-1",
                    Application,
                ))
            }),
        ),
        (
            "update_service_external_secret_database",
            Box::new(|c| {
                unit(update_service_external_secret(
                    c, "KEY1", "r", "s", "svc-1", Database,
                ))
            }),
        ),
        (
            "update_environment_external_secret",
            Box::new(|c| {
                unit(update_environment_external_secret(
                    c, "env-1", "KEY1", "ref/z", "",
                ))
            }),
        ),
        (
            "update_environment_external_secret_missing",
            Box::new(|c| {
                unit(update_environment_external_secret(
                    c, "env-1", "NOPE", "ref/z", "",
                ))
            }),
        ),
        (
            "update_environment_external_secret_put_error",
            Box::new(|c| {
                unit(update_environment_external_secret(
                    c, "env-1", "KEY2", "ref/z", "sma",
                ))
            }),
        ),
        (
            "create_environment_variable",
            Box::new(|c| {
                unit(create_environment_variable(
                    c, "proj-1", "env-1", "EK", "ev", false, false,
                ))
            }),
        ),
        (
            "create_environment_variable_secret_override",
            Box::new(|c| {
                unit(create_environment_variable(
                    c, "proj-1", "env-1", "EK", "ev", true, true,
                ))
            }),
        ),
        (
            "create_environment_variable_conflict",
            Box::new(|c| {
                unit(create_environment_variable(
                    c, "proj-1", "env-1", "CONFLICT", "ev", false, false,
                ))
            }),
        ),
        (
            "create_project_variable",
            Box::new(|c| unit(create_project_variable(c, "proj-1", "PK", "pv", true))),
        ),
        (
            "create_project_variable_raw",
            Box::new(|c| unit(create_project_variable(c, "proj-1", "RAW", "pv", false))),
        ),
        (
            "update_service_variable",
            Box::new(|c| unit(update_service_variable(c, "KEY1", "new", "svc-1", Helm))),
        ),
        (
            "update_service_variable_missing",
            Box::new(|c| unit(update_service_variable(c, "NOPE", "new", "svc-1", Helm))),
        ),
        (
            "update_service_variable_list_error",
            Box::new(|c| {
                unit(update_service_variable(
                    c, "KEY1", "new", "err500", Terraform,
                ))
            }),
        ),
        (
            "update_service_variable_list_empty",
            Box::new(|c| {
                unit(update_service_variable(
                    c,
                    "KEY1",
                    "new",
                    "empty",
                    Application,
                ))
            }),
        ),
        (
            "update_service_variable_list_null",
            Box::new(|c| {
                unit(update_service_variable(
                    c,
                    "KEY1",
                    "new",
                    "null",
                    Application,
                ))
            }),
        ),
        (
            "update_service_variable_put_error",
            Box::new(|c| unit(update_service_variable(c, "KEY2", "", "svc-1", Application))),
        ),
        (
            "update_environment_variable",
            Box::new(|c| unit(update_environment_variable(c, "env-1", "KEY1", "x"))),
        ),
        (
            "update_environment_variable_list_empty",
            Box::new(|c| unit(update_environment_variable(c, "empty", "KEY1", "x"))),
        ),
        (
            "update_project_variable",
            Box::new(|c| unit(update_project_variable(c, "proj-1", "KEY2", "y"))),
        ),
        (
            "update_project_variable_missing",
            Box::new(|c| unit(update_project_variable(c, "proj-1", "NOPE", "y"))),
        ),
        (
            "update_project_variable_list_empty",
            Box::new(|c| unit(update_project_variable(c, "empty", "NOPE", "y"))),
        ),
        (
            "delete_service_variable",
            Box::new(|c| unit(delete_service_variable(c, "svc-1", Terraform, "KEY1"))),
        ),
        (
            "delete_service_variable_missing",
            Box::new(|c| unit(delete_service_variable(c, "svc-1", Terraform, "NOPE"))),
        ),
        (
            "delete_service_variable_error",
            Box::new(|c| unit(delete_service_variable(c, "svc-1", Container, "KEY2"))),
        ),
        (
            "delete_environment_var",
            Box::new(|c| unit(delete_environment_var(c, "env-1", "KEY1"))),
        ),
        (
            "delete_environment_var_missing",
            Box::new(|c| unit(delete_environment_var(c, "env-1", "NOPE"))),
        ),
        (
            "delete_project_var",
            Box::new(|c| unit(delete_project_var(c, "proj-1", "KEY2"))),
        ),
        (
            "delete_project_var_missing",
            Box::new(|c| unit(delete_project_var(c, "proj-1", "NOPE"))),
        ),
        (
            "create_service_alias",
            Box::new(|c| {
                unit(create_service_alias(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    Application,
                    "KEY1",
                    "ALIAS1",
                    "ENVIRONMENT",
                ))
            }),
        ),
        (
            "create_service_alias_missing",
            Box::new(|c| {
                unit(create_service_alias(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    Application,
                    "NOPE",
                    "ALIAS1",
                    "APPLICATION",
                ))
            }),
        ),
        (
            "create_service_alias_bad_scope",
            Box::new(|c| {
                unit(create_service_alias(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    Application,
                    "NOPE",
                    "ALIAS1",
                    "XX",
                ))
            }),
        ),
        (
            "create_service_alias_api_error",
            Box::new(|c| {
                unit(create_service_alias(
                    c, "proj-1", "env-1", "svc-1", Job, "KEY1", "BADALIAS", "JOB",
                ))
            }),
        ),
        (
            "create_service_alias_database",
            Box::new(|c| {
                unit(create_service_alias(
                    c, "proj-1", "env-1", "svc-1", Database, "KEY1", "A", "JOB",
                ))
            }),
        ),
        (
            "create_environment_alias",
            Box::new(|c| {
                unit(create_environment_alias(
                    c, "proj-1", "env-1", "KEY2", "EA", "PROJECT",
                ))
            }),
        ),
        (
            "create_environment_alias_service_scope",
            Box::new(|c| {
                unit(create_environment_alias(
                    c,
                    "proj-1",
                    "env-1",
                    "KEY2",
                    "EA",
                    "APPLICATION",
                ))
            }),
        ),
        (
            "create_environment_alias_missing",
            Box::new(|c| {
                unit(create_environment_alias(
                    c,
                    "proj-1",
                    "env-1",
                    "NOPE",
                    "EA",
                    "ENVIRONMENT",
                ))
            }),
        ),
        (
            "create_project_alias",
            Box::new(|c| unit(create_project_alias(c, "proj-1", "KEY1", "PA"))),
        ),
        (
            "create_project_alias_missing",
            Box::new(|c| unit(create_project_alias(c, "proj-1", "NOPE", "PA"))),
        ),
        (
            "create_service_override",
            Box::new(|c| {
                unit(create_service_override(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    Job,
                    "KEY2",
                    "ov",
                    "APPLICATION",
                ))
            }),
        ),
        (
            "create_service_override_missing",
            Box::new(|c| {
                unit(create_service_override(
                    c,
                    "proj-1",
                    "env-1",
                    "svc-1",
                    Job,
                    "NOPE",
                    "ov",
                    "APPLICATION",
                ))
            }),
        ),
        (
            "create_environment_override",
            Box::new(|c| {
                unit(create_environment_override(
                    c,
                    "proj-1",
                    "env-1",
                    "KEY1",
                    "ov",
                    "ENVIRONMENT",
                ))
            }),
        ),
        (
            "create_environment_override_missing",
            Box::new(|c| {
                unit(create_environment_override(
                    c,
                    "proj-1",
                    "env-1",
                    "NOPE",
                    "ov",
                    "ENVIRONMENT",
                ))
            }),
        ),
        (
            "create_environment_override_list_error",
            Box::new(|c| {
                unit(create_environment_override(
                    c,
                    "proj-1",
                    "err500",
                    "NOPE",
                    "ov",
                    "ENVIRONMENT",
                ))
            }),
        ),
        (
            "sma_ok",
            Box::new(|c| get_secret_manager_access_id_by_name(c, "org-1", "env-1", "gcp-sm")),
        ),
        (
            "sma_missing",
            Box::new(|c| get_secret_manager_access_id_by_name(c, "org-1", "env-1", "no\"pe")),
        ),
        (
            "sma_no_cluster",
            Box::new(|c| {
                get_secret_manager_access_id_by_name(c, "org-1", "env-nocluster", "aws-sm")
            }),
        ),
        (
            "sma_env_error",
            Box::new(|c| get_secret_manager_access_id_by_name(c, "org-1", "env-err", "aws-sm")),
        ),
        (
            "sma_clusters_error",
            Box::new(|c| get_secret_manager_access_id_by_name(c, "org-err", "env-1", "aws-sm")),
        ),
    ]
}

#[test]
fn api_calls_match_go() {
    let expected: Value = serde_json::from_str(
        &std::fs::read_to_string(format!(
            "{}/tests/data/go_api_expected.json",
            env!("CARGO_MANIFEST_DIR")
        ))
        .unwrap(),
    )
    .unwrap();
    let records: Records = Arc::new(Mutex::new(Vec::new()));
    let port = start_server(records.clone());
    // SAFETY: this is the only test of this binary reading the environment.
    unsafe { std::env::set_var("QOVERY_API_URL", format!("http://127.0.0.1:{port}")) };
    let client = qovery_api::Client::new("Token test", "test", false);

    let scenarios = scenarios();
    assert_eq!(
        scenarios.len(),
        expected.as_object().unwrap().len() - 1,
        "every Go scenario is ported"
    );
    for (name, f) in scenarios {
        records.lock().unwrap().clear();
        let res = f(&client);
        let exp = &expected[name];
        assert!(exp.is_object(), "missing Go scenario {name}");
        let (value, error) = match res {
            Ok(v) => (v, String::new()),
            Err(e) => (String::new(), e),
        };
        assert_eq!(error, exp["error"].as_str().unwrap(), "{name}: error");
        assert_eq!(value, exp["value"].as_str().unwrap(), "{name}: value");
        let got = Value::Array(records.lock().unwrap().clone());
        let want = if exp["requests"].is_null() {
            json!([])
        } else {
            exp["requests"].clone()
        };
        assert_eq!(got, want, "{name}: requests");
    }
}

#[test]
fn go_panic_messages() {
    // Child mode: trigger the panic.
    match std::env::var("ENVVARS_TRIGGER_PANIC").as_deref() {
        Ok("data") => {
            let _ = EnvVarLineOutput {
                key: "X".into(),
                ..Default::default()
            }
            .data(true);
            unreachable!();
        }
        Ok("interpolation") => {
            let v = EnvVarLineOutput {
                key: "X".into(),
                value: Some("{{}x{}".into()),
                ..Default::default()
            };
            let _ = get_env_var_json_output(&[v], false);
            unreachable!();
        }
        _ => {}
    }
    let expected: Value = serde_json::from_str(
        &std::fs::read_to_string(format!(
            "{}/tests/data/go_api_expected.json",
            env!("CARGO_MANIFEST_DIR")
        ))
        .unwrap(),
    )
    .unwrap();
    for case in ["data", "interpolation"] {
        let out = std::process::Command::new(std::env::current_exe().unwrap())
            .args([
                "--exact",
                "go_panic_messages",
                "--nocapture",
                "--test-threads=1",
            ])
            .env("ENVVARS_TRIGGER_PANIC", case)
            .output()
            .unwrap();
        assert_eq!(
            out.status.code(),
            Some(2),
            "{case}: Go exits with 2 on panic"
        );
        let stderr = String::from_utf8_lossy(&out.stderr);
        let first = format!("panic: {}\n", expected["_panics"][case].as_str().unwrap());
        assert!(stderr.contains(&first), "{case}: {stderr}");
    }
}
