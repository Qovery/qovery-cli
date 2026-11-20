//! Compare the formatting helpers with outputs of the Go implementation.
//!
//! `tests/data/inputs.json` holds API variable lists; `tests/data/go_format_expected.json` and
//! `tests/data/go_sort_expected.json` were produced by running the Go code of `utils/env_var.go` and
//! `sort.Slice` on them (throw-away Go tests, `TZ=UTC`).

use qovery_envvars::*;
use serde_json::Value;
use std::collections::BTreeMap;

fn load(name: &str) -> Value {
    let path = format!("{}/tests/data/{name}", env!("CARGO_MANIFEST_DIR"));
    serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap()
}

fn as_rows(v: &Value) -> Vec<Vec<String>> {
    match v {
        Value::Null => vec![],
        Value::Array(rows) => rows
            .iter()
            .map(|r| {
                r.as_array()
                    .unwrap()
                    .iter()
                    .map(|c| c.as_str().unwrap().to_string())
                    .collect()
            })
            .collect(),
        _ => panic!("unexpected {v}"),
    }
}

#[test]
fn format_matches_go() {
    let inputs = load("inputs.json");
    let expected = load("go_format_expected.json");
    let mut checked = 0;
    for (name, set) in inputs.as_object().unwrap() {
        let exp = &expected[name];
        let vars: Vec<VariableResponse> = serde_json::from_value(set["variables"].clone()).unwrap();
        let mut lines = EnvVarLines::new();
        let mut variables = Vec::new();
        for v in &vars {
            let s = from_environment_variable_to_env_var_line_output(v);
            variables.push(s.clone());
            lines.add(s);
        }

        assert_eq!(
            get_env_var_json_output(&variables, true),
            exp["json_sorted"].as_str().unwrap(),
            "{name} json sorted"
        );
        assert_eq!(
            get_env_var_json_output(&variables, false),
            exp["json_unsorted"].as_str().unwrap(),
            "{name} json"
        );
        checked += 2;

        if !set["lines"].as_bool().unwrap() {
            continue;
        }
        let hp: Vec<String> = lines.header(true).iter().map(|s| s.to_string()).collect();
        let hn: Vec<String> = lines.header(false).iter().map(|s| s.to_string()).collect();
        assert_eq!(
            hp,
            serde_json::from_value::<Vec<String>>(exp["header_pretty"].clone()).unwrap()
        );
        assert_eq!(
            hn,
            serde_json::from_value::<Vec<String>>(exp["header_plain"].clone()).unwrap()
        );

        for (combo, rows) in exp["lines"].as_object().unwrap() {
            let f: Vec<bool> = combo.split('-').map(|b| b == "true").collect();
            assert_eq!(
                lines.lines(f[0], f[1], f[2]),
                as_rows(rows),
                "{name} lines {combo}"
            );
            checked += 1;
        }

        let mut data = Vec::new();
        for v in &variables {
            data.push(v.data(false));
            data.push(v.data(true));
        }
        assert_eq!(data, as_rows(&exp["data"]), "{name} data");
        checked += 1;
    }
    assert!(checked > 20);
}

#[test]
fn go_sort_slice_matches_go() {
    let expected: BTreeMap<String, Vec<usize>> =
        serde_json::from_value(load("go_sort_expected.json")).unwrap();
    assert!(!expected.is_empty());
    for (case, exp) in expected {
        let parts: Vec<&str> = case.split('-').collect();
        let n: usize = parts[0].parse().unwrap();
        let k: usize = parts[1].parse().unwrap();
        let mut v: Vec<(usize, usize)> = (0..n)
            .map(|i| {
                let key = match parts[2] {
                    "desc" => (n - i) / (n / k + 1),
                    "asc" => i / (n / k + 1),
                    _ => (i * 7919 + 13) % k,
                };
                (key, i)
            })
            .collect();
        go_sort_slice(&mut v, |a, b| a.0 < b.0);
        let got: Vec<usize> = v.iter().map(|e| e.1).collect();
        assert_eq!(got, exp, "case {case}");
    }
}

#[test]
fn interpolation_helpers() {
    assert_eq!(insert_at_index("héllo", "X", 2), "héXllo");
    assert_eq!(insert_at_index("abc", "X", 3), "abcX");
    assert_eq!(insert_at_index("abc", "X", 4), "abc");
    assert_eq!(insert_at_index("abc", "X", -1), "abc");
    assert_eq!(get_value_or_default(None), "xxx secret xxx");
    assert_eq!(get_value_or_default(Some("v")), "v");
    assert_eq!(get_interpolated_value(None, &[], None), None);
}

#[test]
fn time_formats() {
    assert_eq!(
        format_rfc822("2024-03-04T05:06:07.987654321Z"),
        "04 Mar 24 05:06 UTC"
    );
    assert_eq!(
        format_rfc822("2024-03-04T05:06:07+02:00"),
        "04 Mar 24 05:06 +0200"
    );
    assert_eq!(
        format_rfc822("2024-03-04T05:06:07+00:00"),
        "04 Mar 24 05:06 UTC"
    );
    assert_eq!(
        to_iso8601("2024-03-04T05:06:07.987654321Z"),
        "2024-03-04T05:06:07.987Z"
    );
    assert_eq!(
        to_iso8601("2024-12-31T23:59:59-05:30"),
        "2024-12-31T23:59:59.000Z"
    );
}
