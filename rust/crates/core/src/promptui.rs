//! promptui `Prompt` emulation of `utils.Validate` when stdin is not a terminal (ported from the admin crate).
//!
//! On a terminal `qovery_core::select::validate` uses inquire. When stdin is NOT a terminal (scripts,
//! `echo yes | qovery admin ...`, the parity harness), promptui/readline read keys from stdin and re-render
//! the prompt on stdout after each key: [`crate::promptui_prompt`] reproduces those bytes exactly, so the
//! non-interactive behaviour (output, accepted answers, `Prompt failed ^D` on EOF) is identical to Go.

use std::io::{Read, Write};

/// promptui `Prompt.Run()` with `Templates{Prompt,Valid,Invalid,Success: "{{ . }}"}` and the
/// "no input" validation of `utils.getInput`, on a non-terminal stdin
/// (see [`crate::promptui_prompt::run_prompt_piped`]).
pub fn run_piped(
    label: &str,
    input: &mut impl Read,
    out: &mut impl Write,
) -> Result<String, String> {
    let validate = |s: &str| {
        if s.is_empty() {
            Some("no input".to_string())
        } else {
            None
        }
    };
    let opts = crate::promptui_prompt::PromptOptions {
        label,
        default: "",
        templates: crate::promptui_prompt::PromptTemplates::Plain,
        validate: Some(&validate),
    };
    crate::promptui_prompt::run_prompt_piped(&opts, input, out)
}

/// promptui `Prompt.Run()` on a non-terminal stdin, rendering on stdout. Returns the typed line.
pub fn prompt_piped(label: &str) -> Result<String, String> {
    let mut stdin = crate::readline::RawStdin;
    let mut out = std::io::stdout().lock();
    let res = run_piped(label, &mut stdin, &mut out);
    let _ = out.flush();
    res
}

#[cfg(test)]
mod tests {
    use super::*;

    fn run(stdin: &str) -> (Result<String, String>, String) {
        let mut out = Vec::new();
        let r = run_piped("L: ", &mut stdin.as_bytes(), &mut out);
        (r, String::from_utf8(out).unwrap())
    }

    #[test]
    fn accepted() {
        let (r, out) = run("ye\nfoo");
        assert_eq!(r.unwrap(), "ye");
        assert_eq!(
            out,
            "\x1b[?25l\x1b[2K\rL: █\n\x1b[1A\x1b[2K\r\x1b[2K\rL: y█\n\x1b[1A\x1b[2K\r\x1b[2K\rL: ye█\n\
             \x1b[1A\x1b[2K\r\x1b[2K\rL: ye█\n\x1b[1A\x1b[2K\r\x1b[2K\rL: ye\n\x1b[?25h"
        );
    }

    #[test]
    fn eof() {
        let (r, out) = run("");
        assert_eq!(r.unwrap_err(), "^D");
        assert_eq!(
            out,
            "\x1b[?25l\x1b[2K\rL: █\n\x1b[1A\x1b[2K\r\x1b[2K\r\n\x1b[?25h"
        );
    }

    #[test]
    fn empty_line_then_eof() {
        let (r, out) = run("\n");
        assert_eq!(r.unwrap_err(), "^D");
        assert_eq!(
            out,
            "\x1b[?25l\x1b[2K\rL: █\n\x1b[1A\x1b[2K\r\x1b[2K\rL: █\n\x1b[1A\x1b[2K\r\x1b[2K\rL: █\n\
             \x1b[2K\r\x1b[31m>>\x1b[0m \x1b[31mno input\x1b[0m\n\x1b[1A\x1b[2K\r\x1b[1A\x1b[2K\r\x1b[2K\r\n\x1b[?25h"
        );
    }
}
