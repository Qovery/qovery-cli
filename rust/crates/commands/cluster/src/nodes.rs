//! `cluster list-nodes` (cmd/cluster_nodes.go).

use crate::shell::{ReadResult, dial, read_message, ws_url};
use crate::util::encode_query;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, exit_with_error, resources, telemetry};
use serde_json::Value;

pub fn command() -> Command {
    Command::new("list-nodes")
        .short("List cluster nodes")
        .flag(Flag::string("cluster-id", "Cluster ID").short('c'))
        .run(run)
}

fn run(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    // Go reads the package-level `organizationName`, which this command does not define: always the context
    let organization_id = qovery_core::check_error(resources::organization_id(
        &client,
        &ctx.string("organization"),
    ));
    let query = encode_query(&[
        ("organization", organization_id),
        ("cluster", ctx.string("cluster-id")),
    ]);
    let nodes = match exec_list_nodes(&query) {
        Ok(n) => n,
        Err(e) => exit_with_error(e),
    };
    let data: Vec<Vec<String>> = nodes.into_iter().map(|n| vec![n]).collect();
    qovery_log::print_table(&["Name"], &data);
}

/// Go `json.Unmarshal` into `ListNodeResponse{Nodes []NodeResponse{Name string}}` (case-insensitive fields).
fn get_ci<'a>(v: &'a Value, key: &str) -> Option<&'a Value> {
    let obj = v.as_object()?;
    obj.get(key).or_else(|| {
        obj.iter()
            .find(|(k, _)| k.eq_ignore_ascii_case(key))
            .map(|(_, v)| v)
    })
}

fn parse_nodes(payload: &[u8]) -> Result<Vec<String>, String> {
    let v: Value = serde_json::from_slice(payload).map_err(|e| e.to_string())?;
    let nodes = get_ci(&v, "Nodes")
        .and_then(|n| n.as_array())
        .cloned()
        .unwrap_or_default();
    Ok(nodes
        .iter()
        .map(|n| {
            get_ci(n, "Name")
                .and_then(|x| x.as_str())
                .unwrap_or_default()
                .to_string()
        })
        .collect())
}

/// `ExecListNodes`.
fn exec_list_nodes(query: &str) -> Result<Vec<String>, String> {
    let url = ws_url("/cluster/nodes", query);
    let (tt, token) = qovery_core::auth::get_access_token(false)?;
    let auth = qovery_core::auth::authorization_header_value(tt, &token);
    let rt = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|e| e.to_string())?;
    rt.block_on(async {
        let (mut ws, _) = dial(&url, &auth).await.map_err(|e| e.message)?;
        let res = match read_message(&mut ws).await {
            ReadResult::Text(payload) => parse_nodes(&payload),
            ReadResult::Binary(payload) => Err(format!(
                "received invalid message while listing pods: \u{2} {}",
                String::from_utf8_lossy(&payload)
            )),
            ReadResult::Close(e) => Err(e.to_string()),
            ReadResult::Err(e) => Err(e),
        };
        let _ = ws.close(None).await;
        res
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_nodes_case_insensitive() {
        assert_eq!(
            parse_nodes(br#"{"Nodes":[{"Name":"a"},{"name":"b"}]}"#).unwrap(),
            vec!["a", "b"]
        );
        assert_eq!(
            parse_nodes(br#"{"nodes":null}"#).unwrap(),
            Vec::<String>::new()
        );
    }
}
