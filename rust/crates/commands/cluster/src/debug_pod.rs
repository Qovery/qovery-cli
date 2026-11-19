//! `cluster debug-pod` (cmd/cluster_debug_pod.go).

use crate::util::encode_query;
use qovery_cobra::{Command, Ctx, Flag};
use qovery_core::{client, resources, telemetry};

pub fn command() -> Command {
    Command::new("debug-pod")
        .short("Launch a debug pod and attach to it")
        .flag(Flag::string("organization-id", "Organization ID").short('o'))
        .flag(
            Flag::string("cluster-id", "Cluster ID")
                .short('c')
                .required(),
        )
        .flag(
            Flag::string(
                "node-selector",
                "Specify a node selector for the debug pod to be started on",
            )
            .short('n'),
        )
        .flag(
            Flag::bool(
                "full-privilege",
                "Start a full privileged debug pod which has access to host machine. ",
            )
            .short('p'),
        )
        .run(run)
}

/// `DebugPodRequest` encoded with go-querystring (`url.Values.Encode()`, keys sorted).
pub fn debug_pod_query(
    organization_id: &str,
    cluster_id: &str,
    flavor: &str,
    node_selector: &str,
    width: u16,
    height: u16,
) -> String {
    let mut pairs = vec![
        ("organization", organization_id.to_string()),
        ("cluster", cluster_id.to_string()),
        ("tty_width", width.to_string()),
        ("tty_height", height.to_string()),
        ("flavor", flavor.to_string()),
    ];
    if !node_selector.is_empty() {
        pairs.push(("node_selector", node_selector.to_string()));
    }
    encode_query(&pairs)
}

fn run(ctx: &Ctx) {
    telemetry::capture(ctx);
    let client = qovery_core::check_error(client::get_client());
    let mut organization_id = ctx.string("organization-id");
    if organization_id.is_empty() {
        organization_id = qovery_core::check_error(resources::organization_id(
            &client,
            &ctx.string("organization"),
        ));
    }
    let flavor = if ctx.bool("full-privilege") {
        "FULL_PRIVILEGE"
    } else {
        "REGULAR_PRIVILEGE"
    };
    let cluster_id = ctx.string("cluster-id");
    let node_selector = ctx.string("node-selector");
    crate::shell::exec_shell(
        &|w, h| debug_pod_query(&organization_id, &cluster_id, flavor, &node_selector, w, h),
        "/shell/debug",
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn query_like_go_querystring() {
        assert_eq!(
            debug_pod_query("o", "c", "REGULAR_PRIVILEGE", "", 0, 0),
            "cluster=c&flavor=REGULAR_PRIVILEGE&organization=o&tty_height=0&tty_width=0"
        );
        assert_eq!(
            debug_pod_query("o", "c", "FULL_PRIVILEGE", "a=b", 80, 24),
            "cluster=c&flavor=FULL_PRIVILEGE&node_selector=a%3Db&organization=o&tty_height=24&tty_width=80"
        );
    }
}
