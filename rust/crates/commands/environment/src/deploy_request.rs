//! `json.Unmarshal([]byte(servicesJson), &deployAllRequest)` (`environment deploy --services`) followed by the Go
//! client marshalling of the `qovery.DeployAllRequest` model.
//!
//! Reproduces the generated decoders of `qovery-client-go`: entries are decoded in document order and the first
//! error is returned; known fields match case-insensitively (`encoding/json`) while the required-property check
//! and the removal of known keys from the additional properties are exact; nested models run their own
//! `json.Unmarshal` (so their errors carry no outer path).

use serde::de::{Deserialize, Deserializer, MapAccess, Visitor};
use serde_json::value::RawValue;
use serde_json::{Map, Value};
use std::fmt;

/// Object entries in document order (duplicates kept).
struct Entries(Vec<(String, Box<RawValue>)>);

impl<'de> Deserialize<'de> for Entries {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        struct V;
        impl<'de> Visitor<'de> for V {
            type Value = Entries;
            fn expecting(&self, f: &mut fmt::Formatter) -> fmt::Result {
                f.write_str("a JSON object")
            }
            fn visit_map<A: MapAccess<'de>>(self, mut map: A) -> Result<Entries, A::Error> {
                let mut entries = Vec::new();
                while let Some(entry) = map.next_entry::<String, Box<RawValue>>()? {
                    entries.push(entry);
                }
                Ok(Entries(entries))
            }
        }
        d.deserialize_map(V)
    }
}

/// `encoding/json` name of the JSON kind of a (valid) raw value, as used in `UnmarshalTypeError`.
fn kind(raw: &RawValue) -> &'static str {
    match raw.get().trim_start().as_bytes().first() {
        Some(b'{') => "object",
        Some(b'[') => "array",
        Some(b'"') => "string",
        Some(b't' | b'f') => "bool",
        Some(b'n') => "null",
        _ => "number",
    }
}

fn is_null(raw: &RawValue) -> bool {
    kind(raw) == "null"
}

fn entries(raw: &RawValue) -> Result<Vec<(String, Box<RawValue>)>, String> {
    serde_json::from_str::<Entries>(raw.get())
        .map(|e| e.0)
        .map_err(|e| e.to_string())
}

fn array(raw: &RawValue) -> Result<Vec<Box<RawValue>>, String> {
    serde_json::from_str(raw.get()).map_err(|e| e.to_string())
}

fn string(raw: &RawValue) -> Result<String, String> {
    serde_json::from_str(raw.get()).map_err(|e| e.to_string())
}

fn value(raw: &RawValue) -> Result<Value, String> {
    serde_json::from_str(raw.get()).map_err(|e| e.to_string())
}

/// `encoding/json` field name matching (case-insensitive).
fn matches(key: &str, field: &str) -> bool {
    key.eq_ignore_ascii_case(field) || key.to_lowercase() == field
}

/// The Go additional properties: every key but the exact known ones (the last duplicate wins).
fn additional_properties(
    entries: &[(String, Box<RawValue>)],
    known: &[&str],
) -> Result<Map<String, Value>, String> {
    let mut out = Map::new();
    for (k, v) in entries {
        if !known.contains(&k.as_str()) {
            out.insert(k.clone(), value(v)?);
        }
    }
    Ok(out)
}

#[derive(Clone, Copy)]
enum FieldKind {
    /// `string` (required property): `null` leaves the previous value, always marshalled.
    Required,
    /// `*string`: `null` resets it, marshalled when set.
    Optional,
    /// `NullableString`: decoded by its own `json.Unmarshal`, marshalled (`null` included) when present.
    Nullable,
}

struct Model {
    /// Go type name of the decoded struct (`_DeployAllRequestApplicationsInner`).
    go_type: &'static str,
    fields: &'static [(&'static str, FieldKind)],
    required: &'static [&'static str],
}

const APPLICATION: Model = Model {
    go_type: "_DeployAllRequestApplicationsInner",
    fields: &[
        ("application_id", FieldKind::Required),
        ("git_commit_id", FieldKind::Optional),
    ],
    required: &["application_id"],
};
const CONTAINER: Model = Model {
    go_type: "_DeployAllRequestContainersInner",
    fields: &[
        ("id", FieldKind::Required),
        ("image_tag", FieldKind::Optional),
    ],
    required: &["id"],
};
const JOB: Model = Model {
    go_type: "_DeployAllRequestJobsInner",
    fields: &[
        ("id", FieldKind::Optional),
        ("image_tag", FieldKind::Optional),
        ("git_commit_id", FieldKind::Optional),
    ],
    required: &[],
};
const HELM: Model = Model {
    go_type: "_DeployAllRequestHelmsInner",
    fields: &[
        ("id", FieldKind::Optional),
        ("chart_version", FieldKind::Optional),
        ("git_commit_id", FieldKind::Optional),
        ("values_override_git_commit_id", FieldKind::Optional),
    ],
    required: &[],
};
const TERRAFORM: Model = Model {
    go_type: "_TerraformDeployRequest",
    fields: &[
        ("id", FieldKind::Nullable),
        ("git_commit_id", FieldKind::Optional),
        ("action", FieldKind::Nullable),
    ],
    required: &[],
};

/// `(*Model).UnmarshalJSON` + `ToMap` of one list element.
fn decode_model(model: &Model, raw: &RawValue) -> Result<Value, String> {
    let k = kind(raw);
    if !model.required.is_empty() {
        // json.Unmarshal(data, &map[string]interface{}) then the required-property check
        if k == "null" {
            return Err(format!(
                "no value given for required property {}",
                model.required[0]
            ));
        }
        if k != "object" {
            return Err(format!(
                "json: cannot unmarshal {k} into Go value of type map[string]interface {{}}"
            ));
        }
        let entries = entries(raw)?;
        for required in model.required {
            if !entries.iter().any(|(key, _)| key == required) {
                return Err(format!("no value given for required property {required}"));
            }
        }
    } else if k == "null" {
        return Ok(Value::Object(Map::new()));
    } else if k != "object" {
        return Err(format!(
            "json: cannot unmarshal {k} into Go value of type qovery.{}",
            model.go_type
        ));
    }

    let entries = entries(raw)?;
    let mut values: Vec<Option<Value>> = model
        .fields
        .iter()
        .map(|(_, kind)| match kind {
            FieldKind::Required => Some(Value::String(String::new())),
            _ => None,
        })
        .collect();
    for (key, v) in &entries {
        let Some(i) = model.fields.iter().position(|(f, _)| matches(key, f)) else {
            continue;
        };
        let vk = kind(v);
        values[i] = match (model.fields[i].1, vk) {
            (FieldKind::Required, "null") => continue,
            (FieldKind::Optional, "null") => None,
            (FieldKind::Nullable, "null") => Some(Value::Null),
            (_, "string") => Some(Value::String(string(v)?)),
            (FieldKind::Nullable, _) => {
                return Err(format!(
                    "json: cannot unmarshal {vk} into Go value of type string"
                ));
            }
            _ => {
                return Err(format!(
                    "json: cannot unmarshal {vk} into Go struct field {}.{key} of type string",
                    model.go_type
                ));
            }
        };
    }

    let known: Vec<&str> = model.fields.iter().map(|(f, _)| *f).collect();
    let mut out = additional_properties(&entries, &known)?;
    for ((name, _), v) in model.fields.iter().zip(values) {
        if let Some(v) = v {
            out.insert((*name).to_string(), v);
        }
    }
    Ok(Value::Object(out))
}

#[derive(Clone, Copy)]
enum ListKind {
    Strings,
    Models(&'static Model),
}

const FIELDS: &[(&str, &str, ListKind)] = &[
    (
        "applications",
        "[]qovery.DeployAllRequestApplicationsInner",
        ListKind::Models(&APPLICATION),
    ),
    ("databases", "[]string", ListKind::Strings),
    (
        "containers",
        "[]qovery.DeployAllRequestContainersInner",
        ListKind::Models(&CONTAINER),
    ),
    (
        "jobs",
        "[]qovery.DeployAllRequestJobsInner",
        ListKind::Models(&JOB),
    ),
    (
        "helms",
        "[]qovery.DeployAllRequestHelmsInner",
        ListKind::Models(&HELM),
    ),
    (
        "terraforms",
        "[]qovery.TerraformDeployRequest",
        ListKind::Models(&TERRAFORM),
    ),
    ("agentic_workflows", "[]string", ListKind::Strings),
];

/// Decodes `--services` like Go and returns the JSON body the Go client sends.
pub(crate) fn parse_deploy_all_request(s: &str) -> Result<Value, String> {
    qovery_log::gojson::go_json_check_valid(s.as_bytes())?;
    let raw: Box<RawValue> = serde_json::from_str(s).map_err(|e| e.to_string())?;
    match kind(&raw) {
        "null" => return Ok(Value::Object(Map::new())),
        "object" => {}
        k => {
            return Err(format!(
                "json: cannot unmarshal {k} into Go value of type qovery._DeployAllRequest"
            ));
        }
    }

    let entries = entries(&raw)?;
    let mut lists: Vec<Option<Vec<Value>>> = vec![None; FIELDS.len()];
    for (key, v) in &entries {
        let Some(i) = FIELDS.iter().position(|(f, _, _)| matches(key, f)) else {
            continue;
        };
        let (_, go_type, list_kind) = FIELDS[i];
        let vk = kind(v);
        if vk == "null" {
            lists[i] = None;
            continue;
        }
        if vk != "array" {
            return Err(format!(
                "json: cannot unmarshal {vk} into Go struct field _DeployAllRequest.{key} of type {go_type}"
            ));
        }
        let mut items = Vec::new();
        for (idx, item) in array(v)?.iter().enumerate() {
            items.push(match list_kind {
                ListKind::Models(model) => decode_model(model, item)?,
                ListKind::Strings if is_null(item) => Value::String(String::new()),
                ListKind::Strings if kind(item) == "string" => Value::String(string(item)?),
                ListKind::Strings => {
                    return Err(format!(
                        "json: cannot unmarshal {} into _DeployAllRequest.{key}.{idx} of type string",
                        kind(item)
                    ));
                }
            });
        }
        lists[i] = Some(items);
    }

    let known: Vec<&str> = FIELDS.iter().map(|(f, _, _)| *f).collect();
    let mut out = additional_properties(&entries, &known)?;
    for ((name, _, _), list) in FIELDS.iter().zip(lists) {
        if let Some(list) = list {
            out.insert((*name).to_string(), Value::Array(list));
        }
    }
    Ok(Value::Object(out))
}

#[cfg(test)]
mod tests {
    use super::parse_deploy_all_request as parse;
    use serde_json::json;

    #[test]
    fn nominal_and_nulls() {
        assert_eq!(parse("null").unwrap(), json!({}));
        assert_eq!(
            parse(r#"{"applications":[{"application_id":"a","git_commit_id":null}],"databases":["d",null],"jobs":null,"x":null}"#).unwrap(),
            json!({"applications":[{"application_id":"a"}],"databases":["d",""],"x":null})
        );
        assert_eq!(
            parse(r#"{"applications":[{"application_id":null}],"jobs":[null,{"id":null}],"terraforms":[{"id":null,"action":null,"git_commit_id":null}]}"#).unwrap(),
            json!({"applications":[{"application_id":""}],"jobs":[{},{}],"terraforms":[{"id":null,"action":null}]})
        );
        // case-insensitive fields, exact additional properties
        assert_eq!(
            parse(r#"{"Databases":["a"],"jobs":[{"ID":"j"}]}"#).unwrap(),
            json!({"Databases":["a"],"databases":["a"],"jobs":[{"ID":"j","id":"j"}]})
        );
    }

    #[test]
    fn go_errors() {
        let cases = [
            (
                "[]",
                "json: cannot unmarshal array into Go value of type qovery._DeployAllRequest",
            ),
            (
                r#"{"applications":"x"}"#,
                "json: cannot unmarshal string into Go struct field _DeployAllRequest.applications of type []qovery.DeployAllRequestApplicationsInner",
            ),
            (
                r#"{"DATABASES":"x"}"#,
                "json: cannot unmarshal string into Go struct field _DeployAllRequest.DATABASES of type []string",
            ),
            (
                r#"{"databases":["a",1]}"#,
                "json: cannot unmarshal number into _DeployAllRequest.databases.1 of type string",
            ),
            (
                r#"{"applications":[1]}"#,
                "json: cannot unmarshal number into Go value of type map[string]interface {}",
            ),
            (
                r#"{"applications":[null]}"#,
                "no value given for required property application_id",
            ),
            (
                r#"{"containers":[{"image_tag":"t"}]}"#,
                "no value given for required property id",
            ),
            (
                r#"{"applications":[{"git_commit_id":5,"application_id":2}]}"#,
                "json: cannot unmarshal number into Go struct field _DeployAllRequestApplicationsInner.git_commit_id of type string",
            ),
            (
                r#"{"jobs":[1]}"#,
                "json: cannot unmarshal number into Go value of type qovery._DeployAllRequestJobsInner",
            ),
            (
                r#"{"terraforms":[{"id":3}]}"#,
                "json: cannot unmarshal number into Go value of type string",
            ),
            (
                r#"{"databases":"x","applications":[{}]}"#,
                "json: cannot unmarshal string into Go struct field _DeployAllRequest.databases of type []string",
            ),
            (
                r#"{"applications":[{}],"databases":"x"}"#,
                "no value given for required property application_id",
            ),
        ];
        for (input, err) in cases {
            assert_eq!(parse(input).unwrap_err(), err, "{input}");
        }
    }
}
