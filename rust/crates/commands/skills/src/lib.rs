//! `qovery skills` command. Port of `cmd/skills.go` and `pkg/skills/*.go`.

use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::{exit_with_error, telemetry};

pub mod installer;
pub mod offer;

pub use installer::{Installation, MANUAL_INSTALL_COMMAND, find_installations, install, uninstall};

/// The `skills` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("skills")
        .short("Manage the Qovery skills for AI coding agents")
        .long("Manage the Qovery skills, which teach AI coding agents (Claude Code, Cursor, OpenCode,\nGemini CLI and other tools supporting Agent Skills) to deploy, troubleshoot and optimize\napplications on Qovery.\n\nSource: https://github.com/Qovery/qovery-skills")
        .run(run_skills)
        .subcommand(
            Command::new("install")
                .args(ArgsRule::NoArgs)
                .short("Install or update the Qovery skills")
                .long("Install the Qovery skills, globally by default or in the current project with --project.\nRun it again to update them to the latest version.")
                .flag(Flag::bool("project", "Install in the current project only"))
                .run(run_install),
        )
        .subcommand(
            Command::new("status")
                .args(ArgsRule::NoArgs)
                .short("Show where the Qovery skills are installed")
                .run(run_status),
        )
        .subcommand(
            Command::new("uninstall")
                .args(ArgsRule::NoArgs)
                .short("Remove the Qovery skills from every location")
                .run(run_uninstall),
        )
}

fn run_skills(ctx: &Ctx) {
    telemetry::capture(ctx);
    ctx.help();
}

fn run_install(ctx: &Ctx) {
    telemetry::capture(ctx);
    if let Err(e) = install(ctx.bool("project")) {
        exit_with_error(e);
    }
}

fn run_status(ctx: &Ctx) {
    telemetry::capture(ctx);
    let installations = find_installations();
    if installations.is_empty() {
        println!("The Qovery skills are not installed. Install them with 'qovery skills install'.");
        return;
    }
    for installation in installations {
        println!(
            "{} (version {})",
            installation.path.display(),
            installation.version
        );
    }
}

fn run_uninstall(ctx: &Ctx) {
    telemetry::capture(ctx);
    if let Err(e) = uninstall() {
        exit_with_error(e);
    }
}
