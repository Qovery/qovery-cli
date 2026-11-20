//! Port of `github.com/pkg/browser.OpenURL` (`qovery console`, `qovery auth`).

use crate::gosys::look_path;
use std::process::Command;

/// `runCmd`: run the opener attached to our stdout/stderr and wait for it.
fn run_cmd(prog: &str, args: &[&str]) -> Result<(), String> {
    let status = Command::new(prog)
        .args(args)
        .status()
        .map_err(|e| e.to_string())?;
    if status.success() {
        return Ok(());
    }
    match status.code() {
        Some(c) => Err(format!("exit status {c}")),
        None => Err("signal: killed".to_string()),
    }
}

/// `browser.OpenURL(url)`.
pub fn open_url(url: &str) -> Result<(), String> {
    if cfg!(target_os = "macos") {
        return run_cmd("open", &[url]);
    }
    if cfg!(windows) {
        return run_cmd("rundll32", &["url.dll,FileProtocolHandler", url]);
    }
    let providers = ["xdg-open", "x-www-browser", "www-browser"];
    for provider in providers {
        if look_path(provider).is_ok() {
            return run_cmd(provider, &[url]);
        }
    }
    Err(format!(
        "exec: {}: executable file not found in $PATH",
        qovery_log::gofmt::quote(&providers.join(","))
    ))
}
