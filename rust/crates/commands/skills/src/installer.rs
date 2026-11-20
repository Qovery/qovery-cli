//! Port of `pkg/skills/skills.go`: locate, install and uninstall the Qovery skills.

use std::io::Write;
use std::path::{Path, PathBuf};
use std::time::Duration;

/// `InstallScriptURL`: the official installer documented in https://github.com/Qovery/qovery-skills
pub const INSTALL_SCRIPT_URL: &str =
    "https://raw.githubusercontent.com/Qovery/qovery-skills/main/install.sh";
/// `ManualInstallCommand`: shown to users who cannot run the installer from the CLI.
pub const MANUAL_INSTALL_COMMAND: &str = "curl -fsSL https://skill.qovery.com/install.sh | bash";

/// Installed with every other Qovery skill: finding it is enough to know the skills are there.
pub(crate) const ROUTER_SKILL: &str = "qovery";
pub(crate) const VERSION_FILE: &str = "_version.txt";

const MAX_INSTALLER_SIZE: usize = 1 << 20;

/// One directory where the Qovery skills are installed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Installation {
    pub path: PathBuf,
    pub version: String,
}

/// `installBaseDirs`: the global and project locations written by install.sh.
fn install_base_dirs(home: &Path, cwd: &Path) -> Vec<PathBuf> {
    vec![
        home.join(".claude").join("skills"),
        home.join(".config").join("opencode").join("skills"),
        home.join(".agents").join("skills"),
        cwd.join(".claude").join("skills"),
        cwd.join(".opencode").join("skills"),
        cwd.join(".agents").join("skills"),
    ]
}

/// Go `filepath.Join` cleans the path: emulate it for the paths compared / printed.
fn clean(p: PathBuf) -> PathBuf {
    let mut out = PathBuf::new();
    for c in p.components() {
        match c {
            std::path::Component::CurDir => {}
            std::path::Component::ParentDir => {
                if !out.pop() {
                    out.push("..");
                }
            }
            other => out.push(other.as_os_str()),
        }
    }
    out
}

pub(crate) fn find_installations_in(home: &Path, cwd: &Path) -> Vec<Installation> {
    let mut installations = Vec::new();
    let mut seen = std::collections::HashSet::new();
    for base in install_base_dirs(home, cwd) {
        let dir = clean(base.join(ROUTER_SKILL));
        // Running from the home directory makes the project locations the same as the global ones
        if !seen.insert(dir.clone()) {
            continue;
        }
        if std::fs::metadata(dir.join("SKILL.md")).is_err() {
            continue;
        }
        let version = match std::fs::read(dir.join(VERSION_FILE)) {
            Ok(content) if !String::from_utf8_lossy(&content).trim().is_empty() => {
                String::from_utf8_lossy(&content).trim().to_string()
            }
            _ => "unknown".to_string(),
        };
        installations.push(Installation { path: dir, version });
    }
    installations
}

/// Go `os.UserHomeDir()` (`$HOME`, `%USERPROFILE%` on Windows); the callers ignore its error.
pub(crate) fn user_home_dir() -> Option<PathBuf> {
    qovery_core::gosys::user_home_dir().ok()
}

/// `FindInstallations`: every location where the Qovery skills are installed, globally or in the current project.
pub fn find_installations() -> Vec<Installation> {
    let Some(home) = user_home_dir() else {
        return Vec::new();
    };
    let cwd = std::env::current_dir().unwrap_or_else(|_| home.clone());
    find_installations_in(&home, &cwd)
}

/// The AI coding tools that load skills from the locations install.sh writes to.
const AGENT_TOOLS: &[(&str, &str)] = &[
    ("Claude Code", ".claude"),
    ("Cursor", ".cursor"),
    ("Codex", ".codex"),
    ("OpenCode", ".config/opencode"),
    ("Gemini CLI", ".gemini"),
    ("Windsurf", ".codeium/windsurf"),
];

/// `detectAgentTools`: the AI coding tools that have a configuration directory in the user's home.
pub(crate) fn detect_agent_tools(home: &Path) -> Vec<String> {
    AGENT_TOOLS
        .iter()
        .filter(|(_, dir)| std::fs::metadata(home.join(dir)).is_ok_and(|m| m.is_dir()))
        .map(|(name, _)| name.to_string())
        .collect()
}

/// `Install`: run the official skills installer, in the current directory only with `project`.
pub fn install(project: bool) -> Result<(), String> {
    run_installer(&installer_args(project, false))
}

/// `Uninstall`: remove the Qovery skills from every known location.
pub fn uninstall() -> Result<(), String> {
    run_installer(&installer_args(false, true))
}

pub(crate) fn installer_args(project: bool, uninstall: bool) -> Vec<&'static str> {
    if uninstall {
        vec!["--uninstall"]
    } else if project {
        vec!["--project"]
    } else {
        vec!["--global"]
    }
}

use qovery_core::gosys::look_path;

fn run_installer(args: &[&str]) -> Result<(), String> {
    if cfg!(windows) {
        return Err(format!(
            "the skills installer needs bash: run it from WSL or Git Bash with '{MANUAL_INSTALL_COMMAND}'"
        ));
    }
    let bash = look_path("bash").map_err(|e| {
        format!("the skills installer needs bash, which was not found in PATH: {e}")
    })?;

    let script = download_installer()?;

    qovery_log::flush();
    let status = std::process::Command::new(&bash)
        .arg(&*script)
        .args(args)
        .status();
    drop(script);
    match status {
        Ok(s) if s.success() => Ok(()),
        Ok(s) => Err(format!(
            "the skills installer failed: {}",
            match s.code() {
                Some(c) => format!("exit status {c}"),
                None => "signal: killed".to_string(),
            }
        )),
        Err(e) => Err(format!("the skills installer failed: {e}")),
    }
}

/// `downloadInstaller`: save the installer to a temporary file, so it is never piped into a shell.
fn download_installer() -> Result<tempfile::TempPath, String> {
    let client = reqwest::blocking::Client::builder()
        .timeout(Duration::from_secs(30))
        .build()
        .map_err(|e| e.to_string())?;
    let res = client.get(INSTALL_SCRIPT_URL).send().map_err(|e| {
        let mut reason = e.to_string();
        let mut source = std::error::Error::source(&e);
        while let Some(s) = source {
            reason = s.to_string();
            source = s.source();
        }
        format!("cannot download the skills installer: Get \"{INSTALL_SCRIPT_URL}\": {reason}")
    })?;
    if res.status().as_u16() != 200 {
        return Err(format!(
            "cannot download the skills installer: {INSTALL_SCRIPT_URL} returned {}",
            qovery_core::api::status_line(res.status().as_u16())
        ));
    }

    // The installer is a few kilobytes: cap the read so a broken response cannot fill the disk,
    // and reject an oversized one rather than running a truncated script
    let mut content = Vec::new();
    std::io::Read::read_to_end(
        &mut std::io::Read::take(res, MAX_INSTALLER_SIZE as u64 + 1),
        &mut content,
    )
    .map_err(|e| format!("cannot download the skills installer: {e}"))?;
    if content.len() > MAX_INSTALLER_SIZE {
        return Err(format!(
            "cannot download the skills installer: the response is larger than {MAX_INSTALLER_SIZE} bytes"
        ));
    }
    if !content.starts_with(b"#!") {
        return Err(
            "cannot download the skills installer: the response is not a shell script".to_string(),
        );
    }

    let mut file = tempfile::Builder::new()
        .prefix("qovery-skills-install-")
        .suffix(".sh")
        .tempfile()
        .map_err(|e| e.to_string())?;
    file.write_all(&content).map_err(|e| e.to_string())?;
    Ok(file.into_temp_path())
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;

    pub fn install_skill_at(base: &Path, version: &str) {
        let dir = base.join(ROUTER_SKILL);
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("SKILL.md"), "---\nname: qovery\n---\n").unwrap();
        if !version.is_empty() {
            std::fs::write(dir.join(VERSION_FILE), format!("{version}\n")).unwrap();
        }
    }

    #[test]
    fn find_installations_reads_version_and_deduplicates() {
        let home = tempfile::tempdir().unwrap();
        let home = home.path();
        install_skill_at(&home.join(".claude").join("skills"), "abc1234");
        install_skill_at(&home.join(".agents").join("skills"), "");

        let installations = find_installations_in(home, home);
        assert_eq!(
            installations,
            vec![
                Installation {
                    path: home.join(".claude/skills/qovery"),
                    version: "abc1234".into()
                },
                Installation {
                    path: home.join(".agents/skills/qovery"),
                    version: "unknown".into()
                },
            ]
        );
    }

    #[test]
    fn args() {
        assert_eq!(installer_args(false, false), vec!["--global"]);
        assert_eq!(installer_args(true, false), vec!["--project"]);
        assert_eq!(installer_args(false, true), vec!["--uninstall"]);
    }
}
