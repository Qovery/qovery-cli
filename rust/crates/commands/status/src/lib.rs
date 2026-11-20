//! `qovery status` command. Port of `cmd/status.go`.

use qovery_cobra::{Command, Ctx};
use qovery_core::context::ServiceType;
use qovery_core::{auth, client, context, exit, telemetry};
use qovery_log::table::Table;
use serde_json::Value;

/// The `status` command.
pub fn command() -> Command {
    Command::new("status")
        .short("Print the status of your application")
        .run(run_status)
}

/// Every error of the Go command is printed then exits with status 0.
fn or_exit0<T, E: std::fmt::Display>(r: Result<T, E>) -> T {
    match r {
        Ok(v) => v,
        Err(e) => {
            qovery_log::println_error(e);
            exit(0)
        }
    }
}

fn run_status(ctx: &Ctx) {
    telemetry::capture(ctx);

    let (token_type, token) = or_exit0(auth::get_access_token(false));
    let service = or_exit0(context::current_service(true));
    let client = client::client_for(token_type, &token);

    let (path, header) = match ServiceType::parse(&service.service_type) {
        Some(ServiceType::Application) => {
            (format!("/application/{}/status", service.id), "Application")
        }
        Some(ServiceType::Container) => (format!("/container/{}/status", service.id), "Container"),
        _ => return,
    };
    let status: Value = or_exit0(client.get(&path));
    let state = status["state"].as_str().unwrap_or_default().to_string();
    Table::new(vec![
        vec![header.to_string(), "Status".to_string()],
        vec![service.name.clone(), state],
    ])
    .render();
}
