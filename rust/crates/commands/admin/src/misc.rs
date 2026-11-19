//! Single admin commands: enable-user-connect, encrypt-secret, force-delete-project,
//! force-failed-deployments-to-internal-error, publish-environment-deployment-rules,
//! require-extended-credentials. Port of the corresponding `cmd/admin_*.go` and `pkg/*.go` files.

use crate::http::{Req, authorization};
use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::client::admin_url;
use qovery_core::select::validate;
use qovery_core::{check_error, exit, exit_with_error};
use qovery_log::gofmt::go_json;
use qovery_log::logrus;
use reqwest::Method;
use serde_json::json;
use std::time::Duration;

// ---------------------------------------------------------------- enable-user-connect

pub fn enable_user_connect() -> Command {
    Command::new("enable-user-connect")
        .short("Allow a new user to connect after sign up")
        .long(
            "Allow a new user to connect after sign up with the specified email and authentication provider.

Example:
  qovery admin enable-user-connect --user-email \"user@example.com\"
  qovery admin enable-user-connect --user-email \"user@example.com\" --provider \"github\"
",
        )
        .flag(Flag::string("user-email", "User email address (required)").short('e').required())
        .flag(Flag::string("provider", "Authentication provider (github, gitlab, bitbucket, microsoft, google)").short('p'))
        .run(run_enable_user_connect)
}

/// `validProviders` (a Go map: the order of the keys listed in the "invalid provider" error is random in Go).
const VALID_PROVIDERS: &[(&str, &str)] = &[
    ("github", "GITHUB"),
    ("gitlab", "GITLAB"),
    ("bitbucket", "BITBUCKET"),
    ("microsoft", "MICROSOFT"),
    ("google", "GOOGLE"),
];

fn run_enable_user_connect(ctx: &Ctx) {
    let user_email = ctx.string("user-email");
    let provider = ctx.string("provider");
    if user_email.is_empty() {
        exit_with_error("user email is required");
    }
    let mut provider_enum = "";
    if !provider.is_empty() {
        let lower = provider.to_lowercase();
        match VALID_PROVIDERS.iter().find(|(k, _)| *k == lower) {
            Some((_, p)) => provider_enum = p,
            None => {
                let opts: Vec<&str> = VALID_PROVIDERS.iter().map(|(k, _)| *k).collect();
                exit_with_error(format!(
                    "invalid provider '{provider}'. Valid values are: [{}]",
                    opts.join(" ")
                ));
            }
        }
    }
    let auth = check_error(authorization());
    let mut payload = format!("{{\"user_email\":{}", go_json(&json!(user_email)));
    if !provider_enum.is_empty() {
        payload.push_str(&format!(",\"provider\":{}", go_json(&json!(provider_enum))));
    }
    payload.push('}');
    let url = format!("{}/enableUserSignUp", admin_url());
    let res = match Req::new(Method::POST, &url, &auth).body(payload).send() {
        Ok(r) => r,
        Err(e) => exit_with_error(format!("failed to execute request: {e}")),
    };
    if let Some(e) = &res.read_err {
        exit_with_error(format!("failed to read response body: {e}"));
    }
    if res.status == 200 {
        qovery_log::println("✅ User signup enabled successfully");
        if !res.body.is_empty() {
            qovery_log::println(format!("Response: {}", res.text()));
        }
    } else {
        exit_with_error(format!(
            "❌ failed to enable user signup: {} - {}",
            res.status_line(),
            res.text()
        ));
    }
}

// ---------------------------------------------------------------- encrypt-secret

pub fn encrypt_secret() -> Command {
    Command::new("encrypt-secret")
        .short("Encrypt a clear text message as a secret that can be used in core DB")
        .flag(
            Flag::string(
                "organization-id",
                "Organization ID of which the secret need to be encrypted of",
            )
            .short('o'),
        )
        .flag(Flag::string("message", "The message/value to encrypt").short('m'))
        .run(run_encrypt_secret)
}

fn run_encrypt_secret(ctx: &Ctx) {
    let organization_id = ctx.string("organization-id");
    let message = ctx.string("message");
    if organization_id.is_empty() {
        qovery_log::println_info("organization-id is required");
        exit(1);
    }
    if message.is_empty() {
        qovery_log::println_info("message is required");
        exit(1);
    }
    let secret = check_error(call_encrypt_secret(&organization_id, &message));
    qovery_log::println_info(format!("{message} ==> {secret}"));
}

fn call_encrypt_secret(organization_id: &str, secret: &str) -> Result<String, String> {
    let auth = authorization().map_err(|e| format!("failed to get access token: {e}"))?;
    let url = crate::goutil::url_with_joined_path(
        &admin_url(),
        &["organization", organization_id, "secret"],
    );
    let res = Req::new(Method::POST, &url, &auth)
        .body(secret.as_bytes().to_vec())
        .timeout(Duration::from_secs(30))
        .send()
        .map_err(|e| format!("request failed: {e}"))?;
    if res.status != 200 && res.status != 204 {
        return Err(format!("failed to encrypt secret (status={})", res.status));
    }
    if let Some(e) = res.read_err {
        return Err(format!("failed to read body: {e}"));
    }
    Ok(res.text())
}

// ---------------------------------------------------------------- force-delete-project

pub fn force_delete_project() -> Command {
    Command::new("force-delete-project")
        .short("Force delete project by id (only Qovery DB side, without calling the engine)")
        .flag(
            Flag::string("project", "Project's id")
                .short('p')
                .required(),
        )
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .run(run_force_delete_project)
}

/// `pkg.httpDelete` / `pkg.deleteWithBody`: `None` when dry run is enabled (no request).
pub(crate) fn delete_with_body(
    url: &str,
    dry_run_disabled: bool,
    body: Option<Vec<u8>>,
) -> Option<crate::http::Res> {
    let auth = match authorization() {
        Ok(a) => a,
        Err(e) => {
            qovery_log::println_error(e);
            exit(0)
        }
    };
    if !dry_run_disabled {
        return None;
    }
    let mut req = Req::new(Method::DELETE, url, &auth);
    if let Some(b) = body {
        req = req.body(b);
    }
    match req.send() {
        Ok(r) => Some(r),
        Err(e) => logrus::fatal(e),
    }
}

fn run_force_delete_project(ctx: &Ctx) {
    let project_id = ctx.string("project");
    let dry_run_disabled = ctx.bool("disable-dry-run");
    qovery_log::dry_run_print(dry_run_disabled);
    if validate("delete") {
        let res = delete_with_body(
            &format!("{}/project/{}", admin_url(), project_id),
            dry_run_disabled,
            None,
        );
        match res {
            None => println!("Project with id {project_id} deletable."),
            Some(res) => {
                let status = res.status_line();
                if !status.contains("200") && !status.contains("204") {
                    logrus::error(format!(
                        "Could not delete project with id {project_id} : {status}. {}",
                        res.text()
                    ));
                } else {
                    println!("Project with id {project_id} deleted.");
                }
            }
        }
    }
}

// ---------------------------------------------------------------- force-failed-deployments-to-internal-error

pub fn force_failed_deployments_to_internal_error() -> Command {
    Command::new("force-failed-deployments-to-internal-error")
        .short("Force the status of environment deployments in a non-final state to INTERNAL_ERROR, and also force any of the deployment statuses associated")
        .flag(
            Flag::string(
                "safeguardDuration",
                "wait at least the duration for env in non final state that haven't been updated to mark them as failed",
            )
            .short('d')
            .default_value("20m"),
        )
        .run(run_force_failed_deployments)
}

fn run_force_failed_deployments(ctx: &Ctx) {
    let safe_duration = ctx.string("safeguardDuration");
    let nanos = match qovery_log::gofmt::parse_duration_nanos(&safe_duration) {
        Ok(n) => n,
        Err(e) => {
            logrus::error(format!(
                "Could not parse duration : {e}. Got {safe_duration}"
            ));
            exit(1)
        }
    };
    if !validate("force deployment status") {
        return;
    }
    // int(duration.Minutes()): truncation toward zero
    let nb_minutes = (nanos as f64 / 6e10) as i64;
    if nb_minutes < 5 {
        logrus::error(format!(
            "Could not force the deployments if safeguard is lower than 5minutes. Got {nb_minutes}"
        ));
    }
    let duration_iso8601 = format!("PT{nb_minutes}M");
    let url = format!(
        "{}/deployment/forceFailedDeploymentsToInternalErrorStatus?{}",
        admin_url(),
        crate::goutil::encode_query(&[("safeguardDuration", &duration_iso8601)])
    );
    let res = exec_admin_request(&url, Method::POST, true);
    if !res.status_line().contains("200") {
        logrus::error(format!(
            "Could not force the deployments status : {}. {}",
            res.status_line(),
            res.text()
        ));
    } else {
        println!("INTERNAL_ERROR status forced");
    }
}

/// `pkg.execAdminRequest` (query parameters already encoded in `url`).
fn exec_admin_request(url: &str, method: Method, dry_run_disabled: bool) -> crate::http::Res {
    let auth = match authorization() {
        Ok(a) => a,
        Err(e) => {
            qovery_log::println_error(e);
            exit(0)
        }
    };
    let body = if dry_run_disabled {
        "{ \"metadata\": { \"dry_run_deploy\": false } }"
    } else {
        "{ \"metadata\": { \"dry_run_deploy\": true } }"
    };
    match Req::new(method, url, &auth).body(body).send() {
        Ok(r) => r,
        Err(e) => logrus::fatal(e),
    }
}

// ---------------------------------------------------------------- publish-environment-deployment-rules

pub fn publish_environment_deployment_rules() -> Command {
    Command::new("publish-environment-deployment-rules")
        .short("Republish environment deployment rules to scheduler")
        .run(run_publish_environment_deployment_rules)
}

fn run_publish_environment_deployment_rules(_ctx: &Ctx) {
    admin_url();
    qovery_log::println("Publishing environment deployment rules to scheduler...");
    let auth = match authorization() {
        Ok(a) => a,
        Err(e) => {
            qovery_log::println_error(e);
            exit(0)
        }
    };
    let url = format!("{}/environmentDeploymentRules/pushToScheduler", admin_url());
    let res = match Req::new(Method::POST, &url, &auth).send() {
        Ok(r) => r,
        Err(_) => {
            // Go dereferences the nil response before checking the error
            eprintln!("panic: runtime error: invalid memory address or nil pointer dereference");
            exit(2)
        }
    };
    if res.status != 200 {
        logrus::fatal(format!(
            "Failed to publish environment deployment rules to scheduler. Status code: {}",
            res.status_line()
        ));
    }
    qovery_log::println("Environment deployment rules successfully published to scheduler.");
}

// ---------------------------------------------------------------- require-extended-credentials

pub fn require_extended_credentials() -> Command {
    Command::new("require-extended-credentials")
        .short("Require extended credentials to deploy clusters")
        .args(ArgsRule::NoArgs)
        .flag(Flag::bool(
            "production",
            "Require extended credentials for production clusters",
        ))
        .run(run_require_extended_credentials)
}

fn run_require_extended_credentials(ctx: &Ctx) {
    let production = ctx.bool("production");
    if let Err(e) = require_extended_credentials_call(production) {
        exit_with_error(e);
    }
}

fn require_extended_credentials_call(production: bool) -> Result<(), String> {
    let body = go_json(&json!({ "production": production }));
    let url = format!("{}/cluster/requireWideAccessToDeploy", admin_url());
    let auth = authorization()?;
    let res = Req::new(Method::POST, &url, &auth).body(body).send()?;
    let result = res.text();
    if !(200..300).contains(&res.status) {
        return Err(format!(
            "could not require extended credentials (error {}: {result})",
            res.status_line()
        ));
    }
    qovery_log::println(format!(
        "Extended credentials required (production={production}): {result}"
    ));
    Ok(())
}

#[cfg(test)]
mod tests {
    /// `--safeguardDuration` goes through Go `time.ParseDuration`: one optional sign, negative values allowed.
    #[test]
    fn safeguard_durations() {
        let p = qovery_log::gofmt::parse_duration_nanos;
        assert_eq!(p("20m").unwrap(), 20 * 60 * 1_000_000_000);
        assert_eq!(p("-10m").unwrap(), -10 * 60 * 1_000_000_000);
        assert_eq!(p("-0").unwrap(), 0);
        assert_eq!(p("-abc").unwrap_err(), "time: invalid duration \"-abc\"");
        assert_eq!(p("-+10m").unwrap_err(), "time: invalid duration \"-+10m\"");
        assert_eq!(p("--0").unwrap_err(), "time: invalid duration \"--0\"");
        assert_eq!(
            p("-5x").unwrap_err(),
            "time: unknown unit \"x\" in duration \"-5x\""
        );
    }
}
