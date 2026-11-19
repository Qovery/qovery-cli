//! Port of `github.com/fatih/color`.
//!
//! fatih/color disables colors globally when `NO_COLOR` is set, when `TERM=dumb`, or when *stdout* is
//! not a terminal (even for strings that end up on stderr). `color.RedString("x")` => `"\x1b[31mx\x1b[0m"`.

use is_terminal::IsTerminal;
use std::sync::OnceLock;

/// Mirrors `color.NoColor`.
pub fn no_color() -> bool {
    static NO_COLOR: OnceLock<bool> = OnceLock::new();
    *NO_COLOR.get_or_init(|| {
        std::env::var_os("NO_COLOR").is_some_and(|v| !v.is_empty())
            || std::env::var("TERM").is_ok_and(|t| t == "dumb")
            || !std::io::stdout().is_terminal()
    })
}

/// SGR attributes used by the CLI.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Attr {
    Bold = 1,
    FgBlack = 30,
    FgRed = 31,
    FgGreen = 32,
    FgYellow = 33,
    FgBlue = 34,
    FgMagenta = 35,
    FgCyan = 36,
    FgWhite = 37,
    FgHiBlack = 90,
    FgHiRed = 91,
    FgHiGreen = 92,
    FgHiYellow = 93,
    FgHiBlue = 94,
    FgHiMagenta = 95,
    FgHiCyan = 96,
    FgHiWhite = 97,
}

/// `color.New(attrs...).Sprint(s)` honouring `color.NoColor`.
pub fn sprint(attrs: &[Attr], s: &str) -> String {
    if no_color() {
        return s.to_string();
    }
    force_sprint(attrs, s)
}

/// `color.New(attrs...).Sprint(s)` with colors forced on (`c.EnableColor()`).
pub fn force_sprint(attrs: &[Attr], s: &str) -> String {
    let codes: Vec<String> = attrs.iter().map(|a| (*a as u8).to_string()).collect();
    format!("\x1b[{}m{}\x1b[0m", codes.join(";"), s)
}

pub fn red(s: &str) -> String {
    sprint(&[Attr::FgRed], s)
}
pub fn green(s: &str) -> String {
    sprint(&[Attr::FgGreen], s)
}
pub fn yellow(s: &str) -> String {
    sprint(&[Attr::FgYellow], s)
}
pub fn blue(s: &str) -> String {
    sprint(&[Attr::FgBlue], s)
}
pub fn cyan(s: &str) -> String {
    sprint(&[Attr::FgCyan], s)
}
pub fn magenta(s: &str) -> String {
    sprint(&[Attr::FgMagenta], s)
}
pub fn white(s: &str) -> String {
    sprint(&[Attr::FgWhite], s)
}
pub fn hi_black(s: &str) -> String {
    sprint(&[Attr::FgHiBlack], s)
}
pub fn bold(s: &str) -> String {
    sprint(&[Attr::Bold], s)
}
