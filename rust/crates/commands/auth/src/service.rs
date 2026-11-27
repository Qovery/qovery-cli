//! Port of `pkg/auth_service.go`: browser (PKCE) and headless (device code) sign-in flows, and the retry of API
//! requests rejected as unauthorized.
//!
//! `pkg.DoRequestUserToAuthenticate` is also used by `admin cluster` commands: these functions are public so they
//! can be called from there (or moved to `qovery-core`).

use base64::Engine;
use qovery_core::api::ApiError;
use qovery_core::{auth, exit};
use serde::Deserialize;
use std::io::{BufRead, BufReader, Write};
use std::net::{TcpListener, TcpStream};
use std::time::{Duration, Instant};

const HTTP_AUTH_PORT: u16 = 10999;
const OAUTH_QOVERY_URL: &str = "https://auth.qovery.com/login";
const OAUTH_CLIENT: &str = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0";
const OAUTH_HEADLESS_CLIENT: &str = "f9drkTNpxsEw2VU2PVDrxhyT3vVuFT0Y";
const OAUTH_AUDIENCE: &str = "https://core.qovery.com";
const OAUTH_RESPONSE_TYPE: &str = "code";
const OAUTH_SCOPES: &str = "offline_access openid profile email";
const OAUTH_TOKEN_ENDPOINT: &str = "https://auth.qovery.com/oauth/token";
const OAUTH_DEVICE_CODE_ENDPOINT: &str = "https://auth.qovery.com/oauth/device/code";
const QOVERY_CONSOLE_URL: &str = "https://console.qovery.com";

fn oauth_redirect() -> String {
    format!("http://localhost:{HTTP_AUTH_PORT}/authorization")
}

#[derive(Deserialize, Default)]
struct TokensResponse {
    #[serde(default)]
    access_token: String,
    #[serde(default)]
    refresh_token: String,
    #[serde(default)]
    expires_in: u64,
}

#[derive(Deserialize, Default)]
struct DeviceFlowParameters {
    #[serde(default)]
    device_code: String,
    #[serde(default)]
    user_code: String,
    #[serde(default)]
    verification_uri: String,
    #[serde(default)]
    expires_in: i64,
    #[serde(default)]
    interval: i64,
}

use qovery_api::query_escape;

/// `encode`: base64 URL-safe without padding.
fn encode(msg: &[u8]) -> String {
    base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(msg)
}

/// `createCodeVerifier`: 64 random bytes in [0, 255).
fn create_code_verifier() -> String {
    use rand::Rng;
    let mut r = rand::thread_rng();
    let b: Vec<u8> = (0..64).map(|_| r.gen_range(0..255u8)).collect();
    encode(&b)
}

/// `createCodeChallengeS256`.
fn create_code_challenge_s256(verifier: &str) -> String {
    use sha2::Digest;
    encode(&sha2::Sha256::digest(verifier.as_bytes()))
}

fn store_tokens(tokens: &TokensResponse) {
    let expired_at =
        chrono::Local::now() + chrono::Duration::seconds(tokens.expires_in as i64 - 60);
    let _ = auth::set_access_token(&tokens.access_token, expired_at, &tokens.refresh_token);
    qovery_log::println_info("Success!");
}

/// `pkg.DoRequestUserToAuthenticate(headless, skipVersionCheck)`.
pub fn do_request_user_to_authenticate(headless: bool, skip_version_check: bool) {
    if !skip_version_check {
        let (available, message, _) = qovery_core::version::check_available_new_version();
        if available {
            println!("{message}");
        }
    }
    if headless {
        run_headless_flow();
        return;
    }

    let verifier = create_code_verifier();
    let challenge = create_code_challenge_s256(&verifier);
    let login_url = format!(
        "{OAUTH_QOVERY_URL}?code_challenge_method=S256&scope={}&client={}&protocol=oauth2&response_type={}&audience={}&redirect_uri={}&code_challenge={}",
        query_escape(OAUTH_SCOPES),
        OAUTH_CLIENT,
        query_escape(OAUTH_RESPONSE_TYPE),
        query_escape(OAUTH_AUDIENCE),
        query_escape(&oauth_redirect()),
        challenge
    );
    let _ = qovery_core::browser::open_url(&login_url);

    println!("\nOpening your browser, waiting for your authentication... ");
    qovery_log::flush();

    // Go ignores the error of ListenAndServe (e.g. port already in use): the command then simply returns
    let Ok(listener) = TcpListener::bind(("localhost", HTTP_AUTH_PORT)) else {
        return;
    };
    for stream in listener.incoming() {
        let Ok(stream) = stream else { continue };
        if serve_authorization(stream, &verifier) {
            break;
        }
    }
}

fn write_http(stream: &mut TcpStream, status: &str, content_type: &str, body: &[u8]) {
    let _ = write!(
        stream,
        "HTTP/1.1 {status}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    );
    let _ = stream.write_all(body);
    let _ = stream.flush();
}

/// Handle one request of the local callback server; true once the token has been received.
fn serve_authorization(mut stream: TcpStream, verifier: &str) -> bool {
    let mut reader = BufReader::new(match stream.try_clone() {
        Ok(s) => s,
        Err(_) => return false,
    });
    let mut request_line = String::new();
    if reader.read_line(&mut request_line).is_err() {
        return false;
    }
    loop {
        let mut line = String::new();
        match reader.read_line(&mut line) {
            Ok(0) | Err(_) => break,
            Ok(_) if line == "\r\n" || line == "\n" => break,
            Ok(_) => {}
        }
    }
    let target = request_line
        .split_whitespace()
        .nth(1)
        .unwrap_or("/")
        .to_string();
    let parsed = url::Url::parse(&format!("http://localhost{target}"));
    let (path, code) = match &parsed {
        Ok(u) => (
            u.path().to_string(),
            u.query_pairs()
                .find(|(k, _)| k == "code")
                .map(|(_, v)| v.to_string()),
        ),
        Err(_) => (target.clone(), None),
    };

    match path.as_str() {
        "/authorization" => {
            let js = format!(
                r#"<script type="text/javascript" charset="utf-8">
				var hash = window.location.search.split("=")[1].split("&")[0];
				var xmlHttp = new XMLHttpRequest();
				xmlHttp.open("GET", "http://localhost:{HTTP_AUTH_PORT}/authorization/valid?code=" + hash, false);
				xmlHttp.send(null);
				xmlHttp.responseText;
				window.setTimeout('window.location="{QOVERY_CONSOLE_URL}"; ',2000);
             </script>"#
            );
            let body = format!(
                "{js}Authentication successful, you'll be redirected to Qovery console. If it's not the case, click on this link: <a href='{QOVERY_CONSOLE_URL}'>{QOVERY_CONSOLE_URL}</a>"
            );
            write_http(
                &mut stream,
                "200 OK",
                "text/html; charset=utf-8",
                body.as_bytes(),
            );
            false
        }
        "/authorization/valid" => {
            let Some(code) = code else {
                write_http(
                    &mut stream,
                    "500 Internal Server Error",
                    "text/plain; charset=utf-8",
                    b"",
                );
                return false;
            };
            let unsuccessful = || -> ! {
                qovery_log::println_error(format!(
                    "authentication unsuccessful. Try again later or contact Qovery support: {}",
                    auth::SUPPORT_URL
                ));
                exit(0)
            };
            let redirect = oauth_redirect();
            let res = qovery_api::http_client_builder()
                .build()
                .expect("cannot build http client")
                .post(OAUTH_TOKEN_ENDPOINT)
                .form(&[
                    ("client_id", OAUTH_CLIENT),
                    ("code", code.as_str()),
                    ("code_verifier", verifier),
                    ("grant_type", "authorization_code"),
                    ("redirect_uri", redirect.as_str()),
                ])
                .send();
            let Ok(res) = res else { unsuccessful() };
            let Ok(tokens) = res.json::<TokensResponse>() else {
                unsuccessful()
            };
            store_tokens(&tokens);
            write_http(&mut stream, "200 OK", "text/plain; charset=utf-8", b"");
            true
        }
        _ => {
            write_http(
                &mut stream,
                "404 Not Found",
                "text/plain; charset=utf-8",
                b"404 page not found\n",
            );
            false
        }
    }
}

fn print_contact_support_message(msg: &str) {
    println!("{msg}");
    println!("Please contact Qovery support: {}", auth::SUPPORT_URL);
}

fn run_headless_flow() {
    let parameters = device_flow_parameters();
    println!(
        "Please, open browser @ {} using any device and enter {} code. ",
        parameters.verification_uri, parameters.user_code
    );
    let start = Instant::now();

    println!("Waiting for code confirmation...");
    qovery_log::flush();

    while start.elapsed().as_secs_f64() < parameters.expires_in as f64 {
        std::thread::sleep(Duration::from_secs(parameters.interval.max(0) as u64));
        if let Ok(tokens) = get_tokens_with(&parameters) {
            store_tokens(&tokens);
            return;
        }
    }

    println!("Code has expired! ");
    exit(0)
}

fn device_flow_parameters() -> DeviceFlowParameters {
    let payload = format!(
        "client_id={}&scope={}&audience={}&redirect_uri={}",
        query_escape(OAUTH_HEADLESS_CLIENT),
        query_escape(OAUTH_SCOPES),
        query_escape(OAUTH_AUDIENCE),
        query_escape(&oauth_redirect())
    );
    let res = qovery_api::http_client_builder()
        .build()
        .expect("cannot build http client")
        .post(OAUTH_DEVICE_CODE_ENDPOINT)
        .header("content-type", "application/x-www-form-urlencoded")
        .body(payload)
        .send();
    let Ok(res) = res else {
        print_contact_support_message("Error getting device code. ");
        exit(0)
    };
    if res.status().as_u16() != 200 {
        print_contact_support_message("Error getting device code. ");
        exit(0)
    }
    match res.json::<DeviceFlowParameters>() {
        Ok(p) => p,
        Err(_) => {
            print_contact_support_message("Error parsing device code response. ");
            exit(0)
        }
    }
}

fn get_tokens_with(params: &DeviceFlowParameters) -> Result<TokensResponse, String> {
    let payload = format!(
        "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code&device_code={}&client_id={OAUTH_HEADLESS_CLIENT}",
        params.device_code
    );
    let res = qovery_api::http_client_builder()
        .build()
        .expect("cannot build http client")
        .post(OAUTH_TOKEN_ENDPOINT)
        .header("content-type", "application/x-www-form-urlencoded")
        .body(payload)
        .send();
    let Ok(res) = res else {
        print_contact_support_message("Error pooling access token. ");
        exit(0)
    };
    if res.status().as_u16() == 200 {
        res.json::<TokensResponse>().map_err(|e| e.to_string())
    } else {
        Err("could not fetch tokens".to_string())
    }
}

// ---------------------------------------------------------------------------------------------------------------
// RetryQoveryClientApiRequestOnUnauthorized

fn is_unauthorized<T>(r: &Result<T, ApiError>) -> bool {
    matches!(r, Err(e) if e.is_unauthorized())
}

/// `pkg.RetryQoveryClientApiRequestOnUnauthorized`: retry the request once with a refreshed token when the first
/// attempt is unauthorized (401), and fall back to an interactive login when the stored credentials cannot be
/// refreshed. `request(need_to_refetch_client)`.
pub fn retry_qovery_client_api_request_on_unauthorized<T>(
    request: impl FnMut(bool) -> Result<T, ApiError>,
) -> Result<T, ApiError> {
    retry_with_login(request, || do_request_user_to_authenticate(false, true))
}

fn retry_with_login<T>(
    mut request: impl FnMut(bool) -> Result<T, ApiError>,
    mut request_user_to_authenticate: impl FnMut(),
) -> Result<T, ApiError> {
    let first = request(false);
    if !is_unauthorized(&first) {
        return first;
    }
    let status = first.as_ref().err().and_then(|e| e.status);
    let with_status = |message: String| ApiError {
        status,
        message,
        body: String::new(),
    };

    // A token set in the environment can neither be refreshed nor replaced by an interactive login
    if !auth::access_token_from_env().is_empty() {
        return Err(with_status("the access token set in QOVERY_CLI_ACCESS_TOKEN (or Q_CLI_ACCESS_TOKEN) is rejected (401 Unauthorized): check it is valid and has not expired".to_string()));
    }

    // The access token has most likely expired: refresh it silently before asking for an interactive login
    match auth::get_access_token(false) {
        Ok(_) => {
            let second = request(true);
            if !is_unauthorized(&second) {
                return second;
            }
        }
        Err(e) if !e.contains(auth::ERR_SIGN_IN_REQUIRED) => {
            // A transport or organization error cannot be solved by signing in again
            return Err(with_status(e));
        }
        Err(_) => {}
    }

    qovery_log::println("Needs to re-authenticate as the response is UNAUTHORIZED (401)");
    request_user_to_authenticate();
    request(true)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Read;
    use std::sync::Mutex;

    static ENV_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn pkce() {
        let v = create_code_verifier();
        assert!(!v.contains('=') && !v.contains('+') && !v.contains('/'));
        assert_eq!(
            create_code_challenge_s256("abc"),
            "ungWv48Bz-pBQUDeXa4iI7ADYaOWF3qctBD_YfIAFa0"
        );
        assert_eq!(
            query_escape("offline_access openid profile email"),
            "offline_access+openid+profile+email"
        );
        assert_eq!(
            query_escape("http://localhost:10999/authorization"),
            "http%3A%2F%2Flocalhost%3A10999%2Fauthorization"
        );
    }

    /// Serves `status`/`body` to every request on a local port.
    fn serve(status: u16, body: &'static str) -> String {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        std::thread::spawn(move || {
            for mut s in listener.incoming().flatten() {
                let mut buf = [0u8; 4096];
                let _ = s.read(&mut buf);
                let _ = write!(
                    s,
                    "HTTP/1.1 {status} X\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
            }
        });
        format!("http://127.0.0.1:{port}")
    }

    struct Setup {
        _home: tempfile::TempDir,
        logins: usize,
    }

    fn setup(status: u16, body: &'static str, refresh_token: Option<&str>) -> Setup {
        let home = tempfile::tempdir().unwrap();
        let url = serve(status, body);
        unsafe {
            std::env::set_var("HOME", home.path());
            std::env::set_var("QOVERY_API_URL", url);
            std::env::remove_var("QOVERY_CLI_ACCESS_TOKEN");
            std::env::remove_var("Q_CLI_ACCESS_TOKEN");
        }
        if let Some(rt) = refresh_token {
            std::fs::create_dir_all(home.path().join(".qovery")).unwrap();
            let exp =
                qovery_core::auth::go_time_json(chrono::Local::now() + chrono::Duration::hours(1));
            std::fs::write(
                home.path().join(".qovery/context.json"),
                format!(r#"{{"access_token":"stored-access-token","access_token_expiration":"{exp}","refresh_token":"{rt}"}}"#),
            )
            .unwrap();
        }
        Setup {
            _home: home,
            logins: 0,
        }
    }

    const ORGS: &str = r#"{"results":[{"id":"org-1","created_at":"2024-01-01T00:00:00Z","name":"test","plan":"BUSINESS_2025"}]}"#;

    fn unauthorized() -> Result<String, ApiError> {
        Err(ApiError {
            status: Some(401),
            message: "401 Unauthorized".into(),
            body: String::new(),
        })
    }
    fn authorized() -> Result<String, ApiError> {
        Ok("ok".to_string())
    }

    #[test]
    fn retry_on_unauthorized() {
        let _guard = ENV_LOCK.lock().unwrap();

        // no retry when the first request is authorized
        let mut s = setup(500, "{}", None);
        let mut calls = vec![];
        let r = retry_with_login(
            |b| {
                calls.push(b);
                authorized()
            },
            || s.logins += 1,
        );
        assert_eq!(r.unwrap(), "ok");
        assert_eq!(calls, vec![false]);
        assert_eq!(s.logins, 0);

        // the rejected token comes from the environment
        for var in ["QOVERY_CLI_ACCESS_TOKEN", "Q_CLI_ACCESS_TOKEN"] {
            let mut s = setup(500, "{}", None);
            unsafe { std::env::set_var(var, "env-token") };
            let mut calls = vec![];
            let r = retry_with_login(
                |b| {
                    calls.push(b);
                    unauthorized()
                },
                || s.logins += 1,
            );
            let e = r.unwrap_err();
            assert!(e.message.contains("QOVERY_CLI_ACCESS_TOKEN"));
            assert_eq!(e.status, Some(401));
            assert_eq!(calls, vec![false]);
            assert_eq!(s.logins, 0);
            unsafe { std::env::remove_var(var) };
        }

        // refetch silently when the stored token is accepted
        let mut s = setup(200, ORGS, Some("stored-refresh-token"));
        let mut calls = vec![];
        let r = retry_with_login(
            |b| {
                calls.push(b);
                if b { authorized() } else { unauthorized() }
            },
            || s.logins += 1,
        );
        assert_eq!(r.unwrap(), "ok");
        assert_eq!(calls, vec![false, true]);
        assert_eq!(s.logins, 0);

        // transport error after a successful refresh
        let mut s = setup(200, ORGS, Some("stored-refresh-token"));
        let mut calls = vec![];
        let r = retry_with_login(
            |b| {
                calls.push(b);
                if b {
                    Err(ApiError::new("dial tcp: i/o timeout"))
                } else {
                    unauthorized()
                }
            },
            || s.logins += 1,
        );
        assert_eq!(r.unwrap_err().message, "dial tcp: i/o timeout");
        assert_eq!(calls, vec![false, true]);
        assert_eq!(s.logins, 0);

        // token error that signing in again cannot fix
        let mut s = setup(200, r#"{"results":[]}"#, Some("stored-refresh-token"));
        let mut calls = vec![];
        let r = retry_with_login(
            |b| {
                calls.push(b);
                unauthorized()
            },
            || s.logins += 1,
        );
        let e = r.unwrap_err();
        assert!(e.message.contains("you don't have any organization"));
        assert_eq!(e.status, Some(401));
        assert_eq!(calls, vec![false]);
        assert_eq!(s.logins, 0);

        // stored credentials cannot be refreshed: interactive auth
        let mut s = setup(401, "{}", Some(""));
        let mut calls = vec![];
        let r = retry_with_login(
            |b| {
                calls.push(b);
                if b { authorized() } else { unauthorized() }
            },
            || s.logins += 1,
        );
        assert_eq!(r.unwrap(), "ok");
        assert_eq!(calls, vec![false, true]);
        assert_eq!(s.logins, 1);

        // still unauthorized after a silent refresh: interactive auth
        let mut s = setup(200, ORGS, Some("stored-refresh-token"));
        let mut calls = vec![];
        let r = retry_with_login(
            |b| {
                calls.push(b);
                if calls.len() <= 2 {
                    unauthorized()
                } else {
                    authorized()
                }
            },
            || s.logins += 1,
        );
        assert_eq!(r.unwrap(), "ok");
        assert_eq!(calls, vec![false, true, true]);
        assert_eq!(s.logins, 1);
    }
}
