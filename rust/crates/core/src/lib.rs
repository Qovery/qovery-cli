//! Shared logic of the Qovery CLI (Go packages `utils` and the cross-command helpers of `cmd`).
//!
//! * [`context`] — `~/.qovery/context.json` (current organization / project / environment / service, tokens)
//! * [`auth`] — access token retrieval and refresh (`utils.GetAccessToken`, `utils.RefreshAccessToken`)
//! * [`client`] — API client creation (`utils.GetQoveryClient`, `GetQoveryClientPanicInCaseOfError`)
//! * [`select`] — interactive selection of organization / project / environment / service
//! * [`find`] — lookups by name (`utils.FindByXxxName`)
//! * [`resources`] — resolution of `--organization/--project/--environment/...` flags against the context
//! * [`status`] — status texts and colors
//! * [`telemetry`] — PostHog events (`utils.Capture`)
//! * [`version`] — CLI version helpers
//! * [`gosys`] — Go runtime / `os` / `os/exec` / `os/user` behaviours (`GOOS`/`GOARCH`, errno and `*fs.PathError`
//!   texts, `exec.LookPath`, `os.WriteFile`, `user.Current`, unrecovered panics)
//! * [`browser`] — `github.com/pkg/browser.OpenURL`
//! * [`promptui`], [`promptui_select`], [`promptui_prompt`], [`readline`] — byte-identical promptui emulation when stdin is
//!   not a terminal (used by [`select`])

pub mod auth;
pub mod browser;
pub mod client;
pub mod context;
pub mod find;
pub mod gosys;
pub mod promptui;
pub mod promptui_prompt;
pub mod promptui_select;
pub(crate) mod readline;
pub mod resources;
pub mod select;
pub mod status;
pub mod telemetry;
pub mod version;

pub use qovery_api as api;
pub use qovery_cobra as cobra;
pub use qovery_log as log;

/// `utils.CheckError(err)` / `checkError(err)`: print `Error: <err>` on stdout and exit with status 1.
pub fn check_error<T, E: std::fmt::Display>(r: Result<T, E>) -> T {
    match r {
        Ok(v) => v,
        Err(e) => exit_with_error(e),
    }
}

/// `utils.PrintlnError(err); os.Exit(1)`.
pub fn exit_with_error(e: impl std::fmt::Display) -> ! {
    qovery_log::println_error(e);
    qovery_log::flush();
    qovery_log::exit::exit(1)
}

/// `os.Exit(code)` after flushing stdout/stderr.
pub fn exit(code: i32) -> ! {
    qovery_log::flush();
    qovery_log::exit::exit(code)
}
