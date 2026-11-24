#!/usr/bin/env python3
"""Generates parity/scenarios/environment.json.

Usage: python3 parity/generators/environment.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "environment.json")

def git_containers_fixture():
    """The default container list plus a Git container (source.git_repository)."""
    default = json.load(open(os.path.join(PARITY, "fixtures", "default.json")))
    route = next(f for f in default if f.get("path") == "/environment/env-1/container")
    image = route["body"]["results"][0]
    git = dict(image, id="cont-g", name="git-container", image_name="qovery-built", tag="abc123",
               source={"git_repository": {"provider": "GITHUB", "owner": "acme", "url": "https://github.com/acme/app.git",
                                          "name": "acme/app", "branch": "main", "dockerfile_path": "Dockerfile"}})
    return {"method": "GET", "path": "/environment/env-1/container", "status": 200, "body": {"results": [image, git]}}


GIT_CONTAINERS = git_containers_fixture()

def status(id_, state):
    return {"id": id_, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "last_deployment_date": "2024-06-02T11:30:00Z",
            "execution_id": "exec-" + id_, "steps": None}

def statuses(env_state, last_state=None):
    return {"environment": {"id": "env-1", "state": env_state, "last_deployment_state": last_state or env_state,
                            "last_deployment_date": "2024-06-02T11:30:00Z", "last_deployment_id": "dep-1",
                            "total_deployment_duration_in_seconds": 42},
            "applications": [status("app-1", last_state or env_state)], "containers": [], "jobs": [],
            "databases": [], "helms": [], "terraforms": []}

def statuses_fx(*states):
    return {"method": "GET", "path": "/environment/env-1/statuses",
            "responses": [{"status": 200, "body": statuses(s)} for s in states]}

ENV_STATUS = {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYING"}
SVC_STATUS = {"id": "app-1", "state": "QUEUED", "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
              "status_details": {"action": "DEPLOY", "status": "QUEUED", "sub_action": "NONE"},
              "deployment_request_id": None, "deployment_requests_count": 0}
ENVIRONMENT = {"id": "env-1", "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": "2024-06-02T11:30:00Z",
               "name": "my-env", "organization": {"id": "org-1"}, "project": {"id": "proj-1"},
               "cloud_provider": {"provider": "AWS", "cluster": "us-east-2"}, "mode": "DEVELOPMENT",
               "cluster_id": "cluster-1", "cluster_name": "my-cluster"}
CLONED = dict(ENVIRONMENT, id="env-2", name="new-env")
STAGE_RESP = {"id": "stage-9", "created_at": "2024-05-01T10:00:00Z", "environment": {"id": "env-1"}, "name": "new"}

def L(ts, step, stage, ttype, tname, tid, msg, err=None):
    d = {"type": "info", "timestamp": ts,
         "details": {"organization_id": "org-1", "cluster_id": "cluster-1", "execution_id": "exec-1",
                     "transmitter": {"type": ttype, "name": tname, "id": tid},
                     "stage": {"step": step, "name": stage, "id": None}},
         "error": err, "message": {"safe_message": msg, "full_details": None}}
    return d

ERR_FULL = {"tag": "K8S_CANNOT_APPLY_CONFIG", "user_log_message": "liveness probe failed",
            "hint_message": "check the probe path", "link": "https://hub.qovery.com/docs/probes"}
ERR_MIN = {"tag": "ENV_FAILED", "user_log_message": "deployment failed"}

LOGS = [
    L("2024-06-02T11:00:00.100Z", "PreCheck", None, "Environment", "my-env", "env-1", "pre-check started"),
    L("2024-06-02T11:00:01Z", "Queued", "build-stage", "TaskManager", "tm", "tm-1", "queued"),
    L("2024-06-02T11:00:05Z", "Build", "build-stage", "Application", "my-app", "app-1", "building image"),
    L("2024-06-02T11:00:03Z", "Build", "build-stage", "Environment", "my-env", "env-1", "stage build started"),
    L("2024-06-02T11:00:35.250Z", "Build", "build-stage", "Application", "my-app", "app-1", "image built\nwith 2 layers"),
    L("2024-06-02T11:01:00Z", "Deploy", "deploy-stage", "Application", "my-app", "app-1", "deploying <app> & co"),
    L("2024-06-02T13:02:10.5+02:00", "Deploy", "deploy-stage", "Application", "my-app", "app-1", "deployment failed", ERR_FULL),
    L("2024-06-02T11:01:30Z", "Deploy", "deploy-stage", "Terraform", "S3", "tf-1", "terraform apply"),
    L("2024-06-02T11:01:30Z", "Deploy", "deploy-stage", "Terraform", "S3", "tf-1", "terraform done"),
    L("2024-06-02T11:03:40Z", "Deployed", "deploy-stage", "Environment", "my-env", "env-1", "deployment error", ERR_MIN),
    {"type": "info", "timestamp": "2024-06-02T11:03:41Z", "details": {}, "error": None, "message": None},
]
LOGS_OK = [l for l in LOGS if l["error"] is None]
LOGS_ENV_ONLY = [l for l in LOGS if l["details"].get("transmitter", {}).get("type") in ("Environment", "TaskManager")]

def logs_fx(body, version=None):
    fx = {"method": "GET", "path": "/environment/env-1/logs", "status": 200, "body": body}
    if version:
        fx["query"] = {"version": version}
    return fx

HISTORY = {"page": 1, "page_size": 10, "results": [
    {"id": "dep-1", "created_at": "2024-06-02T11:30:00Z", "updated_at": "2024-06-02T11:31:05.5Z", "status": "DEPLOYED"},
    {"id": "dep-2", "created_at": "2024-06-01T10:00:00.123Z", "updated_at": "2024-06-01T10:12:00Z", "status": "DEPLOYMENT_ERROR"},
    {"id": "dep-3", "created_at": "2024-05-30T08:00:00+02:00", "updated_at": "2024-05-30T08:00:42+02:00", "status": "CANCELED"},
    {"id": "dep-4", "created_at": "2024-05-29T08:00:00Z"},
]}

def dss(id_, sid, stype, skipped):
    return {"id": id_, "created_at": "2024-05-01T10:00:00Z", "service_id": sid, "service_type": stype, "is_skipped": skipped}

STAGES = {"results": [
    {"id": "stage-1", "created_at": "2024-05-01T10:00:00Z", "environment": {"id": "env-1"}, "name": "first",
     "description": "", "deployment_order": 0,
     "services": [dss("dss-1", "app-1", "APPLICATION", False), dss("dss-2", "db-1", "DATABASE", True)]},
    {"id": "stage-2", "created_at": "2024-05-01T10:00:00Z", "environment": {"id": "env-1"}, "name": "second",
     "description": "my <stage>", "deployment_order": 1,
     "services": [dss("dss-3", "cont-1", "CONTAINER", False), dss("dss-4", "cron-1", "JOB", False)]},
    {"id": "stage-3", "created_at": "2024-05-01T10:00:00Z", "environment": {"id": "env-1"}, "name": "empty",
     "deployment_order": 2, "services": []},
    {"id": "stage-4", "created_at": "2024-05-01T10:00:00Z", "environment": {"id": "env-1"}, "name": "helms",
     "deployment_order": 3, "services": [dss("dss-5", "helm-1", "HELM", True), dss("dss-6", "life-1", "JOB", False)]},
]}

VARS = {"results": [
    {"id": "var-1", "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": "2024-06-02T11:30:00Z", "key": "FOO",
     "value": "bar", "scope": "ENVIRONMENT", "variable_type": "VALUE", "is_secret": False, "owned_by": "QOVERY"},
    {"id": "var-2", "created_at": "2024-05-02T10:00:00Z", "updated_at": "2024-05-02T10:00:00Z", "key": "DB_PASSWORD",
     "value": None, "scope": "ENVIRONMENT", "variable_type": "VALUE", "is_secret": True},
    {"id": "var-3", "created_at": "2024-05-02T10:00:00Z", "updated_at": "2024-05-02T10:00:00Z", "key": "EXT",
     "value": "arn:aws:secret", "scope": "ENVIRONMENT", "variable_type": "VALUE", "is_secret": False,
     "secret_manager_access_id": "sma-1"},
    {"id": "var-4", "created_at": "2024-05-03T10:00:00Z", "updated_at": "2024-05-03T10:00:00Z", "key": "PROJ_VAR",
     "value": "x", "scope": "PROJECT", "variable_type": "VALUE", "is_secret": False},
]}
VAR_RESP = {"id": "var-9", "created_at": "2024-07-01T10:00:00Z", "key": "NEW_KEY", "value": "val", "scope": "ENVIRONMENT",
            "variable_type": "VALUE", "is_secret": False}
CLUSTERS_SM = {"results": [{"id": "cluster-1", "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": "2024-06-02T11:30:00Z",
    "organization": {"id": "org-1"}, "name": "my-cluster", "region": "us-east-2", "cloud_provider": "AWS",
    "kubernetes": "MANAGED", "status": "DEPLOYED", "version": "1.30", "is_default": True, "production": False,
    "instance_type": "t3a.large", "min_running_nodes": 3, "max_running_nodes": 10, "disk_size": 50,
    "secret_manager_accesses": [{"id": "sma-1", "name": "my-sm", "created_at": "2024-05-01T10:00:00Z",
        "updated_at": "2024-05-01T10:00:00Z", "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"},
        "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}]},
    {"id": "cluster-2", "created_at": "2024-05-01T10:00:00Z", "organization": {"id": "org-1"}, "name": "Prod-Cluster",
     "region": "eu-west-3", "cloud_provider": "AWS"}]}
COMMITS = {"results": [
    {"created_at": "2024-06-01T10:00:00Z", "git_commit_id": "old111", "tag": "", "message": "old", "author_name": "me"},
    {"created_at": "2024-06-03T10:00:00Z", "git_commit_id": "new999", "tag": "", "message": "newest commit\nbody", "author_name": "me"}]}

FIXTURES = [
    {"method": "GET", "path": "/environment/env-1/deploymentHistory", "status": 200, "body": HISTORY},
    logs_fx(LOGS),
    {"method": "GET", "path": "/environment/env-1/deploymentStage", "status": 200, "body": STAGES},
    {"method": "POST", "path": "/environment/env-1/deploymentStage", "status": 201, "body": STAGE_RESP},
    {"method": "PUT", "path_regex": "^/deploymentStage/[^/]+/service/[^/]+$", "status": 200, "body": {"results": []}},
    {"method": "PUT", "path_regex": "^/deploymentStage/[^/]+$", "status": 200, "body": STAGE_RESP},
    {"method": "DELETE", "path_regex": "^/deploymentStage/[^/]+$", "status": 204},
    {"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200, "body": ENV_STATUS},
    {"method": "POST", "path": "/environment/env-1/clone", "status": 201, "body": CLONED},
    {"method": "DELETE", "path": "/environment/env-1", "status": 204},
    {"method": "POST", "path": "/environment/env-1/deploy", "status": 202, "body": SVC_STATUS},
    {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202, "body": ENV_STATUS},
    {"method": "POST", "path": "/environment/env-1/stop", "status": 202, "body": ENV_STATUS},
    {"method": "PUT", "path": "/environment/env-1", "status": 200, "body": ENVIRONMENT},
    {"method": "GET", "path": "/variable", "query": {"parent_id": "env-1", "scope": "ENVIRONMENT"}, "status": 200, "body": VARS},
    {"method": "POST", "path": "/variable", "status": 201, "body": VAR_RESP},
    {"method": "PUT", "path_regex": "^/variable/var-[0-9]+$", "status": 200, "body": VAR_RESP},
    {"method": "DELETE", "path_regex": "^/variable/var-[0-9]+$", "status": 204},
    {"method": "POST", "path_regex": "^/variable/var-[0-9]+/alias$", "status": 201, "body": VAR_RESP},
    {"method": "POST", "path_regex": "^/variable/var-[0-9]+/override$", "status": 201, "body": VAR_RESP},
    {"method": "GET", "path_regex": "^/(application|job|helm)/[^/]+/commit$", "status": 200, "body": COMMITS},
]

def err(method, path, status=500, body=None):
    return {"method": method, "path": path, "status": status, "body": body if body is not None else {"message": "boom"}}

E = "environment"
cases = []
def case(name, args, **kw):
    c = {"name": name, "args": args}
    c.update(kw)
    cases.append(c)

# groups / generic
case("environment (group)", [E])
case("environment unknown sub", [E, "nope"])
case("deployment (group)", [E, "deployment"])
case("env (group)", [E, "env"])
case("env alias (group)", [E, "env", "alias"])
case("env override (group)", [E, "env", "override"])
case("external-secret (group)", [E, "external-secret"])
case("stage (group)", [E, "stage"])

# list
case("list", [E, "list"])
case("list --json", [E, "list", "--json"])
case("list --organization --project", [E, "list", "--organization", "my-org", "--project", "my-project"])
case("list unknown project", [E, "list", "--project", "nope"])
case("list unknown organization", [E, "list", "--organization", "nope", "--project", "my-project"])
case("list empty", [E, "list"], fixtures=[{"method": "GET", "path": "/project/proj-1/environment", "body": {"results": []}}])
case("list --json empty", [E, "list", "--json"], fixtures=[{"method": "GET", "path": "/project/proj-1/environment", "body": {"results": []}}])
case("list api error", [E, "list"], fixtures=[err("GET", "/project/proj-1/environment")])
case("list status api error", [E, "list"], fixtures=[err("GET", "/project/proj-1/environment/status", 403)])
case("list no status", [E, "list"], fixtures=[{"method": "GET", "path": "/project/proj-1/environment/status", "body": {"results": []}}])
case("list no token", [E, "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
case("list extra args", [E, "list", "foo"])
case("list unknown flag", [E, "list", "--nope"])

# statuses
case("statuses", [E, "statuses"])
case("statuses --json", [E, "statuses", "--json"])
case("statuses --environment", [E, "statuses", "--environment", "my-env"])
case("statuses unknown environment", [E, "statuses", "--environment", "nope"])
case("statuses api error", [E, "statuses"], fixtures=[err("GET", "/environment/env-1/statuses", 404)])
case("statuses --json nulls", [E, "statuses", "--json"], fixtures=[{"method": "GET", "path": "/environment/env-1/statuses",
    "body": {"environment": {"id": "env-1", "state": "STOPPED", "last_deployment_state": "STOPPED", "last_deployment_date": None,
             "origin": "CLI", "extra_field": 1, "deployment_status": None},
             "applications": [dict(status("app-1", "STOPPED"), last_deployment_date="2024-06-02T11:30:00.500+00:00", execution_id=None)],
             "containers": None, "jobs": [], "databases": [], "helms": [], "terraforms": [], "agentic_workflows": None}}])

# cancel
case("cancel", [E, "cancel"])
case("cancel --force", [E, "cancel", "--force"])
case("cancel api error (silent)", [E, "cancel"], fixtures=[err("POST", "/environment/env-1/cancelDeployment", 400)])
# the EnvironmentStatus response is decoded: Go exits 1 silently when it cannot
case("cancel empty body", [E, "cancel"], fixtures=[{"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200}])
case("cancel syntax error", [E, "cancel"], fixtures=[{"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200,
                                                     "body_raw": "nope", "headers": {"Content-Type": "application/json"}}])
case("cancel text body", [E, "cancel"], fixtures=[{"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200, "body_raw": "x"}])
case("cancel type mismatch", [E, "cancel"], fixtures=[{"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200,
                                                      "body": {"id": 1, "state": "CANCELING", "last_deployment_state": "DEPLOYED"}}])
case("cancel null body", [E, "cancel"], fixtures=[{"method": "POST", "path": "/environment/env-1/cancelDeployment", "status": 200, "body": None}])
case("cancel --watch", [E, "cancel", "-w"], fixtures=[statuses_fx("CANCELING", "CANCELED")])
case("cancel unknown environment", [E, "cancel", "--environment", "nope"])

# clone
case("clone", [E, "clone", "-n", "new-env"])
case("clone all flags", [E, "clone", "-o", "my-org", "-p", "my-project", "-e", "my-env", "-n", "new-env",
                         "-c", "prod-cluster", "-t", "staging", "--apply-deployment-rule", "--target-project", "my-project"])
case("clone unknown cluster and type", [E, "clone", "-n", "new-env", "--cluster", "nope", "--environment-type", "weird"])
case("clone type production", [E, "clone", "-n", "new-env", "-t", "Production"])
case("clone type development", [E, "clone", "-n", "new-env", "-t", "development"])
case("clone unknown target project", [E, "clone", "-n", "new-env", "--target-project", "nope"])
case("clone clusters api error", [E, "clone", "-n", "new-env", "--cluster", "x"], fixtures=[err("GET", "/organization/org-1/cluster")])
case("clone api error", [E, "clone", "-n", "new-env"], fixtures=[err("POST", "/environment/env-1/clone", 400, {"message": "name already used"})])
case("clone missing --new-environment-name", [E, "clone"])
case("clone unknown environment", [E, "clone", "-n", "x", "-e", "nope"])

# delete / redeploy / stop
case("delete", [E, "delete"])
case("delete --environment", [E, "delete", "--environment", "my-env", "--project", "my-project", "--organization", "my-org"])
case("delete api error", [E, "delete"], fixtures=[err("DELETE", "/environment/env-1", 403)])
case("delete --watch", [E, "delete", "-w"], fixtures=[statuses_fx("DELETING", "DELETED")], timeout=40)
case("delete unknown environment", [E, "delete", "--environment", "nope"])
case("redeploy", [E, "redeploy"])
case("redeploy api error", [E, "redeploy"], fixtures=[err("POST", "/environment/env-1/deploy")])
case("redeploy --watch error", [E, "redeploy", "--watch"], fixtures=[statuses_fx("DEPLOYING", "DEPLOYMENT_ERROR")], timeout=40)
case("stop", [E, "stop"])
case("stop api error", [E, "stop"], fixtures=[err("POST", "/environment/env-1/stop", 400)])
case("stop --watch", [E, "stop", "-w"], fixtures=[statuses_fx("STOPPED")])
case("stop no token", [E, "stop"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# statuses: a response the Go client cannot decode fails (the error text differs: serde vs Go, not compared)
for js in ([], ["--json"]):
    case("statuses type mismatch %s" % js, [E, "statuses"] + js, ignore=["stdout"],
         fixtures=[{"method": "GET", "path": "/environment/env-1/statuses",
                    "body": dict(statuses("DEPLOYED"), applications="x")}])
# deploy
case("deploy", [E, "deploy"])
case("deploy api error", [E, "deploy"], fixtures=[err("POST", "/environment/env-1/deploy", 409)])
case("deploy --watch", [E, "deploy", "-w"], fixtures=[statuses_fx("DEPLOYING", "DEPLOYED")], timeout=40)
case("deploy --skip-paused-services", [E, "deploy", "--skip-paused-services"])
case("deploy --skip-paused-services all stopped", [E, "deploy", "--skip-paused-services"], fixtures=[statuses_fx("STOPPED")])
case("deploy --skip-paused-services statuses error", [E, "deploy", "--skip-paused-services"], fixtures=[err("GET", "/environment/env-1/statuses")])
case("deploy --skip-paused-services with --applications", [E, "deploy", "--skip-paused-services", "--applications", "my-app"])
case("deploy --services", [E, "deploy", "--services",
     '{"applications":[{"application_id":"app-1","git_commit_id":null}],"databases":["db-1"],"jobs":null,"extra":{"a":1}}'])
case("deploy --services invalid json", [E, "deploy", "--services", "{nope"])
# --services is decoded like the Go client models (type errors, required properties, null handling, key case)
for i, services in enumerate([
        '[]', '{"applications":"x"}', '{"DATABASES":"x"}', '{"databases":["a",1]}', '{"applications":[1]}',
        '{"applications":[null]}', '{"containers":[{"image_tag":"t"}]}', '{"applications":[{"git_commit_id":5,"application_id":2}]}',
        '{"jobs":[1]}', '{"terraforms":[{"id":3}]}', '{"databases":"x","applications":[{}]}', '{"applications":[{}],"databases":"x"}',
        '{"applications":[{"application_id":null}],"databases":[null,"d"],"jobs":[null,{"id":null}],"helms":null}',
        '{"terraforms":[{"id":null,"action":null,"git_commit_id":null}],"Databases":["a"],"x":null}',
        '{"jobs":[{"ID":"j","image_tag":null}],"applications":[{"application_id":"a","GIT_COMMIT_ID":"b"}]}']):
    case("deploy --services decoding %d" % i, [E, "deploy", "--services", services])
case("deploy --services api error", [E, "deploy", "--services", '{"databases":["db-1"]}'],
     fixtures=[err("POST", "/environment/env-1/service/deploy", 400)])
case("deploy --applications", [E, "deploy", "--applications", "my-app"])
case("deploy --applications commit", [E, "deploy", "--applications", "my-app:abc,my-app:latest"])
case("deploy --applications unknown", [E, "deploy", "--applications", "nope"])
case("deploy --applications commit api error", [E, "deploy", "--applications", "my-app:latest"],
     fixtures=[err("GET", "/application/app-1/commit")])
case("deploy --containers", [E, "deploy", "--containers", "my-container:v2,my-container"])
case("deploy --containers unknown", [E, "deploy", "--containers", "nope:v1"])
case("deploy --containers git container", [E, "deploy", "--containers", "git-container,my-container:v2"],
     fixtures=[GIT_CONTAINERS])
case("deploy --containers git container tag", [E, "deploy", "--containers", "my-container:v2,git-container:v3"],
     fixtures=[GIT_CONTAINERS])
case("deploy --cronjobs", [E, "deploy", "--cronjobs", "my-cron:latest"])
case("deploy --cronjobs commit", [E, "deploy", "--cronjobs", "my-cron:abc"])
case("deploy --lifecycles", [E, "deploy", "--lifecycles", "my-lifecycle:1.2"])
case("deploy --lifecycles and --cronjobs", [E, "deploy", "--lifecycles", "my-lifecycle", "--cronjobs", "my-cron"])
case("deploy --lifecycles unknown", [E, "deploy", "--lifecycles", "nope"])
case("deploy --cronjobs unknown", [E, "deploy", "--cronjobs", "nope"])
case("deploy --lifecycles with a cron job (Go nil pointer panic)", [E, "deploy", "--lifecycles", "my-cron"], ignore=["stderr"])
case("deploy --helms", [E, "deploy", "--helms", "my-helm:19.0.0"])
case("deploy --helms git latest", [E, "deploy", "--helms", "my-helm:latest"], fixtures=[{"method": "GET", "path": "/environment/env-1/helm",
     "body": {"results": [{"id": "helm-1", "created_at": "2024-05-01T10:00:00Z", "environment": {"id": "env-1"}, "name": "my-helm",
              "auto_preview": False, "auto_deploy": False, "arguments": [], "allow_cluster_wide_resources": False,
              "values_override": {"set": [], "set_string": [], "set_json": [], "file": None}, "icon_uri": "x", "service_type": "HELM",
              "source": {"git": {"git_repository": {"provider": "GITHUB", "owner": "me", "url": "https://github.com/me/h.git",
                                                    "name": "me/h", "branch": "main", "root_path": "/", "has_access": True}}}}]}}])
case("deploy --helms unknown", [E, "deploy", "--helms", "nope"])
case("deploy --helms api error", [E, "deploy", "--helms", "my-helm"], fixtures=[err("GET", "/environment/env-1/helm")])
case("deploy services api error", [E, "deploy", "--containers", "my-container"], fixtures=[err("POST", "/environment/env-1/service/deploy")])
case("deploy unknown environment", [E, "deploy", "--environment", "nope"])

# update
case("update", [E, "update", "-e", "my-env"])
case("update --name --type", [E, "update", "--environment", "my-env", "--name", "renamed", "--type", "production"])
case("update --type unknown", [E, "update", "--environment", "my-env", "--type", "preview"])
case("update unknown environment", [E, "update", "--environment", "nope"])
case("update no --environment", [E, "update"])
case("update api error", [E, "update", "-e", "my-env"], fixtures=[err("PUT", "/environment/env-1", 400)])
case("update list api error", [E, "update", "-e", "my-env"], fixtures=[err("GET", "/project/proj-1/environment")])
case("update unknown project", [E, "update", "-e", "my-env", "-p", "nope"])

# deployment list
case("deployment list", [E, "deployment", "list"])
case("deployment list --json", [E, "deployment", "list", "--json"])
case("deployment list empty", [E, "deployment", "list"], fixtures=[{"method": "GET", "path": "/environment/env-1/deploymentHistory", "body": {"page": 1, "page_size": 10, "results": []}}])
case("deployment list --json empty", [E, "deployment", "list", "--json"], fixtures=[{"method": "GET", "path": "/environment/env-1/deploymentHistory", "body": {"page": 1, "page_size": 10, "results": []}}])
case("deployment list api error", [E, "deployment", "list"], fixtures=[err("GET", "/environment/env-1/deploymentHistory")])
case("deployment list unknown environment", [E, "deployment", "list", "--environment", "nope"])

# deployment logs
D = [E, "deployment", "logs"]
case("logs", D)
case("logs --json", D + ["--json"])
case("logs --execution-id", D + ["--execution-id", "exec-9"], fixtures=[logs_fx(LOGS_OK[:3], "exec-9")])
case("logs --id (KNOWN DRIFT: pflag normalize alias of --execution-id, unsupported by qovery_cobra)", D + ["--id", "exec-9"],
     fixtures=[logs_fx(LOGS_OK[:3], "exec-9")])
case("logs --service", D + ["--service", "my-app"])
case("logs -s terraform --json", D + ["-s", "S3", "--json"])
case("logs --service-id", D + ["--service-id", "tf-1"])
case("logs --service unknown", D + ["--service", "nope"])
case("logs --service unknown env only", D + ["--service", "nope"], fixtures=[logs_fx(LOGS_ENV_ONLY)])
case("logs --pre-check", D + ["--pre-check"])
case("logs --errors-only", D + ["--errors-only"])
case("logs --errors-only --json", D + ["--errors-only", "--json"])
case("logs --errors-only --service", D + ["--errors-only", "--service", "my-app"])
case("logs --errors-only no errors", D + ["--errors-only"], fixtures=[logs_fx(LOGS_OK)])
case("logs --errors-only --service no errors", D + ["--errors-only", "-s", "S3"])
case("logs --errors-only --service unknown", D + ["--errors-only", "-s", "nope"])
case("logs --pre-check --errors-only", D + ["--pre-check", "--errors-only"])
case("logs --pre-check with --service", D + ["--pre-check", "--service", "my-app"])
case("logs --service and --service-id", D + ["--service", "my-app", "--service-id", "app-1"])
case("logs empty", D, fixtures=[logs_fx([])])
case("logs empty --json", D + ["--json"], fixtures=[logs_fx([])])
case("logs null --json", D + ["--json"], fixtures=[logs_fx(None)])
case("logs api error", D, fixtures=[err("GET", "/environment/env-1/logs")])
case("logs unknown environment", D + ["--environment", "nope"])
case("logs no token", D, env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# deployment explain
X = [E, "deployment", "explain"]
case("explain", X)
case("explain --id", X + ["--id", "exec-9"], fixtures=[logs_fx(LOGS_OK[:5], "exec-9")])
for lvl in ["all", "stage", "service", "step", "message", ""]:
    case("explain --level " + (lvl or "(empty)"), X + ["--level", lvl])
case("explain invalid level", X + ["--level", "nope"])
case("explain empty logs", X, fixtures=[logs_fx([])])
case("explain env api error", X, fixtures=[err("GET", "/environment/env-1", 404)])
case("explain logs api error", X, fixtures=[err("GET", "/environment/env-1/logs")])
case("explain unknown environment", X + ["--environment", "nope"])

# stage
S = [E, "stage"]
case("stage list", S + ["list"])
case("stage list --json", S + ["list", "--json"])
case("stage list empty", S + ["list"], fixtures=[{"method": "GET", "path": "/environment/env-1/deploymentStage", "body": {"results": []}}])
case("stage list --json empty", S + ["list", "--json"], fixtures=[{"method": "GET", "path": "/environment/env-1/deploymentStage", "body": {"results": []}}])
case("stage list name api error", S + ["list"], fixtures=[err("GET", "/application/app-1")])
case("stage list api error", S + ["list"], fixtures=[err("GET", "/environment/env-1/deploymentStage")])
case("stage create", S + ["create", "-n", "new"])
case("stage create --description", S + ["create", "--name", "new", "-d", "a stage"])
case("stage create api error", S + ["create", "-n", "new"], fixtures=[err("POST", "/environment/env-1/deploymentStage", 400)])
case("stage create missing --name", S + ["create"])
case("stage delete", S + ["delete", "-n", "second"])
case("stage delete unknown", S + ["delete", "-n", "nope"])
case("stage delete api error", S + ["delete", "-n", "first"], fixtures=[err("DELETE", "/deploymentStage/stage-1")])
case("stage delete list api error", S + ["delete", "-n", "first"], fixtures=[err("GET", "/environment/env-1/deploymentStage")])
case("stage edit", S + ["edit", "-n", "first", "--new-name", "premier"])
case("stage edit --new-description", S + ["edit", "-n", "first", "--new-name", "premier", "--new-description", "desc"])
case("stage edit unknown", S + ["edit", "-n", "nope", "--new-name", "x"])
case("stage edit api error", S + ["edit", "-n", "first", "--new-name", "x"], fixtures=[err("PUT", "/deploymentStage/stage-1", 400)])
case("stage edit missing --new-name", S + ["edit", "-n", "first"])
case("stage move", S + ["move", "-n", "my-cron", "-s", "first"])
case("stage move app", S + ["move", "--name", "my-app", "--stage", "second"])
case("stage move unknown service", S + ["move", "-n", "nope", "-s", "first"])
case("stage move helm (unsupported type)", S + ["move", "-n", "my-helm", "-s", "first"])
case("stage move lifecycle after helm", S + ["move", "-n", "my-lifecycle", "-s", "first"])
case("stage move unknown stage", S + ["move", "-n", "my-app", "-s", "nope"])
case("stage move api error", S + ["move", "-n", "my-app", "-s", "second"], fixtures=[err("PUT", "/deploymentStage/stage-2/service/app-1")])
case("stage move service api error", S + ["move", "-n", "my-cron", "-s", "first"], fixtures=[err("GET", "/application/app-1")])
case("stage move missing flags", S + ["move"])
case("stage skip", S + ["skip", "-n", "my-container"])
case("stage skip db", S + ["skip", "-n", "my-db"])
case("stage skip unknown", S + ["skip", "-n", "nope"])
case("stage skip api error", S + ["skip", "-n", "my-app"], fixtures=[err("PUT", "/deploymentStage/stage-1/service/app-1", 403)])
case("stage skip missing --name", S + ["skip"])
case("stage unskip", S + ["unskip", "--name", "my-db"])
case("stage unskip unknown", S + ["unskip", "--name", "my-helm"])

# env
V = [E, "env"]
PE = ["--project", "my-project", "--environment", "my-env"]
# without --sort, Go iterates over a map (random order): only one variable for those cases
ONE_VAR = [{"method": "GET", "path": "/variable", "body": {"results": VARS["results"][:1]}}]
case("env list", V + ["list"] + PE, fixtures=ONE_VAR)
case("env list --sort", V + ["list", "--sort"] + PE)
case("env list --show-values --sort", V + ["list", "--show-values", "--sort"] + PE)
case("env list --pretty-print", V + ["list", "--pretty-print"] + PE, fixtures=ONE_VAR)
case("env list --pretty-print --sort", V + ["list", "--pretty-print", "--sort"] + PE)
case("env list --json", V + ["list", "--json"] + PE, fixtures=ONE_VAR)
case("env list --json --sort", V + ["list", "--json", "--sort"] + PE)
case("env list empty", V + ["list"] + PE, fixtures=[{"method": "GET", "path": "/variable", "body": {"results": []}}])
case("env list api error", V + ["list"] + PE, fixtures=[err("GET", "/variable")])
case("env list unknown environment", V + ["list", "--project", "my-project", "--environment", "nope"])
case("env list unknown project", V + ["list", "--project", "nope", "--environment", "my-env"])
case("env list blank project (Go nil pointer panic)", V + ["list", "--project", " ", "--environment", "my-env"], ignore=["stderr"])
case("env list missing flags", V + ["list"])
case("env list projects api error", V + ["list"] + PE, fixtures=[err("GET", "/organization/org-1/project")])
case("env create", V + ["create", "-k", "NEW_KEY", "-v", "val"] + PE)
case("env create --secret --override-if-exists", V + ["create", "-k", "NEW_KEY", "-v", "val", "--secret", "--override-if-exists", "--scope", "PROJECT"] + PE)
case("env create api error", V + ["create", "-k", "NEW_KEY", "-v", "val"] + PE,
     fixtures=[err("POST", "/variable", 409, {"status": 409, "detail": "key FOO already exists at PROJECT scope"})])
case("env create missing --value", V + ["create", "-k", "NEW_KEY"] + PE)
case("env update", V + ["update", "-k", "FOO", "-v", "new"] + PE)
case("env update unknown key", V + ["update", "-k", "NOPE", "-v", "new"] + PE)
case("env update api error", V + ["update", "-k", "FOO", "-v", "new"] + PE, fixtures=[err("PUT", "/variable/var-1")])
case("env delete", V + ["delete", "-k", "FOO"] + PE)
case("env delete unknown key", V + ["delete", "-k", "NOPE"] + PE)
case("env delete api error", V + ["delete", "-k", "FOO"] + PE, fixtures=[err("DELETE", "/variable/var-1", 404)])
case("env alias create", V + ["alias", "create", "-k", "FOO", "--alias", "FOO2"] + PE)
case("env alias create --scope PROJECT", V + ["alias", "create", "-k", "FOO", "--alias", "FOO2", "--scope", "PROJECT"] + PE)
case("env alias create bad scope", V + ["alias", "create", "-k", "FOO", "--alias", "FOO2", "--scope", "WEIRD"] + PE)
case("env alias create unknown key", V + ["alias", "create", "-k", "NOPE", "--alias", "FOO2"] + PE)
case("env alias create api error", V + ["alias", "create", "-k", "FOO", "--alias", "X"] + PE, fixtures=[err("POST", "/variable/var-1/alias", 400)])
case("env alias create missing --alias", V + ["alias", "create", "-k", "FOO"] + PE)
case("env override create", V + ["override", "create", "-k", "PROJ_VAR", "--value", "y"] + PE)
case("env override create unknown key", V + ["override", "create", "-k", "NOPE", "--value", "y"] + PE)
case("env override create api error", V + ["override", "create", "-k", "FOO", "--value", "y"] + PE, fixtures=[err("POST", "/variable/var-1/override")])
case("env override create missing --value", V + ["override", "create", "-k", "FOO"] + PE)
case("env create no token", V + ["create", "-k", "A", "-v", "b"] + PE, env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# external-secret
XS = [E, "external-secret"]
SMFX = [{"method": "GET", "path": "/organization/org-1/cluster", "status": 200, "body": CLUSTERS_SM}]
case("external-secret create", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm"] + PE, fixtures=SMFX)
case("external-secret create --mount-path --scope", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm",
     "--mount-path", "/etc/x", "--scope", "PROJECT"] + PE, fixtures=SMFX)
case("external-secret create unknown sm", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "nope"] + PE, fixtures=SMFX)
case("external-secret create no sm in cluster", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm"] + PE)
case("external-secret create unknown project", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm",
     "--project", "nope", "--environment", "my-env"], fixtures=SMFX)
case("external-secret create blank project", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm",
     "--project", " ", "--environment", "my-env"], fixtures=SMFX)
case("external-secret create unknown environment", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm",
     "--project", "my-project", "--environment", "nope"], fixtures=SMFX)
case("external-secret create api error", XS + ["create", "-k", "EXT2", "-r", "arn:x", "--secret-manager-access-name", "my-sm"] + PE,
     fixtures=SMFX + [err("POST", "/variable", 400, {"detail": "bad reference"})])
case("external-secret create missing flags", XS + ["create"] + PE)
case("external-secret update", XS + ["update", "-k", "EXT", "-r", "arn:y", "--secret-manager-access-name", "my-sm"] + PE, fixtures=SMFX)
case("external-secret update unknown key", XS + ["update", "-k", "NOPE", "-r", "arn:y", "--secret-manager-access-name", "my-sm"] + PE, fixtures=SMFX)
case("external-secret update no sm name", XS + ["update", "-k", "EXT", "-r", "arn:y"] + PE, fixtures=SMFX)
case("external-secret update api error", XS + ["update", "-k", "EXT", "--secret-manager-access-name", "my-sm"] + PE,
     fixtures=SMFX + [err("PUT", "/variable/var-3")])
case("external-secret update env api error", XS + ["update", "-k", "EXT", "--secret-manager-access-name", "my-sm"] + PE,
     fixtures=SMFX + [err("GET", "/environment/env-1", 404)])
case("external-secret delete", XS + ["delete", "-k", "EXT"] + PE)
case("external-secret delete unknown key", XS + ["delete", "-k", "NOPE"] + PE)
case("external-secret delete unknown project", XS + ["delete", "-k", "EXT", "--project", "nope", "--environment", "my-env"])
case("external-secret delete missing --key", XS + ["delete"] + PE)

json.dump({"fixtures": FIXTURES, "cases": cases}, open(OUT, "w"), indent=1)
print(len(cases), "cases")
