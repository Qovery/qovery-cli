//! `qovery enterprise-connection` command. Port of `cmd/enterprise_connection*.go`.
//!
//! [`service`] is the port of `pkg/enterpriseconnection` (public, reusable by `admin enterprise-connection`).

pub mod service;

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, exit, telemetry};
use service::{EnterpriseConnectionService, create_connection_update_dto, parse_idp_group_names};

/// The `enterprise-connection` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("enterprise-connection")
        .short("Manage enterprise connections")
        .run(run_group)
        .subcommand(get())
        .subcommand(group_mappings())
        .subcommand(update())
}

fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

fn organization_flag() -> Flag {
    Flag::string("organization", "Organization Name").short('o')
}

fn connection_flag() -> Flag {
    Flag::string("connection", "Connection Name").short('c')
}

fn get() -> Command {
    Command::new("get")
        .short("Get enterprise connection information")
        .flag(organization_flag())
        .flag(connection_flag())
        .run(run_get)
}

fn update() -> Command {
    Command::new("update")
        .short("Update enterprise connection information")
        .flag(organization_flag())
        .flag(connection_flag().required())
        .flag(Flag::string("default-role", "Default Role").short('r'))
        .flag(Flag::bool("enforce-group-sync", "").short('e'))
        .run(run_update)
}

fn group_mappings() -> Command {
    Command::new("group-mappings")
        .short("Manage enterprise connection group mappings")
        .run(run_group)
        .subcommand(
            Command::new("add")
                .short("Add or modify an enterprise connection group mapping")
                .flag(organization_flag())
                .flag(connection_flag().required())
                .flag(Flag::string("qovery-role", "Qovery role name to target").short('q'))
                .flag(
                    Flag::string("idp-group-names", "Your IDP group names (comma separated)")
                        .short('i'),
                )
                .run(run_group_mappings_add),
        )
        .subcommand(
            Command::new("delete")
                .short("Delete enterprise connection group mapping")
                .flag(organization_flag())
                .flag(connection_flag().required())
                .flag(Flag::string("qovery-role", "Qovery role to target").short('q'))
                .run(run_group_mappings_delete),
        )
        .subcommand(
            Command::new("get")
                .short("Get enterprise connection group mappings")
                .flag(organization_flag())
                .flag(connection_flag().required())
                .run(run_group_mappings_get),
        )
}

fn run_get(ctx: &Ctx) {
    let service = check_error(EnterpriseConnectionService::new(
        &ctx.string("organization"),
    ));
    let connections = service.list_enterprise_connections(&ctx.string("connection"));
    for (i, connection) in connections.iter().enumerate() {
        if i > 0 {
            qovery_log::println(format!("\n{}\n", "-".repeat(50)));
        }
        service.display_enterprise_connection(connection);
    }
}

fn run_update(ctx: &Ctx) {
    let service = check_error(EnterpriseConnectionService::new(
        &ctx.string("organization"),
    ));
    let connection_name = ctx.string("connection");
    let existing = check_error(service.get_enterprise_connection(&connection_name));

    let default_role = ctx.string("default-role");
    let role = if default_role.is_empty() {
        existing.default_role.clone()
    } else {
        check_error(service.resolve_provided_role_name_or_custom_role_id(&default_role))
    };

    let dto = create_connection_update_dto(
        &role,
        ctx.bool("enforce-group-sync"),
        existing.group_mappings.clone(),
    );
    let connection = check_error(service.update_enterprise_connection(&connection_name, &dto));
    service.display_enterprise_connection(&connection);
}

fn run_group_mappings_add(ctx: &Ctx) {
    let service = check_error(EnterpriseConnectionService::new(
        &ctx.string("organization"),
    ));
    let connection_name = ctx.string("connection");
    let existing = check_error(service.get_enterprise_connection(&connection_name));

    let qovery_role = ctx.string("qovery-role");
    if let Err(e) = service.validate_role(&qovery_role) {
        qovery_log::println_error(format!(
            "this role doesn't exist in your organization: {qovery_role} - {e}"
        ));
        return;
    }
    let role = check_error(service.resolve_provided_role_name_or_custom_role_id(&qovery_role));

    let mut mappings = existing.group_mappings.clone();
    mappings.insert(role, parse_idp_group_names(&ctx.string("idp-group-names")));

    let dto = create_connection_update_dto(
        &existing.default_role,
        existing.enforce_group_sync,
        mappings,
    );
    let connection = check_error(service.update_enterprise_connection(&connection_name, &dto));
    service.display_group_mappings_table(&connection.group_mappings);
}

fn run_group_mappings_delete(ctx: &Ctx) {
    let service = check_error(EnterpriseConnectionService::new(
        &ctx.string("organization"),
    ));
    let connection_name = ctx.string("connection");
    let existing = check_error(service.get_enterprise_connection(&connection_name));

    let qovery_role = ctx.string("qovery-role");
    let role = check_error(service.resolve_provided_role_name_or_custom_role_id(&qovery_role));

    let mut mappings = existing.group_mappings.clone();
    if mappings.remove(&role).is_none() {
        qovery_log::println_info(format!(
            "The role '{qovery_role}' is not present in group mappings, skipping."
        ));
        return;
    }

    let dto = create_connection_update_dto(
        &existing.default_role,
        existing.enforce_group_sync,
        mappings,
    );
    let connection = check_error(service.update_enterprise_connection(&connection_name, &dto));
    service.display_group_mappings_table(&connection.group_mappings);
}

fn run_group_mappings_get(ctx: &Ctx) {
    let service = check_error(EnterpriseConnectionService::new(
        &ctx.string("organization"),
    ));
    let connection = check_error(service.get_enterprise_connection(&ctx.string("connection")));
    service.display_group_mappings_table(&connection.group_mappings);
}
