//! Execution context given to command implementations.

use crate::flag::FlagKind;
use crate::help;
use crate::parse::{FlagValue, parse_go_bool, parse_go_int, parse_go_uint, read_as_csv};
use crate::tree::{NodeId, Tree};
use std::collections::BTreeMap;
use std::io::Write;
use std::time::Duration;

/// What a command implementation receives: its parsed flags and positional args.
pub struct Ctx<'a> {
    pub(crate) tree: &'a Tree,
    pub(crate) id: NodeId,
    pub(crate) args: Vec<String>,
    pub(crate) flags: BTreeMap<String, FlagValue>,
}

impl<'a> Ctx<'a> {
    /// Positional arguments (cobra `args`).
    pub fn args(&self) -> &[String] {
        &self.args
    }

    /// The whole command tree (read only), e.g. for `list-commands` or completion.
    pub fn tree(&self) -> &Tree {
        self.tree
    }

    /// Id of the running command in [`Ctx::tree`].
    pub fn node_id(&self) -> NodeId {
        self.id
    }

    /// cobra `cmd.Name()`.
    pub fn name(&self) -> &str {
        self.tree.node(self.id).name()
    }

    /// cobra `cmd.CommandPath()` (e.g. `qovery application list`).
    pub fn command_path(&self) -> String {
        self.tree.command_path(self.id)
    }

    /// cobra `cmd.UseLine()`.
    pub fn use_line(&self) -> String {
        self.tree.use_line(self.id)
    }

    /// cobra `cmd.Help()`: prints the help of the command on stdout.
    pub fn help(&self) {
        let mut out = std::io::stdout().lock();
        let _ = out.write_all(help::help_string(self.tree, self.id).as_bytes());
        let _ = out.flush();
    }

    /// cobra `cmd.UsageString()`.
    pub fn usage_string(&self) -> String {
        help::usage_string(self.tree, self.id)
    }

    /// cobra `cmd.Usage()`: prints the usage on stderr.
    pub fn usage(&self) {
        let _ = std::io::stderr().write_all(self.usage_string().as_bytes());
    }

    /// Flags not defined on the command read as their zero value (like the Go package-level flag
    /// variables shared between commands, which stay at their zero value when another command runs).
    fn value(&self, name: &str) -> Option<&FlagValue> {
        self.flags.get(name)
    }

    /// Whether the flag was set on the command line (pflag `Flag.Changed`).
    pub fn changed(&self, name: &str) -> bool {
        self.flags.get(name).is_some_and(|v| v.changed)
    }

    /// Whether the flag is defined on this command (`cmd.Flags().Lookup(name) != nil`).
    pub fn has_flag(&self, name: &str) -> bool {
        self.flags.contains_key(name)
    }

    /// Effective raw string value: last value given, or the default (pflag `Flag.Value.String()` for scalars).
    pub fn raw(&self, name: &str) -> String {
        let Some(v) = self.value(name) else {
            return String::new();
        };
        match v.raw.last() {
            Some(s) => s.clone(),
            None => v.flag.default.clone(),
        }
    }

    /// The flag's pflag `DefValue`.
    pub fn default_of(&self, name: &str) -> String {
        self.value(name)
            .map(|v| v.flag.default.clone())
            .unwrap_or_default()
    }

    /// `GetString`.
    pub fn string(&self, name: &str) -> String {
        self.raw(name)
    }

    /// `GetBool`.
    pub fn bool(&self, name: &str) -> bool {
        parse_go_bool(&self.raw(name)).unwrap_or(false)
    }

    /// `GetInt` / `GetInt64` / `GetInt32`.
    pub fn int(&self, name: &str) -> i64 {
        let bits = match self.value(name).map(|v| v.flag.kind) {
            Some(FlagKind::Int32) => 32,
            _ => 64,
        };
        parse_go_int(&self.raw(name), bits).unwrap_or(0)
    }

    /// `GetInt32`.
    pub fn int32(&self, name: &str) -> i32 {
        self.int(name) as i32
    }

    /// `GetUint`.
    pub fn uint(&self, name: &str) -> u64 {
        parse_go_uint(&self.raw(name)).unwrap_or(0)
    }

    /// `GetFloat64`.
    pub fn float64(&self, name: &str) -> f64 {
        self.raw(name).parse().unwrap_or(0.0)
    }

    /// `GetDuration` as signed nanoseconds (Go durations can be negative).
    pub fn duration_nanos(&self, name: &str) -> i64 {
        qovery_log::gofmt::parse_duration_nanos(&self.raw(name)).unwrap_or(0)
    }

    /// `GetDuration` (negative values read as zero: use [`Ctx::duration_nanos`] when the sign matters).
    pub fn duration(&self, name: &str) -> Duration {
        qovery_log::gofmt::parse_duration(&self.raw(name)).unwrap_or_default()
    }

    fn default_list(&self, name: &str) -> Vec<String> {
        let d = self.default_of(name);
        let inner = d
            .strip_prefix('[')
            .and_then(|s| s.strip_suffix(']'))
            .unwrap_or(&d);
        if inner.is_empty() {
            Vec::new()
        } else {
            read_as_csv(inner)
        }
    }

    /// `GetStringSlice` (comma split) / `GetStringArray` (no split).
    pub fn strings(&self, name: &str) -> Vec<String> {
        let Some(v) = self.value(name) else {
            return Vec::new();
        };
        if !v.changed {
            return self.default_list(name);
        }
        match v.flag.kind {
            FlagKind::StringSlice => v.raw.iter().flat_map(|r| read_as_csv(r)).collect(),
            _ => v.raw.clone(),
        }
    }

    /// `GetStringToString`.
    pub fn string_map(&self, name: &str) -> BTreeMap<String, String> {
        let mut out = BTreeMap::new();
        let Some(v) = self.value(name) else {
            return out;
        };
        for r in &v.raw {
            for (k, val) in crate::parse::string_to_string_pairs(r).unwrap_or_default() {
                out.insert(k, val);
            }
        }
        out
    }

    /// `(flag name, effective raw value, changed)` for every flag (useful for telemetry / debugging).
    pub fn all_flags(&self) -> Vec<(String, String, bool)> {
        self.flags
            .keys()
            .map(|k| (k.clone(), self.raw(k), self.changed(k)))
            .collect()
    }
}
