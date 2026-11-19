//! Status helpers (`utils.FindStatus`, `GetStatusTextWithColor`, ...).

use qovery_api::models::{EnvironmentStatus, Status};
use qovery_log::pterm;

/// `utils.FindStatus`.
pub fn find_status(statuses: &[Status], service_id: &str) -> String {
    statuses
        .iter()
        .find(|s| s.id == service_id)
        .map(|s| s.state.clone())
        .unwrap_or_else(|| "Unknown".to_string())
}

/// `utils.FindStatusTextWithColor`.
pub fn find_status_text_with_color(statuses: &[Status], service_id: &str) -> String {
    statuses
        .iter()
        .find(|s| s.id == service_id)
        .map(|s| status_text_with_color(&s.state))
        .unwrap_or_else(|| "Unknown".to_string())
}

/// `utils.GetEnvironmentStatus`.
pub fn environment_status(statuses: &[EnvironmentStatus], id: &str) -> String {
    statuses
        .iter()
        .find(|s| s.id == id)
        .map(|s| s.state.clone())
        .unwrap_or_else(|| "Unknown".to_string())
}

/// `utils.GetEnvironmentStatusWithColor`.
pub fn environment_status_with_color(statuses: &[EnvironmentStatus], id: &str) -> String {
    statuses
        .iter()
        .find(|s| s.id == id)
        .map(|s| status_text_with_color(&s.state))
        .unwrap_or_else(|| "Unknown".to_string())
}

/// `utils.GetStatusTextWithColor(StateEnum)`.
pub fn status_text_with_color(state: &str) -> String {
    if state == "DEPLOYED" || state == "RESTARTED" {
        pterm::FG_GREEN.sprint(state)
    } else if state.ends_with("ERROR") {
        pterm::FG_RED.sprint(state)
    } else if state.ends_with("ING") {
        pterm::FG_LIGHT_BLUE.sprint(state)
    } else if state.ends_with("QUEUED") {
        pterm::FG_LIGHT_YELLOW.sprint(state)
    } else if state == "READY" || state == "STOPPED" {
        pterm::FG_YELLOW.sprint(state)
    } else {
        state.to_string()
    }
}

/// `utils.GetClusterStatusTextWithColor(ClusterStateEnum)`.
pub fn cluster_status_text_with_color(state: &str) -> String {
    if state == "DEPLOYED" || state == "RESTARTED" {
        pterm::FG_GREEN.sprint(state)
    } else if state.ends_with("ERROR") || state == "INVALID_CREDENTIALS" {
        pterm::FG_RED.sprint(state)
    } else if state.ends_with("ING") {
        pterm::FG_LIGHT_BLUE.sprint(state)
    } else if state.ends_with("QUEUED") {
        pterm::FG_LIGHT_YELLOW.sprint(state)
    } else if state == "READY" || state == "STOPPED" {
        pterm::FG_YELLOW.sprint(state)
    } else {
        state.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn colors_match_go() {
        assert_eq!(
            status_text_with_color("BUILDING"),
            "\x1b[94mBUILDING\x1b[0m"
        );
        assert_eq!(
            status_text_with_color("DEPLOYED"),
            "\x1b[32mDEPLOYED\x1b[0m"
        );
        assert_eq!(
            status_text_with_color("DELETE_QUEUED"),
            "\x1b[93mDELETE_QUEUED\x1b[0m"
        );
        assert_eq!(status_text_with_color("STOPPED"), "\x1b[33mSTOPPED\x1b[0m");
        assert_eq!(status_text_with_color("CANCELED"), "CANCELED");
        assert_eq!(
            cluster_status_text_with_color("INVALID_CREDENTIALS"),
            "\x1b[31mINVALID_CREDENTIALS\x1b[0m"
        );
    }
}
