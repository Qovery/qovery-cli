//! Port of `utils/auth.go` and the token part of `utils/context.go`.

use crate::context::{get_current_context, store_context};
use base64::Engine;
use serde::Deserialize;
use serde_json::Value;

/// `utils.ErrSignInRequired`.
pub const ERR_SIGN_IN_REQUIRED: &str =
    "sign in using 'qovery auth' or 'qovery auth --headless' command";

/// OAuth client of the browser flow (`qovery auth`).
pub const OAUTH_BROWSER_CLIENT_ID: &str = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0";
pub const OAUTH_TOKEN_ENDPOINT: &str = "https://auth.qovery.com/oauth/token";
pub const SUPPORT_URL: &str =
    "https://www.qovery.com/docs/getting-started/useful-resources/help-and-support";

/// `Bearer` or `Token`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TokenType {
    Bearer,
    Token,
}

impl TokenType {
    pub fn as_str(&self) -> &'static str {
        match self {
            TokenType::Bearer => "Bearer",
            TokenType::Token => "Token",
        }
    }
}

/// `utils.GetAccessToken(false)` + `utils.GetAuthorizationHeaderValue(tokenType, token)`: the `Authorization`
/// header value (may refresh the token through the API).
pub fn authorization() -> Result<String, String> {
    let (token_type, token) = get_access_token(false)?;
    Ok(authorization_header_value(token_type, &token))
}

/// `utils.GetAuthorizationHeaderValue`.
pub fn authorization_header_value(token_type: TokenType, token: &str) -> String {
    format!("{} {}", token_type.as_str(), token.trim())
}

/// `utils.AccessTokenFromEnv()`: `QOVERY_CLI_ACCESS_TOKEN` or `Q_CLI_ACCESS_TOKEN`.
pub fn access_token_from_env() -> String {
    match std::env::var("QOVERY_CLI_ACCESS_TOKEN") {
        Ok(t) if !t.is_empty() => t,
        _ => std::env::var("Q_CLI_ACCESS_TOKEN").unwrap_or_default(),
    }
}

/// Decode (without verifying) the claims of a JWT.
pub fn jwt_claims(token: &str) -> Option<serde_json::Map<String, Value>> {
    let payload = token.trim().split('.').nth(1)?;
    let bytes = base64::engine::general_purpose::URL_SAFE_NO_PAD
        .decode(payload.trim_end_matches('='))
        .ok()?;
    match serde_json::from_slice::<Value>(&bytes).ok()? {
        Value::Object(m) => Some(m),
        _ => None,
    }
}

/// `oAuthClientOf(token)`: client that issued the token (`azp` claim), browser client by default.
pub fn oauth_client_of(token: &str) -> String {
    jwt_claims(token)
        .and_then(|c| c.get("azp").and_then(|v| v.as_str()).map(|s| s.to_string()))
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| OAUTH_BROWSER_CLIENT_ID.to_string())
}

/// Go `time.Time` RFC3339Nano JSON encoding of a local time.
pub fn go_time_json(t: chrono::DateTime<chrono::Local>) -> String {
    qovery_log::gofmt::time_rfc3339_nano(&t)
}

/// `utils.SetAccessToken(token, expiration, refreshToken)`.
pub fn set_access_token(
    token: &str,
    expiration: chrono::DateTime<chrono::Local>,
    refresh_token: &str,
) -> Result<(), String> {
    let mut ctx = get_current_context()?;
    ctx.access_token = token.to_string();
    ctx.access_token_expiration = go_time_json(expiration);
    ctx.refresh_token = refresh_token.to_string();
    if let Some(sub) =
        jwt_claims(token).and_then(|c| c.get("sub").and_then(|v| v.as_str()).map(|s| s.to_string()))
    {
        ctx.user = sub;
    }
    store_context(&ctx)
}

#[derive(Deserialize, Default)]
struct TokensResponse {
    #[serde(default)]
    access_token: String,
    #[serde(default)]
    expires_in: u64,
}

#[derive(Deserialize, Default)]
struct TokenErrorResponse {
    #[serde(default)]
    error: String,
    #[serde(default)]
    error_description: String,
}

/// `utils.RefreshAccessToken(refreshToken, clientId)`: get a new access token and store it.
pub fn refresh_access_token(refresh_token: &str, client_id: &str) -> Result<String, String> {
    let rt = refresh_token.trim();
    if rt.is_empty() {
        return Err(format!(
            "could not reauthenticate automatically. Please, {ERR_SIGN_IN_REQUIRED}. "
        ));
    }
    let res = qovery_api::http_client_builder()
        .build()
        .expect("cannot build http client")
        .post(OAUTH_TOKEN_ENDPOINT)
        .form(&[
            ("client_id", client_id),
            ("grant_type", "refresh_token"),
            ("refresh_token", rt),
        ])
        .send()
        .map_err(|e| {
            format!(
                "could not reach the authentication server: {e}. Check your network connection and retry"
            )
        })?;
    let status = res.status().as_u16();
    match status {
        200 => {}
        400 | 401 | 403 => {
            let mut reason: TokenErrorResponse = res.json().unwrap_or_default();
            if reason.error_description.is_empty() {
                reason.error_description = reason.error.clone();
            }
            if reason.error_description.is_empty() {
                reason.error_description = qovery_api::status_line(status);
            }
            return Err(format!(
                "could not reauthenticate automatically: {} Please, {ERR_SIGN_IN_REQUIRED}. ",
                reason.error_description
            ));
        }
        _ => {
            return Err(format!(
                "could not refresh the access token: the authentication server answered {}",
                qovery_api::status_line(status)
            ));
        }
    }
    let tokens: TokensResponse = res.json().map_err(|e| {
        format!(
            "could not read the authentication server answer: {e}. If it persists, contact Qovery support: {SUPPORT_URL}"
        )
    })?;
    if tokens.access_token.is_empty() {
        return Err("could not refresh the access token: the authentication server returned no access token".to_string());
    }
    let expired_at =
        chrono::Local::now() + chrono::Duration::seconds(tokens.expires_in as i64 - 60);
    let _ = set_access_token(&tokens.access_token, expired_at, refresh_token);
    Ok(tokens.access_token)
}

fn check_orga_valid(orgs: &[Value]) -> Result<(), String> {
    if orgs.is_empty() {
        Err("you don't have any organization. Please create an account on https://start.qovery.com . ".to_string())
    } else {
        Ok(())
    }
}

fn list_organizations(token: &str) -> Result<Vec<Value>, qovery_api::ApiError> {
    let client = crate::client::client_for(TokenType::Bearer, token);
    client.list::<Value>("/organization")
}

/// `utils.GetAccessToken(skipOrgaCheck)`: a valid access token, refreshed if expired.
pub fn get_access_token(skip_orga_check: bool) -> Result<(TokenType, String), String> {
    let api_token = access_token_from_env();
    if !api_token.is_empty() {
        let first = api_token.split('.').next().unwrap_or("");
        let is_b64 = base64::engine::general_purpose::STANDARD
            .decode(first)
            .is_ok();
        return Ok(if is_b64 {
            (TokenType::Bearer, api_token)
        } else {
            (TokenType::Token, api_token)
        });
    }
    let ctx = get_current_context()?;
    let token = ctx.access_token.clone();
    if token.is_empty() {
        return Err(format!(
            "access token has not been found. Please, {ERR_SIGN_IN_REQUIRED}. "
        ));
    }
    if let Ok(orgs) = list_organizations(&token) {
        if !skip_orga_check {
            check_orga_valid(&orgs)?;
        }
        return Ok((TokenType::Bearer, token));
    }
    let token = refresh_access_token(&ctx.refresh_token, &oauth_client_of(&ctx.access_token))?;
    match list_organizations(&token) {
        Ok(orgs) => {
            if !skip_orga_check {
                check_orga_valid(&orgs)?;
            }
            Ok((TokenType::Bearer, token))
        }
        Err(e) if e.is_unauthorized() => Err(format!(
            "access token is invalid or expired. Please, {ERR_SIGN_IN_REQUIRED}. "
        )),
        Err(e) => Err(format!("could not validate the access token: {e}")),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_time() {
        use chrono::TimeZone;
        let t = chrono::Local.with_ymd_and_hms(2026, 1, 2, 3, 4, 5).unwrap();
        let s = go_time_json(t);
        assert!(s.starts_with("2026-01-02T03:04:05"), "{s}");
    }
}
