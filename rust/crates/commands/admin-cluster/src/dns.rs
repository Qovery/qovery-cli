//! `qovery admin cluster update-domain`, `update-dns-provider` and `kubeconfig`.
//! Port of `cmd/admin_cluster_update_domain.go`, `cmd/admin_cluster_update_dns_provider.go`,
//! `cmd/admin_cluster_update_kubeconfig.go` and the `pkg` functions they call.

use crate::common::{Method, http_request};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_log::{println_error, println_info};
use serde_json::{Value, json};
use std::time::Duration;

pub fn update_domain() -> Command {
    Command::new("update-domain")
        .short("Update cluster domain/managed dns for a new one. Cluster and all apps need to be re-deployed after")
        .flag(Flag::string("cluster-id", "The cluster id to target"))
        .flag(Flag::string("domain", "The new domain for the cluster"))
        .run(run_update_domain)
}

pub fn update_dns_provider() -> Command {
    Command::new("update-dns-provider")
        .short("Update cluster DNS provider credentials and domain. Cluster and all apps need to be re-deployed after")
        .long(
            r#"Update the DNS provider configuration for a cluster. This allows you to switch between or reconfigure
DNS providers (Cloudflare, Qovery, or Route53). After updating, the cluster and all applications must be re-deployed.

Examples:
  # Update to Cloudflare
  qovery admin cluster update-dns-provider --cluster-id <id> --domain example.com \
    --provider cloudflare --cloudflare-email user@example.com --cloudflare-token <token>

  # Update to Route53
  qovery admin cluster update-dns-provider --cluster-id <id> --domain example.com \
    --provider route53 --route53-access-key-id <key> --route53-secret-access-key <secret> \
    --route53-region us-east-1 [--route53-hosted-zone-id <zone-id>]

  # Update to Qovery DNS
  qovery admin cluster update-dns-provider --cluster-id <id> --domain example.com \
    --provider qovery --qovery-api-url https://dns.qovery.com"#,
        )
        .flag(Flag::string("cluster-id", "The cluster id to target (required)").required())
        .flag(Flag::string("domain", "The domain for the cluster (required)").required())
        .flag(Flag::string("provider", "DNS provider: cloudflare, qovery, or route53 (required)").required())
        .flag(Flag::string("cloudflare-email", "Cloudflare email"))
        .flag(Flag::string("cloudflare-token", "Cloudflare API token"))
        .flag(Flag::bool("cloudflare-proxied", "Enable Cloudflare proxy"))
        .flag(Flag::string("qovery-api-url", "Qovery DNS API URL"))
        .flag(Flag::string("route53-access-key-id", "AWS Access Key ID for Route53"))
        .flag(Flag::string("route53-secret-access-key", "AWS Secret Access Key for Route53"))
        .flag(Flag::string("route53-region", "AWS Region for Route53"))
        .flag(Flag::string("route53-hosted-zone-id", "AWS Route53 Hosted Zone ID (optional)"))
        .run(run_update_dns_provider)
}

pub fn kubeconfig() -> Command {
    Command::new("kubeconfig")
        .short("Update cluster kubeconfig")
        .flag(Flag::string(
            "organization-id",
            "The cluster's organization ",
        ))
        .flag(Flag::string("cluster-id", "The cluster id to target"))
        .flag(Flag::string(
            "kubeconfig",
            "The cluster kubeconfig string value",
        ))
        .run(run_kubeconfig)
}

/// `utils.PrintlnError(nil); utils.PrintlnInfo(msg); os.Exit(1)`.
fn missing(msg: &str) -> ! {
    println_error("<nil>");
    println_info(msg);
    qovery_core::exit(1)
}

/// Go `path.Join(u.Path, elems...)` on the admin URL, then `u.String()`.
fn admin_url_join(elems: &[&str]) -> Result<url::Url, String> {
    let admin = crate::common::admin_url();
    let mut u = url::Url::parse(&admin).map_err(|e| {
        format!(
            "invalid admin URL: parse {}: {e}",
            qovery_log::gofmt::quote(&admin)
        )
    })?;
    let mut parts: Vec<String> = Vec::new();
    let joined = std::iter::once(u.path().to_string())
        .chain(elems.iter().map(|s| s.to_string()))
        .collect::<Vec<_>>()
        .join("/");
    for seg in joined.split('/') {
        match seg {
            "" | "." => {}
            ".." => {
                parts.pop();
            }
            s => parts.push(s.to_string()),
        }
    }
    u.set_path(&format!("/{}", parts.join("/")));
    Ok(u)
}

fn run_update_domain(ctx: &Ctx) {
    let cluster_id = ctx.string("cluster-id");
    let domain = ctx.string("domain");
    if cluster_id.is_empty() {
        missing("cluster-id is required");
    }
    if domain.is_empty() {
        missing("domain is required");
    }
    if let Err(e) = update_cluster_domain_name(&cluster_id, &domain) {
        qovery_core::exit_with_error(e);
    }
    println_info("domain updated successfully");
}

/// `pkg.UpdateClusterDomainName`.
fn update_cluster_domain_name(cluster_id: &str, domain: &str) -> Result<(), String> {
    let auth =
        crate::common::authorization().map_err(|e| format!("failed to get access token: {e}"))?;
    let mut u = admin_url_join(&["cluster", cluster_id, "domain"])?;
    u.query_pairs_mut().clear().append_pair("name", domain);
    let res = http_request(
        Method::PUT,
        u.as_str(),
        &[
            ("Authorization", &auth),
            ("Content-Type", "application/json"),
        ],
        None,
        Some(Duration::from_secs(30)),
    )
    .map_err(|e| format!("request failed: {e}"))?;
    if res.status != 200 && res.status != 204 {
        return Err(format!(
            "failed to update cluster domain (status={})",
            res.status
        ));
    }
    Ok(())
}

fn run_update_dns_provider(ctx: &Ctx) {
    let s = |k: &str| ctx.string(k);
    let (cluster_id, domain, provider) = (s("cluster-id"), s("domain"), s("provider"));
    if cluster_id.is_empty() {
        missing("cluster-id is required");
    }
    if domain.is_empty() {
        missing("domain is required");
    }
    if provider.is_empty() {
        missing("provider is required (cloudflare, qovery, or route53)");
    }
    match provider.as_str() {
        "cloudflare" => {
            if s("cloudflare-email").is_empty() || s("cloudflare-token").is_empty() {
                missing(
                    "--cloudflare-email and --cloudflare-token are required for Cloudflare provider",
                );
            }
        }
        "qovery" => {
            if s("qovery-api-url").is_empty() {
                missing("--qovery-api-url is required for Qovery provider");
            }
        }
        "route53" => {
            if s("route53-access-key-id").is_empty()
                || s("route53-secret-access-key").is_empty()
                || s("route53-region").is_empty()
            {
                missing(
                    "--route53-access-key-id, --route53-secret-access-key, and --route53-region are required for Route53 provider",
                );
            }
        }
        _ => missing("Invalid provider. Must be cloudflare, qovery, or route53"),
    }
    let body = match provider.as_str() {
        "cloudflare" => json!({
            "domain": domain,
            "cloudflare": {"email": s("cloudflare-email"), "token": s("cloudflare-token"), "is_proxied": ctx.bool("cloudflare-proxied")},
        }),
        "qovery" => json!({"domain": domain, "qovery": {"api_url": s("qovery-api-url")}}),
        _ => {
            let mut creds = json!({
                "aws_access_key_id": s("route53-access-key-id"),
                "aws_secret_access_key": s("route53-secret-access-key"),
                "aws_region": s("route53-region"),
            });
            if !s("route53-hosted-zone-id").is_empty() {
                creds["hosted_zone_id"] = Value::String(s("route53-hosted-zone-id"));
            }
            json!({"domain": domain, "route53": creds})
        }
    };
    if let Err(e) = update_cluster_dns_provider(&cluster_id, &body) {
        qovery_core::exit_with_error(e);
    }
    println_info(
        "DNS provider updated successfully. Please redeploy the cluster and all applications.",
    );
}

/// `pkg.UpdateClusterDnsProvider`.
fn update_cluster_dns_provider(cluster_id: &str, body: &Value) -> Result<(), String> {
    let auth =
        crate::common::authorization().map_err(|e| format!("failed to get access token: {e}"))?;
    let u = admin_url_join(&["cluster", cluster_id, "updateDnsProvider"])?;
    let res = http_request(
        Method::PUT,
        u.as_str(),
        &[
            ("Authorization", &auth),
            ("Content-Type", "application/json"),
        ],
        Some(serde_json::to_vec(body).unwrap_or_default()),
        Some(Duration::from_secs(30)),
    )
    .map_err(|e| format!("request failed: {e}"))?;
    if res.status != 200 && res.status != 204 {
        return Err(format!(
            "failed to update DNS provider (status={}): {}",
            res.status,
            res.text()
        ));
    }
    Ok(())
}

fn run_kubeconfig(ctx: &Ctx) {
    let organization_id = ctx.string("organization-id");
    let cluster_id = ctx.string("cluster-id");
    let client = qovery_core::check_error(qovery_core::client::get_client());
    // cluster.NewClusterService(...).GetClusterByID
    let clusters: Vec<Value> = qovery_core::check_error(
        client.list::<Value>(&format!("/organization/{organization_id}/cluster")),
    );
    let cluster = clusters
        .iter()
        .find(|c| c.get("id").and_then(|i| i.as_str()) == Some(cluster_id.as_str()))
        .unwrap_or_else(|| {
            qovery_core::exit_with_error(format!(
                "Cluster with id {cluster_id} doesn't exists in organization {organization_id}"
            ))
        });
    let kubernetes = cluster.get("kubernetes").and_then(|k| k.as_str());
    if !matches!(kubernetes, Some("SELF_MANAGED" | "PARTIALLY_MANAGED")) {
        qovery_core::exit_with_error(
            "kubeconfig update is supported for SELF MANAGED and PARTIALLY MANAGED clusters only",
        );
    }
    if let Err(e) =
        update_cluster_kubeconfig(&organization_id, &cluster_id, &ctx.string("kubeconfig"))
    {
        println_error(&e);
        qovery_core::exit_with_error(e);
    }
}

/// `pkg.UpdateClusterKubeconfig`: `EditClusterKubeconfig` with a `application/x-yaml` body.
fn update_cluster_kubeconfig(
    organization_id: &str,
    cluster_id: &str,
    kubeconfig: &str,
) -> Result<(), String> {
    if kubeconfig.is_empty() {
        // the Go client refuses an empty body
        return Err("invalid body type application/x-yaml\n".to_string());
    }
    let auth = crate::common::authorization()?;
    let url = format!(
        "{}/organization/{organization_id}/cluster/{cluster_id}/kubeconfig",
        qovery_api::api_base_url()
    );
    let ua = format!("CLI {}", qovery_core::version::VERSION);
    let res = http_request(
        Method::PUT,
        &url,
        &[
            ("Authorization", &auth),
            ("User-Agent", &ua),
            ("Content-Type", "application/x-yaml"),
        ],
        Some(kubeconfig.as_bytes().to_vec()),
        None,
    )?;
    if res.status >= 300 {
        return Err(res.status_line());
    }
    Ok(())
}
