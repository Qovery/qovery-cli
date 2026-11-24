//! cobra `Execute()` / `ExecuteC()` / `execute()`.

use crate::command::Command;
use crate::ctx::Ctx;
use crate::help;
use crate::parse::{self, FlagValue};
use crate::tree::{NodeId, Tree};
use std::collections::BTreeMap;
use std::io::Write;

/// The application: root command + `cobra.OnInitialize` hooks.
pub struct App {
    pub root: Command,
    /// `cobra.OnInitialize(...)`: run right before a command's `Run` (not for help / errors).
    pub initializers: Vec<fn()>,
    /// Called with the error when `Execute` fails (the Go root does `utils.PrintlnError(err); os.Exit(0)`).
    pub on_error: fn(&str) -> !,
}

fn default_on_error(err: &str) -> ! {
    qovery_log::println_error(err);
    qovery_log::exit::exit(0)
}

impl App {
    pub fn new(root: Command) -> Self {
        App {
            root,
            initializers: Vec::new(),
            on_error: default_on_error,
        }
    }
    pub fn on_initialize(mut self, f: fn()) -> Self {
        self.initializers.push(f);
        self
    }
    pub fn on_error(mut self, f: fn(&str) -> !) -> Self {
        self.on_error = f;
        self
    }
}

fn help_command(root_name: &str) -> Command {
    let mut c = Command::new("help [command]").short("Help about any command").long(&format!(
        "Help provides help for any command in the application.\nSimply type {root_name} help [path to command] for full details."
    ));
    c.run = Some(run_help);
    c.is_help_command = true;
    c
}

/// Go `%#q` of a `[]string`.
fn backquote_list(args: &[String]) -> String {
    let items: Vec<String> = args
        .iter()
        .map(|a| {
            if a.contains('`') || a.chars().any(|c| c.is_control()) {
                qovery_log::gofmt::quote(a)
            } else {
                format!("`{a}`")
            }
        })
        .collect();
    format!("[{}]", items.join(" "))
}

fn run_help(ctx: &Ctx) {
    let tree = ctx.tree();
    let (id, _, err) = tree.find(ctx.args());
    if err.is_some() {
        eprintln!("Unknown help topic {}", backquote_list(ctx.args()));
        let _ = std::io::stderr().write_all(help::usage_string(tree, 0).as_bytes());
        return;
    }
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(help::help_string(tree, id).as_bytes());
    let _ = out.flush();
}

/// `rootCmd.Execute()` followed by the Go `main` error handling. Never returns.
pub fn execute(app: App) -> ! {
    let args: Vec<String> = std::env::args().skip(1).collect();
    execute_with_args(app, args)
}

/// Same as [`execute`] with explicit args (without the binary name).
pub fn execute_with_args(mut app: App, args: Vec<String>) -> ! {
    if !app.root.children.is_empty() && !app.root.children.iter().any(|c| c.name() == "help") {
        let name = app.root.name().to_string();
        app.root.children.push(help_command(&name));
    }
    let tree = Tree::build(app.root);

    if std::env::var_os("QOVERY_DUMP_SPEC").is_some() {
        println!("{}", crate::spec::dump(&tree));
        qovery_log::exit::exit(0);
    }

    match execute_c(&tree, &args, &app.initializers) {
        Ok(()) => {
            qovery_log::flush();
            qovery_log::exit::exit(0)
        }
        Err(e) => (app.on_error)(&e),
    }
}

fn print_err_and_usage(tree: &Tree, id: NodeId, msg: &str) {
    let mut err = std::io::stderr().lock();
    let _ = writeln!(err, "Error: {msg}");
    let _ = writeln!(err, "{}", help::usage_string(tree, id));
}

fn execute_c(tree: &Tree, args: &[String], initializers: &[fn()]) -> Result<(), String> {
    let (id, rest, find_err) = tree.find(args);
    if let Some(e) = find_err {
        let mut err = std::io::stderr().lock();
        let _ = writeln!(err, "Error: {e}");
        let _ = writeln!(err, "Run '{} --help' for usage.", tree.command_path(id));
        return Err(e);
    }
    let node = tree.node(id);
    if !node.deprecated.is_empty() {
        eprintln!(
            "Command {} is deprecated, {}",
            qovery_log::gofmt::quote(node.name()),
            node.deprecated
        );
    }

    let all_flags = tree.all_flags(id);
    let (flags, positional) = if node.disable_flag_parsing {
        let flags: BTreeMap<String, FlagValue> = all_flags
            .iter()
            .map(|f| {
                (
                    f.name.clone(),
                    FlagValue {
                        flag: f.clone(),
                        raw: Vec::new(),
                        changed: false,
                    },
                )
            })
            .collect();
        (flags, rest.clone())
    } else {
        match parse::parse(node.name(), &all_flags, &rest) {
            Ok(p) => (p.flags, p.args),
            Err(msg) => {
                print_err_and_usage(tree, id, &msg);
                return Err(msg);
            }
        }
    };

    let ctx = Ctx {
        tree,
        id,
        args: positional,
        flags,
    };
    if ctx.has_flag("verbose") {
        crate::set_verbose(ctx.bool("verbose"));
    }
    for (name, value, changed) in ctx.all_flags() {
        let _ = value;
        if changed {
            let f = &ctx.flags[&name].flag;
            if !f.deprecated.is_empty() {
                eprintln!("Flag --{} has been deprecated, {}", f.name, f.deprecated);
            }
        }
    }

    if ctx.has_flag("help") && ctx.bool("help") {
        ctx.help();
        return Ok(());
    }
    let Some(run) = node.run else {
        ctx.help();
        return Ok(());
    };

    for init in initializers {
        init();
    }

    if let Err(msg) = node.args.validate(ctx.args(), &tree.command_path(id)) {
        print_err_and_usage(tree, id, &msg);
        return Err(msg);
    }

    let missing: Vec<String> = all_flags
        .iter()
        .filter(|f| f.required && !ctx.changed(&f.name))
        .map(|f| f.name.clone())
        .collect();
    if !missing.is_empty() {
        let msg = format!("required flag(s) \"{}\" not set", missing.join("\", \""));
        print_err_and_usage(tree, id, &msg);
        return Err(msg);
    }

    run(&ctx);
    Ok(())
}
