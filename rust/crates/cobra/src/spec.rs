//! Machine readable dump of the command tree (`QOVERY_DUMP_SPEC=1 qovery`), in the same shape as the
//! dump produced from the Go implementation, to diff both interfaces.

use crate::tree::{NodeId, Tree};
use serde_json::{Value, json};

fn dump_node(tree: &Tree, id: NodeId) -> Value {
    let n = tree.node(id);
    let mut local: Vec<_> = n
        .flags
        .iter()
        .filter(|f| !f.persistent && f.name != "help")
        .cloned()
        .collect();
    local.sort_by(|a, b| a.name.cmp(&b.name));
    let mut persistent: Vec<_> = n.flags.iter().filter(|f| f.persistent).cloned().collect();
    persistent.sort_by(|a, b| a.name.cmp(&b.name));
    let flags: Vec<Value> = local
        .iter()
        .chain(persistent.iter())
        .map(|f| {
            json!({
                "Name": f.name,
                "Shorthand": f.shorthand.map(|c| c.to_string()).unwrap_or_default(),
                "Type": f.kind.type_name(),
                "Default": f.default,
                "Usage": f.usage,
                "Hidden": f.hidden,
                "Persistent": f.persistent,
                "Required": f.required,
                "Deprecated": f.deprecated,
            })
        })
        .collect();
    let children: Vec<Value> = tree
        .sorted_children(id)
        .iter()
        .map(|c| dump_node(tree, c.id))
        .collect();
    json!({
        "Path": tree.command_path(id),
        "Use": n.use_,
        "Short": n.short,
        "Long": n.long,
        "Example": n.example,
        "Aliases": if n.aliases.is_empty() { Value::Null } else { json!(n.aliases) },
        "Hidden": n.hidden,
        "Deprecated": n.deprecated,
        "Runnable": n.runnable(),
        "Flags": if flags.is_empty() { Value::Null } else { Value::Array(flags) },
        "Children": if children.is_empty() { Value::Null } else { Value::Array(children) },
    })
}

pub fn dump(tree: &Tree) -> String {
    serde_json::to_string_pretty(&dump_node(tree, 0)).unwrap_or_default()
}
