//! `qovery env` command. Port of `cmd/env.go`, `cmd/env_import.go` and `cmd/env_parse.go`.

mod dotenv;
mod gojson;

use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{context, exit, telemetry};
use qovery_log::{println, println_error, println_info};
use std::collections::BTreeMap;
use std::io::Read;
use std::path::{Path, PathBuf};

/// The `env` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("env")
        .short("Manage Environment Variables and Secrets")
        .subcommand(
            Command::new("import")
                .short("Import environment variables/secrets for a Qovery app")
                .flag(Flag::bool("sort", "Sort environment variables by key"))
                .run(run_import),
        )
        .subcommand(
            Command::new("parse")
                .short("Parse environment variables and create .env (dot env) file")
                .flag(
                    Flag::bool(
                        "heroku-json",
                        "Parse environment variables from a Heroku JSON payload",
                    )
                    .short('j'),
                )
                .run(run_parse),
        )
}

/// survey errors: `EOF` when stdin is not a terminal, `interrupt` on Ctrl-C.
fn survey_error(e: &inquire::InquireError) -> String {
    match e {
        inquire::InquireError::NotTTY => "EOF".to_string(),
        inquire::InquireError::OperationInterrupted | inquire::InquireError::OperationCanceled => {
            "interrupt".to_string()
        }
        other => other.to_string(),
    }
}

/// survey `Select`.
fn survey_select(message: &str, options: &[&str]) -> Result<String, String> {
    let options: Vec<String> = options.iter().map(|s| s.to_string()).collect();
    inquire::Select::new(message, options)
        .prompt()
        .map_err(|e| survey_error(&e))
}

fn run_import(ctx: &Ctx) {
    telemetry::capture(ctx);
    let args = ctx.args();

    let dot_env_file_path = match args.len() {
        0 => match scan_and_select_dot_env_file() {
            Ok(f) => f,
            Err(e) => {
                println_error(e);
                return;
            }
        },
        1 => args[0].clone(),
        _ => {
            println_error("more than one arg is not allowed");
            return;
        }
    };

    if dot_env_file_path.is_empty() {
        println_error("no dot env file specified");
        return;
    }

    let envs = match dotenv::read(&dot_env_file_path) {
        Ok(e) => e,
        Err(e) => {
            println_error(e);
            return;
        }
    };

    let service = match context::current_service(true) {
        Ok(s) => s,
        Err(e) => {
            println_error(e);
            exit(0);
        }
    };

    if service.service_type != "application" && service.service_type != "container" {
        println_error(format!(
            "cannot import variables for service of type {} (only Application and Container are supported)",
            service.service_type
        ));
        exit(0);
    }

    println_info(format!("dot env file to import: '{dot_env_file_path}'"));

    let env_var_or_secret = match survey_select(
        "Do you want to import Environment Variables or Secrets?",
        &["Environment Variables", "Secrets"],
    ) {
        Ok(s) => s,
        Err(e) => {
            println_error(e);
            return;
        }
    };
    let is_secrets = env_var_or_secret == "Secrets";

    let envs_to_import = get_envs_to_import(&envs, ctx.bool("sort"));
    if envs_to_import.is_empty() {
        println_error("no environment variables to import");
        return;
    }

    let override_answer = match survey_select(
        &format!("Do you want to overwrite existing {env_var_or_secret}?"),
        &["No", "Yes"],
    ) {
        Ok(s) => s,
        Err(e) => {
            println_error(e);
            return;
        }
    };
    let override_existing = override_answer == "Yes";

    let is_container = service.service_type == "container";
    let id = service.id.as_str();
    let mut errors: Vec<String> = Vec::new();
    // Go iterates over a map (random order); keys are sorted here
    for (k, v) in &envs_to_import {
        let result = match (is_container, is_secrets) {
            (true, true) => {
                if override_existing {
                    let _ = qovery_services::delete_container_secret(id, k);
                }
                qovery_services::add_container_secret(id, k, v)
            }
            (true, false) => {
                if override_existing {
                    let _ = qovery_services::delete_container_environment_variable(id, k);
                }
                qovery_services::add_container_environment_variable(id, k, v)
            }
            (false, true) => {
                if override_existing {
                    let _ = qovery_services::delete_secret(id, k);
                }
                qovery_services::add_secret(id, k, v)
            }
            (false, false) => {
                if override_existing {
                    let _ = qovery_services::delete_environment_variable(id, k);
                }
                qovery_services::add_environment_variable(id, k, v)
            }
        };
        if let Err(e) = result {
            errors.push(format!("{k} ({e})"));
        }
    }

    if errors.is_empty() {
        println_info(format!("✅ {env_var_or_secret} successfully imported!"));
    } else {
        println_error(format!(
            "those {env_var_or_secret} have failed to be imported: {}",
            errors.join(", ")
        ));
    }
}

/// `scanAndSelectDotEnvFile`: every path under the current directory containing `.env`, then a selection.
fn scan_and_select_dot_env_file() -> Result<String, String> {
    let current_dir = go_getwd().map_err(|e| e.to_string())?;

    let mut files = Vec::new();
    walk(&current_dir, &mut |p: &Path| {
        let s = p.to_string_lossy();
        if s.contains(".env") {
            files.push(s.into_owned());
        }
    });

    if files.is_empty() {
        return Err(format!("no .env file found in '{}'", current_dir.display()));
    }

    qovery_core::select::prompt_select_with_label("Select your .env file", &files, None)
        .map(|(_, v)| v)
}

/// Go `os.Getwd()`: `$PWD` when it designates the current directory.
#[cfg(unix)]
fn go_getwd() -> std::io::Result<PathBuf> {
    let cwd = std::env::current_dir()?;
    if let Ok(pwd) = std::env::var("PWD") {
        let pwd = PathBuf::from(pwd);
        if pwd.is_absolute()
            && let (Ok(a), Ok(b)) = (std::fs::metadata(&pwd), std::fs::metadata(&cwd))
        {
            use std::os::unix::fs::MetadataExt;
            if a.dev() == b.dev() && a.ino() == b.ino() {
                return Ok(pwd);
            }
        }
    }
    Ok(cwd)
}

/// Go `os.Getwd()` on Windows: the process working directory, `$PWD` is not consulted.
#[cfg(not(unix))]
fn go_getwd() -> std::io::Result<PathBuf> {
    std::env::current_dir()
}

/// Go `filepath.Walk` (lexical order, symlinks not followed, errors ignored by the callback).
fn walk(path: &Path, f: &mut dyn FnMut(&Path)) {
    f(path);
    let is_dir = std::fs::symlink_metadata(path)
        .map(|m| m.is_dir())
        .unwrap_or(false);
    if !is_dir {
        return;
    }
    let Ok(rd) = std::fs::read_dir(path) else {
        return;
    };
    let mut names: Vec<_> = rd.filter_map(|e| e.ok()).map(|e| e.file_name()).collect();
    names.sort_by(|a, b| a.as_encoded_bytes().cmp(b.as_encoded_bytes()));
    for name in names {
        walk(&path.join(name), f);
    }
}

/// `getEnvsToImport`: multi-selection of the `KEY=value` lines to import.
fn get_envs_to_import(
    envs: &BTreeMap<String, String>,
    _sort_keys: bool,
) -> BTreeMap<String, String> {
    // Go lists the keys in map order (random) unless sorted: sorted in both cases here
    let env_keys: Vec<String> = envs.iter().map(|(k, v)| format!("{k}={v}")).collect();

    let selected = match inquire::MultiSelect::new(
        "What environment variables do you want to import?",
        env_keys,
    )
    .prompt()
    {
        Ok(s) => s,
        Err(_) => return BTreeMap::new(),
    };

    let mut results = BTreeMap::new();
    for key in selected {
        let k = key.split('=').next().unwrap_or_default().to_string();
        let v = envs.get(&k).cloned().unwrap_or_default();
        results.insert(k, v);
    }
    results
}

fn run_parse(ctx: &Ctx) {
    telemetry::capture(ctx);

    if ctx.args().len() > 1 {
        println_error("more than one arg is not allowed");
        return;
    }

    if !ctx.bool("heroku-json") {
        println_error(
            "you need to specify an option like '--heroku-json' or other. Type -h to see different options",
        );
        return;
    }

    let mut input = Vec::new();
    let decoded = match std::io::stdin().read_to_end(&mut input) {
        Ok(_) => gojson::decode_string_map(&input),
        Err(e) => Err(e.to_string()),
    };

    let envs = match decoded {
        Ok(e) => e.unwrap_or_default(),
        Err(e) => {
            println_error(e);
            println("Did you execute 'heroku config -a <your_heroku_app_name> --json' command?");
            return;
        }
    };

    // Go iterates over a map (random order); input order here
    let mut out = String::new();
    for (key, value) in envs {
        out.push_str(&format!("{key}={value}\n"));
    }
    print!("{out}");
}
