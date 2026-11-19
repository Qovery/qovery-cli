//! `qovery list-pods` command. Port of `cmd/service_list_pods.go` and `pkg/service_list_pods.go`.

use futures_util::StreamExt;
use qovery_cmd_port_forward::{
    PortForwardRequest, request_with_application_url, request_without_arg,
};
use qovery_cmd_shell::ws;
use qovery_cobra::{Command, Ctx};
use qovery_core::telemetry;

/// The `list-pods` command.
pub fn command() -> Command {
    Command::new("list-pods")
        .short("List the pods of a service with their pods")
        .run(run_list_pods)
}

fn run_list_pods(ctx: &Ctx) {
    telemetry::capture(ctx);

    // the --pod flag is not defined on this command: the pod name stays empty
    let request = if !ctx.args().is_empty() {
        request_with_application_url(ctx.args(), String::new())
    } else {
        request_without_arg(String::new())
    };
    let request = match request {
        Ok(r) => r,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };

    let pods = match exec_list_pods(&request) {
        Ok(p) => p,
        Err(e) => {
            qovery_log::println_error(e);
            return;
        }
    };

    let data: Vec<Vec<String>> = pods
        .into_iter()
        .map(|mut pod| {
            pod.ports.sort_unstable();
            let ports: Vec<String> = pod.ports.iter().map(|p| p.to_string()).collect();
            vec![pod.name, ports.join(", ")]
        })
        .collect();
    qovery_log::print_table(&["Pod Name", "Ports"], &data);
}

/// `pkg.PodResponse`.
#[derive(Debug, Default, Clone, PartialEq, Eq)]
pub struct PodResponse {
    pub name: String,
    pub ports: Vec<u16>,
}

/// `pkg.ExecListPods(req)`: the pods sent by the gateway in the first message of `/service/pods`.
pub fn exec_list_pods(req: &PortForwardRequest) -> Result<Vec<PodResponse>, String> {
    let url = qovery_cmd_shell::query::websocket_request_url("/service/pods", &req.query());
    let auth = ws::authorization()?;
    let rt = ws::runtime();
    let (is_text, payload) = rt.block_on(async {
        let conn = ws::dial(&url, &auth).await.1?;
        let (_sink, mut source) = conn.split();
        ws::read_message(&mut source)
            .await
            .map_err(|e| e.to_string())
    })?;
    if !is_text {
        // "... pods: " + string(rune(msgType)) + " " + string(payload)
        return Err(format!(
            "received invalid message while listing pods: \u{2} {}",
            String::from_utf8_lossy(&payload)
        ));
    }
    decode_list_pod_response(&payload)
}

/// A JSON value keeping the order of object keys and the text of numbers.
#[derive(Debug, Clone, PartialEq)]
enum Json {
    Null,
    Bool,
    Number(String),
    String(String),
    Array(Vec<Json>),
    Object(Vec<(String, Json)>),
}

impl<'de> serde::Deserialize<'de> for Json {
    fn deserialize<D: serde::Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        struct V;
        impl<'de> serde::de::Visitor<'de> for V {
            type Value = Json;
            fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
                f.write_str("a JSON value")
            }
            fn visit_unit<E>(self) -> Result<Json, E> {
                Ok(Json::Null)
            }
            fn visit_bool<E>(self, _: bool) -> Result<Json, E> {
                Ok(Json::Bool)
            }
            fn visit_i64<E>(self, v: i64) -> Result<Json, E> {
                Ok(Json::Number(v.to_string()))
            }
            fn visit_u64<E>(self, v: u64) -> Result<Json, E> {
                Ok(Json::Number(v.to_string()))
            }
            fn visit_f64<E>(self, v: f64) -> Result<Json, E> {
                Ok(Json::Number(v.to_string()))
            }
            fn visit_str<E>(self, v: &str) -> Result<Json, E> {
                Ok(Json::String(v.to_string()))
            }
            fn visit_string<E>(self, v: String) -> Result<Json, E> {
                Ok(Json::String(v))
            }
            fn visit_seq<A: serde::de::SeqAccess<'de>>(self, mut a: A) -> Result<Json, A::Error> {
                let mut items = Vec::new();
                while let Some(v) = a.next_element()? {
                    items.push(v);
                }
                Ok(Json::Array(items))
            }
            fn visit_map<A: serde::de::MapAccess<'de>>(self, mut a: A) -> Result<Json, A::Error> {
                let mut entries = Vec::new();
                while let Some((k, v)) = a.next_entry::<String, Json>()? {
                    entries.push((k, v));
                }
                Ok(Json::Object(entries))
            }
        }
        d.deserialize_any(V)
    }
}

impl Json {
    fn kind(&self) -> &'static str {
        match self {
            Json::Null => "null",
            Json::Bool => "bool",
            Json::Number(_) => "number",
            Json::String(_) => "string",
            Json::Array(_) => "array",
            Json::Object(_) => "object",
        }
    }
}

/// Go's `json.Unmarshal` (jsonv2 in legacy mode) error: `path` is the location in the input (keys as written,
/// array indexes); "Go struct field" is only written when the location ends with a field name.
struct Decoder {
    first_error: Option<String>,
}

impl Decoder {
    fn error(&mut self, kind: &str, path: &[String], at_field: bool, go_type: &str) {
        if self.first_error.is_none() {
            let prefix = if at_field { "Go struct field " } else { "" };
            self.first_error = Some(format!(
                "json: cannot unmarshal {kind} into {prefix}ListPodResponse.{} of type {go_type}",
                path.join(".")
            ));
        }
    }

    fn pod(&mut self, v: &Json, path: &mut Vec<String>) -> PodResponse {
        let mut pod = PodResponse::default();
        match v {
            Json::Null => {}
            Json::Object(entries) => {
                for (key, value) in entries {
                    path.push(key.clone());
                    if key.eq_ignore_ascii_case("name") {
                        match value {
                            Json::Null => {}
                            Json::String(s) => pod.name = s.clone(),
                            other => self.error(other.kind(), path, true, "string"),
                        }
                    } else if key.eq_ignore_ascii_case("ports") {
                        match value {
                            Json::Null => pod.ports = Vec::new(),
                            Json::Array(items) => {
                                pod.ports = Vec::new();
                                for (i, item) in items.iter().enumerate() {
                                    path.push(i.to_string());
                                    let port = match item {
                                        Json::Null => 0,
                                        Json::Number(n) => n.parse::<u16>().unwrap_or_else(|_| {
                                            self.error(
                                                &format!("number {n}"),
                                                path,
                                                false,
                                                "uint16",
                                            );
                                            0
                                        }),
                                        other => {
                                            self.error(other.kind(), path, false, "uint16");
                                            0
                                        }
                                    };
                                    pod.ports.push(port);
                                    path.pop();
                                }
                            }
                            other => self.error(other.kind(), path, true, "[]uint16"),
                        }
                    }
                    path.pop();
                }
            }
            other => self.error(other.kind(), path, false, "pkg.PodResponse"),
        }
        pod
    }
}

/// `json.Unmarshal(payload, &pkg.ListPodResponse{})`: the pods, or Go's error message.
pub fn decode_list_pod_response(data: &[u8]) -> Result<Vec<PodResponse>, String> {
    qovery_cmd_log::logs::go_json_check_valid(data)?;
    let doc: Json = serde_json::from_slice(data).map_err(|e| e.to_string())?;
    let entries = match doc {
        Json::Null => return Ok(Vec::new()),
        Json::Object(entries) => entries,
        other => {
            return Err(format!(
                "json: cannot unmarshal {} into Go value of type pkg.ListPodResponse",
                other.kind()
            ));
        }
    };
    let mut decoder = Decoder { first_error: None };
    let mut pods = Vec::new();
    for (key, value) in &entries {
        if !key.eq_ignore_ascii_case("pods") {
            continue;
        }
        let mut path = vec![key.clone()];
        match value {
            Json::Null => pods = Vec::new(),
            Json::Array(items) => {
                pods = Vec::new();
                for (i, item) in items.iter().enumerate() {
                    path.push(i.to_string());
                    pods.push(decoder.pod(item, &mut path));
                    path.pop();
                }
            }
            other => decoder.error(other.kind(), &path, true, "[]pkg.PodResponse"),
        }
    }
    match decoder.first_error {
        Some(e) => Err(e),
        None => Ok(pods),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn err(s: &str) -> String {
        decode_list_pod_response(s.as_bytes()).unwrap_err()
    }

    #[test]
    fn decode_pods() {
        let pods = decode_list_pod_response(
            br#"{"PODS": [{"Name": "p", "ports": [443, 80]}, null], "x": 1}"#,
        )
        .unwrap();
        assert_eq!(
            pods,
            vec![
                PodResponse {
                    name: "p".into(),
                    ports: vec![443, 80]
                },
                PodResponse::default()
            ]
        );
        assert_eq!(decode_list_pod_response(b"null").unwrap(), vec![]);
        assert_eq!(
            decode_list_pod_response(br#"{"pods": [{"name": "a"}], "PODS": [{"name": "b"}]}"#)
                .unwrap()[0]
                .name,
            "b"
        );
    }

    #[test]
    fn decode_errors() {
        assert_eq!(
            err(r#"{"pods": [{"ports": [70000]}]}"#),
            "json: cannot unmarshal number 70000 into ListPodResponse.pods.0.ports.0 of type uint16"
        );
        assert_eq!(
            err(r#"{"pods": [{"name": 1}]}"#),
            "json: cannot unmarshal number into Go struct field ListPodResponse.pods.0.name of type string"
        );
        assert_eq!(
            err(r#"{"pods": "x"}"#),
            "json: cannot unmarshal string into Go struct field ListPodResponse.pods of type []pkg.PodResponse"
        );
        assert_eq!(
            err(r#"{"pods": ["x"]}"#),
            "json: cannot unmarshal string into ListPodResponse.pods.0 of type pkg.PodResponse"
        );
        assert_eq!(
            err("[1]"),
            "json: cannot unmarshal array into Go value of type pkg.ListPodResponse"
        );
        assert_eq!(
            err("not json"),
            "invalid character 'o' in literal null (expecting 'u')"
        );
    }
}
