//! Interactive token information selection of `utils/qovery.go` (`SelectRole`, `SelectTokenInformation`).
//!
//! Prompts through `qovery_core::select` (promptui byte-identical when stdin is not a terminal).

use crate::util::{path_escape, str_field};
use qovery_core::client::get_client;
use qovery_core::context::IdName;
use qovery_core::select::{prompt_select, prompt_text, select_organization};
use serde_json::Value;
use std::collections::HashMap;

/// `utils.Role`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Role {
    pub id: String,
    pub name: String,
}

/// `utils.TokenInformation`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct TokenInformation {
    pub organization: IdName,
    pub role: Role,
    pub name: String,
    pub description: String,
}

/// Port of `utils.SelectRole(organization)`: prints `Roles:` and prompts among the available roles of the
/// organization (`GET /organization/{id}/availableRole`), unsorted, even when there is a single role.
pub fn select_role(organization: &IdName) -> Result<Role, String> {
    let client = get_client()?;
    let roles: Vec<Value> = client
        .list(&format!(
            "/organization/{}/availableRole",
            path_escape(&organization.id)
        ))
        .map_err(|e| e.to_string())?;
    let mut names: Vec<String> = Vec::new();
    let mut ids: HashMap<String, String> = HashMap::new();
    for r in &roles {
        names.push(str_field(r, "name").to_string());
        ids.insert(
            str_field(r, "name").to_string(),
            str_field(r, "id").to_string(),
        );
    }
    if names.is_empty() {
        return Err("no role found".to_string());
    }
    println!("Roles:");
    let selected = prompt_select(&names, None)?;
    Ok(Role {
        id: ids.get(&selected).cloned().unwrap_or_default(),
        name: selected,
    })
}

/// Port of `utils.SelectTokenInformation()`: organization, role (`Info: Select Role`), token name
/// (`Choose a token name`, must not be empty) and description (`Choose a token description`).
pub fn select_token_information() -> Result<TokenInformation, String> {
    let organization = select_organization()?;
    qovery_log::println_info("Select Role");
    let role = select_role(&organization)?;

    println!("Choose a token name");
    let name = prompt_text("Token name")?;
    if name.is_empty() {
        return Err("token name must not be empty".to_string());
    }

    println!("Choose a token description");
    let description = prompt_text("Token description")?;

    Ok(TokenInformation {
        organization,
        role,
        name,
        description,
    })
}
