//! promptui (v0.9.0) `Prompt` when stdin is NOT a terminal: byte-identical rendering and stdin consumption
//! (see [`crate::readline`] for how keys are read from a pipe).
//!
//! After every key the line `<label template><input with █ cursor>` is redrawn (plus the `>> <error>`
//! validation line after a rejected Enter); on success it is replaced by `<success template><input>`,
//! on error (`^D` on EOF / Ctrl-D, `^C`) it is erased.

use crate::readline::{
    CHAR_BACKSPACE, CHAR_BACKWARD, CHAR_CTRL_H, CHAR_ENTER, CHAR_FORWARD, Cursor, HIDE_CURSOR,
    KeyOutcome, OpEvent, RawStdin, SHOW_CURSOR, ScreenBuf, Terminal, op_read_key, styler,
};
use std::io::{Read, Write};

/// Which promptui templates the prompt uses.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PromptTemplates {
    /// promptui's defaults: `✔ Label: ` (bold) while typing, `✗ Label: ` when invalid, `Label: ` (faint) at the end.
    Default,
    /// `Templates{Prompt, Valid, Invalid, Success: "{{ . }}"}`: the label as is (`utils.Validate`).
    Plain,
}

/// `Validate` function: `Some(message)` when the input is invalid.
pub type Validator<'a> = &'a dyn Fn(&str) -> Option<String>;

/// promptui `Prompt` options supported by the emulation.
pub struct PromptOptions<'a> {
    pub label: &'a str,
    /// `Default` (erased by the first typed key, `AllowEdit` false).
    pub default: &'a str,
    pub templates: PromptTemplates,
    /// `Validate`: `Some(message)` when the input is invalid.
    pub validate: Option<Validator<'a>>,
}

fn bold(s: &str) -> String {
    styler("1", s)
}

impl PromptOptions<'_> {
    fn valid(&self) -> String {
        match self.templates {
            PromptTemplates::Plain => self.label.to_string(),
            PromptTemplates::Default => format!(
                "{} {}{} ",
                bold("\x1b[32m✔\x1b[0m"),
                bold(self.label),
                bold(":")
            ),
        }
    }

    fn invalid(&self) -> String {
        match self.templates {
            PromptTemplates::Plain => self.label.to_string(),
            PromptTemplates::Default => format!(
                "{} {}{} ",
                bold("\x1b[31m✗\x1b[0m"),
                bold(self.label),
                bold(":")
            ),
        }
    }

    fn success(&self) -> String {
        match self.templates {
            PromptTemplates::Plain => self.label.to_string(),
            PromptTemplates::Default => format!("{}{} ", styler("2", self.label), styler("2", ":")),
        }
    }

    fn check(&self, s: &str) -> Option<String> {
        self.validate.and_then(|v| v(s))
    }
}

/// promptui `Prompt.Run()` reading keys from `input` (a non-terminal) and rendering on `out`.
/// Returns the typed text, or the promptui error (`^D`, `^C`).
pub fn run_prompt_piped(
    opts: &PromptOptions,
    input: &mut dyn Read,
    out: &mut dyn Write,
) -> Result<String, String> {
    let mut term = Terminal::new(input);
    let _ = out.write_all(HIDE_CURSOR.as_bytes());
    let mut sb = ScreenBuf::new();
    let mut cur = Cursor::new(opts.default, !opts.default.is_empty());
    let mut input_err: Option<String> = None;

    // the listener promptui gives to readline
    let mut listen = |line: Option<&str>,
                      key: i32,
                      cur: &mut Cursor,
                      input_err: &mut Option<String>,
                      out: &mut dyn Write| {
        // Cursor.Listen
        if let Some(line) = line {
            cur.update(line);
        }
        match key {
            0 | CHAR_ENTER => {}
            CHAR_BACKSPACE | CHAR_CTRL_H => {
                if cur.erase {
                    cur.erase = false;
                    cur.replace("");
                }
                cur.backspace();
            }
            CHAR_FORWARD => {
                cur.erase = false;
                cur.move_by(1);
            }
            CHAR_BACKWARD => cur.move_by(-1),
            k => {
                if cur.erase {
                    cur.erase = false;
                    cur.replace("");
                    cur.update(&crate::readline::rune_char(k).to_string());
                }
            }
        }
        let prompt = if opts.check(&cur.get()).is_some() {
            opts.invalid()
        } else {
            opts.valid()
        };
        sb.reset();
        sb.write(&format!("{prompt}{}", cur.format()));
        if let Some(e) = input_err.take() {
            sb.write(&format!("\x1b[31m>>\x1b[0m \x1b[31m{e}\x1b[0m"));
        }
        sb.flush(out);
    };

    let err = loop {
        // rl.Readline(): the listener is called once with no key, then once per key
        listen(None, 0, &mut cur, &mut input_err, out);
        let res = loop {
            match op_read_key(&mut term, out) {
                OpEvent::Eof => break Err("^D"),
                OpEvent::Key { key, line, outcome } => {
                    listen(Some(&line), key, &mut cur, &mut input_err, out);
                    match outcome {
                        KeyOutcome::Continue => {}
                        KeyOutcome::Line => break Ok(()),
                        KeyOutcome::Eof => break Err("^D"),
                        KeyOutcome::Interrupt => break Err("^C"),
                    }
                }
            }
        };
        input_err = opts.check(&cur.get());
        if input_err.is_none() {
            break res.err();
        }
        if let Err(e) = res {
            break Some(e);
        }
    };

    if let Some(e) = err {
        sb.reset();
        sb.write("");
        sb.flush(out);
        let _ = out.write_all(SHOW_CURSOR.as_bytes());
        let _ = out.flush();
        return Err(e.to_string());
    }
    let value = cur.get();
    sb.reset();
    sb.write(&format!("{}{value}", opts.success()));
    sb.flush(out);
    let _ = out.write_all(SHOW_CURSOR.as_bytes());
    let _ = out.flush();
    Ok(value)
}

/// [`run_prompt_piped`] on the process stdin (read unbuffered, like Go) and stdout.
pub fn prompt_piped(opts: &PromptOptions) -> Result<String, String> {
    let mut stdin = RawStdin;
    let mut out = std::io::stdout().lock();
    run_prompt_piped(opts, &mut stdin, &mut out)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn run(opts: &PromptOptions, stdin: &str) -> (Result<String, String>, String) {
        let mut out = Vec::new();
        let mut input = stdin.as_bytes();
        let r = run_prompt_piped(opts, &mut input, &mut out);
        (r, String::from_utf8(out).unwrap())
    }

    #[test]
    fn default_templates() {
        let opts = PromptOptions {
            label: "Token name",
            default: "",
            templates: PromptTemplates::Default,
            validate: None,
        };
        let (r, out) = run(&opts, "ab\n");
        assert_eq!(r.unwrap(), "ab");
        let p = "\x1b[1m\x1b[32m✔\x1b[0m \x1b[1mToken name\x1b[0m\x1b[1m:\x1b[0m ";
        let up = "\x1b[1A\x1b[2K\r";
        assert_eq!(
            out,
            format!(
                "\x1b[?25l\x1b[2K\r{p}█\n{up}\x1b[2K\r{p}a█\n{up}\x1b[2K\r{p}ab█\n{up}\x1b[2K\r{p}ab█\n\
                 {up}\x1b[2K\r\x1b[2mToken name\x1b[0m\x1b[2m:\x1b[0m ab\n\x1b[?25h"
            )
        );
        // EOF without newline
        let (r, _) = run(&opts, "ab");
        assert_eq!(r.unwrap_err(), "^D");
        let (r, _) = run(&opts, "\n");
        assert_eq!(r.unwrap(), "");
    }

    #[test]
    fn editing_keys() {
        let opts = PromptOptions {
            label: "L",
            default: "",
            templates: PromptTemplates::Plain,
            validate: None,
        };
        let (r, out) = run(&opts, "abc\x1b[D\x1b[D\x7fx\x1b[C\x7f\r");
        assert_eq!(r.unwrap(), "xc");
        assert!(out.contains("La█c\n"));
        assert_eq!(out.matches('\x07').count(), 2);
        let (r, _) = run(&opts, "ab\x03");
        assert_eq!(r.unwrap_err(), "^C");
    }

    #[test]
    fn default_value_erased_by_typing() {
        let opts = PromptOptions {
            label: "L",
            default: "def",
            templates: PromptTemplates::Plain,
            validate: None,
        };
        let (r, out) = run(&opts, "x\n");
        assert_eq!(r.unwrap(), "x");
        assert!(out.contains("L█ef\n"));
        let (r, _) = run(&opts, "\r");
        assert_eq!(r.unwrap(), "def");
        // `\n` is not promptui's Enter key: it replaces the default value before ending the line
        let (r, _) = run(&opts, "\n");
        assert_eq!(r.unwrap(), "\n");
        let (r, _) = run(&opts, "\x1b[Cx\n");
        assert_eq!(r.unwrap(), "dxef");
    }
}
