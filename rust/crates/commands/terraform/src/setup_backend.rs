//! `qovery terraform setup-backend`. Port of `cmd/terraform_setup_backend.go`.

use k8s_openapi::api::core::v1::Secret;
use kube::api::{Api, ListParams};
use kube::config::{KubeConfigOptions, Kubeconfig, KubeconfigError};
use qovery_api::models::{Environment, Terraform};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{check_error, client, exit, exit_with_error, telemetry};
use qovery_services::path_escape;

pub(crate) fn command() -> Command {
    Command::new("setup-backend")
        .short("Generate a Terraform backend configuration file that can be used to access your tf-state")
        .flag(
            Flag::string("terraform", "Terraform UUID. If not provided, the CLI will use the service context")
                .short('t'),
        )
        .run(run)
}

fn run(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = client::get_client_or_exit();

    // Retrieve terraform service and its environment
    let terraform_id = ctx.string("terraform");
    let terraform: Terraform =
        check_error(client.get(&format!("/terraform/{}", path_escape(&terraform_id))));
    let env_id = terraform
        .environment
        .as_ref()
        .map(|e| e.id.clone())
        .unwrap_or_default();
    let env: Environment =
        check_error(client.get(&format!("/environment/{}", path_escape(&env_id))));
    qovery_log::println(format!(
        "Preparing backend.tf file for terraform `{}` of environment `{}`",
        terraform.name, env.name
    ));

    // Download kubeconfig to connect to the cluster
    let kubeconfig_path = download_kubeconfig(&env.cluster_id, false);

    // Create kubeclient to retrieve the namespace of the tfstate secret
    let label_selector = format!("qovery.com/service-id={},tfstate=true", terraform.id);
    let namespaces = match list_secret_namespaces(&kubeconfig_path, &label_selector) {
        Ok(n) => n,
        Err(e) => exit_with_error(e),
    };
    let Some(namespace) = namespaces.first() else {
        qovery_log::logrus::error(format!(
            "No tfstate secret found for terraform {}. The service must be deployed at least succesfully once",
            terraform.id
        ));
        exit(1);
    };

    // Generate backend.tf file
    let backendtf = format!(
        "
terraform {{
  backend \"kubernetes\" {{
    secret_suffix  = \"{}\"
    namespace      = \"{}\"
    config_path    = \"{}\"
  }}
}}
",
        terraform.id, namespace, kubeconfig_path
    );

    qovery_log::println("Would you like to write `backend.tf` in current directory ?");
    if !qovery_core::select::validate("") {
        return;
    }

    qovery_log::println("Writing `backend.tf` file in current directory");
    if let Err(e) = qovery_core::gosys::write_file(
        std::path::Path::new("backend.tf"),
        backendtf.as_bytes(),
        0o600,
    ) {
        exit_with_error(go_path_error("open", "backend.tf", &e));
    }
    let command_name = match terraform.extra.get("engine").and_then(|v| v.as_str()) {
        Some("TERRAFORM") => "terraform",
        Some("OPEN_TOFU") => "tofu",
        _ => "",
    };
    qovery_log::println(format!(
        "You can now run `{command_name} init` to initialize your project with your tf-state configured on your cluster"
    ));
}

use qovery_cmd_cluster::download_kubeconfig;

/// `clientcmd.BuildConfigFromFlags("", kubeconfigPath)` + `CoreV1().Secrets("").List(labelSelector)`:
/// the namespace of each matching secret. Errors are formatted like client-go's when possible.
fn list_secret_namespaces(
    kubeconfig_path: &str,
    label_selector: &str,
) -> Result<Vec<String>, String> {
    let kubeconfig =
        Kubeconfig::read_from(kubeconfig_path).map_err(|e| kubeconfig_error(e, kubeconfig_path))?;
    let rt = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|e| e.to_string())?;
    rt.block_on(async {
        let config =
            kube::Config::from_custom_kubeconfig(kubeconfig, &KubeConfigOptions::default())
                .await
                .map_err(|e| kubeconfig_error(e, kubeconfig_path))?;
        let request_url = secrets_url(&config.cluster_url.to_string(), label_selector);
        let host_port = host_port(&config.cluster_url);
        let kube_client = kube::Client::try_from(config).map_err(|e| e.to_string())?;
        let secrets: Api<Secret> = Api::all(kube_client);
        let list = secrets
            .list(&ListParams::default().labels(label_selector))
            .await
            .map_err(|e| kube_error(e, &request_url, &host_port))?;
        Ok(list
            .items
            .into_iter()
            .map(|s| s.metadata.namespace.unwrap_or_default())
            .collect())
    })
}

/// client-go's texts for the kubeconfig errors that have an equivalent (the YAML parser details differ).
fn kubeconfig_error(e: KubeconfigError, path: &str) -> String {
    match e {
        KubeconfigError::Parse(inner) => {
            let text = inner.to_string();
            format!("error loading config file \"{path}\": yaml: {}", text.lines().next().unwrap_or_default())
        }
        KubeconfigError::CurrentContextNotSet => {
            "invalid configuration: no configuration has been provided, try setting KUBERNETES_MASTER environment variable"
                .to_string()
        }
        e => e.to_string(),
    }
}

/// The URL client-go requests: `<server>/api/v1/secrets?labelSelector=<url.QueryEscape(selector)>`.
fn secrets_url(server: &str, label_selector: &str) -> String {
    let query: String = url::form_urlencoded::Serializer::new(String::new())
        .append_pair("labelSelector", label_selector)
        .finish();
    format!("{}/api/v1/secrets?{query}", server.trim_end_matches('/'))
}

fn host_port(uri: &http::Uri) -> String {
    let host = uri.host().unwrap_or_default();
    let port = uri
        .port_u16()
        .unwrap_or(if uri.scheme_str() == Some("http") {
            80
        } else {
            443
        });
    if host.contains(':') {
        format!("[{host}]:{port}")
    } else {
        format!("{host}:{port}")
    }
}

/// Go error strings: the `Status.message` of an API error, `Get "<url>": dial tcp <addr>: connect: connection
/// refused` for a refused connection; other errors keep the kube-rs text.
fn kube_error(e: kube::Error, request_url: &str, host_port: &str) -> String {
    if let kube::Error::Api(status) = &e {
        return status.message.clone();
    }
    let mut source: Option<&dyn std::error::Error> = Some(&e);
    while let Some(s) = source {
        if let Some(io) = s.downcast_ref::<std::io::Error>()
            && io.kind() == std::io::ErrorKind::ConnectionRefused
        {
            return format!(
                "Get \"{request_url}\": dial tcp {host_port}: connect: connection refused"
            );
        }
        source = s.source();
    }
    e.to_string()
}

use qovery_core::gosys::path_error as go_path_error;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn secrets_url_like_client_go() {
        assert_eq!(
            secrets_url(
                "https://127.0.0.1:1/",
                "qovery.com/service-id=tf-1,tfstate=true"
            ),
            "https://127.0.0.1:1/api/v1/secrets?labelSelector=qovery.com%2Fservice-id%3Dtf-1%2Ctfstate%3Dtrue"
        );
        assert_eq!(
            host_port(&"https://k8s.example".parse().unwrap()),
            "k8s.example:443"
        );
        assert_eq!(
            host_port(&"http://10.0.0.1:8080".parse().unwrap()),
            "10.0.0.1:8080"
        );
    }
}
