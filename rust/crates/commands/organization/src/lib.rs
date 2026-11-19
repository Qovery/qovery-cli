//! `qovery organization` command. Port of `cmd/organization*.go`.
//!
//! This crate is the reference example of how a Go cobra command is ported:
//! * the command tree is declared with `qovery_cobra::{Command, Flag}` (same Use/Short/Long/flags as Go)
//! * the run function is a plain `fn(&Ctx)` that prints its errors and exits like the Go code

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, telemetry};
use qovery_log::gofmt::go_json;
use serde_json::Value;

/// The `organization` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("organization")
        .short("Manage Organization")
        .subcommand(list())
}

fn list() -> Command {
    Command::new("list")
        .short("List organizations the authenticated token has access to")
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let organizations: Vec<Value> = check_error(client.list("/organization"));

    if ctx.bool("json") {
        qovery_log::println(go_json(&Value::Array(organizations)));
        return;
    }

    let data: Vec<Vec<String>> = organizations
        .iter()
        .map(|o| {
            vec![
                o["id"].as_str().unwrap_or_default().to_string(),
                o["name"].as_str().unwrap_or_default().to_string(),
                o["plan"].as_str().unwrap_or_default().to_string(),
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Plan"], &data);
}
