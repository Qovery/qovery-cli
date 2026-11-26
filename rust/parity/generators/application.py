#!/usr/bin/env python3
"""Generates parity/scenarios/application.json.

Usage: python3 parity/generators/application.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import copy
import json

D = json.load(open(os.path.join(PARITY, "fixtures", "default.json")))
def dflt(path):
    return next(r for r in D if r["path"] == path)["body"]

APP1 = dflt("/environment/env-1/application")["results"][0]
APP2 = copy.deepcopy(APP1)
APP2.update({"id": "app-2", "name": "other-app"})
APP2["git_repository"] = dict(APP2["git_repository"], deployed_commit_id="def456", branch="develop")
APPS = {"method": "GET", "path": "/environment/env-1/application", "body": {"results": [APP1, APP2]}}

TS = "2024-06-02T11:30:00Z"

def st(id, state, exec_id):
    return {"id": id, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": exec_id}

def stages(*statuses):
    return {"environment": {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYED"}, "stages": [{"applications": list(statuses)}]}

def watch(*bodies):
    return {"method": "GET", "path": "/environment/env-1/statusesWithStages",
            "responses": [{"status": 200, "body": b} for b in bodies]}

def err(method, path, status=500, body=None):
    return {"method": method, "path": path, "status": status, "body": body if body is not None else {"message": "boom"}}

def ok(method, path, body=None, status=200):
    r = {"method": method, "path": path, "status": status}
    if body is not None:
        r["body"] = body
    else:
        r["body_raw"] = ""
    return r

STATUS_OK = {"id": "x", "state": "QUEUED", "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": False,
             "status_details": {"action": "DEPLOY", "status": "QUEUED", "sub_action": "NONE"},
             "deployment_request_id": None, "deployment_requests_count": 0}
ENV_ST = {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYED"}
SVC_DEPLOY = ok("POST", "/environment/env-1/service/deploy", ENV_ST, 202)
SVC_STOP = ok("POST", "/environment/env-1/service/stop", None, 202)
SVC_DELETE = ok("POST", "/environment/env-1/service/delete", None, 202)
SVC_RESTART = ok("POST", "/environment/env-1/service/restart-service", STATUS_OK, 202)
APP_DEPLOY = ok("POST", "/application/app-1/deploy", STATUS_OK, 202)

COMMITS = {"method": "GET", "path": "/application/app-1/commit", "body": {"results": [
    {"created_at": "2024-06-01T10:00:00Z", "git_commit_id": "old111", "tag": "", "message": "old commit", "author_name": "me"},
    {"created_at": "2024-06-03T10:00:00Z", "git_commit_id": "new999", "tag": "v1", "message": "newest commit\n\nbody", "author_name": "me"},
]}}
COMMITS2 = {"method": "GET", "path": "/application/app-2/commit", "body": {"results": [
    {"created_at": "2024-06-04T10:00:00Z", "git_commit_id": "app2new", "tag": "", "message": "app2 commit", "author_name": "me"},
]}}

CLONED = dict(copy.deepcopy(APP1), id="app-9", name="my-clone")
UPDATED = copy.deepcopy(APP1)

CD = {"id": "cd-1", "created_at": "2024-05-01T10:00:00Z", "domain": "www.example.com", "generate_certificate": True,
      "use_cdn": False, "validation_domain": "zxy.qovery.io"}
CD2 = {"id": "cd-2", "created_at": "2024-05-01T10:00:00Z", "domain": "api.example.com", "generate_certificate": False,
       "validation_domain": "abc.qovery.io"}
DOMAINS = {"method": "GET", "path": "/application/app-1/customDomain", "body": {"results": [CD, CD2]}}

def link(url, default=False):
    return {"service_id": "app-1", "service_type": "APPLICATION", "url": url, "internal_port": 80, "external_port": 443,
            "is_qovery_domain": "qovery" in url, "is_default": default}
LINKS = {"method": "GET", "path": "/application/app-1/link", "body": {"results": [
    link("https://p80-app-1.qovery.io", True), link("https://www.example.com")]}}

def var(id, key, value, scope="APPLICATION", vtype="VALUE", secret=False, **kw):
    v = {"id": id, "created_at": "2024-05-01T10:00:00Z", "updated_at": TS, "key": key, "value": value,
         "scope": scope, "variable_type": vtype, "is_secret": secret}
    v.update(kw)
    return v
VARS = {"method": "GET", "path": "/variable", "query": {"parent_id": "app-1", "scope": "APPLICATION"}, "body": {"results": [
    var("var-1", "FOO", "bar"),
    var("var-2", "DB_PASSWORD", None, scope="ENVIRONMENT", secret=True),
    var("var-3", "PROJECT_VAR", "pv", scope="PROJECT"),
    var("var-4", "EXT", "aws/secret/ref", secret_manager_access_id="sma-1"),
    var("var-5", "FOO_ALIAS", "bar", vtype="ALIAS", aliased_variable={"id": "var-1", "key": "FOO", "value": "bar", "mount_path": "", "scope": "APPLICATION", "variable_type": "VALUE"}),
]}}
VAR_CREATED = var("var-new", "NEW", "v")
POST_VAR = ok("POST", "/variable", VAR_CREATED, 201)
PUT_VAR1 = ok("PUT", "/variable/var-1", var("var-1", "FOO", "baz"))
PUT_VAR4 = ok("PUT", "/variable/var-4", var("var-4", "EXT", "new/ref"))
DEL_VAR1 = ok("DELETE", "/variable/var-1", None, 204)
DEL_VAR4 = ok("DELETE", "/variable/var-4", None, 204)
ALIAS_VAR3 = ok("POST", "/variable/var-3/alias", var("var-al", "MY_ALIAS", "pv", vtype="ALIAS"), 201)
OVERRIDE_VAR3 = ok("POST", "/variable/var-3/override", var("var-ov", "PROJECT_VAR", "ov", vtype="OVERRIDE"), 201)

CLUSTERS = copy.deepcopy(dflt("/organization/org-1/cluster"))
CLUSTERS["results"][0]["secret_manager_accesses"] = [{
    "id": "sma-1", "name": "my-sm", "created_at": "2024-05-01T10:00:00Z", "updated_at": TS,
    "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"}, "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}]
CLUSTER_FX = {"method": "GET", "path": "/organization/org-1/cluster", "body": CLUSTERS}

cases = []
def case(name, args, fixtures=None, **kw):
    c = {"name": name, "args": args}
    if fixtures:
        c["fixtures"] = fixtures
    c.update(kw)
    cases.append(c)

A = ["application"]
N = ["-n", "my-app"]

# group commands
case("application (group)", A)
for g in ["domain", "env", "external-secret"]:
    case("application %s (group)" % g, A + [g])
case("application env alias (group)", A + ["env", "alias"])
case("application env override (group)", A + ["env", "override"])
case("application unknown subcommand", A + ["nope"])

# list
case("list", A + ["list"])
case("list --json", A + ["list", "--json"])
case("list two apps", A + ["list"], [APPS])
case("list two apps --json", A + ["list", "--json"], [APPS])
case("list empty --json", A + ["list", "--json"], [{"method": "GET", "path": "/environment/env-1/application", "body": {"results": []}}])
case("list empty", A + ["list"], [{"method": "GET", "path": "/environment/env-1/application", "body": {"results": []}}])
case("list with names", A + ["list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
case("list env not found", A + ["list", "--organization", "my-org", "--project", "my-project", "--environment", "nope"])
case("list project not found", A + ["list", "--organization", "my-org", "--project", "nope", "--environment", "my-env"])
case("list api error", A + ["list"], [err("GET", "/environment/env-1/application")])
case("list statuses error", A + ["list"], [err("GET", "/environment/env-1/statuses", 403)])
case("list no token", A + ["list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# deploy
case("deploy", A + ["deploy"] + N, [SVC_DEPLOY])
case("deploy commit id", A + ["deploy"] + N + ["-c", "abcdef"], [SVC_DEPLOY])
case("deploy commit latest", A + ["deploy"] + N + ["--commit-id", "latest"], [SVC_DEPLOY, COMMITS])
case("deploy commit LATEST", A + ["deploy"] + N + ["--commit-id", "LATEST"], [SVC_DEPLOY, COMMITS])
case("deploy commit latest no commits", A + ["deploy"] + N + ["--commit-id", "latest"],
     [SVC_DEPLOY, {"method": "GET", "path": "/application/app-1/commit", "body": {"results": []}}])
case("deploy commit latest error", A + ["deploy"] + N + ["--commit-id", "latest"], [SVC_DEPLOY, err("GET", "/application/app-1/commit")])
case("deploy multi", A + ["deploy", "--applications", "my-app, other-app"], [APPS, SVC_DEPLOY])
case("deploy multi latest", A + ["deploy", "--applications", "my-app,other-app", "-c", "latest"], [APPS, SVC_DEPLOY, COMMITS, COMMITS2])
case("deploy multi not found", A + ["deploy", "--applications", "my-app, nope"], [APPS, SVC_DEPLOY])
IMAGE_APP = {k: v for k, v in APP1.items() if k not in ("git_repository", "build_mode", "dockerfile_path")}
IMAGE_APPS = {"method": "GET", "path": "/environment/env-1/application", "body": {"results": [IMAGE_APP, APP2]}}
case("deploy without git repository", A + ["deploy"] + N, [IMAGE_APPS, SVC_DEPLOY])
def refused(flag="--commit-id"):
    """An image application refuses a Git-only flag: the error, and no request after the application list."""
    return {"exit": 1, "stdout_contains": "application my-app runs an image, not a Git repository: %s only applies to Git applications" % flag,
            "requests": ["GET /environment/env-1/application"]}
case("deploy without git repository commit id", A + ["deploy"] + N + ["-c", "abcdef"], [IMAGE_APPS], expect=refused())
case("deploy without git repository commit latest", A + ["deploy"] + N + ["--commit-id", "latest"], [IMAGE_APPS], expect=refused())
case("deploy multi one without git repository", A + ["deploy", "--applications", "other-app,my-app"], [IMAGE_APPS, SVC_DEPLOY])
case("deploy multi latest one without git repository", A + ["deploy", "--applications", "other-app,my-app", "-c", "latest"],
     [IMAGE_APPS], expect=refused())
case("deploy without git repository commit id --watch", A + ["deploy"] + N + ["-c", "abcdef", "--watch"], [IMAGE_APPS], expect=refused())
case("deploy both flags", A + ["deploy", "-n", "my-app", "--applications", "my-app"])
case("deploy no flags", A + ["deploy"])
case("deploy not found", A + ["deploy", "-n", "nope"])
case("deploy api error", A + ["deploy"] + N, [err("POST", "/environment/env-1/service/deploy", 400, {"message": "bad"})])
case("deploy list error", A + ["deploy"] + N, [err("GET", "/environment/env-1/application")])
case("deploy --watch", A + ["deploy"] + N + ["--watch"],
     [SVC_DEPLOY, watch(stages(st("app-1", "DEPLOYED", "e0")), stages(st("app-1", "DEPLOYED", "e1")))])
case("deploy -w multi", A + ["deploy", "--applications", "my-app,other-app", "-w"],
     [APPS, SVC_DEPLOY, watch(stages(st("app-1", "DEPLOYED", "e0"), st("app-2", "STOPPED", "f0")),
                              stages(st("app-1", "DEPLOYED", "e1"), st("app-2", "DEPLOYED", "f1")))])
case("deploy --watch error", A + ["deploy"] + N + ["--watch"],
     [SVC_DEPLOY, watch(stages(st("app-1", "DEPLOYED", "e0")), stages(st("app-1", "DEPLOYMENT_ERROR", "e1")))])
case("deploy --watch statuses error", A + ["deploy"] + N + ["--watch"],
     [SVC_DEPLOY, err("GET", "/environment/env-1/statusesWithStages")], timeout=60)

# redeploy
case("redeploy", A + ["redeploy"] + N, [APP_DEPLOY])
case("redeploy commit", A + ["redeploy"] + N + ["-c", "abcdef"], [APP_DEPLOY])
case("redeploy commit latest", A + ["redeploy"] + N + ["-c", "latest"], [APP_DEPLOY, COMMITS])
case("redeploy no deployed commit", A + ["redeploy", "-n", "other-app"],
     [{"method": "GET", "path": "/environment/env-1/application", "body": {"results": [dict(APP2, git_repository={"provider": "GITHUB", "owner": "me", "url": "u", "name": "n"})]}},
      ok("POST", "/application/app-2/deploy", STATUS_OK, 202)])
case("redeploy without git repository", A + ["redeploy"] + N, [IMAGE_APPS, APP_DEPLOY])
case("redeploy without git repository commit", A + ["redeploy"] + N + ["-c", "abcdef"], [IMAGE_APPS], expect=refused())
case("redeploy without git repository commit latest", A + ["redeploy"] + N + ["-c", "latest", "-w"], [IMAGE_APPS], expect=refused())
case("redeploy missing app flag", A + ["redeploy"])
case("redeploy not found", A + ["redeploy", "-n", "nope"])
case("redeploy api error", A + ["redeploy"] + N, [err("POST", "/application/app-1/deploy", 500)])
case("redeploy --watch", A + ["redeploy"] + N + ["-w"],
     [APP_DEPLOY, watch(stages(st("app-1", "DEPLOYED", "e0")), stages(st("app-1", "DEPLOYED", "e1")))])

# restart
case("restart", A + ["restart"] + N, [SVC_RESTART])
case("restart multi", A + ["restart", "--applications", "my-app,other-app"], [APPS, SVC_RESTART])
case("restart no flags", A + ["restart"])
case("restart not found", A + ["restart", "-n", "nope"])
case("restart api error", A + ["restart"] + N, [err("POST", "/environment/env-1/service/restart-service", 400)])
case("restart --watch", A + ["restart"] + N + ["--watch"],
     [SVC_RESTART, watch(stages(st("app-1", "DEPLOYED", "e0")), stages(st("app-1", "RESTARTED", "e1")))])

# stop
case("stop", A + ["stop"] + N, [SVC_STOP])
case("stop multi", A + ["stop", "--applications", "my-app,other-app"], [APPS, SVC_STOP])
case("stop both", A + ["stop", "--applications", "my-app", "-n", "my-app"])
case("stop no flags", A + ["stop"])
case("stop not found", A + ["stop", "-n", "nope"])
case("stop api error", A + ["stop"] + N, [err("POST", "/environment/env-1/service/stop", 500)])
case("stop --watch", A + ["stop"] + N + ["--watch"],
     [SVC_STOP, watch(stages(st("app-1", "DEPLOYED", "e0")), stages(st("app-1", "STOPPED", "e1")))])
case("stop --watch wrong final state", A + ["stop"] + N + ["--watch"],
     [SVC_STOP, watch(stages(st("app-1", "DEPLOYED", "e0")), stages(st("app-1", "DEPLOYED", "e1")))])

# delete
case("delete", A + ["delete"] + N, [SVC_DELETE])
case("delete multi", A + ["delete", "--applications", "my-app,other-app"], [APPS, SVC_DELETE])
case("delete no flags", A + ["delete"])
case("delete not found", A + ["delete", "-n", "nope"])
case("delete api error", A + ["delete"] + N, [err("POST", "/environment/env-1/service/delete", 403)])
case("delete --watch", A + ["delete"] + N + ["--watch"],
     [SVC_DELETE, watch(stages(st("app-1", "DEPLOYED", "e0")), {"environment": {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYED"}, "stages": [{"applications": []}]})])
case("delete --watch multi", A + ["delete", "--applications", "my-app,other-app", "--watch"],
     [APPS, SVC_DELETE, watch(stages(st("app-1", "DEPLOYED", "e0"), st("app-2", "DEPLOYED", "f0")),
                              stages(st("app-2", "DELETING", "f1")), stages(st("app-2", "DELETED", "f1")))], timeout=30)

# cancel
ENV_STATUSES = copy.deepcopy(dflt("/environment/env-1/statuses"))
def env_statuses(env_state, app_state):
    b = copy.deepcopy(ENV_STATUSES)
    b["environment"]["state"] = env_state
    b["applications"][0]["state"] = app_state
    return b
case("cancel nothing to cancel", A + ["cancel"] + N)
case("cancel", A + ["cancel"] + N,
     [{"method": "GET", "path": "/environment/env-1/statuses", "body": env_statuses("DEPLOYING", "DEPLOYING")},
      ok("POST", "/environment/env-1/cancelDeployment", {"id": "env-1", "state": "CANCELING", "last_deployment_state": "DEPLOYED"})])
case("cancel --watch", A + ["cancel"] + N + ["--watch"],
     [{"method": "GET", "path": "/environment/env-1/statuses", "responses": [
         {"status": 200, "body": env_statuses("DEPLOYING", "DEPLOYING")},
         {"status": 200, "body": env_statuses("CANCELED", "CANCELED")}]},
      ok("POST", "/environment/env-1/cancelDeployment", {"id": "env-1", "state": "CANCELING", "last_deployment_state": "DEPLOYED"})], timeout=30)
case("cancel waits previous deployment", A + ["cancel"] + N,
     [{"method": "GET", "path": "/environment/env-1/statuses", "responses": [
         {"status": 200, "body": env_statuses("DEPLOYING", "DEPLOYED")},
         {"status": 200, "body": env_statuses("DEPLOYED", "DEPLOYED")}]}], timeout=30)
case("cancel api error", A + ["cancel"] + N,
     [{"method": "GET", "path": "/environment/env-1/statuses", "body": env_statuses("DEPLOYING", "DEPLOYING")},
      err("POST", "/environment/env-1/cancelDeployment", 400)])
case("cancel statuses error", A + ["cancel"] + N, [err("GET", "/environment/env-1/statuses")])
case("cancel missing flag", A + ["cancel"])
case("cancel not found", A + ["cancel", "-n", "nope"])

# clone
CLONE_OK = ok("POST", "/application/app-1/clone", CLONED, 201)
case("clone", A + ["clone"] + N, [CLONE_OK])
case("clone target name", A + ["clone"] + N + ["--target-application-name", "my-clone"], [CLONE_OK])
case("clone target project env", A + ["clone"] + N + ["--target-project", "my-project", "--target-environment", "my-env"], [CLONE_OK])
case("clone target env not found", A + ["clone"] + N + ["--target-environment", "nope"], [CLONE_OK])
case("clone target project not found", A + ["clone"] + N + ["--target-project", "nope"], [CLONE_OK])
case("clone api error", A + ["clone"] + N, [err("POST", "/application/app-1/clone", 400, {"message": "name already used"})])
case("clone not found", A + ["clone", "-n", "nope"])
case("clone missing flag", A + ["clone"])

# update
UPD = ok("PUT", "/application/app-1", UPDATED)
case("update", A + ["update"] + N, [UPD])
case("update branch", A + ["update"] + N + ["--branch", "feature"], [UPD])
case("update auto-deploy false", A + ["update"] + N + ["--auto-deploy=false"], [UPD])
case("update auto-deploy", A + ["update"] + N + ["--auto-deploy"], [UPD])
case("update auto-preview", A + ["update"] + N + ["--auto-preview"], [UPD])
PREVIEW_ON = dict(copy.deepcopy(APP1), auto_preview=True)
case("update auto-preview false", A + ["update"] + N + ["--auto-preview=false"],
     [{"method": "GET", "path": "/environment/env-1/application", "body": {"results": [PREVIEW_ON]}}, UPD])
case("update auto-deploy and auto-preview", A + ["update"] + N + ["--auto-deploy=false", "--auto-preview"], [UPD])
RICH = copy.deepcopy(APP1)
RICH.update({
    "description": "desc", "storage": [{"id": "st-1", "type": "FAST_SSD", "size": 10, "mount_point": "/data"}],
    "ports": [{"id": "p-1", "name": "p80", "internal_port": 80, "external_port": 443, "publicly_accessible": True,
               "is_default": True, "protocol": "HTTP", "public_path": None}],
    "arguments": ["--x"], "entrypoint": "/bin/run", "dockerfile_path": None, "auto_deploy": None,
    "healthchecks": {"readiness_probe": {"type": {"tcp": {"port": 80}, "http": None}, "initial_delay_seconds": 30,
                                         "period_seconds": 10, "timeout_seconds": 5, "success_threshold": 1, "failure_threshold": 3},
                     "liveness_probe": None},
    "autoscaling": {"id": "as-1", "created_at": "2024-05-01T10:00:00Z", "service_id": "app-1", "mode": "KEDA", "polling_interval_seconds": 30, "cooldown_period_seconds": 300, "scalers": [
        {"id": "sc-1", "created_at": "2024-05-01T10:00:00Z", "scaler_type": "cpu", "enabled": True, "role": "PRIMARY", "config_json": {"value": "50"}}]},
})
RICH["git_repository"]["git_token_id"] = None
case("update rich application", A + ["update"] + N + ["--branch", "b2"],
     [{"method": "GET", "path": "/environment/env-1/application", "body": {"results": [RICH]}}, UPD])
case("update without git repository", A + ["update"] + N, [IMAGE_APPS, UPD])
case("update git repository null", A + ["update"] + N,
     [{"method": "GET", "path": "/environment/env-1/application", "body": {"results": [dict(IMAGE_APP, git_repository=None)]}}, UPD])
case("update without git repository auto-preview", A + ["update"] + N + ["--auto-preview"], [IMAGE_APPS, UPD])
case("update without git repository branch", A + ["update"] + N + ["--branch", "feature", "--auto-preview"], [IMAGE_APPS],
     expect=refused("--branch"))
case("update api error", A + ["update"] + N, [err("PUT", "/application/app-1", 400)])
case("update not found", A + ["update", "-n", "nope"])
case("update missing flag", A + ["update"])

# domain
DOM = ["domain"]
case("domain list", A + DOM + ["list"] + N, [DOMAINS, LINKS])
case("domain list --json", A + DOM + ["list"] + N + ["--json"], [DOMAINS, LINKS])
case("domain list empty --json", A + DOM + ["list"] + N + ["--json"],
     [{"method": "GET", "path": "/application/app-1/customDomain", "body": {"results": []}},
      {"method": "GET", "path": "/application/app-1/link", "body": {"results": []}}])
case("domain list links error", A + DOM + ["list"] + N, [DOMAINS, err("GET", "/application/app-1/link")])
case("domain list domains error", A + DOM + ["list"] + N, [err("GET", "/application/app-1/customDomain")])
case("domain list missing flag", A + DOM + ["list"])
case("domain list not found", A + DOM + ["list", "-n", "nope"])
case("domain list nil validation domain", A + DOM + ["list"] + N,
     [{"method": "GET", "path": "/application/app-1/customDomain", "body": {"results": [dict(CD2, validation_domain=None)]}}, LINKS],
     ignore=["stderr"])
CD_NEW = {"id": "cd-3", "created_at": "2024-05-01T10:00:00Z", "domain": "new.example.com", "generate_certificate": False, "use_cdn": True}
case("domain create", A + DOM + ["create"] + N + ["--domain", "new.example.com"],
     [DOMAINS, ok("POST", "/application/app-1/customDomain", dict(CD_NEW, generate_certificate=True), 201)])
case("domain create flags", A + DOM + ["create"] + N + ["--domain", "new.example.com", "--do-not-generate-certificate", "--is-behind-a-cdn"],
     [DOMAINS, ok("POST", "/application/app-1/customDomain", CD_NEW, 201)])
case("domain create exists", A + DOM + ["create"] + N + ["--domain", "www.example.com"], [DOMAINS])
case("domain create api error", A + DOM + ["create"] + N + ["--domain", "new.example.com"],
     [DOMAINS, err("POST", "/application/app-1/customDomain", 400)])
# successful responses the Go client cannot decode (generated client + CustomDomain.UnmarshalJSON)
POST_CD = {"method": "POST", "path": "/application/app-1/customDomain", "status": 201}
for name, resp in [
    ("syntax error", {"body_raw": "{bad", "headers": {"Content-Type": "application/json"}}),
    ("not an object", {"body": [1]}),
    ("not json", {"body_raw": "hello"}),
    ("missing required", {"body": {"id": "cd-3"}}),
    ("wrong type", {"body": dict(CD_NEW, domain=5)}),
    ("wrong bool type", {"body": dict(CD_NEW, use_cdn="yes")}),
    ("null fields", {"body": dict(CD_NEW, domain=None, generate_certificate=None)}),
    ("bad created_at", {"body": dict(CD_NEW, created_at="yesterday")}),
    ("wrong created_at type", {"body": dict(CD_NEW, created_at=5)}),
    ("bad updated_at", {"body": dict(CD_NEW, updated_at="2025-13-02T01:04:45Z")}),
    ("bad status", {"body": dict(CD_NEW, status="FOO")}),
    ("wrong status type", {"body": dict(CD_NEW, status=5)}),
    ("valid status, null updated_at", {"body": dict(CD_NEW, status="VALIDATION_PENDING", updated_at=None)}),
    ("non-string name", {"body": dict(CD_NEW, name=5)}),
]:
    case("domain create response " + name, A + DOM + ["create"] + N + ["--domain", "new.example.com"],
         [DOMAINS, dict(POST_CD, **resp)])
for name, raw in [("empty", ""), ("null", "null")]:
    case("domain create response " + name, A + DOM + ["create"] + N + ["--domain", "new.example.com"],
         [DOMAINS, dict(POST_CD, body_raw=raw, headers={"Content-Type": "application/json"})],
         ignore=["stderr"], note="Go dereferences the nil *CustomDomain: nil pointer panic (stack trace not reproduced)")
case("domain edit response syntax error", A + DOM + ["edit"] + N + ["--domain", "www.example.com"],
     [DOMAINS, {"method": "PUT", "path": "/application/app-1/customDomain/cd-1", "body_raw": "{bad",
                "headers": {"Content-Type": "application/json"}}])
case("domain create missing domain", A + DOM + ["create"] + N)
case("domain create missing all", A + DOM + ["create"])
case("domain edit", A + DOM + ["edit"] + N + ["--domain", "www.example.com", "--do-not-generate-certificate"],
     [DOMAINS, ok("PUT", "/application/app-1/customDomain/cd-1", dict(CD, generate_certificate=False))])
case("domain edit not exists", A + DOM + ["edit"] + N + ["--domain", "nope.example.com"], [DOMAINS])
case("domain edit api error", A + DOM + ["edit"] + N + ["--domain", "www.example.com"],
     [DOMAINS, err("PUT", "/application/app-1/customDomain/cd-1", 500)])
case("domain edit missing flags", A + DOM + ["edit"] + N)
case("domain delete", A + DOM + ["delete"] + N + ["--domain", "api.example.com"],
     [DOMAINS, ok("DELETE", "/application/app-1/customDomain/cd-2", None, 204)])
case("domain delete not exists", A + DOM + ["delete"] + N + ["--domain", "nope.example.com"], [DOMAINS])
case("domain delete api error", A + DOM + ["delete"] + N + ["--domain", "api.example.com"],
     [DOMAINS, err("DELETE", "/application/app-1/customDomain/cd-2", 404)])
case("domain delete missing flags", A + DOM + ["delete"])
case("domain delete app not found", A + DOM + ["delete", "-n", "nope", "--domain", "x"])

# env
E = ["env"]
case("env list", A + E + ["list"] + N, [VARS], ignore=["stdout"], note="Go iterates the EnvVarLines map (random order) when --sort is not given")
case("env list show values", A + E + ["list"] + N + ["--show-values"], [VARS], ignore=["stdout"], note="Go iterates the EnvVarLines map (random order) when --sort is not given")
case("env list pretty sort", A + E + ["list"] + N + ["--pretty-print", "--sort", "--show-values"], [VARS])
case("env list --json", A + E + ["list"] + N + ["--json"], [VARS])
case("env list --json --sort", A + E + ["list"] + N + ["--json", "--sort"], [VARS])
case("env list api error", A + E + ["list"] + N, [err("GET", "/variable", 500)])
case("env list missing flag", A + E + ["list"])
case("env list not found", A + E + ["list", "-n", "nope"])
case("env create", A + E + ["create"] + N + ["-k", "NEW", "-v", "v"], [VARS, POST_VAR])
case("env create secret env scope", A + E + ["create"] + N + ["-k", "NEW", "-v", "v", "--secret", "--scope", "ENVIRONMENT"], [VARS, POST_VAR])
case("env create project scope override", A + E + ["create"] + N + ["-k", "NEW", "-v", "v", "--scope", "PROJECT", "--override-if-exists"], [VARS, POST_VAR])
case("env create bad scope", A + E + ["create"] + N + ["-k", "NEW", "-v", "v", "--scope", "NOPE"], [VARS, POST_VAR])
case("env create api error", A + E + ["create"] + N + ["-k", "NEW", "-v", "v"],
     [VARS, err("POST", "/variable", 400, {"detail": "variable already exists"})])
case("env create missing value", A + E + ["create"] + N + ["-k", "NEW"])
case("env create not found", A + E + ["create", "-n", "nope", "-k", "NEW", "-v", "v"])
case("env update", A + E + ["update"] + N + ["-k", "FOO", "-v", "baz"], [VARS, PUT_VAR1])
case("env update key not found", A + E + ["update"] + N + ["-k", "NOPE", "-v", "baz"], [VARS])
case("env update api error", A + E + ["update"] + N + ["-k", "FOO", "-v", "baz"], [VARS, err("PUT", "/variable/var-1", 400)])
case("env update missing flags", A + E + ["update"] + N)
case("env delete", A + E + ["delete"] + N + ["-k", "FOO"], [VARS, DEL_VAR1])
case("env delete key not found", A + E + ["delete"] + N + ["-k", "NOPE"], [VARS])
case("env delete api error", A + E + ["delete"] + N + ["-k", "FOO"], [VARS, err("DELETE", "/variable/var-1", 403)])
case("env delete missing key", A + E + ["delete"] + N)
case("env alias create", A + E + ["alias", "create"] + N + ["-k", "PROJECT_VAR", "--alias", "MY_ALIAS"], [VARS, ALIAS_VAR3])
case("env alias create env scope", A + E + ["alias", "create"] + N + ["-k", "PROJECT_VAR", "--alias", "MY_ALIAS", "--scope", "ENVIRONMENT"], [VARS, ALIAS_VAR3])
case("env alias create key not found", A + E + ["alias", "create"] + N + ["-k", "NOPE", "--alias", "MY_ALIAS"], [VARS])
case("env alias create api error", A + E + ["alias", "create"] + N + ["-k", "PROJECT_VAR", "--alias", "MY_ALIAS"],
     [VARS, err("POST", "/variable/var-3/alias", 400)])
case("env alias create missing alias", A + E + ["alias", "create"] + N + ["-k", "X"])
case("env override create", A + E + ["override", "create"] + N + ["-k", "PROJECT_VAR", "--value", "ov"], [VARS, OVERRIDE_VAR3])
case("env override create key not found", A + E + ["override", "create"] + N + ["-k", "NOPE", "--value", "ov"], [VARS])
case("env override create api error", A + E + ["override", "create"] + N + ["-k", "PROJECT_VAR", "--value", "ov"],
     [VARS, err("POST", "/variable/var-3/override", 500)])
case("env override create missing value", A + E + ["override", "create"] + N + ["-k", "X"])

# external-secret
X = ["external-secret"]
case("external-secret create", A + X + ["create"] + N + ["-k", "EXT2", "-r", "aws/ref", "--secret-manager-access-name", "my-sm"],
     [VARS, CLUSTER_FX, POST_VAR])
case("external-secret create mount path scope", A + X + ["create"] + N + ["-k", "EXT2", "-r", "aws/ref", "--secret-manager-access-name", "my-sm", "--mount-path", "/etc/s", "--scope", "ENVIRONMENT"],
     [VARS, CLUSTER_FX, POST_VAR])
case("external-secret create sm not found", A + X + ["create"] + N + ["-k", "EXT2", "-r", "aws/ref", "--secret-manager-access-name", "nope"],
     [VARS, CLUSTER_FX])
case("external-secret create clusters error", A + X + ["create"] + N + ["-k", "EXT2", "-r", "aws/ref", "--secret-manager-access-name", "my-sm"],
     [VARS, err("GET", "/organization/org-1/cluster", 500)])
case("external-secret create api error", A + X + ["create"] + N + ["-k", "EXT2", "-r", "aws/ref", "--secret-manager-access-name", "my-sm"],
     [VARS, CLUSTER_FX, err("POST", "/variable", 400, {"detail": "invalid reference"})])
case("external-secret create missing flags", A + X + ["create"] + N + ["-k", "EXT2"])
case("external-secret update", A + X + ["update"] + N + ["-k", "EXT", "-r", "new/ref", "--secret-manager-access-name", "my-sm"],
     [VARS, CLUSTER_FX, PUT_VAR4])
case("external-secret update no sm name", A + X + ["update"] + N + ["-k", "EXT", "-r", "new/ref"], [VARS, CLUSTER_FX, PUT_VAR4])
case("external-secret update key not found", A + X + ["update"] + N + ["-k", "NOPE", "--secret-manager-access-name", "my-sm"], [VARS, CLUSTER_FX])
case("external-secret update api error", A + X + ["update"] + N + ["-k", "EXT", "-r", "x", "--secret-manager-access-name", "my-sm"],
     [VARS, CLUSTER_FX, err("PUT", "/variable/var-4", 500)])
case("external-secret update missing key", A + X + ["update"] + N)
case("external-secret delete", A + X + ["delete"] + N + ["-k", "EXT"], [VARS, DEL_VAR4])
case("external-secret delete key not found", A + X + ["delete"] + N + ["-k", "NOPE"], [VARS])
case("external-secret delete api error", A + X + ["delete"] + N + ["-k", "EXT"], [VARS, err("DELETE", "/variable/var-4", 500)])
case("external-secret delete missing flags", A + X + ["delete"])

json.dump({"cases": cases}, open(sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "application.json"), "w"), indent=1)
print(len(cases), "cases")
