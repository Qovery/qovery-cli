//! Organization related admin commands: delete, deployment-restriction, get-organization-plan,
//! update-organization-plan, transfer-ownership, update-billing-external-id.
//! Port of `cmd/admin_delete_orga.go`, `cmd/admin_organization_*.go` and `pkg/delete_orga.go`.

use crate::goutil::path_escape;
use crate::http::{Req, authorization};
use qovery_api::Method as ApiMethod;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::client::admin_url;
use qovery_core::select::validate;
use qovery_core::{check_error, exit, exit_with_error};
use qovery_log::gofmt::go_json;
use qovery_log::logrus;
use reqwest::Method;
use serde_json::{Value, json};
use std::time::Duration;

fn str_field(v: &Value, k: &str) -> String {
    v.get(k)
        .and_then(|x| x.as_str())
        .unwrap_or_default()
        .to_string()
}

// ---------------------------------------------------------------- delete

pub fn delete() -> Command {
    Command::new("delete")
        .short("Delete one or more organizations by their IDs")
        .long(
            "Delete one or more organizations by providing their IDs.

Examples:
  # Delete a single organization
  qovery admin organization delete --organization-id org-123

  # Delete multiple organizations (comma-separated)
  qovery admin organization delete --organization-id \"org-123,org-456,org-789\"

  # Delete multiple organizations (repeated flag)
  qovery admin organization delete --organization-id org-123 --organization-id org-456

  # Mix both formats
  qovery admin organization delete -o \"org-123,org-456\" -o org-789

  # Allow deletion of organizations with failed clusters
  qovery admin organization delete -o org-123 --allow-failed-clusters

  # Disable dry-run to actually delete
  qovery admin organization delete -o org-123 --disable-dry-run",
        )
        .flag(
            Flag::string_slice(
                "organization-id",
                "Organization ID(s) to delete (comma-separated or repeated flag)",
            )
            .short('o')
            .required(),
        )
        .flag(
            Flag::bool(
                "allow-failed-clusters",
                "Allow deletion of organizations with failed or non-deployed clusters",
            )
            .short('f'),
        )
        .flag(Flag::bool("disable-dry-run", "Disable dry run mode").short('y'))
        .run(run_delete)
}

/// Split `"id1, id2"` values again on commas and trim them (`deleteOrganizations`).
fn parse_organization_ids(values: &[String]) -> Vec<String> {
    values
        .iter()
        .flat_map(|v| v.split(','))
        .map(|p| p.trim().to_string())
        .filter(|p| !p.is_empty())
        .collect()
}

fn run_delete(ctx: &Ctx) {
    let ids = parse_organization_ids(&ctx.strings("organization-id"));
    delete_organizations(
        &ids,
        ctx.bool("allow-failed-clusters"),
        ctx.bool("disable-dry-run"),
    );
}

/// `pkg.DeleteOrganizations`.
fn delete_organizations(
    organization_ids: &[String],
    allow_failed_clusters: bool,
    dry_run_disabled: bool,
) {
    admin_url();
    qovery_log::dry_run_print(dry_run_disabled);
    if organization_ids.is_empty() {
        logrus::error("No organization IDs provided");
        exit(1);
    }
    if !validate("delete") {
        return;
    }
    let mut url = format!("{}/organizations", admin_url());
    if allow_failed_clusters {
        url.push_str("?allowFailedClusters=true");
    }
    let body = go_json(&json!(organization_ids));
    if !dry_run_disabled {
        println!(
            "Would delete {} organization(s) (allowFailedClusters={allow_failed_clusters}):",
            organization_ids.len()
        );
        for id in organization_ids {
            println!("  - {id}");
        }
        return;
    }
    let Some(res) = crate::misc::delete_with_body(&url, true, Some(body.into_bytes())) else {
        logrus::error("Failed to execute delete request");
        return;
    };
    if res.status != 200 {
        logrus::error(format!(
            "Failed to delete organizations (status {}): {}",
            res.status,
            res.text()
        ));
        exit(1);
    }
    if let Some(e) = &res.read_err {
        logrus::error(format!("Failed to read response: {e}"));
        exit(1);
    }
    if let Err(e) = crate::gojson::check_valid(&res.body) {
        logrus::error(format!("Failed to parse response: {e}"));
        exit(1);
    }
    let response: Value = serde_json::from_slice(&res.body).unwrap_or(Value::Null);
    display_deletion_results(&response, organization_ids.len());
}

fn display_deletion_results(response: &Value, total_requested: usize) {
    let deleted: Vec<String> = response
        .get("deleted")
        .and_then(|d| d.as_array())
        .map(|a| {
            a.iter()
                .map(|x| x.as_str().unwrap_or_default().to_string())
                .collect()
        })
        .unwrap_or_default();
    let failed: Vec<(String, String)> = response
        .get("failed")
        .and_then(|d| d.as_array())
        .map(|a| {
            a.iter()
                .map(|f| (str_field(f, "organization_id"), str_field(f, "reason")))
                .collect()
        })
        .unwrap_or_default();

    println!();
    println!("====================================");
    println!("   Organization Deletion Results");
    println!("====================================");
    println!();
    println!("Total requested: {total_requested}");
    println!("Successfully deleted: {}", deleted.len());
    println!("Failed to delete: {}", failed.len());
    println!();
    if !deleted.is_empty() {
        println!("✓ Successfully deleted organizations:");
        for id in &deleted {
            println!("  ✓ {id}");
        }
        println!();
    }
    if !failed.is_empty() {
        println!("✗ Failed to delete organizations:");
        for (id, reason) in &failed {
            println!("  ✗ {id}");
            println!("    Reason: {reason}");
        }
        println!();
    }
    if !failed.is_empty() {
        println!("Some deletions failed. Check the reasons above.");
        exit(1);
    } else {
        println!("All organizations deleted successfully! ✓");
    }
}

// ---------------------------------------------------------------- deployment-restriction

pub fn deployment_restriction() -> Command {
    Command::new("deployment-restriction")
        .short("Block or unblock organization deployments")
        .long(
            "Manage organization deployment restrictions.
This command allows you to block or unblock deployments for a specific organization.

Examples:
  qovery admin deployment-restriction --organization-id 12345678-1234-1234-1234-123456789abc --action block --message \"Payment overdue\"
  qovery admin deployment-restriction --organization-id 12345678-1234-1234-1234-123456789abc --action unblock",
        )
        .flag(Flag::string("organization-id", "Organization ID").short('o').required())
        .flag(Flag::string("action", "Action to perform: block or unblock").short('a').required())
        .flag(
            Flag::string("message", "Message explaining the reason for blocking (required when action is 'block')").short('m'),
        )
        .run(run_deployment_restriction)
}

fn run_deployment_restriction(ctx: &Ctx) {
    let organization_id = ctx.string("organization-id");
    let action = ctx.string("action");
    let message = ctx.string("message");
    if action != "block" && action != "unblock" {
        exit_with_error(format!(
            "action must be either 'block' or 'unblock', got: {action}"
        ));
    }
    if organization_id.is_empty() {
        exit_with_error("organization ID is required");
    }
    if organization_id.len() != 36
        || !organization_id.contains('-')
        || organization_id.matches('-').count() != 4
    {
        exit_with_error(
            "organization ID must be a valid UUID format (e.g., 12345678-1234-1234-1234-123456789abc)",
        );
    }
    if action == "block" {
        if message.is_empty() {
            exit_with_error(
                "message is required when action is 'block'. Use --message flag to provide a reason",
            );
        }
        if message.trim().len() < 3 {
            exit_with_error("message must be at least 3 characters long");
        }
        if message.len() > 500 {
            exit_with_error("message must be less than 500 characters");
        }
    }
    if action == "unblock" && !message.is_empty() {
        exit_with_error("message should not be provided when action is 'unblock'");
    }

    qovery_log::println_info(format!(
        "You are about to {action} deployments for organization: {organization_id}"
    ));
    if action == "block" {
        qovery_log::println_info(format!("Reason: {message}"));
    }
    qovery_log::println_info("This action will affect ALL deployments for this organization.");
    if !validate("deployment restriction") {
        qovery_log::println_info("Operation cancelled.");
        return;
    }

    let auth = check_error(authorization());
    let mut payload = format!("{{\"action\":{}", go_json(&json!(action)));
    if !message.is_empty() {
        payload.push_str(&format!(",\"message\":{}", go_json(&json!(message))));
    }
    payload.push('}');
    let url = format!(
        "{}/organization/{organization_id}/deploymentRestriction",
        admin_url()
    );
    let res = match Req::new(Method::POST, &url, &auth).body(payload).send() {
        Ok(r) => r,
        Err(e) => exit_with_error(format!("failed to execute request: {e}")),
    };
    if let Some(e) = &res.read_err {
        exit_with_error(format!("failed to read response body: {e}"));
    }
    let body = res.text();
    match res.status {
        200 => {
            let msg = serde_json::from_slice::<Value>(&res.body)
                .ok()
                .filter(|_| crate::gojson::check_valid(&res.body).is_ok())
                .filter(|v| v.is_object() || v.is_null())
                .map(|v| str_field(&v, "message"))
                .unwrap_or_default();
            if !msg.is_empty() {
                qovery_log::println_info(format!("✅ {msg}"));
            } else {
                let action_text = if action == "unblock" {
                    "unblocked"
                } else {
                    "blocked"
                };
                qovery_log::println_info(format!(
                    "✅ Organization {organization_id} has been {action_text} successfully"
                ));
            }
        }
        404 => exit_with_error(format!("❌ Organization not found: {organization_id}")),
        401 => exit_with_error("❌ Unauthorized: You don't have permission to perform this action"),
        403 => exit_with_error("❌ Forbidden: You don't have permission to perform this action"),
        400 => exit_with_error(format!("❌ Bad request: {body}")),
        _ => exit_with_error(format!(
            "❌ Request failed with status {}: {body}",
            res.status_line()
        )),
    }
}

// ---------------------------------------------------------------- organization plan

/// Kept in sync with the Plan enum of q-core.
const ADMIN_ORGANIZATION_PLANS: &[&str] = &[
    "FREE",
    "PROFESSIONAL",
    "TEAM",
    "BUSINESS",
    "ENTERPRISE",
    "USER_2025",
    "TEAM_2025",
    "BUSINESS_2025",
    "ENTERPRISE_2025",
    "STARTER_2026",
    "GROWTH_2026",
    "SCALE_2026",
];

pub fn get_organization_plan() -> Command {
    Command::new("get-organization-plan")
        .short("Get the plan of an organization stored in the Qovery database")
        .long(
            "Get the plan of an organization as stored in the Qovery database, with its Chargebee subscription ID so both can be compared.

Example:
  qovery admin get-organization-plan --organization-id \"xxx-xxx-xxx\"
",
        )
        .flag(Flag::string("organization-id", "Organization ID (required)").short('o').required())
        .run(run_get_organization_plan)
}

pub fn update_organization_plan() -> Command {
    let long = format!(
        "Force the plan of an organization in the Qovery database, without touching Chargebee.

The Chargebee subscription is left untouched: the organization keeps being invoiced on its current item price until someone updates Chargebee by hand. Use it for organizations billed out-of-band, and fix Chargebee yourself otherwise.

Accepted plans: {}

Example:
  qovery admin update-organization-plan --organization-id \"xxx-xxx-xxx\" --plan \"ENTERPRISE_2025\"
",
        ADMIN_ORGANIZATION_PLANS.join(", ")
    );
    Command::new("update-organization-plan")
        .short("Force the plan of an organization without updating Chargebee")
        .long(&long)
        .flag(
            Flag::string("organization-id", "Organization ID (required)")
                .short('o')
                .required(),
        )
        .flag(
            Flag::string("plan", "Plan to force (required)")
                .short('p')
                .required(),
        )
        .run(run_update_organization_plan)
}

/// `json.Unmarshal` into a struct: only syntax errors are reproduced.
fn parse_response(body: &[u8]) -> Value {
    if let Err(e) = crate::gojson::check_valid(body) {
        exit_with_error(format!("failed to parse response: {e}"));
    }
    serde_json::from_slice(body).unwrap_or(Value::Null)
}

fn run_get_organization_plan(ctx: &Ctx) {
    let body =
        admin_organization_plan_request_or_exit(&ctx.string("organization-id"), Method::GET, None);
    let response = parse_response(&body);
    let billing = match response.get("billing_external_id").and_then(|b| b.as_str()) {
        Some(b) if !b.is_empty() => b.to_string(),
        _ => "<none>".to_string(),
    };
    qovery_log::print_table(
        &[
            "Organization ID",
            "Name",
            "Plan",
            "Chargebee Subscription ID",
        ],
        &[vec![
            str_field(&response, "organization_id"),
            str_field(&response, "name"),
            str_field(&response, "plan"),
            billing,
        ]],
    );
}

fn run_update_organization_plan(ctx: &Ctx) {
    let plan = match parse_admin_organization_plan(&ctx.string("plan")) {
        Ok(p) => p,
        Err(e) => exit_with_error(e),
    };
    let request_body = go_json(&json!({ "plan": plan }));
    let body = admin_organization_plan_request_or_exit(
        &ctx.string("organization-id"),
        Method::PUT,
        Some(request_body),
    );
    let response = parse_response(&body);
    qovery_log::print_table(
        &["Organization ID", "Previous Plan", "Plan"],
        &[vec![
            str_field(&response, "organization_id"),
            str_field(&response, "previous_plan"),
            str_field(&response, "plan"),
        ]],
    );
    qovery_log::println_info(str_field(&response, "warning"));
}

fn parse_admin_organization_plan(plan: &str) -> Result<String, String> {
    let normalized = plan.trim().to_uppercase();
    if !ADMIN_ORGANIZATION_PLANS.contains(&normalized.as_str()) {
        return Err(format!(
            "invalid plan {plan}, must be one of: {}",
            ADMIN_ORGANIZATION_PLANS.join(", ")
        ));
    }
    Ok(normalized)
}

fn admin_organization_plan_request_or_exit(
    organization_id: &str,
    method: Method,
    request_body: Option<String>,
) -> Vec<u8> {
    if organization_id.is_empty() {
        exit_with_error("organization ID is required");
    }
    let auth = check_error(authorization());
    match request_admin_organization_plan(
        method,
        &admin_url(),
        &auth,
        organization_id,
        request_body,
    ) {
        Ok(b) => b,
        Err(e) => exit_with_error(e),
    }
}

fn organization_plan_endpoint(admin_url: &str, organization_id: &str) -> String {
    format!(
        "{}/organization/{}/plan",
        admin_url.trim_end_matches('/'),
        path_escape(organization_id)
    )
}

fn request_admin_organization_plan(
    method: Method,
    admin_url: &str,
    authorization: &str,
    organization_id: &str,
    request_body: Option<String>,
) -> Result<Vec<u8>, String> {
    let endpoint = organization_plan_endpoint(admin_url, organization_id);
    let mut req = Req::new(method, &endpoint, authorization).timeout(Duration::from_secs(60));
    if let Some(b) = request_body {
        req = req.body(b);
    }
    let res = req.send()?;
    if let Some(e) = res.read_err {
        return Err(e);
    }
    if res.status >= 400 {
        return Err(format!(
            "request failed (status={}): {}",
            res.status,
            res.text()
        ));
    }
    Ok(res.body)
}

// ---------------------------------------------------------------- transfer-ownership

pub fn transfer_ownership() -> Command {
    Command::new("transfer-ownership")
        .short("Transfer organization ownership to another user")
        .long(
            "Transfer organization ownership to another user by providing the organization ID and either the new owner's user ID or email.

Example:
  qovery admin transfer-ownership --organization-id \"xxx-xxx-xxx\" --user-id \"auth0|xxx\"
  qovery admin transfer-ownership --organization-id \"xxx-xxx-xxx\" --email \"user@example.com\"
  qovery admin transfer-ownership --organization-id \"xxx-xxx-xxx\" --email \"user@example.com\" --provider \"github\"
",
        )
        .flag(Flag::string("organization-id", "Organization ID (required)").short('o').required())
        .flag(Flag::string("user-id", "New owner user ID").short('u'))
        .flag(Flag::string("email", "New owner email address").short('e'))
        .flag(
            Flag::string("provider", "Auth provider (auth0, github, gitlab, google, etc.) - required if multiple users have the same email")
                .short('p'),
        )
        .run(run_transfer_ownership)
}

/// The provider part of a `provider|id` user id.
fn provider_of(user_id: &str) -> Option<&str> {
    let parts: Vec<&str> = user_id.split('|').collect();
    if parts.len() >= 2 {
        Some(parts[0])
    } else {
        None
    }
}

fn run_transfer_ownership(ctx: &Ctx) {
    let organization_id = ctx.string("organization-id");
    let new_owner_user_id = ctx.string("user-id");
    let new_owner_email = ctx.string("email");
    let auth_provider = ctx.string("provider");
    if organization_id.is_empty() {
        exit_with_error("organization ID is required");
    }
    if new_owner_user_id.is_empty() && new_owner_email.is_empty() {
        exit_with_error("either --user-id or --email must be provided");
    }
    if !new_owner_user_id.is_empty() && !new_owner_email.is_empty() {
        exit_with_error("only one of --user-id or --email should be provided, not both");
    }
    let (tt, token) = check_error(qovery_core::auth::get_access_token(false));
    let client = qovery_core::client::client_for(tt, &token);

    let mut target_user_id = new_owner_user_id.clone();
    if !new_owner_email.is_empty() {
        qovery_log::println(format!("🔍 Looking up user with email: {new_owner_email}"));
        let members: Vec<Value> =
            match client.list(&format!("/organization/{organization_id}/member")) {
                Ok(m) => m,
                Err(e) => {
                    qovery_log::println_error(format!("failed to list organization members: {e}"));
                    if let Some(s) = e.status {
                        qovery_log::println_error(format!(
                            "response status: {}",
                            qovery_api::status_line(s)
                        ));
                    }
                    exit(1);
                }
            };
        let matching: Vec<&Value> = members
            .iter()
            .filter(|m| str_field(m, "email") == new_owner_email)
            .collect();
        if matching.is_empty() {
            exit_with_error(format!(
                "no member found with email '{new_owner_email}' in organization {organization_id}"
            ));
        }
        if matching.len() > 1 {
            if auth_provider.is_empty() {
                let providers: Vec<String> = matching
                    .iter()
                    .filter_map(|m| provider_of(&str_field(m, "id")).map(|p| p.to_string()))
                    .collect();
                qovery_log::println_error(format!(
                    "multiple users found with email '{new_owner_email}'. Please specify --provider flag"
                ));
                exit_with_error(format!("available providers: [{}]", providers.join(" ")));
            }
            let found = matching.iter().find(|m| {
                provider_of(&str_field(m, "id"))
                    .is_some_and(|p| p.to_lowercase() == auth_provider.to_lowercase())
            });
            let Some(found) = found else {
                exit_with_error(format!(
                    "no member found with email '{new_owner_email}' and provider '{auth_provider}'"
                ));
            };
            target_user_id = str_field(found, "id");
            qovery_log::println(format!(
                "✅ Found user: {} (Provider: {auth_provider}, ID: {target_user_id})",
                str_field(found, "email")
            ));
        } else {
            target_user_id = str_field(matching[0], "id");
            let provider = provider_of(&target_user_id).unwrap_or("unknown");
            qovery_log::println(format!(
                "✅ Found user: {} (Provider: {provider}, ID: {target_user_id})",
                str_field(matching[0], "email")
            ));
        }
    }

    qovery_log::println(format!(
        "🔄 Transferring ownership to user {target_user_id}..."
    ));
    let body = json!({ "user_id": target_user_id });
    if let Err(e) = client.execute(
        ApiMethod::POST,
        &format!("/organization/{organization_id}/transferOwnership"),
        &[],
        Some(&body),
    ) {
        qovery_log::println_error(format!("failed to transfer ownership: {e}"));
        if let Some(s) = e.status {
            qovery_log::println_error(format!("response status: {}", qovery_api::status_line(s)));
        }
        exit(1);
    }
    if !new_owner_email.is_empty() {
        qovery_log::println(format!(
            "✅ Successfully transferred ownership of organization {organization_id} to {new_owner_email}"
        ));
    } else {
        qovery_log::println(format!(
            "✅ Successfully transferred ownership of organization {organization_id} to user {target_user_id}"
        ));
    }
}

// ---------------------------------------------------------------- update-billing-external-id

pub fn update_billing_external_id() -> Command {
    Command::new("update-billing-external-id")
        .short("Update the billing external ID (Chargebee subscription ID) of an organization")
        .long(
            "Update the billing external ID of an organization. The billing external ID is the Chargebee subscription ID.

Example:
  qovery admin update-billing-external-id --organization-id \"xxx-xxx-xxx\" --billing-external-id \"AzyXZ8T0EI4jB4AZf\"
",
        )
        .flag(Flag::string("organization-id", "Organization ID (required)").short('o').required())
        .flag(Flag::string("billing-external-id", "Chargebee subscription ID (required)").short('b').required())
        .run(run_update_billing_external_id)
}

fn run_update_billing_external_id(ctx: &Ctx) {
    let organization_id = ctx.string("organization-id");
    let billing_external_id = ctx.string("billing-external-id");
    if organization_id.is_empty() {
        exit_with_error("organization ID is required");
    }
    if billing_external_id.is_empty() {
        exit_with_error("billing external ID is required");
    }
    let auth = check_error(authorization());
    let body = go_json(&json!({ "billing_external_id": billing_external_id }));
    let url = format!(
        "{}/organization/{organization_id}/billingExternalId",
        admin_url()
    );
    let res = match Req::new(Method::PUT, &url, &auth).body(body).send() {
        Ok(r) => r,
        Err(e) => exit_with_error(format!("failed to execute request: {e}")),
    };
    if res.status >= 400 {
        exit_with_error(format!(
            "request failed (status={}): {}",
            res.status,
            res.text()
        ));
    }
    qovery_log::println(format!(
        "✅ Successfully updated billing external ID for organization {organization_id}"
    ));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_plan_normalizes_accepted_values() {
        for plan in ["ENTERPRISE_2025", "enterprise_2025", "  Enterprise_2025  "] {
            assert_eq!(
                parse_admin_organization_plan(plan).unwrap(),
                "ENTERPRISE_2025"
            );
        }
    }

    #[test]
    fn parse_plan_rejects_unknown_values() {
        // TEAM_YEARLY exists in the public API client but not in the Plan enum of Core
        for plan in ["", "   ", "entreprise_2025", "TEAM_YEARLY"] {
            let err = parse_admin_organization_plan(plan).unwrap_err();
            assert!(err.contains("ENTERPRISE_2025"), "{err}");
        }
    }

    #[test]
    fn plan_endpoint() {
        assert_eq!(
            organization_plan_endpoint("http://h//", "org-1"),
            "http://h/organization/org-1/plan"
        );
        assert_eq!(
            organization_plan_endpoint("http://h", "a/b c"),
            "http://h/organization/a%2Fb%20c/plan"
        );
    }

    #[test]
    fn organization_ids() {
        let ids = parse_organization_ids(&["a, b".to_string(), ",".to_string(), " c ".to_string()]);
        assert_eq!(ids, vec!["a", "b", "c"]);
    }
}
