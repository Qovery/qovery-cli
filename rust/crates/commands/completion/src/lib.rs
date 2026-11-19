//! `qovery completion` (cobra v1.10.2's default completion command, `InitDefaultCompletionCmd`) and the hidden
//! `__complete` / `__completeNoDesc` command (`initCompleteCmd`) used by the generated shell scripts.

mod complete;
mod pflag;
mod scripts;

use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use std::io::Write;

pub(crate) const COMPLETION_CMD_NAME: &str = "completion";
const NO_DESC_FLAG_NAME: &str = "no-descriptions";
const NO_DESC_FLAG_DESC: &str = "disable completion descriptions";

/// Commands added to the root by this crate: `completion`, plus `__complete` when it is the command being
/// called (like cobra, which only adds it in that case so that it never shows up anywhere else).
pub fn commands() -> Vec<Command> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    commands_for_args(&args)
}

/// [`commands`] for explicit process args (without the binary name).
pub fn commands_for_args(args: &[String]) -> Vec<Command> {
    let mut v = vec![completion_command()];
    if complete::is_called(args) {
        v.push(complete::command());
    }
    v
}

fn root_name(ctx: &Ctx) -> String {
    ctx.tree().root().name().to_string()
}

fn no_desc_flag() -> Flag {
    Flag::bool(NO_DESC_FLAG_NAME, NO_DESC_FLAG_DESC)
}

const SHORT_DESC: &str = "Generate the autocompletion script for";

/// The root name appears in the long descriptions (`c.Root().Name()`), always `qovery`.
const ROOT: &str = "qovery";

fn completion_command() -> Command {
    let bash = Command::new("bash")
        .short(&format!("{SHORT_DESC} bash"))
        .long(&format!(
            r#"Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <({ROOT} completion bash)

To load completions for every new session, execute once:

#### Linux:

	{ROOT} completion bash > /etc/bash_completion.d/{ROOT}

#### macOS:

	{ROOT} completion bash > $(brew --prefix)/etc/bash_completion.d/{ROOT}

You will need to start a new shell for this setup to take effect.
"#
        ))
        .args(ArgsRule::NoArgs)
        .disable_flags_in_use_line()
        .flag(no_desc_flag())
        .run(run_bash);

    let zsh = Command::new("zsh")
        .short(&format!("{SHORT_DESC} zsh"))
        .long(&format!(
            r#"Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <({ROOT} completion zsh)

To load completions for every new session, execute once:

#### Linux:

	{ROOT} completion zsh > "${{fpath[1]}}/_{ROOT}"

#### macOS:

	{ROOT} completion zsh > $(brew --prefix)/share/zsh/site-functions/_{ROOT}

You will need to start a new shell for this setup to take effect.
"#
        ))
        .args(ArgsRule::NoArgs)
        .flag(no_desc_flag())
        .run(run_zsh);

    let fish = Command::new("fish")
        .short(&format!("{SHORT_DESC} fish"))
        .long(&format!(
            r#"Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	{ROOT} completion fish | source

To load completions for every new session, execute once:

	{ROOT} completion fish > ~/.config/fish/completions/{ROOT}.fish

You will need to start a new shell for this setup to take effect.
"#
        ))
        .args(ArgsRule::NoArgs)
        .flag(no_desc_flag())
        .run(run_fish);

    let powershell = Command::new("powershell")
        .short(&format!("{SHORT_DESC} powershell"))
        .long(&format!(
            r#"Generate the autocompletion script for powershell.

To load completions in your current shell session:

	{ROOT} completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.
"#
        ))
        .args(ArgsRule::NoArgs)
        .flag(no_desc_flag())
        .run(run_powershell);

    Command::new(COMPLETION_CMD_NAME)
        .short("Generate the autocompletion script for the specified shell")
        .long(&format!(
            "Generate the autocompletion script for {ROOT} for the specified shell.\nSee each sub-command's help for details on how to use the generated script.\n"
        ))
        .args(ArgsRule::NoArgs)
        .subcommands([bash, zsh, fish, powershell])
}

fn write_out(s: &str) {
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(s.as_bytes());
    let _ = out.flush();
}

fn run_bash(ctx: &Ctx) {
    write_out(&scripts::bash(
        &root_name(ctx),
        !ctx.bool(NO_DESC_FLAG_NAME),
    ));
}

fn run_zsh(ctx: &Ctx) {
    write_out(&scripts::zsh(&root_name(ctx), !ctx.bool(NO_DESC_FLAG_NAME)));
}

fn run_fish(ctx: &Ctx) {
    write_out(&scripts::fish(
        &root_name(ctx),
        !ctx.bool(NO_DESC_FLAG_NAME),
    ));
}

fn run_powershell(ctx: &Ctx) {
    write_out(&scripts::powershell(
        &root_name(ctx),
        !ctx.bool(NO_DESC_FLAG_NAME),
    ));
}
