#!/usr/bin/env python3
"""Generates parity/scenarios/cronjob.json.

Usage: python3 parity/generators/cronjob.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import copy
import json

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "cronjob.json")
T = "2024-05-01T10:00:00.123456Z"

default = json.load(open(os.path.join(PARITY, "fixtures", "default.json")))
jobs_default = next(f for f in default if f["path"] == "/environment/env-1/job")["body"]["results"]
cron1 = copy.deepcopy(jobs_default[0])
life1 = copy.deepcopy(jobs_default[1])

cron2 = copy.deepcopy(cron1)
cron2.update({
    "id": "cron-2", "name": "my-image-cron", "updated_at": "2024-07-03T08:15:30.5+02:00",
    "description": "an image cron",
    "source": {"image": {"image_name": "busybox", "tag": "1.36", "registry_id": "reg-1",
                         "registry": {"id": "reg-1", "name": "Docker Hub", "url": "https://docker.io", "kind": "DOCKER_HUB"}}},
    "schedule": {"cronjob": {"scheduled_at": "*/5 * * * *", "arguments": ["-c", "echo hi"], "entrypoint": "/bin/sh", "timezone": "Europe/Paris"}},
    "auto_deploy": False, "max_nb_restart": 2, "port": 8080,
    "healthchecks": {"readiness_probe": None, "liveness_probe": {"type": {"tcp": {"port": 8080, "host": None}, "http": None},
                                                                  "initial_delay_seconds": 30, "period_seconds": 10,
                                                                  "timeout_seconds": 5, "success_threshold": 1, "failure_threshold": 3}},
})
cron1_full = copy.deepcopy(cron1)
cron1_full["source"]["docker"]["git_repository"]["git_token_id"] = None

JOBS = {"method": "GET", "path": "/environment/env-1/job", "status": 200, "body": {"results": [life1, cron1_full, cron2]}}


def status(id_, state, execution_id):
    return {"id": id_, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": execution_id}


ENV_STATUS = {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED",
              "last_deployment_date": "2024-06-02T11:30:00Z", "last_deployment_id": "dep-1",
              "total_deployment_duration_in_seconds": 42}


def with_stages(jobs):
    return {"body": {"environment": ENV_STATUS, "stages": [{"jobs": jobs}]}}


def stages_fixture(*responses):
    return {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": list(responses)}


def env_statuses(env_state, last_state, jobs):
    env = dict(ENV_STATUS, state=env_state, last_deployment_state=last_state)
    return {"status": 200, "body": {"environment": env, "applications": [], "containers": [], "jobs": jobs,
                                    "databases": [], "helms": [], "terraforms": []}}


def var(id_, key, value, scope="JOB", vtype="VALUE", secret=False, **extra):
    v = {"id": id_, "created_at": T, "updated_at": "2024-06-02T11:30:00Z", "key": key, "value": value, "scope": scope,
         "variable_type": vtype, "is_secret": secret}
    v.update(extra)
    return v


VARS = [
    var("var-1", "FOO", "bar"),
    var("var-2", "SECRET_KEY", None, secret=True),
    var("var-3", "ENV_LEVEL", "env-value", scope="ENVIRONMENT", description="from env"),
    var("var-4", "ALIAS_FOO", None, vtype="ALIAS",
        aliased_variable={"id": "var-1", "key": "FOO", "value": "bar", "mount_path": "", "scope": "JOB", "variable_type": "VALUE"}),
    var("var-5", "A_FILE", "content\nline2", vtype="FILE", mount_path="/etc/a"),
    var("var-6", "EXT", None, vtype="EXTERNAL_SECRET", secret=True, reference="arn:aws:secret", owned_by="QOVERY"),
]
VAR_LIST = {"method": "GET", "path": "/variable", "query": {"parent_id": "cron-1", "scope": "JOB"}, "status": 200,
            "body": {"results": VARS}}
VAR_CREATED = var("var-new", "NEW", "v")
POST_VAR = {"method": "POST", "path": "/variable", "status": 201, "body": VAR_CREATED}
PUT_VAR = {"method": "PUT", "path_regex": "^/variable/[^/]+$", "status": 200, "body": VAR_CREATED}
DEL_VAR = {"method": "DELETE", "path_regex": "^/variable/[^/]+$", "status": 204, "body_raw": ""}
ALIAS = {"method": "POST", "path_regex": "^/variable/[^/]+/alias$", "status": 201, "body": VAR_CREATED}
OVERRIDE = {"method": "POST", "path_regex": "^/variable/[^/]+/override$", "status": 201, "body": VAR_CREATED}
CLUSTERS = {"method": "GET", "path": "/organization/org-1/cluster", "status": 200, "body": {"results": [
    {"id": "cluster-1", "created_at": T, "updated_at": "2024-06-02T11:30:00Z", "organization": {"id": "org-1"},
     "name": "my-cluster", "region": "us-east-2", "cloud_provider": "AWS",
     "secret_manager_accesses": [
         {"id": "sma-1", "name": "my-sm", "created_at": T, "updated_at": T,
          "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"},
          "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}]}]}}

STATUS_BODY = status("cron-1", "QUEUED", "exec-cron-1")
DEPLOY_ALL = {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202, "body": ENV_STATUS}
STOP = {"method": "POST", "path": "/environment/env-1/service/stop", "status": 202, "body_raw": ""}
DELETE = {"method": "POST", "path": "/environment/env-1/service/delete", "status": 204, "body_raw": ""}
JOB_DEPLOY = {"method": "POST", "path": "/job/cron-1/deploy", "status": 202, "body": STATUS_BODY}
COMMITS = {"method": "GET", "path": "/job/cron-1/commit", "status": 200, "body": {"results": [
    {"created_at": "2024-06-01T10:00:00Z", "git_commit_id": "aaa111", "tag": "", "message": "old commit", "author_name": "me"},
    {"created_at": "2024-06-03T10:00:00Z", "git_commit_id": "bbb222", "tag": "", "message": "newest commit\nbody", "author_name": "me"},
]}}

cases = []


def case(name, args, fixtures=None, **kw):
    c = {"name": name, "args": args}
    if fixtures:
        c["fixtures"] = fixtures
    c.update(kw)
    cases.append(c)


def err(method, path, status, body=None):
    f = {"method": method, "path": path, "status": status}
    f["body"] = body if body is not None else {"message": "boom"}
    return f


# ---- groups ----
case("cronjob (group)", ["cronjob"])
case("cronjob env (group)", ["cronjob", "env"])
case("cronjob env alias (group)", ["cronjob", "env", "alias"])
case("cronjob env override (group)", ["cronjob", "env", "override"])
case("cronjob external-secret (group)", ["cronjob", "external-secret"])
case("cronjob unknown sub-command", ["cronjob", "nope"])

# ---- list ----
case("list", ["cronjob", "list"])
case("list --json", ["cronjob", "list", "--json"])
case("list default fixtures", ["cronjob", "list"], [{"method": "GET", "path": "/environment/env-1/job", "status": 200, "body": {"results": jobs_default}}])
case("list empty", ["cronjob", "list"], [{"method": "GET", "path": "/environment/env-1/job", "status": 200, "body": {"results": [life1]}}])
case("list empty --json", ["cronjob", "list", "--json"], [{"method": "GET", "path": "/environment/env-1/job", "status": 200, "body": {"results": []}}])
case("list jobs api error", ["cronjob", "list"], [err("GET", "/environment/env-1/job", 500)])
case("list statuses api error", ["cronjob", "list", "--json"], [err("GET", "/environment/env-1/statuses", 403)])
case("list with flags", ["cronjob", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
case("list unknown environment", ["cronjob", "list", "--environment", "nope"])
case("list unknown organization", ["cronjob", "list", "--organization", "nope"])
case("list no token", ["cronjob", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
case("list extra arg", ["cronjob", "list", "foo"])

# ---- deploy ----
case("deploy docker cron", ["cronjob", "deploy", "-n", "my-cron"], [DEPLOY_ALL])
case("deploy image cron --tag", ["cronjob", "deploy", "-n", "my-image-cron", "--tag", "2.0"], [DEPLOY_ALL])
case("deploy image cron no tag", ["cronjob", "deploy", "-n", "my-image-cron"], [DEPLOY_ALL])
case("deploy --commit-id", ["cronjob", "deploy", "-n", "my-cron", "-c", "abc123"], [DEPLOY_ALL])
case("deploy --commit-id latest", ["cronjob", "deploy", "-n", "my-cron", "--commit-id", "latest"], [DEPLOY_ALL, COMMITS])
case("deploy --commit-id latest no commits", ["cronjob", "deploy", "-n", "my-cron", "--commit-id", "LATEST"],
     [DEPLOY_ALL, {"method": "GET", "path": "/job/cron-1/commit", "status": 200, "body": {"results": []}}])
case("deploy --commit-id latest api error", ["cronjob", "deploy", "-n", "my-cron", "--commit-id", "latest"],
     [DEPLOY_ALL, err("GET", "/job/cron-1/commit", 500)])
case("deploy multi --cronjobs", ["cronjob", "deploy", "--cronjobs", "my-cron, my-image-cron", "-t", "3.1", "-c", ""], [DEPLOY_ALL])
case("deploy multi --cronjobs commit", ["cronjob", "deploy", "--cronjobs", "my-cron,my-image-cron", "-c", "c0ffee"], [DEPLOY_ALL])
case("deploy --tag and --commit-id", ["cronjob", "deploy", "-n", "my-cron", "-t", "1", "-c", "2"])
case("deploy no name", ["cronjob", "deploy"])
case("deploy --cronjob and --cronjobs", ["cronjob", "deploy", "-n", "my-cron", "--cronjobs", "my-image-cron"])
case("deploy unknown", ["cronjob", "deploy", "-n", "nope"])
case("deploy lifecycle is not a cron", ["cronjob", "deploy", "-n", "my-lifecycle"])
case("deploy --cronjobs one unknown", ["cronjob", "deploy", "--cronjobs", "my-cron, nope "])
case("deploy api error", ["cronjob", "deploy", "-n", "my-cron"], [err("POST", "/environment/env-1/service/deploy", 400, {"message": "bad"})])
case("deploy jobs api error", ["cronjob", "deploy", "-n", "my-cron"], [err("GET", "/environment/env-1/job", 500)])
case("deploy unknown environment", ["cronjob", "deploy", "-n", "my-cron", "--environment", "nope"])
case("deploy no token", ["cronjob", "deploy", "-n", "my-cron"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
case("deploy --watch success", ["cronjob", "deploy", "-n", "my-cron", "-w"], [DEPLOY_ALL, stages_fixture(
    with_stages([status("cron-1", "DEPLOYED", "exec-old"), status("cron-2", "DEPLOYED", "exec-2")]),
    with_stages([status("cron-1", "DEPLOYING", "exec-new"), status("cron-2", "DEPLOYED", "exec-2")]),
    with_stages([status("cron-1", "DEPLOYED", "exec-new"), status("cron-2", "DEPLOYED", "exec-2")]),
)], timeout=60)
case("deploy --watch multi error", ["cronjob", "deploy", "--cronjobs", "my-cron,my-image-cron", "--watch"], [DEPLOY_ALL, stages_fixture(
    with_stages([status("cron-1", "DEPLOYED", "exec-old"), status("cron-2", "DEPLOYED", "exec-2")]),
    with_stages([status("cron-1", "DEPLOYED", "exec-new"), status("cron-2", "DEPLOYMENT_ERROR", "exec-3")]),
)], timeout=60)
case("deploy --watch statuses error", ["cronjob", "deploy", "-n", "my-cron", "-w"],
     [DEPLOY_ALL, err("GET", "/environment/env-1/statusesWithStages", 500)], timeout=60)

# ---- redeploy ----
case("redeploy", ["cronjob", "redeploy", "-n", "my-cron"], [JOB_DEPLOY])
case("redeploy missing flag", ["cronjob", "redeploy"])
case("redeploy unknown", ["cronjob", "redeploy", "-n", "nope"])
case("redeploy api error", ["cronjob", "redeploy", "-n", "my-cron"], [err("POST", "/job/cron-1/deploy", 500)])
case("redeploy --watch", ["cronjob", "redeploy", "-n", "my-cron", "-w"], [JOB_DEPLOY, stages_fixture(
    with_stages([status("cron-1", "DEPLOYED", "exec-old")]),
    with_stages([status("cron-1", "DEPLOYED", "exec-new")]),
)], timeout=60)

# ---- stop ----
case("stop", ["cronjob", "stop", "-n", "my-cron"], [STOP])
case("stop multi", ["cronjob", "stop", "--cronjobs", "my-image-cron,my-cron"], [STOP])
case("stop no name", ["cronjob", "stop"])
case("stop unknown", ["cronjob", "stop", "--cronjobs", "nope"])
case("stop api error", ["cronjob", "stop", "-n", "my-cron"], [err("POST", "/environment/env-1/service/stop", 500)])
case("stop --watch", ["cronjob", "stop", "-n", "my-cron", "-w"], [STOP, stages_fixture(
    with_stages([status("cron-1", "DEPLOYED", "exec-old")]),
    with_stages([status("cron-1", "STOPPED", "exec-new")]),
)], timeout=60)
case("stop --watch wrong final state", ["cronjob", "stop", "-n", "my-cron", "-w"], [STOP, stages_fixture(
    with_stages([status("cron-1", "DEPLOYED", "exec-old")]),
    with_stages([status("cron-1", "DEPLOYED", "exec-new")]),
)], timeout=60)

# ---- delete ----
case("delete", ["cronjob", "delete", "-n", "my-cron"], [DELETE])
case("delete multi", ["cronjob", "delete", "--cronjobs", "my-cron,my-image-cron"], [DELETE])
case("delete both flags", ["cronjob", "delete", "-n", "a", "--cronjobs", "b"])
case("delete unknown", ["cronjob", "delete", "-n", "nope"])
case("delete api error", ["cronjob", "delete", "-n", "my-cron"], [err("POST", "/environment/env-1/service/delete", 404)])
case("delete --watch", ["cronjob", "delete", "-n", "my-cron", "-w"], [DELETE, stages_fixture(
    with_stages([status("cron-1", "DEPLOYED", "exec-old"), status("cron-2", "DEPLOYED", "exec-2")]),
    with_stages([status("cron-1", "DELETING", "exec-new"), status("cron-2", "DEPLOYED", "exec-2")]),
    with_stages([status("cron-2", "DEPLOYED", "exec-2")]),
)], timeout=60)

# ---- cancel ----
CANCEL = {"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200, "body": ENV_STATUS}
case("cancel nothing to cancel", ["cronjob", "cancel", "-n", "my-cron"])
case("cancel", ["cronjob", "cancel", "-n", "my-cron"], [CANCEL, {"method": "GET", "path": "/environment/env-1/statuses", **env_statuses(
    "DEPLOYING", "DEPLOYING", [status("cron-1", "DEPLOYING", "exec-1")])}])
case("cancel --watch", ["cronjob", "cancel", "-n", "my-cron", "--watch"], [CANCEL, {"method": "GET", "path": "/environment/env-1/statuses", "responses": [
    env_statuses("DEPLOYING", "DEPLOYING", [status("cron-1", "DEPLOYING", "exec-1")]),
    env_statuses("CANCELING", "CANCELING", [status("cron-1", "CANCELING", "exec-1")]),
    env_statuses("CANCELED", "CANCELED", [status("cron-1", "CANCELED", "exec-1")]),
]}], timeout=60)
case("cancel api error", ["cronjob", "cancel", "-n", "my-cron"], [err("POST", "/environment/env-1/cancelDeployment", 400),
     {"method": "GET", "path": "/environment/env-1/statuses", **env_statuses("DEPLOYING", "DEPLOYING", [status("cron-1", "DEPLOYING", "exec-1")])}])
case("cancel statuses error", ["cronjob", "cancel", "-n", "my-cron"], [err("GET", "/environment/env-1/statuses", 500)])
case("cancel missing flag", ["cronjob", "cancel"])
case("cancel unknown", ["cronjob", "cancel", "-n", "nope"])
case("cancel lifecycle", ["cronjob", "cancel", "-n", "my-lifecycle"])

# ---- clone ----
cloned = copy.deepcopy(cron1_full)
cloned.update({"id": "cron-9", "name": "my-cron-copy"})
CLONE = {"method": "POST", "path": "/job/cron-1/clone", "status": 201, "body": cloned}
case("clone", ["cronjob", "clone", "-n", "my-cron"], [CLONE])
case("clone with targets", ["cronjob", "clone", "-n", "my-cron", "--target-project", "my-project", "--target-environment",
                            "my-env", "--target-cronjob-name", "my-cron-copy"], [CLONE])
case("clone unknown", ["cronjob", "clone", "-n", "nope"])
case("clone lifecycle", ["cronjob", "clone", "-n", "my-lifecycle"])
case("clone jobs api error", ["cronjob", "clone", "-n", "my-cron"], [err("GET", "/environment/env-1/job", 500)])
case("clone unknown target project", ["cronjob", "clone", "-n", "my-cron", "--target-project", "nope"])
case("clone unknown target environment", ["cronjob", "clone", "-n", "my-cron", "--target-environment", "nope"])
case("clone api error", ["cronjob", "clone", "-n", "my-cron"], [err("POST", "/job/cron-1/clone", 400, {"message": "name already used"})])
case("clone missing flag", ["cronjob", "clone"])

# ---- update ----
PUT_JOB = {"method": "PUT", "path": "/job/cron-1", "status": 200, "body": cron1_full}
PUT_JOB2 = {"method": "PUT", "path": "/job/cron-2", "status": 200, "body": cron2}
case("update docker --branch", ["cronjob", "update", "-n", "my-cron", "--branch", "dev"], [PUT_JOB])
case("update image --tag --image-name", ["cronjob", "update", "-n", "my-image-cron", "-t", "2.0", "--image-name", "alpine"], [PUT_JOB2])
case("update image --tag", ["cronjob", "update", "-n", "my-image-cron", "--tag", "2.1"], [PUT_JOB2])
case("update image --image-name", ["cronjob", "update", "-n", "my-image-cron", "--image-name", "alpine"], [PUT_JOB2])
case("update docker with --tag", ["cronjob", "update", "-n", "my-cron", "--tag", "1"])
case("update image with --branch", ["cronjob", "update", "-n", "my-image-cron", "-b", "dev"])
case("update no source flag", ["cronjob", "update", "-n", "my-cron"])
case("update --tag and --branch", ["cronjob", "update", "-n", "my-cron", "--tag", "1", "--branch", "dev"])
case("update unknown", ["cronjob", "update", "-n", "nope", "--branch", "dev"])
case("update lifecycle", ["cronjob", "update", "-n", "my-lifecycle", "--tag", "1"])
case("update api error", ["cronjob", "update", "-n", "my-cron", "--branch", "dev"], [err("PUT", "/job/cron-1", 400, {"message": "invalid branch"})])
case("update no token", ["cronjob", "update", "-n", "my-cron", "--branch", "x"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# ---- env ----
ENVF = [VAR_LIST, POST_VAR, PUT_VAR, DEL_VAR, ALIAS, OVERRIDE]
# without --sort, Go iterates over a map (random order): unsorted tables use a single variable
ONE_VAR = [{"method": "GET", "path": "/variable", "status": 200, "body": {"results": [VARS[0]]}}]
case("env list", ["cronjob", "env", "list", "-n", "my-cron"], ONE_VAR)
case("env list --show-values", ["cronjob", "env", "list", "-n", "my-cron", "--show-values"], ONE_VAR)
case("env list --sort", ["cronjob", "env", "list", "-n", "my-cron", "--sort"], ENVF)
case("env list --pretty-print --sort", ["cronjob", "env", "list", "-n", "my-cron", "--pretty-print", "--sort", "--show-values"], ENVF)
case("env list --json", ["cronjob", "env", "list", "-n", "my-cron", "--json"], ENVF)
case("env list --json --sort", ["cronjob", "env", "list", "-n", "my-cron", "--json", "--sort"], ENVF)
case("env list empty", ["cronjob", "env", "list", "-n", "my-cron", "--json"],
     [{"method": "GET", "path": "/variable", "status": 200, "body": {"results": []}}])
case("env list missing flag", ["cronjob", "env", "list"])
case("env list unknown", ["cronjob", "env", "list", "-n", "nope"])
case("env list api error", ["cronjob", "env", "list", "-n", "my-cron"], [err("GET", "/variable", 500)])
case("env create", ["cronjob", "env", "create", "-n", "my-cron", "-k", "NEW", "-v", "v"], ENVF)
case("env create secret env scope", ["cronjob", "env", "create", "-n", "my-cron", "-k", "NEW", "-v", "v", "--secret", "--scope", "ENVIRONMENT"], ENVF)
case("env create project scope override", ["cronjob", "env", "create", "-n", "my-cron", "-k", "NEW", "-v", "v", "--scope", "PROJECT", "--override-if-exists"], ENVF)
case("env create bad scope", ["cronjob", "env", "create", "-n", "my-cron", "-k", "NEW", "-v", "v", "--scope", "FOO"], ENVF)
case("env create api error", ["cronjob", "env", "create", "-n", "my-cron", "-k", "NEW", "-v", "v"],
     [{"method": "POST", "path": "/variable", "status": 400, "body": {"status": 400, "detail": "key already exists"}}])
case("env create missing flags", ["cronjob", "env", "create", "-n", "my-cron"])
case("env create unknown", ["cronjob", "env", "create", "-n", "nope", "-k", "A", "-v", "b"])
case("env update", ["cronjob", "env", "update", "-n", "my-cron", "-k", "FOO", "-v", "new"], ENVF)
case("env update unknown key", ["cronjob", "env", "update", "-n", "my-cron", "-k", "NOPE", "-v", "new"], ENVF)
case("env update api error", ["cronjob", "env", "update", "-n", "my-cron", "-k", "FOO", "-v", "new"], [err("PUT", "/variable/var-1", 500)] + ENVF)
case("env update missing value", ["cronjob", "env", "update", "-n", "my-cron", "-k", "FOO"])
case("env delete", ["cronjob", "env", "delete", "-n", "my-cron", "-k", "FOO"], ENVF)
case("env delete unknown key", ["cronjob", "env", "delete", "-n", "my-cron", "-k", "NOPE"], ENVF)
case("env delete api error", ["cronjob", "env", "delete", "-n", "my-cron", "-k", "FOO"], [err("DELETE", "/variable/var-1", 403)] + ENVF)
case("env delete missing key", ["cronjob", "env", "delete", "-n", "my-cron"])
case("env alias create", ["cronjob", "env", "alias", "create", "-n", "my-cron", "-k", "FOO", "--alias", "BAR"], ENVF)
case("env alias create env scope", ["cronjob", "env", "alias", "create", "-n", "my-cron", "-k", "FOO", "--alias", "BAR", "--scope", "ENVIRONMENT"], ENVF)
case("env alias create project scope", ["cronjob", "env", "alias", "create", "-n", "my-cron", "-k", "FOO", "--alias", "BAR", "--scope", "PROJECT"], ENVF)
case("env alias create unknown key", ["cronjob", "env", "alias", "create", "-n", "my-cron", "-k", "NOPE", "--alias", "BAR"], ENVF)
case("env alias create api error", ["cronjob", "env", "alias", "create", "-n", "my-cron", "-k", "FOO", "--alias", "BAR"],
     [err("POST", "/variable/var-1/alias", 400)] + ENVF)
case("env alias create missing flags", ["cronjob", "env", "alias", "create"])
case("env override create", ["cronjob", "env", "override", "create", "-n", "my-cron", "-k", "ENV_LEVEL", "--value", "x"], ENVF)
case("env override create env scope", ["cronjob", "env", "override", "create", "-n", "my-cron", "-k", "FOO", "--value", "x", "--scope", "ENVIRONMENT"], ENVF)
case("env override create unknown key", ["cronjob", "env", "override", "create", "-n", "my-cron", "-k", "NOPE", "--value", "x"], ENVF)
case("env override create missing value", ["cronjob", "env", "override", "create", "-n", "my-cron", "-k", "FOO"])
case("env override create unknown cron", ["cronjob", "env", "override", "create", "-n", "nope", "-k", "FOO", "--value", "x"])

# ---- external-secret ----
ESF = ENVF + [CLUSTERS]
case("external-secret create", ["cronjob", "external-secret", "create", "-n", "my-cron", "-k", "EXT2", "-r", "arn:x",
                                "--secret-manager-access-name", "my-sm"], ESF)
case("external-secret create mount path env scope", ["cronjob", "external-secret", "create", "-n", "my-cron", "-k", "EXT2", "-r", "arn:x",
                                "--secret-manager-access-name", "my-sm", "--mount-path", "/etc/x", "--scope", "ENVIRONMENT"], ESF)
case("external-secret create unknown sma", ["cronjob", "external-secret", "create", "-n", "my-cron", "-k", "EXT2", "-r", "arn:x",
                                "--secret-manager-access-name", "nope"], ESF)
case("external-secret create clusters error", ["cronjob", "external-secret", "create", "-n", "my-cron", "-k", "EXT2", "-r", "arn:x",
                                "--secret-manager-access-name", "my-sm"], [err("GET", "/organization/org-1/cluster", 500)] + ESF)
case("external-secret create api error", ["cronjob", "external-secret", "create", "-n", "my-cron", "-k", "EXT2", "-r", "arn:x",
                                "--secret-manager-access-name", "my-sm"],
     [{"method": "POST", "path": "/variable", "status": 400, "body": {"detail": "invalid reference"}}] + ESF)
case("external-secret create missing flags", ["cronjob", "external-secret", "create", "-n", "my-cron", "-k", "A"])
case("external-secret create unknown cron", ["cronjob", "external-secret", "create", "-n", "nope", "-k", "A", "-r", "b",
                                "--secret-manager-access-name", "my-sm"], ESF)
case("external-secret update", ["cronjob", "external-secret", "update", "-n", "my-cron", "-k", "EXT", "-r", "arn:y",
                                "--secret-manager-access-name", "my-sm"], ESF)
case("external-secret update unknown key", ["cronjob", "external-secret", "update", "-n", "my-cron", "-k", "NOPE", "-r", "arn:y",
                                "--secret-manager-access-name", "my-sm"], ESF)
case("external-secret update no sma", ["cronjob", "external-secret", "update", "-n", "my-cron", "-k", "EXT"], ESF)
case("external-secret update missing key", ["cronjob", "external-secret", "update", "-n", "my-cron"])
case("external-secret delete", ["cronjob", "external-secret", "delete", "-n", "my-cron", "-k", "EXT"], ESF)
case("external-secret delete unknown key", ["cronjob", "external-secret", "delete", "-n", "my-cron", "-k", "NOPE"], ESF)
case("external-secret delete missing flags", ["cronjob", "external-secret", "delete"])

json.dump({"fixtures": [JOBS], "cases": cases}, open(OUT, "w"), indent=1)
print(len(cases), "cases")
