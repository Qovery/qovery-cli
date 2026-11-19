//! Port of `demoQoveryAPI` (`cmd/demo_up.go`, `cmd/demo_destroy.go`): the Qovery API calls of `qovery demo`.

use crate::failure::*;
use crate::local::Bootstrap;
use chrono::{DateTime, Utc};
use qovery_api::{ApiError, Client, Method};
use qovery_log::gofmt::quote;
use serde_json::{Map, Value, json};
use std::collections::BTreeMap;

pub(crate) const DEMO_PLATFORM_TEMPLATE_KEY: &str = "qovery-demo-v0";
const DEMO_OPERATOR_COMPONENT_KEY: &str = "qovery-operator";
const DEMO_CPU_ARCHITECTURES_FIELD: &str = "cpuArchitectures";
const DEMO_REDACTED_VALUE: &str = "<redacted>";

/// `demoCredential`.
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct Credential {
    pub id: String,
    pub name: String,
}

/// `demoOperatorStatus`.
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct OperatorStatus {
    pub connected: bool,
    pub last_heartbeat: Option<DateTime<Utc>>,
}

/// `demoAPI`.
pub(crate) trait DemoApi {
    fn ensure_on_premise_credentials(
        &mut self,
        organization_id: &str,
    ) -> Result<Credential, DemoError>;
    fn find_cluster(
        &mut self,
        organization_id: &str,
        name: &str,
    ) -> Result<(String, bool), DemoError>;
    fn create_cluster(
        &mut self,
        organization_id: &str,
        name: &str,
        credential: &Credential,
    ) -> Result<String, DemoError>;
    fn configure_operator(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
        cpu_architecture: &str,
    ) -> Result<(), DemoError>;
    fn get_operator_bootstrap(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<Bootstrap, DemoError>;
    fn attach_operator(&mut self, organization_id: &str, cluster_id: &str)
    -> Result<(), DemoError>;
    fn get_operator_status(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<OperatorStatus, DemoError>;
    fn deploy_cluster(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<String, DemoError>;
    fn get_cluster_status(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<String, DemoError>;
}

/// `demoDestroyAPI`.
pub(crate) trait DemoDestroyApi {
    fn find_cluster(
        &mut self,
        organization_id: &str,
        name: &str,
    ) -> Result<(String, bool), DemoError>;
    fn delete_cluster_config(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<(), DemoError>;
}

/// `demoQoveryAPI`.
pub(crate) struct QoveryApi {
    pub client: Client,
    /// Prefix of the request paths: empty to use the client's base URL (tests point it to a local server).
    pub base: String,
}

fn api_error(e: ApiError) -> DemoError {
    DemoError {
        message: e.message,
        api_status: e.status,
        api_body: e.body,
        ..Default::default()
    }
}

fn s(v: &Value, key: &str) -> String {
    v.get(key)
        .and_then(|x| x.as_str())
        .unwrap_or_default()
        .to_string()
}

impl QoveryApi {
    pub fn new(client: Client) -> Self {
        QoveryApi {
            client,
            base: String::new(),
        }
    }

    /// `callDemoAPIWithoutBody`: refreshes the access token once on a 401: an installation outlives its
    /// 15-minute lifetime. It never falls back to a browser login, which would block the installation.
    fn call(
        &mut self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
    ) -> Result<Vec<u8>, DemoError> {
        let url = format!("{}{}", self.base, path);
        let mut result = self.client.execute(method.clone(), &url, query, body);
        if let Err(e) = &result
            && e.status == Some(401)
        {
            match qovery_core::auth::get_access_token(false) {
                Err(token_err) => {
                    return Err(DemoError {
                        message: format!(
                            "the Qovery session expired and cannot be refreshed; run `qovery auth` and retry: {token_err}"
                        ),
                        api_status: Some(401),
                        ..Default::default()
                    });
                }
                Ok((token_type, token)) => {
                    self.client = qovery_core::client::client_for(token_type, &token);
                    result = self.client.execute(method, &url, query, body);
                }
            }
        }
        result.map(|r| r.body).map_err(api_error)
    }

    /// `callDemoAPI`: like [`QoveryApi::call`], with an error on an empty successful response.
    fn call_json(
        &mut self,
        method: Method,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
    ) -> Result<Value, DemoError> {
        let raw = self.call(method, path, query, body)?;
        if raw.iter().all(|b| b.is_ascii_whitespace()) {
            return Err(DemoError::new("qovery API returned an empty response"));
        }
        match serde_json::from_slice::<Value>(&raw) {
            Ok(Value::Null) => Err(DemoError::new("qovery API returned an empty response")),
            Ok(v) => Ok(v),
            Err(e) => Err(DemoError {
                message: e.to_string(),
                api_status: Some(200),
                ..Default::default()
            }),
        }
    }

    fn demo_platform_configuration(
        &mut self,
        organization_id: &str,
    ) -> Result<PlatformConfiguration, DemoError> {
        let catalog = self
            .call_json(
                Method::GET,
                &format!("/organization/{organization_id}/platformTemplate"),
                &[
                    ("clusterMode", "CUSTOMER_MANAGED".to_string()),
                    ("cloudProvider", "UNKNOWN".to_string()),
                ],
                None,
            )
            .map_err(|e| e.prefixed("cannot list the platform templates"))?;
        new_demo_platform_configuration(Some(&catalog))
    }

    fn find(&mut self, organization_id: &str, name: &str) -> Result<(String, bool), DemoError> {
        let list = self.call_json(
            Method::GET,
            &format!("/organization/{organization_id}/cluster"),
            &[],
            None,
        )?;
        let clusters = list
            .get("results")
            .and_then(|r| r.as_array())
            .cloned()
            .unwrap_or_default();
        find_demo_cluster(&clusters, name)
    }
}

/// `demoCredentialFromQovery(value)`.
fn demo_credential_from_qovery(value: &Value) -> Result<Credential, DemoError> {
    let object_type = s(value, "object_type");
    let generic = object_type == "OTHER" || object_type == "GenericClusterCredentials";
    let id = s(value, "id");
    if !generic || id.is_empty() {
        return Err(DemoError::new(
            "qovery returned invalid On-Premise credentials",
        ));
    }
    Ok(Credential {
        id,
        name: s(value, "name"),
    })
}

/// `findDemoCluster(clusters, name)`.
pub(crate) fn find_demo_cluster(
    clusters: &[Value],
    name: &str,
) -> Result<(String, bool), DemoError> {
    let Some(cluster) = qovery_core::find::find_by_name(clusters, name) else {
        return Ok((String::new(), false));
    };
    if !cluster
        .get("is_demo")
        .and_then(|v| v.as_bool())
        .unwrap_or(false)
    {
        let mut err = DemoError::new(format!(
            "cluster {} in this organization {ERR_NOT_A_DEMO_CLUSTER}",
            quote(name)
        ));
        err.not_a_demo = true;
        return Err(err);
    }
    Ok((s(cluster, "id"), true))
}

/// `newDemoClusterRequest(name, credential)` serialized like the Go client.
pub(crate) fn new_demo_cluster_request(name: &str, credential: &Credential) -> Value {
    json!({
        "name": name,
        "region": "on-premise",
        "cloud_provider": "ON_PREMISE",
        "kubernetes": "SELF_MANAGED",
        "production": false,
        "cloud_provider_credentials": {
            "cloud_provider": "ON_PREMISE",
            "credentials": {"id": credential.id, "name": credential.name},
            "region": "unknown",
        },
        "features": [],
        "is_demo": true,
    })
}

// --- platform configuration ------------------------------------------------------------------------------

/// `qovery.PlatformSelection`.
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct PlatformSelection {
    pub template_key: String,
    pub template_version: String,
    pub layer_selections: Option<BTreeMap<String, bool>>,
    pub managed_config: Option<BTreeMap<String, BTreeMap<String, Value>>>,
}

/// `qovery.ClusterPlatformConfigurationResponse` / `Request` (the parts the demo uses).
#[derive(Debug, Clone, Default, PartialEq)]
pub(crate) struct PlatformConfiguration {
    pub platform: PlatformSelection,
    pub cluster_inputs: Option<ClusterInputs>,
}

impl PlatformConfiguration {
    pub fn new(template_key: &str, template_version: &str) -> Self {
        PlatformConfiguration {
            platform: PlatformSelection {
                template_key: template_key.to_string(),
                template_version: template_version.to_string(),
                ..Default::default()
            },
            cluster_inputs: Some(BTreeMap::new()),
        }
    }

    /// Decodes the JSON of a `ClusterPlatformConfigurationResponse`; like Go's typed decoding, a
    /// `clusterInputs` value that is not a string is an error.
    pub fn from_json(v: &Value) -> Result<Self, DemoError> {
        let platform = v.get("platform").cloned().unwrap_or(Value::Null);
        let layer_selections = platform
            .get("layerSelections")
            .and_then(|l| l.as_object())
            .map(|m| {
                m.iter()
                    .map(|(k, v)| (k.clone(), v.as_bool().unwrap_or(false)))
                    .collect()
            });
        let managed_config = platform
            .get("managedConfig")
            .and_then(|l| l.as_object())
            .map(|m| {
                m.iter()
                    .map(|(k, v)| {
                        let inner = v
                            .as_object()
                            .map(|o| o.iter().map(|(k, v)| (k.clone(), v.clone())).collect())
                            .unwrap_or_default();
                        (k.clone(), inner)
                    })
                    .collect()
            });
        let cluster_inputs = decode_cluster_inputs(v.get("clusterInputs"))?;
        Ok(PlatformConfiguration {
            platform: PlatformSelection {
                template_key: s(&platform, "templateKey"),
                template_version: s(&platform, "templateVersion"),
                layer_selections,
                managed_config,
            },
            cluster_inputs,
        })
    }

    /// JSON of a `ClusterPlatformConfigurationRequest`.
    pub fn to_request_json(&self) -> Value {
        let mut platform = Map::new();
        platform.insert("templateKey".into(), json!(self.platform.template_key));
        platform.insert(
            "templateVersion".into(),
            json!(self.platform.template_version),
        );
        if let Some(l) = &self.platform.layer_selections {
            platform.insert("layerSelections".into(), json!(l));
        }
        if let Some(m) = &self.platform.managed_config {
            let m: Map<String, Value> = m
                .iter()
                .map(|(k, v)| {
                    (
                        k.clone(),
                        Value::Object(
                            v.iter()
                                .map(|(k, v)| (k.clone(), go_interface_roundtrip(v)))
                                .collect(),
                        ),
                    )
                })
                .collect();
            platform.insert("managedConfig".into(), Value::Object(m));
        }
        json!({"platform": platform, "clusterInputs": self.cluster_inputs.clone().unwrap_or_default()})
    }
}

/// A JSON value decoded into a Go `interface{}` and encoded again: numbers go through `float64`.
fn go_interface_roundtrip(v: &Value) -> Value {
    match v {
        Value::Number(n) => {
            let f = n.as_f64().unwrap_or(0.0);
            if f.fract() == 0.0 && f.abs() < 1e21 {
                if f.abs() <= i64::MAX as f64 {
                    return json!(f as i64);
                }
                return serde_json::Number::from_f64(f)
                    .map(Value::Number)
                    .unwrap_or(Value::Null);
            }
            serde_json::Number::from_f64(f)
                .map(Value::Number)
                .unwrap_or(Value::Null)
        }
        Value::Array(a) => Value::Array(a.iter().map(go_interface_roundtrip).collect()),
        Value::Object(o) => Value::Object(
            o.iter()
                .map(|(k, v)| (k.clone(), go_interface_roundtrip(v)))
                .collect(),
        ),
        other => other.clone(),
    }
}

/// `newDemoPlatformConfiguration(catalog)`.
pub(crate) fn new_demo_platform_configuration(
    catalog: Option<&Value>,
) -> Result<PlatformConfiguration, DemoError> {
    let no_template = || {
        DemoError::new(format!(
            "qovery returned no {} platform template for the local demo",
            quote(DEMO_PLATFORM_TEMPLATE_KEY)
        ))
    };
    let Some(catalog) = catalog else {
        return Err(no_template());
    };
    let results = catalog
        .get("results")
        .and_then(|r| r.as_array())
        .cloned()
        .unwrap_or_default();
    let Some(template) = results
        .iter()
        .find(|c| s(c, "key") == DEMO_PLATFORM_TEMPLATE_KEY && !s(c, "version").trim().is_empty())
    else {
        return Err(no_template());
    };
    let mut layer_selections = BTreeMap::new();
    for layer in template
        .get("layers")
        .and_then(|l| l.as_array())
        .cloned()
        .unwrap_or_default()
    {
        if !layer
            .get("mandatory")
            .and_then(|v| v.as_bool())
            .unwrap_or(false)
        {
            layer_selections.insert(
                s(&layer, "key"),
                layer
                    .get("enabledByDefault")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false),
            );
        }
    }
    let mut configuration =
        PlatformConfiguration::new(&s(template, "key"), &s(template, "version"));
    configuration.platform.layer_selections = Some(layer_selections);
    configuration.platform.managed_config = Some(BTreeMap::new());
    Ok(configuration)
}

/// `selectDemoPlatformConfiguration(existing, demo)`: the selected configuration and whether it is the
/// existing one.
pub(crate) fn select_demo_platform_configuration(
    existing: Option<&PlatformConfiguration>,
    demo: &PlatformConfiguration,
) -> (PlatformConfiguration, bool) {
    if let Some(existing) = existing
        && existing.platform.template_key == demo.platform.template_key
        && existing.platform.template_version == demo.platform.template_version
    {
        return (existing.clone(), true);
    }
    (demo.clone(), false)
}

/// Cluster inputs keyed by component key, then by input key.
pub(crate) type ClusterInputs = BTreeMap<String, BTreeMap<String, String>>;

/// `ClusterPlatformConfigurationResponse.ClusterInputs` (`map[string]map[string]string`) decoded with Go's
/// `encoding/json`: `null` leaves a zero value, any other JSON type mismatch fails with an `UnmarshalTypeError`.
fn decode_cluster_inputs(value: Option<&Value>) -> Result<Option<ClusterInputs>, DemoError> {
    const FIELD: &str = "_ClusterPlatformConfigurationResponse.clusterInputs";
    let components = match value {
        None | Some(Value::Null) => return Ok(None),
        Some(Value::Object(m)) => m,
        Some(other) => {
            return Err(unmarshal_type_error(
                other,
                FIELD,
                "map[string]map[string]string",
            ));
        }
    };
    let mut inputs = BTreeMap::new();
    for (component, values) in components {
        let values = match values {
            Value::Null => {
                inputs.insert(component.clone(), BTreeMap::new());
                continue;
            }
            Value::Object(m) => m,
            other => {
                return Err(unmarshal_type_error(
                    other,
                    &format!("{FIELD}.{component}"),
                    "map[string]string",
                ));
            }
        };
        let mut decoded = BTreeMap::new();
        for (key, value) in values {
            let text = match value {
                Value::Null => String::new(),
                Value::String(text) => text.clone(),
                other => {
                    return Err(unmarshal_type_error(
                        other,
                        &format!("{FIELD}.{component}.{key}"),
                        "string",
                    ));
                }
            };
            decoded.insert(key.clone(), text);
        }
        inputs.insert(component.clone(), decoded);
    }
    Ok(Some(inputs))
}

/// `json.UnmarshalTypeError.Error()` of a struct field.
fn unmarshal_type_error(value: &Value, field: &str, go_type: &str) -> DemoError {
    let kind = match value {
        Value::Bool(_) => "bool",
        Value::Number(_) => "number",
        Value::String(_) => "string",
        Value::Array(_) => "array",
        Value::Object(_) => "object",
        Value::Null => "null",
    };
    DemoError {
        message: format!(
            "json: cannot unmarshal {kind} into Go struct field {field} of type {go_type}"
        ),
        api_status: Some(200),
        ..Default::default()
    }
}

/// `normalizeDemoCPUArchitecture(cpuArchitecture)`.
pub(crate) fn normalize_demo_cpu_architecture(cpu_architecture: &str) -> Result<String, DemoError> {
    let architecture = cpu_architecture.trim().to_uppercase();
    if architecture != "AMD64" && architecture != "ARM64" {
        return Err(DemoError::new(format!(
            "unsupported local CPU architecture {}",
            quote(cpu_architecture)
        )));
    }
    Ok(architecture)
}

/// `demoOperatorIsAlreadyConfigured(existing, configuration, architecture)`.
fn demo_operator_is_already_configured(
    existing: Option<&PlatformConfiguration>,
    is_existing: bool,
    architecture: &str,
) -> bool {
    let Some(existing) = existing else {
        return false;
    };
    if !is_existing {
        return false;
    }
    existing
        .platform
        .managed_config
        .as_ref()
        .and_then(|m| m.get(DEMO_OPERATOR_COMPONENT_KEY))
        .and_then(|c| c.get(DEMO_CPU_ARCHITECTURES_FIELD))
        .and_then(|v| v.as_str())
        == Some(architecture)
}

/// `ensureDemoRequestHasNoRedactedValues(request)`.
pub(crate) fn ensure_demo_request_has_no_redacted_values(
    request: &PlatformConfiguration,
) -> Result<(), DemoError> {
    let mut fields = Vec::new();
    for (component, values) in request.platform.managed_config.iter().flatten() {
        for (key, value) in values {
            if value.as_str() == Some(DEMO_REDACTED_VALUE) {
                fields.push(format!("{component}.{key}"));
            }
        }
    }
    if fields.is_empty() {
        return Ok(());
    }
    fields.sort();
    Err(DemoError::new(format!(
        "qovery returned redacted values for {}; qovery demo sends the whole configuration back and would store the redaction marker instead, so nothing was saved. Retry later; if it persists, these fields are sensitive and qovery demo cannot keep them: clear them, or recreate the cluster with `qovery demo destroy --delete-qovery-config`",
        fields.join(", ")
    )))
}

/// `demoPlatformConfigurationSaveError(response, err)`.
fn demo_platform_configuration_save_error(err: DemoError) -> DemoError {
    if err.api_status == Some(409) {
        let message = err.api_body.trim().to_string();
        if !message.is_empty() {
            let text = format!(
                "cannot save the cluster platform configuration: {}: {message}",
                err.message
            );
            return err.wrap(text);
        }
    }
    err.prefixed("cannot save the cluster platform configuration")
}

/// `newDemoOperatorConfigurationRequest(configuration, architecture)`.
pub(crate) fn new_demo_operator_configuration_request(
    configuration: Option<&PlatformConfiguration>,
    architecture: &str,
) -> Result<PlatformConfiguration, DemoError> {
    let Some(configuration) = configuration.filter(|c| {
        !c.platform.template_key.trim().is_empty() && !c.platform.template_version.trim().is_empty()
    }) else {
        return Err(DemoError::new(
            "qovery returned an invalid platform configuration",
        ));
    };
    let mut managed_config = configuration
        .platform
        .managed_config
        .clone()
        .unwrap_or_default();
    managed_config
        .entry(DEMO_OPERATOR_COMPONENT_KEY.to_string())
        .or_default()
        .insert(
            DEMO_CPU_ARCHITECTURES_FIELD.to_string(),
            json!(architecture),
        );
    let mut request = PlatformConfiguration::new(
        &configuration.platform.template_key,
        &configuration.platform.template_version,
    );
    request.platform.layer_selections = Some(
        configuration
            .platform
            .layer_selections
            .clone()
            .unwrap_or_default(),
    );
    request.platform.managed_config = Some(managed_config);
    request.cluster_inputs = Some(configuration.cluster_inputs.clone().unwrap_or_default());
    Ok(request)
}

impl DemoApi for QoveryApi {
    fn ensure_on_premise_credentials(
        &mut self,
        organization_id: &str,
    ) -> Result<Credential, DemoError> {
        let path = format!("/organization/{organization_id}/onPremise/credentials");
        let list = self.call_json(Method::GET, &path, &[], None)?;
        if let Some(first) = list
            .get("results")
            .and_then(|r| r.as_array())
            .and_then(|r| r.first())
        {
            return demo_credential_from_qovery(first);
        }
        let created = self.call_json(
            Method::POST,
            &path,
            &[],
            Some(&json!({"name": "on-premise"})),
        )?;
        demo_credential_from_qovery(&created)
    }

    fn find_cluster(
        &mut self,
        organization_id: &str,
        name: &str,
    ) -> Result<(String, bool), DemoError> {
        self.find(organization_id, name)
    }

    fn create_cluster(
        &mut self,
        organization_id: &str,
        name: &str,
        credential: &Credential,
    ) -> Result<String, DemoError> {
        let cluster = self.call_json(
            Method::POST,
            &format!("/organization/{organization_id}/cluster"),
            &[],
            Some(&new_demo_cluster_request(name, credential)),
        )?;
        Ok(s(&cluster, "id"))
    }

    fn configure_operator(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
        cpu_architecture: &str,
    ) -> Result<(), DemoError> {
        let architecture = normalize_demo_cpu_architecture(cpu_architecture)?;
        let demo_configuration = self.demo_platform_configuration(organization_id)?;
        let path = format!("/v1/cluster/{cluster_id}/platformConfiguration");
        let existing = match self.call_json(Method::GET, &path, &[], None) {
            Ok(v) => Some(
                PlatformConfiguration::from_json(&v)
                    .map_err(|e| e.prefixed("cannot read the cluster platform configuration"))?,
            ),
            Err(e) if e.api_status == Some(404) => None,
            Err(e) => return Err(e.prefixed("cannot read the cluster platform configuration")),
        };
        let (configuration, is_existing) =
            select_demo_platform_configuration(existing.as_ref(), &demo_configuration);
        if demo_operator_is_already_configured(existing.as_ref(), is_existing, &architecture) {
            return Ok(());
        }
        let request = new_demo_operator_configuration_request(Some(&configuration), &architecture)?;
        ensure_demo_request_has_no_redacted_values(&request)?;
        self.call_json(Method::PUT, &path, &[], Some(&request.to_request_json()))
            .map_err(demo_platform_configuration_save_error)?;
        Ok(())
    }

    fn get_operator_bootstrap(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<Bootstrap, DemoError> {
        let b = self.call_json(
            Method::GET,
            &format!("/organization/{organization_id}/cluster/{cluster_id}/operator/bootstrap"),
            &[],
            None,
        )?;
        Ok(Bootstrap {
            release_name: s(&b, "release_name"),
            chart_reference: s(&b, "chart_reference"),
            chart_version: s(&b, "chart_version"),
            namespace: s(&b, "namespace"),
            values_yaml: s(&b, "values_yaml"),
        })
    }

    fn attach_operator(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<(), DemoError> {
        self.call(
            Method::POST,
            &format!("/organization/{organization_id}/cluster/{cluster_id}/operator/attach"),
            &[],
            None,
        )?;
        Ok(())
    }

    fn get_operator_status(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<OperatorStatus, DemoError> {
        let status = self.call_json(
            Method::GET,
            &format!("/organization/{organization_id}/cluster/{cluster_id}/operator/status"),
            &[],
            None,
        )?;
        let last_heartbeat = match status.get("last_heartbeat").and_then(|v| v.as_str()) {
            None => None,
            Some(t) => Some(
                DateTime::parse_from_rfc3339(t)
                    .map_err(|_| {
                        DemoError::new(format!(
                            "parsing time {} as \"2006-01-02T15:04:05Z07:00\"",
                            quote(t)
                        ))
                    })?
                    .with_timezone(&Utc),
            ),
        };
        Ok(OperatorStatus {
            connected: status
                .get("operator_connected")
                .and_then(|v| v.as_bool())
                .unwrap_or(false),
            last_heartbeat,
        })
    }

    fn deploy_cluster(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<String, DemoError> {
        // q-core answers QUEUED here, never the previous terminal status (ClusterApiController.deployCluster).
        let status = self.call_json(
            Method::POST,
            &format!("/organization/{organization_id}/cluster/{cluster_id}/deploy"),
            &[],
            None,
        )?;
        Ok(s(&status, "status"))
    }

    fn get_cluster_status(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<String, DemoError> {
        let status = self.call_json(
            Method::GET,
            &format!("/organization/{organization_id}/cluster/{cluster_id}/status"),
            &[],
            None,
        )?;
        Ok(s(&status, "status"))
    }
}

impl DemoDestroyApi for QoveryApi {
    fn find_cluster(
        &mut self,
        organization_id: &str,
        name: &str,
    ) -> Result<(String, bool), DemoError> {
        self.find(organization_id, name)
    }

    fn delete_cluster_config(
        &mut self,
        organization_id: &str,
        cluster_id: &str,
    ) -> Result<(), DemoError> {
        self.call(
            Method::DELETE,
            &format!("/organization/{organization_id}/cluster/{cluster_id}"),
            &[("deleteMode", "DELETE_QOVERY_CONFIG".to_string())],
            None,
        )?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{BufRead, BufReader, Read, Write};
    use std::sync::{Arc, Mutex};

    fn json_eq(expected: &str, actual: &Value) {
        let expected: Value = serde_json::from_str(expected).unwrap();
        assert_eq!(expected, *actual);
    }

    fn test_platform_configuration(key: &str, version: &str) -> PlatformConfiguration {
        PlatformConfiguration::new(key, version)
    }

    #[test]
    fn cluster_request_serializes_is_demo() {
        let request = new_demo_cluster_request(
            "local-demo-user",
            &Credential {
                id: "credential-id".into(),
                name: "on-premise".into(),
            },
        );
        assert_eq!(json!(true), request["is_demo"]);
        assert_eq!(json!("ON_PREMISE"), request["cloud_provider"]);
        assert_eq!(json!("SELF_MANAGED"), request["kubernetes"]);
        assert_eq!(json!(false), request["production"]);
    }

    #[test]
    fn platform_configuration_rejects_non_string_cluster_inputs() {
        let decode = |inputs: Value| {
            PlatformConfiguration::from_json(&json!({"platform": {}, "clusterInputs": inputs}))
                .map(|c| c.cluster_inputs)
                .map_err(|e| e.to_string())
        };

        assert_eq!(
            Err("json: cannot unmarshal number into Go struct field _ClusterPlatformConfigurationResponse.clusterInputs.qovery-operator.replicas of type string".to_string()),
            decode(json!({"qovery-operator": {"input": "value", "replicas": 2}}))
        );
        assert_eq!(
            Err("json: cannot unmarshal string into Go struct field _ClusterPlatformConfigurationResponse.clusterInputs.qovery-operator of type map[string]string".to_string()),
            decode(json!({"qovery-operator": "value"}))
        );
        assert_eq!(
            Err("json: cannot unmarshal array into Go struct field _ClusterPlatformConfigurationResponse.clusterInputs of type map[string]map[string]string".to_string()),
            decode(json!([]))
        );
        assert_eq!(
            Ok(Some(BTreeMap::from([(
                "qovery-operator".to_string(),
                BTreeMap::from([("input".to_string(), String::new())]),
            )]))),
            decode(json!({"qovery-operator": {"input": null}}))
        );
    }

    #[test]
    fn operator_configuration_preserves_existing_configuration() {
        let mut configuration = test_platform_configuration(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        configuration.platform.layer_selections =
            Some(BTreeMap::from([("qovery-stack".to_string(), false)]));
        configuration.platform.managed_config = Some(BTreeMap::from([(
            "qovery-operator".to_string(),
            BTreeMap::from([("existing".to_string(), json!("value"))]),
        )]));
        configuration.cluster_inputs = Some(BTreeMap::from([(
            "qovery-operator".to_string(),
            BTreeMap::from([("input".to_string(), "value".to_string())]),
        )]));

        let request =
            new_demo_operator_configuration_request(Some(&configuration), "ARM64").unwrap();

        json_eq(
            r#"{"platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.1.0", "layerSelections": {"qovery-stack": false},
                "managedConfig": {"qovery-operator": {"existing": "value", "cpuArchitectures": "ARM64"}}},
                "clusterInputs": {"qovery-operator": {"input": "value"}}}"#,
            &request.to_request_json(),
        );
    }

    #[test]
    fn operator_configuration_request_always_sends_cluster_inputs() {
        let mut configuration = PlatformConfiguration::new(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        configuration.cluster_inputs = None;
        let request =
            new_demo_operator_configuration_request(Some(&configuration), "AMD64").unwrap();
        json_eq(
            r#"{"platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.1.0", "layerSelections": {},
                "managedConfig": {"qovery-operator": {"cpuArchitectures": "AMD64"}}}, "clusterInputs": {}}"#,
            &request.to_request_json(),
        );
    }

    #[test]
    fn platform_configuration_selects_the_demo_template() {
        let catalog = json!({"results": [
            {"key": "qovery-cluster-v0", "version": "0.1.0"},
            {"key": DEMO_PLATFORM_TEMPLATE_KEY, "version": "0.2.0", "layers": [
                {"key": "qovery-stack", "mandatory": false, "enabledByDefault": true},
                {"key": "dns-certificates", "mandatory": true, "enabledByDefault": true},
            ]},
        ]});
        let configuration = new_demo_platform_configuration(Some(&catalog)).unwrap();
        assert_eq!(
            DEMO_PLATFORM_TEMPLATE_KEY,
            configuration.platform.template_key
        );
        assert_eq!("0.2.0", configuration.platform.template_version);
        assert_eq!(
            Some(BTreeMap::from([("qovery-stack".to_string(), true)])),
            configuration.platform.layer_selections
        );
        assert!(configuration.platform.managed_config.unwrap().is_empty());
        assert!(configuration.cluster_inputs.unwrap().is_empty());
    }

    #[test]
    fn platform_configuration_requires_the_demo_template() {
        let catalog = json!({"results": [{"key": "qovery-cluster-v0", "version": "0.1.0"}]});
        let err = new_demo_platform_configuration(Some(&catalog)).unwrap_err();
        assert_eq!(
            r#"qovery returned no "qovery-demo-v0" platform template for the local demo"#,
            err.to_string()
        );
    }

    #[test]
    fn select_preserves_matching_configuration() {
        let mut existing = test_platform_configuration(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        existing.platform.managed_config = Some(BTreeMap::from([(
            "qovery-operator".to_string(),
            BTreeMap::from([("existing".to_string(), json!("value"))]),
        )]));
        let desired = test_platform_configuration(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        let (selected, is_existing) = select_demo_platform_configuration(Some(&existing), &desired);
        assert!(is_existing);
        assert_eq!(existing, selected);
    }

    #[test]
    fn select_replaces_standard_configuration() {
        let mut existing = test_platform_configuration("qovery-cluster-v0", "0.1.0");
        existing.platform.layer_selections =
            Some(BTreeMap::from([("log-infra".to_string(), false)]));
        let mut desired = test_platform_configuration(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        desired.platform.layer_selections = Some(BTreeMap::new());
        let (selected, is_existing) = select_demo_platform_configuration(Some(&existing), &desired);
        assert!(!is_existing);
        assert!(
            !selected
                .platform
                .layer_selections
                .unwrap()
                .contains_key("log-infra")
        );
    }

    #[test]
    fn rejects_redacted_values_it_would_send_back() {
        let mut configuration = test_platform_configuration(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        let config = json!({
            "qovery-operator": {"cpuArchitectures": "<redacted>"},
            "karpenter": {"replicas": 2, "token": "<redacted>"},
            "cert-manager": {"email": "<redacted>", "issuer": "<redacted>"},
            "external-dns": {"provider": "cloudflare"},
        });
        configuration.platform.managed_config = Some(serde_json::from_value(config).unwrap());
        let request =
            new_demo_operator_configuration_request(Some(&configuration), "ARM64").unwrap();
        let err = ensure_demo_request_has_no_redacted_values(&request).unwrap_err();
        assert_eq!(
            "qovery returned redacted values for cert-manager.email, cert-manager.issuer, karpenter.token; qovery demo sends the whole configuration back and would store the redaction marker instead, so nothing was saved. Retry later; if it persists, these fields are sensitive and qovery demo cannot keep them: clear them, or recreate the cluster with `qovery demo destroy --delete-qovery-config`",
            err.to_string()
        );
    }

    #[test]
    fn normalize_cpu_architecture() {
        assert_eq!("ARM64", normalize_demo_cpu_architecture(" arm64 ").unwrap());
        assert_eq!("AMD64", normalize_demo_cpu_architecture("AMD64").unwrap());
        assert_eq!(
            r#"unsupported local CPU architecture "386""#,
            normalize_demo_cpu_architecture("386")
                .unwrap_err()
                .to_string()
        );
    }

    #[test]
    fn accepts_unredacted_values() {
        let mut request = PlatformConfiguration::new(DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0");
        request.platform.managed_config = Some(
            serde_json::from_value(json!({"qovery-operator": {
                "cpuArchitectures": "ARM64", "note": "redacted", "nested": {"token": "<redacted>"},
            }}))
            .unwrap(),
        );
        assert!(ensure_demo_request_has_no_redacted_values(&request).is_ok());
    }

    #[test]
    fn find_demo_cluster_refuses_a_cluster_that_is_not_a_demo() {
        let clusters = vec![
            json!({"id": "demo-id", "name": "local-demo-user", "is_demo": true}),
            json!({"id": "prod-id", "name": "production", "is_demo": false}),
            json!({"id": "legacy-id", "name": "unflagged"}),
        ];
        assert_eq!(
            ("demo-id".to_string(), true),
            find_demo_cluster(&clusters, "local-demo-user").unwrap()
        );
        assert_eq!(
            (String::new(), false),
            find_demo_cluster(&clusters, "missing").unwrap()
        );
        for name in ["production", "unflagged"] {
            let err = find_demo_cluster(&clusters, name).unwrap_err();
            assert!(err.to_string().contains("is not a demo cluster"));
            assert!(err.not_a_demo);
        }
    }

    #[test]
    fn interface_roundtrip_numbers() {
        assert_eq!(
            "2",
            go_interface_roundtrip(&serde_json::from_str("2.0").unwrap()).to_string()
        );
        assert_eq!("2.5", go_interface_roundtrip(&json!(2.5)).to_string());
    }

    // --- fakeDemoPlatformServer -------------------------------------------------------------------------

    const TEST_DEMO_PLATFORM_CATALOG: &str = r#"{"results": [
        {"key": "qovery-cluster-v0", "version": "0.1.0", "status": "PUBLISHED", "description": null, "layers": []},
        {"key": "qovery-demo-v0", "version": "0.2.0", "status": "PUBLISHED", "description": "Local demo cluster",
         "layers": [
            {"key": "qovery-stack", "mandatory": false, "enabledByDefault": true, "modes": ["CUSTOMER_MANAGED"], "componentKeys": [], "components": []},
            {"key": "dns-certificates", "mandatory": true, "enabledByDefault": true, "modes": ["CUSTOMER_MANAGED"], "componentKeys": [], "components": []}
         ]}
    ]}"#;

    const TEST_DEMO_DEFAULT_PLATFORM_CONFIGURATION_REQUEST: &str = r#"{
        "platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": true},
                     "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64"}}},
        "clusterInputs": {}
    }"#;

    const LIST_TEMPLATES: &str = "GET /organization/organization-id/platformTemplate";
    const READ_CONFIGURATION: &str = "GET /v1/cluster/cluster-id/platformConfiguration";
    const SAVE_CONFIGURATION: &str = "PUT /v1/cluster/cluster-id/platformConfiguration";

    #[derive(Default)]
    struct ServerState {
        configuration: String,
        failures: BTreeMap<String, (u16, String)>,
        once: BTreeMap<String, (u16, String)>,
        received: Vec<String>,
        saved: String,
    }

    struct FakePlatformServer {
        url: String,
        state: Arc<Mutex<ServerState>>,
    }

    impl FakePlatformServer {
        fn new(configuration: &str) -> Self {
            let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
            let url = format!("http://{}", listener.local_addr().unwrap());
            let state = Arc::new(Mutex::new(ServerState {
                configuration: configuration.to_string(),
                ..Default::default()
            }));
            let shared = state.clone();
            std::thread::spawn(move || {
                for stream in listener.incoming() {
                    let Ok(mut stream) = stream else { continue };
                    let mut reader = BufReader::new(stream.try_clone().unwrap());
                    let mut line = String::new();
                    if reader.read_line(&mut line).is_err() {
                        continue;
                    }
                    let mut parts = line.split_whitespace();
                    let method = parts.next().unwrap_or("").to_string();
                    let target = parts.next().unwrap_or("").to_string();
                    let path = target.split('?').next().unwrap_or("").to_string();
                    let mut length = 0usize;
                    loop {
                        let mut header = String::new();
                        if reader.read_line(&mut header).is_err() || header.trim().is_empty() {
                            break;
                        }
                        if let Some((k, v)) = header.split_once(':')
                            && k.eq_ignore_ascii_case("content-length")
                        {
                            length = v.trim().parse().unwrap_or(0);
                        }
                    }
                    let mut body = vec![0u8; length];
                    let _ = reader.read_exact(&mut body);
                    let call = format!("{method} {path}");
                    let (status, payload) = {
                        let mut state = shared.lock().unwrap();
                        state.received.push(call.clone());
                        if let Some((status, body)) = state.once.remove(&call) {
                            (status, body)
                        } else if let Some((status, body)) = state.failures.get(&call) {
                            (*status, body.clone())
                        } else if call == LIST_TEMPLATES {
                            (200, TEST_DEMO_PLATFORM_CATALOG.to_string())
                        } else if call == READ_CONFIGURATION {
                            if state.configuration.is_empty() {
                                (404, String::new())
                            } else {
                                (200, state.configuration.clone())
                            }
                        } else if call == SAVE_CONFIGURATION {
                            state.saved = String::from_utf8_lossy(&body).to_string();
                            (200, r#"{"clusterId":"cluster-id","organizationId":"organization-id","platform":{"templateKey":"qovery-demo-v0","templateVersion":"0.2.0"},"clusterInputs":{},"layers":[]}"#.to_string())
                        } else {
                            (404, String::new())
                        }
                    };
                    let response = format!(
                        "HTTP/1.1 {} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                        status,
                        qovery_api::status_line(status)
                            .split_once(' ')
                            .map(|x| x.1)
                            .unwrap_or(""),
                        payload.len(),
                        payload
                    );
                    let _ = stream.write_all(response.as_bytes());
                }
            });
            FakePlatformServer { url, state }
        }

        fn fail(&self, call: &str, status: u16, body: &str) {
            self.state
                .lock()
                .unwrap()
                .failures
                .insert(call.to_string(), (status, body.to_string()));
        }

        fn fail_once(&self, call: &str, status: u16, body: &str) {
            self.state
                .lock()
                .unwrap()
                .once
                .insert(call.to_string(), (status, body.to_string()));
        }

        fn api(&self) -> QoveryApi {
            QoveryApi {
                client: Client::new("Token test", "test", false),
                base: self.url.clone(),
            }
        }

        fn requests(&self) -> Vec<String> {
            self.state.lock().unwrap().received.clone()
        }

        fn saved(&self) -> String {
            self.state.lock().unwrap().saved.clone()
        }
    }

    fn saved_json_eq(expected: &str, server: &FakePlatformServer) {
        let actual: Value = serde_json::from_str(&server.saved()).unwrap();
        json_eq(expected, &actual);
    }

    #[test]
    fn configure_operator_creates_the_demo_configuration() {
        let server = FakePlatformServer::new("");
        server
            .api()
            .configure_operator("organization-id", "cluster-id", "arm64")
            .unwrap();
        assert_eq!(
            vec![LIST_TEMPLATES, READ_CONFIGURATION, SAVE_CONFIGURATION],
            server.requests()
        );
        saved_json_eq(TEST_DEMO_DEFAULT_PLATFORM_CONFIGURATION_REQUEST, &server);
    }

    #[test]
    fn configure_operator_replaces_another_redacted_configuration() {
        for (key, version) in [
            ("qovery-cluster-v0", "0.2.0"),
            (DEMO_PLATFORM_TEMPLATE_KEY, "0.1.0"),
        ] {
            let server = FakePlatformServer::new(&format!(
                r#"{{"clusterId": "cluster-id", "organizationId": "organization-id",
                    "platform": {{"templateKey": "{key}", "templateVersion": "{version}",
                                  "managedConfig": {{"qovery-operator": {{"cpuArchitectures": "<redacted>", "logLevel": "<redacted>"}}}}}},
                    "clusterInputs": {{"qovery-operator": {{"input": "value"}}}}, "layers": []}}"#
            ));
            server
                .api()
                .configure_operator("organization-id", "cluster-id", "arm64")
                .unwrap();
            saved_json_eq(TEST_DEMO_DEFAULT_PLATFORM_CONFIGURATION_REQUEST, &server);
        }
    }

    #[test]
    fn configure_operator_overwrites_a_redacted_cpu_architecture() {
        let server = FakePlatformServer::new(
            r#"{"clusterId": "cluster-id", "organizationId": "organization-id",
                "platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": false},
                             "managedConfig": {"qovery-operator": {"cpuArchitectures": "<redacted>"}}},
                "clusterInputs": {"qovery-operator": {"input": "value"}}, "layers": []}"#,
        );
        server
            .api()
            .configure_operator("organization-id", "cluster-id", "arm64")
            .unwrap();
        saved_json_eq(
            r#"{"platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": false},
                             "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64"}}},
                "clusterInputs": {"qovery-operator": {"input": "value"}}}"#,
            &server,
        );
    }

    #[test]
    fn configure_operator_updates_the_existing_demo_configuration() {
        let server = FakePlatformServer::new(
            r#"{"clusterId": "cluster-id", "organizationId": "organization-id",
                "platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": false},
                             "managedConfig": {"qovery-operator": {"cpuArchitectures": "AMD64", "logLevel": "debug"}}},
                "clusterInputs": {"qovery-operator": {"input": "value"}}, "layers": []}"#,
        );
        server
            .api()
            .configure_operator("organization-id", "cluster-id", "arm64")
            .unwrap();
        saved_json_eq(
            r#"{"platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": false},
                             "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64", "logLevel": "debug"}}},
                "clusterInputs": {"qovery-operator": {"input": "value"}}}"#,
            &server,
        );
    }

    #[test]
    fn configure_operator_does_not_save_redacted_configuration() {
        let server = FakePlatformServer::new(
            r#"{"clusterId": "cluster-id", "organizationId": "organization-id",
                "platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": true},
                             "managedConfig": {"qovery-operator": {"cpuArchitectures": "<redacted>", "logLevel": "<redacted>"}}},
                "clusterInputs": {}, "layers": []}"#,
        );
        let err = server
            .api()
            .configure_operator("organization-id", "cluster-id", "arm64")
            .unwrap_err();
        assert!(
            err.to_string()
                .contains("qovery returned redacted values for qovery-operator.logLevel;")
        );
        assert_eq!(vec![LIST_TEMPLATES, READ_CONFIGURATION], server.requests());
        assert!(server.saved().is_empty());
    }

    #[test]
    fn configure_operator_skips_an_up_to_date_configuration() {
        let server = FakePlatformServer::new(
            r#"{"clusterId": "cluster-id", "organizationId": "organization-id",
                "platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": true},
                             "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64", "logLevel": "<redacted>"}}},
                "clusterInputs": {}, "layers": []}"#,
        );
        server
            .api()
            .configure_operator("organization-id", "cluster-id", "arm64")
            .unwrap();
        assert_eq!(vec![LIST_TEMPLATES, READ_CONFIGURATION], server.requests());
        assert!(server.saved().is_empty());
    }

    #[test]
    fn configure_operator_rejects_an_unsupported_architecture_before_calling_qovery() {
        let server = FakePlatformServer::new("");
        let err = server
            .api()
            .configure_operator("organization-id", "cluster-id", "386")
            .unwrap_err();
        assert_eq!(
            r#"unsupported local CPU architecture "386""#,
            err.to_string()
        );
        assert!(server.requests().is_empty());
    }

    #[test]
    fn configure_operator_returns_qovery_errors() {
        let conflict = r#"{"type":"about:blank","title":"Conflict","status":409,"detail":"The cluster uses a platform profile owned by another cluster"}"#;
        let cases: Vec<(&str, u16, String, String, Vec<&str>)> = vec![
            (
                LIST_TEMPLATES,
                503,
                String::new(),
                "cannot list the platform templates: 503 Service Unavailable".into(),
                vec![LIST_TEMPLATES],
            ),
            (
                READ_CONFIGURATION,
                403,
                String::new(),
                "cannot read the cluster platform configuration: 403 Forbidden".into(),
                vec![LIST_TEMPLATES, READ_CONFIGURATION],
            ),
            (
                READ_CONFIGURATION,
                500,
                String::new(),
                "cannot read the cluster platform configuration: 500 Internal Server Error".into(),
                vec![LIST_TEMPLATES, READ_CONFIGURATION],
            ),
            (
                SAVE_CONFIGURATION,
                409,
                format!("{conflict}\n"),
                format!("cannot save the cluster platform configuration: 409 Conflict: {conflict}"),
                vec![LIST_TEMPLATES, READ_CONFIGURATION, SAVE_CONFIGURATION],
            ),
            (
                SAVE_CONFIGURATION,
                400,
                String::new(),
                "cannot save the cluster platform configuration: 400 Bad Request".into(),
                vec![LIST_TEMPLATES, READ_CONFIGURATION, SAVE_CONFIGURATION],
            ),
        ];
        for (call, status, body, error, requests) in cases {
            let server = FakePlatformServer::new("");
            server.fail(call, status, &body);
            let err = server
                .api()
                .configure_operator("organization-id", "cluster-id", "arm64")
                .unwrap_err();
            assert_eq!(error, err.to_string());
            assert_eq!(requests, server.requests());
        }
    }

    const CLUSTER_STATUS: &str = "GET /organization/organization-id/cluster/cluster-id/status";

    #[test]
    fn call_retries_once_with_a_fresh_client_on_unauthorized() {
        // SAFETY: no other test of this crate reads the environment concurrently.
        unsafe { std::env::set_var("QOVERY_CLI_ACCESS_TOKEN", "qov_test-token") };
        let server = FakePlatformServer::new("");
        server.fail_once(CLUSTER_STATUS, 401, "");
        server.fail(CLUSTER_STATUS, 200, r#"{"status":"DEPLOYED"}"#);
        let mut api = server.api();
        api.client = Client::new("Bearer expired", "test", false);

        let status = api
            .get_cluster_status("organization-id", "cluster-id")
            .unwrap();

        assert_eq!("DEPLOYED", status);
        assert_eq!(vec![CLUSTER_STATUS, CLUSTER_STATUS], server.requests());
        assert_eq!("Token qov_test-token", api.client.authorization());
    }

    #[test]
    fn call_does_not_retry_other_errors() {
        let server = FakePlatformServer::new("");
        server.fail(CLUSTER_STATUS, 500, "");
        let err = server
            .api()
            .get_cluster_status("organization-id", "cluster-id")
            .unwrap_err();
        assert_eq!("500 Internal Server Error", err.to_string());
        assert_eq!(vec![CLUSTER_STATUS], server.requests());
    }

    #[test]
    fn call_rejects_an_empty_successful_response() {
        let server = FakePlatformServer::new("");
        server.fail(CLUSTER_STATUS, 200, "");
        let err = server
            .api()
            .get_cluster_status("organization-id", "cluster-id")
            .unwrap_err();
        assert!(err.to_string().contains("empty response"));
    }
}
