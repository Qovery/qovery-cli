//! `qovery service deploy`. Port of `cmd/service_deploy.go`.

use qovery_api::Client;
use qovery_api::models::{Application, Container, Database, Helm, Job};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::find_by_name;
use qovery_core::{check_error, client, exit, resources, telemetry};
use qovery_log::pterm;
use qovery_services as services;

const LONG: &str = r#"Deploy a service by automatically detecting its type.
This command works with applications, containers, databases, jobs (cronjobs and lifecycle), and helm charts.

The --version parameter accepts:
  - Git commit IDs (for applications, git-based jobs, git-based helms)
  - Container image tags (for image containers, image-based jobs)
  - Helm chart versions (for helm repository charts)

A Git container takes no --version: it deploys its current commit. To deploy another
commit, use: qovery container deploy -n my-container --commit-id abc123

For helm charts, you can also specify:
  - --values-override-version: Git commit ID for helm values override

Examples:
  qovery service deploy -n my-app --version abc123
  qovery service deploy -n my-container --version v1.2.3
  qovery service deploy -n my-database
  qovery service deploy -n my-helm-repo --version 1.2.3
  qovery service deploy -n my-helm-git --version abc123
  qovery service deploy -n my-helm --version 1.2.3 --values-override-version def456
  qovery service deploy --services "service1,service2,service3""#;

pub fn command() -> Command {
    Command::new("deploy")
        .short("Deploy a service (application, container, database, job, or helm)")
        .long(LONG)
        .flag(Flag::string("organization", "Organization Name"))
        .flag(Flag::string("project", "Project Name"))
        .flag(Flag::string("environment", "Environment Name"))
        .flag(Flag::string("service", "Service Name").short('n'))
        .flag(Flag::string(
            "services",
            "Service Names (comma separated) Example: --services \"svc1,svc2,svc3\"",
        ))
        .flag(
            Flag::string(
                "version",
                "Version (git commit ID, image tag, or chart version)",
            )
            .short('v'),
        )
        .flag(Flag::string(
            "values-override-version",
            "Helm values override version (git commit ID)",
        ))
        .flag(
            Flag::bool(
                "watch",
                "Watch service status until it's ready or an error occurs",
            )
            .short('w'),
        )
        .run(run_deploy)
}

/// `serviceDeployInfo`.
enum ServiceKind {
    Application(Application),
    Container(Container),
    Database(Database),
    Job(Job),
    Helm(Helm),
}

struct ServiceDeployInfo {
    name: String,
    kind: ServiceKind,
}

/// `serviceDeployId`.
fn service_deploy_id(svc: &ServiceDeployInfo) -> String {
    match &svc.kind {
        ServiceKind::Application(a) => a.id.clone(),
        ServiceKind::Container(c) => c.id.clone(),
        ServiceKind::Database(d) => d.id.clone(),
        ServiceKind::Job(j) => services::get_job_id(j),
        ServiceKind::Helm(h) => h.id.clone(),
    }
}

fn fail(msg: impl std::fmt::Display) -> ! {
    qovery_log::println_error(msg);
    exit(1)
}

/// `validateServiceDeployArguments`.
fn validate_arguments(service_name: &str, service_names: &str) {
    if service_name.is_empty() && service_names.is_empty() {
        fail("use either --service or --services");
    }
    if !service_name.is_empty() && !service_names.is_empty() {
        fail("use either --service or --services, not both");
    }
}

/// `getServicesToDeployByNames`.
fn services_to_deploy_by_names(
    client: &Client,
    env_id: &str,
    service_name: &str,
    service_names: &str,
) -> Vec<ServiceDeployInfo> {
    let mut names_to_find: Vec<String> = Vec::new();
    if !service_name.is_empty() {
        names_to_find.push(service_name.to_string());
    }
    if !service_names.is_empty() {
        names_to_find.extend(service_names.split(',').map(|n| n.trim().to_string()));
    }

    let applications: Vec<Application> =
        check_error(client.list(&format!("/environment/{env_id}/application")));
    let containers: Vec<Container> =
        check_error(client.list(&format!("/environment/{env_id}/container")));
    let databases: Vec<Database> =
        check_error(client.list(&format!("/environment/{env_id}/database")));
    let jobs: Vec<Job> = check_error(client.list(&format!("/environment/{env_id}/job")));
    let helms: Vec<Helm> = check_error(client.list(&format!("/environment/{env_id}/helm")));
    // utils.FindByJobName only matches cron and lifecycle jobs
    let jobs: Vec<Job> = jobs
        .into_iter()
        .filter(|j| j.is_cron() || j.is_lifecycle())
        .collect();

    let mut result = Vec::new();
    for name in names_to_find {
        let kind = if let Some(a) = find_by_name(&applications, &name) {
            ServiceKind::Application(a.clone())
        } else if let Some(c) = find_by_name(&containers, &name) {
            ServiceKind::Container(c.clone())
        } else if let Some(d) = find_by_name(&databases, &name) {
            ServiceKind::Database(d.clone())
        } else if let Some(j) = find_by_name(&jobs, &name) {
            ServiceKind::Job(j.clone())
        } else if let Some(h) = find_by_name(&helms, &name) {
            ServiceKind::Helm(h.clone())
        } else {
            qovery_log::println_error(format!("service '{name}' not found"));
            qovery_log::println_info("You can list all services with: qovery service list");
            exit(1)
        };
        result.push(ServiceDeployInfo { name, kind });
    }
    result
}

fn run_deploy(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = client::get_client_or_exit();
    let service_name = ctx.string("service");
    let service_names = ctx.string("services");
    validate_arguments(&service_name, &service_names);
    let env_id = resources::environment_id_from_flags_or_exit(&client, ctx);

    let to_deploy = services_to_deploy_by_names(&client, &env_id, &service_name, &service_names);
    if to_deploy.is_empty() {
        fail("no services found to deploy");
    }

    let mut applications = Vec::new();
    let mut containers = Vec::new();
    let mut databases = Vec::new();
    let mut jobs = Vec::new();
    let mut helms = Vec::new();
    for svc in &to_deploy {
        match &svc.kind {
            ServiceKind::Application(a) => applications.push(a.clone()),
            ServiceKind::Container(c) => containers.push(c.clone()),
            ServiceKind::Database(d) => databases.push(d.clone()),
            ServiceKind::Job(j) => jobs.push(j.clone()),
            ServiceKind::Helm(h) => helms.push(h.clone()),
        }
    }

    let version = ctx.string("version");
    // checked before any deploy so a rejected container or application leaves the other services untouched
    for container in &containers {
        check_error(services::check_container_image_tag(container, &version));
    }
    check_error(services::check_applications_commit_id(
        &applications,
        &version,
        "--version",
    ));

    let watch = services::new_services_watch(
        &client,
        &env_id,
        to_deploy.iter().map(service_deploy_id).collect(),
        ctx.bool("watch"),
    );

    if !applications.is_empty() {
        check_error(services::deploy_applications(
            &client,
            &env_id,
            &applications,
            &version,
        ));
    }
    if !containers.is_empty() {
        check_error(services::deploy_containers(
            &client,
            &env_id,
            &containers,
            &version,
        ));
    }
    if !databases.is_empty() {
        check_error(services::deploy_databases(&client, &env_id, &databases));
    }
    if !jobs.is_empty() {
        check_error(services::deploy_jobs(
            &client, &env_id, &jobs, &version, &version,
        ));
    }
    if !helms.is_empty() {
        check_error(services::deploy_helms(
            &client,
            &env_id,
            &helms,
            &version,
            &version,
            &ctx.string("values-override-version"),
        ));
    }

    let names: Vec<&str> = to_deploy.iter().map(|s| s.name.as_str()).collect();
    qovery_log::println(format!(
        "Request to deploy service(s) {} has been queued..",
        pterm::FG_BLUE.sprint(names.join(", "))
    ));

    watch.wait(services::state::DEPLOYED);
}
