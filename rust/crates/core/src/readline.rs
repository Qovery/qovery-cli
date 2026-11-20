//! Emulation of what `github.com/chzyer/readline` v1.5.1 does for promptui when stdin is NOT a terminal,
//! plus promptui's `screenbuf.ScreenBuf`, `Cursor` and `Styler` (v0.9.0).
//!
//! When stdin is a pipe or a file, readline does not echo anything (its `RuneBuffer` is not interactive):
//! everything printed comes from promptui's listener, which re-renders the whole prompt after every key.
//! This module reproduces, synchronously, the parts of readline that matter for the output:
//!
//! * **stdin consumption**: each readline instance (one per `Prompt.Run` / `Select.Run`) reads stdin through
//!   its own `bufio.Reader` of 4096 bytes, and only calls `read(2)` when it does not hold a full rune.
//!   Whatever was read but not consumed when the prompt ends is lost: with `printf 'a\nb\n' | qovery ...`
//!   the whole input is read at once by the first prompt and the next prompts see EOF. [`GoStdin`] reads
//!   fd 0 directly (not Rust's buffered `std::io::stdin()`) to keep that behaviour.
//! * **key decoding** (`Terminal.ioloop`): escape sequences (`ESC [ A` -> `CharPrev`, ...), `ESC x` -> `x`
//!   or a meta key, EOF.
//! * **built-in key handling** (`Operation.ioloop`) on an always-empty line buffer (promptui's listener
//!   resets it after every key): bells on backspace / history keys, `ESC[H` on Ctrl-L, `\t` inserted by the
//!   default tab completer, Enter / Ctrl-J ending the line, Ctrl-D (EOF) and Ctrl-C (interrupt), the
//!   listener being called after every key but EOF.
//!
//! readline signals the end of the line (or the error) to promptui's goroutine *before* it calls the
//! listener for that key: both then run concurrently. The listener almost always wins (it is already
//! running), so the listener is called first here. Not reproduced: the readline search mode (Ctrl-R,
//! Ctrl-S: treated as no-op keys) and Ctrl-Z (readline suspends the process).

use std::io::{Read, Write};

pub(crate) const HIDE_CURSOR: &str = "\x1b[?25l";
pub(crate) const SHOW_CURSOR: &str = "\x1b[?25h";

// readline key codes (utils.go)
pub(crate) const CHAR_LINE_START: i32 = 1;
pub(crate) const CHAR_BACKWARD: i32 = 2;
pub(crate) const CHAR_INTERRUPT: i32 = 3;
pub(crate) const CHAR_DELETE: i32 = 4;
pub(crate) const CHAR_LINE_END: i32 = 5;
pub(crate) const CHAR_FORWARD: i32 = 6;
pub(crate) const CHAR_BELL: i32 = 7;
pub(crate) const CHAR_CTRL_H: i32 = 8;
pub(crate) const CHAR_TAB: i32 = 9;
pub(crate) const CHAR_CTRL_J: i32 = 10;
pub(crate) const CHAR_KILL: i32 = 11;
pub(crate) const CHAR_CTRL_L: i32 = 12;
pub(crate) const CHAR_ENTER: i32 = 13;
pub(crate) const CHAR_NEXT: i32 = 14;
pub(crate) const CHAR_PREV: i32 = 16;
pub(crate) const CHAR_BCK_SEARCH: i32 = 18;
pub(crate) const CHAR_FWD_SEARCH: i32 = 19;
pub(crate) const CHAR_TRANSPOSE: i32 = 20;
pub(crate) const CHAR_CTRL_U: i32 = 21;
pub(crate) const CHAR_CTRL_W: i32 = 23;
pub(crate) const CHAR_CTRL_Y: i32 = 25;
pub(crate) const CHAR_CTRL_Z: i32 = 26;
pub(crate) const CHAR_ESC: i32 = 27;
pub(crate) const CHAR_BACKSPACE: i32 = 127;
pub(crate) const META_BACKWARD: i32 = -1;
pub(crate) const META_FORWARD: i32 = -2;
pub(crate) const META_DELETE: i32 = -3;
pub(crate) const META_BACKSPACE: i32 = -4;
pub(crate) const META_TRANSPOSE: i32 = -5;

const RUNE_ERROR: i32 = 0xFFFD;

/// Go `string(r)` of a rune.
pub(crate) fn rune_char(r: i32) -> char {
    u32::try_from(r)
        .ok()
        .and_then(char::from_u32)
        .unwrap_or('\u{FFFD}')
}

/// The real stdin, read without any buffering on the Rust side (Go's `os.Stdin`).
pub(crate) struct RawStdin;

impl Read for RawStdin {
    #[cfg(unix)]
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        use std::os::fd::FromRawFd;
        // SAFETY: fd 0 stays open for the whole process; ManuallyDrop prevents closing it.
        let mut f = std::mem::ManuallyDrop::new(unsafe { std::fs::File::from_raw_fd(0) });
        loop {
            match f.read(buf) {
                Err(e) if e.kind() == std::io::ErrorKind::Interrupted => continue,
                r => return r,
            }
        }
    }

    #[cfg(not(unix))]
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        std::io::stdin().read(buf)
    }
}

/// Go `bufio.NewReader(stdin)` (4096 bytes) used through `ReadRune`.
pub(crate) struct GoStdin<'a> {
    rd: &'a mut dyn Read,
    buf: Vec<u8>,
    r: usize,
    w: usize,
    err: bool,
}

impl<'a> GoStdin<'a> {
    pub(crate) fn new(rd: &'a mut dyn Read) -> Self {
        GoStdin {
            rd,
            buf: vec![0; 4096],
            r: 0,
            w: 0,
            err: false,
        }
    }

    fn fill(&mut self) {
        if self.r > 0 {
            self.buf.copy_within(self.r..self.w, 0);
            self.w -= self.r;
            self.r = 0;
        }
        // Rust readers return Ok(0) only at EOF (Go's os.File returns io.EOF): a single read
        match self.rd.read(&mut self.buf[self.w..]) {
            Ok(0) | Err(_) => self.err = true,
            Ok(n) => self.w += n,
        }
    }

    /// `bufio.Reader.ReadRune`; `None` on error (EOF).
    pub(crate) fn read_rune(&mut self) -> Option<i32> {
        while self.r + 4 > self.w
            && !full_rune(&self.buf[self.r..self.w])
            && !self.err
            && self.w - self.r < self.buf.len()
        {
            self.fill();
        }
        if self.r == self.w {
            // readErr() clears the error
            self.err = false;
            return None;
        }
        let (rune, size) = decode_rune(&self.buf[self.r..self.w]);
        self.r += size;
        Some(rune)
    }
}

fn utf8_len(b0: u8) -> usize {
    match b0 {
        0x00..=0x7F => 1,
        0xC2..=0xDF => 2,
        0xE0..=0xEF => 3,
        0xF0..=0xF4 => 4,
        _ => 1,
    }
}

/// `utf8.FullRune`.
fn full_rune(p: &[u8]) -> bool {
    if p.is_empty() {
        return false;
    }
    let n = utf8_len(p[0]);
    if p.len() >= n {
        return true;
    }
    // an invalid continuation byte makes it a (1-byte) full rune
    if p.len() > 1 && (p[1] & 0xC0) != 0x80 {
        return true;
    }
    if p.len() > 1 {
        let invalid_second = match p[0] {
            0xE0 => p[1] < 0xA0,
            0xED => p[1] > 0x9F,
            0xF0 => p[1] < 0x90,
            0xF4 => p[1] > 0x8F,
            _ => false,
        };
        if invalid_second {
            return true;
        }
    }
    if p.len() > 2 && (p[2] & 0xC0) != 0x80 {
        return true;
    }
    false
}

/// `utf8.DecodeRune`: (rune, size), `RuneError` with size 1 on invalid input.
fn decode_rune(p: &[u8]) -> (i32, usize) {
    let n = utf8_len(p[0]);
    if n == 1 {
        return if p[0] < 0x80 {
            (p[0] as i32, 1)
        } else {
            (RUNE_ERROR, 1)
        };
    }
    if p.len() < n {
        return (RUNE_ERROR, 1);
    }
    match std::str::from_utf8(&p[..n]) {
        Ok(s) => (s.chars().next().map(|c| c as i32).unwrap_or(RUNE_ERROR), n),
        Err(_) => (RUNE_ERROR, 1),
    }
}

/// readline `Terminal.ioloop`: turns stdin runes into keys. Returns 0 forever after EOF.
pub(crate) struct Terminal<'a> {
    stdin: GoStdin<'a>,
    closed: bool,
    is_escape: bool,
    is_escape_ex: bool,
    is_escape_ss3: bool,
}

impl<'a> Terminal<'a> {
    pub(crate) fn new(rd: &'a mut dyn Read) -> Self {
        Terminal {
            stdin: GoStdin::new(rd),
            closed: false,
            is_escape: false,
            is_escape_ex: false,
            is_escape_ss3: false,
        }
    }

    /// `readEscKey`: digits and `;` then the type rune (errors read as rune 0).
    fn read_esc_key(&mut self, mut r: i32) -> (String, i32) {
        let mut attr = String::new();
        loop {
            if r != ';' as i32 && !rune_char(r).is_numeric() {
                return (attr, r);
            }
            attr.push(rune_char(r));
            r = self.stdin.read_rune().unwrap_or(0);
        }
    }

    pub(crate) fn read_key(&mut self) -> i32 {
        if self.closed {
            return 0;
        }
        loop {
            let Some(mut r) = self.stdin.read_rune() else {
                self.closed = true;
                return 0;
            };
            if self.is_escape {
                self.is_escape = false;
                if r == '[' as i32 {
                    self.is_escape_ex = true;
                    continue;
                } else if r == 'O' as i32 {
                    self.is_escape_ss3 = true;
                    continue;
                }
                r = match r {
                    0x62 /* b */ => META_BACKWARD,
                    0x66 /* f */ => META_FORWARD,
                    0x64 /* d */ => META_DELETE,
                    CHAR_TRANSPOSE => META_TRANSPOSE,
                    CHAR_BACKSPACE => META_BACKSPACE,
                    other => other,
                };
            } else if self.is_escape_ex {
                self.is_escape_ex = false;
                let (attr, typ) = self.read_esc_key(r);
                r = match typ {
                    0x44 /* D */ => CHAR_BACKWARD,
                    0x43 /* C */ => CHAR_FORWARD,
                    0x41 /* A */ => CHAR_PREV,
                    0x42 /* B */ => CHAR_NEXT,
                    0x48 /* H */ => CHAR_LINE_START,
                    0x46 /* F */ => CHAR_LINE_END,
                    0x7E /* ~ */ if attr == "3" => CHAR_DELETE,
                    _ => 0,
                };
                if typ == 'R' as i32 {
                    // cursor position report: swallowed when it has two numbers
                    let sp: Vec<&str> = attr.split(';').collect();
                    if sp.len() >= 2 && sp[0].parse::<i64>().is_ok() && sp[1].parse::<i64>().is_ok()
                    {
                        continue;
                    }
                }
                if r == 0 {
                    continue;
                }
            } else if self.is_escape_ss3 {
                self.is_escape_ss3 = false;
                let (_, typ) = self.read_esc_key(r);
                r = match typ {
                    0x44 => CHAR_BACKWARD,
                    0x43 => CHAR_FORWARD,
                    0x41 => CHAR_PREV,
                    0x42 => CHAR_NEXT,
                    0x48 => CHAR_LINE_START,
                    0x46 => CHAR_LINE_END,
                    _ => 0,
                };
                if r == 0 {
                    continue;
                }
            }
            if r == CHAR_ESC {
                self.is_escape = true;
                continue;
            }
            return r;
        }
    }
}

/// What readline does with a key, before calling promptui's listener.
pub(crate) enum KeyOutcome {
    /// keep reading keys
    Continue,
    /// Enter / Ctrl-J: `Readline()` returns the line
    Line,
    /// Ctrl-D on an empty line: `Readline()` returns `io.EOF`
    Eof,
    /// Ctrl-C: `Readline()` returns `ErrInterrupt`
    Interrupt,
}

pub(crate) enum OpEvent {
    /// stdin EOF (or a NUL rune): `Readline()` returns `io.EOF`, the listener is NOT called
    Eof,
    /// a key: call the listener with `line` (readline's buffer) and `key`, then act on `outcome`
    Key {
        key: i32,
        line: String,
        outcome: KeyOutcome,
    },
}

/// readline `Operation.ioloop` for one key, on an empty line buffer (promptui resets it after every key).
/// Bells and screen clears are written to `out` (readline writes them directly to stdout).
pub(crate) fn op_read_key(term: &mut Terminal, out: &mut dyn Write) -> OpEvent {
    let key = term.read_key();
    let mut line = String::new();
    let mut outcome = KeyOutcome::Continue;
    match key {
        0 => return OpEvent::Eof,
        CHAR_BELL => {}
        // the default TabCompleter completes with "\t" (single candidate)
        CHAR_TAB => line.push('\t'),
        // readline search mode: not reproduced
        CHAR_BCK_SEARCH | CHAR_FWD_SEARCH => {}
        CHAR_CTRL_U | CHAR_KILL | META_FORWARD | CHAR_TRANSPOSE | META_BACKWARD | META_DELETE
        | CHAR_LINE_START | CHAR_LINE_END => {}
        CHAR_BACKSPACE | CHAR_CTRL_H => {
            let _ = out.write_all(b"\x07");
        }
        // readline suspends the process (SIGTSTP): not reproduced
        CHAR_CTRL_Z => {}
        CHAR_CTRL_L => {
            let _ = out.write_all(b"\x1b[H");
        }
        META_BACKSPACE | CHAR_CTRL_W | CHAR_CTRL_Y => {}
        CHAR_ENTER | CHAR_CTRL_J => outcome = KeyOutcome::Line,
        CHAR_BACKWARD | CHAR_FORWARD => {}
        // no history (HistoryLimit: -1)
        CHAR_PREV | CHAR_NEXT => {
            let _ = out.write_all(b"\x07");
        }
        CHAR_DELETE => outcome = KeyOutcome::Eof,
        CHAR_INTERRUPT => outcome = KeyOutcome::Interrupt,
        other => line.push(rune_char(other)),
    }
    OpEvent::Key { key, line, outcome }
}

/// promptui `screenbuf.ScreenBuf`.
pub(crate) struct ScreenBuf {
    buf: Vec<u8>,
    reset: bool,
    cursor: usize,
    height: usize,
}

const CLEAR_LINE: &[u8] = b"\x1b[2K\r";
const MOVE_UP: &[u8] = b"\x1b[1A";
const MOVE_DOWN: &[u8] = b"\x1b[1B";

impl ScreenBuf {
    pub(crate) fn new() -> Self {
        ScreenBuf {
            buf: Vec::new(),
            reset: false,
            cursor: 0,
            height: 0,
        }
    }

    pub(crate) fn reset(&mut self) {
        self.buf.clear();
        self.reset = true;
    }

    pub(crate) fn clear(&mut self) {
        for _ in 0..self.height {
            self.buf.extend_from_slice(MOVE_UP);
            self.buf.extend_from_slice(CLEAR_LINE);
        }
        self.cursor = 0;
        self.height = 0;
        self.reset = false;
    }

    /// Lines containing `\r` or `\n` are rejected (not written) like in Go.
    pub(crate) fn write(&mut self, b: &str) {
        if b.contains(['\r', '\n']) {
            return;
        }
        if self.reset {
            self.clear();
        }
        if self.cursor == self.height {
            self.buf.extend_from_slice(CLEAR_LINE);
            self.buf.extend_from_slice(b.as_bytes());
            self.buf.push(b'\n');
            self.height += 1;
            self.cursor += 1;
        } else if self.cursor < self.height {
            self.buf.extend_from_slice(CLEAR_LINE);
            self.buf.extend_from_slice(b.as_bytes());
            self.buf.extend_from_slice(MOVE_DOWN);
            self.cursor += 1;
        }
    }

    pub(crate) fn flush(&mut self, out: &mut dyn Write) {
        for _ in self.cursor..self.height {
            self.buf.extend_from_slice(CLEAR_LINE);
            self.buf.extend_from_slice(MOVE_DOWN);
        }
        let _ = out.write_all(&self.buf);
        let _ = out.flush();
        self.buf.clear();
        for _ in 0..self.height {
            self.buf.extend_from_slice(MOVE_UP);
        }
        self.cursor = 0;
    }
}

/// promptui `Cursor` with the default pointer (`█`, hiding the rune under the cursor).
pub(crate) struct Cursor {
    input: Vec<char>,
    pub(crate) position: usize,
    pub(crate) erase: bool,
}

impl Cursor {
    pub(crate) fn new(starting_input: &str, erase_default: bool) -> Self {
        let input: Vec<char> = starting_input.chars().collect();
        let position = if erase_default { 0 } else { input.len() };
        Cursor {
            input,
            position,
            erase: erase_default,
        }
    }

    pub(crate) fn format(&self) -> String {
        let mut out = String::new();
        if self.position < self.input.len() {
            out.extend(&self.input[..self.position]);
            out.push('\u{2588}');
            out.extend(&self.input[self.position + 1..]);
        } else {
            out.extend(&self.input);
            out.push('\u{2588}');
        }
        out
    }

    pub(crate) fn update(&mut self, new_input: &str) {
        let b: Vec<char> = new_input.chars().collect();
        let n = b.len();
        let tail = self.input.split_off(self.position);
        self.input.extend(b);
        self.input.extend(tail);
        self.move_by(n as i64);
    }

    pub(crate) fn get(&self) -> String {
        self.input.iter().collect()
    }

    pub(crate) fn replace(&mut self, input: &str) {
        self.input = input.chars().collect();
        self.position = self.input.len();
    }

    pub(crate) fn move_by(&mut self, shift: i64) {
        let p = (self.position as i64 + shift).clamp(0, self.input.len() as i64);
        self.position = p as usize;
    }

    pub(crate) fn backspace(&mut self) {
        if self.position == 0 {
            return;
        }
        self.input.remove(self.position - 1);
        self.move_by(-1);
    }
}

/// promptui `Styler(attr)(v)` on a string: no extra reset when `v` already ends with one.
pub(crate) fn styler(attr: &str, v: &str) -> String {
    let end = if v.ends_with("\x1b[0m") {
        ""
    } else {
        "\x1b[0m"
    };
    format!("\x1b[{attr}m{v}{end}")
}

/// Go `strings.ToLower`.
pub(crate) fn go_to_lower(s: &str) -> String {
    s.chars()
        .map(|c| {
            if c == '\u{130}' {
                return 'i';
            }
            let mut l = c.to_lowercase();
            match (l.next(), l.next()) {
                (Some(x), None) => x,
                _ => c,
            }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn keys(input: &[u8]) -> Vec<i32> {
        let mut rd = input;
        let mut t = Terminal::new(&mut rd);
        let mut v = Vec::new();
        loop {
            let k = t.read_key();
            v.push(k);
            if k == 0 {
                return v;
            }
        }
    }

    #[test]
    fn decode_keys() {
        assert_eq!(
            keys(b"j\x1b[A\x1b[B\x1b[C\x1b[D\x1bOA\x1b[3~\x1bbx\x1b"),
            vec![
                'j' as i32,
                CHAR_PREV,
                CHAR_NEXT,
                CHAR_FORWARD,
                CHAR_BACKWARD,
                CHAR_PREV,
                CHAR_DELETE,
                META_BACKWARD,
                'x' as i32,
                0
            ]
        );
        assert_eq!(
            keys(b"\xc3\xa9\xe2\x96\x88\xe2\x96"),
            vec!['é' as i32, 0x2588, RUNE_ERROR, RUNE_ERROR, 0]
        );
        assert_eq!(
            keys(b"\xffa\x1b[12;3R\x1b[5~b"),
            vec![RUNE_ERROR, 'a' as i32, 'b' as i32, 0]
        );
    }

    #[test]
    fn chunked_reads_lose_the_rest() {
        let mut rd: &[u8] = b"ab";
        {
            let mut s = GoStdin::new(&mut rd);
            assert_eq!(s.read_rune(), Some('a' as i32));
        }
        // the whole input was read by the first reader
        assert!(rd.is_empty());
    }

    #[test]
    fn screenbuf() {
        let mut sb = ScreenBuf::new();
        let mut out = Vec::new();
        sb.write("a");
        sb.write("b");
        sb.flush(&mut out);
        sb.write("c");
        sb.flush(&mut out);
        sb.reset();
        sb.write("");
        sb.flush(&mut out);
        assert_eq!(
            String::from_utf8(out).unwrap(),
            "\x1b[2K\ra\n\x1b[2K\rb\n\x1b[1A\x1b[1A\x1b[2K\rc\x1b[1B\x1b[2K\r\x1b[1B\x1b[1A\x1b[2K\r\x1b[1A\x1b[2K\r\x1b[2K\r\n"
        );
    }

    #[test]
    fn cursor() {
        let mut c = Cursor::new("abc", true);
        assert_eq!(c.format(), "\u{2588}bc");
        c.move_by(1);
        c.update("x");
        assert_eq!(c.get(), "axbc");
        assert_eq!(c.format(), "ax\u{2588}c");
        c.backspace();
        assert_eq!(c.get(), "abc");
        assert_eq!(styler("4", "x"), "\x1b[4mx\x1b[0m");
        assert_eq!(styler("32", "\x1b[32m✔\x1b[0m"), "\x1b[32m\x1b[32m✔\x1b[0m");
    }
}
