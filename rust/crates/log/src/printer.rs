//! Port of `utils/printer.go`.

use crate::fatih::{self, Attr};
use is_terminal::IsTerminal;
use std::fmt::Display;
use std::io::Write;

/// `utils.PrintlnError(err)`: `Error: <err>` on **stdout**, "Error" in red when stdout is a tty.
pub fn println_error(err: impl Display) {
    let mut out = std::io::stdout().lock();
    let _ = writeln!(out, "{}: {}", fatih::red("Error"), err);
    let _ = out.flush();
}

/// `utils.PrintlnInfo(info)`: `Info: <info>` on **stdout**, "Info" in cyan when stdout is a tty.
pub fn println_info(info: impl Display) {
    let mut out = std::io::stdout().lock();
    let _ = writeln!(out, "{}: {}", fatih::cyan("Info"), info);
    let _ = out.flush();
}

/// `utils.Println(text)`.
pub fn println(text: impl Display) {
    let mut out = std::io::stdout().lock();
    let _ = writeln!(out, "{text}");
    let _ = out.flush();
}

fn stderr_colored(attr: Attr, s: &str) -> String {
    let enabled = std::env::var("NO_COLOR")
        .map(|v| v.is_empty())
        .unwrap_or(true)
        && std::env::var("TERM").map(|t| t != "dumb").unwrap_or(true)
        && std::io::stderr().is_terminal();
    if enabled {
        fatih::force_sprint(&[attr], s)
    } else {
        s.to_string()
    }
}

/// `utils.PrintlnErrorToStderr(err)`.
pub fn println_error_to_stderr(err: impl Display) {
    let mut out = std::io::stderr().lock();
    let _ = writeln!(out, "{}: {}", stderr_colored(Attr::FgRed, "Error"), err);
}

/// `utils.PrintlnInfoToStderr(info)`.
pub fn println_info_to_stderr(info: impl Display) {
    let mut out = std::io::stderr().lock();
    let _ = writeln!(out, "{}: {}", stderr_colored(Attr::FgCyan, "Info"), info);
}

/// `utils.DryRunPrint(dryRunDisabled)`.
pub fn dry_run_print(dry_run_disabled: bool) {
    let message = if dry_run_disabled {
        fatih::red("disabled")
    } else {
        fatih::green("enabled")
    };
    crate::logrus::info(format!("Dry run: {message}"));
}

/// `utils.PrintTable(headers, data)`: pterm table with header row, printed on stdout.
pub fn print_table(headers: &[&str], data: &[Vec<String>]) {
    let mut rows: Vec<Vec<String>> = Vec::with_capacity(data.len() + 1);
    rows.push(headers.iter().map(|h| h.to_string()).collect());
    rows.extend(data.iter().cloned());
    crate::table::Table::new(rows).with_header(true).render();
}

/// Go's `println()` builtin writes to **stderr**.
pub fn go_builtin_println(text: impl Display) {
    let _ = writeln!(std::io::stderr(), "{text}");
}

/// Flush stdout (useful before `std::process::exit`).
pub fn flush() {
    let _ = std::io::stdout().flush();
    let _ = std::io::stderr().flush();
}
