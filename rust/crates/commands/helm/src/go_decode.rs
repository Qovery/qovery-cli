//! Decoding of a successful response like the Go generated client (`APIClient.decode` + the model's
//! `UnmarshalJSON`): the envelope checks only, field type errors are not reproduced.

use serde_json::{Map, Value};

/// `APIClient.decode` into a `*Model`, then `Model.UnmarshalJSON` up to its required properties check:
/// `None` for an empty body or `null` (Go leaves the model pointer nil), the Go error text otherwise.
pub(crate) fn go_decode_model(
    res: &qovery_api::Response,
    required: &[&str],
) -> Result<Option<Map<String, Value>>, String> {
    if res.body.is_empty() {
        return Ok(None);
    }
    let content_type = res
        .headers
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default();
    if !is_go_json_content_type(content_type) {
        return Err("undefined response type".to_string());
    }
    qovery_log::gojson::go_json_check_valid(&res.body)?;
    let value: Value = serde_json::from_slice(&res.body).map_err(|e| e.to_string())?;
    let kind = match value {
        Value::Null => return Ok(None),
        Value::Object(map) => {
            if let Some(p) = required.iter().find(|p| !map.contains_key(**p)) {
                return Err(format!("no value given for required property {p}"));
            }
            return Ok(Some(map));
        }
        Value::Array(_) => "array",
        Value::String(_) => "string",
        Value::Number(_) => "number",
        Value::Bool(_) => "bool",
    };
    Err(format!(
        "json: cannot unmarshal {kind} into Go value of type map[string]interface {{}}"
    ))
}

/// The generated client's `JsonCheck`: `(?i:(?:application|text)/(?:[^;]+\+)?json)` (unanchored).
fn is_go_json_content_type(content_type: &str) -> bool {
    let ct = content_type.to_ascii_lowercase();
    ["application/", "text/"].iter().any(|prefix| {
        ct.match_indices(prefix).any(|(i, _)| {
            let rest = &ct[i + prefix.len()..];
            rest.starts_with("json")
                || rest
                    .match_indices("+json")
                    .any(|(j, _)| j > 0 && !rest[..j].contains(';'))
        })
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn response(content_type: &str, body: &str) -> qovery_api::Response {
        let mut res = qovery_api::Response {
            status: 201,
            reason: None,
            body: body.as_bytes().to_vec(),
            headers: Default::default(),
        };
        res.headers
            .insert("content-type", content_type.parse().unwrap());
        res
    }

    #[test]
    fn json_content_types() {
        for ct in [
            "application/json",
            "Application/JSON; charset=utf-8",
            "text/json",
            "application/problem+json",
            "foo application/json",
        ] {
            assert!(is_go_json_content_type(ct), "{ct}");
        }
        for ct in [
            "text/plain",
            "",
            "application/+json",
            "application/x;a+json",
            "json",
        ] {
            assert!(!is_go_json_content_type(ct), "{ct}");
        }
    }

    #[test]
    fn decode() {
        let json = "application/json";
        assert_eq!(go_decode_model(&response(json, ""), &["id"]), Ok(None));
        assert_eq!(go_decode_model(&response(json, "null"), &["id"]), Ok(None));
        assert_eq!(
            go_decode_model(&response("text/plain", "hello"), &["id"]),
            Err("undefined response type".to_string())
        );
        assert_eq!(
            go_decode_model(&response(json, "{bad"), &["id"]),
            Err("invalid character 'b' looking for beginning of object key string".to_string())
        );
        assert_eq!(
            go_decode_model(&response(json, "[1]"), &["id"]),
            Err(
                "json: cannot unmarshal array into Go value of type map[string]interface {}"
                    .to_string()
            )
        );
        assert_eq!(
            go_decode_model(&response(json, r#"{"id":"x"}"#), &["id", "created_at"]),
            Err("no value given for required property created_at".to_string())
        );
        let ok = go_decode_model(&response(json, r#"{"id":"x","name":"n"}"#), &["id"]).unwrap();
        assert_eq!(ok.unwrap()["name"], "n");
    }
}
