//! `qovery list-commands` command. Port of `cmd/commands_list.go`.

use qovery_cobra::{Command, Ctx, FlagKind, NodeId, Tree};

/// The `list-commands` command.
pub fn command() -> Command {
    Command::new("list-commands")
        .short("List all available commands with descriptions, aliases, args, and flags")
        .run(run)
}

fn run(ctx: &Ctx) {
    let mut out = String::from("Available commands:\n");
    print_commands_recursive(ctx.tree(), 0, "", ctx.node_id(), &mut out);
    print!("{out}");
}

/// cobra `UseLine()` as seen by the Go walk: it is computed before `LocalFlags()` merges the persistent
/// flags of the parents into the command, and the `help` flag only exists on the executed command.
fn use_line(tree: &Tree, id: NodeId, current: NodeId) -> String {
    let n = tree.node(id);
    let useline = match n.parent {
        Some(p) => format!("{} {}", tree.command_path(p), n.use_),
        None => n.use_.clone(),
    };
    let has_available_flags = n
        .flags
        .iter()
        .any(|f| !f.hidden && (f.name != "help" || id == current));
    if has_available_flags && !useline.contains("[flags]") {
        format!("{useline} [flags]")
    } else {
        useline
    }
}

fn print_commands_recursive(
    tree: &Tree,
    id: NodeId,
    parent_path: &str,
    current: NodeId,
    out: &mut String,
) {
    for c in tree.sorted_children(id) {
        if c.hidden {
            continue;
        }
        let full_cmd = format!("{parent_path} {}", c.name()).trim().to_string();
        let aliases = if c.aliases.is_empty() {
            String::new()
        } else {
            format!(" (aliases: {})", c.aliases.join(", "))
        };
        out.push_str(&format!("  {full_cmd}: {}{aliases}\n", c.short));

        if c.use_ != c.name() {
            out.push_str(&format!("    Usage: {}\n", use_line(tree, c.id, current)));
        }

        // `c.LocalFlags().VisitAll`: sorted, hidden flags included, no `help` flag except on the executed command
        let mut flags: Vec<_> = c
            .flags
            .iter()
            .filter(|f| f.name != "help" || c.id == current)
            .collect();
        flags.sort_by(|a, b| a.name.cmp(&b.name));
        for f in flags {
            let def_val = if f.kind == FlagKind::String && f.default.is_empty() {
                "\"\"".to_string()
            } else {
                f.default.clone()
            };
            let short = f.shorthand.map(|c| format!("-{c}, ")).unwrap_or_default();
            out.push_str(&format!(
                "    Flag: {short}--{} ({}), default: {def_val}\n",
                f.name,
                f.kind.type_name()
            ));
        }

        print_commands_recursive(tree, c.id, &full_cmd, current, out);
    }
}
