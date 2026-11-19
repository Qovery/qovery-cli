//! Port of `utils/commit_resolver.go`: resolution of the `latest` git commit keyword.

use crate::util::{decode_go_response, parse_api_time, path_escape};
use qovery_api::models::null_as_default;
use qovery_api::{Client, Method};
use qovery_log::pterm;
use serde::Deserialize;

/// `utils.LatestCommitKeyword`.
pub const LATEST_COMMIT_KEYWORD: &str = "latest";

/// Port of `utils.IsLatestCommitKeyword(value)`: `strings.EqualFold(strings.TrimSpace(value), "latest")`.
pub fn is_latest_commit_keyword(value: &str) -> bool {
    let v = value.trim();
    let mut chars = v.chars();
    for t in LATEST_COMMIT_KEYWORD.chars() {
        match chars.next() {
            // Unicode simple folding of the keyword letters (U+017F LATIN SMALL LETTER LONG S folds to `s`)
            Some(c) if c.to_ascii_lowercase() == t || (t == 's' && c == '\u{17f}') => {}
            _ => return false,
        }
    }
    chars.next().is_none()
}

/// `qovery.Commit` (fields used by the CLI); Go decodes `null` into the zero value.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Commit {
    #[serde(default, deserialize_with = "null_as_default")]
    pub created_at: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub git_commit_id: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub tag: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub message: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub author_name: String,
}

/// `qovery.CommitResponseList` (`results` missing or `null`: no commit).
#[derive(Debug, Clone, Default, Deserialize)]
struct CommitResponseList {
    #[serde(default, deserialize_with = "null_as_default")]
    results: Vec<Commit>,
}

/// Port of `newestCommit(commits)`: the commit with the greatest creation date (the first one on a tie).
pub fn newest_commit(commits: &[Commit]) -> Result<&Commit, String> {
    let Some(mut newest) = commits.first() else {
        return Err("no commit found".to_string());
    };
    for c in commits {
        if parse_api_time(&c.created_at) > parse_api_time(&newest.created_at) {
            newest = c;
        }
    }
    Ok(newest)
}

/// Port of `firstLine(message)`: the trimmed first line.
pub fn first_line(message: &str) -> &str {
    message.split('\n').next().unwrap_or("").trim()
}

fn resolve(
    client: &Client,
    path: String,
    kind: &str,
    service_name: &str,
) -> Result<String, String> {
    let list: Option<CommitResponseList> = client
        .execute(Method::GET, &path, &[], None)
        .map_err(|e| e.to_string())
        .and_then(|res| decode_go_response(&res))
        .map_err(|e| format!("cannot list commits of {kind} {service_name}: {e}"))?;
    resolve_latest_commit(list.map(|l| l.results).as_deref(), service_name)
}

/// Port of `utils.ResolveLatestApplicationCommit(client, applicationId, serviceName)` (`GET /application/{id}/commit`).
pub fn resolve_latest_application_commit(
    client: &Client,
    application_id: &str,
    service_name: &str,
) -> Result<String, String> {
    resolve(
        client,
        format!("/application/{}/commit", path_escape(application_id)),
        "application",
        service_name,
    )
}

/// Port of `utils.ResolveLatestJobCommit(client, jobId, serviceName)` (`GET /job/{id}/commit`).
pub fn resolve_latest_job_commit(
    client: &Client,
    job_id: &str,
    service_name: &str,
) -> Result<String, String> {
    resolve(
        client,
        format!("/job/{}/commit", path_escape(job_id)),
        "job",
        service_name,
    )
}

/// Port of `utils.ResolveLatestHelmChartCommit(client, helmId, serviceName)` (`GET /helm/{id}/commit`).
pub fn resolve_latest_helm_chart_commit(
    client: &Client,
    helm_id: &str,
    service_name: &str,
) -> Result<String, String> {
    resolve(
        client,
        format!("/helm/{}/commit", path_escape(helm_id)),
        "helm",
        service_name,
    )
}

/// Port of `utils.ResolveLatestTerraformCommit(client, terraformId, serviceName)` (`GET /terraform/{id}/commit`).
pub fn resolve_latest_terraform_commit(
    client: &Client,
    terraform_id: &str,
    service_name: &str,
) -> Result<String, String> {
    resolve(
        client,
        format!("/terraform/{}/commit", path_escape(terraform_id)),
        "terraform",
        service_name,
    )
}

/// Port of `resolveLatestCommit(commits, serviceName)`: prints
/// `Resolved latest commit for <name>: <commit> (<first line of the message>)` and returns the commit id.
/// `commits` is `None` when the API returned no list (Go nil).
pub fn resolve_latest_commit(
    commits: Option<&[Commit]>,
    service_name: &str,
) -> Result<String, String> {
    let Some(commits) = commits else {
        return Err(format!("no commit found for {service_name}"));
    };
    let commit = newest_commit(commits).map_err(|e| format!("{e} for {service_name}"))?;
    qovery_log::println(format!(
        "Resolved latest commit for {}: {} ({})",
        pterm::FG_BLUE.sprint(service_name),
        pterm::FG_BLUE.sprint(&commit.git_commit_id),
        first_line(&commit.message)
    ));
    Ok(commit.git_commit_id.clone())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn is_latest_commit_keyword_cases() {
        let tests = [
            ("latest", true),
            ("LATEST", true),
            ("Latest", true),
            (" latest ", true),
            ("\tlatest\n", true),
            ("", false),
            ("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", false),
            ("latest-branch", false),
            ("v1.0.0", false),
        ];
        for (value, expected) in tests {
            assert_eq!(
                is_latest_commit_keyword(value),
                expected,
                "IsLatestCommitKeyword({value:?})"
            );
        }
    }

    fn commit_at(id: &str, created_at: &str) -> Commit {
        Commit {
            git_commit_id: id.to_string(),
            created_at: created_at.to_string(),
            message: format!("commit {id}"),
            ..Default::default()
        }
    }

    #[test]
    fn newest_commit_cases() {
        let oldest = commit_at("oldest", "2024-01-01T00:00:00Z");
        let middle = commit_at("middle", "2024-06-01T00:00:00Z");
        let newest = commit_at("newest", "2024-12-01T00:00:00Z");
        let tests = [
            (
                "newest first",
                vec![newest.clone(), middle.clone(), oldest.clone()],
                "newest",
            ),
            (
                "oldest first",
                vec![oldest.clone(), middle.clone(), newest.clone()],
                "newest",
            ),
            (
                "unordered",
                vec![middle.clone(), newest.clone(), oldest.clone()],
                "newest",
            ),
            ("single commit", vec![middle.clone()], "middle"),
        ];
        for (name, commits, expected) in tests {
            assert_eq!(
                newest_commit(&commits).unwrap().git_commit_id,
                expected,
                "{name}"
            );
        }
    }

    #[test]
    fn newest_commit_empty_list() {
        assert!(newest_commit(&[]).is_err());
    }

    #[test]
    fn newest_commit_tie_keeps_first() {
        let first = commit_at("first", "2024-06-01T00:00:00Z");
        let second = commit_at("second", "2024-06-01T00:00:00Z");
        assert_eq!(
            newest_commit(&[first, second]).unwrap().git_commit_id,
            "first"
        );
    }

    #[test]
    fn first_line_cases() {
        let tests = [
            ("feat: add thing", "feat: add thing"),
            ("feat: add thing\n\nlong body\nmore body", "feat: add thing"),
            ("  feat: add thing  \nbody", "feat: add thing"),
            ("", ""),
        ];
        for (message, expected) in tests {
            assert_eq!(first_line(message), expected, "firstLine({message:?})");
        }
    }

    #[test]
    fn commit_null_fields_decode_as_zero_values() {
        // Go decodes `null` into the zero value of a required property (untagged commit)
        let list: CommitResponseList = serde_json::from_str(
            r#"{"results": [{"created_at": null, "git_commit_id": "abc", "tag": null, "message": null, "author_name": null}]}"#,
        )
        .unwrap();
        assert_eq!(list.results.len(), 1);
        assert_eq!(list.results[0].git_commit_id, "abc");
        assert_eq!(list.results[0].tag, "");
        let list: CommitResponseList = serde_json::from_str(r#"{"results": null}"#).unwrap();
        assert!(list.results.is_empty());
        // a malformed list is an error, not an empty list
        assert!(serde_json::from_str::<CommitResponseList>(r#"{"results": {"a": 1}}"#).is_err());
    }

    #[test]
    fn resolve_errors() {
        assert_eq!(
            resolve_latest_commit(None, "app").unwrap_err(),
            "no commit found for app"
        );
        assert_eq!(
            resolve_latest_commit(Some(&[]), "app").unwrap_err(),
            "no commit found for app"
        );
        let commits = [commit_at("abc", "2024-06-01T00:00:00Z")];
        assert_eq!(resolve_latest_commit(Some(&commits), "app").unwrap(), "abc");
    }
}
