//! cobra/pflag compatible command layer, built on top of clap.
//!
//! The Go CLI is built with `spf13/cobra`. To keep the user interface strictly identical, this crate
//! reproduces cobra's behaviour:
//!
//! * command lookup (`Find`, `stripFlags`, `legacyArgs`, suggestions "Did you mean this?")
//! * pflag syntax (`--flag value`, `--flag=value`, `-f value`, `-fvalue`, `--bool=false`, slices, maps,
//!   durations, interspersed positional arguments, `--`), parsed with **clap**
//! * help / usage templates, flag usage alignment, `(default ...)` annotations
//! * error messages (`unknown flag: --foo`, `required flag(s) "x" not set`, `accepts 1 arg(s), received 0`, ...)
//!   followed by the usage, and the `help` command.
//!
//! Commands are declared with [`Command`] and [`Flag`] builders, and implemented by a `fn(&Ctx)`.

mod args;
mod command;
mod ctx;
mod exec;
mod flag;
mod help;
mod parse;
mod spec;
mod tree;

pub use args::ArgsRule;
pub use command::{Command, RunFn};
pub use ctx::Ctx;
pub use exec::{App, execute, execute_with_args};
pub use flag::{Flag, FlagKind};
/// pflag value parsers (Go `strconv` / pflag semantics and error messages).
pub use parse::{parse_go_bool, parse_go_int, parse_go_uint, read_as_csv, string_to_string_pairs};
pub use tree::{Node, NodeId, Tree};

use std::sync::atomic::{AtomicBool, Ordering};

static VERBOSE: AtomicBool = AtomicBool::new(false);

/// Value of the global persistent `--verbose` flag (Go: `variable.Verbose`).
pub fn verbose() -> bool {
    VERBOSE.load(Ordering::Relaxed)
}

pub(crate) fn set_verbose(v: bool) {
    VERBOSE.store(v, Ordering::Relaxed);
}
