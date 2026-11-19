//! `qovery database list`. Port of `cmd/database_list.go`.

use qovery_api::Client;
use qovery_api::models::{Database, EnvironmentStatuses, Terraform};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::status::{find_status, find_status_text_with_color};
use qovery_core::{check_error, client, resources, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_services::{path_escape, to_iso8601_value};
use serde::Deserialize;
use serde_json::{Value, json};

pub(crate) fn command() -> Command {
    Command::new("list")
        .short("List databases")
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
        .flag(Flag::bool("show-credentials", "Show Credentials"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

/// `qovery.Credentials`.
#[derive(Debug, Clone, Default, Deserialize)]
struct Credentials {
    #[serde(default)]
    host: String,
    #[serde(default)]
    port: i32,
    #[serde(default)]
    login: String,
    #[serde(default)]
    password: String,
}

/// `qovery.BlueprintDatabaseResponseEndpoint`: `port` is `None` when absent, `Some(None)` when null.
#[derive(Debug, Clone, Default, Deserialize)]
struct Endpoint {
    #[serde(default)]
    host: String,
    #[serde(default, deserialize_with = "double_option")]
    port: Option<Option<i32>>,
}

fn double_option<'de, D: serde::Deserializer<'de>>(d: D) -> Result<Option<Option<i32>>, D::Error> {
    Ok(Some(Option::<i32>::deserialize(d)?))
}

/// `qovery.BlueprintDatabaseResponse`.
#[derive(Debug, Clone, Default, Deserialize)]
struct BlueprintDatabaseResponse {
    #[serde(default)]
    kind: String,
    #[serde(default)]
    endpoint: Option<Endpoint>,
}

/// `blueprintDatabase`: a terraform service created by a database blueprint.
struct BlueprintDatabase {
    terraform: Terraform,
    kind: String,
    endpoint: Option<Endpoint>,
    credentials: Option<Credentials>,
}

impl BlueprintDatabase {
    fn connection(&self) -> (String, String, String, String) {
        let na = || "N/A".to_string();
        match (&self.credentials, &self.endpoint) {
            (Some(c), _) => (
                c.host.clone(),
                c.port.to_string(),
                c.login.clone(),
                c.password.clone(),
            ),
            (None, Some(e)) => match e.port {
                Some(Some(p)) => (e.host.clone(), p.to_string(), na(), na()),
                _ => (e.host.clone(), na(), na(), na()),
            },
            (None, None) => (na(), na(), na(), na()),
        }
    }
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let env = path_escape(&env_id);
    let databases: Vec<Database> =
        check_error(client.list(&format!("/environment/{env}/database")));
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{env}/statuses")));
    let show_credentials = ctx.bool("show-credentials");
    let blueprint_databases =
        check_error(list_blueprint_databases(&client, &env_id, show_credentials));

    if ctx.bool("json") {
        qovery_log::println(get_database_json_output(
            &client,
            &statuses,
            &databases,
            &blueprint_databases,
            show_credentials,
        ));
        return;
    }

    let mut data: Vec<Vec<String>> = Vec::new();
    for database in &databases {
        let (host, port, credentials) = check_error(native_database_connection(
            &client,
            database,
            show_credentials,
        ));
        let mut login = "********".to_string();
        let mut password = "********".to_string();
        if let Some(c) = credentials {
            login = c.login;
            if login.is_empty() {
                login = "N/A".to_string();
            }
            password = c.password;
        }
        data.push(vec![
            database.id.clone(),
            database.name.clone(),
            "Database".to_string(),
            find_status_text_with_color(&statuses.databases, &database.id),
            host,
            port.to_string(),
            login,
            password,
            go_time_string(database.updated_at.as_deref()),
        ]);
    }

    for database in &blueprint_databases {
        let (host, port, mut login, mut password) = database.connection();
        if !show_credentials {
            login = "********".to_string();
            password = "********".to_string();
        }
        data.push(vec![
            database.terraform.id.clone(),
            database.terraform.name.clone(),
            "Blueprint database".to_string(),
            find_status_text_with_color(&statuses.terraforms, &database.terraform.id),
            host,
            port,
            login,
            password,
            go_time_string(database.terraform.updated_at.as_deref()),
        ]);
    }

    qovery_log::print_table(
        &[
            "Id",
            "Name",
            "Type",
            "Status",
            "Host",
            "Port",
            "Login",
            "Password",
            "Last Update",
        ],
        &data,
    );
}

/// Go `UpdatedAt.String()` on a `*time.Time`: a nil pointer panics.
fn go_time_string(t: Option<&str>) -> String {
    match t {
        Some(s) => time_string_from_rfc3339(s),
        None => qovery_envvars::go_nil_pointer_panic(),
    }
}

/// Port of `getDatabaseJsonOutput`.
fn get_database_json_output(
    client: &Client,
    statuses: &EnvironmentStatuses,
    databases: &[Database],
    blueprint_databases: &[BlueprintDatabase],
    show_credentials: bool,
) -> String {
    let mut results: Vec<Value> = Vec::new();

    for database in databases {
        let (host, port, credentials) = check_error(native_database_connection(
            client,
            database,
            show_credentials,
        ));
        let (login, password) = match credentials {
            Some(c) => (Value::String(c.login), Value::String(c.password)),
            None => (Value::Null, Value::Null),
        };
        results.push(json!({
            "id": database.id,
            "updated_at": to_iso8601_value(database.updated_at.as_ref().map(|s| Value::String(s.clone())).as_ref()),
            "name": database.name,
            "type": "Database",
            "database_type": database.db_type,
            "status": find_status(&statuses.databases, &database.id),
            "host": host,
            "port": port,
            "login": login,
            "password": password,
        }));
    }

    for database in blueprint_databases {
        let (mut host, mut port, mut login, mut password) =
            (Value::Null, Value::Null, Value::Null, Value::Null);
        if let Some(c) = &database.credentials {
            host = json!(c.host);
            port = json!(c.port);
            if show_credentials {
                login = json!(c.login);
                password = json!(c.password);
            }
        } else if let Some(e) = &database.endpoint {
            host = json!(e.host);
            if let Some(Some(p)) = e.port {
                port = json!(p);
            }
        }
        results.push(json!({
            "id": database.terraform.id,
            "updated_at": to_iso8601_value(
                database.terraform.updated_at.as_ref().map(|s| Value::String(s.clone())).as_ref()
            ),
            "name": database.terraform.name,
            "type": "Blueprint database",
            "database_type": database.kind,
            "status": find_status(&statuses.terraforms, &database.terraform.id),
            "host": host,
            "port": port,
            "login": login,
            "password": password,
        }));
    }

    if results.is_empty() {
        // json.Marshal of a nil slice
        return "null".to_string();
    }
    go_json(&Value::Array(results))
}

/// Port of `nativeDatabaseConnection`: master credentials are only fetched when they are going to be shown.
fn native_database_connection(
    client: &Client,
    database: &Database,
    show_credentials: bool,
) -> Result<(String, i64, Option<Credentials>), String> {
    if !show_credentials {
        let host = database
            .extra
            .get("host")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        let port = database
            .extra
            .get("port")
            .and_then(Value::as_i64)
            .unwrap_or_default();
        return Ok((host, port, None));
    }
    let credentials: Credentials = client
        .get(&format!(
            "/database/{}/masterCredentials",
            path_escape(&database.id)
        ))
        .map_err(|e| e.to_string())?;
    Ok((
        credentials.host.clone(),
        credentials.port as i64,
        Some(credentials),
    ))
}

/// Port of `listBlueprintDatabases`: master credentials are only fetched when they are going to be shown.
fn list_blueprint_databases(
    client: &Client,
    env_id: &str,
    with_credentials: bool,
) -> Result<Vec<BlueprintDatabase>, String> {
    let terraforms: Vec<Terraform> = client
        .list(&format!("/environment/{}/terraform", path_escape(env_id)))
        .map_err(|e| e.to_string())?;

    let mut databases = Vec::new();
    for terraform in terraforms {
        let blueprint_id = terraform
            .extra
            .get("blueprint_id")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_string();
        if blueprint_id.is_empty() {
            continue;
        }

        let database: BlueprintDatabaseResponse = match client.get(&format!(
            "/blueprint/{}/database",
            path_escape(&blueprint_id)
        )) {
            Ok(d) => d,
            // the blueprint is not a database
            Err(e) if e.status == Some(404) => continue,
            Err(e) => return Err(e.to_string()),
        };

        let mut credentials = None;
        if with_credentials {
            match client.get::<Credentials>(&format!(
                "/blueprint/{}/database/masterCredentials",
                path_escape(&blueprint_id)
            )) {
                Ok(c) => credentials = Some(c),
                // not deployed yet, or the user may see the database but not its master credentials
                Err(e) if e.status == Some(404) || e.status == Some(403) => {}
                Err(e) => return Err(e.to_string()),
            }
        }

        databases.push(BlueprintDatabase {
            terraform,
            kind: database.kind,
            endpoint: database.endpoint,
            credentials,
        });
    }
    Ok(databases)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn bp(endpoint: Option<&str>, credentials: Option<Credentials>) -> BlueprintDatabase {
        BlueprintDatabase {
            terraform: Terraform::default(),
            kind: "POSTGRESQL".to_string(),
            endpoint: endpoint.map(|e| serde_json::from_str(e).unwrap()),
            credentials,
        }
    }

    #[test]
    fn blueprint_connection() {
        let na = || "N/A".to_string();
        assert_eq!(bp(None, None).connection(), (na(), na(), na(), na()));
        assert_eq!(
            bp(Some(r#"{"host":"h"}"#), None).connection(),
            ("h".into(), na(), na(), na())
        );
        assert_eq!(
            bp(Some(r#"{"host":"h","port":null}"#), None).connection(),
            ("h".into(), na(), na(), na())
        );
        assert_eq!(
            bp(Some(r#"{"host":"h","port":5432}"#), None).connection(),
            ("h".into(), "5432".into(), na(), na())
        );
        let c = Credentials {
            host: "c".into(),
            port: 1,
            login: "l".into(),
            password: "p".into(),
        };
        assert_eq!(
            bp(Some(r#"{"host":"h","port":5432}"#), Some(c)).connection(),
            ("c".into(), "1".into(), "l".into(), "p".into())
        );
    }
}
