//! `qovery terraform` command. Port of `cmd/terraform*.go`.

mod external_secret;
mod setup_backend;

use qovery_api::Client;
use qovery_api::models::{EnvironmentStatuses, Terraform};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::status::{find_status, find_status_text_with_color};
use qovery_core::{check_error, client, exit, exit_with_error, resources, telemetry};
use qovery_log::gofmt::{go_json, time_string_from_rfc3339};
use qovery_log::pterm::FG_BLUE;
use qovery_services::{new_services_watch, path_escape, state, terraform_ids, to_iso8601_value};
use serde_json::{Value, json};

/// The `terraform` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("terraform")
        .short("Manage terraform services")
        .run(run_group)
        .subcommand(delete())
        .subcommand(external_secret::command())
        .subcommand(force_unlock())
        .subcommand(list())
        .subcommand(migrate_state())
        .subcommand(plan())
        .subcommand(plan_and_apply())
        .subcommand(setup_backend::command())
}

/// Go group commands: `utils.Capture(cmd); if len(args) == 0 { cmd.Help(); os.Exit(0) }`.
pub(crate) fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

pub(crate) fn context_flags(cmd: Command) -> Command {
    cmd.flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
}

const TERRAFORMS_USAGE: &str =
    "Terraform Names (comma separated) Example: --terraforms \"tf1,tf2,tf3\"";
const COMMIT_ID_USAGE: &str = "Git Commit ID, or 'latest' for the newest commit of the branch (optional, defaults to deployed commit)";
const WATCH_USAGE: &str = "Watch terraform status until it's ready or an error occurs";

/// Commands deploying terraforms (`plan-and-apply`, `plan`, `force-unlock`, `migrate-state`).
fn deploy_like(cmd: Command) -> Command {
    context_flags(cmd)
        .flag(Flag::string("terraform", "Terraform Name").short('n'))
        .flag(Flag::string("terraforms", TERRAFORMS_USAGE))
        .flag(Flag::string("commit-id", COMMIT_ID_USAGE).short('c'))
        .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
}

fn delete() -> Command {
    context_flags(
        Command::new("delete")
            .short("Delete terraform resources")
            .long(
                "Delete terraform resources and remove from Qovery.

By default, this will execute 'terraform destroy' to delete all resources
managed by this terraform service, then remove the service from Qovery.

Use --skip-destroy to keep the infrastructure resources but remove the
Qovery configuration. This is useful when you want to manage the resources
outside of Qovery or import them into another system.

Use --resources-only to delete the infrastructure resources but keep the
Qovery configuration. This is useful when you want to clean up resources
while keeping the terraform service definition in Qovery.",
            ),
    )
    .flag(Flag::string("terraform", "Terraform Name").short('n'))
    .flag(Flag::string("terraforms", TERRAFORMS_USAGE))
    .flag(Flag::bool(
        "skip-destroy",
        "Skip terraform destroy (keep resources, only remove from Qovery)",
    ))
    .flag(Flag::bool(
        "resources-only",
        "Delete resources only (keep Qovery configuration)",
    ))
    .flag(Flag::bool("watch", WATCH_USAGE).short('w'))
    .run(run_delete)
}

fn force_unlock() -> Command {
    deploy_like(
        Command::new("force-unlock")
            .short("Force unlock terraform state file")
            .long(
                "Force unlock terraform state file when it's stuck.

This command will execute 'terraform force-unlock' on the specified terraform service(s).
Use this when a state lock is preventing operations and you're certain no other
operations are running.",
            ),
    )
    .run(|ctx| {
        run_deploy(
            ctx,
            Some("FORCE_UNLOCK"),
            "Request to force unlock terraform(s) {} state has been queued..",
        )
    })
}

fn migrate_state() -> Command {
    deploy_like(
        Command::new("migrate-state")
            .short("Migrate terraform state to new backend")
            .long(
                "Migrate terraform state to a new backend configuration.

This command will execute 'terraform init -migrate-state' on the specified
terraform service(s). Use this when changing backend configuration (e.g.,
moving from local state to S3, or changing S3 bucket).

Make sure to update your terraform backend configuration before running
this command.",
            ),
    )
    .run(|ctx| {
        run_deploy(
            ctx,
            Some("MIGRATE_STATE"),
            "Request to migrate terraform(s) {} state has been queued..",
        )
    })
}

fn plan() -> Command {
    deploy_like(Command::new("plan").short("Run terraform plan (dry-run)")).run(|ctx| {
        run_deploy(
            ctx,
            Some("PLAN"),
            "Request to plan terraform(s) {} has been queued..",
        )
    })
}

fn plan_and_apply() -> Command {
    deploy_like(Command::new("plan-and-apply").short("Deploy terraform (plan and apply)")).run(
        |ctx| {
            run_deploy(
                ctx,
                None,
                "Request to deploy terraform(s) {} has been queued..",
            )
        },
    )
}

fn list() -> Command {
    context_flags(Command::new("list").short("List terraform services"))
        .flag(Flag::bool("json", "JSON output"))
        .run(run_list)
}

/// Common prelude of the action commands: client, arguments validation and environment id.
fn prelude(ctx: &Ctx) -> (Client, String, String, String) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();
    let name = ctx.string("terraform");
    let names = ctx.string("terraforms");
    validate_terraform_arguments(&name, &names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);
    (client, env_id, name, names)
}

fn names_text(name: &str, names: &str) -> String {
    FG_BLUE.sprint(format!("{name}{names}"))
}

fn run_deploy(ctx: &Ctx, action: Option<&str>, message: &str) {
    let (client, env_id, name, names) = prelude(ctx);
    let terraforms = build_terraform_list_from_terraform_names(&client, &env_id, &name, &names);
    let watch = new_services_watch(
        &client,
        &env_id,
        terraform_ids(&terraforms),
        ctx.bool("watch"),
    );
    check_error(qovery_services::deploy_terraforms(
        &client,
        &env_id,
        &terraforms,
        &ctx.string("commit-id"),
        action,
    ));
    qovery_log::println(message.replacen("{}", &names_text(&name, &names), 1));
    watch.wait(state::DEPLOYED);
}

fn run_delete(ctx: &Ctx) {
    let (client, env_id, name, names) = prelude(ctx);
    let skip_destroy = ctx.bool("skip-destroy");
    let resources_only = ctx.bool("resources-only");

    // Validate that skip-destroy and resources-only are mutually exclusive
    if skip_destroy && resources_only {
        qovery_log::println_error(
            "--skip-destroy and --resources-only flags are mutually exclusive",
        );
        return;
    }

    let terraforms = build_terraform_list_from_terraform_names(&client, &env_id, &name, &names);
    let watch = new_services_watch(
        &client,
        &env_id,
        terraform_ids(&terraforms),
        ctx.bool("watch"),
    );
    check_error(qovery_services::delete_terraforms(
        &client,
        &env_id,
        &terraforms,
        skip_destroy,
        resources_only,
    ));

    let text = names_text(&name, &names);
    if skip_destroy {
        qovery_log::println(format!(
            "Request to remove terraform(s) {text} from Qovery (keeping resources) has been queued.."
        ));
    } else if resources_only {
        qovery_log::println(format!(
            "Request to delete resources for terraform(s) {text} (keeping Qovery configuration) has been queued.."
        ));
    } else {
        qovery_log::println(format!(
            "Request to delete terraform(s) {text} has been queued.."
        ));
    }
    watch.wait(state::DELETED);
}

fn run_list(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = check_error(client::get_client());
    let (_, _, env_id) = check_error(resources::ids_from_flags(&client, ctx));
    let env = path_escape(&env_id);
    let terraforms: Vec<Terraform> =
        check_error(client.list(&format!("/environment/{env}/terraform")));
    let statuses: EnvironmentStatuses =
        check_error(client.get(&format!("/environment/{env}/statuses")));

    if ctx.bool("json") {
        print!(
            "{}",
            get_terraform_json_output(&statuses.terraforms, &terraforms)
        );
        return;
    }

    let data: Vec<Vec<String>> = terraforms
        .iter()
        .map(|t| {
            vec![
                t.id.clone(),
                t.name.clone(),
                "Terraform".to_string(),
                find_status_text_with_color(&statuses.terraforms, &t.id),
                match t.updated_at.as_deref() {
                    Some(s) => time_string_from_rfc3339(s),
                    // Go: UpdatedAt.String() on a nil *time.Time
                    None => qovery_envvars::go_nil_pointer_panic(),
                },
            ]
        })
        .collect();
    qovery_log::print_table(&["Id", "Name", "Type", "Status", "Last Update"], &data);
}

/// Port of `getTerraformJsonOutput`.
fn get_terraform_json_output(
    statuses: &[qovery_api::models::Status],
    terraforms: &[Terraform],
) -> String {
    let results: Vec<Value> = terraforms
        .iter()
        .map(|t| {
            json!({
                "id": t.id,
                "name": t.name,
                "type": "Terraform",
                "status": find_status(statuses, &t.id),
                "updated_at": to_iso8601_value(t.updated_at.as_ref().map(|s| Value::String(s.clone())).as_ref()),
            })
        })
        .collect();
    if results.is_empty() {
        // json.Marshal of a nil slice
        return "null".to_string();
    }
    go_json(&Value::Array(results))
}

/// Port of `buildTerraformListFromTerraformNames`.
fn build_terraform_list_from_terraform_names(
    client: &Client,
    environment_id: &str,
    terraform_name: &str,
    terraform_names: &str,
) -> Vec<Terraform> {
    let terraforms: Vec<Terraform> = check_error(client.list(&format!(
        "/environment/{}/terraform",
        path_escape(environment_id)
    )));
    let mut list = Vec::new();
    if !terraform_name.is_empty() {
        match qovery_core::find::find_by_name(&terraforms, terraform_name) {
            Some(t) => list.push(t.clone()),
            None => terraform_not_found(terraform_name),
        }
    }
    if !terraform_names.is_empty() {
        for name in terraform_names.split(',') {
            match qovery_core::find::find_by_name(&terraforms, name.trim()) {
                Some(t) => list.push(t.clone()),
                None => terraform_not_found(name),
            }
        }
    }
    list
}

pub(crate) fn terraform_not_found(name: &str) -> ! {
    qovery_log::println_error(format!("terraform {name} not found"));
    qovery_log::println_info("You can list all terraforms with: qovery terraform list");
    exit(1)
}

/// Port of `validateTerraformArguments`.
fn validate_terraform_arguments(terraform_name: &str, terraform_names: &str) {
    if terraform_name.is_empty() && terraform_names.is_empty() {
        exit_with_error(
            "use either --terraform \"<terraform name>\" or --terraforms \"<terraform1 name>, <terraform2 name>\" but not both at the same time",
        );
    }
    if !terraform_name.is_empty() && !terraform_names.is_empty() {
        exit_with_error("you can't use --terraform and --terraforms at the same time");
    }
}
