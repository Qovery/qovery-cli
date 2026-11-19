//! `qovery admin jwt ...` and `qovery admin jwt-qovery-usage ...`.
//! Port of `cmd/admin_jwt*.go` and `cmd/admin_jw_qovery_usage*.go`.

use crate::http::{Req, authorization};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::client::admin_url;
use qovery_log::gofmt::go_json;
use qovery_log::logrus;
use qovery_log::tabwriter::TabWriter;
use reqwest::Method;
use serde_json::Value;

pub fn jwt() -> Command {
    Command::new("jwt")
        .short("Manage JWT associated to clusters")
        .run(crate::run_group_help)
        .subcommand(
            Command::new("create")
                .short("Create a Jwt for a cluster")
                .flag(Flag::string("cluster", "Cluster's id").short('c'))
                .run(run_create_jwt),
        )
        .subcommand(
            Command::new("delete")
                .short("Delete a Jwt")
                .flag(Flag::string("kid", "Cluster's id"))
                .run(run_delete_jwt),
        )
        .subcommand(
            Command::new("list")
                .short("List Jwt of a cluster")
                .flag(Flag::string("cluster", "Cluster's id").short('c'))
                .run(run_list_jwts),
        )
}

pub fn jwt_qovery_usage() -> Command {
    Command::new("jwt-qovery-usage")
        .short("Manage JWT for qovery usage ")
        .run(crate::run_group_help)
        .subcommand(
            Command::new("create")
                .short("Create a Jwt for Qovery usage")
                .flag(Flag::string("cluster-id", "Cluster's id").short('c'))
                .flag(Flag::string("organization-id", "Organization's id"))
                .flag(Flag::string("root-dns", "root dns"))
                .flag(
                    Flag::string(
                        "additional-claims",
                        "Additional claims in JSON format (e.g., '{\"key1\":\"value1\",\"key2\":\"value2\"}')",
                    )
                    .default_value("{}"),
                )
                .flag(Flag::string("description", "Description of the JWT").short('d'))
                .run(run_create_jwt_for_qovery_usage),
        )
        .subcommand(
            Command::new("delete")
                .short("Delete a Jwt for Qovery Usage")
                .flag(Flag::string("kid", "Cluster's id"))
                .run(run_delete_jwt_for_qovery_usage),
        )
        .subcommand(Command::new("list").short("List Jwt for Qovery usage").run(run_list_jwts_for_qovery_usage))
}

/// `tokenType, token, err := utils.GetAccessToken(false); if err != nil { utils.PrintlnError(err); os.Exit(0) }`
fn authorization_or_exit0() -> String {
    match authorization() {
        Ok(a) => a,
        Err(e) => {
            qovery_log::println_error(e);
            qovery_core::exit(0)
        }
    }
}

fn str_field(v: &Value, k: &str) -> String {
    v.get(k)
        .and_then(|x| x.as_str())
        .unwrap_or_default()
        .to_string()
}

/// `json.Unmarshal(body, &struct)` error handling: `log.Fatal(err)` (only syntax errors are reproduced).
fn decode_or_fatal(body: &[u8]) -> Value {
    if let Err(e) = crate::gojson::check_valid(body) {
        logrus::fatal(e);
    }
    serde_json::from_slice(body).unwrap_or(Value::Null)
}

fn send_or_fatal(req: Req) -> crate::http::Res {
    match req.send() {
        Ok(r) => r,
        Err(e) => logrus::fatal(e),
    }
}

fn run_create_jwt(ctx: &Ctx) {
    let auth = authorization_or_exit0();
    let url = format!("{}/clusters/{}/jwts", admin_url(), ctx.string("cluster"));
    let res = send_or_fatal(Req::new(Method::POST, &url, &auth).body("{  }"));
    if res.status != 200 {
        qovery_log::println_error(format!(
            "error uploading debug logs: {} {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let jwt = decode_or_fatal(&res.body);
    let mut w = TabWriter::new(1, 1);
    let line = |a: &str, b: &str, c: &str, d: &str| format!("{a}\t | {b}\t | {c}\t | {d}\n");
    w.write(&line("", "cluster_id", "key_id", "created_at"));
    w.write(&line(
        "1",
        &str_field(&jwt, "cluster_id"),
        &str_field(&jwt, "key_id"),
        &str_field(&jwt, "created_at"),
    ));
    print!("{}", w.flush());
}

fn run_delete_jwt(ctx: &Ctx) {
    let auth = authorization_or_exit0();
    let url = format!("{}/clusters/jwts/{}", admin_url(), ctx.string("kid"));
    let res = match Req::new(Method::DELETE, &url, &auth).send() {
        Ok(r) => r,
        // Go dereferences a nil response here (panic)
        Err(e) => go_nil_response_panic(&e),
    };
    if res.status != 204 {
        qovery_log::println_error(format!("error: {}", res.status_line()));
    }
}

fn go_nil_response_panic(_e: &str) -> ! {
    eprintln!("panic: runtime error: invalid memory address or nil pointer dereference");
    eprintln!("[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x0]");
    qovery_core::exit(2)
}

fn run_list_jwts(ctx: &Ctx) {
    let auth = authorization_or_exit0();
    let url = format!("{}/clusters/{}/jwts", admin_url(), ctx.string("cluster"));
    let res = send_or_fatal(Req::new(Method::GET, &url, &auth));
    if res.status != 200 {
        qovery_log::println_error(format!(
            "error uploading debug logs: {} {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let resp = decode_or_fatal(&res.body);
    let mut w = TabWriter::new(1, 1);
    let line = |a: &str, b: &str, c: &str, d: &str| format!("{a}\t | {b}\t | {c}\t | {d}\n");
    w.write(&line("", "cluster_id", "key_id", "created_at"));
    if let Some(results) = resp.get("results").and_then(|r| r.as_array()) {
        for (idx, jwt) in results.iter().enumerate() {
            w.write(&line(
                &(idx + 1).to_string(),
                &str_field(jwt, "cluster_id"),
                &str_field(jwt, "key_id"),
                &str_field(jwt, "created_at"),
            ));
        }
    }
    print!("{}", w.flush());
}

fn run_create_jwt_for_qovery_usage(ctx: &Ctx) {
    let auth = authorization_or_exit0();
    let claims =
        match crate::gojson::unmarshal_string_map(ctx.string("additional-claims").as_bytes()) {
            Ok(c) => c,
            Err(e) => {
                println!("Error when parsing additional-claims : {e}");
                return;
            }
        };
    let claims_value = match claims {
        None => Value::Null,
        Some(entries) => Value::Object(
            entries
                .into_iter()
                .map(|(k, v)| (k, Value::String(v)))
                .collect(),
        ),
    };
    let payload = format!(
        "{{\"organization_id\":{},\"cluster_id\":{},\"root_dns\":{},\"additional_claims\":{},\"description\":{}}}",
        go_json(&Value::String(ctx.string("organization-id"))),
        go_json(&Value::String(ctx.string("cluster-id"))),
        go_json(&Value::String(ctx.string("root-dns"))),
        go_json(&claims_value),
        go_json(&Value::String(ctx.string("description"))),
    );
    let url = format!("{}/jwts", admin_url());
    let res = send_or_fatal(Req::new(Method::POST, &url, &auth).body(payload));
    if res.status != 200 {
        qovery_log::println_error(format!(
            "error uploading debug logs: {} {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let jwt = decode_or_fatal(&res.body);
    let token = str_field(&jwt, "decrypted_jwt");
    let payload = match decode_jwt(&token) {
        Ok((_, p)) => p,
        Err(e) => logrus::fatal(e),
    };
    let mut w = TabWriter::new(1, 1);
    w.write("Field\t | Value\n");
    w.write("------\t | ------\n");
    w.write(&format!("key_id\t | {}\n", str_field(&jwt, "key_id")));
    w.write(&format!(
        "description\t | {}\n",
        str_field(&jwt, "description")
    ));
    w.write(&format!("jwt payload\t | {payload}\n"));
    w.write(&format!("jwt\t | {token}\n"));
    w.write(&format!(
        "created_at\t | {}\n",
        str_field(&jwt, "created_at")
    ));
    print!("{}", w.flush());
}

fn run_delete_jwt_for_qovery_usage(ctx: &Ctx) {
    let auth = authorization_or_exit0();
    let url = format!("{}/jwts/{}", admin_url(), ctx.string("kid"));
    let res = match Req::new(Method::DELETE, &url, &auth).send() {
        Ok(r) => r,
        Err(_) => {
            qovery_log::println_error("error sending delete HTTP request");
            return;
        }
    };
    if res.status != 204 {
        qovery_log::println_error(format!("error: {}", res.status_line()));
    }
}

fn run_list_jwts_for_qovery_usage(_ctx: &Ctx) {
    let auth = authorization_or_exit0();
    let url = format!("{}/jwts", admin_url());
    let res = send_or_fatal(Req::new(Method::GET, &url, &auth));
    if res.status != 200 {
        qovery_log::println_error(format!(
            "error uploading debug logs: {} {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let resp = decode_or_fatal(&res.body);
    let mut w = TabWriter::new(1, 1);
    let mut out = String::new();
    if let Some(results) = resp.get("results").and_then(|r| r.as_array()) {
        for (idx, jwt) in results.iter().enumerate() {
            let token = str_field(jwt, "decrypted_jwt");
            let payload = match decode_jwt(&token) {
                Ok((_, p)) => p,
                Err(e) => {
                    // log.Fatal: what was written to the tabwriter is lost (never flushed)
                    logrus::fatal(e)
                }
            };
            w.write("\t\n");
            w.write("Field\t | Value\n");
            w.write("------\t | ------\n");
            w.write(&format!("index\t | {}\n", idx + 1));
            w.write(&format!("key_id\t | {}\n", str_field(jwt, "key_id")));
            w.write(&format!(
                "description\t | {}\n",
                str_field(jwt, "description")
            ));
            w.write(&format!("jwt payload\t | {payload}\n"));
            w.write(&format!("jwt\t | {token}\n"));
            w.write(&format!(
                "created_at\t | {}\n",
                str_field(jwt, "created_at")
            ));
        }
    }
    out.push_str(&w.flush());
    print!("{out}");
}

/// Go `base64.RawURLEncoding.DecodeString` with Go's error text.
fn raw_url_b64_decode(s: &str) -> Result<Vec<u8>, String> {
    let val = |c: u8| -> Option<u32> {
        match c {
            b'A'..=b'Z' => Some((c - b'A') as u32),
            b'a'..=b'z' => Some((c - b'a') as u32 + 26),
            b'0'..=b'9' => Some((c - b'0') as u32 + 52),
            b'-' => Some(62),
            b'_' => Some(63),
            _ => None,
        }
    };
    let corrupt = |i: usize| format!("illegal base64 data at input byte {i}");
    let mut out = Vec::new();
    let mut quantum: Vec<u32> = Vec::new();
    let mut last_idx = 0;
    for (i, &c) in s.as_bytes().iter().enumerate() {
        if c == b'\r' || c == b'\n' {
            continue;
        }
        let v = val(c).ok_or_else(|| corrupt(i))?;
        quantum.push(v);
        last_idx = i;
        if quantum.len() == 4 {
            let n = (quantum[0] << 18) | (quantum[1] << 12) | (quantum[2] << 6) | quantum[3];
            out.extend_from_slice(&[(n >> 16) as u8, (n >> 8) as u8, n as u8]);
            quantum.clear();
        }
    }
    match quantum.len() {
        0 => {}
        1 => return Err(corrupt(last_idx)),
        2 => {
            let n = (quantum[0] << 18) | (quantum[1] << 12);
            out.push((n >> 16) as u8);
        }
        _ => {
            let n = (quantum[0] << 18) | (quantum[1] << 12) | (quantum[2] << 6);
            out.extend_from_slice(&[(n >> 16) as u8, (n >> 8) as u8]);
        }
    }
    Ok(out)
}

/// Go `json.Unmarshal(data, &map[string]interface{})` (only syntax and top-level type errors).
fn unmarshal_map(data: &[u8], go_type: &str) -> Result<Value, String> {
    crate::gojson::check_valid(data)?;
    let v: Value = serde_json::from_slice(data).map_err(|e| e.to_string())?;
    let kind = match &v {
        Value::Object(_) | Value::Null => return Ok(to_go_floats(v)),
        Value::Array(_) => "array",
        Value::String(_) => "string",
        Value::Number(_) => "number",
        Value::Bool(_) => "bool",
    };
    Err(format!(
        "json: cannot unmarshal {kind} into Go value of type {go_type}"
    ))
}

/// Numbers decoded into `interface{}` are float64 in Go.
fn to_go_floats(v: Value) -> Value {
    match v {
        Value::Number(n) if !n.is_f64() => {
            let f = n.as_f64().unwrap_or(0.0);
            serde_json::Number::from_f64(f)
                .map(Value::Number)
                .unwrap_or(Value::Null)
        }
        Value::Array(a) => Value::Array(a.into_iter().map(to_go_floats).collect()),
        Value::Object(m) => {
            Value::Object(m.into_iter().map(|(k, v)| (k, to_go_floats(v))).collect())
        }
        other => other,
    }
}

const SIGNING_METHODS: &[&str] = &[
    "ES256", "ES384", "ES512", "HS256", "HS384", "HS512", "EdDSA", "PS256", "PS384", "PS512",
    "RS256", "RS384", "RS512", "none",
];

/// `DecodeJWT`: golang-jwt `ParseUnverified` with `MapClaims`, then `json.Marshal` of header and claims.
pub fn decode_jwt(token: &str) -> Result<(String, String), String> {
    let wrap = |msg: &str| format!("failed to parse token: {msg}");
    let malformed = |msg: &str| wrap(&format!("token is malformed: {msg}"));
    let parts: Vec<&str> = token.split('.').collect();
    if parts.len() != 3 {
        return Err(malformed("token contains an invalid number of segments"));
    }
    let header_bytes = raw_url_b64_decode(parts[0])
        .map_err(|e| malformed(&format!("could not base64 decode header: {e}")))?;
    let header = unmarshal_map(&header_bytes, "map[string]interface {}")
        .map_err(|e| malformed(&format!("could not JSON decode header: {e}")))?;
    let claim_bytes = raw_url_b64_decode(parts[1])
        .map_err(|e| malformed(&format!("could not base64 decode claim: {e}")))?;
    let claims = unmarshal_map(&claim_bytes, "jwt.MapClaims")
        .map_err(|e| malformed(&format!("could not JSON decode claim: {e}")))?;
    match header.get("alg").and_then(|a| a.as_str()) {
        Some(alg) if SIGNING_METHODS.contains(&alg) => {}
        Some(_) => {
            return Err(wrap(
                "token is unverifiable: signing method (alg) is unavailable",
            ));
        }
        None => {
            return Err(wrap(
                "token is unverifiable: signing method (alg) is unspecified",
            ));
        }
    }
    Ok((go_json(&header), go_json(&claims)))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decode() {
        // {"alg":"HS256"} . {"sub":"1","n":1.0,"b":"<"}
        let t = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIiwibiI6MS4wLCJiIjoiPCJ9.c2ln";
        let (h, c) = decode_jwt(t).unwrap();
        assert_eq!(h, "{\"alg\":\"HS256\"}");
        assert_eq!(c, "{\"b\":\"\\u003c\",\"n\":1,\"sub\":\"1\"}");
        assert_eq!(
            decode_jwt("a.b").unwrap_err(),
            "failed to parse token: token is malformed: token contains an invalid number of segments"
        );
        assert_eq!(
            decode_jwt("a=.b.c").unwrap_err(),
            "failed to parse token: token is malformed: could not base64 decode header: illegal base64 data at input byte 1"
        );
        assert_eq!(
            decode_jwt("eyJhbGciOiJYIn0.e30.").unwrap_err(),
            "failed to parse token: token is unverifiable: signing method (alg) is unavailable"
        );
    }
}
