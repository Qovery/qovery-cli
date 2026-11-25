//! Port of `utils/version.go` and `pkg/version.go`.

/// `utils.Version`: injected at build time (`QOVERY_CLI_VERSION`), `unknown` otherwise (like `go build`).
pub const VERSION: &str = match option_env!("QOVERY_CLI_VERSION") {
    Some(v) => v,
    None => "unknown",
};

/// `pkg.GetCurrentVersion()`.
pub fn current_version() -> Result<semver::Version, String> {
    let raw = VERSION.trim_start_matches('v');
    semver::Version::parse(raw).map_err(|e| {
        format!(
            "error trying to get semver from raw string `{VERSION}`, error: `{}`",
            go_semver_error(raw, &e.to_string())
        )
    })
}

fn go_semver_error(_raw: &str, _rust_err: &str) -> String {
    // Masterminds/semver error message for an invalid version
    "invalid semantic version".to_string()
}

/// `pkg.GetLatestOnlineVersionUrl()`: follows the GitHub "latest release" redirect and returns the URL path.
pub fn latest_online_version_url() -> Result<String, String> {
    let url = "https://github.com/Qovery/qovery-cli/releases/latest";
    let res = qovery_api::http_get(url)
        .map_err(|_| "can't reach Github, please check your network connectivity".to_string())?;
    Ok(res.url().path().to_string())
}

/// `pkg.GetLatestOnlineVersionNumber()` (prints the error and exits 0 on failure, like Go).
pub fn latest_online_version_number() -> semver::Version {
    let path = match latest_online_version_url() {
        Ok(p) => p,
        Err(e) => {
            qovery_log::println_error(e);
            crate::exit(0)
        }
    };
    let last = path.rsplit("/v").next().unwrap_or("").to_string();
    match semver::Version::parse(last.trim_start_matches('v')) {
        Ok(v) => v,
        Err(_) => {
            qovery_log::println_error("invalid semantic version");
            crate::exit(0)
        }
    }
}

/// `pkg.CheckAvailableNewVersion()`.
pub fn check_available_new_version() -> (bool, String, Option<semver::Version>) {
    let latest = latest_online_version_number();
    let current = match current_version() {
        Ok(v) => v,
        Err(e) => {
            return (
                false,
                format!(
                    "Error while trying to get the current version, mostlikely current version `{VERSION}` is not a valid semver string, error: `{e}`"
                ),
                None,
            );
        }
    };
    if latest > current {
        return (
            true,
            format!(
                "A new version has been found {latest}, please upgrade it. \nYou can use your package manager or 'qovery upgrade' command. "
            ),
            Some(latest),
        );
    }
    (
        false,
        "You're already using the latest version. ".to_string(),
        Some(latest),
    )
}
