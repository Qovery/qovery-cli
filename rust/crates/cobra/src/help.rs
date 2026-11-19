//! cobra default help / usage rendering and pflag `FlagUsages`.

use crate::flag::{Flag, FlagKind};
use crate::tree::{NodeId, Tree};
use qovery_log::gofmt::quote;

/// pflag `FlagUsagesWrapped(0)`.
pub fn flag_usages(flags: &[Flag]) -> String {
    let mut lines: Vec<String> = Vec::new();
    let mut maxlen = 0usize;
    for f in flags {
        if f.hidden {
            continue;
        }
        let mut line = match f.shorthand {
            Some(c) => format!("  -{c}, --{}", f.name),
            None => format!("      --{}", f.name),
        };
        let (varname, usage) = f.unquote_usage();
        if !varname.is_empty() {
            line.push(' ');
            line.push_str(&varname);
        }
        line.push('\x00');
        if line.len() > maxlen {
            maxlen = line.len();
        }
        line.push_str(&usage);
        if !f.default_is_zero() {
            if f.kind == FlagKind::String {
                line.push_str(&format!(" (default {})", quote(&f.default)));
            } else {
                line.push_str(&format!(" (default {})", f.default));
            }
        }
        if !f.deprecated.is_empty() {
            line.push_str(&format!(" (DEPRECATED: {})", f.deprecated));
        }
        lines.push(line);
    }
    let mut buf = String::new();
    for line in lines {
        let sidx = line.find('\x00').unwrap();
        let spacing = " ".repeat(maxlen - sidx);
        let rest = &line[sidx + 1..];
        // wrap(maxlen+2, 0, rest): only re-indent embedded newlines
        let wrapped = rest.replace('\n', &format!("\n{}", " ".repeat(maxlen + 2)));
        buf.push_str(&format!("{} {} {}\n", &line[..sidx], spacing, wrapped));
    }
    buf
}

fn trim_right_space(s: &str) -> &str {
    s.trim_end_matches(|c: char| c.is_whitespace())
}

fn rpad(s: &str, padding: usize) -> String {
    format!("{s:<padding$}")
}

/// cobra `UsageString()`.
pub fn usage_string(tree: &Tree, id: NodeId) -> String {
    let n = tree.node(id);
    let mut w = String::from("Usage:");
    if n.runnable() {
        w.push_str(&format!("\n  {}", tree.use_line(id)));
    }
    if tree.has_available_sub_commands(id) {
        w.push_str(&format!("\n  {} [command]", tree.command_path(id)));
    }
    if !n.aliases.is_empty() {
        w.push_str("\n\nAliases:\n");
        let mut names = vec![n.name().to_string()];
        names.extend(n.aliases.iter().cloned());
        w.push_str(&format!("  {}", names.join(", ")));
    }
    if !n.example.is_empty() {
        w.push_str("\n\nExamples:\n");
        w.push_str(&n.example);
    }
    if tree.has_available_sub_commands(id) {
        w.push_str("\n\nAvailable Commands:");
        for c in tree.sorted_children(id) {
            if tree.is_available_command(c.id) || c.name() == "help" {
                w.push_str(&format!(
                    "\n  {} {}",
                    rpad(c.name(), tree.name_padding(c.id)),
                    c.short
                ));
            }
        }
    }
    if tree.has_available_local_flags(id) {
        w.push_str("\n\nFlags:\n");
        w.push_str(trim_right_space(&flag_usages(&tree.local_flags(id))));
    }
    if tree.has_available_inherited_flags(id) {
        w.push_str("\n\nGlobal Flags:\n");
        w.push_str(trim_right_space(&flag_usages(&tree.inherited_flags(id))));
    }
    if tree.has_help_sub_commands(id) {
        w.push_str("\n\nAdditional help topics:");
        let children = tree.sorted_children(id);
        let pad = children
            .iter()
            .map(|c| tree.command_path(c.id).len())
            .max()
            .unwrap_or(0)
            .max(11);
        for c in children {
            if tree.is_additional_help_topic_command(c.id) {
                w.push_str(&format!(
                    "\n  {} {}",
                    rpad(&tree.command_path(c.id), pad),
                    c.short
                ));
            }
        }
    }
    if tree.has_available_sub_commands(id) {
        w.push_str(&format!(
            "\n\nUse \"{} [command] --help\" for more information about a command.",
            tree.command_path(id)
        ));
    }
    w.push('\n');
    w
}

/// cobra default help (`cmd.Help()`).
pub fn help_string(tree: &Tree, id: NodeId) -> String {
    let n = tree.node(id);
    let mut w = String::new();
    let usage = if n.long.is_empty() { &n.short } else { &n.long };
    let usage = trim_right_space(usage);
    if !usage.is_empty() {
        w.push_str(usage);
        w.push_str("\n\n");
    }
    if n.runnable() || tree.has_sub_commands(id) {
        w.push_str(&usage_string(tree, id));
    }
    w
}
