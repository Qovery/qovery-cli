//! Parity helper: rebuilds the Go command tree from a `QOVERY_DUMP_SPEC` dump (`spec.json` next to the
//! executable) and runs it with this crate's `completion` / `__complete` commands, to compare `__complete`
//! with the Go binary on the whole tree before every command is ported.

use qovery_cobra::{App, Command, Flag, FlagKind};
use serde_json::Value;

fn kind(t: &str) -> FlagKind {
    match t {
        "bool" => FlagKind::Bool,
        "int" => FlagKind::Int,
        "int32" => FlagKind::Int32,
        "int64" => FlagKind::Int64,
        "uint" => FlagKind::Uint,
        "float64" => FlagKind::Float64,
        "stringSlice" => FlagKind::StringSlice,
        "stringArray" => FlagKind::StringArray,
        "stringToString" => FlagKind::StringToString,
        "duration" => FlagKind::Duration,
        _ => FlagKind::String,
    }
}

fn build(v: &Value, is_root: bool) -> Command {
    let s = |k: &str| v[k].as_str().unwrap_or("").to_string();
    let mut c = Command::new(&s("Use"))
        .short(&s("Short"))
        .long(&s("Long"))
        .example(&s("Example"));
    for a in v["Aliases"].as_array().into_iter().flatten() {
        c = c.alias(a.as_str().unwrap());
    }
    if v["Hidden"].as_bool() == Some(true) {
        c = c.hidden();
    }
    if !s("Deprecated").is_empty() {
        c = c.deprecated(&s("Deprecated"));
    }
    if v["Runnable"].as_bool() == Some(true) {
        c = c.run(|_| {});
    }
    for f in v["Flags"].as_array().into_iter().flatten() {
        let fs = |k: &str| f[k].as_str().unwrap_or("").to_string();
        let mut fl =
            Flag::new(kind(&fs("Type")), &fs("Name"), &fs("Usage")).default_value(fs("Default"));
        if let Some(c) = fs("Shorthand").chars().next() {
            fl = fl.short(c);
        }
        if f["Hidden"].as_bool() == Some(true) {
            fl = fl.hidden();
        }
        if f["Persistent"].as_bool() == Some(true) {
            fl = fl.persistent();
        }
        if f["Required"].as_bool() == Some(true) {
            fl = fl.required();
        }
        if !fs("Deprecated").is_empty() {
            fl = fl.deprecated(&fs("Deprecated"));
        }
        c = c.flag(fl);
    }
    for ch in v["Children"].as_array().into_iter().flatten() {
        let name = ch["Use"]
            .as_str()
            .unwrap_or("")
            .split_whitespace()
            .next()
            .unwrap_or("");
        if is_root && (name == "help" || name == "completion" || name == "list-commands") {
            continue;
        }
        c = c.subcommand(build(ch, false));
    }
    c
}

fn main() {
    let path = std::env::current_exe().unwrap().with_file_name("spec.json");
    let spec: Value = serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap();
    let root = build(&spec, true)
        .subcommand(qovery_cmd_list_commands::command())
        .subcommands(qovery_cmd_completion::commands());
    qovery_cobra::execute(App::new(root));
}
