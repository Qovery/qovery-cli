//! Environment variable and secret helpers of `utils/qovery.go` (`AddEnvironmentVariable`, `DeleteSecret`,
//! `AddContainerEnvironmentVariable`, ...). Like Go, they create their own API client; the `_with`
//! variants take an existing client.

use crate::util::{path_escape, str_field};
use qovery_api::{Client, Method};
use qovery_core::client::get_client;
use serde_json::{Value, json};

/// `application` / `container` and the matching resource word of the Go messages.
#[derive(Clone, Copy)]
enum Owner {
    Application,
    Container,
}

impl Owner {
    fn path(self) -> &'static str {
        match self {
            Owner::Application => "application",
            Owner::Container => "container",
        }
    }
}

#[derive(Clone, Copy)]
enum Kind {
    EnvironmentVariable,
    Secret,
}

impl Kind {
    fn path(self) -> &'static str {
        match self {
            Kind::EnvironmentVariable => "environmentVariable",
            Kind::Secret => "secret",
        }
    }
}

fn add(
    client: &Client,
    owner: Owner,
    kind: Kind,
    id: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    let body = json!({"key": key, "value": value});
    client
        .execute(
            Method::POST,
            &format!("/{}/{}/{}", owner.path(), path_escape(id), kind.path()),
            &[],
            Some(&body),
        )
        .map(|_| ())
        .map_err(|e| e.to_string())
}

fn delete(client: &Client, owner: Owner, kind: Kind, id: &str, key: &str) -> Result<(), String> {
    let base = format!("/{}/{}/{}", owner.path(), path_escape(id), kind.path());
    let items: Vec<Value> = client.list(&base).map_err(|e| e.to_string())?;
    let Some(item) = items.iter().find(|v| str_field(v, "key") == key) else {
        return Ok(());
    };
    client
        .delete(&format!("{base}/{}", path_escape(str_field(item, "id"))))
        .map(|_| ())
        .map_err(|e| e.to_string())
}

/// Port of `utils.AddEnvironmentVariable(application, key, value)`
/// (`POST /application/{id}/environmentVariable` with `{"key","value"}`).
pub fn add_environment_variable(application: &str, key: &str, value: &str) -> Result<(), String> {
    add_environment_variable_with(&get_client()?, application, key, value)
}

/// [`add_environment_variable`] with an existing client.
pub fn add_environment_variable_with(
    client: &Client,
    application: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    add(
        client,
        Owner::Application,
        Kind::EnvironmentVariable,
        application,
        key,
        value,
    )
}

/// Port of `utils.DeleteEnvironmentVariable(application, key)`: lists the variables of the application and
/// deletes the first one with this key (nothing when none).
pub fn delete_environment_variable(application: &str, key: &str) -> Result<(), String> {
    delete_environment_variable_with(&get_client()?, application, key)
}

/// [`delete_environment_variable`] with an existing client.
pub fn delete_environment_variable_with(
    client: &Client,
    application: &str,
    key: &str,
) -> Result<(), String> {
    delete(
        client,
        Owner::Application,
        Kind::EnvironmentVariable,
        application,
        key,
    )
}

/// Port of `utils.AddSecret(application, key, value)` (`POST /application/{id}/secret`).
pub fn add_secret(application: &str, key: &str, value: &str) -> Result<(), String> {
    add_secret_with(&get_client()?, application, key, value)
}

/// [`add_secret`] with an existing client.
pub fn add_secret_with(
    client: &Client,
    application: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    add(
        client,
        Owner::Application,
        Kind::Secret,
        application,
        key,
        value,
    )
}

/// Port of `utils.DeleteSecret(application, key)`.
pub fn delete_secret(application: &str, key: &str) -> Result<(), String> {
    delete_secret_with(&get_client()?, application, key)
}

/// [`delete_secret`] with an existing client.
pub fn delete_secret_with(client: &Client, application: &str, key: &str) -> Result<(), String> {
    delete(client, Owner::Application, Kind::Secret, application, key)
}

/// Port of `utils.AddContainerEnvironmentVariable(container, key, value)`.
pub fn add_container_environment_variable(
    container: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    add_container_environment_variable_with(&get_client()?, container, key, value)
}

/// [`add_container_environment_variable`] with an existing client.
pub fn add_container_environment_variable_with(
    client: &Client,
    container: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    add(
        client,
        Owner::Container,
        Kind::EnvironmentVariable,
        container,
        key,
        value,
    )
}

/// Port of `utils.DeleteContainerEnvironmentVariable(container, key)`.
pub fn delete_container_environment_variable(container: &str, key: &str) -> Result<(), String> {
    delete_container_environment_variable_with(&get_client()?, container, key)
}

/// [`delete_container_environment_variable`] with an existing client.
pub fn delete_container_environment_variable_with(
    client: &Client,
    container: &str,
    key: &str,
) -> Result<(), String> {
    delete(
        client,
        Owner::Container,
        Kind::EnvironmentVariable,
        container,
        key,
    )
}

/// Port of `utils.AddContainerSecret(container, key, value)`.
pub fn add_container_secret(container: &str, key: &str, value: &str) -> Result<(), String> {
    add_container_secret_with(&get_client()?, container, key, value)
}

/// [`add_container_secret`] with an existing client.
pub fn add_container_secret_with(
    client: &Client,
    container: &str,
    key: &str,
    value: &str,
) -> Result<(), String> {
    add(
        client,
        Owner::Container,
        Kind::Secret,
        container,
        key,
        value,
    )
}

/// Port of `utils.DeleteContainerSecret(container, key)`.
pub fn delete_container_secret(container: &str, key: &str) -> Result<(), String> {
    delete_container_secret_with(&get_client()?, container, key)
}

/// [`delete_container_secret`] with an existing client.
pub fn delete_container_secret_with(
    client: &Client,
    container: &str,
    key: &str,
) -> Result<(), String> {
    delete(client, Owner::Container, Kind::Secret, container, key)
}
