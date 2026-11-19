//! `utils.FindByXxxName`: lookups by name, ignoring surrounding Unicode whitespace.

use qovery_api::models::*;

/// `trimName`: strip Unicode whitespace (including U+00A0) at both ends.
pub fn trim_name(s: &str) -> &str {
    s.trim_matches(|c: char| c.is_whitespace() || c == '\u{85}' || c == '\u{a0}')
}

/// Anything with a name.
pub trait Named {
    fn name(&self) -> &str;
}

macro_rules! named {
    ($($t:ty),*) => { $(impl Named for $t { fn name(&self) -> &str { &self.name } })* };
}
named!(
    Organization,
    Project,
    Environment,
    Application,
    Container,
    Job,
    Database,
    Helm,
    Terraform,
    Cluster
);

impl Named for CustomDomain {
    #[allow(
        clippy::misnamed_getters,
        reason = "the name of a custom domain is its domain (`utils.FindByCustomDomainName`)"
    )]
    fn name(&self) -> &str {
        &self.domain
    }
}

impl Named for serde_json::Value {
    fn name(&self) -> &str {
        self.get("name").and_then(|v| v.as_str()).unwrap_or("")
    }
}

/// Generic `FindByXxxName`: first item whose trimmed name equals the trimmed `name`.
pub fn find_by_name<'a, T: Named>(items: &'a [T], name: &str) -> Option<&'a T> {
    let name = trim_name(name);
    items.iter().find(|i| trim_name(i.name()) == name)
}

/// `utils.FindByCustomDomainName`: first domain whose `domain` field equals `name` exactly (Go compares
/// `d.Domain == name`, without trimming).
pub fn find_by_custom_domain_name<'a>(
    items: &'a [CustomDomain],
    name: &str,
) -> Option<&'a CustomDomain> {
    items.iter().find(|d| d.domain == name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn custom_domain_lookup_is_exact() {
        let d = |domain: &str| CustomDomain {
            domain: domain.to_string(),
            ..Default::default()
        };
        let items = [d("www.example.com"), d(" padded.example.com ")];
        assert!(find_by_custom_domain_name(&items, "www.example.com").is_some());
        assert!(find_by_custom_domain_name(&items, " www.example.com ").is_none());
        assert!(find_by_custom_domain_name(&items, "padded.example.com").is_none());
        assert!(find_by_custom_domain_name(&items, " padded.example.com ").is_some());
        // the other lookups keep trimming (`trimName`)
        assert!(find_by_name(&items, " www.example.com ").is_some());
    }
}
