//! pflag compatible flag declarations.

/// pflag value types. The string returned by [`FlagKind::type_name`] is pflag's `Value.Type()`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum FlagKind {
    Bool,
    String,
    Int,
    Int32,
    Int64,
    Uint,
    Float64,
    /// `StringSlice`: comma separated, repeatable, `[a,b]` default representation
    StringSlice,
    /// `StringArray`: repeatable, no comma splitting
    StringArray,
    /// `StringToString`: `k=v,k2=v2`, repeatable
    StringToString,
    Duration,
}

impl FlagKind {
    pub fn type_name(&self) -> &'static str {
        match self {
            FlagKind::Bool => "bool",
            FlagKind::String => "string",
            FlagKind::Int => "int",
            FlagKind::Int32 => "int32",
            FlagKind::Int64 => "int64",
            FlagKind::Uint => "uint",
            FlagKind::Float64 => "float64",
            FlagKind::StringSlice => "stringSlice",
            FlagKind::StringArray => "stringArray",
            FlagKind::StringToString => "stringToString",
            FlagKind::Duration => "duration",
        }
    }

    /// Name displayed after the flag in usages when the usage has no back-quoted name (pflag `UnquoteUsage`).
    pub(crate) fn display_name(&self) -> &'static str {
        match self {
            FlagKind::Bool => "",
            FlagKind::Float64 => "float",
            FlagKind::Int64 => "int",
            FlagKind::StringSlice => "strings",
            other => other.type_name(),
        }
    }

    pub(crate) fn is_multi(&self) -> bool {
        matches!(
            self,
            FlagKind::StringSlice | FlagKind::StringArray | FlagKind::StringToString
        )
    }

    pub(crate) fn zero_default(&self) -> &'static str {
        match self {
            FlagKind::Bool => "false",
            FlagKind::String => "",
            FlagKind::Int
            | FlagKind::Int32
            | FlagKind::Int64
            | FlagKind::Uint
            | FlagKind::Float64 => "0",
            FlagKind::StringSlice | FlagKind::StringArray | FlagKind::StringToString => "[]",
            FlagKind::Duration => "0s",
        }
    }
}

/// A flag declaration, the equivalent of `cmd.Flags().StringVarP(&v, name, shorthand, default, usage)`.
///
/// `default` is pflag's `DefValue` string representation (e.g. `"30"`, `"15m0s"`, `"[sh]"`, `"true"`).
#[derive(Clone, Debug)]
pub struct Flag {
    pub name: String,
    pub shorthand: Option<char>,
    pub kind: FlagKind,
    pub default: String,
    pub usage: String,
    pub hidden: bool,
    pub required: bool,
    pub persistent: bool,
    pub deprecated: String,
    /// Other long names normalized to this flag (pflag `SetNormalizeFunc`); never listed in help.
    pub aliases: Vec<String>,
}

impl Flag {
    pub fn new(kind: FlagKind, name: &str, usage: &str) -> Self {
        Flag {
            name: name.to_string(),
            shorthand: None,
            kind,
            default: kind.zero_default().to_string(),
            usage: usage.to_string(),
            hidden: false,
            required: false,
            persistent: false,
            deprecated: String::new(),
            aliases: Vec::new(),
        }
    }

    /// `Flags().Bool(name, false, usage)`
    pub fn bool(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Bool, name, usage)
    }
    /// `Flags().String(name, "", usage)`
    pub fn string(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::String, name, usage)
    }
    /// `Flags().Int(name, 0, usage)`
    pub fn int(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Int, name, usage)
    }
    /// `Flags().Int32(name, 0, usage)`
    pub fn int32(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Int32, name, usage)
    }
    /// `Flags().Int64(name, 0, usage)`
    pub fn int64(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Int64, name, usage)
    }
    /// `Flags().Uint(name, 0, usage)`
    pub fn uint(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Uint, name, usage)
    }
    /// `Flags().Float64(name, 0, usage)`
    pub fn float64(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Float64, name, usage)
    }
    /// `Flags().StringSlice(name, nil, usage)`
    pub fn string_slice(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::StringSlice, name, usage)
    }
    /// `Flags().StringArray(name, nil, usage)`
    pub fn string_array(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::StringArray, name, usage)
    }
    /// `Flags().StringToString(name, nil, usage)`
    pub fn string_to_string(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::StringToString, name, usage)
    }
    /// `Flags().Duration(name, 0, usage)`
    pub fn duration(name: &str, usage: &str) -> Self {
        Flag::new(FlagKind::Duration, name, usage)
    }

    /// Shorthand letter (`StringVarP(..., "n", ...)`).
    pub fn short(mut self, c: char) -> Self {
        self.shorthand = Some(c);
        self
    }

    /// pflag `DefValue` representation of the default value: `"x"` for strings, `"30"` for ints,
    /// `"true"` for bools, `"15m0s"` for durations, `"[a,b]"` for slices.
    pub fn default_value(mut self, v: impl Into<String>) -> Self {
        self.default = v.into();
        self
    }

    /// `cmd.MarkFlagRequired(name)`
    pub fn required(mut self) -> Self {
        self.required = true;
        self
    }

    /// `cmd.Flags().MarkHidden(name)`
    pub fn hidden(mut self) -> Self {
        self.hidden = true;
        self
    }

    /// Declare on `PersistentFlags()` instead of `Flags()`.
    pub fn persistent(mut self) -> Self {
        self.persistent = true;
        self
    }

    /// Accept `--alias` as a spelling of this flag (pflag `SetNormalizeFunc` mapping `alias` -> name).
    pub fn alias(mut self, alias: &str) -> Self {
        self.aliases.push(alias.to_string());
        self
    }

    /// `cmd.Flags().MarkDeprecated(name, msg)`
    pub fn deprecated(mut self, msg: &str) -> Self {
        self.deprecated = msg.to_string();
        self
    }

    /// pflag `defaultIsZeroValue`.
    pub(crate) fn default_is_zero(&self) -> bool {
        match self.kind {
            FlagKind::Bool => self.default == "false",
            FlagKind::Duration => self.default == "0" || self.default == "0s",
            FlagKind::Int
            | FlagKind::Int32
            | FlagKind::Int64
            | FlagKind::Uint
            | FlagKind::Float64 => self.default == "0",
            FlagKind::String => self.default.is_empty(),
            FlagKind::StringSlice | FlagKind::StringArray => self.default == "[]",
            FlagKind::StringToString => {
                matches!(self.default.as_str(), "false" | "<nil>" | "" | "0")
            }
        }
    }

    /// pflag `UnquoteUsage`: (varname, usage).
    pub(crate) fn unquote_usage(&self) -> (String, String) {
        let usage = &self.usage;
        if let Some(start) = usage.find('`')
            && let Some(len) = usage[start + 1..].find('`')
        {
            let end = start + 1 + len;
            let name = usage[start + 1..end].to_string();
            let new_usage = format!("{}{}{}", &usage[..start], name, &usage[end + 1..]);
            return (name, new_usage);
        }
        (self.kind.display_name().to_string(), usage.clone())
    }
}
