//! Decoding of `CheckBlueprintUpdate`'s response like the Go client: `APIClient.decode` into a
//! `*BlueprintUpdateResponse`, whose generated `UnmarshalJSON` checks the required properties then decodes the
//! fields (`encoding/json` reports the first error in document order). The new majors are checked completely
//! (types, required properties, `BlueprintMajorSwitchMode`); for the other lists and `engine_diff`, only their
//! JSON kinds and the required properties of their items are.

use serde::de::{Deserialize, Deserializer, MapAccess, SeqAccess, Visitor};

/// A JSON value keeping the document order of the object keys (Go reports the first error in that order).
#[derive(Debug, Clone, PartialEq)]
enum Json {
    Null,
    Bool,
    Number,
    String(String),
    Array(Vec<Json>),
    Object(Vec<(String, Json)>),
}

impl Json {
    /// The kind `encoding/json` names in its type errors.
    fn kind(&self) -> &'static str {
        match self {
            Json::Null => "null",
            Json::Bool => "bool",
            Json::Number => "number",
            Json::String(_) => "string",
            Json::Array(_) => "array",
            Json::Object(_) => "object",
        }
    }
}

impl<'de> Deserialize<'de> for Json {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        struct V;
        impl<'de> Visitor<'de> for V {
            type Value = Json;
            fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
                f.write_str("a JSON value")
            }
            fn visit_unit<E>(self) -> Result<Json, E> {
                Ok(Json::Null)
            }
            fn visit_none<E>(self) -> Result<Json, E> {
                Ok(Json::Null)
            }
            fn visit_bool<E>(self, _: bool) -> Result<Json, E> {
                Ok(Json::Bool)
            }
            fn visit_i64<E>(self, _: i64) -> Result<Json, E> {
                Ok(Json::Number)
            }
            fn visit_u64<E>(self, _: u64) -> Result<Json, E> {
                Ok(Json::Number)
            }
            fn visit_f64<E>(self, _: f64) -> Result<Json, E> {
                Ok(Json::Number)
            }
            fn visit_str<E>(self, s: &str) -> Result<Json, E> {
                Ok(Json::String(s.to_string()))
            }
            fn visit_string<E>(self, s: String) -> Result<Json, E> {
                Ok(Json::String(s))
            }
            fn visit_seq<A: SeqAccess<'de>>(self, mut seq: A) -> Result<Json, A::Error> {
                let mut items = Vec::new();
                while let Some(item) = seq.next_element()? {
                    items.push(item);
                }
                Ok(Json::Array(items))
            }
            fn visit_map<A: MapAccess<'de>>(self, mut map: A) -> Result<Json, A::Error> {
                let mut entries = Vec::new();
                while let Some(entry) = map.next_entry::<String, Json>()? {
                    entries.push(entry);
                }
                Ok(Json::Object(entries))
            }
        }
        d.deserialize_any(V)
    }
}

/// `qovery.AllowedBlueprintMajorSwitchModeEnumValues`.
const SWITCH_MODES: [&str; 3] = ["RELABEL", "UPGRADE", "UNSUPPORTED"];

/// The error of the Go client decoding a successful `CheckBlueprintUpdate` response, if any.
pub(crate) fn check_update_response(body: &[u8], content_type: &str) -> Result<(), String> {
    if body.is_empty() {
        return Ok(());
    }
    if !is_go_json_content_type(content_type) {
        return Err("undefined response type".to_string());
    }
    qovery_log::gojson::go_json_check_valid(body)?;
    let value: Json = serde_json::from_slice(body).map_err(|e| e.to_string())?;
    if value == Json::Null {
        // the `*BlueprintUpdateResponse` stays nil
        return Ok(());
    }
    let fields = model_fields(
        &value,
        &[
            "is_up_to_date",
            "current_tag",
            "latest_tag",
            "new_required_values",
            "new_optional_values",
            "now_required_values",
            "updated_values",
            "removed_values",
            "engine_diff",
            "new_major_versions",
        ],
    )?;
    const RESPONSE: &str = "_BlueprintUpdateResponse";
    for (key, v) in fields {
        match key.as_str() {
            "is_up_to_date" => scalar(v, RESPONSE, key, "bool")?,
            "current_tag" | "latest_tag" => scalar(v, RESPONSE, key, "string")?,
            "new_required_values" | "now_required_values" => model_list(
                v,
                key,
                "BlueprintUpdateNewRequiredValue",
                &["name", "type", "is_secret"],
            )?,
            "new_optional_values" => model_list(
                v,
                key,
                "BlueprintUpdateNewOptionalValue",
                &["name", "type", "is_secret"],
            )?,
            "updated_values" => model_list(
                v,
                key,
                "BlueprintUpdateUpdatedValue",
                &["name", "type", "is_secret"],
            )?,
            "removed_values" => model_list(v, key, "BlueprintUpdateRemovedValue", &["name"])?,
            "engine_diff" => {
                model_fields(v, &["updated_values"])?;
            }
            "new_major_versions" => match v {
                Json::Null => {}
                Json::Array(items) => items.iter().try_for_each(check_new_major)?,
                other => {
                    return Err(struct_field_error(
                        other,
                        RESPONSE,
                        key,
                        "[]qovery.BlueprintUpdateNewMajorVersion",
                    ));
                }
            },
            _ => {}
        }
    }
    Ok(())
}

/// `BlueprintUpdateNewMajorVersion.UnmarshalJSON`.
fn check_new_major(item: &Json) -> Result<(), String> {
    const MAJOR: &str = "_BlueprintUpdateNewMajorVersion";
    for (key, v) in model_fields(item, &["service_version", "latest_tag", "mode"])? {
        match key.as_str() {
            "service_version" | "latest_tag" => scalar(v, MAJOR, key, "string")?,
            // `BlueprintMajorSwitchMode.UnmarshalJSON` (also called for `null`, which leaves "")
            "mode" => {
                let mode = match v {
                    Json::Null => "",
                    Json::String(s) => s.as_str(),
                    other => return Err(go_value_error(other, "string")),
                };
                if !SWITCH_MODES.contains(&mode) {
                    return Err(format!("{mode} is not a valid BlueprintMajorSwitchMode"));
                }
            }
            // `NullableString.UnmarshalJSON`
            "unsupported_reason" if !matches!(v, Json::Null | Json::String(_)) => {
                return Err(go_value_error(v, "string"));
            }
            _ => {}
        }
    }
    Ok(())
}

/// The start of a generated `UnmarshalJSON`: decoding into `map[string]interface{}` (`null` gives an empty
/// map) and the required properties check. Returns the fields in document order.
fn model_fields<'a>(value: &'a Json, required: &[&str]) -> Result<&'a [(String, Json)], String> {
    let fields: &[(String, Json)] = match value {
        Json::Null => &[],
        Json::Object(fields) => fields,
        other => return Err(go_value_error(other, "map[string]interface {}")),
    };
    if let Some(p) = required
        .iter()
        .find(|p| !fields.iter().any(|(k, _)| k == *p))
    {
        return Err(format!("no value given for required property {p}"));
    }
    Ok(fields)
}

/// A `[]Model` field of `_BlueprintUpdateResponse`: its kind and the start of each item's `UnmarshalJSON`.
fn model_list(value: &Json, field: &str, model: &str, required: &[&str]) -> Result<(), String> {
    match value {
        Json::Null => Ok(()),
        Json::Array(items) => items
            .iter()
            .try_for_each(|item| model_fields(item, required).map(|_| ())),
        other => Err(struct_field_error(
            other,
            "_BlueprintUpdateResponse",
            field,
            &format!("[]qovery.{model}"),
        )),
    }
}

/// A `string` / `bool` struct field (`null` keeps the zero value).
fn scalar(value: &Json, strukt: &str, field: &str, go_type: &str) -> Result<(), String> {
    match (value, go_type) {
        (Json::Null, _) | (Json::String(_), "string") | (Json::Bool, "bool") => Ok(()),
        (other, _) => Err(struct_field_error(other, strukt, field, go_type)),
    }
}

fn struct_field_error(value: &Json, strukt: &str, field: &str, go_type: &str) -> String {
    format!(
        "json: cannot unmarshal {} into Go struct field {strukt}.{field} of type {go_type}",
        value.kind()
    )
}

fn go_value_error(value: &Json, go_type: &str) -> String {
    format!(
        "json: cannot unmarshal {} into Go value of type {go_type}",
        value.kind()
    )
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

    const JSON: &str = "application/json";

    fn response(majors: &str) -> String {
        format!(
            r#"{{"is_up_to_date":false,"current_tag":"t","latest_tag":"t","new_required_values":[],
            "new_optional_values":[],"now_required_values":[],"updated_values":[],"removed_values":[],
            "engine_diff":{{"updated_values":[]}},"new_major_versions":{majors}}}"#
        )
    }

    fn check(majors: &str) -> Result<(), String> {
        check_update_response(response(majors).as_bytes(), JSON)
    }

    #[test]
    fn valid_responses() {
        assert_eq!(check_update_response(b"", JSON), Ok(()));
        assert_eq!(check_update_response(b"null", JSON), Ok(()));
        assert_eq!(check("null"), Ok(()));
        assert_eq!(check("[]"), Ok(()));
        assert_eq!(
            check(
                r#"[{"service_version":"18","latest_tag":"x","mode":"UPGRADE","unsupported_reason":null,"extra":1}]"#
            ),
            Ok(())
        );
        assert_eq!(
            check(r#"[{"service_version":null,"latest_tag":"x","mode":"RELABEL"}]"#),
            Ok(())
        );
    }

    #[test]
    fn envelope_errors() {
        assert_eq!(
            check_update_response(b"hi", "text/plain"),
            Err("undefined response type".to_string())
        );
        assert_eq!(
            check_update_response(b"{x", JSON),
            Err("invalid character 'x' looking for beginning of object key string".to_string())
        );
        assert_eq!(
            check_update_response(b"[]", JSON),
            Err(
                "json: cannot unmarshal array into Go value of type map[string]interface {}"
                    .to_string()
            )
        );
        assert_eq!(
            check_update_response(br#"{"is_up_to_date":false}"#, JSON),
            Err("no value given for required property current_tag".to_string())
        );
    }

    #[test]
    fn major_errors() {
        let e = |s: &str| Err(s.to_string());
        assert_eq!(
            check(r#"[{"service_version":"18","latest_tag":"x","mode":5}]"#),
            e("json: cannot unmarshal number into Go value of type string")
        );
        assert_eq!(
            check(r#"[{"service_version":"18","latest_tag":"x","mode":null}]"#),
            e(" is not a valid BlueprintMajorSwitchMode")
        );
        assert_eq!(
            check(r#"[{"service_version":18,"latest_tag":"x","mode":"UPGRADE"}]"#),
            e(
                "json: cannot unmarshal number into Go struct field _BlueprintUpdateNewMajorVersion.service_version of type string"
            )
        );
        // the first error in document order
        assert_eq!(
            check(r#"[{"mode":5,"service_version":18,"latest_tag":"x"}]"#),
            e("json: cannot unmarshal number into Go value of type string")
        );
        assert_eq!(
            check(
                r#"[{"service_version":"18","latest_tag":"x","mode":"UPGRADE","unsupported_reason":3}]"#
            ),
            e("json: cannot unmarshal number into Go value of type string")
        );
        assert_eq!(
            check("[null]"),
            e("no value given for required property service_version")
        );
        assert_eq!(
            check(r#"["x"]"#),
            e("json: cannot unmarshal string into Go value of type map[string]interface {}")
        );
        assert_eq!(
            check(r#"{"a":1}"#),
            e(
                "json: cannot unmarshal object into Go struct field _BlueprintUpdateResponse.new_major_versions of type []qovery.BlueprintUpdateNewMajorVersion"
            )
        );
    }

    #[test]
    fn other_field_errors() {
        let body = response("[]").replace(r#""current_tag":"t""#, r#""current_tag":5"#);
        assert_eq!(
            check_update_response(body.as_bytes(), JSON),
            Err(
                "json: cannot unmarshal number into Go struct field _BlueprintUpdateResponse.current_tag of type string"
                    .to_string()
            )
        );
        let body = response("[]").replace(r#""removed_values":[]"#, r#""removed_values":[1]"#);
        assert_eq!(
            check_update_response(body.as_bytes(), JSON),
            Err(
                "json: cannot unmarshal number into Go value of type map[string]interface {}"
                    .to_string()
            )
        );
    }
}
