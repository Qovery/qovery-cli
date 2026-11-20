//! Go runtime / `os` / `os/exec` / `os/user` behaviours shared by the commands: `runtime.GOOS`/`GOARCH`,
//! errno and `*fs.PathError` texts, `exec.LookPath`, `os.WriteFile`, `user.Current()`, `os.UserHomeDir()`,
//! unrecovered panics.

use std::path::{Path, PathBuf};

/// Go `runtime.GOOS`.
pub fn go_os() -> &'static str {
    match std::env::consts::OS {
        "macos" => "darwin",
        other => other,
    }
}

/// Go `runtime.GOARCH`.
pub fn go_arch() -> &'static str {
    match std::env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        "x86" => "386",
        "powerpc64" => "ppc64",
        "loongarch64" => "loong64",
        other => other,
    }
}

/// Go `os.UserHomeDir()`: `$HOME` (`%USERPROFILE%` on Windows), never the passwd database. An unset or
/// empty variable is the error `$HOME is not defined` (`%userprofile% is not defined` on Windows).
pub fn user_home_dir() -> Result<PathBuf, String> {
    user_home_dir_from(cfg!(windows), |k| std::env::var_os(k))
}

fn user_home_dir_from(
    windows: bool,
    getenv: impl Fn(&str) -> Option<std::ffi::OsString>,
) -> Result<PathBuf, String> {
    let (env, enverr) = if windows {
        ("USERPROFILE", "%userprofile%")
    } else {
        ("HOME", "$HOME")
    };
    match getenv(env) {
        Some(v) if !v.is_empty() => Ok(PathBuf::from(v)),
        _ => Err(format!("{enverr} is not defined")),
    }
}

/// Go `syscall.Errno.Error()` text of an IO error (`no such file or directory`, `address already in use`, ...):
/// the C `strerror` text lowercased, which is what Go's errno table holds.
pub fn errno_text(e: &std::io::Error) -> String {
    if e.raw_os_error().is_none() {
        match e.kind() {
            std::io::ErrorKind::NotFound => return "no such file or directory".to_string(),
            std::io::ErrorKind::PermissionDenied => return "permission denied".to_string(),
            std::io::ErrorKind::AlreadyExists => return "file exists".to_string(),
            _ => {}
        }
    }
    let s = e.to_string();
    match s.find(" (os error") {
        Some(i) => s[..i].to_lowercase(),
        None => s,
    }
}

/// Go `*fs.PathError` text: `open /path: no such file or directory`.
pub fn path_error(op: &str, path: impl AsRef<Path>, e: &std::io::Error) -> String {
    format!("{op} {}: {}", path.as_ref().display(), errno_text(e))
}

#[cfg(unix)]
fn is_executable(p: &Path) -> Result<(), String> {
    use std::os::unix::fs::PermissionsExt;
    let m = std::fs::metadata(p).map_err(|e| path_error("stat", p, &e))?;
    if m.is_dir() {
        return Err("is a directory".to_string());
    }
    if m.permissions().mode() & 0o111 != 0 {
        Ok(())
    } else {
        Err("permission denied".to_string())
    }
}

#[cfg(not(unix))]
fn is_executable(p: &Path) -> Result<(), String> {
    if p.is_file() {
        Ok(())
    } else {
        Err("executable file not found in %PATH%".to_string())
    }
}

/// Go `exec.LookPath(file)`; the error is Go's `*exec.Error` text
/// (`exec: "kubectl": executable file not found in $PATH`).
pub fn look_path(file: &str) -> Result<PathBuf, String> {
    let exec_error = |reason: &str| format!("exec: {}: {reason}", qovery_log::gofmt::quote(file));
    if file.contains('/') {
        return is_executable(Path::new(file))
            .map(|_| PathBuf::from(file))
            .map_err(|e| exec_error(&e));
    }
    let path = std::env::var("PATH").unwrap_or_default();
    let separator = if cfg!(windows) { ';' } else { ':' };
    for dir in path.split(separator) {
        let dir = if dir.is_empty() { "." } else { dir };
        let candidate = Path::new(dir).join(file);
        if is_executable(&candidate).is_ok() {
            // Go refuses results found through a relative PATH entry (exec.ErrDot)
            if !Path::new(dir).is_absolute() {
                return Err(exec_error(
                    "cannot run executable found relative to current directory",
                ));
            }
            return Ok(candidate);
        }
    }
    Err(exec_error("executable file not found in $PATH"))
}

/// Go `os.WriteFile(name, data, perm)` (the permissions apply when the file is created).
pub fn write_file(path: impl AsRef<Path>, data: &[u8], perm: u32) -> std::io::Result<()> {
    use std::io::Write;
    let mut opts = std::fs::OpenOptions::new();
    opts.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        opts.mode(perm);
    }
    #[cfg(not(unix))]
    let _ = perm;
    opts.open(path)?.write_all(data)
}

/// Go `user.Current().Username` (cgo: `getpwuid_r(getuid())`, then `$USER` when `$HOME` is set).
#[cfg(unix)]
pub fn current_username() -> Option<String> {
    // SAFETY: getpwuid_r writes into the zeroed `pwd` and the local buffer, both outliving the call; `pw_name`
    // points into `buf` and is read before `buf` is dropped.
    unsafe {
        let uid = libc::getuid();
        let mut pwd: libc::passwd = std::mem::zeroed();
        let mut result: *mut libc::passwd = std::ptr::null_mut();
        let mut buf = vec![0 as libc::c_char; 16 * 1024];
        let rc = libc::getpwuid_r(uid, &mut pwd, buf.as_mut_ptr(), buf.len(), &mut result);
        if rc == 0 && !result.is_null() && !pwd.pw_name.is_null() {
            return Some(
                std::ffi::CStr::from_ptr(pwd.pw_name)
                    .to_string_lossy()
                    .to_string(),
            );
        }
    }
    let user = std::env::var("USER").unwrap_or_default();
    let home = std::env::var("HOME").unwrap_or_default();
    if !user.is_empty() && !home.is_empty() {
        Some(user)
    } else {
        None
    }
}

/// Go `user.Current().Username` (`$USERNAME` on Windows).
#[cfg(not(unix))]
pub fn current_username() -> Option<String> {
    std::env::var("USERNAME").ok().filter(|u| !u.is_empty())
}

/// Print `panic: <msg>` + a goroutine header on stderr and exit with status 2 (unrecovered Go panic; the
/// Go stack trace that follows cannot be reproduced).
pub fn go_panic(msg: &str) -> ! {
    qovery_log::flush();
    eprint!("panic: {msg}\n\ngoroutine 1 [running]:\n");
    qovery_log::flush();
    std::process::exit(2)
}

/// Go runtime panic on a nil pointer dereference (exit status 2).
pub fn go_nil_pointer_panic() -> ! {
    go_panic(
        "runtime error: invalid memory address or nil pointer dereference\n[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x0]",
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_user_home_dir() {
        let env = |vars: &'static [(&'static str, &'static str)]| {
            move |k: &str| {
                vars.iter()
                    .find(|(n, _)| *n == k)
                    .map(|(_, v)| std::ffi::OsString::from(v))
            }
        };
        assert_eq!(
            user_home_dir_from(false, env(&[("HOME", "/home/u/")])).unwrap(),
            PathBuf::from("/home/u/")
        );
        assert_eq!(
            user_home_dir_from(false, env(&[("HOME", ""), ("USERPROFILE", "C:\\u")])).unwrap_err(),
            "$HOME is not defined"
        );
        assert_eq!(
            user_home_dir_from(false, env(&[])).unwrap_err(),
            "$HOME is not defined"
        );
        assert_eq!(
            user_home_dir_from(true, env(&[("HOME", "/home/u"), ("USERPROFILE", "C:\\u")]))
                .unwrap(),
            PathBuf::from("C:\\u")
        );
        assert_eq!(
            user_home_dir_from(true, env(&[("HOME", "/home/u")])).unwrap_err(),
            "%userprofile% is not defined"
        );
    }

    #[test]
    fn errno() {
        assert_eq!(
            errno_text(&std::io::Error::from_raw_os_error(2)),
            "no such file or directory"
        );
        assert_eq!(
            errno_text(&std::io::Error::from_raw_os_error(98)),
            "address already in use"
        );
        assert_eq!(
            errno_text(&std::io::Error::from_raw_os_error(1)),
            "operation not permitted"
        );
        assert_eq!(
            path_error("open", "/x", &std::io::Error::from_raw_os_error(2)),
            "open /x: no such file or directory"
        );
    }

    #[test]
    fn look_path_missing() {
        assert_eq!(
            look_path("definitely-not-a-binary-xyz").unwrap_err(),
            "exec: \"definitely-not-a-binary-xyz\": executable file not found in $PATH"
        );
        assert_eq!(
            look_path("/nonexistent/x").unwrap_err(),
            "exec: \"/nonexistent/x\": stat /nonexistent/x: no such file or directory"
        );
    }
}
