//! `qovery auth` command. Port of `cmd/auth.go`, `cmd/auth_status.go`, `cmd/auth_token.go`; the sign-in flows
//! (`pkg/auth_service.go`) are in [`service`], the skills offer made after signing in (`pkg/skills/offer.go`) in
//! `qovery_cmd_skills::offer` (behind the `skills` feature, enabled by `qovery-cli`'s `cmd-skills`).

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{auth, client, context, exit, telemetry};
use qovery_log::gofmt::go_marshal_indent;
use serde::Serialize;

pub mod service;

const STATUS_LONG: &str = r#"Show whether the CLI is currently authenticated, as whom, which organization
is selected, and when the session expires.

This command never prints the access token or any other secret value, regardless
of flags. Use it as the safe way to check authentication from scripts, CI, and
automation, instead of parsing the output of 'qovery auth token':

  qovery auth status >/dev/null 2>&1 && echo authenticated || echo "not authenticated"

Unlike most other commands, this always makes a live call to the Qovery API to
confirm the token is currently accepted — a token that is present and well-formed
but has been revoked or expired server-side is reported as not authenticated,
whether it came from a browser login or from QOVERY_CLI_ACCESS_TOKEN / Q_CLI_ACCESS_TOKEN.

Exit code is 0 when authenticated, 1 otherwise."#;

const TOKEN_LONG: &str = r#"Output the current valid access token (refreshing it if expired).

This command provides a valid access token that can be used to make direct API calls 
to the Qovery API. The token is automatically refreshed if it has expired.

For security reasons, the token is not printed by default. You must explicitly
use --print or --json to output the token value.

If you only need to check whether the CLI is authenticated (e.g. from a script
or an automated agent), use 'qovery auth status' instead — it never prints the
token, even with --json.

Examples:
  # Print the raw token value
  qovery auth token --print

  # Use directly in a curl command
  curl -H "Authorization: Bearer $(qovery auth token --print)" https://api.qovery.com/organization

  # Print the full Authorization header value
  qovery auth token --print --authorization-header

  # Get structured JSON output with token, type, expiration, and API URL
  qovery auth token --json

  # Get JSON with the authorization header pre-formatted
  qovery auth token --json --authorization-header"#;

/// The `auth` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("auth")
        .short("Log in to Qovery")
        .flag(Flag::bool("headless", "Headless auth"))
        .flag(Flag::bool(
            "skipVersionCheck",
            "Skip CLI version check during authentication",
        ))
        .run(run_auth)
        .subcommand(
            Command::new("status")
                .short("Show authentication status without exposing the access token")
                .long(STATUS_LONG)
                .flag(Flag::bool("json", "JSON output"))
                .run(run_status),
        )
        .subcommand(
            Command::new("token")
                .short("Output the current valid access token")
                .long(TOKEN_LONG)
                .flag(Flag::bool(
                    "print",
                    "Print the raw access token value to stdout",
                ))
                .flag(Flag::bool(
                    "json",
                    "Output as JSON with token, type, expiration, and API URL",
                ))
                .flag(Flag::bool(
                    "authorization-header",
                    "Output the full Authorization header value (e.g. 'Bearer eyJ...')",
                ))
                .run(run_token),
        )
}

// ---------------------------------------------------------------------------------------------------------------
// qovery auth

fn run_auth(ctx: &Ctx) {
    telemetry::capture(ctx);
    let headless = ctx.bool("headless");
    #[cfg(feature = "skills")]
    let previous_token = current_access_token();
    service::do_request_user_to_authenticate(headless, ctx.bool("skipVersionCheck"));
    // Right after signing in is when the user is most likely to set up their AI agent
    #[cfg(feature = "skills")]
    if has_new_session(&previous_token) {
        qovery_cmd_skills::offer::offer_after_auth(ctx, headless);
    }
}

/// The context as Go decodes it: `None` when the file is missing or invalid (including a bad expiration time).
fn decoded_context() -> Option<(
    context::QoveryContext,
    Option<chrono::DateTime<chrono::Utc>>,
)> {
    let ctx = context::get_current_context().ok()?;
    // Go's `time.Parse(time.RFC3339)` rules (lenient where chrono is strict, e.g. a one-digit hour)
    let expiration = ctx.access_token_expiration_time()?;
    let zero = context::QoveryContext::default().access_token_expiration_time()?;
    Some((
        ctx,
        if expiration == zero {
            None
        } else {
            Some(expiration)
        },
    ))
}

#[cfg(feature = "skills")]
fn current_access_token() -> String {
    decoded_context()
        .map(|(c, _)| c.access_token)
        .unwrap_or_default()
}

/// `hasNewSession`: whether this run stored a new valid session.
#[cfg(feature = "skills")]
fn has_new_session(previous_token: &str) -> bool {
    match decoded_context() {
        Some((c, expiration)) => {
            !c.access_token.is_empty()
                && c.access_token != previous_token
                && expiration.is_some_and(|e| e > chrono::Utc::now())
        }
        None => false,
    }
}

fn format_expiration(e: chrono::DateTime<chrono::Utc>) -> String {
    e.format("%Y-%m-%dT%H:%M:%SZ").to_string()
}

// ---------------------------------------------------------------------------------------------------------------
// qovery auth status

#[derive(Serialize, Default)]
struct AuthStatusOutput {
    authenticated: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    token_type: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    expires_at: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    organization_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    organization_name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    user: String,
    api_url: String,
}

fn run_status(ctx: &Ctx) {
    telemetry::capture(ctx);
    let json = ctx.bool("json");
    let not_authenticated = || -> ! {
        print_auth_status(
            json,
            &AuthStatusOutput {
                api_url: client::api_base_url(),
                ..Default::default()
            },
        );
        exit(1)
    };

    let Ok((token_type, token)) = auth::get_access_token(true) else {
        not_authenticated()
    };

    // Always re-verify, whatever the origin of the token
    let api = client::client_for(token_type, &token);
    if api.list::<serde_json::Value>("/organization").is_err() {
        not_authenticated();
    }

    let mut output = AuthStatusOutput {
        authenticated: true,
        token_type: token_type.as_str().to_string(),
        api_url: client::api_base_url(),
        ..Default::default()
    };
    // Best-effort: the context is only populated for browser/device-flow logins
    if let Some((c, expiration)) = decoded_context() {
        if let Some(e) = expiration {
            output.expires_at = format_expiration(e);
        }
        output.organization_id = c.organization_id;
        output.organization_name = c.organization_name;
        output.user = c.user;
    }
    print_auth_status(json, &output);
}

fn print_auth_status(json: bool, output: &AuthStatusOutput) {
    if json {
        qovery_log::println(go_marshal_indent(output, "  "));
        return;
    }
    if !output.authenticated {
        qovery_log::println("Not authenticated. Run 'qovery auth' to log in.");
        return;
    }
    qovery_log::println("Authenticated:  yes");
    qovery_log::println(format!("Token type:     {}", output.token_type));
    if !output.expires_at.is_empty() {
        qovery_log::println(format!("Expires at:     {}", output.expires_at));
    }
    if !output.organization_name.is_empty() {
        qovery_log::println(format!(
            "Organization:   {} ({})",
            output.organization_name, output.organization_id
        ));
    }
    if !output.user.is_empty() {
        qovery_log::println(format!("User:           {}", output.user));
    }
    qovery_log::println(format!("API URL:        {}", output.api_url));
}

// ---------------------------------------------------------------------------------------------------------------
// qovery auth token

#[derive(Serialize, Default)]
struct AuthTokenJsonOutput {
    #[serde(skip_serializing_if = "String::is_empty")]
    access_token: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    token_type: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    authorization_header: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    expires_at: String,
    api_url: String,
}

fn run_token(ctx: &Ctx) {
    telemetry::capture(ctx);
    let print = ctx.bool("print");
    let json = ctx.bool("json");
    let authorization_header = ctx.bool("authorization-header");

    // Neither --print nor --json: show help and available flags
    if !print && !json {
        ctx.help();
        return;
    }

    let (token_type, token) = match auth::get_access_token(false) {
        Ok(t) => t,
        Err(e) => {
            eprintln!("Error: {e}");
            exit(1)
        }
    };

    if json {
        let mut output = AuthTokenJsonOutput {
            api_url: client::api_base_url(),
            ..Default::default()
        };
        if authorization_header {
            output.authorization_header = auth::authorization_header_value(token_type, &token);
        } else {
            output.access_token = token.clone();
            output.token_type = token_type.as_str().to_string();
        }
        // Expiration only available for Bearer tokens from context.json
        if token_type == auth::TokenType::Bearer
            && let Some((_, Some(e))) = decoded_context()
        {
            output.expires_at = format_expiration(e);
        }
        println!("{}", go_marshal_indent(&output, "  "));
        return;
    }

    if authorization_header {
        print!("{}", auth::authorization_header_value(token_type, &token));
        qovery_log::flush();
        return;
    }
    // --print: just the raw token value
    print!("{token}");
    qovery_log::flush();
}
