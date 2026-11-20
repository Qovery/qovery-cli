//! `--json` output of the `env list` commands and value interpolation (`{{KEY}}`).

use crate::gosort::go_sort_slice;
use crate::lines::{EnvVarLineOutput, go_panic, to_iso8601};
use serde_json::{Map, Value};

/// Go `insertAtIndex(src, insert, index)`: insert `insert` at rune index `index` (unchanged if out of range).
pub fn insert_at_index(src: &str, insert: &str, index: isize) -> String {
    let runes: Vec<char> = src.chars().collect();
    if index < 0 || index as usize > runes.len() {
        return src.to_string();
    }
    let index = index as usize;
    let mut out: String = runes[..index].iter().collect();
    out.push_str(insert);
    out.extend(runes[index..].iter());
    out
}

/// Go `getValueOrDefault(value)`: the value, or `xxx secret xxx` when it is unknown (secret).
pub fn get_value_or_default(value: Option<&str>) -> String {
    match value {
        None => "xxx secret xxx".to_string(),
        Some(v) => v.to_string(),
    }
}

/// Go `getInterpolatedValue(value, variables, aliasParentKey)`: the value with its `{{KEY}}` references
/// replaced by the values of the other variables (recursively, aliases resolved to their parent's value).
///
/// The quirks of the Go scanner are kept (e.g. `a{b{{A}}` -> `a{b1`). Like Go, a malformed reference such as
/// `{{}x{}` makes the program panic (`slice bounds out of range`).
pub fn get_interpolated_value(
    value: Option<&str>,
    variables: &[EnvVarLineOutput],
    alias_parent_key: Option<&str>,
) -> Option<String> {
    let value = value?;

    if let Some(apk) = alias_parent_key
        && let Some(x) = variables.iter().find(|x| x.key == apk)
    {
        return x.value.clone();
    }

    if !value.contains("{{") {
        return Some(value.to_string());
    }

    let runes: Vec<char> = value.chars().collect();

    let mut start_index: isize = -1;
    let mut end_index: isize = -1;
    let mut found_first_first_delimiter = false;
    let mut found_first_last_delimiter = false;
    for (idx, &c) in runes.iter().enumerate() {
        let idx = idx as isize;
        if c == '{' && !found_first_first_delimiter {
            found_first_first_delimiter = true;
        } else if c == '{' {
            start_index = idx - 1;
        } else if start_index > -1 && c == '}' && !found_first_last_delimiter {
            found_first_last_delimiter = true;
        } else if start_index > -1 && c == '}' {
            end_index = idx;
            break;
        }
    }

    if start_index == -1 || end_index == -1 {
        return Some(value.to_string());
    }

    let (start, end) = (start_index as usize, end_index as usize);
    let (lo, hi) = (start + 2, end - 1);
    if lo > hi {
        go_panic(&format!(
            "runtime error: slice bounds out of range [{lo}:{hi}]"
        ));
    }
    let key_to_interpolate: String = runes[lo..hi].iter().collect();

    let value_without_interpolation: String = runes[..start]
        .iter()
        .chain(runes[end + 1..].iter())
        .collect();

    let mut final_value = value.to_string();

    'first_loop: for v in variables {
        if v.key == key_to_interpolate {
            if let Some(apk) = &v.alias_parent_key {
                // where v is an Alias, interpolate the value of the parent key
                if let Some(x) = variables.iter().find(|x| &x.key == apk) {
                    final_value = insert_at_index(
                        &value_without_interpolation,
                        &get_value_or_default(x.value.as_deref()),
                        start_index,
                    );
                    continue 'first_loop;
                }
            }
            final_value = insert_at_index(
                &value_without_interpolation,
                &get_value_or_default(v.value.as_deref()),
                start_index,
            );
            break;
        }
    }

    if final_value.contains("{{") && final_value != value {
        return get_interpolated_value(Some(&final_value), variables, None);
    }

    Some(final_value)
}

/// Go `utils.GetEnvVarJsonOutput(variables, sortKeys)`: the `--json` output of the `env list` commands
/// (`null` when there is no variable). With `sort_keys`, variables are sorted by key with Go's (unstable)
/// `sort.Slice`, so variables sharing a key keep Go's order.
pub fn get_env_var_json_output(variables: &[EnvVarLineOutput], sort_keys: bool) -> String {
    let mut sorted;
    let variables = if sort_keys {
        sorted = variables.to_vec();
        go_sort_slice(&mut sorted, |a, b| a.key < b.key);
        &sorted[..]
    } else {
        variables
    };

    if variables.is_empty() {
        return "null".to_string();
    }

    let opt = |v: Option<String>| v.map(Value::String).unwrap_or(Value::Null);
    let results: Vec<Value> = variables
        .iter()
        .map(|v| {
            let mut m = Map::new();
            m.insert("id".into(), Value::String(v.id.clone()));
            m.insert(
                "created_at".into(),
                Value::String(to_iso8601(&v.created_at)),
            );
            m.insert(
                "updated_at".into(),
                opt(v.updated_at.as_deref().map(to_iso8601)),
            );
            m.insert("key".into(), Value::String(v.key.clone()));
            m.insert("value".into(), opt(v.value.clone()));
            m.insert(
                "interpolated_value".into(),
                opt(get_interpolated_value(
                    v.value.as_deref(),
                    variables,
                    v.alias_parent_key.as_deref(),
                )),
            );
            m.insert("service_name".into(), opt(v.service.clone()));
            m.insert("scope".into(), Value::String(v.scope.clone()));
            m.insert("alias_parent_key".into(), opt(v.alias_parent_key.clone()));
            m.insert(
                "override_parent_value".into(),
                opt(v.override_parent_key.clone()),
            );
            Value::Object(m)
        })
        .collect();

    qovery_log::gofmt::go_json(&Value::Array(results))
}
