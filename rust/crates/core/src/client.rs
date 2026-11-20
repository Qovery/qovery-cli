//! API client creation (`utils.GetQoveryClient`, `utils.GetQoveryClientPanicInCaseOfError`).

use crate::auth::{TokenType, authorization_header_value, get_access_token};
use qovery_api::Client;

pub use qovery_api::{ApiError, Client as ApiClient, api_base_url};

/// `utils.GetQoveryClient(tokenType, token)`.
pub fn client_for(token_type: TokenType, token: &str) -> Client {
    Client::new(
        &authorization_header_value(token_type, token),
        crate::version::VERSION,
        qovery_cobra::verbose(),
    )
}

/// `utils.GetAccessToken(false)` + `utils.GetQoveryClient(...)`.
pub fn get_client() -> Result<Client, String> {
    let (tt, token) = get_access_token(false)?;
    Ok(client_for(tt, &token))
}

/// `utils.GetQoveryClientPanicInCaseOfError()`: prints the error and exits 1 when no valid token.
pub fn get_client_or_exit() -> Client {
    crate::check_error(get_client())
}

/// `utils.WebsocketUrl()`: `QOVERY_WS_URL` or `wss://ws.qovery.com`.
pub fn websocket_url() -> String {
    match std::env::var("QOVERY_WS_URL") {
        Ok(u) if !u.is_empty() => u,
        _ => "wss://ws.qovery.com".to_string(),
    }
}

/// `utils.GetAdminUrl()`: `ADMIN_URL`, or logs a fatal error (exit 1) when unset.
pub fn admin_url() -> String {
    match std::env::var("ADMIN_URL") {
        Ok(u) => u,
        Err(_) => qovery_log::logrus::fatal("You must set the Qovery admin root url (ADMIN_URL)."),
    }
}
