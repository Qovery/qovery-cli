//! Command declarations (the equivalent of `&cobra.Command{...}` + `AddCommand` + flags).

use crate::args::ArgsRule;
use crate::ctx::Ctx;
use crate::flag::Flag;

/// Command implementation. Mirrors cobra's `Run: func(cmd *cobra.Command, args []string)`:
/// it prints its own errors and calls `std::process::exit` itself when needed.
pub type RunFn = fn(&Ctx);

#[derive(Clone)]
pub struct Command {
    /// cobra `Use` (e.g. `"deploy"`, `"get-token <cluster-id>"`); the name is the first word.
    pub use_: String,
    pub short: String,
    pub long: String,
    pub example: String,
    pub aliases: Vec<String>,
    pub hidden: bool,
    pub deprecated: String,
    pub args: ArgsRule,
    pub flags: Vec<Flag>,
    pub run: Option<RunFn>,
    pub children: Vec<Command>,
    /// cobra `DisableFlagParsing`: every argument is passed as positional arg.
    pub disable_flag_parsing: bool,
    /// cobra `DisableFlagsInUseLine`: no ` [flags]` suffix in the usage line.
    pub disable_flags_in_use_line: bool,
    /// Set on cobra's builtin `help` command.
    pub(crate) is_help_command: bool,
}

impl Command {
    pub fn new(use_: &str) -> Self {
        Command {
            use_: use_.to_string(),
            short: String::new(),
            long: String::new(),
            example: String::new(),
            aliases: Vec::new(),
            hidden: false,
            deprecated: String::new(),
            args: ArgsRule::Legacy,
            flags: Vec::new(),
            run: None,
            children: Vec::new(),
            disable_flag_parsing: false,
            disable_flags_in_use_line: false,
            is_help_command: false,
        }
    }

    /// Command name: first word of `Use`.
    pub fn name(&self) -> &str {
        self.use_.split_whitespace().next().unwrap_or("")
    }

    pub fn short(mut self, s: &str) -> Self {
        self.short = s.to_string();
        self
    }
    pub fn long(mut self, s: &str) -> Self {
        self.long = s.to_string();
        self
    }
    pub fn example(mut self, s: &str) -> Self {
        self.example = s.to_string();
        self
    }
    pub fn alias(mut self, a: &str) -> Self {
        self.aliases.push(a.to_string());
        self
    }
    pub fn aliases(mut self, a: &[&str]) -> Self {
        self.aliases.extend(a.iter().map(|s| s.to_string()));
        self
    }
    pub fn hidden(mut self) -> Self {
        self.hidden = true;
        self
    }
    pub fn deprecated(mut self, msg: &str) -> Self {
        self.deprecated = msg.to_string();
        self
    }
    pub fn args(mut self, rule: ArgsRule) -> Self {
        self.args = rule;
        self
    }
    pub fn flag(mut self, f: Flag) -> Self {
        self.flags.push(f);
        self
    }
    pub fn flags(mut self, fs: impl IntoIterator<Item = Flag>) -> Self {
        self.flags.extend(fs);
        self
    }
    pub fn disable_flag_parsing(mut self) -> Self {
        self.disable_flag_parsing = true;
        self
    }
    pub fn disable_flags_in_use_line(mut self) -> Self {
        self.disable_flags_in_use_line = true;
        self
    }
    pub fn run(mut self, f: RunFn) -> Self {
        self.run = Some(f);
        self
    }
    pub fn subcommand(mut self, c: Command) -> Self {
        self.children.push(c);
        self
    }
    pub fn subcommands(mut self, cs: impl IntoIterator<Item = Command>) -> Self {
        self.children.extend(cs);
        self
    }

    /// Mutable access to a direct child by name (to add sub-commands declared in other crates).
    pub fn child_mut(&mut self, name: &str) -> Option<&mut Command> {
        self.children.iter_mut().find(|c| c.name() == name)
    }
}
