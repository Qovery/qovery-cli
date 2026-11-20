//! `qovery api` command. Port of `cmd/api.go` and `cmd/api_spec.go`.

use qovery_cobra::{ArgsRule, Command, Ctx, Flag};
use qovery_core::{auth, context, exit, exit_with_error, telemetry};
use qovery_log::gofmt::go_json;
use serde_json::Value;
use std::collections::{BTreeMap, HashMap};
use std::io::Write;
use std::time::Duration;

const API_LONG: &str = r#"Make an authenticated HTTP request to the Qovery API.

EXAMPLES

  # List organizations
  $ qovery api organization

  # Get a specific organization
  $ qovery api organization/<id>

  # List projects in current organization (from context)
  $ qovery api organization/{organizationId}/project

  # Get current environment's services (fully from context)
  $ qovery api organization/{organizationId}/project/{projectId}/environment/{environmentId}/service

  # Create an organization using --field
  $ qovery api organization --field name=my-org --field plan=FREE

  # Pipe body from stdin
  $ echo '{"name":"my-org","plan":"FREE"}' | qovery api organization --input -

  # Send a JSON file as body
  $ qovery api organization/<id>/project --input - < body.json

  # Delete a resource
  $ qovery api organization/<id> --method DELETE

  # Show response headers
  $ qovery api organization --include

  # Add a custom header
  $ qovery api organization -H "X-Request-Id: abc123"

  # Use a staging environment
  $ QOVERY_API_URL=https://staging.api.qovery.com qovery api organization"#;

const SPEC_LONG: &str = r#"Fetch and print the Qovery API's OpenAPI specification (YAML), sourced from
https://github.com/Qovery/qovery-openapi-spec.

Use it to discover valid endpoints, methods, and request/response shapes before
calling 'qovery api <endpoint>' — instead of guessing or relying on a copy that
may be out of date. This command does not require authentication.

EXAMPLES

  # Print the spec to stdout
  $ qovery api spec

  # Save it to a file
  $ qovery api spec -o openapi.yaml

  # Look up one path with a YAML query tool
  $ qovery api spec | yq '.paths["/organization"]'"#;

/// `openAPISpecURL`: canonical source of the Qovery API spec.
const OPENAPI_SPEC_URL: &str =
    "https://raw.githubusercontent.com/Qovery/qovery-openapi-spec/main/openapi.yaml";

/// The `api` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("api <endpoint>")
        .short("Make an authenticated request to the Qovery API")
        .long(API_LONG)
        .args(ArgsRule::Exact(1))
        .flag(Flag::string("method", "HTTP method (GET, POST, PUT, PATCH, DELETE)").short('X'))
        .flag(Flag::string(
            "input",
            "Body: '-' for stdin (pipe JSON to command)",
        ))
        .flag(
            Flag::string_array(
                "field",
                "Add a key=value pair to the JSON body (repeatable, smart type coercion)",
            )
            .short('f')
            .default_value("[]"),
        )
        .flag(
            Flag::string_array(
                "header",
                "Additional request headers in 'Key: Value' format (repeatable)",
            )
            .short('H')
            .default_value("[]"),
        )
        .flag(
            Flag::bool(
                "include",
                "Print HTTP response status and headers before body",
            )
            .short('i'),
        )
        .run(run_api)
        .subcommand(spec())
}

fn spec() -> Command {
    Command::new("spec")
        .short("Print the Qovery API's OpenAPI specification")
        .long(SPEC_LONG)
        .args(ArgsRule::NoArgs)
        .flag(Flag::string("output", "Write the spec to a file instead of stdout").short('o'))
        .run(run_spec)
}

// ---------------------------------------------------------------------------------------------------------------
// validation

/// `isValidHTTPHeaderName`: RFC 7230 token.
fn is_valid_http_header_name(name: &str) -> bool {
    !name.is_empty()
        && name.bytes().all(|ch| {
            ch.is_ascii_alphanumeric()
                || matches!(
                    ch,
                    b'!' | b'#'
                        | b'$'
                        | b'%'
                        | b'&'
                        | b'\''
                        | b'*'
                        | b'+'
                        | b'-'
                        | b'.'
                        | b'^'
                        | b'_'
                        | b'`'
                        | b'|'
                        | b'~'
                )
        })
}

/// `validateAPIArgs`: validates arguments and flag values before any I/O.
fn validate_api_args(
    endpoint: &str,
    method: &str,
    input: &str,
    fields: &[String],
    headers: &[String],
) -> Result<(), String> {
    if endpoint.starts_with("http://") || endpoint.starts_with("https://") {
        return Err("endpoint must be a path (e.g. /organization), not a full URL".to_string());
    }
    if !input.is_empty() && input != "-" {
        return Err("--input only accepts '-' (stdin); to send a file: qovery api <endpoint> --input - < file.json".to_string());
    }
    if !fields.is_empty() && !input.is_empty() {
        return Err("--field and --input are mutually exclusive".to_string());
    }
    if !method.is_empty() && !matches!(method, "GET" | "POST" | "PUT" | "PATCH" | "DELETE") {
        return Err(format!(
            "invalid HTTP method {}: must be one of GET, POST, PUT, PATCH, DELETE",
            qovery_log::gofmt::quote(method)
        ));
    }
    for h in headers {
        match h.find(':') {
            Some(idx) if idx > 0 => {
                let name = &h[..idx];
                if !is_valid_http_header_name(name) {
                    return Err(format!(
                        "invalid header name {}: must be a non-empty HTTP token",
                        qovery_log::gofmt::quote(name)
                    ));
                }
            }
            _ => {
                return Err(format!(
                    "invalid header {}: must be in 'Key: Value' format",
                    qovery_log::gofmt::quote(h)
                ));
            }
        }
    }
    let mut seen = std::collections::HashSet::new();
    for f in fields {
        let Some(idx) = f.find('=') else {
            return Err(format!(
                "invalid field {}: must be in 'key=value' format",
                qovery_log::gofmt::quote(f)
            ));
        };
        let key = &f[..idx];
        if key.is_empty() {
            return Err(format!(
                "invalid field {}: key must not be empty",
                qovery_log::gofmt::quote(f)
            ));
        }
        if !seen.insert(key.to_string()) {
            return Err(format!(
                "duplicate field key {}: each key may only appear once",
                qovery_log::gofmt::quote(key)
            ));
        }
    }
    Ok(())
}

// ---------------------------------------------------------------------------------------------------------------
// request building

/// `substitutePathPlaceholders`: replace `{organizationId}`, `{projectId}`, `{environmentId}`, `{serviceId}` with
/// the values of the current context (best effort; empty values leave the placeholder unchanged).
fn substitute_path_placeholders(path: &str) -> String {
    let ctx = context::get_current_context().unwrap_or_default();
    let mut path = path.to_string();
    for (placeholder, value) in [
        ("{organizationId}", &ctx.organization_id),
        ("{projectId}", &ctx.project_id),
        ("{environmentId}", &ctx.environment_id),
        ("{serviceId}", &ctx.service_id),
    ] {
        if !value.is_empty() {
            path = path.replace(placeholder, value);
        }
    }
    path
}

/// A `--field` value after `coerceFieldValue`.
#[derive(Debug, Clone, PartialEq)]
enum FieldValue {
    Bool(bool),
    Int(i64),
    Float(f64),
    Str(String),
}

/// `coerceFieldValue`: bool -> int64 -> float64 -> string.
fn coerce_field_value(v: &str) -> FieldValue {
    if v == "true" {
        return FieldValue::Bool(true);
    }
    if v == "false" {
        return FieldValue::Bool(false);
    }
    if let Ok(i) = v.parse::<i64>() {
        return FieldValue::Int(i);
    }
    if let Some(f) = go_parse_float(v) {
        return FieldValue::Float(f);
    }
    FieldValue::Str(v.to_string())
}

/// `strconv.ParseFloat(v, 64)` (decimal forms, inf/nan; out of range values are errors).
fn go_parse_float(v: &str) -> Option<f64> {
    if v.contains('_') {
        return None;
    }
    let f: f64 = v.parse().ok()?;
    if f.is_infinite() {
        let lower = v.to_ascii_lowercase();
        let unsigned = lower.trim_start_matches(['+', '-']);
        if unsigned != "inf" && unsigned != "infinity" {
            return None; // ErrRange
        }
    }
    Some(f)
}

/// Go `encoding/json` float64 encoding.
fn go_json_float(f: f64) -> Result<String, String> {
    if f.is_nan() {
        return Err("json: unsupported value: NaN".to_string());
    }
    if f.is_infinite() {
        return Err(format!(
            "json: unsupported value: {}",
            if f > 0.0 { "+Inf" } else { "-Inf" }
        ));
    }
    let abs = f.abs();
    if abs != 0.0 && !(1e-6..1e21).contains(&abs) {
        let s = format!("{f:e}");
        let (mantissa, exp) = s.split_once('e').unwrap_or((&s, "0"));
        return Ok(match exp.strip_prefix('-') {
            Some(digits) => format!("{mantissa}e-{digits}"),
            None => format!("{mantissa}e+{exp:0>2}"),
        });
    }
    Ok(format!("{f}"))
}

/// `json.Marshal(map[string]any)` of the coerced fields (sorted keys).
fn fields_json(fields: &BTreeMap<String, FieldValue>) -> Result<String, String> {
    let mut out = String::from("{");
    for (i, (k, v)) in fields.iter().enumerate() {
        if i > 0 {
            out.push(',');
        }
        out.push_str(&go_json(&Value::String(k.clone())));
        out.push(':');
        match v {
            FieldValue::Bool(b) => out.push_str(if *b { "true" } else { "false" }),
            FieldValue::Int(n) => out.push_str(&n.to_string()),
            FieldValue::Float(f) => out.push_str(&go_json_float(*f)?),
            FieldValue::Str(s) => out.push_str(&go_json(&Value::String(s.clone()))),
        }
    }
    out.push('}');
    Ok(out)
}

/// Go `textproto.CanonicalMIMEHeaderKey`.
fn canonical_header_key(key: &str) -> String {
    if !is_valid_http_header_name(key) {
        return key.to_string();
    }
    let mut upper = true;
    key.chars()
        .map(|c| {
            let r = if upper {
                c.to_ascii_uppercase()
            } else {
                c.to_ascii_lowercase()
            };
            upper = c == '-';
            r
        })
        .collect()
}

use qovery_api::go_url_error_op as go_method_op;

/// Go `*url.Error` message of a failed `client.Do(req)`.
pub(crate) fn go_transport_error(method: &str, url: &str, e: &reqwest::Error) -> String {
    let reason = transport_reason(url, e);
    format!("{} \"{}\": {}", go_method_op(method), url, reason)
}

fn transport_reason(url: &str, e: &reqwest::Error) -> String {
    if e.is_timeout() {
        return "context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
            .to_string();
    }
    let mut source: Option<&dyn std::error::Error> = Some(e);
    while let Some(s) = source {
        if let Some(io) = s.downcast_ref::<std::io::Error>() {
            let addr = url::Url::parse(url)
                .ok()
                .and_then(|u| {
                    let host = u.host_str()?.to_string();
                    let port = u.port_or_known_default()?;
                    Some(if host.contains(':') && !host.starts_with('[') {
                        format!("[{host}]:{port}")
                    } else {
                        format!("{host}:{port}")
                    })
                })
                .unwrap_or_default();
            let what = match io.kind() {
                std::io::ErrorKind::ConnectionRefused => "connection refused".to_string(),
                std::io::ErrorKind::ConnectionReset => "connection reset by peer".to_string(),
                std::io::ErrorKind::TimedOut => "i/o timeout".to_string(),
                _ => {
                    let m = io.to_string();
                    match m.find(" (os error") {
                        Some(i) => m[..i].to_lowercase(),
                        None => m,
                    }
                }
            };
            return format!("dial tcp {addr}: connect: {what}");
        }
        source = s.source();
    }
    let mut msg = e.to_string();
    let mut source = std::error::Error::source(e);
    while let Some(s) = source {
        msg = s.to_string();
        source = s.source();
    }
    msg
}

fn run_api(ctx: &Ctx) {
    let endpoint = ctx.args()[0].clone();
    let method_flag = ctx.string("method");
    let input = ctx.string("input");
    let fields = ctx.strings("field");
    let headers = ctx.strings("header");
    let include = ctx.bool("include");

    if let Err(e) = validate_api_args(&endpoint, &method_flag, &input, &fields, &headers) {
        exit_with_error(e);
    }

    // Parse headers (format already validated)
    let mut parsed_headers: HashMap<String, String> = HashMap::new();
    for h in &headers {
        let idx = h.find(':').unwrap_or(0);
        let value = &h[idx + 1..];
        parsed_headers.insert(
            h[..idx].to_string(),
            value.strip_prefix(' ').unwrap_or(value).to_string(),
        );
    }

    // Parse fields (format already validated)
    let mut parsed_fields: BTreeMap<String, FieldValue> = BTreeMap::new();
    for f in &fields {
        let idx = f.find('=').unwrap_or(0);
        parsed_fields.insert(f[..idx].to_string(), coerce_field_value(&f[idx + 1..]));
    }

    let method = if !method_flag.is_empty() {
        method_flag.clone()
    } else if !input.is_empty() || !fields.is_empty() {
        "POST".to_string()
    } else {
        "GET".to_string()
    };

    let path = substitute_path_placeholders(endpoint.trim_start_matches('/'));
    let full_url = format!("{}/{}", qovery_core::client::api_base_url(), path);

    let has_body = !input.is_empty() || !fields.is_empty();
    enum Body {
        None,
        Stdin,
        Bytes(Vec<u8>),
    }
    let body = if input == "-" {
        Body::Stdin
    } else if !fields.is_empty() {
        match fields_json(&parsed_fields) {
            Ok(s) => Body::Bytes(s.into_bytes()),
            Err(e) => exit_with_error(e),
        }
    } else {
        Body::None
    };

    let parsed_url = match url::Url::parse(&full_url) {
        Ok(u) => u,
        Err(e) => exit_with_error(format!(
            "parse {}: {}",
            qovery_log::gofmt::quote(&full_url),
            e
        )),
    };

    // Creating the very first organization is the one case where the caller may have no organization yet.
    let is_org_creation = path == "organization" && method == "POST";
    let (token_type, token) = match auth::get_access_token(is_org_creation) {
        Ok(t) => t,
        Err(e) => exit_with_error(e),
    };

    // Go only decompresses transparently when it added `Accept-Encoding` itself
    let user_accept_encoding = parsed_headers
        .keys()
        .any(|k| k.eq_ignore_ascii_case("accept-encoding"));
    let mut builder = reqwest::blocking::Client::builder().timeout(Duration::from_secs(60));
    if user_accept_encoding {
        builder = builder.no_gzip();
    }
    let client = match builder.build() {
        Ok(c) => c,
        Err(e) => exit_with_error(e),
    };
    let req_method = reqwest::Method::from_bytes(method.as_bytes()).unwrap_or(reqwest::Method::GET);
    let mut req = client.request(req_method, parsed_url).header(
        "Authorization",
        auth::authorization_header_value(token_type, &token),
    );
    if has_body {
        req = req.header("Content-Type", "application/json");
    }
    for (k, v) in &parsed_headers {
        req = req.header(k.as_str(), v.as_str());
    }
    req = match body {
        Body::None => req,
        Body::Stdin => req.body(reqwest::blocking::Body::new(std::io::stdin())),
        Body::Bytes(b) => req.body(b),
    };

    let resp = match req.send() {
        Ok(r) => r,
        Err(e) => exit_with_error(go_transport_error(&method, &full_url, &e)),
    };

    match write_response(resp, include) {
        Ok(true) => {}
        Ok(false) => exit(1),
        Err(e) => exit_with_error(e),
    }
}

/// `writeResponse`: status line + headers (with `--include`) and body on stdout for 2xx responses, on stderr
/// otherwise. Returns whether the response is a 2xx.
fn write_response(mut resp: reqwest::blocking::Response, include: bool) -> Result<bool, String> {
    let status = resp.status();
    let is_2xx = status.is_success();
    let mut buf: Vec<u8> = Vec::new();
    if include {
        let (major, minor) = match resp.version() {
            reqwest::Version::HTTP_09 => (0, 9),
            reqwest::Version::HTTP_10 => (1, 0),
            reqwest::Version::HTTP_2 => (2, 0),
            reqwest::Version::HTTP_3 => (3, 0),
            _ => (1, 1),
        };
        let _ = writeln!(
            buf,
            "HTTP/{major}.{minor} {}",
            qovery_core::api::status_line(status.as_u16())
        );
        let mut headers: BTreeMap<String, Vec<Vec<u8>>> = BTreeMap::new();
        for (name, value) in resp.headers() {
            let key = canonical_header_key(name.as_str());
            // net/http moves Transfer-Encoding out of the response headers
            if key == "Transfer-Encoding" {
                continue;
            }
            headers
                .entry(key)
                .or_default()
                .push(value.as_bytes().to_vec());
        }
        for (k, values) in &headers {
            for v in values {
                buf.extend_from_slice(k.as_bytes());
                buf.extend_from_slice(b": ");
                buf.extend_from_slice(v);
                buf.push(b'\n');
            }
        }
        buf.push(b'\n');
    }
    let mut body = Vec::new();
    let read = std::io::Read::read_to_end(&mut resp, &mut body);
    if is_2xx {
        let mut out = std::io::stdout().lock();
        let _ = out.write_all(&buf);
        if read.is_ok() {
            let _ = out.write_all(&body);
        }
        let _ = out.flush();
    } else {
        let mut out = std::io::stderr().lock();
        let _ = out.write_all(&buf);
        if read.is_ok() {
            let _ = out.write_all(&body);
        }
        let _ = out.flush();
    }
    if let Err(e) = read {
        return Err(e.to_string());
    }
    Ok(is_2xx)
}

// ---------------------------------------------------------------------------------------------------------------
// api spec

fn run_spec(ctx: &Ctx) {
    telemetry::capture(ctx);

    let client = match reqwest::blocking::Client::builder()
        .timeout(Duration::from_secs(30))
        .build()
    {
        Ok(c) => c,
        Err(e) => exit_with_error(e),
    };
    let resp = match client.get(OPENAPI_SPEC_URL).send() {
        Ok(r) => r,
        Err(e) => exit_with_error(format!(
            "could not reach {OPENAPI_SPEC_URL}: {}",
            go_transport_error("GET", OPENAPI_SPEC_URL, &e)
        )),
    };
    if resp.status().as_u16() != 200 {
        exit_with_error(format!(
            "failed to fetch OpenAPI spec: server returned {}",
            qovery_core::api::status_line(resp.status().as_u16())
        ));
    }
    let body = match resp.bytes() {
        Ok(b) => b.to_vec(),
        Err(e) => exit_with_error(e),
    };

    let output = ctx.string("output");
    if !output.is_empty() {
        if let Err(e) = write_file_0644(&output, &body) {
            exit_with_error(context::go_path_error(
                "open",
                std::path::Path::new(&output),
                &e,
            ));
        }
        // stdout stays reserved for the spec itself
        eprintln!("OpenAPI spec written to {output}");
        return;
    }

    let mut out = std::io::stdout().lock();
    if let Err(e) = out.write_all(&body).and_then(|_| out.flush()) {
        drop(out);
        exit_with_error(e);
    }
}

fn write_file_0644(path: &str, content: &[u8]) -> std::io::Result<()> {
    let mut opts = std::fs::OpenOptions::new();
    opts.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        opts.mode(0o644);
    }
    opts.open(path)?.write_all(content)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn s(v: &[&str]) -> Vec<String> {
        v.iter().map(|x| x.to_string()).collect()
    }

    #[test]
    fn validate() {
        assert!(
            validate_api_args("organization", "", "", &[], &s(&[": value"]))
                .unwrap_err()
                .contains("invalid header")
        );
        assert!(
            validate_api_args("https://api.qovery.com/organization", "", "", &[], &[])
                .unwrap_err()
                .contains("not a full URL")
        );
        assert!(
            validate_api_args("organization", "", "-", &s(&["name=x"]), &[])
                .unwrap_err()
                .contains("mutually exclusive")
        );
        assert!(
            validate_api_args("organization", "", "", &s(&["badfield"]), &[])
                .unwrap_err()
                .contains("invalid field")
        );
        assert!(
            validate_api_args("organization", "", "", &s(&["=value"]), &[])
                .unwrap_err()
                .contains("key must not be empty")
        );
        assert!(
            validate_api_args("organization", "", "", &s(&["name=a", "name=b"]), &[])
                .unwrap_err()
                .contains("duplicate field key")
        );
        assert!(
            validate_api_args("organization", "", "file.json", &[], &[])
                .unwrap_err()
                .contains("--input only accepts")
        );
        assert_eq!(
            validate_api_args("organization", "get", "", &[], &[]).unwrap_err(),
            "invalid HTTP method \"get\": must be one of GET, POST, PUT, PATCH, DELETE"
        );
        assert!(validate_api_args("organization", "POST", "-", &[], &s(&["X-A: b"])).is_ok());
    }

    #[test]
    fn coerce() {
        assert_eq!(coerce_field_value("true"), FieldValue::Bool(true));
        assert_eq!(coerce_field_value("false"), FieldValue::Bool(false));
        assert_eq!(coerce_field_value("42"), FieldValue::Int(42));
        assert_eq!(coerce_field_value("2.5"), FieldValue::Float(2.5));
        assert_eq!(coerce_field_value("42.0"), FieldValue::Float(42.0));
        assert_eq!(coerce_field_value("hello"), FieldValue::Str("hello".into()));
        assert_eq!(
            coerce_field_value("123abc"),
            FieldValue::Str("123abc".into())
        );
        assert_eq!(coerce_field_value("1e400"), FieldValue::Str("1e400".into()));
    }

    #[test]
    fn floats() {
        assert_eq!(go_json_float(42.0).unwrap(), "42");
        assert_eq!(go_json_float(2.5).unwrap(), "2.5");
        assert_eq!(go_json_float(1e20).unwrap(), "100000000000000000000");
        assert_eq!(go_json_float(1e21).unwrap(), "1e+21");
        assert_eq!(go_json_float(1e-7).unwrap(), "1e-7");
        assert_eq!(go_json_float(1.5e-10).unwrap(), "1.5e-10");
        assert_eq!(
            go_json_float(f64::INFINITY).unwrap_err(),
            "json: unsupported value: +Inf"
        );
    }

    #[test]
    fn header_keys() {
        assert_eq!(canonical_header_key("content-type"), "Content-Type");
        assert_eq!(canonical_header_key("x-request-id"), "X-Request-Id");
        assert_eq!(canonical_header_key("date"), "Date");
    }
}
