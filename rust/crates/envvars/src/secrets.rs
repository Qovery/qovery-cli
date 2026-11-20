//! Port of `cmd/external_secret_helpers.go`.

use qovery_api::Client;
use qovery_api::models::{Environment, ResultList};
use qovery_log::gofmt::quote;
use serde::Deserialize;

/// `SecretManagerAccess` of a cluster (`secret_manager_accesses` of `GET /organization/{id}/cluster`).
#[derive(Debug, Clone, Default, Deserialize)]
pub struct SecretManagerAccess {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
struct ClusterWithSecretManagers {
    #[serde(default)]
    id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    secret_manager_accesses: Option<Vec<SecretManagerAccess>>,
}

/// Go `getSecretManagerAccessIdByName(client, organizationId, envId, name)` (cmd/external_secret_helpers.go):
/// the id of the secret manager access `name` of the cluster hosting the environment.
///
/// Calls `GET /environment/{envId}` then `GET /organization/{organizationId}/cluster`. Errors:
/// `failed to get environment: <err>`, `failed to list clusters: <err>`,
/// `cluster <id> not found in organization`, `secret manager access "<name>" not found in cluster <cluster name>`.
pub fn get_secret_manager_access_id_by_name(
    client: &Client,
    organization_id: &str,
    env_id: &str,
    name: &str,
) -> Result<String, String> {
    let env: Environment = client
        .get(&format!("/environment/{env_id}"))
        .map_err(|e| format!("failed to get environment: {e}"))?;

    let clusters: Option<ResultList<ClusterWithSecretManagers>> = client
        .get(&format!("/organization/{organization_id}/cluster"))
        .map_err(|e| format!("failed to list clusters: {e}"))?;
    let clusters = clusters.map(|c| c.results).unwrap_or_default();

    let matched_cluster = clusters
        .iter()
        .find(|c| c.id == env.cluster_id)
        .ok_or_else(|| format!("cluster {} not found in organization", env.cluster_id))?;

    matched_cluster
        .secret_manager_accesses
        .iter()
        .flatten()
        .find(|sma| sma.name == name)
        .map(|sma| sma.id.clone())
        .ok_or_else(|| {
            format!(
                "secret manager access {} not found in cluster {}",
                quote(name),
                matched_cluster.name
            )
        })
}
