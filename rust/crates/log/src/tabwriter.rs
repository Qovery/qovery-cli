//! Port of Go's `text/tabwriter` for `tabwriter.NewWriter(w, minwidth, tabwidth, padding, ' ', 0)`
//! (space padding, no flags). Text is buffered with [`TabWriter::write`] and formatted by [`TabWriter::flush`].
//! Cells are terminated by `\t`; a column is aligned over a block of consecutive lines having that column, the
//! text after the last tab of a line is not part of a column (Go's algorithm, nested column blocks included).

pub struct TabWriter {
    minwidth: usize,
    padding: usize,
    /// complete lines (cells) not yet flushed
    lines: Vec<Vec<String>>,
    /// current (incomplete) line
    current: Vec<String>,
    cell: String,
    out: String,
}

impl TabWriter {
    pub fn new(minwidth: usize, padding: usize) -> Self {
        TabWriter {
            minwidth,
            padding,
            lines: Vec::new(),
            current: Vec::new(),
            cell: String::new(),
            out: String::new(),
        }
    }

    pub fn write(&mut self, text: &str) {
        for ch in text.chars() {
            match ch {
                '\t' | '\x0b' => {
                    self.current.push(std::mem::take(&mut self.cell));
                }
                '\n' | '\x0c' => {
                    self.current.push(std::mem::take(&mut self.cell));
                    let ncells = self.current.len();
                    self.lines.push(std::mem::take(&mut self.current));
                    if ch == '\x0c' || ncells == 1 {
                        // a line without tabs ends all column blocks: flush
                        self.format_buffered(true);
                    }
                }
                c => self.cell.push(c),
            }
        }
    }

    /// Format the buffered lines and return everything written so far.
    pub fn flush(mut self) -> String {
        // the trailing partial line
        if !self.cell.is_empty() || !self.current.is_empty() {
            self.current.push(std::mem::take(&mut self.cell));
            let partial = std::mem::take(&mut self.current);
            self.lines.push(partial);
            self.format_buffered(false);
        } else {
            self.format_buffered(true);
        }
        self.out
    }

    /// `last_terminated`: whether every buffered line ends with a newline.
    fn format_buffered(&mut self, last_terminated: bool) {
        let lines = std::mem::take(&mut self.lines);
        let mut widths = Vec::new();
        let mut out = String::new();
        format(
            self.minwidth,
            self.padding,
            &lines,
            &mut widths,
            0,
            lines.len(),
            last_terminated,
            &mut out,
        );
        self.out.push_str(&out);
    }
}

fn width(s: &str) -> usize {
    s.chars().count()
}

#[allow(clippy::too_many_arguments)]
fn format(
    minwidth: usize,
    padding: usize,
    lines: &[Vec<String>],
    widths: &mut Vec<usize>,
    line0: usize,
    line1: usize,
    last_terminated: bool,
    out: &mut String,
) {
    let column = widths.len();
    let mut line0 = line0;
    let mut this = line0;
    while this < line1 {
        let line = &lines[this];
        if column + 1 >= line.len() {
            this += 1;
            continue;
        }
        write_lines(lines, widths, line0, this, last_terminated, out);
        line0 = this;
        let mut w = minwidth;
        while this < line1 {
            let line = &lines[this];
            if column + 1 >= line.len() {
                break;
            }
            let cw = width(&line[column]) + padding;
            if cw > w {
                w = cw;
            }
            this += 1;
        }
        widths.push(w);
        format(
            minwidth,
            padding,
            lines,
            widths,
            line0,
            this,
            last_terminated,
            out,
        );
        widths.pop();
        line0 = this;
    }
    write_lines(lines, widths, line0, line1, last_terminated, out);
}

fn write_lines(
    lines: &[Vec<String>],
    widths: &[usize],
    line0: usize,
    line1: usize,
    last_terminated: bool,
    out: &mut String,
) {
    for (i, line) in lines.iter().enumerate().take(line1).skip(line0) {
        for (j, c) in line.iter().enumerate() {
            out.push_str(c);
            if j < widths.len() {
                let pad = widths[j].saturating_sub(width(c));
                out.extend(std::iter::repeat_n(' ', pad));
            }
        }
        if i + 1 < lines.len() || last_terminated {
            out.push('\n');
        }
    }
}

/// `NewWriter(w, minwidth, _, padding, ' ', 0)`, `Fprint` of every line (`\n`-terminated, cells
/// `\t`-terminated), then `Flush()`.
pub fn tabwrite<S: AsRef<str>>(lines: &[S], minwidth: usize, padding: usize) -> String {
    let mut w = TabWriter::new(minwidth, padding);
    for l in lines {
        w.write(l.as_ref());
    }
    w.flush()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn columns() {
        let mut w = TabWriter::new(1, 1);
        let f = |a: &str, b: &str, c: &str, d: &str| format!("{a}\t | {b}\t | {c}\t | {d}\n");
        w.write(&f("", "cluster_id", "key_id", "created_at"));
        w.write(&f("1", "c", "longer-key", "now"));
        assert_eq!(
            w.flush(),
            "   | cluster_id  | key_id      | created_at\n1  | c           | longer-key  | now\n"
        );
    }

    #[test]
    fn blocks() {
        let mut w = TabWriter::new(1, 1);
        w.write("\t\n");
        w.write("Field\t | Value\n");
        w.write("index\t | 1\n");
        w.write("jwt payload\t | x\n");
        assert_eq!(
            w.flush(),
            "            \nField        | Value\nindex        | 1\njwt payload  | x\n"
        );
    }
}
