//! Services used by `cluster install`: port of pkg/organization, pkg/cluster/credentials,
//! pkg/cluster/containerregistry, pkg/cluster/selfmanaged/self_managed_cluster_service.go and pkg/filewriter.

use crate::manage::{ClusterService, str_field};
use crate::prompt::PromptUiFactory;
use crate::util::{is_empty_or_blank, path_escape};
use qovery_api::{Client, Method};
use qovery_log::fatih;
use serde_json::{Value, json};

/// `organization.OrganizationDto`.
#[derive(Debug, Clone, PartialEq)]
pub struct OrganizationDto {
    pub id: String,
    pub name: String,
}

/// `organization.OrganizationService`.
pub trait OrganizationService {
    fn ask_user_to_select_organization(&self) -> Result<Option<OrganizationDto>, String>;
}

pub struct OrganizationServiceImpl<'a> {
    pub client: &'a Client,
    pub prompt: &'a dyn PromptUiFactory,
}

impl OrganizationService for OrganizationServiceImpl<'_> {
    fn ask_user_to_select_organization(&self) -> Result<Option<OrganizationDto>, String> {
        let res = self.client.send(Method::GET, "/organization", &[], None);
        let organizations: Vec<Value> = match res {
            Err(e) => {
                return Err(format!(
                    "error when listing organizations: {e} (response status = %!s(<nil>))"
                ));
            }
            Ok(r) if r.status >= 300 => {
                return Err(format!(
                    "error when listing organizations: {} (response status = {})",
                    r.status_line(),
                    r.status_line()
                ));
            }
            Ok(r) => serde_json::from_slice::<Value>(&r.body)
                .ok()
                .and_then(|v| v.get("results").and_then(|x| x.as_array()).cloned())
                .unwrap_or_default(),
        };
        let names: Vec<String> = organizations
            .iter()
            .map(|o| str_field(o, "name").to_string())
            .collect();
        // Go keeps the last id of a duplicated name (map)
        let id_of = |name: &str| {
            organizations
                .iter()
                .rev()
                .find(|o| str_field(o, "name") == name)
                .map(|o| str_field(o, "id").to_string())
                .unwrap_or_default()
        };
        if names.is_empty() {
            return Err("no organization found".to_string());
        }
        if names.len() == 1 {
            return Ok(Some(OrganizationDto {
                id: id_of(&names[0]),
                name: names[0].clone(),
            }));
        }
        println!("Organization:");
        let lower: Vec<String> = names.iter().map(|n| n.to_lowercase()).collect();
        let (_, selected) = self.prompt.run_select_with_size_and_searcher(
            "Organization",
            &names,
            30,
            &|input, index| lower[index].contains(&input.to_lowercase()),
        )?;
        Ok(Some(OrganizationDto {
            id: id_of(&selected),
            name: selected,
        }))
    }
}

/// `getName` / `getId` of a `ClusterCredentials` (oneOf of the credentials types).
pub fn credentials_name(creds: &Value) -> Result<String, String> {
    creds
        .get("name")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string())
        .ok_or_else(|| "unknown credentials type".to_string())
}

pub fn credentials_id(creds: &Value) -> Result<String, String> {
    creds
        .get("id")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string())
        .ok_or_else(|| "unknown credentials type".to_string())
}

const GCP_CREDENTIALS_TYPE_WIF: &str = "Workload Identity Federation";
const GCP_CREDENTIALS_TYPE_SERVICE_ACCOUNT: &str = "Service Account JSON Key";

/// `credentials.ClusterCredentialsService`.
pub trait ClusterCredentialsService {
    fn list_cluster_credentials(
        &self,
        organization_id: &str,
        cloud_provider_type: &str,
    ) -> Result<Vec<Value>, String>;
    fn ask_to_create_credentials(
        &self,
        organization_id: &str,
        cloud_provider_type: &str,
    ) -> Result<Value, String>;
}

pub struct ClusterCredentialsServiceImpl<'a> {
    pub client: &'a Client,
    pub prompt: &'a dyn PromptUiFactory,
}

/// `formatCloudProviderCredentialsApiError`.
pub fn format_cloud_provider_credentials_api_error(
    status: Option<u16>,
    body: &str,
    err: Option<&str>,
) -> Option<String> {
    let has_err = err.is_some();
    if !has_err && status.is_none_or(|s| s < 400) {
        return None;
    }
    if status.is_some() && !body.is_empty() {
        return Some(format!(
            "{}: {}\n{}",
            fatih::red("Error"),
            body,
            err.unwrap_or("%!s(<nil>)")
        ));
    }
    let err_s = err.unwrap_or("<nil>");
    Some(format!("{}: {}", fatih::red("Error"), err_s))
}

impl ClusterCredentialsServiceImpl<'_> {
    fn create(
        &self,
        organization_id: &str,
        provider_path: &str,
        body: Value,
    ) -> Result<Value, String> {
        let path = format!(
            "/organization/{}/{provider_path}/credentials",
            path_escape(organization_id)
        );
        match self.client.send(Method::POST, &path, &[], Some(&body)) {
            Err(e) => {
                Err(
                    format_cloud_provider_credentials_api_error(None, "", Some(&e.to_string()))
                        .unwrap_or_default(),
                )
            }
            Ok(r) => {
                let err = if r.status >= 300 {
                    Some(r.status_line())
                } else {
                    None
                };
                if let Some(e) = format_cloud_provider_credentials_api_error(
                    Some(r.status),
                    &r.text(),
                    err.as_deref(),
                ) {
                    return Err(e);
                }
                Ok(serde_json::from_slice(&r.body).unwrap_or(Value::Null))
            }
        }
    }
}

impl ClusterCredentialsService for ClusterCredentialsServiceImpl<'_> {
    fn list_cluster_credentials(
        &self,
        organization_id: &str,
        cloud_provider_type: &str,
    ) -> Result<Vec<Value>, String> {
        let provider_path = match cloud_provider_type {
            "GCP" => "gcp",
            "AWS" => "aws",
            "SCW" => "scaleway",
            "ON_PREMISE" => "onPremise",
            other => {
                return Err(format!(
                    "cannot list credentias for '{other}' cloud provider type"
                ));
            }
        };
        self.client
            .list(&format!(
                "/organization/{}/{provider_path}/credentials",
                path_escape(organization_id)
            ))
            .map_err(|e| e.to_string())
    }

    fn ask_to_create_credentials(
        &self,
        organization_id: &str,
        cloud_provider_type: &str,
    ) -> Result<Value, String> {
        if cloud_provider_type == "ON_PREMISE" {
            return self.create(organization_id, "onPremise", json!({"name": "on-premise"}));
        }
        let credentials_name = self
            .prompt
            .run_prompt("Give a name to your credentials", "")?;
        if is_empty_or_blank(&credentials_name) {
            return Err("please enter a non-empty name for your credentials".to_string());
        }
        match cloud_provider_type {
            "AWS" => {
                let access_key = self.prompt.run_prompt("Enter your AWS access key", "")?;
                let secret_key = self.prompt.run_prompt("Enter your AWS secret key", "")?;
                if is_empty_or_blank(&access_key) {
                    return Err("please enter a non-empty access key".to_string());
                }
                if is_empty_or_blank(&secret_key) {
                    return Err("please enter a non-empty secret key".to_string());
                }
                self.create(
                    organization_id,
                    "aws",
                    json!({"type": "AWS_STATIC", "name": credentials_name, "access_key_id": access_key, "secret_access_key": secret_key}),
                )
            }
            "SCW" => {
                let access_key = self.prompt.run_prompt("Enter your SCW access key", "")?;
                let secret_key = self.prompt.run_prompt("Enter your SCW secret key", "")?;
                let organization = self
                    .prompt
                    .run_prompt("Enter your SCW organization ID", "")?;
                let project = self.prompt.run_prompt("Enter your SCW project ID", "")?;
                if is_empty_or_blank(&access_key) {
                    return Err("please enter a non-empty access key".to_string());
                }
                if is_empty_or_blank(&secret_key) {
                    return Err("please enter a non-empty secret key".to_string());
                }
                if is_empty_or_blank(&organization) {
                    return Err("please enter a non-empty organization id".to_string());
                }
                if is_empty_or_blank(&project) {
                    return Err("please enter a non-empty project id".to_string());
                }
                self.create(
                    organization_id,
                    "scaleway",
                    json!({
                        "name": credentials_name,
                        "scaleway_access_key": access_key,
                        "scaleway_secret_key": secret_key,
                        "scaleway_project_id": project,
                        "scaleway_organization_id": organization,
                    }),
                )
            }
            "GCP" => {
                let items = vec![
                    GCP_CREDENTIALS_TYPE_WIF.to_string(),
                    GCP_CREDENTIALS_TYPE_SERVICE_ACCOUNT.to_string(),
                ];
                let (_, gcp_type) = self
                    .prompt
                    .run_select("Which GCP credentials type do you want to use?", &items)?;
                let body = match gcp_type.as_str() {
                    GCP_CREDENTIALS_TYPE_WIF => {
                        let email = self
                            .prompt
                            .run_prompt("Enter your GCP service account email", "")?;
                        let provider = self
                            .prompt
                            .run_prompt("Enter your GCP Workload Identity provider resource", "")?;
                        if is_empty_or_blank(&email) {
                            return Err(
                                "please enter a non-empty gcp service account email".to_string()
                            );
                        }
                        if is_empty_or_blank(&provider) {
                            return Err(
                                "please enter a non-empty gcp workload identity provider resource"
                                    .to_string(),
                            );
                        }
                        json!({"name": credentials_name, "service_account_email": email, "workload_identity_provider_resource": provider})
                    }
                    GCP_CREDENTIALS_TYPE_SERVICE_ACCOUNT => {
                        let json_credentials = self
                            .prompt
                            .run_prompt("Enter your GCP JSON credentials (*base64* encoded)", "")?;
                        if is_empty_or_blank(&json_credentials) {
                            return Err("please enter a non-empty gcp json credentials".to_string());
                        }
                        json!({"name": credentials_name, "gcp_credentials": json_credentials})
                    }
                    other => {
                        return Err(format!(
                            "unhandled gcp credentials type during credentials creation: {other}"
                        ));
                    }
                };
                self.create(organization_id, "gcp", body)
            }
            other => Err(format!(
                "unhandled cloud provider type during credentials creation: {other}"
            )),
        }
    }
}

/// `containerregistry.ClusterContainerRegistryService`.
pub trait ClusterContainerRegistryService {
    fn ask_to_edit_cluster_container_registry(
        &self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<(), String>;
}

pub struct ClusterContainerRegistryServiceImpl<'a> {
    pub client: &'a Client,
    pub prompt: &'a dyn PromptUiFactory,
}

struct AskRegistryInfo {
    url: String,
    login: String,
    password: String,
    kind: Value,
}

impl ClusterContainerRegistryServiceImpl<'_> {
    fn ask_github_registry_info(&self) -> Result<AskRegistryInfo, String> {
        let login = self.prompt.run_prompt(
            "Enter your Github username to login to the registry. It should be your Github username or Organisation name",
            "",
        )?;
        let password = self.prompt.run_prompt(
            "Enter your Github personal access token (classic) to login to the registry. It must have write and delete packages permissions",
            "",
        )?;
        Ok(AskRegistryInfo {
            url: "https://ghcr.io".to_string(),
            login,
            password,
            kind: json!("GITHUB_CR"),
        })
    }

    fn ask_generic_registry_info(&self, kind: &Value) -> Result<AskRegistryInfo, String> {
        let url = self.prompt.run_prompt("Url of your registry", "https://")?;
        let login = self
            .prompt
            .run_prompt("Username to use to login to your registry", "")?;
        let password = self
            .prompt
            .run_prompt("Password to use to login to your registry", "")?;
        Ok(AskRegistryInfo {
            url,
            login,
            password,
            kind: kind.clone(),
        })
    }
}

impl ClusterContainerRegistryService for ClusterContainerRegistryServiceImpl<'_> {
    fn ask_to_edit_cluster_container_registry(
        &self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<(), String> {
        let items = vec!["Github".to_string(), "A Generic One".to_string()];
        let (_, choice) = self.prompt.run_select(
            "You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry",
            &items,
        )?;
        let registries: Vec<Value> = self
            .client
            .list(&format!(
                "/organization/{}/containerRegistry",
                path_escape(organization_id)
            ))
            .map_err(|e| e.to_string())?;
        // Only 1 container registry exists for a Self Managed cluster: the one of the cluster
        let Some(registry) = registries.iter().find(|r| {
            r.get("cluster")
                .and_then(|c| c.get("id"))
                .and_then(|i| i.as_str())
                == Some(cluster_id)
        }) else {
            // Go indexes the results with -1 (panic)
            return Err("runtime error: index out of range [-1]".to_string());
        };
        let info = match choice.as_str() {
            "Github" => self.ask_github_registry_info()?,
            "A Generic One" => {
                self.ask_generic_registry_info(registry.get("kind").unwrap_or(&Value::Null))?
            }
            other => return Err(format!("cannot configure container registry: {other}")),
        };
        let mut body = json!({
            "name": str_field(registry, "name"),
            "kind": info.kind,
            "url": info.url,
            "config": {"username": info.login, "password": info.password},
        });
        if let Some(d) = registry.get("description").filter(|d| !d.is_null()) {
            body["description"] = d.clone();
        }
        let path = format!(
            "/organization/{}/containerRegistry/{}",
            path_escape(organization_id),
            path_escape(str_field(registry, "id"))
        );
        match self.client.send(Method::PUT, &path, &[], Some(&body)) {
            Err(e) => Err(e.to_string()),
            Ok(r) if r.status >= 300 => Err(format!("{}: {}", fatih::red("Error"), r.text())),
            Ok(_) => Ok(()),
        }
    }
}

/// `filewriter.FileWriterService`.
pub trait FileWriterService {
    fn write_file(&self, name: &str, data: &[u8], perm: u32) -> Result<(), String>;
}

pub struct FileWriterServiceImpl;

impl FileWriterService for FileWriterServiceImpl {
    fn write_file(&self, name: &str, data: &[u8], perm: u32) -> Result<(), String> {
        use std::io::Write;
        let mut opts = std::fs::OpenOptions::new();
        opts.write(true).create(true).truncate(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            opts.mode(perm);
        }
        // Go ignores the permission bits on Windows
        #[cfg(not(unix))]
        let _ = perm;
        let path = std::path::Path::new(name);
        let mut f = opts
            .open(path)
            .map_err(|e| qovery_core::context::go_path_error("open", path, &e))?;
        f.write_all(data)
            .map_err(|e| qovery_core::context::go_path_error("write", path, &e))
    }
}

/// `selfmanaged.SelfManagedClusterService`.
pub trait SelfManagedClusterService {
    fn create(&self, organization_id: &str, cloud_vendor: &str) -> Result<Value, String>;
    fn configure(&self, cluster: &Value) -> Result<(), String>;
    fn get_installation_helm_values(
        &self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<String, String>;
    fn get_base_helm_values_content(&self, kubernetes_type: &str) -> Result<String, String>;
}

pub struct SelfManagedClusterServiceImpl<'a> {
    pub client: &'a Client,
    pub cluster_service: &'a dyn ClusterService,
    pub credentials_service: &'a dyn ClusterCredentialsService,
    pub container_registry_service: &'a dyn ClusterContainerRegistryService,
    pub prompt: &'a dyn PromptUiFactory,
    pub local_base_helm_values_path: String,
}

/// `mapCloudVendorToCloudProviderType`.
pub fn map_cloud_vendor_to_cloud_provider_type(vendor: &str) -> &'static str {
    match vendor {
        "AWS" => "AWS",
        "GCP" => "GCP",
        "SCW" => "SCW",
        _ => "ON_PREMISE",
    }
}

impl SelfManagedClusterServiceImpl<'_> {
    fn find_cluster_region(&self, cloud_provider_type: &str) -> Result<String, String> {
        if cloud_provider_type == "ON_PREMISE" {
            return Ok("on-premise".to_string());
        }
        let regions = self
            .cluster_service
            .list_cluster_regions(cloud_provider_type)?;
        let items: Vec<String> = regions
            .iter()
            .map(|r| str_field(r, "name").to_string())
            .collect();
        qovery_log::println("Cluster Region:");
        let (ix, _) = self.prompt.run_select_with_size_and_searcher(
            "Select the region where your cluster is installed",
            &items,
            30,
            &|input, index| items[index].contains(input),
        )?;
        Ok(str_field(&regions[ix], "name").to_string())
    }

    fn find_or_create_credentials(
        &self,
        organization_id: &str,
        cloud_provider_type: &str,
    ) -> Result<Value, String> {
        let creds = self
            .credentials_service
            .list_cluster_credentials(organization_id, cloud_provider_type)?;
        let mut ix = usize::MAX;
        if cloud_provider_type == "ON_PREMISE" {
            if !creds.is_empty() {
                ix = 0;
            }
        } else {
            let mut items = Vec::new();
            for c in &creds {
                items.push(credentials_name(c)?);
            }
            items.push("Create new credentials".to_string());
            qovery_log::println("Cluster registry credentials:");
            let (i, _) = self.prompt.run_select_with_size(
                "Which credentials do you want to use for the container registry ? A container registry is necessary to build and mirror the images deployed on your cluster.",
                &items,
                10,
            )?;
            ix = i;
        }
        if ix >= creds.len() {
            return self
                .credentials_service
                .ask_to_create_credentials(organization_id, cloud_provider_type);
        }
        Ok(creds[ix].clone())
    }
}

impl SelfManagedClusterService for SelfManagedClusterServiceImpl<'_> {
    fn create(&self, organization_id: &str, cloud_vendor: &str) -> Result<Value, String> {
        let cloud_provider_type = map_cloud_vendor_to_cloud_provider_type(cloud_vendor);
        let region = self.find_cluster_region(cloud_provider_type)?;
        let credentials = self.find_or_create_credentials(organization_id, cloud_provider_type)?;
        let name = self
            .prompt
            .run_prompt("Give a name to your new cluster", "my-cluster")?;
        let credentials_id = credentials_id(&credentials)?;
        let credentials_name = credentials_name(&credentials)?;
        let body = json!({
            "name": name,
            "region": region,
            "cloud_provider": cloud_vendor,
            "kubernetes": "SELF_MANAGED",
            "cloud_provider_credentials": {
                "cloud_provider": cloud_provider_type,
                "credentials": {"id": credentials_id, "name": credentials_name},
                "region": region,
            },
            "features": [],
        });
        let path = format!("/organization/{}/cluster", path_escape(organization_id));
        match self.client.send(Method::POST, &path, &[], Some(&body)) {
            Err(e) => Err(e.to_string()),
            Ok(r) if r.status >= 300 => Err(format!("{}: {}", fatih::red("Error"), r.text())),
            Ok(r) => Ok(serde_json::from_slice(&r.body).unwrap_or(Value::Null)),
        }
    }

    fn configure(&self, cluster: &Value) -> Result<(), String> {
        if map_cloud_vendor_to_cloud_provider_type(str_field(cluster, "cloud_provider"))
            != "ON_PREMISE"
        {
            return Ok(());
        }
        let org_id = cluster
            .get("organization")
            .map(|o| str_field(o, "id"))
            .unwrap_or_default();
        self.container_registry_service
            .ask_to_edit_cluster_container_registry(org_id, str_field(cluster, "id"))?;
        self.cluster_service.ask_to_edit_storage_class(cluster)?;
        Ok(())
    }

    fn get_installation_helm_values(
        &self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<String, String> {
        let path = format!(
            "/organization/{}/cluster/{}/installationHelmValues",
            path_escape(organization_id),
            path_escape(cluster_id)
        );
        match self
            .client
            .send_accept(Method::GET, &path, &[], None, "application/x-yaml")
        {
            Err(e) => Err(e.to_string()),
            Ok(r) if r.status >= 300 => Err(format!("{}: {}", fatih::red("Error"), r.text())),
            Ok(r) => Ok(r.text()),
        }
    }

    fn get_base_helm_values_content(&self, kubernetes_type: &str) -> Result<String, String> {
        if !self.local_base_helm_values_path.is_empty() {
            let p = std::path::Path::new(&self.local_base_helm_values_path);
            return std::fs::read_to_string(p)
                .map_err(|e| qovery_core::context::go_path_error("open", p, &e));
        }
        let url = match kubernetes_type {
            "AWS" => {
                "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-aws.yaml"
            }
            "GCP" => {
                "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-gcp.yaml"
            }
            "SCW" => {
                "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-scaleway.yaml"
            }
            "ON_PREMISE" => {
                "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-local.yaml"
            }
            _ => "",
        };
        let res = qovery_api::http_get(url).map_err(|e| {
            let reason = std::error::Error::source(&e)
                .map(|s| s.to_string())
                .unwrap_or_else(|| e.to_string());
            format!("Get \"{url}\": {reason}")
        })?;
        if res.status().as_u16() != 200 {
            return Err(format!(
                "bad status while downloading Qovery Helm Values file: {}",
                qovery_api::status_line(res.status().as_u16())
            ));
        }
        res.text().map_err(|e| e.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::prompt::PromptUiFactoryMock;
    use crate::testutil::{MockServer, Route};

    fn org(id: &str, name: &str) -> Value {
        json!({"id": id, "name": name, "created_at": "2024-01-01T00:00:00Z", "plan": "FREE"})
    }

    #[test]
    fn ask_user_to_select_organization() {
        // select the correct one
        let server = MockServer::start(vec![Route::get(
            "/organization",
            json!({"results": [org("o1", "first"), org("o2", "second")]}),
        )]);
        let client = server.client();
        let prompt = PromptUiFactoryMock::new(&[], &[("Organization", "second")]);
        let service = OrganizationServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_user_to_select_organization().unwrap(),
            Some(OrganizationDto {
                id: "o2".into(),
                name: "second".into()
            })
        );

        // fails if the prompt fails
        let prompt = PromptUiFactoryMock::new(&["Organization"], &[]);
        let service = OrganizationServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(service.ask_user_to_select_organization().is_err());

        // the only organization is selected
        let server = MockServer::start(vec![Route::get(
            "/organization",
            json!({"results": [org("o1", "first")]}),
        )]);
        let client = server.client();
        let prompt = PromptUiFactoryMock::new(&["Organization"], &[]);
        let service = OrganizationServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_user_to_select_organization().unwrap(),
            Some(OrganizationDto {
                id: "o1".into(),
                name: "first".into()
            })
        );

        // bad request
        let server = MockServer::start(vec![Route::new("GET", "/organization", 400, json!({}))]);
        let client = server.client();
        let service = OrganizationServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_user_to_select_organization(),
            Err("error when listing organizations: 400 Bad Request (response status = 400 Bad Request)".to_string())
        );

        // no organization
        let server = MockServer::start(vec![Route::get("/organization", json!({"results": []}))]);
        let client = server.client();
        let service = OrganizationServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_user_to_select_organization(),
            Err("no organization found".to_string())
        );
    }

    const NAME: &str = "Give a name to your credentials";

    #[test]
    fn credentials_name_on_create_credentials() {
        let server = MockServer::start(vec![]);
        let client = server.client();
        for (force, value) in [(true, ""), (false, ""), (false, "   ")] {
            let forced: Vec<&str> = if force { vec![NAME] } else { vec![] };
            let prompt = PromptUiFactoryMock::new(&forced, &[(NAME, value)]);
            let service = ClusterCredentialsServiceImpl {
                client: &client,
                prompt: &prompt,
            };
            let res = service.ask_to_create_credentials("org", "AWS");
            assert!(res.is_err());
            if !force {
                assert_eq!(
                    res,
                    Err("please enter a non-empty name for your credentials".to_string())
                );
            }
        }
    }

    #[test]
    fn format_api_error() {
        assert_eq!(
            format_cloud_provider_credentials_api_error(None, "", Some("transport error")),
            Some("Error: transport error".to_string())
        );
        assert_eq!(
            format_cloud_provider_credentials_api_error(Some(201), "{}", None),
            None
        );
        assert_eq!(
            format_cloud_provider_credentials_api_error(
                Some(400),
                "{\"m\":1}",
                Some("400 Bad Request")
            ),
            Some("Error: {\"m\":1}\n400 Bad Request".to_string())
        );
    }

    fn creds_routes(provider: &str) -> Vec<Route> {
        let creds = json!({"id": "cred-1", "name": "my-creds"});
        vec![
            Route::new(
                "POST",
                &format!("/organization/org/{provider}/credentials"),
                201,
                creds.clone(),
            ),
            Route::get(
                &format!("/organization/org/{provider}/credentials"),
                json!({"results": [creds]}),
            ),
        ]
    }

    #[test]
    fn aws_credentials() {
        let server = MockServer::start(creds_routes("aws"));
        let client = server.client();
        let prompt = PromptUiFactoryMock::new(
            &[],
            &[
                (NAME, "creds"),
                ("Enter your AWS access key", "ak"),
                ("Enter your AWS secret key", "sk"),
            ],
        );
        let service = ClusterCredentialsServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            credentials_id(&service.ask_to_create_credentials("org", "AWS").unwrap()).unwrap(),
            "cred-1"
        );
        let post = server
            .requests()
            .into_iter()
            .find(|r| r.method == "POST")
            .unwrap();
        assert_eq!(
            post.body,
            json!({"type": "AWS_STATIC", "name": "creds", "access_key_id": "ak", "secret_access_key": "sk"})
        );

        for (label, err) in [
            (
                "Enter your AWS access key",
                "please enter a non-empty access key",
            ),
            (
                "Enter your AWS secret key",
                "please enter a non-empty secret key",
            ),
        ] {
            let mut answers = vec![
                (NAME, "creds"),
                ("Enter your AWS access key", "ak"),
                ("Enter your AWS secret key", "sk"),
            ];
            answers.retain(|(l, _)| *l != label);
            answers.push((label, " "));
            let prompt = PromptUiFactoryMock::new(&[], &answers);
            let service = ClusterCredentialsServiceImpl {
                client: &client,
                prompt: &prompt,
            };
            assert_eq!(
                service.ask_to_create_credentials("org", "AWS"),
                Err(err.to_string())
            );
        }
        assert_eq!(
            service
                .list_cluster_credentials("org", "AWS")
                .unwrap()
                .len(),
            1
        );
    }

    #[test]
    fn scaleway_credentials() {
        let server = MockServer::start(creds_routes("scaleway"));
        let client = server.client();
        let all = [
            (NAME, "creds"),
            ("Enter your SCW access key", "ak"),
            ("Enter your SCW secret key", "sk"),
            ("Enter your SCW organization ID", "o"),
            ("Enter your SCW project ID", "p"),
        ];
        let prompt = PromptUiFactoryMock::new(&[], &all);
        let service = ClusterCredentialsServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(service.ask_to_create_credentials("org", "SCW").is_ok());
        let post = server
            .requests()
            .into_iter()
            .find(|r| r.method == "POST")
            .unwrap();
        assert_eq!(
            post.body,
            json!({"name": "creds", "scaleway_access_key": "ak", "scaleway_secret_key": "sk", "scaleway_project_id": "p", "scaleway_organization_id": "o"})
        );
        for (i, err) in [
            (1, "please enter a non-empty access key"),
            (2, "please enter a non-empty secret key"),
            (3, "please enter a non-empty organization id"),
            (4, "please enter a non-empty project id"),
        ] {
            let mut answers = all.to_vec();
            answers[i].1 = "";
            let prompt = PromptUiFactoryMock::new(&[], &answers);
            let service = ClusterCredentialsServiceImpl {
                client: &client,
                prompt: &prompt,
            };
            assert_eq!(
                service.ask_to_create_credentials("org", "SCW"),
                Err(err.to_string())
            );
        }
        assert_eq!(
            service
                .list_cluster_credentials("org", "SCW")
                .unwrap()
                .len(),
            1
        );
    }

    #[test]
    fn gcp_credentials() {
        let server = MockServer::start(creds_routes("gcp"));
        let client = server.client();
        let select = "Which GCP credentials type do you want to use?";
        let prompt = PromptUiFactoryMock::new(
            &[],
            &[
                (NAME, "creds"),
                (select, GCP_CREDENTIALS_TYPE_SERVICE_ACCOUNT),
                ("Enter your GCP JSON credentials (*base64* encoded)", "e30="),
            ],
        );
        let service = ClusterCredentialsServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(service.ask_to_create_credentials("org", "GCP").is_ok());
        let post = server
            .requests()
            .into_iter()
            .find(|r| r.method == "POST")
            .unwrap();
        assert_eq!(
            post.body,
            json!({"name": "creds", "gcp_credentials": "e30="})
        );

        let wif = [
            (NAME, "creds"),
            (select, GCP_CREDENTIALS_TYPE_WIF),
            ("Enter your GCP service account email", "sa@x"),
            ("Enter your GCP Workload Identity provider resource", "res"),
        ];
        let prompt = PromptUiFactoryMock::new(&[], &wif);
        let service = ClusterCredentialsServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(service.ask_to_create_credentials("org", "GCP").is_ok());

        let prompt = PromptUiFactoryMock::new(
            &[],
            &[
                (NAME, "creds"),
                (select, GCP_CREDENTIALS_TYPE_SERVICE_ACCOUNT),
            ],
        );
        let service = ClusterCredentialsServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_to_create_credentials("org", "GCP"),
            Err("please enter a non-empty gcp json credentials".to_string())
        );
        for (i, err) in [
            (2, "please enter a non-empty gcp service account email"),
            (
                3,
                "please enter a non-empty gcp workload identity provider resource",
            ),
        ] {
            let mut answers = wif.to_vec();
            answers[i].1 = "";
            let prompt = PromptUiFactoryMock::new(&[], &answers);
            let service = ClusterCredentialsServiceImpl {
                client: &client,
                prompt: &prompt,
            };
            assert_eq!(
                service.ask_to_create_credentials("org", "GCP"),
                Err(err.to_string())
            );
        }
        assert_eq!(
            service
                .list_cluster_credentials("org", "GCP")
                .unwrap()
                .len(),
            1
        );
    }

    #[test]
    fn on_premise_credentials() {
        let server = MockServer::start(creds_routes("onPremise"));
        let client = server.client();
        let prompt = PromptUiFactoryMock::new(&[NAME], &[]);
        let service = ClusterCredentialsServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(
            service
                .ask_to_create_credentials("org", "ON_PREMISE")
                .is_ok()
        );
        let post = server
            .requests()
            .into_iter()
            .find(|r| r.method == "POST")
            .unwrap();
        assert_eq!(post.body, json!({"name": "on-premise"}));
        assert_eq!(
            service
                .list_cluster_credentials("org", "ON_PREMISE")
                .unwrap()
                .len(),
            1
        );
    }

    const REGISTRY_SELECT: &str = "You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to use as registry";
    const GH_LOGIN: &str = "Enter your Github username to login to the registry. It should be your Github username or Organisation name";
    const GH_PASSWORD: &str = "Enter your Github personal access token (classic) to login to the registry. It must have write and delete packages permissions";

    fn registry_routes(edit_status: u16) -> Vec<Route> {
        let registry = json!({"id": "reg-1", "name": "registry", "kind": "GENERIC_CR", "description": "desc", "cluster": {"id": "c-1"}});
        vec![
            Route::get(
                "/organization/org/containerRegistry",
                json!({"results": [{"id": "other", "name": "o", "kind": "ECR"}, registry]}),
            ),
            Route::new(
                "PUT",
                "/organization/org/containerRegistry/reg-1",
                edit_status,
                json!({"id": "reg-1"}),
            ),
        ]
    }

    #[test]
    fn ask_to_edit_generic_container_registry() {
        let server = MockServer::start(registry_routes(200));
        let client = server.client();
        let answers = [
            (REGISTRY_SELECT, "A Generic One"),
            ("Url of your registry", "https://my.registry"),
            ("Username to use to login to your registry", "user"),
            ("Password to use to login to your registry", "pass"),
        ];
        let prompt = PromptUiFactoryMock::new(&[], &answers);
        let service = ClusterContainerRegistryServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_to_edit_cluster_container_registry("org", "c-1"),
            Ok(())
        );
        let put = server
            .requests()
            .into_iter()
            .find(|r| r.method == "PUT")
            .unwrap();
        assert_eq!(
            put.body,
            json!({"name": "registry", "kind": "GENERIC_CR", "description": "desc", "url": "https://my.registry", "config": {"username": "user", "password": "pass"}})
        );

        // prompts failing
        for label in [
            REGISTRY_SELECT,
            "Url of your registry",
            "Username to use to login to your registry",
            "Password to use to login to your registry",
        ] {
            let prompt = PromptUiFactoryMock::new(&[label], &answers);
            let service = ClusterContainerRegistryServiceImpl {
                client: &client,
                prompt: &prompt,
            };
            assert!(
                service
                    .ask_to_edit_cluster_container_registry("org", "c-1")
                    .is_err(),
                "{label}"
            );
        }
        // unhandled registry type
        let prompt = PromptUiFactoryMock::new(&[], &[(REGISTRY_SELECT, "Docker Hub")]);
        let service = ClusterContainerRegistryServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_to_edit_cluster_container_registry("org", "c-1"),
            Err("cannot configure container registry: Docker Hub".to_string())
        );

        // list call fails
        let server = MockServer::start(vec![]);
        let client = server.client();
        let prompt = PromptUiFactoryMock::new(&[], &answers);
        let service = ClusterContainerRegistryServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(
            service
                .ask_to_edit_cluster_container_registry("org", "c-1")
                .is_err()
        );

        // edit call fails
        let server = MockServer::start(registry_routes(400));
        let client = server.client();
        let service = ClusterContainerRegistryServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert!(
            service
                .ask_to_edit_cluster_container_registry("org", "c-1")
                .unwrap_err()
                .contains("Error")
        );
    }

    #[test]
    fn ask_to_edit_github_container_registry() {
        let server = MockServer::start(registry_routes(200));
        let client = server.client();
        let answers = [
            (REGISTRY_SELECT, "Github"),
            (GH_LOGIN, "me"),
            (GH_PASSWORD, "token"),
        ];
        let prompt = PromptUiFactoryMock::new(&[], &answers);
        let service = ClusterContainerRegistryServiceImpl {
            client: &client,
            prompt: &prompt,
        };
        assert_eq!(
            service.ask_to_edit_cluster_container_registry("org", "c-1"),
            Ok(())
        );
        let put = server
            .requests()
            .into_iter()
            .find(|r| r.method == "PUT")
            .unwrap();
        assert_eq!(put.body["kind"], "GITHUB_CR");
        assert_eq!(put.body["url"], "https://ghcr.io");
        for label in [GH_LOGIN, GH_PASSWORD] {
            let prompt = PromptUiFactoryMock::new(&[label], &answers);
            let service = ClusterContainerRegistryServiceImpl {
                client: &client,
                prompt: &prompt,
            };
            assert!(
                service
                    .ask_to_edit_cluster_container_registry("org", "c-1")
                    .is_err(),
                "{label}"
            );
        }
    }
}
