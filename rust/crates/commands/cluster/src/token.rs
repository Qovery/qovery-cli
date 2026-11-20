//! `cluster get-token`, `cluster kubeconfig` (cmd/cluster_get_token.go, cmd/cluster_kubeconfig.go, pkg/cluster.go).

use crate::util::path_escape;
use qovery_api::Method;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, exit, exit_with_error};

pub fn get_token_command() -> Command {
    Command::new("get-token")
        .short("Get token for a cluster ID")
        .flag(Flag::string("cluster-id", "Cluster ID").short('c'))
        .flag(
            Flag::bool(
                "read-only",
                "Get a read-only service account token instead of an admin token",
            )
            .short('r'),
        )
        .run(run_get_token)
}

pub fn kubeconfig_command() -> Command {
    Command::new("kubeconfig")
        .short("Retrieve kubeconfig with a cluster ID")
        .flag(Flag::string("cluster-id", "Cluster ID").short('c'))
        .flag(
            Flag::bool(
                "read-only",
                "Download a read-only kubeconfig backed by a Kubernetes service account with the view ClusterRole",
            )
            .short('r'),
        )
        .run(run_kubeconfig)
}

fn run_get_token(ctx: &Ctx) {
    let cluster_id = ctx.string("cluster-id");
    if cluster_id.is_empty() {
        qovery_log::println_error("cluster ID is required (--cluster-id)");
    }
    let response = get_token_by_cluster_id(&cluster_id, ctx.bool("read-only"));
    qovery_log::println(response);
}

fn run_kubeconfig(ctx: &Ctx) {
    let cluster_id = ctx.string("cluster-id");
    let read_only = ctx.bool("read-only");
    if cluster_id.is_empty() {
        exit_with_error("cluster ID is required (--cluster-id)");
    }
    let filename = download_kubeconfig(&cluster_id, read_only);
    qovery_log::logrus::info("Kubeconfig file created in the current directory.");
    qovery_log::logrus::info(format!("Execute `export KUBECONFIG={filename}` to use it."));
    if read_only {
        qovery_log::logrus::info(
            "This kubeconfig uses read-only access (ServiceAccount with view ClusterRole).",
        );
    }
}

/// Required properties of `GetClusterTokenByClusterId200Response` (the Go client refuses a response lacking them).
const TOKEN_REQUIRED_PROPERTIES: [&str; 4] = ["apiVersion", "kind", "spec", "status"];

/// Port of `pkg.GetTokenByClusterId`: the raw JSON body of the cluster token (ExecCredential), exits on error.
pub fn get_token_by_cluster_id(cluster_id: &str, read_only: bool) -> String {
    let client = client::get_client_or_exit();
    let mut query = Vec::new();
    if read_only {
        query.push(("read_only", "true".to_string()));
    }
    let res = match client.execute(
        Method::GET,
        &format!("/cluster/{}/token", path_escape(cluster_id)),
        &query,
        None,
    ) {
        Ok(r) => r,
        Err(e) => exit_with_error(e),
    };
    let content_type = res
        .headers
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default()
        .to_string();
    if let Err(e) = check_required_properties(&res.body, &content_type, &TOKEN_REQUIRED_PROPERTIES)
    {
        exit_with_error(e);
    }
    if res.status != 200 {
        qovery_log::println_info(format!(
            "cannot fetch cluster token (status_code={})",
            res.status
        ));
        exit(1);
    }
    res.text()
}

/// Generated Go models: `no value given for required property <name>` when a required property is missing.
fn check_required_properties(
    body: &[u8],
    content_type: &str,
    required: &[&str],
) -> Result<(), String> {
    if body.is_empty() {
        return Ok(());
    }
    let ct = content_type.to_lowercase();
    let is_json = regex::Regex::new(r"(application|text)/([^;]+\+)?json")
        .map(|r| r.is_match(&ct))
        .unwrap_or(false);
    if !is_json {
        return Err("undefined response type".to_string());
    }
    let v: serde_json::Value = serde_json::from_slice(body).map_err(|e| e.to_string())?;
    let Some(obj) = v.as_object() else {
        return Ok(());
    };
    for p in required {
        if !obj.contains_key(*p) {
            return Err(format!("no value given for required property {p}"));
        }
    }
    Ok(())
}

/// Port of `pkg.GetKubeconfigByClusterId`: the kubeconfig of a cluster, exits on error.
pub fn get_kubeconfig_by_cluster_id(cluster_id: &str, read_only: bool) -> String {
    let client = client::get_client_or_exit();
    let mut query = vec![("with_token_from_cli", "true".to_string())];
    if read_only {
        query.push(("read_only", "true".to_string()));
    }
    let path = format!(
        "/organization/00000000-0000-0000-000000000000/cluster/{}/kubeconfig",
        path_escape(cluster_id)
    );
    let res = match client.execute(Method::GET, &path, &query, None) {
        Ok(r) => r,
        Err(e) => exit_with_error(e),
    };
    if res.status != 200 {
        qovery_log::println_info(format!(
            "cannot fetch cluster token (status_code={})",
            res.status
        ));
        exit(1);
    }
    res.text()
}

/// Port of `downloadKubeconfig(clusterId, readOnly)` (cmd/cluster_kubeconfig.go): writes
/// `./kubeconfig[-readonly]-<cluster id>.yaml` and returns its absolute path. Exits on error.
pub fn download_kubeconfig(cluster_id: &str, read_only: bool) -> String {
    let kubeconfig = get_kubeconfig_by_cluster_id(cluster_id, read_only);
    let dir = match std::env::current_dir() {
        Ok(d) => d,
        Err(e) => exit_with_error(format!("getwd: {}", go_errno(&e))),
    };
    let suffix = if read_only { "-readonly" } else { "" };
    let path = dir.join(format!("kubeconfig{suffix}-{cluster_id}.yaml"));
    if let Err(e) = qovery_core::gosys::write_file(&path, kubeconfig.as_bytes(), 0o600) {
        exit_with_error(qovery_core::context::go_path_error("open", &path, &e));
    }
    path.to_string_lossy().to_string()
}

use qovery_core::gosys::errno_text as go_errno;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn required_properties() {
        assert!(
            check_required_properties(
                br#"{"apiVersion":"v1","kind":"k","spec":{},"status":{}}"#,
                "application/json",
                &TOKEN_REQUIRED_PROPERTIES
            )
            .is_ok()
        );
        assert_eq!(
            check_required_properties(
                br#"{"kind":"k"}"#,
                "application/json",
                &TOKEN_REQUIRED_PROPERTIES
            ),
            Err("no value given for required property apiVersion".to_string())
        );
    }
}
