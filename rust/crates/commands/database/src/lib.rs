//! `qovery database` command. Port of `cmd/database*.go`.

mod list;

use qovery_api::models::Database;
use qovery_api::{Client, Method};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit, exit_with_error, resources, telemetry};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{
    RebootServicesRequest, database_ids, new_services_watch, path_escape, state,
};

/// The `database` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("database")
        .short("Manage databases")
        .run(run_group)
        .subcommand(cancel())
        .subcommand(delete())
        .subcommand(deploy())
        .subcommand(list::command())
        .subcommand(redeploy())
        .subcommand(restart())
        .subcommand(stop())
}

/// Go group commands: `utils.Capture(cmd); if len(args) == 0 { cmd.Help(); os.Exit(0) }`.
fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

fn context_flags(cmd: Command) -> Command {
    cmd.flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
}

const WATCH_USAGE: &str = "Watch database status until it's ready or an error occurs";

fn cancel() -> Command {
    Command::new("cancel")
        .short("Cancel a database deployment")
        .run(|ctx| {
            telemetry::capture(ctx);
            qovery_log::println_info("Use: 'qovery environment cancel' to cancel this deployment");
        })
}

fn delete() -> Command {
    context_flags(Command::new("delete").short("Delete a database"))
        .flag(Flag::string("database", "Database Name").short('n'))
        .flag(Flag::string(
            "databases",
            "Database Names (comma separated) Example: --databases \"db1,db2\"",
        ))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_delete)
}

fn deploy() -> Command {
    context_flags(Command::new("deploy").short("Deploy a database"))
        .flag(Flag::string("database", "Database Name").short('n'))
        .flag(Flag::string(
            "databases",
            "Database Names (comma separated) (ex: --databases \"database1,database2\")",
        ))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_deploy)
}

fn redeploy() -> Command {
    context_flags(Command::new("redeploy").short("Redeploy a database"))
        .flag(
            Flag::string("database", "Database Name")
                .short('n')
                .required(),
        )
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_redeploy)
}

fn restart() -> Command {
    context_flags(Command::new("restart").short("Restart a database without redeploying it"))
        .flag(Flag::string("database", "Database Name").short('n'))
        .flag(Flag::string(
            "databases",
            "Database Names (comma separated) Example: --databases \"db1,db2\"",
        ))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_restart)
}

fn stop() -> Command {
    context_flags(Command::new("stop").short("Stop a database"))
        .flag(Flag::string("database", "Database Name").short('n'))
        .flag(Flag::string(
            "databases",
            "Database Names (comma separated) Example: --databases \"db1,db2\"",
        ))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
        .run(run_stop)
}

/// Common prelude of the action commands: client, arguments validation, environment id and the databases.
fn prelude(ctx: &Ctx) -> (Client, String, Vec<Database>, String, String) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let name = ctx.string("database");
    let names = ctx.string("databases");
    validate_database_arguments(&name, &names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);
    let databases = build_database_list_from_database_names(&client, &env_id, &name, &names);
    (client, env_id, databases, name, names)
}

fn names_text(name: &str, names: &str) -> String {
    FG_BLUE.sprint(format!("{name}{names}"))
}

fn post(client: &Client, path: &str, body: Option<&serde_json::Value>) -> Result<(), String> {
    client
        .execute(Method::POST, path, &[], body)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

fn run_delete(ctx: &Ctx) {
    let (client, env_id, databases, name, names) = prelude(ctx);
    let watch = new_services_watch(
        &client,
        &env_id,
        database_ids(&databases),
        ctx.bool("watch"),
    );
    let ids: Vec<String> = databases.iter().map(|d| d.id.clone()).collect();
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "delete",
        &qovery_services::ServiceIdsAllRequest {
            database_ids: ids,
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to delete database(s) {} has been queued...",
        names_text(&name, &names)
    ));
    watch.wait(state::DELETED);
}

fn run_deploy(ctx: &Ctx) {
    let (client, env_id, databases, name, names) = prelude(ctx);
    let watch = new_services_watch(
        &client,
        &env_id,
        database_ids(&databases),
        ctx.bool("watch"),
    );
    check_error(qovery_services::deploy_databases(
        &client, &env_id, &databases,
    ));
    qovery_log::println(format!(
        "Request to deploy database(s) {} has been queued..",
        names_text(&name, &names)
    ));
    watch.wait(state::DEPLOYED);
}

fn run_redeploy(ctx: &Ctx) {
    let (client, env_id, databases, name, names) = prelude(ctx);
    let watch = new_services_watch(
        &client,
        &env_id,
        database_ids(&databases),
        ctx.bool("watch"),
    );
    // Go indexes databaseList[0]: the list always holds the --database (required flag)
    check_error(post(
        &client,
        &format!("/database/{}/deploy", path_escape(&databases[0].id)),
        None,
    ));
    qovery_log::println(format!(
        "Request to redeploy database(s) {} has been queued..",
        names_text(&name, &names)
    ));
    watch.wait(state::DEPLOYED);
}

fn run_restart(ctx: &Ctx) {
    let (client, env_id, databases, name, names) = prelude(ctx);
    let watch = new_services_watch(
        &client,
        &env_id,
        database_ids(&databases),
        ctx.bool("watch"),
    );
    check_error(qovery_services::reboot_services(
        &client,
        &env_id,
        &RebootServicesRequest {
            database_ids: Some(database_ids(&databases)),
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to restart database(s) {} has been queued...",
        names_text(&name, &names)
    ));
    watch.wait(state::RESTARTED);
}

fn run_stop(ctx: &Ctx) {
    let (client, env_id, databases, name, names) = prelude(ctx);
    let watch = new_services_watch(
        &client,
        &env_id,
        database_ids(&databases),
        ctx.bool("watch"),
    );
    let ids: Vec<String> = databases.iter().map(|d| d.id.clone()).collect();
    check_error(qovery_services::selected_services_action(
        &client,
        &env_id,
        "stop",
        &qovery_services::ServiceIdsAllRequest {
            database_ids: ids,
            ..Default::default()
        },
    ));
    qovery_log::println(format!(
        "Request to stop databases {} has been queued...",
        names_text(&name, &names)
    ));
    watch.wait(state::STOPPED);
}

/// Port of `buildDatabaseListFromDatabaseNames`.
fn build_database_list_from_database_names(
    client: &Client,
    environment_id: &str,
    database_name: &str,
    database_names: &str,
) -> Vec<Database> {
    let databases: Vec<Database> = check_error(client.list(&format!(
        "/environment/{}/database",
        path_escape(environment_id)
    )));
    let not_found = |name: &str| -> ! {
        qovery_log::println_error(format!("database {name} not found"));
        qovery_log::println_info("You can list all databases with: qovery database list");
        exit(1)
    };
    let mut list = Vec::new();
    if !database_name.is_empty() {
        match qovery_core::find::find_by_name(&databases, database_name) {
            Some(d) => list.push(d.clone()),
            None => not_found(database_name),
        }
    }
    if !database_names.is_empty() {
        for name in database_names.split(',') {
            match qovery_core::find::find_by_name(&databases, name.trim()) {
                Some(d) => list.push(d.clone()),
                None => not_found(name),
            }
        }
    }
    list
}

/// Port of `validateDatabaseArguments`.
fn validate_database_arguments(database_name: &str, database_names: &str) {
    if database_name.is_empty() && database_names.is_empty() {
        exit_with_error(
            "use either --database \"<database name>\" or --databases \"<database1 name>, <database2 name>\" but not both at the same time",
        );
    }
    if !database_name.is_empty() && !database_names.is_empty() {
        exit_with_error("you can't use --database and --databases at the same time");
    }
}
