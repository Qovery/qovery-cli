//! Minimal port of `github.com/xlab/treeprint` v1.2.0 (string values, no meta values), used by
//! `environment deployment explain`.

const EDGE_LINK: &str = "│";
const EDGE_MID: &str = "├──";
const EDGE_END: &str = "└──";
const INDENT_SIZE: usize = 3;

/// A tree node (`treeprint.Node`).
#[derive(Debug, Clone, Default)]
pub struct Node {
    pub value: String,
    pub nodes: Vec<Node>,
}

impl Node {
    /// `treeprint.New()`: a root node whose value is `.`.
    pub fn new() -> Self {
        Node {
            value: ".".to_string(),
            nodes: Vec::new(),
        }
    }

    /// `AddBranch(v)`: add a child and return it.
    pub fn add_branch(&mut self, v: impl Into<String>) -> &mut Node {
        self.nodes.push(Node {
            value: v.into(),
            nodes: Vec::new(),
        });
        self.nodes.last_mut().expect("just pushed")
    }

    /// `AddNode(v)`: add a leaf child (returns `self`, like Go).
    pub fn add_node(&mut self, v: impl Into<String>) -> &mut Node {
        self.nodes.push(Node {
            value: v.into(),
            nodes: Vec::new(),
        });
        self
    }

    /// `String()` of a root node.
    pub fn string(&self) -> String {
        let mut buf = String::new();
        buf.push_str(&self.value);
        buf.push('\n');
        if !self.nodes.is_empty() {
            let mut lastness = Vec::new();
            print_nodes(&mut buf, 0, &mut lastness, &self.nodes);
        }
        buf
    }
}

/// `printNodes`; `lastness[i]` tells whether the ancestor at level `i` is the last of its siblings
/// (Go's `levelsEnded`).
fn print_nodes(buf: &mut String, level: usize, lastness: &mut Vec<bool>, nodes: &[Node]) {
    for (i, node) in nodes.iter().enumerate() {
        let last = i == nodes.len() - 1;
        let edge = if last { EDGE_END } else { EDGE_MID };
        lastness.push(last);
        print_values(buf, level, lastness, edge, node);
        if !node.nodes.is_empty() {
            print_nodes(buf, level + 1, lastness, &node.nodes);
        }
        lastness.pop();
    }
}

/// `printValues`; `lastness` includes the node itself at index `level`.
fn print_values(buf: &mut String, level: usize, lastness: &[bool], edge: &str, node: &Node) {
    for ended in lastness.iter().take(level) {
        if *ended {
            buf.push_str(&" ".repeat(INDENT_SIZE + 1));
        } else {
            buf.push_str(EDGE_LINK);
            buf.push_str(&" ".repeat(INDENT_SIZE));
        }
    }
    buf.push_str(&format!("{edge} {}\n", render_value(level, lastness, node)));
}

/// `renderValue`: continuation lines of a multi-line value are indented with the tree links.
fn render_value(level: usize, lastness: &[bool], node: &Node) -> String {
    let lines: Vec<&str> = node.value.split('\n').collect();
    if lines.len() < 2 {
        return node.value.clone();
    }
    let pad: String = lastness
        .iter()
        .take(level + 1)
        .map(|last| {
            if *last {
                " ".repeat(INDENT_SIZE + 1)
            } else {
                format!("{EDGE_LINK}{}", " ".repeat(INDENT_SIZE))
            }
        })
        .collect();
    let mut out = lines[0].to_string();
    for l in &lines[1..] {
        out.push('\n');
        out.push_str(&pad);
        out.push_str(l);
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn renders_like_treeprint() {
        let mut tree = Node::new();
        let env = tree.add_branch("Environment: e");
        let s1 = env.add_branch("Stage 1: a");
        let svc = s1.add_branch("svc");
        svc.add_branch("Step 1: build")
            .add_node("line1")
            .add_node("multi\nline");
        svc.add_branch("Step 2: deploy");
        env.add_branch("Stage 2: b");
        let want = "\
.
└── Environment: e
    ├── Stage 1: a
    │   └── svc
    │       ├── Step 1: build
    │       │   ├── line1
    │       │   └── multi
    │       │       line
    │       └── Step 2: deploy
    └── Stage 2: b
";
        assert_eq!(tree.string(), want);
    }

    #[test]
    fn multiline_value_of_non_last_node() {
        let mut tree = Node::new();
        tree.add_node("a\nb").add_node("c");
        assert_eq!(tree.string(), ".\n├── a\n│   b\n└── c\n");
    }
}
