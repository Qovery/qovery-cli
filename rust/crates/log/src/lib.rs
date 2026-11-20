//! Output primitives of the Qovery CLI.
//!
//! Everything in this crate reproduces, byte for byte, what the Go implementation prints:
//!
//! * [`printer`]: `utils.PrintlnError`, `utils.PrintlnInfo`, ... (fatih/color semantics: colors only on a tty)
//! * [`pterm`]: `pterm.FgBlue.Sprintf`, `pterm.Bold.Sprintf`, sections, prefix printers (pterm always colors)
//! * [`table`]: `pterm.DefaultTable` / `utils.PrintTable`
//! * [`logrus`]: `log.Infof`, `log.Warnf`, `log.Fatal`, ... (logrus TextFormatter with FullTimestamp)
//! * [`gofmt`]: Go formatting helpers (`%q`, `time.Time.String()`, `time.Duration.String()`)
//! * [`gotime`]: Go `time.Parse(time.RFC3339, ...)` with its error messages

pub mod fatih;
pub mod gofmt;
pub mod gojson;
pub mod gotime;
pub mod logrus;
pub mod printer;
pub mod pterm;
pub mod table;
pub mod tabwriter;

pub use printer::*;

/// Offset of the local time zone, in seconds east of UTC.
pub fn local_utc_offset_seconds() -> i64 {
    use chrono::Offset;
    chrono::Local::now().offset().fix().local_minus_utc() as i64
}
