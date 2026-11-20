//! API models of the variable endpoints (`qovery-client-go` `model_variable_*.go`) and enums.

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use std::fmt;

/// Go `utils.Var` (`{Key, Value}` pair).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Var {
    pub key: String,
    pub value: String,
}

/// Go `qovery.APIVariableScopeEnum`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum VariableScope {
    Application,
    BuiltIn,
    Environment,
    Project,
    Container,
    Job,
    Helm,
    Terraform,
    AgenticWorkflow,
}

impl VariableScope {
    /// The API value (`APPLICATION`, `BUILT_IN`, ...).
    pub fn as_str(&self) -> &'static str {
        match self {
            VariableScope::Application => "APPLICATION",
            VariableScope::BuiltIn => "BUILT_IN",
            VariableScope::Environment => "ENVIRONMENT",
            VariableScope::Project => "PROJECT",
            VariableScope::Container => "CONTAINER",
            VariableScope::Job => "JOB",
            VariableScope::Helm => "HELM",
            VariableScope::Terraform => "TERRAFORM",
            VariableScope::AgenticWorkflow => "AGENTIC_WORKFLOW",
        }
    }
}

impl fmt::Display for VariableScope {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

impl Serialize for VariableScope {
    fn serialize<S: serde::Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        s.serialize_str(self.as_str())
    }
}

/// Go `qovery.VariableAlias` / `qovery.VariableOverride` (`aliased_variable` / `overridden_variable` of a
/// [`VariableResponse`]). Both models have the same fields.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct VariableAlias {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub key: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<String>,
    #[serde(default)]
    pub mount_path: String,
    #[serde(default)]
    pub scope: String,
    #[serde(default)]
    pub variable_type: String,
    #[serde(flatten)]
    pub extra: Map<String, Value>,
}

/// Go `qovery.VariableOverride` (same shape as [`VariableAlias`]).
pub type VariableOverride = VariableAlias;

/// Go `qovery.VariableResponse` (items of `GET /variable`).
///
/// Decoding is lenient (missing fields get their zero value). Timestamps are kept as the RFC 3339 strings
/// returned by the API. Unknown fields are kept in `extra`.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct VariableResponse {
    #[serde(default)]
    pub id: String,
    /// RFC 3339 (Go `time.Time`)
    #[serde(default)]
    pub created_at: String,
    /// RFC 3339 (Go `*time.Time`)
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<String>,
    #[serde(default)]
    pub key: String,
    /// `null` for secrets (Go `NullableString`)
    #[serde(default)]
    pub value: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mount_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub overridden_variable: Option<VariableOverride>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub aliased_variable: Option<VariableAlias>,
    /// `APIVariableScopeEnum` value (`APPLICATION`, `PROJECT`, ...)
    #[serde(default)]
    pub scope: String,
    /// `APIVariableTypeEnum` value (`VALUE`, `ALIAS`, `OVERRIDE`, `FILE`, `BUILT_IN`)
    #[serde(default)]
    pub variable_type: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub service_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub service_name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub service_type: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub owned_by: Option<String>,
    #[serde(default)]
    pub is_secret: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub secret_manager_access_id: Option<String>,
    #[serde(flatten)]
    pub extra: Map<String, Value>,
}
