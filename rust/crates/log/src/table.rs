//! Port of `pterm.DefaultTable` (left aligned, " | " separator, light cyan header, gray separators).

use crate::pterm::{Style, remove_color, table_header_style, table_separator_style};
use std::io::Write;
use unicode_width::UnicodeWidthStr;

/// `internal.GetStringMaxWidth` of pterm: display width of the longest line, colors removed.
pub fn string_width(s: &str) -> usize {
    remove_color(s)
        .split('\n')
        .map(UnicodeWidthStr::width)
        .max()
        .unwrap_or(0)
}

#[derive(Clone, Debug)]
pub struct Table {
    data: Vec<Vec<String>>,
    has_header: bool,
    separator: String,
    header_style: Style,
    separator_style: Style,
}

impl Table {
    /// `pterm.DefaultTable.WithData(data)`.
    pub fn new(data: Vec<Vec<String>>) -> Self {
        Table {
            data,
            has_header: false,
            separator: " | ".to_string(),
            header_style: table_header_style(),
            separator_style: table_separator_style(),
        }
    }

    /// `.WithHasHeader()`.
    pub fn with_header(mut self, has_header: bool) -> Self {
        self.has_header = has_header;
        self
    }

    /// `.WithSeparator(sep)`.
    pub fn with_separator(mut self, sep: &str) -> Self {
        self.separator = sep.to_string();
        self
    }

    /// `TablePrinter.Srender()`.
    pub fn srender(&self) -> String {
        struct Cell {
            lines: Vec<String>,
        }
        struct Row {
            cells: Vec<Cell>,
            height: usize,
        }
        let mut max_widths: Vec<usize> = Vec::new();
        let mut rows: Vec<Row> = Vec::new();
        for raw in &self.data {
            let mut row = Row {
                cells: Vec::new(),
                height: 0,
            };
            for c in raw {
                let lines: Vec<String> = c.split('\n').map(|s| s.to_string()).collect();
                let width = lines.iter().map(|l| string_width(l)).max().unwrap_or(0);
                row.height = row.height.max(lines.len());
                row.cells.push(Cell { lines });
                let i = row.cells.len() - 1;
                if max_widths.len() <= i {
                    max_widths.push(width);
                } else if width > max_widths[i] {
                    max_widths[i] = width;
                }
            }
            rows.push(row);
        }

        let render_row = |r: &Row| -> String {
            let mut s = String::new();
            for i in 0..r.height {
                for (j, c) in r.cells.iter().enumerate() {
                    let current = c.lines.get(i).map(|s| s.as_str()).unwrap_or("");
                    let padding = max_widths[j].saturating_sub(string_width(current));
                    s.push_str(current);
                    if j < r.cells.len() - 1 {
                        s.push_str(&" ".repeat(padding));
                        s.push_str(&self.separator_style.sprint(&self.separator));
                    } else {
                        s.push_str(&" ".repeat(padding));
                    }
                }
                s.push('\n');
            }
            s
        };

        let mut ret = String::new();
        for (i, r) in rows.iter().enumerate() {
            if i == 0 && self.has_header {
                ret.push_str(&self.header_style.sprint(render_row(r)));
                continue;
            }
            ret.push_str(&render_row(r));
        }
        ret
    }

    /// `TablePrinter.Render()`: prints Srender() followed by a newline on stdout.
    pub fn render(&self) {
        let mut out = std::io::stdout().lock();
        let _ = writeln!(out, "{}", self.srender());
        let _ = out.flush();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn header_table_matches_go() {
        let t = Table::new(vec![
            vec!["Id".into(), "Name".into(), "Status".into()],
            vec!["1234".into(), "my-app".into(), "DEPLOYED".into()],
            vec!["5".into(), "été-ü".into(), "x\ny-two".into()],
        ])
        .with_header(true);
        let expected = "\x1b[96m\x1b[96mId  \x1b[90m\x1b[90m | \x1b[0m\x1b[96m\x1b[0m\x1b[96mName  \x1b[90m\x1b[90m | \x1b[0m\x1b[96m\x1b[0m\x1b[96mStatus  \x1b[0m\n\x1b[96m\x1b[0m\x1b[0m1234\x1b[90m\x1b[90m | \x1b[0m\x1b[0mmy-app\x1b[90m\x1b[90m | \x1b[0m\x1b[0mDEPLOYED\n5   \x1b[90m\x1b[90m | \x1b[0m\x1b[0mété-ü \x1b[90m\x1b[90m | \x1b[0m\x1b[0mx       \n    \x1b[90m\x1b[90m | \x1b[0m\x1b[0m      \x1b[90m\x1b[90m | \x1b[0m\x1b[0my-two   \n";
        assert_eq!(t.srender(), expected);
    }

    #[test]
    fn empty_header_table_matches_go() {
        let t = Table::new(vec![vec!["Id".into(), "Name".into()]]).with_header(true);
        assert_eq!(
            format!("{}\n", t.srender()),
            "\x1b[96m\x1b[96mId\x1b[90m\x1b[90m | \x1b[0m\x1b[96m\x1b[0m\x1b[96mName\x1b[0m\n\x1b[96m\x1b[0m\x1b[0m\n"
        );
    }

    #[test]
    fn plain_table_matches_go() {
        let t = Table::new(vec![
            vec!["Organization".into(), "org".into()],
            vec!["Project".into(), "p".into()],
        ]);
        assert_eq!(
            t.srender(),
            "Organization\x1b[90m\x1b[90m | \x1b[0m\x1b[0morg\nProject     \x1b[90m\x1b[90m | \x1b[0m\x1b[0mp  \n"
        );
    }
}
