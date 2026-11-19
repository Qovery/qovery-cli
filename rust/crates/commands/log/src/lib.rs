//! `qovery log` command. Port of `cmd/log.go`.
//!
//! * [`logs`]: port of `pkg/log.go` (logs streaming over the websocket gateway)
//! * [`ws`]: websocket client helpers and port of `pkg/wserror.go`, reusable by `shell` / `port-forward`

pub mod logs;
pub mod ws;

use qovery_api::models::Environment;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::context::{self, Service, ServiceType};
use qovery_core::{check_error, client, exit, resources, telemetry};

/// The `log` command.
pub fn command() -> Command {
    Command::new("log")
        .short("Print your application logs")
        .flag(Flag::bool("raw", "display logs in raw format (json)").short('r'))
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
        .flag(Flag::string("application", "Application Name").short('a'))
        .flag(Flag::string("container", "Container Name").short('n'))
        .flag(Flag::string("database", "Database Name").short('d'))
        .flag(Flag::string("job", "Job Name").short('j'))
        .flag(Flag::string("service", "Service Name").short('s'))
        .flag(Flag::string(
            "service-id",
            "Service ID (UUID) - skips name lookup, use when you already have the ID from a console URL",
        ))
        .run(run_log)
}

fn service(id: &str, name: &str, t: ServiceType) -> Service {
    Service {
        id: id.to_string(),
        name: name.to_string(),
        service_type: t.as_str().to_string(),
        blueprint_id: String::new(),
    }
}

use qovery_core::gosys::go_nil_pointer_panic as nil_pointer_panic;

fn run_log(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = check_error(client::get_client());
    let (org_id, project_id, env_id) = check_error(resources::ids_from_flags(&client, ctx));

    let application_name = ctx.string("application");
    let service_id = ctx.string("service-id");
    let svc = if !service_id.is_empty() {
        Service {
            id: service_id,
            ..Default::default()
        }
    } else if !application_name.is_empty() {
        let a = check_error(resources::application_by_name(
            &client,
            &application_name,
            &env_id,
        ))
        .unwrap_or_else(|| nil_pointer_panic());
        service(&a.id, &a.name, ServiceType::Application)
    } else if !ctx.string("container").is_empty() {
        let c = check_error(resources::container_by_name(
            &client,
            &ctx.string("container"),
            &env_id,
        ))
        .unwrap_or_else(|| nil_pointer_panic());
        service(&c.id, &c.name, ServiceType::Container)
    } else if !ctx.string("database").is_empty() {
        let d = check_error(resources::database_by_name(
            &client,
            &ctx.string("database"),
            &env_id,
            &application_name,
        ))
        .unwrap_or_else(|| nil_pointer_panic());
        service(&d.id, &d.name, ServiceType::Database)
    } else if !ctx.string("job").is_empty() {
        let j = check_error(resources::job_by_name(&client, &ctx.string("job"), &env_id))
            .unwrap_or_else(|| nil_pointer_panic());
        if !j.is_cron() && !j.is_lifecycle() {
            nil_pointer_panic();
        }
        service(&j.id, &j.name, ServiceType::Job)
    } else if !ctx.string("service").is_empty() {
        check_error(resources::service_by_name(
            &client,
            &ctx.string("service"),
            &env_id,
        ))
        .unwrap_or_else(|| nil_pointer_panic())
    } else {
        match context::current_service(true) {
            Ok(s) => s,
            Err(e) => {
                qovery_log::println_error(e);
                exit(0)
            }
        }
    };

    let environment: Environment = check_error(client.get(&format!("/environment/{env_id}")));

    let req = logs::LogRequest {
        service_id: svc.id,
        organization_id: org_id,
        project_id,
        environment_id: env_id,
        cluster_id: environment.cluster_id,
        raw_format: ctx.bool("raw"),
    };
    logs::exec_log(&req);
}

#[cfg(test)]
mod tests {
    use super::*;

    // port of cmd/log_test.go TestLogCmdFlags
    #[test]
    fn log_cmd_flags() {
        let cmd = command();
        for name in [
            "organization",
            "project",
            "environment",
            "application",
            "container",
            "database",
            "job",
            "service",
            "raw",
        ] {
            assert!(
                cmd.flags.iter().any(|f| f.name == name),
                "flag --{name} should be registered"
            );
        }
    }
}
