//! `qovery token` command. Port of `cmd/token.go`.
//!
//! `utils.SelectTokenInformation` / `utils.SelectRole` (utils/qovery.go) are not in `qovery-services` yet: a
//! private copy lives in [`select`].

use qovery_cobra::{Command, Ctx};
use qovery_core::{auth, client, telemetry};
use serde_json::{Value, json};

/// The `token` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("token")
        .short("Generate an API token")
        .run(run_token)
}

fn run_token(ctx: &Ctx) {
    telemetry::capture(ctx);

    qovery_log::println_info("Select organization");
    let token_information = match select::select_token_information() {
        Ok(t) => t,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };

    let token = match generate_machine_to_machine_api_token(&token_information) {
        Ok(t) => t,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };

    qovery_log::println_info("---- Never share this authentication token and keep it secure ----");
    qovery_log::println_info(token);
    qovery_log::println_info("---- Never share this authentication token and keep it secure ----");
}

/// `generateMachineToMachineAPIToken`.
fn generate_machine_to_machine_api_token(
    info: &select::TokenInformation,
) -> Result<String, String> {
    let (token_type, token) = auth::get_access_token(false)?;
    let client = client::client_for(token_type, &token);
    let body = json!({
        "name": info.name,
        "description": info.description,
        "role_id": info.role.id,
    });
    let created: Value = client
        .post(
            &format!("/organization/{}/apiToken", info.organization.id),
            &body,
        )
        .map_err(|e| e.to_string())?;
    Ok(created
        .get("token")
        .and_then(|t| t.as_str())
        .unwrap_or_default()
        .to_string())
}

/// `utils.SelectTokenInformation` / `utils.SelectRole` (utils/qovery.go).
pub mod select {
    pub use qovery_services::{TokenInformation, select_role, select_token_information};
}
