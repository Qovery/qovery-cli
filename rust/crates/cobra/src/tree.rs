//! Runtime command tree (arena with parent links), cobra's command lookup and flag sets.

use crate::args::ArgsRule;
use crate::command::{Command, RunFn};
use crate::flag::{Flag, FlagKind};

pub type NodeId = usize;

/// A command in the runtime tree.
#[derive(Clone)]
pub struct Node {
    pub id: NodeId,
    pub parent: Option<NodeId>,
    /// children in declaration order
    pub children: Vec<NodeId>,
    pub use_: String,
    pub short: String,
    pub long: String,
    pub example: String,
    pub aliases: Vec<String>,
    pub hidden: bool,
    pub deprecated: String,
    pub args: ArgsRule,
    /// flags declared on this command (local + persistent), help flag included
    pub flags: Vec<Flag>,
    pub run: Option<RunFn>,
    pub is_help_command: bool,
    pub disable_flag_parsing: bool,
    pub disable_flags_in_use_line: bool,
    /// cobra `commandsMaxNameLen`: longest child name (hidden ones included)
    pub commands_max_name_len: usize,
}

impl Node {
    pub fn name(&self) -> &str {
        self.use_.split_whitespace().next().unwrap_or("")
    }
    pub fn runnable(&self) -> bool {
        self.run.is_some()
    }
    pub fn has_alias(&self, s: &str) -> bool {
        self.aliases.iter().any(|a| a == s)
    }
}

/// The full command tree.
#[derive(Clone)]
pub struct Tree {
    pub nodes: Vec<Node>,
}

pub(crate) fn help_flag(name: &str) -> Flag {
    Flag::bool("help", &format!("help for {name}")).short('h')
}

impl Tree {
    pub fn build(root: Command) -> Tree {
        let mut tree = Tree { nodes: Vec::new() };
        tree.add(root, None);
        tree
    }

    fn add(&mut self, cmd: Command, parent: Option<NodeId>) -> NodeId {
        let id = self.nodes.len();
        let name = cmd.name().to_string();
        let mut flags = cmd.flags;
        if !flags.iter().any(|f| f.name == "help") {
            flags.push(help_flag(&name));
        }
        self.nodes.push(Node {
            id,
            parent,
            children: Vec::new(),
            use_: cmd.use_,
            short: cmd.short,
            long: cmd.long,
            example: cmd.example,
            aliases: cmd.aliases,
            hidden: cmd.hidden,
            deprecated: cmd.deprecated,
            args: cmd.args,
            flags,
            run: cmd.run,
            is_help_command: cmd.is_help_command,
            disable_flag_parsing: cmd.disable_flag_parsing,
            disable_flags_in_use_line: cmd.disable_flags_in_use_line,
            commands_max_name_len: 0,
        });
        for child in cmd.children {
            let len = child.name().len();
            let cid = self.add(child, Some(id));
            self.nodes[id].children.push(cid);
            if len > self.nodes[id].commands_max_name_len {
                self.nodes[id].commands_max_name_len = len;
            }
        }
        id
    }

    pub fn root(&self) -> &Node {
        &self.nodes[0]
    }

    pub fn node(&self, id: NodeId) -> &Node {
        &self.nodes[id]
    }

    /// Children sorted by name (cobra `Commands()` with `EnableCommandSorting`).
    pub fn sorted_children(&self, id: NodeId) -> Vec<&Node> {
        let mut v: Vec<&Node> = self.nodes[id]
            .children
            .iter()
            .map(|c| &self.nodes[*c])
            .collect();
        v.sort_by(|a, b| a.name().cmp(b.name()));
        v
    }

    pub fn has_sub_commands(&self, id: NodeId) -> bool {
        !self.nodes[id].children.is_empty()
    }

    /// cobra `IsAvailableCommand`.
    pub fn is_available_command(&self, id: NodeId) -> bool {
        let n = &self.nodes[id];
        if !n.deprecated.is_empty() || n.hidden || n.is_help_command {
            return false;
        }
        n.runnable() || self.has_available_sub_commands(id)
    }

    pub fn has_available_sub_commands(&self, id: NodeId) -> bool {
        self.nodes[id]
            .children
            .iter()
            .any(|c| self.is_available_command(*c))
    }

    /// cobra `IsAdditionalHelpTopicCommand`.
    pub fn is_additional_help_topic_command(&self, id: NodeId) -> bool {
        let n = &self.nodes[id];
        if n.runnable() || !n.deprecated.is_empty() || n.hidden {
            return false;
        }
        n.children
            .iter()
            .all(|c| self.is_additional_help_topic_command(*c))
    }

    pub fn has_help_sub_commands(&self, id: NodeId) -> bool {
        self.nodes[id]
            .children
            .iter()
            .any(|c| self.is_additional_help_topic_command(*c))
    }

    /// cobra `CommandPath()`: `qovery application list`.
    pub fn command_path(&self, id: NodeId) -> String {
        let n = &self.nodes[id];
        match n.parent {
            Some(p) => format!("{} {}", self.command_path(p), n.name()),
            None => n.name().to_string(),
        }
    }

    /// cobra `UseLine()`.
    pub fn use_line(&self, id: NodeId) -> String {
        let n = &self.nodes[id];
        let useline = match n.parent {
            Some(p) => format!("{} {}", self.command_path(p), n.use_),
            None => n.use_.clone(),
        };
        if n.disable_flags_in_use_line {
            return useline;
        }
        if self.has_available_flags(id) && !useline.contains("[flags]") {
            format!("{useline} [flags]")
        } else {
            useline
        }
    }

    /// cobra `NamePadding()`.
    pub fn name_padding(&self, id: NodeId) -> usize {
        const MIN_NAME_PADDING: usize = 11;
        match self.nodes[id].parent {
            None => MIN_NAME_PADDING,
            Some(p) => {
                let max = self.nodes[p].commands_max_name_len;
                if MIN_NAME_PADDING > max {
                    MIN_NAME_PADDING
                } else {
                    max
                }
            }
        }
    }

    /// Persistent flags inherited from ancestors (closest ancestor wins), not shadowed by local flags.
    pub fn inherited_flags(&self, id: NodeId) -> Vec<Flag> {
        let mut out: Vec<Flag> = Vec::new();
        let local: Vec<&str> = self.nodes[id]
            .flags
            .iter()
            .map(|f| f.name.as_str())
            .collect();
        let mut cur = self.nodes[id].parent;
        while let Some(p) = cur {
            for f in &self.nodes[p].flags {
                if f.persistent
                    && !local.contains(&f.name.as_str())
                    && !out.iter().any(|o| o.name == f.name)
                {
                    out.push(f.clone());
                }
            }
            cur = self.nodes[p].parent;
        }
        out.sort_by(|a, b| a.name.cmp(&b.name));
        out
    }

    /// Flags declared on the command itself (local + own persistent), sorted (cobra `LocalFlags()`).
    pub fn local_flags(&self, id: NodeId) -> Vec<Flag> {
        let mut v = self.nodes[id].flags.clone();
        v.sort_by(|a, b| a.name.cmp(&b.name));
        v
    }

    /// Every flag usable on the command, sorted (cobra `Flags()` after `mergePersistentFlags`).
    pub fn all_flags(&self, id: NodeId) -> Vec<Flag> {
        let mut v = self.nodes[id].flags.clone();
        v.extend(self.inherited_flags(id));
        v.sort_by(|a, b| a.name.cmp(&b.name));
        v
    }

    pub fn has_available_flags(&self, id: NodeId) -> bool {
        self.all_flags(id).iter().any(|f| !f.hidden)
    }
    pub fn has_available_local_flags(&self, id: NodeId) -> bool {
        self.local_flags(id).iter().any(|f| !f.hidden)
    }
    pub fn has_available_inherited_flags(&self, id: NodeId) -> bool {
        self.inherited_flags(id).iter().any(|f| !f.hidden)
    }

    fn find_next(&self, id: NodeId, next: &str) -> Option<NodeId> {
        self.nodes[id]
            .children
            .iter()
            .copied()
            .find(|c| self.nodes[*c].name() == next || self.nodes[*c].has_alias(next))
    }

    /// cobra `Find(args)`: returns the command and its remaining args, or the "unknown command" error.
    pub fn find(&self, args: &[String]) -> (NodeId, Vec<String>, Option<String>) {
        // cobra adds the `help` flag only when a command is executed: it is unknown while finding the command
        let find_flags = |id: NodeId| -> Vec<Flag> {
            self.all_flags(id)
                .into_iter()
                .filter(|f| f.name != "help")
                .collect()
        };
        let mut id = 0;
        let mut inner: Vec<String> = args.to_vec();
        loop {
            let without_flags = strip_flags(&inner, &find_flags(id));
            let Some(next) = without_flags.first() else {
                break;
            };
            match self.find_next(id, next) {
                Some(cid) => {
                    inner = args_minus_first_x(&inner, next, &find_flags(id));
                    id = cid;
                }
                None => break,
            }
        }
        if self.nodes[id].args == ArgsRule::Legacy {
            let stripped = strip_flags(&inner, &find_flags(id));
            if let Some(err) = self.legacy_args(id, &stripped) {
                return (id, inner, Some(err));
            }
        }
        (id, inner, None)
    }

    fn legacy_args(&self, id: NodeId, args: &[String]) -> Option<String> {
        if !self.has_sub_commands(id) {
            return None;
        }
        if self.nodes[id].parent.is_none() && !args.is_empty() {
            return Some(format!(
                "unknown command {} for {}{}",
                qovery_log::gofmt::quote(&args[0]),
                qovery_log::gofmt::quote(&self.command_path(id)),
                self.find_suggestions(id, &args[0])
            ));
        }
        None
    }

    /// cobra `findSuggestions`.
    pub fn find_suggestions(&self, id: NodeId, arg: &str) -> String {
        let suggestions = self.suggestions_for(id, arg);
        if suggestions.is_empty() {
            return String::new();
        }
        let mut s = String::from("\n\nDid you mean this?\n");
        for sug in suggestions {
            s.push_str(&format!("\t{sug}\n"));
        }
        s
    }

    /// cobra `SuggestionsFor` (Levenshtein distance <= 2, case insensitive, or prefix).
    pub fn suggestions_for(&self, id: NodeId, typed: &str) -> Vec<String> {
        let mut out = Vec::new();
        for c in &self.nodes[id].children {
            if !self.is_available_command(*c) {
                continue;
            }
            let name = self.nodes[*c].name();
            let d = levenshtein(&typed.to_lowercase(), &name.to_lowercase());
            if d <= 2 || name.to_lowercase().starts_with(&typed.to_lowercase()) {
                out.push(name.to_string());
            }
        }
        out
    }
}

fn levenshtein(s: &str, t: &str) -> usize {
    let s: Vec<char> = s.chars().collect();
    let t: Vec<char> = t.chars().collect();
    let mut d = vec![vec![0usize; t.len() + 1]; s.len() + 1];
    for (i, row) in d.iter_mut().enumerate() {
        row[0] = i;
    }
    for (j, cell) in d[0].iter_mut().enumerate() {
        *cell = j;
    }
    for j in 1..=t.len() {
        for i in 1..=s.len() {
            if s[i - 1] == t[j - 1] {
                d[i][j] = d[i - 1][j - 1];
            } else {
                let mut min = d[i - 1][j];
                if d[i][j - 1] < min {
                    min = d[i][j - 1];
                }
                if d[i - 1][j - 1] < min {
                    min = d[i - 1][j - 1];
                }
                d[i][j] = min + 1;
            }
        }
    }
    d[s.len()][t.len()]
}

fn has_no_opt_def_val(name: &str, flags: &[Flag]) -> bool {
    flags
        .iter()
        .any(|f| f.name == name && f.kind == FlagKind::Bool)
}

fn short_has_no_opt_def_val(name: &str, flags: &[Flag]) -> bool {
    let Some(c) = name.chars().next() else {
        return false;
    };
    if name.chars().count() != 1 {
        return false;
    }
    flags
        .iter()
        .any(|f| f.shorthand == Some(c) && f.kind == FlagKind::Bool)
}

/// cobra `stripFlags`.
pub(crate) fn strip_flags(args: &[String], flags: &[Flag]) -> Vec<String> {
    let mut commands = Vec::new();
    let mut i = 0;
    while i < args.len() {
        let s = &args[i];
        i += 1;
        if s == "--" {
            break;
        }
        let long_with_value =
            s.starts_with("--") && !s.contains('=') && !has_no_opt_def_val(&s[2..], flags);
        let short_with_value = s.starts_with('-')
            && !s.contains('=')
            && s.len() == 2
            && !short_has_no_opt_def_val(&s[1..], flags);
        if long_with_value || short_with_value {
            if args.len() - i <= 1 {
                break;
            }
            i += 1;
            continue;
        }
        if !s.is_empty() && !s.starts_with('-') {
            commands.push(s.clone());
        }
    }
    commands
}

/// cobra `argsMinusFirstX`.
fn args_minus_first_x(args: &[String], x: &str, flags: &[Flag]) -> Vec<String> {
    let mut pos = 0;
    while pos < args.len() {
        let s = &args[pos];
        if s == "--" {
            break;
        }
        let long_with_value =
            s.starts_with("--") && !s.contains('=') && !has_no_opt_def_val(&s[2..], flags);
        let short_with_value = s.starts_with('-')
            && !s.contains('=')
            && s.len() == 2
            && !short_has_no_opt_def_val(&s[1..], flags);
        if long_with_value || short_with_value {
            pos += 2;
            continue;
        }
        if !s.starts_with('-') && s == x {
            let mut ret = args[..pos].to_vec();
            ret.extend_from_slice(&args[pos + 1..]);
            return ret;
        }
        pos += 1;
    }
    args.to_vec()
}
