//! Query string of the websocket gateway requests: port of `query.Values(req)` (appscode/go-querystring)
//! followed by `url.Values.Encode()` and the `%5B([0-9]+)%5D=` -> `[${1}]=` rewrite of `pkg/shell.go`.

use std::collections::BTreeMap;

/// `url.Values`: values of a key keep their insertion order, keys are sorted on encoding.
#[derive(Debug, Default, Clone)]
pub struct Query {
    values: BTreeMap<String, Vec<String>>,
}

impl Query {
    pub fn new() -> Self {
        Self::default()
    }

    /// `values.Add(key, value)`.
    pub fn add(&mut self, key: &str, value: impl Into<String>) -> &mut Self {
        self.values
            .entry(key.to_string())
            .or_default()
            .push(value.into());
        self
    }

    /// A field tagged `omitempty`: skipped when empty.
    pub fn add_omitempty(&mut self, key: &str, value: &str) -> &mut Self {
        if !value.is_empty() {
            self.add(key, value);
        }
        self
    }

    /// A slice field: `key[0]=a&key[1]=b`, or `key[]=` when empty (appscode `reflectArray`).
    pub fn add_slice(&mut self, key: &str, values: &[String]) -> &mut Self {
        for (i, v) in values.iter().enumerate() {
            self.add(&format!("{key}[{i}]"), v.as_str());
        }
        if values.is_empty() {
            self.add(&format!("{key}[]"), "");
        }
        self
    }

    /// `url.Values.Encode()` + the bracket rewrite of the Go code.
    pub fn encode(&self) -> String {
        let mut parts = Vec::new();
        for (k, vs) in &self.values {
            let key = query_escape(k);
            for v in vs {
                parts.push(format!("{key}={}", query_escape(v)));
            }
        }
        let encoded = parts.join("&");
        let re = regex::Regex::new("%5B([0-9]+)%5D=").expect("valid regex");
        re.replace_all(&encoded, "[${1}]=").into_owned()
    }
}

pub use qovery_api::query_escape;

/// `url.Parse(utils.WebsocketUrl() + path)` with `RawQuery` replaced by `query`, then `String()`.
pub fn websocket_request_url(path: &str, query: &Query) -> String {
    let raw = format!("{}{path}", qovery_core::client::websocket_url());
    let (without_fragment, fragment) = match raw.split_once('#') {
        Some((a, f)) => (a.to_string(), Some(f.to_string())),
        None => (raw, None),
    };
    let base = without_fragment
        .split_once('?')
        .map(|(a, _)| a.to_string())
        .unwrap_or(without_fragment);
    let mut url = format!("{base}?{}", query.encode());
    if let Some(f) = fragment {
        url.push('#');
        url.push_str(&f);
    }
    url
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encode_sorted_and_brackets() {
        let mut q = Query::new();
        q.add("service", "s 1").add("cluster", "c/1");
        q.add_slice("command", &["sh".into(), "-c".into(), "echo a&b".into()]);
        q.add("tty_width", "0");
        q.add_omitempty("pod_name", "");
        assert_eq!(
            q.encode(),
            "cluster=c%2F1&command[0]=sh&command[1]=-c&command[2]=echo+a%26b&service=s+1&tty_width=0"
        );
    }

    #[test]
    fn empty_slice_and_uppercase_keys() {
        let mut q = Query::new();
        q.add("port", "80").add("LocalPort", "8080");
        q.add_slice("command", &[]);
        assert_eq!(q.encode(), "LocalPort=8080&command%5B%5D=&port=80");
    }

    #[test]
    fn many_items_sort_as_strings() {
        let mut q = Query::new();
        let items: Vec<String> = (0..11).map(|i| i.to_string()).collect();
        q.add_slice("c", &items);
        assert!(q.encode().starts_with("c[0]=0&c[10]=10&c[1]=1&c[2]=2"));
    }
}
