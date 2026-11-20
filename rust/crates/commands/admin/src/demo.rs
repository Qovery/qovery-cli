//! `qovery admin demo ...`. Port of `cmd/admin_demo*.go`.

use crate::goutil::encode_query;
use crate::http::{Req, authorization};
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::client::admin_url;
use qovery_core::telemetry;
use reqwest::Method;
use serde_json::Value;

pub fn command() -> Command {
    Command::new("demo")
        .short("get errors logs for the demo")
        .run(crate::run_group_help)
        .subcommand(
            Command::new("get-log")
                .short("retrieve a specific log")
                .run(run_get_log),
        )
        .subcommand(
            Command::new("list-logs")
                .short("list error logs from the command demo up")
                .flag(
                    Flag::string("organizationId", "Organization to filter on for listing")
                        .short('o'),
                )
                .run(run_list_logs),
        )
}

fn url_with_query(base: &str, pairs: &[(&str, &str)]) -> String {
    let q = encode_query(pairs);
    if q.is_empty() {
        base.to_string()
    } else {
        format!("{base}?{q}")
    }
}

fn run_get_log(ctx: &Ctx) {
    telemetry::capture(ctx);
    let Some(filename) = ctx.args().first().cloned() else {
        qovery_log::logrus::fatal("You must specify a log filename as argument");
    };
    let auth = qovery_core::check_error(authorization());
    let url = url_with_query(
        &format!("{}/demoDebugLog", admin_url()),
        &[("filename", &filename)],
    );
    let res = match Req::new(Method::GET, &url, &auth).no_content_type().send() {
        Ok(r) => r,
        Err(e) => {
            qovery_log::println_error(format!("error uploading debug logs: {e}"));
            return;
        }
    };
    if res.status != 200 {
        qovery_log::println_error(format!(
            "error uploading debug logs: {} {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let _ = write_file_with_mode(&filename, &res.body, 0o640);
    qovery_log::logrus::info(format!("file written to {filename}"));
}

/// Go `os.WriteFile(name, data, perm)`.
fn write_file_with_mode(path: &str, data: &[u8], mode: u32) -> std::io::Result<()> {
    use std::io::Write;
    let mut opts = std::fs::OpenOptions::new();
    opts.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        opts.mode(mode);
    }
    #[cfg(not(unix))]
    let _ = mode;
    opts.open(path)?.write_all(data)
}

fn run_list_logs(ctx: &Ctx) {
    telemetry::capture(ctx);
    let auth = qovery_core::check_error(authorization());
    let orga_id = ctx.string("organizationId");
    let mut pairs: Vec<(&str, &str)> = Vec::new();
    if !orga_id.is_empty() {
        pairs.push(("organization", &orga_id));
    }
    let url = url_with_query(&format!("{}/demoDebugLog", admin_url()), &pairs);
    let res = match Req::new(Method::GET, &url, &auth).no_content_type().send() {
        Ok(r) => r,
        Err(e) => {
            qovery_log::println_error(format!("error uploading debug logs: {e}"));
            return;
        }
    };
    if res.status != 200 {
        qovery_log::println_error(format!(
            "error uploading debug logs: {} {}",
            res.status_line(),
            res.text()
        ));
        return;
    }
    let logs: Value = serde_json::from_slice(&res.body).unwrap_or(Value::Null);
    let rows: Vec<Vec<String>> = logs
        .as_array()
        .map(|a| {
            a.iter()
                .map(|l| {
                    vec![
                        l.get("last_modified")
                            .and_then(|v| v.as_str())
                            .unwrap_or_default()
                            .to_string(),
                        l.get("filename")
                            .and_then(|v| v.as_str())
                            .unwrap_or_default()
                            .to_string(),
                    ]
                })
                .collect()
        })
        .unwrap_or_default();
    qovery_log::print_table(&["date", "filename"], &rows);
}
