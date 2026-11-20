//! Table rendering of variables: `utils.EnvVarLines` / `utils.EnvVarLineOutput`.

use crate::model::VariableResponse;
use chrono::{DateTime, FixedOffset};
use std::collections::HashMap;

/// Go `utils.EnvVarLineOutput`: one variable as displayed by the `env list` commands.
///
/// Timestamps are the RFC 3339 strings returned by the API (Go keeps a `time.Time` with the parsed offset).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct EnvVarLineOutput {
    pub id: String,
    pub key: String,
    pub value: Option<String>,
    /// RFC 3339 (Go `CreatedAt time.Time`)
    pub created_at: String,
    /// RFC 3339 (Go `UpdatedAt *time.Time`)
    pub updated_at: Option<String>,
    pub service: Option<String>,
    pub scope: String,
    pub is_secret: bool,
    pub alias_parent_key: Option<String>,
    pub override_parent_key: Option<String>,
}

impl EnvVarLineOutput {
    /// Go `EnvVarLineOutput.Data(showValues)`: `[key, type, parent key, value, updated at (RFC822), service, scope]`.
    ///
    /// Like Go (nil pointer dereference of `UpdatedAt`), a variable without `updated_at` makes the program
    /// panic: see [`go_nil_pointer_panic`].
    pub fn data(&self, show_values: bool) -> Vec<String> {
        let service = self.service.clone().unwrap_or_else(|| "N/A".to_string());

        let mut value = "********".to_string();
        if show_values
            && let Some(v) = &self.value
            && !self.is_secret
        {
            value = v.clone();
        }

        let mut key_type = if self.is_secret {
            "Secret".to_string()
        } else {
            "Variable".to_string()
        };

        let mut parent_key = "N/A".to_string();
        if let Some(k) = &self.alias_parent_key {
            parent_key = k.clone();
            key_type += " Alias";
        }
        if let Some(k) = &self.override_parent_key {
            parent_key = k.clone();
            key_type += " Override";
        }

        let updated_at = match &self.updated_at {
            Some(t) => format_rfc822(t),
            None => go_nil_pointer_panic(),
        };

        vec![
            self.key.clone(),
            key_type,
            parent_key,
            value,
            updated_at,
            service,
            self.scope.clone(),
        ]
    }
}

/// Go `utils.FromEnvironmentVariableToEnvVarLineOutput(envVar)`.
pub fn from_environment_variable_to_env_var_line_output(
    env_var: &VariableResponse,
) -> EnvVarLineOutput {
    EnvVarLineOutput {
        id: env_var.id.clone(),
        key: env_var.key.clone(),
        value: env_var.value.clone(),
        created_at: env_var.created_at.clone(),
        updated_at: env_var.updated_at.clone(),
        service: env_var.service_name.clone(),
        scope: env_var.scope.clone(),
        is_secret: env_var.is_secret,
        alias_parent_key: env_var.aliased_variable.as_ref().map(|a| a.key.clone()),
        override_parent_key: env_var.overridden_variable.as_ref().map(|o| o.key.clone()),
    }
}

/// Go `utils.EnvVarLines`: variables grouped by key, aliases and overrides under their parent key.
///
/// Go iterates over a map when `sortKeys` is false (random order); this port uses the insertion order of
/// the groups instead.
#[derive(Debug, Clone, Default)]
pub struct EnvVarLines {
    order: Vec<String>,
    lines: HashMap<String, Vec<EnvVarLineOutput>>,
}

impl EnvVarLines {
    /// Go `utils.NewEnvVarLines()`.
    pub fn new() -> Self {
        Self::default()
    }

    /// Go `EnvVarLines.Add(env)`: aliases / overrides are appended to the group of their parent key, any other
    /// variable **replaces** the group of its key (dropping aliases / overrides added before it, like Go).
    pub fn add(&mut self, env: EnvVarLineOutput) {
        let parent_key = env
            .alias_parent_key
            .clone()
            .or_else(|| env.override_parent_key.clone());
        let key = match parent_key {
            Some(k) => {
                if !self.lines.contains_key(&k) {
                    self.order.push(k.clone());
                }
                self.lines.entry(k).or_default().push(env);
                return;
            }
            None => env.key.clone(),
        };
        if !self.lines.contains_key(&key) {
            self.order.push(key.clone());
        }
        self.lines.insert(key, vec![env]);
    }

    /// Go `EnvVarLines.Header(prettyPrint)`.
    pub fn header(&self, pretty_print: bool) -> Vec<&'static str> {
        if pretty_print {
            return vec!["Key", "Type", "Value", "Updated at", "Service", "Scope"];
        }
        vec![
            "Key",
            "Type",
            "Parent Key",
            "Value",
            "Updated at",
            "Service",
            "Scope",
        ]
    }

    /// Go `EnvVarLines.Lines(showValues, prettyPrint, sortKeys)`: the table rows. With `pretty_print`, the
    /// `Parent Key` column is removed and the children of a group are prefixed with `└── `.
    pub fn lines(
        &self,
        show_values: bool,
        pretty_print: bool,
        sort_keys: bool,
    ) -> Vec<Vec<String>> {
        let mut keys: Vec<&String> = self.order.iter().collect();
        if sort_keys {
            keys.sort();
        }
        let mut lines = Vec::new();
        for key in keys {
            for (idx, env_var) in self.lines[key].iter().enumerate() {
                let mut x = env_var.data(show_values);
                if idx == 0 || !pretty_print {
                    if pretty_print {
                        lines.push(without_parent_key(x));
                    } else {
                        lines.push(x);
                    }
                } else {
                    x[0] = format!("└── {}", x[0]);
                    lines.push(without_parent_key(x));
                }
            }
        }
        lines
    }
}

fn without_parent_key(x: Vec<String>) -> Vec<String> {
    let mut it = x.into_iter();
    let k = it.next().unwrap_or_default();
    let t = it.next().unwrap_or_default();
    let _parent = it.next();
    let mut out = vec![k, t];
    out.extend(it);
    out
}

pub use qovery_core::gosys::go_nil_pointer_panic;

pub(crate) use qovery_core::gosys::go_panic;

/// Parse an API timestamp. `Z` and `+00:00` both become UTC like Go's `time.Parse` with `TZ=UTC`.
fn parse_time(s: &str) -> Option<DateTime<FixedOffset>> {
    DateTime::parse_from_rfc3339(s).ok()
}

/// Go `t.Format(time.RFC822)` (`02 Jan 06 15:04 MST`) of a timestamp decoded from the API.
///
/// The zone abbreviation is `UTC` for a zero offset and the numeric offset (`+0200`) otherwise, which is what
/// Go prints when the local time zone is UTC (the time zone used by the CLI in containers / CI).
/// Unparseable input is returned unchanged.
pub fn format_rfc822(s: &str) -> String {
    parse_time(s)
        .map(|t| qovery_log::gofmt::time_rfc822(&t))
        .unwrap_or_else(|| s.to_string())
}

/// Go `utils.ToIso8601(&t)`: `t.Format("2006-01-02T15:04:05.000Z")` (wall clock of the parsed offset, the `Z`
/// is a literal, milliseconds truncated). Unparseable input is returned unchanged.
pub fn to_iso8601(s: &str) -> String {
    parse_time(s)
        .map(|t| qovery_log::gofmt::time_iso8601(&t))
        .unwrap_or_else(|| s.to_string())
}
