//! Port of `pkg/enterpriseconnection/enterprise_connection_service.go`.

use qovery_api::Client;
use qovery_core::{auth, check_error, client, resources};
use qovery_log::pterm::{self, Section};
use serde_json::{Map, Value};
use std::collections::HashMap;

/// `qovery.EnterpriseConnectionDto`. `group_mappings` keeps the raw JSON values (arrays of strings, or
/// `null` for a nil Go slice) so that a re-serialization is identical to Go.
#[derive(Debug, Clone, Default)]
pub struct EnterpriseConnection {
    pub connection_name: String,
    pub default_role: String,
    pub enforce_group_sync: bool,
    pub group_mappings: Map<String, Value>,
}

impl EnterpriseConnection {
    pub fn from_value(v: &Value) -> Self {
        EnterpriseConnection {
            connection_name: v["connection_name"]
                .as_str()
                .unwrap_or_default()
                .to_string(),
            default_role: v["default_role"].as_str().unwrap_or_default().to_string(),
            enforce_group_sync: v["enforce_group_sync"].as_bool().unwrap_or(false),
            group_mappings: v["group_mappings"].as_object().cloned().unwrap_or_default(),
        }
    }

    /// Go `json.Marshal` of the DTO (`ToMap`, sorted keys).
    pub fn to_value(&self) -> Value {
        let mut m = Map::new();
        m.insert(
            "connection_name".into(),
            Value::String(self.connection_name.clone()),
        );
        m.insert(
            "default_role".into(),
            Value::String(self.default_role.clone()),
        );
        m.insert(
            "enforce_group_sync".into(),
            Value::Bool(self.enforce_group_sync),
        );
        m.insert(
            "group_mappings".into(),
            Value::Object(self.group_mappings.clone()),
        );
        Value::Object(m)
    }
}

/// `enterpriseconnection.CreateConnectionUpdateDto`.
pub fn create_connection_update_dto(
    default_role: &str,
    enforce_group_sync: bool,
    group_mappings: Map<String, Value>,
) -> EnterpriseConnection {
    EnterpriseConnection {
        connection_name: String::new(),
        default_role: default_role.to_string(),
        enforce_group_sync,
        group_mappings,
    }
}

/// `enterpriseconnection.ParseIdpGroupNames`: `[]` for an empty input, `null` (nil slice) when every part is
/// blank, the trimmed non-empty parts otherwise.
pub fn parse_idp_group_names(idp_group_names: &str) -> Value {
    if idp_group_names.is_empty() {
        return Value::Array(vec![]);
    }
    let result: Vec<Value> = idp_group_names
        .split(',')
        .map(|p| p.trim())
        .filter(|p| !p.is_empty())
        .map(|p| Value::String(p.to_string()))
        .collect();
    if result.is_empty() {
        Value::Null
    } else {
        Value::Array(result)
    }
}

pub use qovery_api::path_escape;

/// `uuid.Validate` of `github.com/google/uuid`.
pub fn is_valid_uuid(s: &str) -> bool {
    fn hex(s: &str) -> bool {
        s.bytes().all(|b| b.is_ascii_hexdigit())
    }
    fn canonical(s: &str) -> bool {
        let b = s.as_bytes();
        b.len() == 36
            && b[8] == b'-'
            && b[13] == b'-'
            && b[18] == b'-'
            && b[23] == b'-'
            && [&s[0..8], &s[9..13], &s[14..18], &s[19..23], &s[24..36]]
                .iter()
                .all(|p| hex(p))
    }
    match s.len() {
        36 => canonical(s),
        38 => s.starts_with('{') && s.ends_with('}') && canonical(&s[1..37]),
        45 => {
            s.is_char_boundary(9) && s[..9].eq_ignore_ascii_case("urn:uuid:") && canonical(&s[9..])
        }
        32 => hex(s),
        _ => false,
    }
}

/// pterm `SectionPrinter.Println(text)`: `Sprint(text + "\n")`, without any other newline.
/// (`qovery_log::pterm::Section::println` appends an extra newline, which pterm does not.)
fn section_println(section: &Section, text: &str) {
    section.print(&format!("{text}\n"));
}

/// `enterpriseconnection.EnterpriseConnectionService`.
pub struct EnterpriseConnectionService {
    pub client: Client,
    pub organization_id: String,
    /// roleName (lowercase) -> roleId
    pub available_roles_by_name: HashMap<String, String>,
    /// roleId -> roleName
    pub custom_role_names_by_id: HashMap<String, String>,
    /// roleName (lowercase) -> roleId
    pub custom_role_ids_by_name: HashMap<String, String>,
}

fn str_of(v: &Value) -> String {
    v.as_str().unwrap_or_default().to_string()
}

impl EnterpriseConnectionService {
    /// `NewEnterpriseConnectionService(organizationName)`.
    pub fn new(organization_name: &str) -> Result<Self, String> {
        let (token_type, token) = auth::get_access_token(false)?;
        let client = client::client_for(token_type, &token);
        let organization_id = resources::organization_id(&client, organization_name)?;
        let mut s = EnterpriseConnectionService {
            client,
            organization_id,
            available_roles_by_name: HashMap::new(),
            custom_role_names_by_id: HashMap::new(),
            custom_role_ids_by_name: HashMap::new(),
        };
        s.initialize_role_mappings()?;
        Ok(s)
    }

    fn initialize_role_mappings(&mut self) -> Result<(), String> {
        let org = &self.organization_id;
        let available: Vec<Value> = self
            .client
            .list(&format!("/organization/{}/availableRole", path_escape(org)))
            .map_err(|e| e.to_string())?;
        for role in &available {
            self.available_roles_by_name
                .insert(str_of(&role["name"]).to_lowercase(), str_of(&role["id"]));
        }
        let custom: Vec<Value> = self
            .client
            .list(&format!("/organization/{}/customRole", path_escape(org)))
            .map_err(|e| e.to_string())?;
        for role in &custom {
            let (id, name) = (str_of(&role["id"]), str_of(&role["name"]));
            self.custom_role_names_by_id
                .insert(id.clone(), name.clone());
            self.custom_role_ids_by_name.insert(name.to_lowercase(), id);
        }
        Ok(())
    }

    fn connection_path(&self, connection_name: &str) -> String {
        format!(
            "/organization/{}/enterpriseconnection/{}",
            path_escape(&self.organization_id),
            path_escape(connection_name)
        )
    }

    /// `ListEnterpriseConnections(connectionName)`: API errors print and exit 1 (`utils.CheckError`).
    pub fn list_enterprise_connections(&self, connection_name: &str) -> Vec<EnterpriseConnection> {
        if connection_name.is_empty() {
            let list: Vec<Value> = check_error(self.client.list(&format!(
                "/organization/{}/enterpriseconnection",
                path_escape(&self.organization_id)
            )));
            return list.iter().map(EnterpriseConnection::from_value).collect();
        }
        vec![check_error(self.get_enterprise_connection(connection_name))]
    }

    /// `GetEnterpriseConnection(connectionName)`.
    pub fn get_enterprise_connection(
        &self,
        connection_name: &str,
    ) -> Result<EnterpriseConnection, String> {
        let v: Value = self
            .client
            .get(&self.connection_path(connection_name))
            .map_err(|e| e.to_string())?;
        Ok(EnterpriseConnection::from_value(&v))
    }

    /// `UpdateEnterpriseConnection(connectionName, dto)`.
    pub fn update_enterprise_connection(
        &self,
        connection_name: &str,
        dto: &EnterpriseConnection,
    ) -> Result<EnterpriseConnection, String> {
        let v: Value = self
            .client
            .put(&self.connection_path(connection_name), &dto.to_value())
            .map_err(|e| e.to_string())?;
        Ok(EnterpriseConnection::from_value(&v))
    }

    /// `ResolveRoleDisplayName`: custom role UUID -> its name, anything else as is.
    pub fn resolve_role_display_name(&self, role_id_or_name: &str) -> String {
        if is_valid_uuid(role_id_or_name)
            && let Some(name) = self.custom_role_names_by_id.get(role_id_or_name)
        {
            return name.clone();
        }
        role_id_or_name.to_string()
    }

    /// `ResolveProvidedRoleNameOrCustomRoleId`.
    pub fn resolve_provided_role_name_or_custom_role_id(
        &self,
        role_name: &str,
    ) -> Result<String, String> {
        let lower = role_name.to_lowercase();
        if let Some(id) = self.custom_role_ids_by_name.get(&lower) {
            return Ok(id.clone());
        }
        if self.available_roles_by_name.contains_key(&lower) {
            return Ok(lower);
        }
        Err(format!("role '{role_name}' not found"))
    }

    /// `ValidateRole`.
    pub fn validate_role(&self, role_name: &str) -> Result<(), String> {
        self.resolve_provided_role_name_or_custom_role_id(role_name)
            .map(|_| ())
    }

    /// `DisplayEnterpriseConnection`.
    pub fn display_enterprise_connection(&self, connection: &EnterpriseConnection) {
        Section::default().printfln(&format!("Connection Name: {}", connection.connection_name));
        let default_role_display = self.resolve_role_display_name(&connection.default_role);
        let enforce_sync_display = if connection.enforce_group_sync {
            pterm::FG_GREEN.sprint("✓ true")
        } else {
            pterm::FG_RED.sprint("✗ false")
        };
        section_println(
            &Section::default()
                .with_top_padding(0)
                .with_bottom_padding(0),
            "Connection Settings",
        );
        qovery_log::print_table(
            &["Default Role", "Enforce Sync Group"],
            &[vec![default_role_display, enforce_sync_display]],
        );
        section_println(
            &Section::default()
                .with_top_padding(0)
                .with_bottom_padding(0),
            "Group Mappings",
        );
        self.display_group_mappings_table(&connection.group_mappings);
    }

    /// `DisplayGroupMappingsTable`.
    pub fn display_group_mappings_table(&self, group_mappings: &Map<String, Value>) {
        if group_mappings.is_empty() {
            pterm::info_printer().println("No group mappings configured");
            return;
        }
        let sep = pterm::gray(" ; ");
        let mut data: Vec<Vec<String>> = group_mappings
            .iter()
            .map(|(role, groups)| {
                let groups: Vec<String> = groups
                    .as_array()
                    .map(|a| a.iter().map(str_of).collect())
                    .unwrap_or_default();
                vec![self.resolve_role_display_name(role), groups.join(&sep)]
            })
            .collect();
        data.sort_by(|a, b| a[0].cmp(&b[0]));
        qovery_log::print_table(&["Qovery Role", "Your IDP Groups"], &data);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn idp_group_names() {
        assert_eq!(parse_idp_group_names(""), serde_json::json!([]));
        assert_eq!(parse_idp_group_names(" , ,"), Value::Null);
        assert_eq!(
            parse_idp_group_names("a, b ,,c"),
            serde_json::json!(["a", "b", "c"])
        );
    }

    #[test]
    fn uuid_validation() {
        assert!(is_valid_uuid("0b5b8a3e-6c1f-4b5e-9a3e-1f2b3c4d5e6f"));
        assert!(is_valid_uuid("{0b5b8a3e-6c1f-4b5e-9a3e-1f2b3c4d5e6f}"));
        assert!(is_valid_uuid(
            "urn:uuid:0b5b8a3e-6c1f-4b5e-9a3e-1f2b3c4d5e6f"
        ));
        assert!(is_valid_uuid("0b5b8a3e6c1f4b5e9a3e1f2b3c4d5e6f"));
        assert!(!is_valid_uuid("viewer"));
        assert!(!is_valid_uuid("0b5b8a3e-6c1f-4b5e-9a3e-1f2b3c4d5e6g"));
    }

    #[test]
    fn path_escaping() {
        assert_eq!(path_escape("my conn/x?y"), "my%20conn%2Fx%3Fy");
        assert_eq!(path_escape("a-b_c.d~$&+:=@"), "a-b_c.d~$&+:=@");
    }
}
