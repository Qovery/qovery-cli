//! `qovery upgrade` command. Port of `cmd/upgrade.go`.
//!
//! Like the Go file (`//go:build !windows`), the command does not exist on Windows.
#![cfg(not(windows))]

use qovery_cobra::{Command, Ctx};
use qovery_core::version;
use qovery_log::{println_error, println_info};
use std::io::Read;
use std::path::{Path, PathBuf};

/// The `upgrade` command.
pub fn command() -> Command {
    Command::new("upgrade")
        .short("Upgrade Qovery CLI to latest version")
        .long("UPGRADE performs an upgrade of the binary. For example:\n\tqovery upgrade")
        .run(run)
}

use qovery_core::gosys::{go_arch, go_os};

/// `unix.Access(path, unix.W_OK)`.
fn writable(path: &Path) -> bool {
    use std::os::unix::ffi::OsStrExt;
    let Ok(c) = std::ffi::CString::new(path.as_os_str().as_bytes()) else {
        return false;
    };
    // SAFETY: `c` is a valid NUL-terminated string living for the duration of the call.
    unsafe { libc::access(c.as_ptr(), libc::W_OK) == 0 }
}

/// `cleanArchives`.
fn clean_archives(list: &[PathBuf]) {
    for p in list {
        if let Err(e) = std::fs::remove_file(p)
            && e.kind() != std::io::ErrorKind::NotFound
        {
            println_error(format!(
                "error while removing the element: remove {}: {}",
                p.display(),
                go_io_error(&e)
            ));
            qovery_core::exit(0);
        }
    }
}

/// Go `syscall.Errno` text of an io error (`no such file or directory`, `permission denied`, ...).
fn go_io_error(e: &std::io::Error) -> String {
    let s = e.to_string();
    let s = s.split(" (os error").next().unwrap_or(&s).to_string();
    let mut chars = s.chars();
    match chars.next() {
        Some(c) => c.to_lowercase().collect::<String>() + chars.as_str(),
        None => s,
    }
}

/// URL of the release archive of `version` for this platform.
///
/// Intentional difference with Go: the Rust CLI upgrades to the Rust build (`qovery-cli-rust_*`, published next
/// to the Go `qovery-cli_*` archives by .github/workflows/rust-build.yml, same layout), not to the Go one.
fn archive_url(version: &str, os: &str, arch: &str) -> String {
    format!(
        "https://github.com/Qovery/qovery-cli/releases/download/v{version}/qovery-cli-rust_{version}_{os}_{arch}.tar.gz"
    )
}

fn run(_ctx: &Ctx) {
    let current_binary_filename = std::env::current_exe().unwrap_or_default();
    let filename = "qovery";
    let temp_dir = std::env::temp_dir();
    let archive_name = format!("{filename}.tgz");
    let archive_path_name = temp_dir.join(&archive_name);
    let uncompress_qovery_binary_path = temp_dir.join(filename);
    let clean_list = vec![
        archive_path_name.clone(),
        uncompress_qovery_binary_path.clone(),
    ];

    let (available, message, desired_version) = version::check_available_new_version();
    if !available {
        print!("{message}");
        qovery_core::exit(0);
    }
    let desired_version = desired_version.map(|v| v.to_string()).unwrap_or_default();

    let url = archive_url(&desired_version, go_os(), go_arch());

    if !writable(&current_binary_filename) {
        println_error(format!(
            "upgrade cancelled: no write permission on the Qovery CLI binary file: {}",
            current_binary_filename.display()
        ));
        clean_archives(&clean_list);
        qovery_core::exit(0);
    }

    let resp = match qovery_api::http_get(&url) {
        Ok(r) => r,
        Err(e) => {
            println_error(format!(
                "error while downloading the latest version: Get \"{url}\": {e}"
            ));
            qovery_core::exit(0);
        }
    };
    // The Rust archives are attached to the GitHub release after the Go ones: right after a tag, the latest
    // release can still miss them (Go does not check the status and fails on the gzip header instead).
    if !resp.status().is_success() {
        println_error(format!(
            "error while downloading the latest version: Get \"{url}\": {}",
            resp.status()
        ));
        qovery_core::exit(0);
    }

    // the Go code creates (and never writes) the archive file
    if let Err(e) = std::fs::File::create(&archive_path_name) {
        println_error(format!(
            "error while overriding Qovery CLI binary file: open {}: {}",
            archive_path_name.display(),
            go_io_error(&e)
        ));
        qovery_core::exit(0);
    }

    // Decompress the tar.gz and extract the cli
    if let Err(e) = extract(resp, &uncompress_qovery_binary_path) {
        println_error(format!("error while uncompressing the archive: {e}"));
        qovery_core::exit(0);
    }

    // Fork to avoid override issue on a running program
    println_info(format!(
        "\nUpgrading Qovery CLI to version {desired_version}\n"
    ));
    let command = format!(
        "mv {} {}",
        uncompress_qovery_binary_path.display(),
        current_binary_filename.display()
    );
    if let Err(e) = std::process::Command::new("/bin/sh")
        .arg("-c")
        .arg(command)
        .spawn()
    {
        println_error(go_io_error(&e));
        qovery_core::exit(0);
    }
}

/// Extract the `qovery` file of the `.tar.gz` stream to `dest` (mode 0555).
fn extract(body: impl Read, dest: &Path) -> Result<(), String> {
    let gz = flate2::read::GzDecoder::new(body);
    let mut archive = tar::Archive::new(gz);
    let entries = archive.entries().map_err(|e| format!("gzip: {e}"))?;
    for entry in entries {
        let mut entry = entry.map_err(|e| e.to_string())?;
        let name = entry
            .path()
            .map(|p| p.to_string_lossy().to_string())
            .unwrap_or_default();
        if name != "qovery" {
            continue;
        }
        let mut out = match std::fs::File::create(dest) {
            Ok(f) => f,
            Err(_) => continue,
        };
        if let Err(e) = std::io::copy(&mut entry, &mut out) {
            println_error(format!("error while uncompressing the cli on disk: {e}"));
            qovery_core::exit(0);
        }
        use std::os::unix::fs::PermissionsExt;
        let _ = out.set_permissions(std::fs::Permissions::from_mode(0o555));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn downloads_the_rust_archive() {
        assert_eq!(
            archive_url("1.185.0", "darwin", "arm64"),
            "https://github.com/Qovery/qovery-cli/releases/download/v1.185.0/qovery-cli-rust_1.185.0_darwin_arm64.tar.gz"
        );
    }
}
