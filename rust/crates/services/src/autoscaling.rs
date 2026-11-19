//! Port of `utils/autoscaling.go`.

use serde_json::{Map, Value};

/// Port of `utils.ConvertAutoscalingResponseToRequest(resp)`: turns the `autoscaling` field of a service
/// (`AutoscalingPolicyResponse`, KEDA only) into the `AutoscalingPolicyRequest` JSON of an update call,
/// preserving the KEDA configuration. `None` (Go nil, field omitted) when there is no KEDA policy.
///
/// Serialization follows the Go client: `scalers` is `null` when the policy has no scaler,
/// `config_json` / `config_yaml` / `trigger_authentication` are omitted when null.
pub fn convert_autoscaling_response_to_request(resp: Option<&Value>) -> Option<Value> {
    let keda = resp.filter(|v| v.is_object())?;

    let scalers: Option<Vec<Value>> = keda
        .get("scalers")
        .and_then(|s| s.as_array())
        .filter(|a| !a.is_empty())
        .map(|list| {
            list.iter()
                .map(|s| {
                    let mut out = Map::new();
                    out.insert(
                        "scaler_type".into(),
                        Value::String(
                            s.get("scaler_type")
                                .and_then(|x| x.as_str())
                                .unwrap_or("")
                                .into(),
                        ),
                    );
                    out.insert(
                        "enabled".into(),
                        Value::Bool(s.get("enabled").and_then(|x| x.as_bool()).unwrap_or(false)),
                    );
                    out.insert(
                        "role".into(),
                        Value::String(s.get("role").and_then(|x| x.as_str()).unwrap_or("").into()),
                    );
                    if let Some(cj) = s.get("config_json").filter(|x| x.is_object()) {
                        out.insert("config_json".into(), cj.clone());
                    }
                    if let Some(cy) = s.get("config_yaml").and_then(|x| x.as_str()) {
                        out.insert("config_yaml".into(), Value::String(cy.into()));
                    }
                    if let Some(ta) = s.get("trigger_authentication").filter(|x| x.is_object()) {
                        let mut t = Map::new();
                        t.insert(
                            "name".into(),
                            Value::String(
                                ta.get("name").and_then(|x| x.as_str()).unwrap_or("").into(),
                            ),
                        );
                        if let Some(cy) = ta.get("config_yaml").and_then(|x| x.as_str()) {
                            t.insert("config_yaml".into(), Value::String(cy.into()));
                        }
                        out.insert("trigger_authentication".into(), Value::Object(t));
                    }
                    Value::Object(out)
                })
                .collect()
        });

    let num = |k: &str| keda.get(k).and_then(|x| x.as_i64()).unwrap_or(0);
    let mut req = Map::new();
    req.insert(
        "mode".into(),
        Value::String(
            keda.get("mode")
                .and_then(|x| x.as_str())
                .unwrap_or("")
                .into(),
        ),
    );
    req.insert(
        "polling_interval_seconds".into(),
        Value::from(num("polling_interval_seconds")),
    );
    req.insert(
        "cooldown_period_seconds".into(),
        Value::from(num("cooldown_period_seconds")),
    );
    req.insert(
        "scalers".into(),
        scalers.map(Value::Array).unwrap_or(Value::Null),
    );
    Some(Value::Object(req))
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn converts_keda_policy() {
        let resp = json!({
            "id": "a", "created_at": "2024-01-01T00:00:00Z", "service_id": "s", "mode": "KEDA",
            "polling_interval_seconds": 30, "cooldown_period_seconds": 300,
            "scalers": [
                {"id": "s1", "created_at": "x", "scaler_type": "prometheus", "enabled": true, "role": "PRIMARY",
                 "config_json": {"a": 1}, "config_yaml": null,
                 "trigger_authentication": {"id": "t", "created_at": "x", "organization_id": "o", "name": "auth", "config_yaml": "k: v"}},
                {"id": "s2", "created_at": "x", "scaler_type": "cpu", "enabled": false, "role": "SECONDARY", "config_yaml": "x: 1"}
            ]
        });
        assert_eq!(
            convert_autoscaling_response_to_request(Some(&resp)).unwrap(),
            json!({
                "mode": "KEDA", "polling_interval_seconds": 30, "cooldown_period_seconds": 300,
                "scalers": [
                    {"scaler_type": "prometheus", "enabled": true, "role": "PRIMARY", "config_json": {"a": 1},
                     "trigger_authentication": {"name": "auth", "config_yaml": "k: v"}},
                    {"scaler_type": "cpu", "enabled": false, "role": "SECONDARY", "config_yaml": "x: 1"}
                ]
            })
        );
        let no_scaler = json!({"mode": "KEDA", "polling_interval_seconds": 1, "cooldown_period_seconds": 2, "scalers": []});
        assert_eq!(
            convert_autoscaling_response_to_request(Some(&no_scaler)).unwrap()["scalers"],
            Value::Null
        );
        assert_eq!(convert_autoscaling_response_to_request(None), None);
        assert_eq!(
            convert_autoscaling_response_to_request(Some(&Value::Null)),
            None
        );
    }
}
