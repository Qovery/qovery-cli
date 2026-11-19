//! Go `net/url` escaping: `url.PathEscape` (used by the generated API client on every path parameter),
//! `url.QueryEscape` and `url.Values.Encode()`.

#[derive(Clone, Copy, PartialEq, Eq)]
enum Mode {
    Path,
    PathSegment,
    QueryComponent,
}

/// Go `net/url.shouldEscape` (for the modes used by the CLI).
fn should_escape(c: u8, mode: Mode) -> bool {
    if c.is_ascii_alphanumeric() {
        return false;
    }
    match c {
        b'-' | b'_' | b'.' | b'~' => false,
        b'$' | b'&' | b'+' | b',' | b'/' | b':' | b';' | b'=' | b'?' | b'@' => match mode {
            // the RFC allows : @ & = + $ in a path segment; Go also escapes / ; , ?
            Mode::Path => c == b'?',
            Mode::PathSegment => c == b'/' || c == b';' || c == b',' || c == b'?',
            Mode::QueryComponent => true,
        },
        _ => true,
    }
}

fn escape(s: &str, mode: Mode) -> String {
    let mut out = String::with_capacity(s.len());
    for &b in s.as_bytes() {
        if b == b' ' && mode == Mode::QueryComponent {
            out.push('+');
        } else if should_escape(b, mode) {
            out.push_str(&format!("%{b:02X}"));
        } else {
            out.push(b as char);
        }
    }
    out
}

/// Go `url.PathEscape(s)`.
pub fn path_escape(s: &str) -> String {
    escape(s, Mode::PathSegment)
}

/// Go `url.escape(s, encodePath)`: the path of `url.URL.String()` / `EscapedPath()` (only `?` among the
/// reserved characters is escaped).
pub fn escape_url_path(s: &str) -> String {
    escape(s, Mode::Path)
}

/// Go `url.QueryEscape(s)`.
pub fn query_escape(s: &str) -> String {
    escape(s, Mode::QueryComponent)
}

/// Go `url.Values.Encode()`: `k=v` pairs (both `url.QueryEscape`d) sorted by key (stable for repeated keys),
/// joined with `&`.
pub fn encode_query<K: AsRef<str>, V: AsRef<str>>(pairs: &[(K, V)]) -> String {
    let mut sorted: Vec<&(K, V)> = pairs.iter().collect();
    sorted.sort_by(|a, b| a.0.as_ref().cmp(b.0.as_ref()));
    sorted
        .iter()
        .map(|(k, v)| format!("{}={}", query_escape(k.as_ref()), query_escape(v.as_ref())))
        .collect::<Vec<_>>()
        .join("&")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn path() {
        assert_eq!(
            path_escape("a b/c;d,e?f$&+:=@~é"),
            "a%20b%2Fc%3Bd%2Ce%3Ff$&+:=@~%C3%A9"
        );
    }

    #[test]
    fn query() {
        assert_eq!(
            query_escape("a b/c;d,e?f$&+:=@~"),
            "a+b%2Fc%3Bd%2Ce%3Ff%24%26%2B%3A%3D%40~"
        );
    }

    #[test]
    fn values_encode() {
        assert_eq!(
            encode_query(&[("b", "2"), ("a", "x y"), ("b", "1")]),
            "a=x+y&b=2&b=1"
        );
        assert_eq!(encode_query(&[("k", String::from("v"))]), "k=v");
    }
}
