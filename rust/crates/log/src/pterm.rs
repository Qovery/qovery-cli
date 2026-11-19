//! Port of the parts of `github.com/pterm/pterm` used by the CLI.
//!
//! pterm calls `color.ForceColor()` at init: its colors are **always** emitted, even when piped.
//! The escape sequences (including the redundant nested ones) are reproduced exactly.

use std::io::Write;

const RESET: &str = "\x1b[0m";

/// gookit `color.RenderCode(code, msg)`.
fn render_code(code: &str, msg: &str) -> String {
    if code.is_empty() {
        return msg.to_string();
    }
    format!("\x1b[{code}m{msg}{RESET}")
}

/// A pterm `Color` (single SGR code).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Color(pub u8);

pub const FG_BLACK: Color = Color(30);
pub const FG_RED: Color = Color(31);
pub const FG_GREEN: Color = Color(32);
pub const FG_YELLOW: Color = Color(33);
pub const FG_BLUE: Color = Color(34);
pub const FG_MAGENTA: Color = Color(35);
pub const FG_CYAN: Color = Color(36);
pub const FG_WHITE: Color = Color(37);
pub const FG_DEFAULT: Color = Color(39);
pub const FG_GRAY: Color = Color(90);
pub const FG_LIGHT_RED: Color = Color(91);
pub const FG_LIGHT_GREEN: Color = Color(92);
pub const FG_LIGHT_YELLOW: Color = Color(93);
pub const FG_LIGHT_BLUE: Color = Color(94);
pub const FG_LIGHT_MAGENTA: Color = Color(95);
pub const FG_LIGHT_CYAN: Color = Color(96);
pub const FG_LIGHT_WHITE: Color = Color(97);
pub const BG_CYAN: Color = Color(46);
pub const BG_LIGHT_RED: Color = Color(101);
pub const BOLD: Color = Color(1);

impl Color {
    fn code(&self) -> String {
        self.0.to_string()
    }

    /// `pterm.FgXxx.Sprint(s)` / `pterm.FgXxx.Sprintf(...)`.
    pub fn sprint(&self, s: impl AsRef<str>) -> String {
        let code = self.code();
        let restore = format!("{RESET}\x1b[{code}m");
        s.as_ref()
            .split('\n')
            .map(|line| render_code(&code, &line.replace(RESET, &restore)))
            .collect::<Vec<_>>()
            .join("\n")
    }
}

/// `pterm.Red(s)` and friends are shorthands of `FgRed.Sprint`.
pub fn red(s: impl AsRef<str>) -> String {
    FG_RED.sprint(s)
}
pub fn gray(s: impl AsRef<str>) -> String {
    FG_GRAY.sprint(s)
}
pub fn blue(s: impl AsRef<str>) -> String {
    FG_BLUE.sprint(s)
}
pub fn green(s: impl AsRef<str>) -> String {
    FG_GREEN.sprint(s)
}
pub fn yellow(s: impl AsRef<str>) -> String {
    FG_YELLOW.sprint(s)
}
pub fn bold(s: impl AsRef<str>) -> String {
    BOLD.sprint(s)
}

/// A pterm `Style` (several SGR codes).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Style(pub Vec<Color>);

impl Style {
    pub fn new(colors: &[Color]) -> Self {
        Style(colors.to_vec())
    }

    fn code(&self) -> String {
        self.0
            .iter()
            .map(|c| c.code())
            .collect::<Vec<_>>()
            .join(";")
    }

    /// `pterm.Style.Sprint(s)`.
    pub fn sprint(&self, s: impl AsRef<str>) -> String {
        let code = self.code();
        let restore = format!("{RESET}\x1b[{code}m");
        let message = s
            .as_ref()
            .split('\n')
            .map(|line| render_code(&code, &line.replace(RESET, &restore)))
            .collect::<Vec<_>>()
            .join("\n");
        render_code(&code, &message)
    }
}

pub fn table_style() -> Style {
    Style::new(&[FG_DEFAULT])
}
pub fn table_header_style() -> Style {
    Style::new(&[FG_LIGHT_CYAN])
}
pub fn table_separator_style() -> Style {
    Style::new(&[FG_GRAY])
}
pub fn section_style() -> Style {
    Style::new(&[BOLD, FG_YELLOW])
}

/// `pterm.DefaultSection` (configurable paddings).
#[derive(Clone, Debug)]
pub struct Section {
    pub top_padding: usize,
    pub bottom_padding: usize,
    pub level: usize,
}

impl Default for Section {
    fn default() -> Self {
        Section {
            top_padding: 1,
            bottom_padding: 1,
            level: 1,
        }
    }
}

impl Section {
    pub fn with_top_padding(mut self, p: usize) -> Self {
        self.top_padding = p;
        self
    }
    pub fn with_bottom_padding(mut self, p: usize) -> Self {
        self.bottom_padding = p;
        self
    }
    pub fn with_level(mut self, l: usize) -> Self {
        self.level = l;
        self
    }

    /// `SectionPrinter.Sprint`.
    pub fn sprint(&self, text: &str) -> String {
        let mut ret = String::new();
        for _ in 0..self.top_padding {
            ret.push('\n');
        }
        if self.level > 0 {
            ret.push_str(&"#".repeat(self.level));
            ret.push(' ');
        }
        ret.push_str(&section_style().sprint(text));
        for _ in 0..self.bottom_padding {
            ret.push('\n');
        }
        ret
    }

    /// `SectionPrinter.Println(text)`: `Sprint(text + "\n")` (no extra newline: pterm's `Sprintln`).
    pub fn println(&self, text: &str) {
        print_stdout(&self.sprint(&format!("{text}\n")));
    }

    /// `SectionPrinter.Printfln(...)`: Sprint(text) followed by a newline.
    pub fn printfln(&self, text: &str) {
        let s = self.sprint(text);
        print_stdout(&format!("{s}\n"));
    }

    /// `SectionPrinter.Print(text)`.
    pub fn print(&self, text: &str) {
        print_stdout(&self.sprint(text));
    }
}

/// pterm prefix printers (`pterm.Info`, `pterm.Error`, `pterm.Warning`, `pterm.Success`).
#[derive(Clone, Debug)]
pub struct PrefixPrinter {
    prefix: &'static str,
    prefix_style: Style,
    message_style: Style,
}

pub fn info_printer() -> PrefixPrinter {
    PrefixPrinter {
        prefix: "INFO",
        prefix_style: Style::new(&[FG_BLACK, BG_CYAN]),
        message_style: Style::new(&[FG_LIGHT_CYAN]),
    }
}
pub fn error_printer() -> PrefixPrinter {
    PrefixPrinter {
        prefix: " ERROR ",
        prefix_style: Style::new(&[FG_BLACK, BG_LIGHT_RED]),
        message_style: Style::new(&[FG_LIGHT_RED]),
    }
}
pub fn warning_printer() -> PrefixPrinter {
    PrefixPrinter {
        prefix: "WARNING",
        prefix_style: Style::new(&[FG_BLACK, Color(103)]),
        message_style: Style::new(&[FG_YELLOW]),
    }
}
pub fn success_printer() -> PrefixPrinter {
    PrefixPrinter {
        prefix: "SUCCESS",
        prefix_style: Style::new(&[FG_BLACK, Color(102)]),
        message_style: Style::new(&[FG_GREEN]),
    }
}

impl PrefixPrinter {
    /// `PrefixPrinter.Sprint` for a single-line message.
    pub fn sprint(&self, msg: &str) -> String {
        format!(
            "{} {}",
            self.prefix_style.sprint(format!(" {} ", self.prefix)),
            self.message_style.sprint(msg)
        )
    }
    /// `PrefixPrinter.Println`.
    pub fn println(&self, msg: &str) {
        print_stdout(&format!("{}\n", self.sprint(msg)));
    }
}

fn print_stdout(s: &str) {
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(s.as_bytes());
    let _ = out.flush();
}

/// Strip ANSI escape sequences (pterm `RemoveColorFromString`).
pub fn remove_color(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\x1b' && chars.peek() == Some(&'[') {
            chars.next();
            for c2 in chars.by_ref() {
                if c2.is_ascii_alphabetic() {
                    break;
                }
            }
        } else {
            out.push(c);
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn colors_match_go() {
        assert_eq!(FG_BLUE.sprint("blue"), "\x1b[34mblue\x1b[0m");
        assert_eq!(gray("gray"), "\x1b[90mgray\x1b[0m");
        assert_eq!(
            error_printer().sprint("errfmt 3"),
            "\x1b[30;101m\x1b[30;101m  ERROR  \x1b[0m\x1b[0m \x1b[91m\x1b[91merrfmt 3\x1b[0m\x1b[0m"
        );
        assert_eq!(
            info_printer().sprint("info line"),
            "\x1b[30;46m\x1b[30;46m INFO \x1b[0m\x1b[0m \x1b[96m\x1b[96minfo line\x1b[0m\x1b[0m"
        );
    }

    #[test]
    fn section_matches_go() {
        let s = Section::default()
            .with_top_padding(1)
            .with_bottom_padding(0);
        assert_eq!(
            format!("{}\n", s.sprint("Section A\n")),
            "\n# \x1b[1;33m\x1b[1;33mSection A\x1b[0m\n\x1b[1;33m\x1b[0m\x1b[0m\n"
        );
        assert_eq!(
            format!("{}\n", Section::default().sprint("Section B")),
            "\n# \x1b[1;33m\x1b[1;33mSection B\x1b[0m\x1b[0m\n\n"
        );
    }
}
