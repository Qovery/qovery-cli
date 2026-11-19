//! `qovery cronjob` command. Port of `cmd/cronjob*.go`.

mod actions;
mod env;

use qovery_api::Client;
use qovery_api::models::Job;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::find::trim_name;
use qovery_core::{exit, telemetry};

/// The `cronjob` command with all its sub-commands.
pub fn command() -> Command {
    Command::new("cronjob")
        .short("Manage cronjobs")
        .run(run_group)
        .subcommand(actions::cancel())
        .subcommand(actions::clone())
        .subcommand(actions::delete())
        .subcommand(actions::deploy())
        .subcommand(env::env())
        .subcommand(env::external_secret())
        .subcommand(actions::list())
        .subcommand(actions::redeploy())
        .subcommand(actions::stop())
        .subcommand(actions::update())
}

/// Go groups: `utils.Capture(cmd); if len(args) == 0 { _ = cmd.Help(); os.Exit(0) }`.
pub(crate) fn run_group(ctx: &Ctx) {
    telemetry::capture(ctx);
    if ctx.args().is_empty() {
        ctx.help();
        exit(0);
    }
}

/// `--organization`, `--project`, `--environment`, `--cronjob/-n` flags shared by most sub-commands.
pub(crate) fn context_flags() -> Vec<Flag> {
    vec![
        Flag::string("organization", "Organization Name"),
        Flag::string("project", "Project Name"),
        Flag::string("environment", "Environment Name"),
    ]
}

pub(crate) fn cronjob_flag() -> Flag {
    Flag::string("cronjob", "Cronjob Name").short('n')
}

/// `utils.FindByJobName(jobs, name)`: only cron and lifecycle jobs (other job types are decoded by the Go
/// client as an empty `JobResponse`).
pub(crate) fn find_by_job_name<'a>(jobs: &'a [Job], name: &str) -> Option<&'a Job> {
    let name = trim_name(name);
    jobs.iter()
        .find(|j| (j.is_cron() || j.is_lifecycle()) && trim_name(&j.name) == name)
}

/// `client.JobsAPI.ListJobs(ctx, envId)`.
pub(crate) fn list_jobs(client: &Client, env_id: &str) -> Result<Vec<Job>, String> {
    client
        .list(&format!(
            "/environment/{}/job",
            qovery_services::path_escape(env_id)
        ))
        .map_err(|e| e.to_string())
}

/// `cmd.ListCronjobs(envId, client)`.
pub(crate) fn list_cronjobs(client: &Client, env_id: &str) -> Result<Vec<Job>, String> {
    Ok(list_jobs(client, env_id)?
        .into_iter()
        .filter(Job::is_cron)
        .collect())
}

/// `cronjob %s not found` + `You can list all cronjobs with: qovery cronjob list`, exit 1.
pub(crate) fn cronjob_not_found(name: &str) -> ! {
    qovery_log::println_error(format!("cronjob {name} not found"));
    qovery_log::println_info("You can list all cronjobs with: qovery cronjob list");
    exit(1)
}

/// `FindByJobName` + `cronjob == nil || cronjob.CronJobResponse == nil` check of the Go commands.
pub(crate) fn find_cronjob_or_exit(jobs: &[Job], name: &str) -> Job {
    match find_by_job_name(jobs, name).filter(|j| j.is_cron()) {
        Some(j) => j.clone(),
        None => cronjob_not_found(name),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn find_by_job_name_like_go() {
        let jobs: Vec<Job> = serde_json::from_value(json!([
            {"id": "x", "name": "a", "job_type": "OTHER"},
            {"id": "l", "name": "a", "job_type": "LIFECYCLE"},
            {"id": "c", "name": " b ", "job_type": "CronJobResponse"},
        ]))
        .unwrap();
        assert_eq!(find_by_job_name(&jobs, "a").unwrap().id, "l");
        assert_eq!(find_by_job_name(&jobs, "b\u{a0}").unwrap().id, "c");
        assert!(find_by_job_name(&jobs, "b").unwrap().is_cron());
        assert!(find_by_job_name(&jobs, "z").is_none());
    }
}
