//! Port of `utils/context.go`: the `~/.qovery/context.json` file.

use serde::{Deserialize, Serialize};
use std::fmt;
use std::path::PathBuf;

/// Go's zero `time.Time` as marshalled in JSON.
pub const ZERO_TIME: &str = "0001-01-01T00:00:00Z";

fn zero_time() -> String {
    ZERO_TIME.to_string()
}

/// `utils.QoveryContext` (same JSON field names and order as the Go struct).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QoveryContext {
    #[serde(default)]
    pub access_token: String,
    /// RFC 3339 (Go `time.Time` JSON encoding). [`get_current_context`] rejects a value Go cannot parse.
    #[serde(default = "zero_time")]
    pub access_token_expiration: String,
    #[serde(default)]
    pub refresh_token: String,
    #[serde(default)]
    pub organization_id: String,
    #[serde(default)]
    pub organization_name: String,
    #[serde(default)]
    pub project_id: String,
    #[serde(default)]
    pub project_name: String,
    #[serde(default)]
    pub environment_id: String,
    #[serde(default)]
    pub environment_name: String,
    #[serde(default)]
    pub service_id: String,
    #[serde(default)]
    pub service_name: String,
    #[serde(default)]
    pub service_type: String,
    #[serde(default)]
    pub user: String,
}

impl Default for QoveryContext {
    fn default() -> Self {
        QoveryContext {
            access_token: String::new(),
            access_token_expiration: zero_time(),
            refresh_token: String::new(),
            organization_id: String::new(),
            organization_name: String::new(),
            project_id: String::new(),
            project_name: String::new(),
            environment_id: String::new(),
            environment_name: String::new(),
            service_id: String::new(),
            service_name: String::new(),
            service_type: String::new(),
            user: String::new(),
        }
    }
}

impl QoveryContext {
    /// `AccessTokenExpiration` as Go decodes it (`time.Parse(time.RFC3339)` rules, which accept more than
    /// chrono's RFC 3339: one-digit hours, `,` fraction separator...).
    pub fn access_token_expiration_time(&self) -> Option<chrono::DateTime<chrono::Utc>> {
        let (secs, nsec) = qovery_log::gotime::parse_rfc3339(&self.access_token_expiration).ok()?;
        chrono::DateTime::from_timestamp(secs, nsec)
    }

    /// `isMinimalContextValid`.
    pub fn is_minimal_valid(&self) -> bool {
        !self.access_token.is_empty()
            && self
                .access_token_expiration_time()
                .is_some_and(|t| t > chrono::Utc::now())
            && !self.refresh_token.is_empty()
            && !self.organization_id.is_empty()
    }
}

/// Service kinds stored in the context (`utils.ServiceType`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ServiceType {
    Application,
    Container,
    Database,
    Job,
    Helm,
    Terraform,
}

impl ServiceType {
    pub fn as_str(&self) -> &'static str {
        match self {
            ServiceType::Application => "application",
            ServiceType::Container => "container",
            ServiceType::Database => "database",
            ServiceType::Job => "job",
            ServiceType::Helm => "helm",
            ServiceType::Terraform => "terraform",
        }
    }
    pub fn parse(s: &str) -> Option<ServiceType> {
        Some(match s {
            "application" => ServiceType::Application,
            "container" => ServiceType::Container,
            "database" => ServiceType::Database,
            "job" => ServiceType::Job,
            "helm" => ServiceType::Helm,
            "terraform" => ServiceType::Terraform,
            _ => return None,
        })
    }
}

impl fmt::Display for ServiceType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// `utils.Service`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Service {
    pub id: String,
    pub name: String,
    /// raw service type string (`application`, `container`, ...); may be empty
    pub service_type: String,
    /// Set for a terraform service created by a blueprint
    pub blueprint_id: String,
}

/// Simple `{ID, Name}` pairs (`utils.Organization`, `utils.Project`, ...).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct IdName {
    pub id: String,
    pub name: String,
}

/// `utils.Environment`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct EnvironmentRef {
    pub id: String,
    pub cluster_id: String,
    pub name: String,
}

const CONTEXT_FILE_NAME: &str = "context";

/// `utils.QoveryDirPath()`: `os.UserHomeDir() + "/.qovery"` (string concatenation, like Go).
pub fn qovery_dir_path() -> Result<PathBuf, String> {
    let mut dir = crate::gosys::user_home_dir()?.into_os_string();
    dir.push(std::path::MAIN_SEPARATOR_STR);
    dir.push(".qovery");
    Ok(dir.into())
}

/// `utils.QoveryContextPath()`: `$HOME/.qovery/context.json`.
pub fn qovery_context_path() -> Result<PathBuf, String> {
    let mut path = qovery_dir_path()?.into_os_string();
    path.push(std::path::MAIN_SEPARATOR_STR);
    path.push(format!("{CONTEXT_FILE_NAME}.json"));
    Ok(path.into())
}

pub fn qovery_dir_exists() -> bool {
    match qovery_dir_path() {
        Ok(p) => p.exists(),
        Err(e) => {
            qovery_log::println_error(e);
            crate::exit(0)
        }
    }
}

pub fn qovery_context_exists() -> bool {
    match qovery_context_path() {
        Ok(p) => p.exists(),
        Err(e) => {
            qovery_log::println_error(e);
            crate::exit(0)
        }
    }
}

#[cfg(unix)]
fn chmod_600(path: &std::path::Path) -> std::io::Result<()> {
    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o600))
}
#[cfg(not(unix))]
fn chmod_600(_path: &std::path::Path) -> std::io::Result<()> {
    Ok(())
}

/// `utils.InitializeQoveryContext()`: create `~/.qovery/context.json` containing `{}`.
pub fn initialize_qovery_context() -> Result<(), String> {
    if !qovery_dir_exists() {
        let p = qovery_dir_path()?;
        std::fs::create_dir(&p).map_err(|e| go_path_error("mkdir", &p, &e))?;
    }
    let path = qovery_context_path()?;
    std::fs::write(&path, b"{}").map_err(|e| go_path_error("open", &path, &e))?;
    chmod_600(&path).map_err(|e| go_path_error("chmod", &path, &e))?;
    Ok(())
}

/// Format an IO error like Go's `*fs.PathError`: `open /path: no such file or directory`.
pub fn go_path_error(op: &str, path: &std::path::Path, e: &std::io::Error) -> String {
    crate::gosys::path_error(op, path, e)
}

/// `utils.GetCurrentContext()`.
pub fn get_current_context() -> Result<QoveryContext, String> {
    let path = qovery_context_path()?;
    let bytes = std::fs::read(&path).map_err(|e| go_path_error("open", &path, &e))?;
    decode_context(&bytes)
}

/// `json.Unmarshal(bytes, &QoveryContext{})` as the Go CLI does it (`jsonv2` experiment, legacy semantics):
/// Go's syntax errors, `null` leaves the zero value (top level included), keys matched case-insensitively,
/// type errors (`json: cannot unmarshal number into Go struct field QoveryContext.user of type utils.Name`,
/// with the key as written) and a malformed expiration (`time.Parse(time.RFC3339)` error).
///
/// Decoded through `serde_json::Value`, whose objects are sorted and deduplicated: with several errors, or a
/// key repeated with different cases, Go (input order) may pick another one.
fn decode_context(bytes: &[u8]) -> Result<QoveryContext, String> {
    use serde_json::{Map, Value};

    /// JSON field name and Go type of each `QoveryContext` field.
    const FIELDS: [(&str, &str); 13] = [
        ("access_token", "utils.AccessToken"),
        ("access_token_expiration", "time.Time"),
        ("refresh_token", "utils.RefreshToken"),
        ("organization_id", "utils.Id"),
        ("organization_name", "utils.Name"),
        ("project_id", "utils.Id"),
        ("project_name", "utils.Name"),
        ("environment_id", "utils.Id"),
        ("environment_name", "utils.Name"),
        ("service_id", "utils.Id"),
        ("service_name", "utils.Name"),
        ("service_type", "utils.ServiceType"),
        ("user", "utils.Name"),
    ];

    /// `strings.EqualFold` against an ASCII field name: the only non-ASCII runes folding to ASCII letters are
    /// the Kelvin sign (`k`) and the long s (`s`).
    fn fold_eq(field: &str, key: &str) -> bool {
        let fold = |c: char| match c {
            '\u{212A}' => 'k',
            '\u{17F}' => 's',
            c => c.to_ascii_lowercase(),
        };
        field.chars().map(fold).eq(key.chars().map(fold))
    }

    fn kind(v: &Value) -> &'static str {
        match v {
            Value::Null => "null",
            Value::Bool(_) => "bool",
            Value::Number(_) => "number",
            Value::String(_) => "string",
            Value::Array(_) => "array",
            Value::Object(_) => "object",
        }
    }

    qovery_log::gojson::go_json_check_valid(bytes)?;
    let object = match serde_json::from_slice::<Value>(bytes).map_err(|e| e.to_string())? {
        Value::Null => return Ok(QoveryContext::default()),
        Value::Object(o) => o,
        other => {
            return Err(format!(
                "json: cannot unmarshal {} into Go value of type utils.QoveryContext",
                kind(&other)
            ));
        }
    };
    // Keep one value per field (the exact key first), under its canonical name, for the derived decoder.
    let mut fields = Map::new();
    for (name, go_type) in FIELDS {
        let entry = object
            .get_key_value(name)
            .or_else(|| object.iter().find(|(k, _)| fold_eq(name, k)));
        let Some((key, value)) = entry else { continue };
        match value {
            Value::Null => {}
            Value::String(s) => {
                if go_type == "time.Time" {
                    qovery_log::gotime::parse_rfc3339(s)?;
                }
                fields.insert(name.to_string(), value.clone());
            }
            other => {
                return Err(format!(
                    "json: cannot unmarshal {} into Go struct field QoveryContext.{key} of type {go_type}",
                    kind(other)
                ));
            }
        }
    }
    serde_json::from_value(Value::Object(fields)).map_err(|e| e.to_string())
}

/// `utils.StoreContext(ctx)`.
pub fn store_context(ctx: &QoveryContext) -> Result<(), String> {
    let bytes = serde_json::to_vec(ctx).map_err(|e| e.to_string())?;
    let path = qovery_context_path()?;
    chmod_600(&path).map_err(|e| go_path_error("chmod", &path, &e))?;
    std::fs::write(&path, bytes).map_err(|e| go_path_error("open", &path, &e))
}

const NOT_SELECTED: &str =
    "has not been selected. Please, use 'qovery context set' to set up Qovery context. ";

/// `utils.CurrentOrganization(promptContext)`.
pub fn current_organization(prompt_context: bool) -> Result<IdName, String> {
    let mut r = get_current_context();
    if prompt_context
        && r.as_ref()
            .map(|c| c.organization_id.is_empty())
            .unwrap_or(true)
    {
        r = get_or_set_current_context(false, false, false);
    }
    let ctx = r?;
    if ctx.organization_id.is_empty() || ctx.organization_name.is_empty() {
        return Err(format!("current organization {NOT_SELECTED}"));
    }
    Ok(IdName {
        id: ctx.organization_id,
        name: ctx.organization_name,
    })
}

/// `utils.CurrentProject(promptContext)`.
pub fn current_project(prompt_context: bool) -> Result<IdName, String> {
    let mut r = get_current_context();
    if prompt_context && r.as_ref().map(|c| c.project_id.is_empty()).unwrap_or(true) {
        r = get_or_set_current_context(true, false, false);
    }
    let ctx = r?;
    if ctx.project_id.is_empty() || ctx.project_name.is_empty() {
        return Err(format!("current project {NOT_SELECTED}"));
    }
    Ok(IdName {
        id: ctx.project_id,
        name: ctx.project_name,
    })
}

/// `utils.CurrentEnvironment(promptContext)`.
pub fn current_environment(prompt_context: bool) -> Result<IdName, String> {
    let mut r = get_current_context();
    if prompt_context
        && r.as_ref()
            .map(|c| c.environment_id.is_empty())
            .unwrap_or(true)
    {
        r = get_or_set_current_context(true, true, false);
    }
    let ctx = r?;
    if ctx.environment_id.is_empty() || ctx.environment_name.is_empty() {
        return Err(format!("current environment {NOT_SELECTED}"));
    }
    Ok(IdName {
        id: ctx.environment_id,
        name: ctx.environment_name,
    })
}

/// `utils.CurrentService(promptContext)`.
pub fn current_service(prompt_context: bool) -> Result<Service, String> {
    let mut r = get_current_context();
    if prompt_context && r.as_ref().map(|c| c.service_id.is_empty()).unwrap_or(true) {
        r = get_or_set_current_context(true, true, true);
    }
    let ctx = r?;
    if ctx.service_id.is_empty() || ctx.service_name.is_empty() {
        return Err(format!("current service {NOT_SELECTED}"));
    }
    Ok(Service {
        id: ctx.service_id,
        name: ctx.service_name,
        service_type: ctx.service_type,
        blueprint_id: String::new(),
    })
}

pub fn set_organization(org: &IdName) -> Result<(), String> {
    let mut ctx = get_current_context()?;
    ctx.organization_name = org.name.clone();
    ctx.organization_id = org.id.clone();
    store_context(&ctx)
}

pub fn set_project(project: &IdName) -> Result<(), String> {
    let mut ctx = get_current_context()?;
    ctx.project_name = project.name.clone();
    ctx.project_id = project.id.clone();
    store_context(&ctx)
}

pub fn set_environment(env: &EnvironmentRef) -> Result<(), String> {
    let mut ctx = get_current_context()?;
    ctx.environment_name = env.name.clone();
    ctx.environment_id = env.id.clone();
    store_context(&ctx)
}

pub fn set_service(service: &Service) -> Result<(), String> {
    let mut ctx = get_current_context()?;
    ctx.service_name = service.name.clone();
    ctx.service_id = service.id.clone();
    ctx.service_type = service.service_type.clone();
    store_context(&ctx)
}

/// `utils.ResetApplicationContext()`.
pub fn reset_application_context() -> Result<(), String> {
    let mut ctx = get_current_context()?;
    ctx.organization_name.clear();
    ctx.organization_id.clear();
    ctx.project_name.clear();
    ctx.project_id.clear();
    ctx.environment_name.clear();
    ctx.environment_id.clear();
    ctx.service_name.clear();
    ctx.service_id.clear();
    ctx.service_type = ServiceType::Application.as_str().to_string();
    store_context(&ctx)
}

/// `utils.GetOrSetCurrentContext(setProject, setEnvironment, setService)`.
pub fn get_or_set_current_context(
    set_project: bool,
    set_environment: bool,
    set_service: bool,
) -> Result<QoveryContext, String> {
    let ctx = get_current_context().unwrap_or_default();
    if ctx.is_minimal_valid()
        && (!set_project || !ctx.project_id.is_empty())
        && (!set_environment || !ctx.environment_id.is_empty())
        && (!set_service || !ctx.service_id.is_empty())
    {
        return Ok(ctx);
    }
    set_context(set_project, set_environment, set_service, false)?;
    get_current_context()
}

/// `utils.SetContext(...)`.
pub fn set_context(
    set_project: bool,
    set_environment: bool,
    set_service: bool,
    print_final_context: bool,
) -> Result<(), String> {
    let _ = print_context();
    let _ = reset_application_context();
    let org = crate::select::select_and_set_organization()?;
    if !set_project {
        return Ok(());
    }
    let project = crate::select::select_and_set_project(&org.id)?;
    if !set_environment {
        return Ok(());
    }
    let env = crate::select::select_and_set_environment(&project.id)?;
    if !set_service {
        return Ok(());
    }
    crate::select::select_and_set_service(&env.id)?;
    let _ = current_service(false);
    if print_final_context {
        qovery_log::go_builtin_println("");
        if let Err(e) = print_context() {
            qovery_log::println_error(e);
        }
        qovery_log::go_builtin_println("");
    }
    Ok(())
}

/// `utils.PrintContext()`.
pub fn print_context() -> Result<(), String> {
    let org = current_organization(false)?;
    let project = current_project(false)?;
    let env = current_environment(false)?;
    let srv = current_service(false)?;
    qovery_log::table::Table::new(vec![
        vec!["Organization".into(), org.name],
        vec!["Project".into(), project.name],
        vec!["Environment".into(), env.name],
        vec!["Service".into(), srv.name],
        vec!["Type".into(), srv.service_type],
    ])
    .render();
    Ok(())
}
