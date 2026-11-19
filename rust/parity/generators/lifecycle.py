#!/usr/bin/env python3
"""Generates parity/scenarios/lifecycle.json.

Usage: python3 parity/generators/lifecycle.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import copy
import json

default = json.load(open(os.path.join(PARITY, "fixtures", "default.json")))


def default_body(path):
    for r in default:
        if r.get("path") == path:
            return copy.deepcopy(r["body"])
    raise KeyError(path)


jobs = default_body("/environment/env-1/job")["results"]
cron = [j for j in jobs if j["id"] == "cron-1"][0]
life = [j for j in jobs if j["id"] == "life-1"][0]

life_docker = copy.deepcopy(life)
life_docker.update({
    "id": "life-2",
    "name": "my-lifecycle-docker",
    "updated_at": "2024-07-03T08:15:30.5Z",
    "source": {"docker": {
        "git_repository": {"provider": "GITHUB", "owner": "me", "url": "https://github.com/me/life.git", "name": "me/life",
                           "branch": "main", "root_path": "/", "deployed_commit_id": "abc123", "has_access": True},
        "dockerfile_path": "Dockerfile"}},
    "schedule": {"on_start": {"arguments": ["start"], "entrypoint": "/bin/sh"},
                 "on_stop": {"arguments": []}, "lifecycle_type": "GENERIC"},
    "auto_deploy": True,
    "port": 8080,
})

JOBS = {"results": [cron, life, life_docker]}
STATUS_LIFE2 = {"id": "life-2", "state": "STOPPED", "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": False,
                "status_details": {"action": "STOP", "status": "SUCCESS", "sub_action": "NONE"}, "deployment_request_id": None,
                "deployment_requests_count": 0, "last_deployment_date": "2024-06-02T11:30:00Z", "execution_id": "exec-life-2", "steps": None}
statuses = default_body("/environment/env-1/statuses")
statuses["jobs"].append(STATUS_LIFE2)

ENV_STATUS = {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYED"}


def svc_status(sid, state, execution_id):
    return {"id": sid, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"}, "deployment_request_id": None,
            "deployment_requests_count": 0, "execution_id": execution_id}


def with_stages(job_states):
    return {"body": {"environment": {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED",
                                     "last_deployment_date": "2024-06-02T11:30:00Z", "last_deployment_id": "dep-1",
                                     "total_deployment_duration_in_seconds": 42},
                     "stages": [{"jobs": [svc_status(sid, st, ex) for sid, st, ex in job_states]}]}}


def watch_fixture(seq):
    return {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [with_stages(s) for s in seq]}


def var(vid, key, value, scope="JOB", vtype="VALUE", secret=False, **extra):
    v = {"id": vid, "created_at": "2024-01-02T15:04:05.123456Z", "updated_at": "2024-03-04T05:06:07Z", "key": key, "value": value,
         "mount_path": None, "scope": scope, "variable_type": vtype, "is_secret": secret}
    v.update(extra)
    return v


VARIABLES = {"results": [
    var("var-1", "FOO", "bar", service_id="life-1", service_name="my-lifecycle", service_type="JOB"),
    var("var-2", "API_KEY", None, secret=True, service_id="life-1", service_name="my-lifecycle", service_type="JOB"),
    var("var-3", "DB_HOST", "db.local", scope="ENVIRONMENT"),
    var("var-4", "ALIAS_HOST", "db.local", vtype="ALIAS",
        aliased_variable={"id": "var-3", "key": "DB_HOST", "value": "db.local", "mount_path": "", "scope": "ENVIRONMENT", "variable_type": "VALUE"}),
    var("var-5", "PROJECT_VAR", "p", scope="PROJECT"),
    var("var-6", "EXT", "/path/in/sm", vtype="EXTERNAL_SECRET", service_id="life-1", service_name="my-lifecycle", service_type="JOB"),
]}
VAR_QUERY = {"parent_id": ["life-1"], "scope": ["JOB"]}
VARIABLES_ROUTE = {"method": "GET", "path": "/variable", "query": VAR_QUERY, "body": VARIABLES}

CLUSTERS = {"results": [dict(default_body("/organization/org-1/cluster")["results"][0], secret_manager_accesses=[
    {"id": "sma-1", "name": "my-sm", "created_at": "2024-05-01T10:00:00Z", "updated_at": "2024-05-01T10:00:00Z",
     "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"}, "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}])]}

DEPLOY_OK = {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202, "body": ENV_STATUS}
STOP_OK = {"method": "POST", "path": "/environment/env-1/service/stop", "status": 202, "body": ENV_STATUS}
DELETE_OK = {"method": "POST", "path": "/environment/env-1/service/delete", "status": 202, "body": ENV_STATUS}
REDEPLOY_OK = {"method": "POST", "path": "/job/life-1/deploy", "status": 202, "body": svc_status("life-1", "DEPLOYING", "exec-new")}

COMMITS = {"results": [
    {"created_at": "2024-06-01T10:00:00Z", "git_commit_id": "old111", "tag": "", "message": "old commit", "author_name": "me"},
    {"created_at": "2024-06-03T10:00:00Z", "git_commit_id": "new999", "tag": "", "message": "newest commit\n\nbody", "author_name": "me"},
]}

cloned = copy.deepcopy(life)
cloned.update({"id": "life-9", "name": "my-lifecycle-clone"})
updated = copy.deepcopy(life)

ERR400 = {"status": 400, "body": {"status": 400, "error": "Bad Request", "message": "invalid payload"}}
ERR500 = {"status": 500, "body": {"message": "boom"}}

cases = []


def case(name, args, fixtures=None, **kw):
    c = {"name": name, "args": args}
    if fixtures:
        c["fixtures"] = fixtures
    c.update(kw)
    cases.append(c)


def r(method, path, resp=None, **kw):
    route = {"method": method, "path": path}
    route.update(resp or {"status": 200, "body": {}})
    route.update(kw)
    return route


# ---- groups / cobra -------------------------------------------------------------------------------------
case("lifecycle (group)", ["lifecycle"])
case("lifecycle env (group)", ["lifecycle", "env"])
case("lifecycle env alias (group)", ["lifecycle", "env", "alias"])
case("lifecycle env override (group)", ["lifecycle", "env", "override"])
case("lifecycle external-secret (group)", ["lifecycle", "external-secret"])
case("lifecycle unknown subcommand", ["lifecycle", "nope"])
case("lifecycle list unknown flag", ["lifecycle", "list", "--nope"])

# ---- list -----------------------------------------------------------------------------------------------
case("list", ["lifecycle", "list"])
case("list --json", ["lifecycle", "list", "--json"])
case("list with environment flags", ["lifecycle", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
case("list environment not found", ["lifecycle", "list", "--organization", "my-org", "--project", "my-project", "--environment", "nope"])
case("list empty", ["lifecycle", "list"], [r("GET", "/environment/env-1/job", {"body": {"results": [cron]}})])
case("list empty --json", ["lifecycle", "list", "--json"], [r("GET", "/environment/env-1/job", {"body": {"results": [cron]}})])
case("list jobs api error", ["lifecycle", "list"], [r("GET", "/environment/env-1/job", ERR500)])
case("list statuses api error", ["lifecycle", "list", "--json"], [r("GET", "/environment/env-1/statuses", ERR500)])
case("list no token", ["lifecycle", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# ---- deploy ---------------------------------------------------------------------------------------------
case("deploy image lifecycle", ["lifecycle", "deploy", "-n", "my-lifecycle"], [DEPLOY_OK])
case("deploy --tag", ["lifecycle", "deploy", "-n", "my-lifecycle", "--tag", "v2"], [DEPLOY_OK])
case("deploy docker lifecycle", ["lifecycle", "deploy", "-n", "my-lifecycle-docker"], [DEPLOY_OK])
case("deploy docker --commit-id", ["lifecycle", "deploy", "-n", "my-lifecycle-docker", "-c", "c0ffee"], [DEPLOY_OK])
case("deploy docker --commit-id latest", ["lifecycle", "deploy", "-n", "my-lifecycle-docker", "--commit-id", "latest"],
     [DEPLOY_OK, r("GET", "/job/life-2/commit", {"body": COMMITS})])
case("deploy --commit-id latest no commit", ["lifecycle", "deploy", "-n", "my-lifecycle-docker", "--commit-id", "LATEST"],
     [DEPLOY_OK, r("GET", "/job/life-2/commit", {"body": {"results": []}})])
case("deploy --commit-id latest commits api error", ["lifecycle", "deploy", "-n", "my-lifecycle-docker", "--commit-id", "latest"],
     [DEPLOY_OK, r("GET", "/job/life-2/commit", ERR500)])
case("deploy --lifecycles", ["lifecycle", "deploy", "--lifecycles", "my-lifecycle, my-lifecycle-docker", "-t", "v3"], [DEPLOY_OK])
case("deploy --lifecycles --commit-id", ["lifecycle", "deploy", "--lifecycles", "my-lifecycle,my-lifecycle-docker", "-c", "c0ffee"], [DEPLOY_OK])
case("deploy --tag and --commit-id", ["lifecycle", "deploy", "-n", "my-lifecycle", "--tag", "v2", "--commit-id", "abc"])
case("deploy no lifecycle flag", ["lifecycle", "deploy"])
case("deploy both lifecycle flags", ["lifecycle", "deploy", "-n", "my-lifecycle", "--lifecycles", "my-lifecycle"])
case("deploy not found", ["lifecycle", "deploy", "-n", "nope"])
case("deploy cron is not a lifecycle", ["lifecycle", "deploy", "-n", "my-cron"])
case("deploy --lifecycles one not found", ["lifecycle", "deploy", "--lifecycles", "my-lifecycle, nope"])
case("deploy jobs api error", ["lifecycle", "deploy", "-n", "my-lifecycle"], [r("GET", "/environment/env-1/job", ERR500)])
case("deploy api error", ["lifecycle", "deploy", "-n", "my-lifecycle"], [r("POST", "/environment/env-1/service/deploy", ERR400)])
case("deploy environment not found", ["lifecycle", "deploy", "-n", "my-lifecycle", "--organization", "my-org", "--project", "my-project", "--environment", "nope"])
case("deploy no token", ["lifecycle", "deploy", "-n", "my-lifecycle"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
case("deploy --watch", ["lifecycle", "deploy", "-n", "my-lifecycle", "-w"],
     [DEPLOY_OK, watch_fixture([[("life-1", "DEPLOYED", "exec-old")], [("life-1", "DEPLOYING", "exec-new")], [("life-1", "DEPLOYED", "exec-new")]])],
     timeout=60)
case("deploy --watch failure", ["lifecycle", "deploy", "--lifecycles", "my-lifecycle,my-lifecycle-docker", "--watch"],
     [DEPLOY_OK, watch_fixture([[("life-1", "DEPLOYED", "exec-old"), ("life-2", "STOPPED", "exec-old2")],
                                [("life-1", "DEPLOYED", "exec-new"), ("life-2", "DEPLOYMENT_ERROR", "exec-new")]])],
     timeout=60)
case("deploy --watch statuses error", ["lifecycle", "deploy", "-n", "my-lifecycle", "-w"],
     [DEPLOY_OK, r("GET", "/environment/env-1/statusesWithStages", ERR500)], timeout=60,
     env={"QOVERY_CLI_WATCH_TIMEOUT": "1s"})

# ---- redeploy -------------------------------------------------------------------------------------------
case("redeploy", ["lifecycle", "redeploy", "-n", "my-lifecycle"], [REDEPLOY_OK])
case("redeploy missing lifecycle flag", ["lifecycle", "redeploy"])
case("redeploy not found", ["lifecycle", "redeploy", "-n", "nope"])
case("redeploy api error", ["lifecycle", "redeploy", "-n", "my-lifecycle"], [r("POST", "/job/life-1/deploy", ERR500)])
case("redeploy --watch", ["lifecycle", "redeploy", "-n", "my-lifecycle", "--watch"],
     [REDEPLOY_OK, watch_fixture([[("life-1", "DEPLOYED", "exec-old")], [("life-1", "DEPLOYED", "exec-new")]])], timeout=60)

# ---- stop -----------------------------------------------------------------------------------------------
case("stop", ["lifecycle", "stop", "-n", "my-lifecycle"], [STOP_OK])
case("stop --lifecycles", ["lifecycle", "stop", "--lifecycles", "my-lifecycle-docker, my-lifecycle"], [STOP_OK])
case("stop no lifecycle flag", ["lifecycle", "stop"])
case("stop not found", ["lifecycle", "stop", "--lifecycles", "my-lifecycle,my-cron"])
case("stop api error", ["lifecycle", "stop", "-n", "my-lifecycle"], [r("POST", "/environment/env-1/service/stop", ERR500)])
case("stop --watch", ["lifecycle", "stop", "-n", "my-lifecycle", "-w"],
     [STOP_OK, watch_fixture([[("life-1", "DEPLOYED", "exec-old")], [("life-1", "STOPPING", "exec-new")], [("life-1", "STOPPED", "exec-new")]])],
     timeout=60)

# ---- delete ---------------------------------------------------------------------------------------------
case("delete", ["lifecycle", "delete", "-n", "my-lifecycle"], [DELETE_OK])
case("delete --lifecycles", ["lifecycle", "delete", "--lifecycles", "my-lifecycle,my-lifecycle-docker"], [DELETE_OK])
case("delete both flags", ["lifecycle", "delete", "-n", "a", "--lifecycles", "b"])
case("delete not found", ["lifecycle", "delete", "-n", "nope"])
case("delete api error", ["lifecycle", "delete", "-n", "my-lifecycle"], [r("POST", "/environment/env-1/service/delete", ERR400)])
case("delete --watch", ["lifecycle", "delete", "-n", "my-lifecycle", "--watch"],
     [DELETE_OK, watch_fixture([[("life-1", "DEPLOYED", "exec-old")], [("life-1", "DELETED", "exec-new")]])], timeout=60)

# ---- cancel ---------------------------------------------------------------------------------------------
deploying = copy.deepcopy(statuses)
deploying["environment"]["state"] = "DEPLOYING"
deploying["jobs"][1]["state"] = "DEPLOYING"
waiting = copy.deepcopy(statuses)
waiting["environment"]["state"] = "DEPLOYING"
waiting["jobs"][1]["state"] = "DEPLOYED"
CANCEL_OK = r("POST", "/environment/env-1/cancelDeployment", {"status": 200, "body": dict(ENV_STATUS, state="CANCELING")})
case("cancel nothing to cancel", ["lifecycle", "cancel", "-n", "my-lifecycle"])
case("cancel", ["lifecycle", "cancel", "-n", "my-lifecycle"], [r("GET", "/environment/env-1/statuses", {"body": deploying}), CANCEL_OK])
case("cancel waits then nothing to cancel", ["lifecycle", "cancel", "-n", "my-lifecycle"],
     [r("GET", "/environment/env-1/statuses", {"responses": [{"body": waiting}, {"body": statuses}]})], timeout=60)
case("cancel api error", ["lifecycle", "cancel", "-n", "my-lifecycle"],
     [r("GET", "/environment/env-1/statuses", {"body": deploying}), r("POST", "/environment/env-1/cancelDeployment", ERR500)])
case("cancel statuses api error", ["lifecycle", "cancel", "-n", "my-lifecycle"], [r("GET", "/environment/env-1/statuses", ERR500)])
canceled = copy.deepcopy(statuses)
canceled["environment"]["last_deployment_state"] = "CANCELED"
case("cancel --watch", ["lifecycle", "cancel", "-n", "my-lifecycle", "-w"],
     [r("GET", "/environment/env-1/statuses", {"responses": [{"body": deploying}, {"body": deploying}, {"body": canceled}]}), CANCEL_OK],
     timeout=60)
case("cancel missing flag", ["lifecycle", "cancel"])
case("cancel not found", ["lifecycle", "cancel", "-n", "my-cron"])
case("cancel jobs api error", ["lifecycle", "cancel", "-n", "my-lifecycle"], [r("GET", "/environment/env-1/job", ERR500)])

# ---- clone ----------------------------------------------------------------------------------------------
CLONE_OK = r("POST", "/job/life-1/clone", {"status": 201, "body": cloned})
case("clone", ["lifecycle", "clone", "-n", "my-lifecycle"], [CLONE_OK])
case("clone with targets", ["lifecycle", "clone", "-n", "my-lifecycle", "--target-project", "my-project",
                            "--target-environment", "my-env", "--target-lifecycle-name", "my-lifecycle-clone"], [CLONE_OK])
case("clone target project not found", ["lifecycle", "clone", "-n", "my-lifecycle", "--target-project", "nope"])
case("clone target environment not found", ["lifecycle", "clone", "-n", "my-lifecycle", "--target-environment", "nope"])
case("clone not found", ["lifecycle", "clone", "-n", "nope"])
case("clone cron", ["lifecycle", "clone", "-n", "my-cron"])
case("clone jobs api error", ["lifecycle", "clone", "-n", "my-lifecycle"], [r("GET", "/environment/env-1/job", ERR500)])
case("clone api error", ["lifecycle", "clone", "-n", "my-lifecycle"], [r("POST", "/job/life-1/clone", ERR400)])
case("clone missing flag", ["lifecycle", "clone"])

# ---- update ---------------------------------------------------------------------------------------------
UPDATE1 = r("PUT", "/job/life-1", {"body": updated})
UPDATE2 = r("PUT", "/job/life-2", {"body": life_docker})
case("update --tag", ["lifecycle", "update", "-n", "my-lifecycle", "--tag", "v2"], [UPDATE1])
case("update --image-name", ["lifecycle", "update", "-n", "my-lifecycle", "--image-name", "nginx"], [UPDATE1])
case("update --tag --image-name", ["lifecycle", "update", "-n", "my-lifecycle", "-t", "1.0", "--image-name", "nginx"], [UPDATE1])
case("update --branch", ["lifecycle", "update", "-n", "my-lifecycle-docker", "-b", "dev"], [UPDATE2])
case("update --tag and --branch", ["lifecycle", "update", "-n", "my-lifecycle", "--tag", "v2", "--branch", "dev"])
case("update nothing", ["lifecycle", "update", "-n", "my-lifecycle"])
case("update docker with --tag", ["lifecycle", "update", "-n", "my-lifecycle-docker", "--tag", "v2"])
case("update image with --branch", ["lifecycle", "update", "-n", "my-lifecycle", "--branch", "dev"])
case("update not found", ["lifecycle", "update", "-n", "my-cron", "--tag", "v2"])
case("update api error", ["lifecycle", "update", "-n", "my-lifecycle", "--tag", "v2"], [r("PUT", "/job/life-1", ERR400)])
case("update no token", ["lifecycle", "update", "-n", "my-lifecycle"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# ---- env ------------------------------------------------------------------------------------------------
# without --sort the Go table order comes from a map iteration (random): use a single variable
ONE_VARIABLE_ROUTE = {"method": "GET", "path": "/variable", "query": VAR_QUERY, "body": {"results": [VARIABLES["results"][0]]}}
case("env list", ["lifecycle", "env", "list", "-n", "my-lifecycle"], [ONE_VARIABLE_ROUTE])
case("env list --show-values", ["lifecycle", "env", "list", "-n", "my-lifecycle", "--show-values"], [ONE_VARIABLE_ROUTE])
case("env list --sort", ["lifecycle", "env", "list", "-n", "my-lifecycle", "--sort"], [VARIABLES_ROUTE])
case("env list --show-values --sort", ["lifecycle", "env", "list", "-n", "my-lifecycle", "--show-values", "--sort"], [VARIABLES_ROUTE])
case("env list --pretty-print --sort", ["lifecycle", "env", "list", "-n", "my-lifecycle", "--pretty-print", "--sort"], [VARIABLES_ROUTE])
case("env list --json", ["lifecycle", "env", "list", "-n", "my-lifecycle", "--json"], [VARIABLES_ROUTE])
case("env list --json --sort", ["lifecycle", "env", "list", "-n", "my-lifecycle", "--json", "--sort"], [VARIABLES_ROUTE])
case("env list not found", ["lifecycle", "env", "list", "-n", "nope"])
case("env list missing flag", ["lifecycle", "env", "list"])
case("env list api error", ["lifecycle", "env", "list", "-n", "my-lifecycle"], [r("GET", "/variable", ERR500)])

case("env create", ["lifecycle", "env", "create", "-n", "my-lifecycle", "-k", "NEW", "-v", "val"], [r("POST", "/variable", {"status": 201, "body": var("var-9", "NEW", "val")})])
case("env create secret environment scope", ["lifecycle", "env", "create", "-n", "my-lifecycle", "-k", "NEW", "-v", "val", "--secret", "--scope", "ENVIRONMENT"],
     [r("POST", "/variable", {"status": 201, "body": var("var-9", "NEW", "val")})])
case("env create project scope override", ["lifecycle", "env", "create", "-n", "my-lifecycle", "-k", "NEW", "-v", "val", "--scope", "PROJECT", "--override-if-exists"],
     [r("POST", "/variable", {"status": 201, "body": var("var-9", "NEW", "val")})])
case("env create bad scope", ["lifecycle", "env", "create", "-n", "my-lifecycle", "-k", "NEW", "-v", "val", "--scope", "NOPE"])
case("env create api error", ["lifecycle", "env", "create", "-n", "my-lifecycle", "-k", "NEW", "-v", "val"], [r("POST", "/variable", ERR400)])
case("env create missing flags", ["lifecycle", "env", "create", "-n", "my-lifecycle"])
case("env create not found", ["lifecycle", "env", "create", "-n", "nope", "-k", "A", "-v", "b"])

case("env delete", ["lifecycle", "env", "delete", "-n", "my-lifecycle", "-k", "FOO"], [VARIABLES_ROUTE, r("DELETE", "/variable/var-1", {"status": 204})])
case("env delete key not found", ["lifecycle", "env", "delete", "-n", "my-lifecycle", "-k", "NOPE"], [VARIABLES_ROUTE])
case("env delete api error", ["lifecycle", "env", "delete", "-n", "my-lifecycle", "-k", "FOO"], [VARIABLES_ROUTE, r("DELETE", "/variable/var-1", ERR500)])
case("env delete missing flags", ["lifecycle", "env", "delete"])

case("env update", ["lifecycle", "env", "update", "-n", "my-lifecycle", "-k", "FOO", "-v", "baz"], [VARIABLES_ROUTE, r("PUT", "/variable/var-1", {"body": var("var-1", "FOO", "baz")})])
case("env update key not found", ["lifecycle", "env", "update", "-n", "my-lifecycle", "-k", "NOPE", "-v", "baz"], [VARIABLES_ROUTE])
case("env update api error", ["lifecycle", "env", "update", "-n", "my-lifecycle", "-k", "FOO", "-v", "baz"], [VARIABLES_ROUTE, r("PUT", "/variable/var-1", ERR400)])
case("env update missing value", ["lifecycle", "env", "update", "-n", "my-lifecycle", "-k", "FOO"])

ALIAS_BODY = var("var-10", "MY_ALIAS", "db.local", vtype="ALIAS")
case("env alias create", ["lifecycle", "env", "alias", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "--alias", "MY_ALIAS"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-3/alias", {"status": 201, "body": ALIAS_BODY})])
case("env alias create environment scope", ["lifecycle", "env", "alias", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "--alias", "MY_ALIAS", "--scope", "ENVIRONMENT"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-3/alias", {"status": 201, "body": ALIAS_BODY})])
case("env alias create project scope", ["lifecycle", "env", "alias", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "--alias", "MY_ALIAS", "--scope", "PROJECT"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-3/alias", {"status": 201, "body": ALIAS_BODY})])
case("env alias create key not found", ["lifecycle", "env", "alias", "create", "-n", "my-lifecycle", "-k", "NOPE", "--alias", "MY_ALIAS"], [VARIABLES_ROUTE])
case("env alias create api error", ["lifecycle", "env", "alias", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "--alias", "MY_ALIAS"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-3/alias", ERR400)])
case("env alias create missing alias", ["lifecycle", "env", "alias", "create", "-n", "my-lifecycle", "-k", "DB_HOST"])

OVERRIDE_BODY = var("var-11", "DB_HOST", "other", vtype="OVERRIDE")
case("env override create", ["lifecycle", "env", "override", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "--value", "other"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-3/override", {"status": 201, "body": OVERRIDE_BODY})])
case("env override create environment scope", ["lifecycle", "env", "override", "create", "-n", "my-lifecycle", "-k", "PROJECT_VAR", "--value", "other", "--scope", "ENVIRONMENT"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-5/override", {"status": 201, "body": OVERRIDE_BODY})])
case("env override create key not found", ["lifecycle", "env", "override", "create", "-n", "my-lifecycle", "-k", "NOPE", "--value", "x"], [VARIABLES_ROUTE])
case("env override create api error", ["lifecycle", "env", "override", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "--value", "other"],
     [VARIABLES_ROUTE, r("POST", "/variable/var-3/override", ERR500)])
case("env override create missing value", ["lifecycle", "env", "override", "create", "-n", "my-lifecycle", "-k", "DB_HOST", "-v", "x"])

# ---- external-secret ------------------------------------------------------------------------------------
CLUSTERS_ROUTE = r("GET", "/organization/org-1/cluster", {"body": CLUSTERS})
EXT_BODY = var("var-12", "EXT2", "/sm/ref", vtype="EXTERNAL_SECRET")
case("external-secret create", ["lifecycle", "external-secret", "create", "-n", "my-lifecycle", "-k", "EXT2", "-r", "/sm/ref", "--secret-manager-access-name", "my-sm"],
     [CLUSTERS_ROUTE, r("POST", "/variable", {"status": 201, "body": EXT_BODY})])
case("external-secret create mount path env scope", ["lifecycle", "external-secret", "create", "-n", "my-lifecycle", "-k", "EXT2", "-r", "/sm/ref",
                                                     "--secret-manager-access-name", "my-sm", "--mount-path", "/etc/secret", "--scope", "ENVIRONMENT"],
     [CLUSTERS_ROUTE, r("POST", "/variable", {"status": 201, "body": EXT_BODY})])
case("external-secret create sma not found", ["lifecycle", "external-secret", "create", "-n", "my-lifecycle", "-k", "EXT2", "-r", "/sm/ref", "--secret-manager-access-name", "nope"],
     [CLUSTERS_ROUTE])
case("external-secret create clusters api error", ["lifecycle", "external-secret", "create", "-n", "my-lifecycle", "-k", "EXT2", "-r", "/sm/ref", "--secret-manager-access-name", "my-sm"],
     [r("GET", "/organization/org-1/cluster", ERR500)])
case("external-secret create api error", ["lifecycle", "external-secret", "create", "-n", "my-lifecycle", "-k", "EXT2", "-r", "/sm/ref", "--secret-manager-access-name", "my-sm"],
     [CLUSTERS_ROUTE, r("POST", "/variable", ERR400)])
case("external-secret create missing flags", ["lifecycle", "external-secret", "create", "-n", "my-lifecycle", "-k", "EXT2"])
case("external-secret create not found", ["lifecycle", "external-secret", "create", "-n", "nope", "-k", "EXT2", "-r", "/sm/ref", "--secret-manager-access-name", "my-sm"])

case("external-secret delete", ["lifecycle", "external-secret", "delete", "-n", "my-lifecycle", "-k", "EXT"], [VARIABLES_ROUTE, r("DELETE", "/variable/var-6", {"status": 204})])
case("external-secret delete key not found", ["lifecycle", "external-secret", "delete", "-n", "my-lifecycle", "-k", "NOPE"], [VARIABLES_ROUTE])
case("external-secret delete missing flags", ["lifecycle", "external-secret", "delete", "-n", "my-lifecycle"])

case("external-secret update", ["lifecycle", "external-secret", "update", "-n", "my-lifecycle", "-k", "EXT", "-r", "/new/ref", "--secret-manager-access-name", "my-sm"],
     [CLUSTERS_ROUTE, VARIABLES_ROUTE, r("PUT", "/variable/var-6", {"body": EXT_BODY})])
case("external-secret update without sma name", ["lifecycle", "external-secret", "update", "-n", "my-lifecycle", "-k", "EXT", "-r", "/new/ref"],
     [CLUSTERS_ROUTE, VARIABLES_ROUTE, r("PUT", "/variable/var-6", {"body": EXT_BODY})])
case("external-secret update key not found", ["lifecycle", "external-secret", "update", "-n", "my-lifecycle", "-k", "NOPE", "-r", "/new/ref", "--secret-manager-access-name", "my-sm"],
     [CLUSTERS_ROUTE, VARIABLES_ROUTE])
case("external-secret update api error", ["lifecycle", "external-secret", "update", "-n", "my-lifecycle", "-k", "EXT", "--secret-manager-access-name", "my-sm"],
     [CLUSTERS_ROUTE, VARIABLES_ROUTE, r("PUT", "/variable/var-6", ERR400)])
case("external-secret update missing flags", ["lifecycle", "external-secret", "update", "-k", "EXT"])

scenario = {
    "fixtures": [
        {"method": "GET", "path": "/environment/env-1/job", "status": 200, "body": JOBS},
        {"method": "GET", "path": "/environment/env-1/statuses", "status": 200, "body": statuses},
    ],
    "cases": cases,
}
out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "lifecycle.json")
with open(out, "w") as f:
    json.dump(scenario, f, indent=1)
    f.write("\n")
print("%d cases -> %s" % (len(cases), out), file=sys.stderr)
