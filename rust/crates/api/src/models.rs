//! Minimal typed models of the most used API resources.
//!
//! Only the fields the shared code needs are typed; every other field is kept in `extra`, so a model can
//! be turned back into the full JSON with [`Resource::to_value`]. Commands needing other resources can
//! define their own structs or use `serde_json::Value`.
//!
//! Like Go's `encoding/json`, an explicit `null` leaves the zero value (`""`, empty list) instead of
//! failing: every non-`Option` field goes through [`null_as_default`].

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};

/// Deserialize `null` as the default value (Go decodes `null` into an empty slice/struct).
pub fn null_as_default<'de, D, T>(d: D) -> Result<T, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Default + Deserialize<'de>,
{
    Ok(Option::<T>::deserialize(d)?.unwrap_or_default())
}

/// `{"results": [...]}` list responses (`"results": null` decodes as an empty list, like Go).
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ResultList<T> {
    #[serde(
        default = "Vec::new",
        deserialize_with = "null_as_default",
        bound(deserialize = "T: Deserialize<'de>")
    )]
    pub results: Vec<T>,
}

/// Resources that keep their unknown fields.
pub trait Resource: Serialize {
    fn to_value(&self) -> Value {
        serde_json::to_value(self).unwrap_or(Value::Null)
    }
}

macro_rules! resource {
    ($(#[$m:meta])* $name:ident { $($(#[$fm:meta])* pub $field:ident : $ty:ty),* $(,)? }) => {
        $(#[$m])*
        #[derive(Debug, Clone, Default, Deserialize, Serialize)]
        pub struct $name {
            #[serde(default, deserialize_with = "null_as_default")]
            pub id: String,
            #[serde(default, deserialize_with = "null_as_default")]
            pub name: String,
            $($(#[$fm])* #[serde(default, deserialize_with = "null_as_default")] pub $field: $ty,)*
            #[serde(flatten)]
            pub extra: Map<String, Value>,
        }
        impl Resource for $name {}
    };
}

resource!(Organization {
    pub description: Option<String>,
    pub created_at: Option<String>,
    pub updated_at: Option<String>,
});

resource!(Project {
    pub description: Option<String>,
    pub organization: Option<ReferenceObject>,
    pub created_at: Option<String>,
    pub updated_at: Option<String>,
});

resource!(Environment {
    pub mode: String,
    pub cluster_id: String,
    pub cluster_name: Option<String>,
    pub project: Option<ReferenceObject>,
    pub organization: Option<ReferenceObject>,
    pub created_at: Option<String>,
    pub updated_at: Option<String>,
});

resource!(Application {
    pub environment: Option<ReferenceObject>,
    pub git_repository: Option<Value>,
    pub created_at: Option<String>,
    pub updated_at: Option<String>,
});

resource!(
    /// `ContainerResponse`
    Container {
        pub environment: Option<ReferenceObject>,
        pub image_name: String,
        pub tag: String,
        pub created_at: Option<String>,
        pub updated_at: Option<String>,
    }
);

resource!(
    /// `JobResponse` (oneOf `CronJobResponse` / `LifecycleJobResponse`, discriminated by `job_type`)
    Job {
        pub job_type: String,
        pub environment: Option<ReferenceObject>,
        pub source: Option<Value>,
        pub schedule: Option<Value>,
        pub created_at: Option<String>,
        pub updated_at: Option<String>,
    }
);

impl Job {
    /// Go oneOf discriminator: `CRON` or the schema name `CronJobResponse`.
    pub fn is_cron(&self) -> bool {
        self.job_type == "CRON" || self.job_type == "CronJobResponse"
    }
    /// Go oneOf discriminator: `LIFECYCLE` or the schema name `LifecycleJobResponse`.
    pub fn is_lifecycle(&self) -> bool {
        self.job_type == "LIFECYCLE" || self.job_type == "LifecycleJobResponse"
    }
}

resource!(Database {
    pub environment: Option<ReferenceObject>,
    #[serde(rename = "type")]
    pub db_type: String,
    pub mode: String,
    pub version: String,
    pub created_at: Option<String>,
    pub updated_at: Option<String>,
});

resource!(
    /// `HelmResponse`
    Helm {
        pub environment: Option<ReferenceObject>,
        pub source: Option<Value>,
        pub created_at: Option<String>,
        pub updated_at: Option<String>,
    }
);

resource!(
    /// `TerraformResponse`
    Terraform {
        pub environment: Option<ReferenceObject>,
        pub terraform_files_source: Option<Value>,
        pub created_at: Option<String>,
        pub updated_at: Option<String>,
    }
);

resource!(Cluster {
    pub region: String,
    pub cloud_provider: String,
    pub kubernetes: Option<String>,
    pub status: Option<String>,
    pub version: Option<String>,
    pub created_at: Option<String>,
    pub updated_at: Option<String>,
});

resource!(CustomDomain {
    pub domain: String,
    pub validation_domain: Option<String>,
    pub status: Option<String>,
    pub generate_certificate: Option<bool>,
    pub use_cdn: Option<bool>,
});

/// `{"id": "..."}` references.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct ReferenceObject {
    #[serde(default, deserialize_with = "null_as_default")]
    pub id: String,
    #[serde(flatten)]
    pub extra: Map<String, Value>,
}

/// `Status` of a service.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct Status {
    #[serde(default, deserialize_with = "null_as_default")]
    pub id: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub state: String,
    #[serde(default)]
    pub service_deployment_status: Option<String>,
    #[serde(default)]
    pub last_deployment_date: Option<String>,
    #[serde(default)]
    pub is_part_last_deployment: Option<bool>,
    #[serde(default)]
    pub execution_id: Option<String>,
    #[serde(default)]
    pub status_details: Option<Value>,
    #[serde(default)]
    pub steps: Option<Value>,
    #[serde(flatten)]
    pub extra: Map<String, Value>,
}

/// `EnvironmentStatus`.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct EnvironmentStatus {
    #[serde(default, deserialize_with = "null_as_default")]
    pub id: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub state: String,
    #[serde(default)]
    pub last_deployment_state: Option<String>,
    #[serde(default)]
    pub last_deployment_date: Option<String>,
    #[serde(default)]
    pub last_deployment_id: Option<String>,
    #[serde(default)]
    pub total_deployment_duration_in_seconds: Option<i64>,
    #[serde(default)]
    pub origin: Option<String>,
    #[serde(default)]
    pub triggered_by: Option<String>,
    #[serde(flatten)]
    pub extra: Map<String, Value>,
}

/// `EnvironmentStatuses` (`GET /environment/{id}/statuses`).
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct EnvironmentStatuses {
    #[serde(default, deserialize_with = "null_as_default")]
    pub environment: EnvironmentStatus,
    #[serde(default, deserialize_with = "null_as_default")]
    pub applications: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub containers: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub jobs: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub databases: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub helms: Vec<Status>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub terraforms: Vec<Status>,
    #[serde(flatten)]
    pub extra: Map<String, Value>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn null_is_the_zero_value() {
        let list: ResultList<CustomDomain> = serde_json::from_str(
            r#"{"results": [{"id": null, "name": null, "domain": null, "status": null}]}"#,
        )
        .unwrap();
        assert_eq!(list.results[0].id, "");
        assert_eq!(list.results[0].domain, "");
        assert_eq!(list.results[0].status, None);
        let list: ResultList<Environment> = serde_json::from_str(r#"{"results": null}"#).unwrap();
        assert!(list.results.is_empty());
        let s: EnvironmentStatuses = serde_json::from_str(
            r#"{"environment": {"id": null, "state": null}, "applications": [{"id": "a", "state": null}]}"#,
        )
        .unwrap();
        assert_eq!(s.environment.state, "");
        assert_eq!(s.applications[0].state, "");
        let p: Project =
            serde_json::from_str(r#"{"id": "p", "organization": {"id": null}}"#).unwrap();
        assert_eq!(p.organization.unwrap().id, "");
    }
}
