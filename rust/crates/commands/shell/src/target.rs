//! Resolution of the service targeted by `shell`, `port-forward` and `list-pods`: port of
//! `shellRequestWithoutArg` / `shellRequestFromSelect` / `shellRequestFromContext` /
//! `shellRequestWithApplicationUrl` (`cmd/shell.go`) and of their `portForwardRequest...` twins
//! (`cmd/port-forward.go`), which only differ by a few messages and the terraform support.

use qovery_api::models::Environment;
use qovery_core::context::{self, QoveryContext, ServiceType};
use qovery_core::{client, select};
use qovery_services as services;

/// The command resolving its target.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Feature {
    Shell,
    PortForward,
}

impl Feature {
    fn name(&self) -> &'static str {
        match self {
            Feature::Shell => "shell",
            Feature::PortForward => "port-forward",
        }
    }
}

/// The ids sent to the websocket gateway.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Target {
    pub organization_id: String,
    pub project_id: String,
    pub environment_id: String,
    pub cluster_id: String,
    pub service_id: String,
    /// raw service type (`application`, ...), empty when unknown
    pub service_type: String,
    pub blueprint_id: String,
}

/// `shellRequestWithoutArg()` / `portForwardRequestWithoutArg()`.
pub fn without_arg(feature: Feature) -> Result<Target, String> {
    let mut use_context = false;
    let current = context::get_current_context()?;

    qovery_log::println_info("Current context:");
    if !current.service_id.is_empty()
        && !current.service_name.is_empty()
        && !current.environment_id.is_empty()
        && !current.environment_name.is_empty()
        && !current.project_id.is_empty()
        && !current.project_name.is_empty()
        && !current.organization_id.is_empty()
        && !current.organization_name.is_empty()
    {
        if context::print_context().is_err() {
            println!("Context not yet configured.");
        }
        println!();

        qovery_log::println_info(format!(
            "Continue with {} command using this context ?",
            feature.name()
        ));
        use_context = select::validate("context");
        println!();
    } else if context::print_context().is_err() {
        println!("Context not yet configured.");
        println!(
            "Unable to use current context for `{}` command.",
            feature.name()
        );
        println!();
    }

    if use_context {
        from_context(&current, feature)
    } else {
        from_select()
    }
}

/// `shellRequestFromSelect()` / `portForwardRequestFromSelect()`.
fn from_select() -> Result<Target, String> {
    qovery_log::println_info("Select organization");
    let org = select::select_organization()?;

    qovery_log::println_info("Select project");
    let project = select::select_project(&org.id)?;

    qovery_log::println_info("Select environment");
    let env = select::select_environment(&project.id)?;

    qovery_log::println_info("Select service");
    let service = select::select_service(&env.id)?;

    Ok(Target {
        organization_id: org.id,
        project_id: project.id,
        environment_id: env.id,
        cluster_id: env.cluster_id,
        service_id: service.id,
        service_type: service.service_type,
        blueprint_id: service.blueprint_id,
    })
}

/// `shellRequestFromContext(currentContext)` / `portForwardRequestFromContext(currentContext)`.
fn from_context(current: &QoveryContext, feature: Feature) -> Result<Target, String> {
    let client = client::get_client_or_exit();
    let env: Environment = client
        .get(&format!("/environment/{}", current.environment_id))
        .map_err(|e| e.to_string())?;

    // The context keeps only ids: a blueprint database still needs its blueprint id for the host to be dialed.
    let mut blueprint_id = String::new();
    if feature == Feature::PortForward
        && current
            .service_type
            .eq_ignore_ascii_case(ServiceType::Terraform.as_str())
    {
        blueprint_id = services::get_terraform_by_id(&current.service_id)?.blueprint_id;
    }

    Ok(Target {
        organization_id: current.organization_id.clone(),
        project_id: current.project_id.clone(),
        environment_id: current.environment_id.clone(),
        cluster_id: env.cluster_id,
        service_id: current.service_id.clone(),
        service_type: current.service_type.clone(),
        blueprint_id,
    })
}

/// Go `strings.Replace(s, old, new, 1)`.
fn replace_once(s: &str, old: &str, new: &str) -> String {
    s.replacen(old, new, 1)
}

/// `shellRequestWithApplicationUrl(args)` / `portForwardRequestWithApplicationUrl(args)`.
pub fn with_application_url(args: &[String], feature: Feature) -> Result<Target, String> {
    let mut url = args[0].clone();
    url = replace_once(&url, "https://console.qovery.com/", "");
    url = replace_once(&url, "https://new.console.qovery.com/", "");
    let url_split: Vec<&str> = url.split('/').collect();

    if url_split.len() < 8 {
        return Err(format!("Wrong URL format: {url}"));
    }

    let organization = services::get_organization_by_id(url_split[1])?;
    let project = services::get_project_by_id(url_split[3])?;
    let environment_id = url_split[5];
    let environment = services::get_environment_by_id(environment_id)?;
    let environment_services = services::get_environment_services_by_id(environment_id)?;

    let mut service = context::Service::default();
    let service_id = url_split[7];
    let simple = |id: String, name: String, t: ServiceType| context::Service {
        id,
        name,
        service_type: t.as_str().to_string(),
        blueprint_id: String::new(),
    };
    for env_service in &environment_services {
        if env_service.id != service_id {
            continue;
        }
        service = match env_service.service_type {
            ServiceType::Application => {
                let a = services::get_application_by_id(service_id)?;
                simple(a.id, a.name, ServiceType::Application)
            }
            ServiceType::Container => {
                let c = services::get_container_by_id(service_id)?;
                simple(c.id, c.name, ServiceType::Container)
            }
            ServiceType::Job => {
                let j = services::get_job_by_id(service_id)?;
                simple(j.id, j.name, ServiceType::Job)
            }
            ServiceType::Database => services::get_database_by_id(service_id)?,
            ServiceType::Helm => services::get_helm_by_id(service_id)?,
            ServiceType::Terraform if feature == Feature::PortForward => {
                services::get_terraform_by_id(service_id)?
            }
            other => {
                return Err(format!(
                    "ServiceLevel type `{}` is not supported for {}",
                    other.as_str(),
                    feature.name()
                ));
            }
        };
    }
    if feature == Feature::PortForward && service.id.is_empty() {
        return Err(format!(
            "Service {service_id} not found in environment {environment_id}"
        ));
    }

    qovery_log::table::Table::new(vec![
        vec!["Organization".into(), organization.name.clone()],
        vec!["Project".into(), project.name.clone()],
        vec!["Environment".into(), environment.name.clone()],
        vec!["ServiceLevel".into(), service.name.clone()],
        vec!["ServiceType".into(), service.service_type.clone()],
    ])
    .render();

    Ok(Target {
        organization_id: organization.id,
        project_id: project.id,
        environment_id: environment.id,
        cluster_id: environment.cluster_id,
        service_id: service.id,
        service_type: service.service_type,
        blueprint_id: service.blueprint_id,
    })
}
