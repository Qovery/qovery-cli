//! Interactive selections (`utils.SelectOrganization`, `SelectProject`, `SelectEnvironment`, `SelectService`).
//!
//! When stdin is not a terminal (scripts, pipes, the parity harness), promptui is emulated byte for byte
//! ([`crate::promptui_select`], [`crate::promptui_prompt`]): same rendering on stdout, same keys read from
//! stdin, same stdin consumption (input read by one prompt beyond its answer is lost for the next ones).
//! On a terminal promptui is replaced by `inquire`; the prompt rendering differs, but the surrounding output
//! ("Organization:" header) and the selection logic (case-insensitive sort, automatic choice when there is a
//! single item, case-insensitive substring search) are the same.

use crate::client::get_client;
use crate::context::{
    EnvironmentRef, IdName, Service, ServiceType, set_environment, set_organization, set_project,
    set_service,
};
use qovery_api::models::{
    Application, Container, Database, Environment, Helm, Job, Organization, Project, Terraform,
};
use std::collections::HashMap;

/// `sortNamesCaseInsensitive` (stable sort on lowercase names).
pub fn sort_names_case_insensitive(names: &mut [String]) {
    names.sort_by_key(|a| a.to_lowercase());
}

fn stdin_is_terminal() -> bool {
    std::io::IsTerminal::is_terminal(&std::io::stdin())
}

/// promptui `Select` with a case-insensitive substring searcher. Returns the selected item.
pub fn prompt_select(items: &[String], page_size: Option<usize>) -> Result<String, String> {
    if !stdin_is_terminal() {
        let searcher = crate::promptui_select::contains_searcher(items);
        let opts = crate::promptui_select::SelectOptions {
            label: None,
            items,
            size: page_size.unwrap_or(5),
            searcher: Some(&searcher),
            start_in_search_mode: false,
            hide_help: false,
        };
        return crate::promptui_select::select_piped(&opts).map(|(_, item)| item);
    }
    let scorer: inquire::type_aliases::Scorer<'_, String> = &|input, _option, string_value, idx| {
        string_value
            .to_lowercase()
            .contains(&input.to_lowercase())
            .then_some(-(idx as i64))
    };
    let mut p = inquire::Select::new("", items.to_vec()).with_scorer(scorer);
    if let Some(size) = page_size {
        p = p.with_page_size(size);
    }
    p.prompt().map_err(|e| prompt_error(&e))
}

/// promptui `Select{Label, Items, Size}` without searcher (e.g. `Yes`/`No` confirmations, `Select your .env
/// file`). Returns the index and the value of the selected item.
pub fn prompt_select_with_label(
    label: &str,
    items: &[String],
    page_size: Option<usize>,
) -> Result<(usize, String), String> {
    if !stdin_is_terminal() {
        let opts = crate::promptui_select::SelectOptions {
            label: Some(label),
            items,
            size: page_size.unwrap_or(5),
            searcher: None,
            start_in_search_mode: false,
            hide_help: false,
        };
        return crate::promptui_select::select_piped(&opts);
    }
    let mut p = inquire::Select::new(label, items.to_vec());
    if let Some(size) = page_size {
        p = p.with_page_size(size);
    }
    p.raw_prompt()
        .map(|o| (o.index, o.value))
        .map_err(|e| prompt_error(&e))
}

/// promptui `Select{Label, Items, Size, Searcher, StartInSearchMode: true}`
/// (`promptuifactory.RunSelectWithSizeAndSearcher`). Returns the index and the value of the selected item.
pub fn prompt_select_with_searcher(
    label: &str,
    items: &[String],
    size: usize,
    searcher: &dyn Fn(&str, usize) -> bool,
) -> Result<(usize, String), String> {
    if !stdin_is_terminal() {
        let opts = crate::promptui_select::SelectOptions {
            label: Some(label),
            items,
            size,
            searcher: Some(searcher),
            start_in_search_mode: true,
            hide_help: false,
        };
        return crate::promptui_select::select_piped(&opts);
    }
    let scorer: inquire::type_aliases::Scorer<'_, String> =
        &|input, _option, _string_value, idx| searcher(input, idx).then_some(-(idx as i64));
    inquire::Select::new(label, items.to_vec())
        .with_page_size(if size == 0 { 5 } else { size })
        .with_scorer(scorer)
        .raw_prompt()
        .map(|o| (o.index, o.value))
        .map_err(|e| prompt_error(&e))
}

/// promptui `Prompt` (free text input).
pub fn prompt_text(label: &str) -> Result<String, String> {
    prompt_text_with_default(label, "")
}

/// promptui `Prompt{Label, Default}` (`promptuifactory.RunPrompt`): the default value is replaced by the first
/// typed key, Enter keeps it.
pub fn prompt_text_with_default(label: &str, default: &str) -> Result<String, String> {
    if !stdin_is_terminal() {
        let opts = crate::promptui_prompt::PromptOptions {
            label,
            default,
            templates: crate::promptui_prompt::PromptTemplates::Default,
            validate: None,
        };
        return crate::promptui_prompt::prompt_piped(&opts);
    }
    let mut p = inquire::Text::new(label);
    if !default.is_empty() {
        p = p.with_initial_value(default);
    }
    p.prompt().map_err(|e| prompt_error(&e))
}

/// Map inquire errors to promptui's (`^C`, `^D`).
pub fn prompt_error(e: &inquire::InquireError) -> String {
    match e {
        inquire::InquireError::OperationInterrupted => "^C".to_string(),
        // promptui reads EOF when stdin is not a terminal
        inquire::InquireError::OperationCanceled | inquire::InquireError::NotTTY => {
            "^D".to_string()
        }
        other => other.to_string(),
    }
}

/// `utils.SelectOrganization()`.
pub fn select_organization() -> Result<IdName, String> {
    let client = get_client()?;
    let organizations: Vec<Organization> =
        client.list("/organization").map_err(|e| e.to_string())?;
    let mut names: Vec<String> = Vec::new();
    let mut ids: HashMap<String, String> = HashMap::new();
    for o in organizations {
        names.push(o.name.clone());
        ids.insert(o.name, o.id);
    }
    sort_names_case_insensitive(&mut names);
    if names.is_empty() {
        return Err("no organizations found".to_string());
    }
    if names.len() == 1 {
        return Ok(IdName {
            id: ids[&names[0]].clone(),
            name: names[0].clone(),
        });
    }
    println!("Organization:");
    let selected = prompt_select(&names, Some(30))?;
    Ok(IdName {
        id: ids[&selected].clone(),
        name: selected,
    })
}

/// `utils.SelectAndSetOrganization()`.
pub fn select_and_set_organization() -> Result<IdName, String> {
    let org = select_organization()?;
    if let Err(e) = set_organization(&org) {
        qovery_log::println_error(&e);
        return Err(e);
    }
    Ok(org)
}

/// `utils.SelectProject(organizationID)`.
pub fn select_project(organization_id: &str) -> Result<IdName, String> {
    let client = get_client()?;
    let projects: Vec<Project> = client
        .list(&format!("/organization/{organization_id}/project"))
        .map_err(|e| e.to_string())?;
    let mut names: Vec<String> = Vec::new();
    let mut ids: HashMap<String, String> = HashMap::new();
    for p in projects {
        names.push(p.name.clone());
        ids.insert(p.name, p.id);
    }
    sort_names_case_insensitive(&mut names);
    if names.is_empty() {
        return Err("no projects found".to_string());
    }
    if names.len() == 1 {
        return Ok(IdName {
            id: ids[&names[0]].clone(),
            name: names[0].clone(),
        });
    }
    println!("Project:");
    let selected = prompt_select(&names, None).inspect_err(|e| qovery_log::println_error(e))?;
    Ok(IdName {
        id: ids[&selected].clone(),
        name: selected,
    })
}

/// `utils.SelectAndSetProject(organizationID)`.
pub fn select_and_set_project(organization_id: &str) -> Result<IdName, String> {
    let p = select_project(organization_id)?;
    if let Err(e) = set_project(&p) {
        qovery_log::println_error(&e);
        return Err(e);
    }
    Ok(p)
}

/// `utils.SelectEnvironment(projectID)`.
pub fn select_environment(project_id: &str) -> Result<EnvironmentRef, String> {
    let client = get_client()?;
    let envs: Vec<Environment> = client
        .list(&format!("/project/{project_id}/environment"))
        .map_err(|e| e.to_string())?;
    let mut names: Vec<String> = Vec::new();
    let mut by_name: HashMap<String, Environment> = HashMap::new();
    for e in envs {
        names.push(e.name.clone());
        by_name.insert(e.name.clone(), e);
    }
    sort_names_case_insensitive(&mut names);
    if names.is_empty() {
        return Err("no environments found".to_string());
    }
    let to_ref = |name: &str| {
        let e = &by_name[name];
        EnvironmentRef {
            id: e.id.clone(),
            name: name.to_string(),
            cluster_id: e.cluster_id.clone(),
        }
    };
    if names.len() == 1 {
        return Ok(to_ref(&names[0]));
    }
    println!("Environment:");
    let selected = prompt_select(&names, None).inspect_err(|e| qovery_log::println_error(e))?;
    Ok(to_ref(&selected))
}

/// `utils.SelectAndSetEnvironment(projectID)`.
pub fn select_and_set_environment(project_id: &str) -> Result<EnvironmentRef, String> {
    let e = select_environment(project_id)?;
    if let Err(err) = set_environment(&e) {
        qovery_log::println_error(&err);
        return Err(err);
    }
    Ok(e)
}

/// `utils.SelectService(environmentID)`.
pub fn select_service(environment_id: &str) -> Result<Service, String> {
    let client = get_client()?;
    let e = |x: qovery_api::ApiError| x.to_string();
    let apps: Vec<Application> = client
        .list(&format!("/environment/{environment_id}/application"))
        .map_err(e)?;
    let containers: Vec<Container> = client
        .list(&format!("/environment/{environment_id}/container"))
        .map_err(e)?;
    let databases: Vec<Database> = client
        .list(&format!("/environment/{environment_id}/database"))
        .map_err(e)?;
    let jobs: Vec<Job> = client
        .list(&format!("/environment/{environment_id}/job"))
        .map_err(e)?;
    let helms: Vec<Helm> = client
        .list(&format!("/environment/{environment_id}/helm"))
        .map_err(e)?;
    let terraforms: Vec<Terraform> = client
        .list(&format!("/environment/{environment_id}/terraform"))
        .map_err(e)?;

    let mut names: Vec<String> = Vec::new();
    let mut services: HashMap<String, Service> = HashMap::new();
    let mut add = |id: &str, name: &str, t: ServiceType, blueprint_id: String| {
        names.push(name.to_string());
        services.insert(
            name.to_string(),
            Service {
                id: id.to_string(),
                name: name.to_string(),
                service_type: t.as_str().to_string(),
                blueprint_id,
            },
        );
    };
    for a in &apps {
        add(&a.id, &a.name, ServiceType::Application, String::new());
    }
    for c in &containers {
        add(&c.id, &c.name, ServiceType::Container, String::new());
    }
    for d in &databases {
        add(&d.id, &d.name, ServiceType::Database, String::new());
    }
    for j in &jobs {
        if j.is_cron() || j.is_lifecycle() {
            add(&j.id, &j.name, ServiceType::Job, String::new());
        }
    }
    for h in &helms {
        add(&h.id, &h.name, ServiceType::Helm, String::new());
    }
    for t in &terraforms {
        let bp = t
            .extra
            .get("blueprint_id")
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();
        add(&t.id, &t.name, ServiceType::Terraform, bp);
    }
    sort_names_case_insensitive(&mut names);
    if names.is_empty() {
        return Err("no services found".to_string());
    }
    if names.len() == 1 {
        return Ok(services[&names[0]].clone());
    }
    println!("Services:");
    let selected = prompt_select(&names, None).inspect_err(|e| qovery_log::println_error(e))?;
    Ok(services[&selected].clone())
}

/// `utils.SelectAndSetService(environmentID)`.
pub fn select_and_set_service(environment_id: &str) -> Result<Service, String> {
    let s = select_service(environment_id).inspect_err(|e| qovery_log::println_error(e))?;
    if let Err(e) = set_service(&s) {
        qovery_log::println_error(&e);
        return Err(e);
    }
    Ok(s)
}

/// `utils.Validate(actionType)`: type-to-confirm prompt, true when the user typed `yes`.
/// Exits with status 1 (logrus error `Prompt failed ...`) when the prompt fails.
pub fn validate(action_type: &str) -> bool {
    let label = format!("Please type \"yes\" to validate {action_type}: ");
    if !stdin_is_terminal() {
        return match crate::promptui::prompt_piped(&label) {
            Ok(r) => r == "yes",
            Err(e) => {
                qovery_log::logrus::error(format!("Prompt failed {e}"));
                crate::exit(1)
            }
        };
    }
    let validator =
        |input: &str| -> Result<inquire::validator::Validation, inquire::CustomUserError> {
            if input.is_empty() {
                Ok(inquire::validator::Validation::Invalid("no input".into()))
            } else {
                Ok(inquire::validator::Validation::Valid)
            }
        };
    match inquire::Text::new(&label)
        .with_validator(validator)
        .prompt()
    {
        Ok(r) => r == "yes",
        Err(e) => {
            qovery_log::logrus::error(format!("Prompt failed {}", prompt_error(&e)));
            crate::exit(1)
        }
    }
}
