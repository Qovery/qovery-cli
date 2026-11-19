//! Small ports of Go standard library helpers used by the admin commands
//! (`net/url` escaping, `path.Join`).

pub use qovery_api::gourl::{encode_query, path_escape, query_escape};

/// Go `path.Clean`.
pub fn path_clean(p: &str) -> String {
    if p.is_empty() {
        return ".".to_string();
    }
    let rooted = p.starts_with('/');
    let mut parts: Vec<&str> = Vec::new();
    for seg in p.split('/') {
        match seg {
            "" | "." => {}
            ".." => {
                if parts.last().is_some_and(|l| *l != "..") {
                    parts.pop();
                } else if !rooted {
                    parts.push("..");
                }
            }
            s => parts.push(s),
        }
    }
    let joined = parts.join("/");
    match (rooted, joined.is_empty()) {
        (true, _) => format!("/{joined}"),
        (false, true) => ".".to_string(),
        (false, false) => joined,
    }
}

/// Go `path.Join`.
pub fn path_join(elems: &[&str]) -> String {
    let non_empty: Vec<&str> = elems.iter().copied().filter(|e| !e.is_empty()).collect();
    if non_empty.is_empty() {
        return String::new();
    }
    path_clean(&non_empty.join("/"))
}

/// Go: `u, _ := url.Parse(base); u.Path = path.Join(u.Path, elems...); u.String()`.
pub fn url_with_joined_path(base: &str, elems: &[&str]) -> String {
    let (prefix, rest) = match base.find("://") {
        Some(i) => {
            let after = &base[i + 3..];
            let host_end = after.find(['/', '?', '#']).unwrap_or(after.len());
            (base[..i + 3 + host_end].to_string(), &after[host_end..])
        }
        None => (String::new(), base),
    };
    let (rest, fragment) = match rest.split_once('#') {
        Some((r, f)) => (r, Some(f)),
        None => (rest, None),
    };
    let (path, query) = match rest.split_once('?') {
        Some((p, q)) => (p, Some(q)),
        None => (rest, None),
    };
    let decoded = percent_decode(path);
    let mut all: Vec<&str> = vec![decoded.as_str()];
    all.extend_from_slice(elems);
    let joined = path_join(&all);
    let mut out = prefix.clone();
    let has_host = !prefix.is_empty() && !prefix.ends_with("://");
    if !joined.is_empty() && !joined.starts_with('/') && has_host {
        out.push('/');
    }
    out.push_str(&qovery_api::gourl::escape_url_path(&joined));
    if let Some(q) = query
        && !q.is_empty()
    {
        out.push('?');
        out.push_str(q);
    }
    if let Some(f) = fragment {
        out.push('#');
        out.push_str(f);
    }
    out
}

fn percent_decode(s: &str) -> String {
    let b = s.as_bytes();
    let mut out = Vec::with_capacity(b.len());
    let mut i = 0;
    while i < b.len() {
        if b[i] == b'%'
            && i + 2 < b.len()
            && let Ok(v) =
                u8::from_str_radix(std::str::from_utf8(&b[i + 1..i + 3]).unwrap_or("zz"), 16)
        {
            out.push(v);
            i += 3;
            continue;
        }
        out.push(b[i]);
        i += 1;
    }
    String::from_utf8_lossy(&out).to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn escapes() {
        assert_eq!(
            path_escape("a b/c?d;e,f$&+:=@"),
            "a%20b%2Fc%3Fd%3Be%2Cf$&+:=@"
        );
        assert_eq!(query_escape("a b/c?&="), "a+b%2Fc%3F%26%3D");
    }

    #[test]
    fn joins() {
        assert_eq!(
            path_join(&["", "organization", "x", "secret"]),
            "organization/x/secret"
        );
        assert_eq!(
            path_join(&["/api/", "organization", "../x", "secret"]),
            "/api/x/secret"
        );
        assert_eq!(
            url_with_joined_path("http://h:1", &["organization", "o 1", "secret"]),
            "http://h:1/organization/o%201/secret"
        );
        assert_eq!(
            url_with_joined_path("http://h:1/admin/", &["organization", "o", "secret"]),
            "http://h:1/admin/organization/o/secret"
        );
    }
}
