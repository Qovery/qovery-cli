//! promptui (v0.9.0) `Select` when stdin is NOT a terminal: byte-identical rendering and stdin consumption
//! (see [`crate::readline`] for how keys are read from a pipe).
//!
//! Rendering (default templates): cursor hidden, then after every key the whole prompt is redrawn with
//! promptui's `ScreenBuf`: the help line (`Use the arrow keys to navigate: ↓ ↑ → ←  and / toggles search`)
//! or `Search: <term>█` in search mode, the label (`? <no value>: ` without label), the visible page of items
//! (`▸` + underline for the active one, `↑`/`↓` when the page can scroll) or `No results`. On success the
//! prompt is replaced by `✔ <item>` (green check, faint item); on error (`^D`, `^C`) it is erased.
//!
//! Keys: `j`/`↓`/Ctrl-N next, `k`/`↑`/Ctrl-P previous, `l`/`→`/Ctrl-F page down, `h`/`←`/Ctrl-B page up (letters
//! only outside search mode), `/` toggles search mode, other runes are typed in the search term, backspace
//! deletes, Enter (`\r` or `\n`) selects (in search mode `\n` first resets the cursor to the first match), EOF
//! and Ctrl-D give `^D`, Ctrl-C gives `^C`.

use crate::readline::{
    CHAR_BACKSPACE, CHAR_BACKWARD, CHAR_CTRL_H, CHAR_ENTER, CHAR_FORWARD, CHAR_NEXT, CHAR_PREV,
    Cursor, HIDE_CURSOR, KeyOutcome, OpEvent, RawStdin, SHOW_CURSOR, ScreenBuf, Terminal,
    op_read_key, styler,
};
use std::io::{Read, Write};

/// Searcher: `(input, index) -> matches`.
pub type Searcher<'a> = &'a dyn Fn(&str, usize) -> bool;

/// promptui `Select` options supported by the emulation.
pub struct SelectOptions<'a> {
    /// `Label`; `None` renders as `<no value>` (Go nil label).
    pub label: Option<&'a str>,
    pub items: &'a [String],
    /// `Size` (0 means the default 5).
    pub size: usize,
    /// `Searcher` (search mode is disabled without one).
    pub searcher: Option<Searcher<'a>>,
    pub start_in_search_mode: bool,
    pub hide_help: bool,
}

/// The case-insensitive substring searcher used by every `utils.SelectXxx`:
/// `strings.Contains(strings.ToLower(items[index]), strings.ToLower(input))`.
pub fn contains_searcher(items: &[String]) -> impl Fn(&str, usize) -> bool + '_ {
    move |input, index| {
        crate::readline::go_to_lower(&items[index]).contains(&crate::readline::go_to_lower(input))
    }
}

const NOT_FOUND: i64 = -1;

/// promptui `list.List` (indices into the items).
struct List<'a> {
    items: Vec<usize>,
    scope: Vec<usize>,
    cursor: i64,
    size: i64,
    start: i64,
    searcher: Option<Searcher<'a>>,
}

impl<'a> List<'a> {
    fn new(n: usize, size: usize, searcher: Option<Searcher<'a>>) -> Self {
        let items: Vec<usize> = (0..n).collect();
        List {
            scope: items.clone(),
            items,
            cursor: 0,
            size: size as i64,
            start: 0,
            searcher,
        }
    }

    fn len(&self) -> i64 {
        self.scope.len() as i64
    }

    fn prev(&mut self) {
        if self.cursor > 0 {
            self.cursor -= 1;
        }
        if self.start > self.cursor {
            self.start = self.cursor;
        }
    }

    fn next(&mut self) {
        let max = self.len() - 1;
        if self.cursor < max {
            self.cursor += 1;
        }
        if self.start + self.size <= self.cursor {
            self.start = self.cursor - self.size + 1;
        }
    }

    fn search(&mut self, term: &str) {
        let term = term.trim_matches(' ');
        self.cursor = 0;
        self.start = 0;
        let searcher = self.searcher.expect("search without searcher");
        self.scope = self
            .items
            .iter()
            .copied()
            .filter(|&i| searcher(term, i))
            .collect();
    }

    fn cancel_search(&mut self) {
        self.cursor = 0;
        self.start = 0;
        self.scope = self.items.clone();
    }

    fn page_up(&mut self) {
        let start = self.start - self.size;
        self.start = start.max(0);
        let cursor = self.start;
        if cursor < self.cursor {
            self.cursor = cursor;
        }
    }

    fn page_down(&mut self) {
        let start = self.start + self.size;
        let max = self.len() - self.size;
        self.start = if self.len() < self.size {
            0
        } else if start > max {
            max
        } else {
            start
        };
        let cursor = self.start;
        if cursor == self.cursor {
            self.cursor = self.len() - 1;
        } else if cursor > self.cursor {
            self.cursor = cursor;
        }
    }

    fn can_page_down(&self) -> bool {
        self.start + self.size < self.len()
    }

    fn can_page_up(&self) -> bool {
        self.start > 0
    }

    /// `Items()`: visible item indices and the position of the active one (or -1).
    fn visible(&self) -> (Vec<usize>, i64) {
        let end = (self.start + self.size).min(self.len());
        let mut active = NOT_FOUND;
        let mut result = Vec::new();
        let mut i = self.start;
        while i < end {
            if self.cursor == i {
                active = result.len() as i64;
            }
            result.push(self.scope[i as usize]);
            i += 1;
        }
        (result, active)
    }
}

fn faint(s: &str) -> String {
    styler("2", s)
}

fn help_line(searchable: bool) -> String {
    // `{{ "Use the arrow keys to navigate:" | faint }} {{ .NextKey | faint }} {{ .PrevKey | faint }} ...`
    let mut s = format!(
        "{} {} {} {} {} ",
        faint("Use the arrow keys to navigate:"),
        faint("↓"),
        faint("↑"),
        faint("→"),
        faint("←")
    );
    if searchable {
        s.push_str(&format!(
            " {} {} {}",
            faint("and"),
            faint("/"),
            faint("toggles search")
        ));
    }
    s
}

struct SelectState<'a> {
    opts: &'a SelectOptions<'a>,
    list: List<'a>,
    cur: Cursor,
    search_mode: bool,
    sb: ScreenBuf,
}

impl SelectState<'_> {
    /// The listener promptui gives to readline.
    fn listener(&mut self, line: &str, key: i32, out: &mut dyn Write) {
        let can_search = self.opts.searcher.is_some();
        let search_mode = self.search_mode;
        if key == CHAR_ENTER {
            return;
        } else if key == CHAR_NEXT || (key == 'j' as i32 && !search_mode) {
            self.list.next();
        } else if key == CHAR_PREV || (key == 'k' as i32 && !search_mode) {
            self.list.prev();
        } else if key == '/' as i32 {
            if can_search {
                if search_mode {
                    self.search_mode = false;
                    self.cur.replace("");
                    self.list.cancel_search();
                } else {
                    self.search_mode = true;
                }
            }
        } else if key == CHAR_BACKSPACE || key == CHAR_CTRL_H {
            if can_search && search_mode {
                self.cur.backspace();
                if !self.cur.get().is_empty() {
                    self.list.search(&self.cur.get());
                } else {
                    self.list.cancel_search();
                }
            }
        } else if key == CHAR_BACKWARD || (key == 'h' as i32 && !search_mode) {
            self.list.page_up();
        } else if key == CHAR_FORWARD || (key == 'l' as i32 && !search_mode) {
            self.list.page_down();
        } else if can_search && search_mode {
            self.cur.update(line);
            self.list.search(&self.cur.get());
        }

        if self.search_mode {
            self.sb.write(&format!("Search: {}", self.cur.format()));
        } else if !self.opts.hide_help {
            self.sb.write(&help_line(can_search));
        }
        self.sb.write(&format!(
            "\x1b[34m?\x1b[0m {}: ",
            self.opts.label.unwrap_or("<no value>")
        ));

        let (items, idx) = self.list.visible();
        let last = items.len() as i64 - 1;
        for (i, &item) in items.iter().enumerate() {
            let i = i as i64;
            let mut page = " ";
            if i == 0 {
                if self.list.can_page_up() {
                    page = "↑";
                }
            } else if i == last && self.list.can_page_down() {
                page = "↓";
            }
            let text = &self.opts.items[item];
            let rendered = if i == idx {
                format!("\x1b[1m▸\x1b[0m {}", styler("4", text))
            } else {
                format!("  {text}")
            };
            self.sb.write(&format!("{page} {rendered}"));
        }
        if idx == NOT_FOUND {
            self.sb.write("");
            self.sb.write("No results");
        }
        self.sb.flush(out);
    }
}

/// promptui `Select.Run()` reading keys from `input` (a non-terminal) and rendering on `out`.
/// Returns the index and the value of the selected item, or the promptui error (`^D`, `^C`).
pub fn run_select_piped(
    opts: &SelectOptions,
    input: &mut dyn Read,
    out: &mut dyn Write,
) -> Result<(usize, String), String> {
    let size = if opts.size == 0 { 5 } else { opts.size };
    let mut st = SelectState {
        opts,
        list: List::new(opts.items.len(), size, opts.searcher),
        cur: Cursor::new("", false),
        search_mode: opts.start_in_search_mode,
        sb: ScreenBuf::new(),
    };
    let mut term = Terminal::new(input);
    let _ = out.write_all(HIDE_CURSOR.as_bytes());

    let err = 'readline: loop {
        // rl.Readline(): the listener is called once with no key, then once per key
        st.listener("", 0, out);
        let res = loop {
            match op_read_key(&mut term, out) {
                OpEvent::Eof => break Err("^D"),
                OpEvent::Key { key, line, outcome } => {
                    st.listener(&line, key, out);
                    match outcome {
                        KeyOutcome::Continue => {}
                        KeyOutcome::Line => break Ok(()),
                        KeyOutcome::Eof => break Err("^D"),
                        KeyOutcome::Interrupt => break Err("^C"),
                    }
                }
            }
        };
        match res {
            Err(e) => break 'readline Some(e),
            Ok(()) => {
                if st.list.visible().1 != NOT_FOUND {
                    break 'readline None;
                }
            }
        }
    };

    if let Some(e) = err {
        st.sb.reset();
        st.sb.write("");
        st.sb.flush(out);
        let _ = out.write_all(SHOW_CURSOR.as_bytes());
        let _ = out.flush();
        return Err(e.to_string());
    }

    let (items, idx) = st.list.visible();
    let item = items[idx as usize];
    let value = opts.items[item].clone();
    st.sb.reset();
    st.sb
        .write(&format!("\x1b[32m\x1b[32m✔\x1b[0m {}", faint(&value)));
    st.sb.flush(out);
    let _ = out.write_all(SHOW_CURSOR.as_bytes());
    let _ = out.flush();
    Ok((item, value))
}

/// [`run_select_piped`] on the process stdin (read unbuffered, like Go) and stdout.
pub fn select_piped(opts: &SelectOptions) -> Result<(usize, String), String> {
    let mut stdin = RawStdin;
    let mut out = std::io::stdout().lock();
    run_select_piped(opts, &mut stdin, &mut out)
}

#[cfg(test)]
mod tests {
    use super::*;

    const HELP: &str = "\x1b[2mUse the arrow keys to navigate:\x1b[0m \x1b[2m↓\x1b[0m \x1b[2m↑\x1b[0m \x1b[2m→\x1b[0m \x1b[2m←\x1b[0m  \x1b[2mand\x1b[0m \x1b[2m/\x1b[0m \x1b[2mtoggles search\x1b[0m";
    const LABEL: &str = "\x1b[34m?\x1b[0m <no value>: ";

    fn run(n: usize, size: usize, stdin: &str) -> (Result<(usize, String), String>, String) {
        let items: Vec<String> = (0..n).map(|i| format!("item-{i}")).collect();
        let searcher = contains_searcher(&items);
        let opts = SelectOptions {
            label: None,
            items: &items,
            size,
            searcher: Some(&searcher),
            start_in_search_mode: false,
            hide_help: false,
        };
        let mut out = Vec::new();
        let mut input = stdin.as_bytes();
        let r = run_select_piped(&opts, &mut input, &mut out);
        (r, String::from_utf8(out).unwrap())
    }

    fn active(s: &str) -> String {
        format!("\x1b[1m▸\x1b[0m \x1b[4m{s}\x1b[0m")
    }

    #[test]
    fn eof() {
        let (r, out) = run(3, 5, "");
        assert_eq!(r.unwrap_err(), "^D");
        let expected = format!(
            "\x1b[?25l\x1b[2K\r{HELP}\n\x1b[2K\r{LABEL}\n\x1b[2K\r  {}\n\x1b[2K\r    item-1\n\x1b[2K\r    item-2\n{}\x1b[2K\r\n\x1b[?25h",
            active("item-0"),
            "\x1b[1A\x1b[2K\r".repeat(5)
        );
        assert_eq!(out, expected);
    }

    #[test]
    fn next_and_select() {
        let (r, out) = run(3, 5, "j\n");
        assert_eq!(r.unwrap(), (1, "item-1".to_string()));
        let first = format!(
            "\x1b[?25l\x1b[2K\r{HELP}\n\x1b[2K\r{LABEL}\n\x1b[2K\r  {}\n\x1b[2K\r    item-1\n\x1b[2K\r    item-2\n",
            active("item-0")
        );
        let redraw = format!(
            "{}\x1b[2K\r{HELP}\x1b[1B\x1b[2K\r{LABEL}\x1b[1B\x1b[2K\r    item-0\x1b[1B\x1b[2K\r  {}\x1b[1B\x1b[2K\r    item-2\x1b[1B",
            "\x1b[1A".repeat(5),
            active("item-1")
        );
        let fin = format!(
            "{}\x1b[2K\r\x1b[32m\x1b[32m✔\x1b[0m \x1b[2mitem-1\x1b[0m\n\x1b[?25h",
            "\x1b[1A\x1b[2K\r".repeat(5)
        );
        assert_eq!(out, format!("{first}{redraw}{redraw}{fin}"));
    }

    #[test]
    fn carriage_return_does_not_redraw() {
        let (r, out) = run(3, 5, "\r");
        assert_eq!(r.unwrap(), (0, "item-0".to_string()));
        assert_eq!(out.matches(HELP).count(), 1);
    }

    #[test]
    fn arrows_ring_the_bell() {
        let (r, out) = run(3, 5, "\x1b[B\x1b[B\x1b[A\r");
        assert_eq!(r.unwrap(), (1, "item-1".to_string()));
        assert_eq!(out.matches('\x07').count(), 3);
    }

    #[test]
    fn pagination() {
        let (r, out) = run(8, 3, "jjj\r");
        assert_eq!(r.unwrap(), (3, "item-3".to_string()));
        // last redraw: items 1..=3, scrollable both ways
        assert!(out.contains(&format!(
            "\x1b[2K\r↑   item-1\x1b[1B\x1b[2K\r    item-2\x1b[1B\x1b[2K\r↓ {}\x1b[1B",
            active("item-3")
        )));
        let (r, _) = run(8, 3, "l\r");
        assert_eq!(r.unwrap(), (3, "item-3".to_string()));
        let (r, _) = run(8, 3, "ll\r");
        assert_eq!(r.unwrap(), (5, "item-5".to_string()));
        let (r, _) = run(8, 3, "lllh\r");
        assert_eq!(r.unwrap(), (2, "item-2".to_string()));
    }

    #[test]
    fn search() {
        let (r, out) = run(12, 5, "/M-1\x7f1\x1b[B\r");
        assert_eq!(r.unwrap(), (10, "item-10".to_string()));
        assert!(out.contains("Search: M-1\u{2588}"));
        assert!(out.contains("Search: M-\u{2588}"));
        // `\n` re-runs the search (cursor back to the first match) before the selection
        let (r, _) = run(12, 5, "/1\x1b[B\n");
        assert_eq!(r.unwrap(), (1, "item-1".to_string()));
        // `/` leaves search mode
        let (r, _) = run(12, 5, "/11/j\r");
        assert_eq!(r.unwrap(), (1, "item-1".to_string()));
    }

    #[test]
    fn no_results_loops_until_eof() {
        let (r, out) = run(3, 5, "/zz\n");
        assert_eq!(r.unwrap_err(), "^D");
        assert!(out.contains("\x1b[2K\r\x1b[1B\x1b[2K\rNo results\x1b[1B"));
    }

    #[test]
    fn interrupt() {
        let (r, out) = run(3, 5, "\x03j\n");
        assert_eq!(r.unwrap_err(), "^C");
        assert_eq!(out.matches(HELP).count(), 2);
        let (r, _) = run(3, 5, "\x04");
        assert_eq!(r.unwrap_err(), "^D");
    }
}
