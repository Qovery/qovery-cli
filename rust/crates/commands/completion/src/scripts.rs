//! Shell completion scripts of cobra v1.10.2 (`GenBashCompletionV2`, `GenZshCompletion[NoDesc]`,
//! `GenFishCompletion`, `GenPowerShellCompletion[WithDesc]`).
//!
//! The templates in `templates/*.tmpl` are the Go `fmt.Sprintf` format strings copied verbatim from cobra's
//! sources (`bash_completionsV2.go`, `zsh_completions.go`, `fish_completions.go`, `powershell_completions.go`);
//! they are rendered with a minimal `fmt.Sprintf` supporting the verbs they use.

use crate::complete::{
    ACTIVE_HELP_MARKER, DIRECTIVE_ERROR, DIRECTIVE_FILTER_DIRS, DIRECTIVE_FILTER_FILE_EXT,
    DIRECTIVE_KEEP_ORDER, DIRECTIVE_NO_FILE_COMP, DIRECTIVE_NO_SPACE,
    SHELL_COMP_NO_DESC_REQUEST_CMD, SHELL_COMP_REQUEST_CMD, config_env_var,
};

const BASH: &str = include_str!("templates/bash.tmpl");
const ZSH: &str = include_str!("templates/zsh.tmpl");
const FISH: &str = include_str!("templates/fish.tmpl");
const POWERSHELL: &str = include_str!("templates/powershell.tmpl");

/// A `fmt.Sprintf` argument.
enum Arg<'a> {
    S(&'a str),
    D(i64),
}

impl Arg<'_> {
    fn render(&self) -> String {
        match self {
            Arg::S(s) => s.to_string(),
            Arg::D(d) => d.to_string(),
        }
    }
}

/// Minimal Go `fmt.Sprintf`: `%%`, `%s`, `%d`, explicit indexes (`%[2]s`) and a `-`/width (`%-36[1]s`).
fn sprintf(format: &str, args: &[Arg]) -> String {
    let mut out = String::with_capacity(format.len() + 1024);
    let mut chars = format.chars().peekable();
    let mut next_arg = 0usize;
    while let Some(c) = chars.next() {
        if c != '%' {
            out.push(c);
            continue;
        }
        if chars.peek() == Some(&'%') {
            chars.next();
            out.push('%');
            continue;
        }
        let mut left = false;
        let mut width = 0usize;
        if chars.peek() == Some(&'-') {
            chars.next();
            left = true;
        }
        while let Some(d) = chars.peek().and_then(|c| c.to_digit(10)) {
            width = width * 10 + d as usize;
            chars.next();
        }
        if chars.peek() == Some(&'[') {
            chars.next();
            let mut idx = 0usize;
            while let Some(d) = chars.peek().and_then(|c| c.to_digit(10)) {
                idx = idx * 10 + d as usize;
                chars.next();
            }
            chars.next(); // ']'
            next_arg = idx - 1;
        }
        let verb = chars.next().unwrap_or('%');
        let s = match args.get(next_arg) {
            Some(a) => a.render(),
            None => format!("%!{verb}(MISSING)"),
        };
        let _ = verb;
        next_arg += 1;
        let len = s.chars().count();
        if len < width {
            let pad = " ".repeat(width - len);
            if left {
                out.push_str(&s);
                out.push_str(&pad);
            } else {
                out.push_str(&pad);
                out.push_str(&s);
            }
        } else {
            out.push_str(&s);
        }
    }
    out
}

fn comp_cmd(include_desc: bool) -> &'static str {
    if include_desc {
        SHELL_COMP_REQUEST_CMD
    } else {
        SHELL_COMP_NO_DESC_REQUEST_CMD
    }
}

/// Variables should not contain a '-' or ':' character.
fn name_for_var(name: &str) -> String {
    name.replace(['-', ':'], "_")
}

fn directives() -> [Arg<'static>; 6] {
    [
        Arg::D(DIRECTIVE_ERROR),
        Arg::D(DIRECTIVE_NO_SPACE),
        Arg::D(DIRECTIVE_NO_FILE_COMP),
        Arg::D(DIRECTIVE_FILTER_FILE_EXT),
        Arg::D(DIRECTIVE_FILTER_DIRS),
        Arg::D(DIRECTIVE_KEEP_ORDER),
    ]
}

/// cobra `genBashComp`.
pub fn bash(name: &str, include_desc: bool) -> String {
    let mut args = vec![Arg::S(name), Arg::S(comp_cmd(include_desc))];
    args.extend(directives());
    args.push(Arg::S(ACTIVE_HELP_MARKER));
    sprintf(BASH, &args)
}

/// cobra `genZshComp`.
pub fn zsh(name: &str, include_desc: bool) -> String {
    let mut args = vec![Arg::S(name), Arg::S(comp_cmd(include_desc))];
    args.extend(directives());
    args.push(Arg::S(ACTIVE_HELP_MARKER));
    sprintf(ZSH, &args)
}

/// cobra `genFishComp`.
pub fn fish(name: &str, include_desc: bool) -> String {
    let var = name_for_var(name);
    let active_help = config_env_var(name, "ACTIVE_HELP");
    let mut out = sprintf(
        "# fish completion for %-36s -*- shell-script -*-\n",
        &[Arg::S(name)],
    );
    let mut args = vec![Arg::S(&var), Arg::S(name), Arg::S(comp_cmd(include_desc))];
    args.extend(directives());
    args.push(Arg::S(&active_help));
    out.push_str(&sprintf(FISH, &args));
    out
}

/// cobra `genPowerShellComp`.
pub fn powershell(name: &str, include_desc: bool) -> String {
    let var = name_for_var(name);
    let active_help = config_env_var(name, "ACTIVE_HELP");
    let mut args = vec![Arg::S(name), Arg::S(&var), Arg::S(comp_cmd(include_desc))];
    args.extend(directives());
    args.push(Arg::S(&active_help));
    sprintf(POWERSHELL, &args)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sprintf_verbs() {
        assert_eq!(
            sprintf("a %% %[2]s %[1]d %-5[2]s|", &[Arg::D(3), Arg::S("x")]),
            "a % x 3 x    |"
        );
        assert_eq!(sprintf("%s-%s", &[Arg::S("a"), Arg::S("b")]), "a-b");
    }

    #[test]
    fn headers() {
        assert!(bash("qovery", true).starts_with(
            "# bash completion V2 for qovery                               -*- shell-script -*-\n"
        ));
        assert!(bash("qovery", true).contains("requestComp=\"${words[0]} __complete ${args[*]}\""));
        assert!(
            bash("qovery", false)
                .contains("requestComp=\"${words[0]} __completeNoDesc ${args[*]}\"")
        );
        assert!(fish("qovery", true).contains(
            "set -l requestComp \"QOVERY_ACTIVE_HELP=0 $args[1] __complete $args[2..-1] $lastArg\""
        ));
    }
}
