//! `qovery admin load-aws-credentials`, `load-credentials` and `k9s`.
//! Port of `cmd/admin_load_*credentials.go`, `cmd/admin_k9s.go`, `pkg/admin_load_credentials.go`, `pkg/bastion.go`
//! and `pkg.GetKubeconfigByClusterId` (`pkg/cluster.go`).

use crate::common::{
    Method, admin_request, admin_url, authorization, look_path, run_inherited, set_env, unset_env,
    write_in_file,
};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_log::{logrus, println_error, println_info};
use std::collections::BTreeMap;
use std::time::{Duration, Instant};

pub fn load_aws_credentials() -> Command {
    Command::new("load-aws-credentials")
        .short("Load aws credentials from a role ARN")
        .long(
            r#"This command is used to load aws credentials
The role is assumed with the external id "<organization id>:<cluster id>", or "<organization id>:00000000-0000-0000-0000-000000000000" when no cluster id is given. The trust policy must require that external id.

> Examples
----------
* Load AWS credentials from a role ARN arn:aws:iam::123456789012:role/qovery-user-role-xxx of an organization
qovery admin load-aws-credentials --role-arn arn:aws:iam::123456789012:role/qovery-user-role-xxx --organization-id 12345678-1234-1234-1234-123456789012

* Load AWS credentials from a role ARN on behalf of one of the organization clusters
qovery admin load-aws-credentials --role-arn arn:aws:iam::123456789012:role/qovery-user-role-xxx --organization-id 12345678-1234-1234-1234-123456789012 --cluster-id 87654321-4321-4321-4321-210987654321

"#,
        )
        .flag(Flag::string("role-arn", "ARN of the AWS IAM role to assume").short('r'))
        .flag(Flag::string("organization-id", "ID of the organization the role belongs to").short('o').required())
        .flag(Flag::string("cluster-id", "ID of the cluster to assume the role for (optional)").short('c'))
        .run(|ctx| {
            let r = load_aws(&ctx.string("role-arn"), &ctx.string("organization-id"), &ctx.string("cluster-id"));
            qovery_core::check_error(r);
        })
}

pub fn load_credentials() -> Command {
    Command::new("load-credentials")
        .short("Load credentials for a given cluster ID")
        .long(
            "This command is used to load credentials \n> Examples
----------
* Load credentials from a clusterID 12345678-1234-1234-1234-123456789012
qovery admin load-credentials --cluster-id 12345678-1234-1234-1234-123456789012

",
        )
        .flag(Flag::string("cluster-id", "ID of the cluster to load credentials for").short('c'))
        .flag(Flag::bool("no-bastion", "do not connect to the bastion").short('n'))
        .run(|ctx| {
            let r = load_cluster_credentials(&ctx.string("cluster-id"), ctx.bool("no-bastion"));
            qovery_core::check_error(r);
        })
}

pub fn k9s() -> Command {
    Command::new("k9s")
        .short("Launch k9s with a cluster ID")
        .flag(Flag::bool("no-bastion", "do not connect to the bastion").short('n'))
        .flag(
            Flag::bool(
                "read-write",
                "run k9s in read-write mode (default is read-only)",
            )
            .short('w'),
        )
        .run(run_k9s)
}

/// `assumeRoleUrl`.
pub fn assume_role_url(
    admin: &str,
    role_arn: &str,
    organization_id: &str,
    cluster_id: &str,
) -> String {
    let mut q = BTreeMap::new();
    q.insert("role_arn", role_arn.to_string());
    q.insert("organization_id", organization_id.to_string());
    if !cluster_id.is_empty() {
        q.insert("cluster_id", cluster_id.to_string());
    }
    format!(
        "{admin}/aws/credentials/assume-role?{}",
        qovery_api::encode_query(&q.into_iter().collect::<Vec<_>>())
    )
}

/// `pkg.LoadAwsCredentials`.
fn load_aws(role_arn: &str, organization_id: &str, cluster_id: &str) -> Result<(), String> {
    let auth = qovery_core::check_error(authorization());
    let url = assume_role_url(&admin_url(), role_arn, organization_id, cluster_id);
    let res = qovery_core::check_error(admin_request(Method::POST, &url, &auth, None));
    if res.status != 200 {
        qovery_core::exit_with_error(format!(
            "cannot fetch aws credentials (status_code={}) {}",
            res.status,
            res.text()
        ));
    }
    let v: serde_json::Value = qovery_core::check_error(serde_json::from_slice(&res.body));
    // go-jose's json decoder matches the keys case-sensitively
    let s = |k: &str| {
        v.get(k)
            .and_then(|x| x.as_str())
            .unwrap_or_default()
            .to_string()
    };
    set_env("AWS_ACCESS_KEY_ID", &s("access_key_id"));
    set_env("AWS_SECRET_ACCESS_KEY", &s("secret_access_key"));
    set_env("AWS_SESSION_TOKEN", &s("session_token"));
    println_info(
        "AWS credentials loaded successfully in current environment for child process. (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN)",
    );
    start_child_shell()
}

/// `pkg.StartChildShell`.
fn start_child_shell() -> Result<(), String> {
    let shell = std::env::var("SHELL")
        .ok()
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| "/bin/bash".to_string());
    println_info("Launching new shell with credentials...");
    run_inherited(&shell, &[]).map_err(|e| format!("error launching shell: {e}"))
}

/// `pkg.LoadCredentials`.
fn load_cluster_credentials(cluster_id: &str, no_bastion: bool) -> Result<(), String> {
    // `defer SetBastionConnection()()`: the SSH tunnel is closed once the child shell exits, or on error
    let cleanup = (!no_bastion).then(set_bastion_connection);
    let r = load_cluster_credentials_with_tunnel(cluster_id);
    if let Some(cleanup) = cleanup {
        cleanup();
    }
    r
}

fn load_cluster_credentials_with_tunnel(cluster_id: &str) -> Result<(), String> {
    let creds = cluster_credentials(cluster_id);
    if creds.is_empty() {
        return Err(format!("no credentials found for cluster ID {cluster_id}"));
    }
    for (k, v) in &creds {
        set_env(k, v);
        println_info(format!("Set environment variable {k} for child process"));
    }
    let kubeconfig = kubeconfig_by_cluster_id(cluster_id);
    let path = write_in_file(cluster_id, "kubeconfig", kubeconfig.as_bytes());
    set_env("KUBECONFIG", &path);
    if kubeconfig.contains("command: qovery")
        && let Err(e) = look_path("qovery")
    {
        println_info(format!(
            "KUBECONFIG uses qovery as an exec credential command, but qovery was not found in PATH: {e}"
        ));
    }
    start_child_shell()
}

/// `getClusterCredentials`.
fn cluster_credentials(cluster_id: &str) -> Vec<(String, String)> {
    let auth = match authorization() {
        Ok(a) => a,
        Err(e) => {
            println_error(e);
            qovery_core::exit(0)
        }
    };
    let url = format!("{}/cluster/{}/credential", admin_url(), cluster_id);
    let res = admin_request(Method::GET, &url, &auth, Some(b"{}".to_vec()))
        .unwrap_or_else(|e| logrus::fatal(e));
    if res.status != 200 {
        let err = format!(
            "error retrieving cluster credentials: {} {}",
            res.status_line(),
            res.text()
        );
        println_error(&err);
        logrus::fatal(err);
    }
    let payload: serde_json::Map<String, serde_json::Value> =
        serde_json::from_slice(&res.body).unwrap_or_else(|e| logrus::fatal(e.to_string()));
    let payload: BTreeMap<String, String> = payload
        .into_iter()
        .map(|(k, v)| (k, v.as_str().unwrap_or_default().to_string()))
        .collect();
    credentials_from_payload(cluster_id, &payload)
}

fn gcp_project_vars(creds: &mut Vec<(String, String)>, project: &str) {
    for k in [
        "GOOGLE_PROJECT",
        "GOOGLE_CLOUD_PROJECT",
        "CLOUDSDK_CORE_PROJECT",
    ] {
        creds.push((k.into(), project.into()));
    }
}

fn gcp_region_vars(creds: &mut Vec<(String, String)>, region: &str) {
    for k in ["GOOGLE_REGION", "CLOUDSDK_COMPUTE_REGION"] {
        creds.push((k.into(), region.into()));
    }
}

/// `clusterCredentialsFromPayload` (Go iterates a map: the order of the variables is random there).
pub fn credentials_from_payload(
    cluster_id: &str,
    payload: &BTreeMap<String, String>,
) -> Vec<(String, String)> {
    let is_gcp = [
        "json_credentials",
        "gcp_access_token",
        "gcp_project_id",
        "gcp_region",
        "gcp_access_token_expiration",
        "gcp_credentials_type",
    ]
    .iter()
    .any(|k| payload.contains_key(*k));
    let mut creds: Vec<(String, String)> = Vec::new();
    let push = |c: &mut Vec<(String, String)>, k: &str, v: &str| c.push((k.into(), v.into()));
    for (key, value) in payload {
        match key.as_str() {
            "access_key_id" => push(&mut creds, "AWS_ACCESS_KEY_ID", value),
            "secret_access_key" => push(&mut creds, "AWS_SECRET_ACCESS_KEY", value),
            "aws_session_token" => push(&mut creds, "AWS_SESSION_TOKEN", value),
            "region" => {
                if is_gcp {
                    if !payload.contains_key("gcp_region") {
                        gcp_region_vars(&mut creds, value);
                    }
                } else {
                    push(&mut creds, "AWS_DEFAULT_REGION", value);
                }
            }
            "scaleway_access_key" => push(&mut creds, "SCW_ACCESS_KEY", value),
            "scaleway_secret_key" => push(&mut creds, "SCW_SECRET_KEY", value),
            "scaleway_project_id" => push(&mut creds, "SCW_PROJECT_ID", value),
            "scaleway_organization_id" => push(&mut creds, "SCW_ORGANIZATION_ID", value),
            "json_credentials" => {
                let path = write_in_file(cluster_id, "google_creds.json", value.as_bytes());
                push(&mut creds, "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", &path);
                push(&mut creds, "GOOGLE_CREDENTIALS", value);
            }
            "gcp_access_token" => {
                let path = write_in_file(cluster_id, "google_access_token", value.as_bytes());
                push(&mut creds, "GOOGLE_OAUTH_ACCESS_TOKEN", value);
                push(&mut creds, "CLOUDSDK_AUTH_ACCESS_TOKEN_FILE", &path);
            }
            "gcp_project_id" => gcp_project_vars(&mut creds, value),
            "gcp_region" => gcp_region_vars(&mut creds, value),
            "gcp_access_token_expiration" => {
                push(&mut creds, "GOOGLE_OAUTH_ACCESS_TOKEN_EXPIRATION", value)
            }
            "gcp_credentials_type" => push(&mut creds, "GCP_CREDENTIALS_TYPE", value),
            _ => {}
        }
    }
    creds
}

/// `pkg.GetKubeconfigByClusterId(clusterId, false)`.
fn kubeconfig_by_cluster_id(cluster_id: &str) -> String {
    let client = qovery_core::check_error(qovery_core::client::get_client());
    let res = client
        .execute_accept(
            Method::GET,
            &format!(
                "/organization/00000000-0000-0000-000000000000/cluster/{cluster_id}/kubeconfig"
            ),
            &[("with_token_from_cli", "true".to_string())],
            None,
            "application/x-yaml",
        )
        .unwrap_or_else(|e| qovery_core::exit_with_error(e));
    if res.status != 200 {
        println_info(format!(
            "cannot fetch cluster token (status_code={})",
            res.status
        ));
        qovery_core::exit(1);
    }
    res.text()
}

/// `pkg.SetBastionConnection`: an SSH SOCKS tunnel through the bastion, `HTTPS_PROXY` pointing to it.
/// Returns the cleanup function (a no-op when the tunnel could not be set up).
///
/// The tunnel never outlives the CLI: when the CLI exits before the cleanup function is called (`check_error`,
/// `logrus::fatal`, ...), an exit hook kills ssh, silently like Go where ssh dies with it (`Pdeathsig`); a CLI
/// killed by a signal is covered by `Pdeathsig` on Linux.
fn set_bastion_connection() -> Box<dyn FnOnce()> {
    match setup_ssh_connection() {
        Ok(child) => {
            let child = std::sync::Arc::new(std::sync::Mutex::new(Some(child)));
            let take = |c: &std::sync::Mutex<Option<std::process::Child>>| {
                c.lock().unwrap_or_else(|e| e.into_inner()).take()
            };
            let on_exit = child.clone();
            qovery_log::exit::at_exit(move || {
                if let Some(mut c) = take(&on_exit) {
                    let _ = c.kill();
                    let _ = c.wait();
                }
            });
            Box::new(move || {
                if let Some(c) = take(&child) {
                    cleanup_ssh_connection(c)
                }
            })
        }
        Err(e) => {
            logrus::error(format!("Failed to setup SSH connection: {e}"));
            logrus::warn(
                "Connection failure might be due to issues with your SSH configuration. Consider checking and updating your ~/.ssh/known_hosts file to ensure the host is trusted.",
            );
            Box::new(|| {})
        }
    }
}

fn setup_ssh_connection() -> Result<std::process::Child, String> {
    let Ok(bastion) = std::env::var("BASTION_ADDR") else {
        logrus::error("You must set the bastion address (BASTION_ADDR).");
        qovery_core::exit(1)
    };
    let destination = format!("root@{bastion}");
    let args = [
        "-N",
        "-D",
        "127.0.0.1:1080",
        "-p",
        "2222",
        "-4",
        "-o",
        "StrictHostKeychecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "ServerAliveInterval=10",
        "-o",
        "ServerAliveCountMax=3",
        "-o",
        "TCPKeepAlive=yes",
        destination.as_str(),
    ];
    let ssh = look_path("ssh").map_err(|e| format!("error starting SSH command: {e}"))?;
    let mut cmd = std::process::Command::new(ssh);
    cmd.args(args);
    kill_with_parent(&mut cmd);
    let mut child = cmd.spawn().map_err(|e| {
        format!(
            "error starting SSH command: {}",
            crate::common::go_os_error(&e)
        )
    })?;
    if let Err(e) = wait_for_ssh_connection("127.0.0.1:1080", Duration::from_secs(30)) {
        if let Err(k) = child.kill() {
            logrus::error(format!("failed to kill SSH process: {k}"));
        }
        return Err(format!("error waiting for SSH connection: {e}"));
    }
    logrus::info("SSH connection established successfully");
    // socks5 as in Go: the child shell tools (kube clients, Bun...) reject socks5h. The CLI's own requests still
    // resolve the host names at the bastion (qovery_api::http_client_builder)
    set_env("HTTPS_PROXY", SOCKS_PROXY);
    Ok(child)
}

/// `HTTPS_PROXY` of the bastion tunnel.
pub const SOCKS_PROXY: &str = "socks5://127.0.0.1:1080";

/// `SysProcAttr{Pdeathsig: SIGTERM}` (`pkg/bastion_linux.go`): ssh gets SIGTERM when the CLI dies.
#[cfg(target_os = "linux")]
fn kill_with_parent(cmd: &mut std::process::Command) {
    use std::os::unix::process::CommandExt;
    let parent = std::process::id() as libc::pid_t;
    // SAFETY: prctl, getppid and raise are async-signal-safe, and nothing else runs between fork and exec
    unsafe {
        cmd.pre_exec(move || {
            if libc::prctl(libc::PR_SET_PDEATHSIG, libc::SIGTERM) == -1 {
                return Err(std::io::Error::last_os_error());
            }
            // The CLI died between fork and prctl: no signal will come, deliver it now (as the Go runtime does)
            if libc::getppid() != parent {
                libc::raise(libc::SIGTERM);
            }
            Ok(())
        });
    }
}

/// No parent death signal outside of Linux (macOS and Windows have no equivalent, Go's `Pdeathsig` is Linux only
/// too). The tunnel is still closed when the CLI exits by itself (cleanup function and exit hooks), and Ctrl-C or a
/// terminal hangup reach ssh directly (same foreground process group): only a CLI killed from outside
/// (`kill <pid>`) leaves ssh running there, as with the Go CLI.
#[cfg(not(target_os = "linux"))]
fn kill_with_parent(_cmd: &mut std::process::Command) {}

fn wait_for_ssh_connection(address: &str, timeout: Duration) -> Result<(), String> {
    let deadline = Instant::now() + timeout;
    let addr: std::net::SocketAddr = address
        .parse()
        .map_err(|e: std::net::AddrParseError| e.to_string())?;
    loop {
        std::thread::sleep(Duration::from_secs(1));
        if Instant::now() >= deadline {
            return Err("timeout waiting for SSH connection".to_string());
        }
        if std::net::TcpStream::connect_timeout(&addr, Duration::from_secs(1)).is_ok() {
            return Ok(());
        }
    }
}

fn cleanup_ssh_connection(mut child: std::process::Child) {
    logrus::info("Terminating SSH process...");
    // SIGTERM through the kill command (no libc dependency); fall back to SIGKILL
    let term = std::process::Command::new("kill")
        .arg("-TERM")
        .arg(child.id().to_string())
        .status();
    if !matches!(term, Ok(s) if s.success()) {
        logrus::error("Failed to terminate SSH process: os: process already finished");
        if let Err(e) = child.kill() {
            logrus::error(format!("Failed to kill SSH process: {e}"));
        }
    }
    let _ = child.wait();
    logrus::info("SSH process terminated");
    unset_env("HTTPS_PROXY");
    logrus::info("HTTPS_PROXY has been unset");
}

fn run_k9s(ctx: &Ctx) {
    if std::env::var_os("BASTION_ADDR").is_none() {
        logrus::error("You must set the bastion address (BASTION_ADDR).");
        qovery_core::exit(1);
    }
    let Some(cluster_id) = ctx.args().first().cloned() else {
        logrus::error("You must enter a cluster ID.");
        return;
    };
    let cleanup = (!ctx.bool("no-bastion")).then(set_bastion_connection);

    let kubeconfig = kubeconfig_by_cluster_id(&cluster_id);
    let path = write_in_file(&cluster_id, "kubeconfig", kubeconfig.as_bytes());
    set_env("KUBECONFIG", &path);
    logrus::info("Launching k9s.");
    let mut args: Vec<&str> = Vec::new();
    if !ctx.bool("read-write") {
        args.push("--readonly");
        logrus::info(
            "Running k9s in read-only mode. Use --read-write flag to enable write operations.",
        );
    } else {
        logrus::info("Running k9s in read-write mode.");
    }
    if let Err(e) = run_inherited("k9s", &args) {
        logrus::error(format!("Can't launch k9s : {e}"));
    }
    crate::common::delete_folder(path.strip_suffix("kubeconfig").unwrap_or(&path));
    if let Some(cleanup) = cleanup {
        logrus::info("Cleaning up SSH tunnel...");
        cleanup();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn payload(pairs: &[(&str, &str)]) -> BTreeMap<String, String> {
        pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect()
    }

    fn sorted(mut v: Vec<(String, String)>) -> Vec<(String, String)> {
        v.sort();
        v
    }

    fn vars(pairs: &[(&str, &str)]) -> Vec<(String, String)> {
        sorted(
            pairs
                .iter()
                .map(|(k, v)| (k.to_string(), v.to_string()))
                .collect(),
        )
    }

    #[test]
    fn aws_payload() {
        let got = credentials_from_payload(
            "c",
            &payload(&[
                ("access_key_id", "a"),
                ("secret_access_key", "s"),
                ("aws_session_token", "t"),
                ("region", "r"),
            ]),
        );
        assert_eq!(
            sorted(got),
            vars(&[
                ("AWS_ACCESS_KEY_ID", "a"),
                ("AWS_SECRET_ACCESS_KEY", "s"),
                ("AWS_SESSION_TOKEN", "t"),
                ("AWS_DEFAULT_REGION", "r")
            ])
        );
    }

    #[test]
    fn gcp_region_from_region_key() {
        let got = credentials_from_payload(
            "c",
            &payload(&[("gcp_project_id", "p"), ("region", "europe-west1")]),
        );
        assert_eq!(
            sorted(got),
            vars(&[
                ("GOOGLE_PROJECT", "p"),
                ("GOOGLE_CLOUD_PROJECT", "p"),
                ("CLOUDSDK_CORE_PROJECT", "p"),
                ("GOOGLE_REGION", "europe-west1"),
                ("CLOUDSDK_COMPUTE_REGION", "europe-west1"),
            ])
        );
    }

    #[test]
    fn gcp_region_key_wins() {
        let got = credentials_from_payload(
            "c",
            &payload(&[
                ("gcp_region", "g"),
                ("region", "r"),
                ("gcp_credentials_type", "t"),
                ("gcp_access_token_expiration", "e"),
            ]),
        );
        assert_eq!(
            sorted(got),
            vars(&[
                ("GOOGLE_REGION", "g"),
                ("CLOUDSDK_COMPUTE_REGION", "g"),
                ("GCP_CREDENTIALS_TYPE", "t"),
                ("GOOGLE_OAUTH_ACCESS_TOKEN_EXPIRATION", "e")
            ])
        );
    }

    #[test]
    fn scaleway_payload() {
        let got = credentials_from_payload(
            "c",
            &payload(&[
                ("scaleway_access_key", "a"),
                ("scaleway_secret_key", "s"),
                ("scaleway_project_id", "p"),
                ("scaleway_organization_id", "o"),
                ("unknown", "x"),
            ]),
        );
        assert_eq!(
            sorted(got),
            vars(&[
                ("SCW_ACCESS_KEY", "a"),
                ("SCW_SECRET_KEY", "s"),
                ("SCW_PROJECT_ID", "p"),
                ("SCW_ORGANIZATION_ID", "o")
            ])
        );
    }

    #[test]
    fn assume_role_urls() {
        assert_eq!(
            assume_role_url("https://admin", "arn:aws:iam::1:role/x", "org", ""),
            "https://admin/aws/credentials/assume-role?organization_id=org&role_arn=arn%3Aaws%3Aiam%3A%3A1%3Arole%2Fx"
        );
        assert_eq!(
            assume_role_url("https://admin", "r", "org", "c"),
            "https://admin/aws/credentials/assume-role?cluster_id=c&organization_id=org&role_arn=r"
        );
    }
}
