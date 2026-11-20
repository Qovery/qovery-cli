//! Port of `github.com/sirupsen/logrus` as configured by the CLI
//! (`log.SetFormatter(&log.TextFormatter{FullTimestamp: true})`, level Info, output stderr).
//!
//! * stderr is a tty:   `\x1b[36mINFO\x1b[0m[2026-11-19T09:50:02+01:00] message                                      `
//! * otherwise:         `time="2026-11-19T09:50:02+01:00" level=info msg="message"`

use crate::gofmt::quote;
use is_terminal::IsTerminal;
use std::io::Write;
use std::sync::OnceLock;
use std::sync::atomic::{AtomicU8, Ordering};

#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
#[repr(u8)]
pub enum Level {
    Panic = 0,
    Fatal = 1,
    Error = 2,
    Warn = 3,
    Info = 4,
    Debug = 5,
    Trace = 6,
}

impl Level {
    fn text(&self) -> &'static str {
        match self {
            Level::Panic => "panic",
            Level::Fatal => "fatal",
            Level::Error => "error",
            Level::Warn => "warning",
            Level::Info => "info",
            Level::Debug => "debug",
            Level::Trace => "trace",
        }
    }
    fn color(&self) -> u8 {
        match self {
            Level::Debug | Level::Trace => 37,
            Level::Warn => 33,
            Level::Error | Level::Fatal | Level::Panic => 31,
            Level::Info => 36,
        }
    }
}

static LEVEL: AtomicU8 = AtomicU8::new(Level::Info as u8);

/// `log.SetLevel(level)`.
pub fn set_level(level: Level) {
    LEVEL.store(level as u8, Ordering::Relaxed);
}

fn enabled(level: Level) -> bool {
    (level as u8) <= LEVEL.load(Ordering::Relaxed)
}

fn is_terminal() -> bool {
    static TTY: OnceLock<bool> = OnceLock::new();
    *TTY.get_or_init(|| std::io::stderr().is_terminal())
}

fn needs_quoting(text: &str) -> bool {
    !text.chars().all(|ch| {
        ch.is_ascii_alphanumeric() || matches!(ch, '-' | '.' | '_' | '/' | '@' | '^' | '+')
    })
}

/// Format an entry exactly like logrus' TextFormatter (with optional fields, sorted by key).
pub fn format_entry(level: Level, msg: &str, fields: &[(&str, String)], tty: bool) -> String {
    let now = chrono::Local::now();
    let ts = now.format("%Y-%m-%dT%H:%M:%S%:z").to_string();
    let mut fields: Vec<(&str, String)> = fields.to_vec();
    fields.sort_by(|a, b| a.0.cmp(b.0));
    let mut b = String::new();
    if tty {
        let msg = msg.strip_suffix('\n').unwrap_or(msg);
        let level_text: String = level.text().to_uppercase().chars().take(4).collect();
        b.push_str(&format!(
            "\x1b[{}m{}\x1b[0m[{}] {:<44} ",
            level.color(),
            level_text,
            ts,
            msg
        ));
        for (k, v) in &fields {
            let v = if needs_quoting(v) {
                quote(v)
            } else {
                v.clone()
            };
            b.push_str(&format!(" \x1b[{}m{}\x1b[0m={}", level.color(), k, v));
        }
    } else {
        let mut kv = |k: &str, v: &str| {
            if !b.is_empty() {
                b.push(' ');
            }
            b.push_str(k);
            b.push('=');
            if needs_quoting(v) {
                b.push_str(&quote(v));
            } else {
                b.push_str(v);
            }
        };
        kv("time", &ts);
        kv("level", level.text());
        if !msg.is_empty() {
            kv("msg", msg);
        }
        for (k, v) in &fields {
            kv(k, v);
        }
    }
    b.push('\n');
    b
}

fn emit(level: Level, msg: &str, fields: &[(&str, String)]) {
    if !enabled(level) {
        return;
    }
    let line = format_entry(level, msg, fields, is_terminal());
    let mut err = std::io::stderr().lock();
    let _ = err.write_all(line.as_bytes());
}

pub fn trace(msg: impl AsRef<str>) {
    emit(Level::Trace, msg.as_ref(), &[]);
}
pub fn debug(msg: impl AsRef<str>) {
    emit(Level::Debug, msg.as_ref(), &[]);
}
pub fn info(msg: impl AsRef<str>) {
    emit(Level::Info, msg.as_ref(), &[]);
}
pub fn warn(msg: impl AsRef<str>) {
    emit(Level::Warn, msg.as_ref(), &[]);
}
pub fn error(msg: impl AsRef<str>) {
    emit(Level::Error, msg.as_ref(), &[]);
}
/// `log.Fatal(...)`: logs at fatal level then exits with status 1.
pub fn fatal(msg: impl AsRef<str>) -> ! {
    emit(Level::Fatal, msg.as_ref(), &[]);
    crate::printer::flush();
    std::process::exit(1)
}
/// `log.Panic(...)`: logs at panic level then panics (Go exits with status 2 and a stack trace).
pub fn panic(msg: impl AsRef<str>) -> ! {
    emit(Level::Panic, msg.as_ref(), &[]);
    crate::printer::flush();
    std::process::exit(2)
}
/// `log.WithFields(fields).Info(msg)` and friends.
pub fn with_fields(level: Level, msg: impl AsRef<str>, fields: &[(&str, String)]) {
    emit(level, msg.as_ref(), fields);
}

/// `log.Infof(format, args...)`.
#[macro_export]
macro_rules! log_info { ($($arg:tt)*) => { $crate::logrus::info(format!($($arg)*)) }; }
/// `log.Warnf(format, args...)`.
#[macro_export]
macro_rules! log_warn { ($($arg:tt)*) => { $crate::logrus::warn(format!($($arg)*)) }; }
/// `log.Errorf(format, args...)`.
#[macro_export]
macro_rules! log_error { ($($arg:tt)*) => { $crate::logrus::error(format!($($arg)*)) }; }
/// `log.Debugf(format, args...)`.
#[macro_export]
macro_rules! log_debug { ($($arg:tt)*) => { $crate::logrus::debug(format!($($arg)*)) }; }
/// `log.Fatalf(format, args...)`.
#[macro_export]
macro_rules! log_fatal { ($($arg:tt)*) => { $crate::logrus::fatal(format!($($arg)*)) }; }

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn plain_format() {
        let l = format_entry(Level::Info, "Dry run: x", &[], false);
        assert!(l.contains(" level=info msg=\"Dry run: x\"\n"), "{l}");
        let l = format_entry(Level::Warn, "simple", &[], false);
        assert!(l.ends_with(" level=warning msg=simple\n"), "{l}");
    }
}
