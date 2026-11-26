#!/usr/bin/env python3
"""Generates parity/scenarios/container.json.

Usage: python3 parity/generators/container.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import copy
import json

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "container.json")

TS = "2024-05-01T10:00:00.123456Z"
UPD = "2024-06-02T11:30:00Z"


def container(cid, name, **extra):
    c = {
        "id": cid, "created_at": TS, "updated_at": UPD, "image_name": "nginx", "tag": "1.25",
        "registry": {"id": "reg-1", "name": "Docker Hub", "url": "https://docker.io", "kind": "DOCKER_HUB"},
        "environment": {"id": "env-1"}, "maximum_cpu": 4000, "maximum_memory": 8192, "maximum_gpu": 0,
        "name": name, "cpu": 250, "memory": 256, "gpu": 0, "min_running_instances": 1, "max_running_instances": 1,
        "healthchecks": {"checks": []}, "auto_preview": False, "icon_uri": "app://qovery-console/container",
        "service_type": "CONTAINER", "auto_deploy": False, "ports": [], "arguments": [],
        "annotations_groups": [], "labels_groups": [],
    }
    c.update(extra)
    return c


CONT1 = container("cont-1", "my-container")
CONT2 = container(
    "cont-2", "other-container",
    updated_at="2024-07-03T08:15:30.5+02:00",
    description="second container",
    entrypoint="/bin/sh",
    arguments=["-c", "run"],
    auto_deploy=None,
    auto_preview=True,
    cpu=500, memory=512, min_running_instances=2, max_running_instances=4,
    ports=[
        {"id": "port-1", "name": "p80", "internal_port": 80, "external_port": 443, "publicly_accessible": True,
         "is_default": True, "protocol": "HTTP"},
        {"id": "port-2", "internal_port": 9090, "publicly_accessible": False, "protocol": "GRPC"},
    ],
    storage=[{"id": "st-1", "type": "FAST_SSD", "size": 10, "mount_point": "/data"}],
    healthchecks={"readiness_probe": {"type": {"http": {"path": "/", "scheme": "HTTP"}}, "initial_delay_seconds": 30,
                                      "period_seconds": 10, "timeout_seconds": 5, "success_threshold": 1,
                                      "failure_threshold": 3}, "liveness_probe": None},
    autoscaling={"id": "as-1", "created_at": TS, "service_id": "cont-2", "mode": "KEDA", "min_replicas": 1, "max_replicas": 5, "polling_interval_seconds": 30,
                 "cooldown_period_seconds": 300,
                 "scalers": [{"id": "sc-1", "created_at": TS, "scaler_type": "prometheus", "enabled": True, "role": "PRIMARY",
                              "config_json": {"threshold": "10"}}]},
)

CONTAINERS = {"method": "GET", "path": "/environment/env-1/container", "status": 200,
              "body": {"results": [CONT1, CONT2]}}


def status(sid, state, execution_id):
    return {"id": sid, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": execution_id}


ENV_STATUS = {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED",
              "last_deployment_date": UPD, "last_deployment_id": "dep-1", "total_deployment_duration_in_seconds": 42}


def with_stages(*statuses):
    return {"body": {"environment": ENV_STATUS, "stages": [{"containers": list(statuses)}]}}


def watch_fixture(final, ids=("cont-1",), fail=None):
    before = with_stages(*[status(i, "STOPPED", "exec-old-" + i) for i in ids])
    running = with_stages(*[status(i, "DEPLOYING", "exec-new-" + i) for i in ids])
    done = with_stages(*[status(i, fail or final, "exec-new-" + i) for i in ids])
    return {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [before, running, done]}


def statuses_body(env_state, last_state, cont_state):
    return {"environment": {"id": "env-1", "state": env_state, "last_deployment_state": last_state},
            "applications": [], "containers": [status("cont-1", cont_state, "exec-1")], "jobs": [], "databases": [],
            "helms": [], "terraforms": []}


ENV_STATUS_RESP = {"status": 202, "body": {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYING"}}
CONT_STATUS_RESP = {"status": 202, "body": status("cont-1", "DEPLOYING", "exec-1")}


def post(path, resp=None, status_code=202):
    r = {"method": "POST", "path": path}
    r.update(resp or {"status": status_code, "body": {}})
    return r


ERR400 = {"status": 400, "body": {"status": 400, "message": "bad request"}}
ERR500 = {"status": 500, "body": {"status": 500, "message": "boom"}}


def err(method, path, e=ERR500):
    r = {"method": method, "path": path}
    r.update(e)
    return r


LINKS = {"method": "GET", "path": "/container/cont-1/link", "status": 200, "body": {"results": [
    {"service_id": "cont-1", "service_type": "CONTAINER", "url": "https://p80-zabc.example.qovery.io",
     "internal_port": 80, "external_port": 443, "is_qovery_domain": True, "is_default": True},
    {"service_id": "cont-1", "service_type": "CONTAINER", "url": "https://my.custom.com",
     "internal_port": 80, "external_port": 443, "is_qovery_domain": False, "is_default": False},
]}}

DOMAINS = {"method": "GET", "path": "/container/cont-1/customDomain", "status": 200, "body": {"results": [
    {"id": "cd-1", "created_at": TS, "domain": "my.custom.com", "validation_domain": "zabc.example.qovery.io",
     "generate_certificate": True, "status": "VALIDATION_PENDING"},
    {"id": "cd-2", "created_at": TS, "domain": "other.custom.com", "validation_domain": "zdef.example.qovery.io",
     "generate_certificate": False, "use_cdn": True},
]}}


def domain_resp(domain, gen):
    return {"status": 201, "body": {"id": "cd-9", "created_at": TS, "domain": domain, "generate_certificate": gen}}


VARS = {"method": "GET", "path": "/variable", "query": {"parent_id": "cont-1", "scope": "CONTAINER"}, "status": 200,
        "body": {"results": [
            {"id": "var-1", "created_at": TS, "updated_at": UPD, "key": "FOO", "value": "bar", "scope": "CONTAINER",
             "variable_type": "VALUE", "is_secret": False, "service_name": "my-container", "service_id": "cont-1",
             "service_type": "CONTAINER"},
            {"id": "var-2", "created_at": TS, "updated_at": "2024-05-02T10:00:00Z", "key": "API_KEY", "value": None,
             "scope": "CONTAINER", "variable_type": "VALUE", "is_secret": True},
            {"id": "var-3", "created_at": TS, "updated_at": "2024-05-03T10:00:00Z", "key": "PROJ_VAR",
             "value": "p", "scope": "PROJECT", "variable_type": "VALUE", "is_secret": False},
            {"id": "var-4", "created_at": TS, "updated_at": "2024-05-04T10:00:00Z", "key": "ALIAS_FOO", "value": None,
             "scope": "CONTAINER", "variable_type": "ALIAS", "is_secret": False,
             "aliased_variable": {"id": "var-3", "key": "PROJ_VAR", "value": "p", "mount_path": "",
                                  "scope": "PROJECT", "variable_type": "VALUE"}},
            {"id": "var-5", "created_at": TS, "updated_at": "2024-05-05T10:00:00Z", "key": "EXT_SECRET",
             "value": "aws/secret", "scope": "CONTAINER", "variable_type": "EXTERNAL_SECRET", "is_secret": False,
             "secret_manager_access_id": "sma-1"},
        ]}}

VAR_RESP = {"status": 201, "body": {"id": "var-9", "created_at": TS, "key": "NEW", "value": "v", "scope": "CONTAINER",
                                    "variable_type": "VALUE", "is_secret": False}}

def sma(i, n):
    return {"id": i, "name": n, "created_at": TS, "updated_at": UPD,
            "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"},
            "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}


CLUSTERS_SMA = {"method": "GET", "path": "/organization/org-1/cluster", "status": 200, "body": {"results": [
    {"id": "cluster-1", "created_at": TS, "organization": {"id": "org-1"}, "name": "my-cluster",
     "region": "us-east-2", "cloud_provider": "AWS",
     "secret_manager_accesses": [sma("sma-1", "aws-sm"), sma("sma-2", "gcp-sm")]}]}}

REGISTRIES = {"method": "GET", "path": "/organization/org-1/containerRegistry", "status": 200, "body": {"results": [
    {"id": "reg-1", "created_at": TS, "name": "Docker Hub", "kind": "DOCKER_HUB", "url": "https://docker.io"},
    {"id": "reg-2", "created_at": TS, "name": "My <ECR>", "kind": "ECR",
     "url": "https://123.dkr.ecr.us-east-1.amazonaws.com"},
    {"id": "reg-3", "created_at": TS},
]}}

cases = []


def case(name, args, fixtures=None, **kw):
    c = {"name": name, "args": args}
    if fixtures:
        c["fixtures"] = fixtures
    c.update(kw)
    cases.append(c)


# ---- groups --------------------------------------------------------------------------------------------------
case("container (group)", ["container"])
case("container extra arg", ["container", "foo"])
for g in (["domain"], ["env"], ["env", "alias"], ["env", "override"], ["external-secret"], ["registry"]):
    case("container %s (group)" % " ".join(g), ["container"] + g)
case("container unknown flag", ["container", "list", "--nope"])

# ---- list ----------------------------------------------------------------------------------------------------
case("list", ["container", "list"])
case("list --json", ["container", "list", "--json"])
case("list default fixtures", ["container", "list"], [
    {"method": "GET", "path": "/environment/env-1/container", "status": 200, "body": {"results": [CONT1]}}])
case("list empty", ["container", "list"], [
    {"method": "GET", "path": "/environment/env-1/container", "status": 200, "body": {"results": []}}])
case("list --json empty", ["container", "list", "--json"], [
    {"method": "GET", "path": "/environment/env-1/container", "status": 200, "body": {"results": []}}])
case("list with org/project/env flags", ["container", "list", "--organization", "my-org", "--project", "my-project",
                                         "--environment", "my-env"])
case("list unknown environment", ["container", "list", "--organization", "my-org", "--project", "my-project",
                                  "--environment", "nope"])
case("list unknown organization", ["container", "list", "--organization", "nope"])
case("list api error", ["container", "list"], [err("GET", "/environment/env-1/container")])
case("list statuses api error", ["container", "list"], [err("GET", "/environment/env-1/statuses", ERR400)])
case("list no token", ["container", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)

# ---- deploy --------------------------------------------------------------------------------------------------
DEPLOY = post("/environment/env-1/service/deploy", ENV_STATUS_RESP)
case("deploy", ["container", "deploy", "-n", "my-container"], [DEPLOY])
case("deploy --tag", ["container", "deploy", "-n", "my-container", "--tag", "2.0"], [DEPLOY])
case("deploy -t", ["container", "deploy", "-n", "my-container", "-t", "3.0"], [DEPLOY])
case("deploy --containers", ["container", "deploy", "--containers", "my-container, other-container"], [DEPLOY])
case("deploy --containers --tag", ["container", "deploy", "--containers", "my-container,other-container", "-t", "9"],
     [DEPLOY])
case("deploy --containers unknown", ["container", "deploy", "--containers", "my-container, nope"], [DEPLOY])
case("deploy unknown container", ["container", "deploy", "-n", "nope"], [DEPLOY])
case("deploy no container", ["container", "deploy"], [DEPLOY])
case("deploy both flags", ["container", "deploy", "-n", "my-container", "--containers", "other-container"], [DEPLOY])
case("deploy api error", ["container", "deploy", "-n", "my-container"],
     [err("POST", "/environment/env-1/service/deploy", ERR400)])
case("deploy list api error", ["container", "deploy", "-n", "my-container"],
     [err("GET", "/environment/env-1/container")])
case("deploy unknown environment", ["container", "deploy", "-n", "my-container", "--organization", "my-org",
                                    "--project", "my-project", "--environment", "nope"])
case("deploy --watch", ["container", "deploy", "-n", "my-container", "-w"],
     [DEPLOY, watch_fixture("DEPLOYED")], timeout=60)
case("deploy --watch multi", ["container", "deploy", "--containers", "my-container,other-container", "-w"],
     [DEPLOY, watch_fixture("DEPLOYED", ("cont-1", "cont-2"))], timeout=60)
case("deploy --watch error", ["container", "deploy", "-n", "my-container", "--watch"],
     [DEPLOY, watch_fixture("DEPLOYED", fail="DEPLOYMENT_ERROR")], timeout=60)
case("deploy --watch api error on request", ["container", "deploy", "-n", "my-container", "--watch"],
     [err("POST", "/environment/env-1/service/deploy", ERR400), watch_fixture("DEPLOYED")], timeout=60)

# ---- Git containers (source.git_repository) ------------------------------------------------------------------
GIT_REPO = {"provider": "GITHUB", "owner": "acme", "url": "https://github.com/acme/app.git", "name": "acme/app",
            "dockerfile_path": "Dockerfile"}
GIT_CONT = container("cont-g", "git-container", image_name="qovery-built", tag="abc123",
                     source={"git_repository": dict(GIT_REPO, branch="main", deployed_commit_id="abc123")})
GIT_CONT_NO_BRANCH = container("cont-h", "git-nobranch", source={"git_repository": GIT_REPO})
IMAGE_SRC_CONT = container("cont-i", "image-source",
                           source={"image": {"image_name": "nginx", "tag": "1.25", "registry": {
                               "id": "reg-1", "name": "Docker Hub", "url": "https://docker.io", "kind": "DOCKER_HUB"}}})
GIT_CONTAINERS = {"method": "GET", "path": "/environment/env-1/container", "status": 200,
                  "body": {"results": [CONT1, GIT_CONT, GIT_CONT_NO_BRANCH, IMAGE_SRC_CONT]}}
GIT_DEPLOY = post("/container/cont-g/deploy", CONT_STATUS_RESP)
case("list git containers", ["container", "list"], [GIT_CONTAINERS])
case("list git containers --json", ["container", "list", "--json"], [GIT_CONTAINERS])
case("deploy git container", ["container", "deploy", "-n", "git-container"], [GIT_CONTAINERS, DEPLOY])
case("deploy git and image containers", ["container", "deploy", "--containers", "git-container,my-container,image-source"],
     [GIT_CONTAINERS, DEPLOY])
case("deploy git container --tag", ["container", "deploy", "-n", "git-container", "-t", "v2"], [GIT_CONTAINERS, DEPLOY])
case("deploy git container no branch --tag", ["container", "deploy", "-n", "git-nobranch", "-t", "v2"],
     [GIT_CONTAINERS, DEPLOY])
case("deploy --commit-id", ["container", "deploy", "-n", "git-container", "--commit-id", "def456"],
     [GIT_CONTAINERS, GIT_DEPLOY])
case("deploy -c", ["container", "deploy", "-n", "git-container", "-c", "def456"], [GIT_CONTAINERS, GIT_DEPLOY])
case("deploy --commit-id image container", ["container", "deploy", "-n", "my-container", "-c", "def456"],
     [GIT_CONTAINERS, GIT_DEPLOY])
case("deploy --commit-id mixed containers", ["container", "deploy", "--containers", "git-container,my-container",
                                             "-c", "def456"], [GIT_CONTAINERS, GIT_DEPLOY])
case("deploy --commit-id and --tag", ["container", "deploy", "-n", "git-container", "-c", "def456", "-t", "v2"],
     [GIT_CONTAINERS, GIT_DEPLOY])
case("deploy --commit-id latest", ["container", "deploy", "-n", "git-container", "-c", " Latest "],
     [GIT_CONTAINERS, GIT_DEPLOY])
case("deploy --commit-id api error", ["container", "deploy", "-n", "git-container", "-c", "nope"],
     [GIT_CONTAINERS, err("POST", "/container/cont-g/deploy",
                          {"status": 400, "body": {"status": 400, "detail": "commit nope not found"}})])
case("deploy --commit-id api error raw body", ["container", "deploy", "-n", "git-container", "-c", "nope"],
     [GIT_CONTAINERS, err("POST", "/container/cont-g/deploy", ERR400)])
case("deploy --commit-id --watch", ["container", "deploy", "-n", "git-container", "-c", "def456", "-w"],
     [GIT_CONTAINERS, GIT_DEPLOY, watch_fixture("DEPLOYED", ("cont-g",))], timeout=60)
case("deploy git container --tag --watch", ["container", "deploy", "-n", "git-container", "-t", "v2", "-w"],
     [GIT_CONTAINERS, DEPLOY, watch_fixture("DEPLOYED", ("cont-g",))], timeout=60)

# ---- redeploy ------------------------------------------------------------------------------------------------
REDEPLOY = post("/container/cont-1/deploy", CONT_STATUS_RESP)
case("redeploy", ["container", "redeploy", "-n", "my-container"], [REDEPLOY])
case("redeploy missing --container", ["container", "redeploy"])
case("redeploy unknown container", ["container", "redeploy", "-n", "nope"])
case("redeploy api error", ["container", "redeploy", "-n", "my-container"],
     [err("POST", "/container/cont-1/deploy", ERR400)])
case("redeploy --watch", ["container", "redeploy", "-n", "my-container", "-w"],
     [REDEPLOY, watch_fixture("DEPLOYED")], timeout=60)

# ---- restart -------------------------------------------------------------------------------------------------
RESTART = post("/environment/env-1/service/restart-service", ENV_STATUS_RESP)
case("restart", ["container", "restart", "-n", "my-container"], [RESTART])
case("restart --containers", ["container", "restart", "--containers", "my-container,other-container"], [RESTART])
case("restart unknown", ["container", "restart", "-n", "nope"], [RESTART])
case("restart no container", ["container", "restart"], [RESTART])
case("restart api error", ["container", "restart", "-n", "my-container"],
     [err("POST", "/environment/env-1/service/restart-service")])
case("restart --watch", ["container", "restart", "-n", "my-container", "-w"],
     [RESTART, watch_fixture("RESTARTED")], timeout=60)

# ---- stop ----------------------------------------------------------------------------------------------------
STOP = post("/environment/env-1/service/stop")
case("stop", ["container", "stop", "-n", "my-container"], [STOP])
case("stop --containers", ["container", "stop", "--containers", "my-container,other-container"], [STOP])
case("stop unknown", ["container", "stop", "--containers", "nope"], [STOP])
case("stop both flags", ["container", "stop", "-n", "a", "--containers", "b"], [STOP])
case("stop api error", ["container", "stop", "-n", "my-container"], [err("POST", "/environment/env-1/service/stop")])
case("stop --watch", ["container", "stop", "-n", "my-container", "-w"],
     [STOP, watch_fixture("STOPPED")], timeout=60)
case("stop --watch wrong final state", ["container", "stop", "-n", "my-container", "-w"],
     [STOP, watch_fixture("STOPPED", fail="DEPLOYED")], timeout=60)

# ---- delete --------------------------------------------------------------------------------------------------
DELETE = post("/environment/env-1/service/delete")
case("delete", ["container", "delete", "-n", "my-container"], [DELETE])
case("delete --containers", ["container", "delete", "--containers", "my-container,other-container"], [DELETE])
case("delete unknown", ["container", "delete", "-n", "nope"], [DELETE])
case("delete no container", ["container", "delete"], [DELETE])
case("delete api error", ["container", "delete", "-n", "my-container"],
     [err("POST", "/environment/env-1/service/delete", ERR400)])
case("delete --watch", ["container", "delete", "-n", "my-container", "-w"], [DELETE, {
    "method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        with_stages(status("cont-1", "STOPPED", "exec-old")),
        with_stages(status("cont-1", "DELETING", "exec-new")),
        with_stages()]}], timeout=60)
case("delete --watch statuses api error", ["container", "delete", "-n", "my-container", "-w"],
     [DELETE, err("GET", "/environment/env-1/statusesWithStages")], timeout=60)

# ---- cancel --------------------------------------------------------------------------------------------------
CANCEL = post("/environment/env-1/cancelDeployment", ENV_STATUS_RESP)
case("cancel nothing to cancel", ["container", "cancel", "-n", "my-container"])
case("cancel", ["container", "cancel", "-n", "my-container"], [CANCEL, {
    "method": "GET", "path": "/environment/env-1/statuses", "status": 200,
    "body": statuses_body("DEPLOYING", "DEPLOYING", "DEPLOYING")}])
case("cancel --watch", ["container", "cancel", "-n", "my-container", "-w"], [CANCEL, {
    "method": "GET", "path": "/environment/env-1/statuses", "responses": [
        {"body": statuses_body("DEPLOYING", "DEPLOYING", "DEPLOYING")},
        {"body": statuses_body("CANCELING", "CANCELING", "CANCELING")},
        {"body": statuses_body("CANCELED", "CANCELED", "CANCELED")}]}], timeout=60)
case("cancel waits for previous deployment", ["container", "cancel", "-n", "my-container"], [CANCEL, {
    "method": "GET", "path": "/environment/env-1/statuses", "responses": [
        {"body": statuses_body("DEPLOYING", "DEPLOYING", "DEPLOYED")},
        {"body": statuses_body("DEPLOYED", "DEPLOYED", "DEPLOYED")}]}], timeout=60)
case("cancel api error", ["container", "cancel", "-n", "my-container"], [err("POST", "/environment/env-1/cancelDeployment"), {
    "method": "GET", "path": "/environment/env-1/statuses", "status": 200,
    "body": statuses_body("DEPLOYING", "DEPLOYING", "DEPLOYING")}])
case("cancel statuses api error", ["container", "cancel", "-n", "my-container"],
     [err("GET", "/environment/env-1/statuses", ERR400)])
case("cancel unknown container", ["container", "cancel", "-n", "nope"])
case("cancel missing --container", ["container", "cancel"])

# ---- clone ---------------------------------------------------------------------------------------------------
CLONED = container("cont-9", "my-container", environment={"id": "env-1"})
CLONE = post("/container/cont-1/clone", {"status": 201, "body": CLONED})
case("clone", ["container", "clone", "-n", "my-container"], [CLONE])
case("clone target name", ["container", "clone", "-n", "my-container", "--target-container-name", "copy"],
     [post("/container/cont-1/clone", {"status": 201, "body": container("cont-9", "copy")})])
case("clone target project/env", ["container", "clone", "-n", "my-container", "--target-project", "my-project",
                                  "--target-environment", "my-env"], [CLONE])
case("clone unknown target project", ["container", "clone", "-n", "my-container", "--target-project", "nope"], [CLONE])
case("clone unknown target environment", ["container", "clone", "-n", "my-container", "--target-environment", "nope"],
     [CLONE])
case("clone unknown container", ["container", "clone", "-n", "nope"], [CLONE])
case("clone api error", ["container", "clone", "-n", "my-container"],
     [err("POST", "/container/cont-1/clone", ERR400)])
case("clone list api error", ["container", "clone", "-n", "my-container"], [err("GET", "/environment/env-1/container")])
case("clone missing --container", ["container", "clone"])

# ---- create --------------------------------------------------------------------------------------------------
CREATED = container("cont-new", "new-container")
CREATE = {"method": "POST", "path": "/environment/env-1/container", "status": 201, "body": CREATED}
NEW_LINKS = {"method": "GET", "path": "/container/cont-new/link", "status": 200, "body": {"results": [
    {"service_id": "cont-new", "service_type": "CONTAINER", "url": "https://p8080-zabc.example.qovery.io",
     "internal_port": 8080, "external_port": 443, "is_qovery_domain": True, "is_default": True}]}}
CREATE_ARGS = ["container", "create", "-n", "new-container", "--registry", "reg-1", "--image-name", "nginx", "-t", "1.25"]
case("create", CREATE_ARGS, [CREATE, NEW_LINKS])
case("create --json", CREATE_ARGS + ["--json"], [CREATE, NEW_LINKS])
case("create with port", CREATE_ARGS + ["-p", "8080", "--cpu", "1000", "--memory", "1024", "--min-instances", "2",
                                         "--max-instances", "3"], [CREATE, NEW_LINKS])
case("create with port --json", CREATE_ARGS + ["--port", "8080", "--json"], [CREATE, NEW_LINKS])
case("create with port no links", CREATE_ARGS + ["--port", "8080"], [CREATE, {
    "method": "GET", "path": "/container/cont-new/link", "status": 200, "body": {"results": []}}])
case("create with port links api error", CREATE_ARGS + ["--port", "8080", "--json"],
     [CREATE, err("GET", "/container/cont-new/link")])
case("create api error", CREATE_ARGS, [err("POST", "/environment/env-1/container", ERR400)])
case("create api error 500", CREATE_ARGS + ["--json"], [err("POST", "/environment/env-1/container")])
case("create missing flags", ["container", "create", "-n", "x"])
case("create missing all", ["container", "create"])
case("create bad port", CREATE_ARGS + ["--port", "abc"])
case("create unknown environment", CREATE_ARGS + ["--organization", "my-org", "--project", "my-project",
                                                  "--environment", "nope"])

# ---- update --------------------------------------------------------------------------------------------------
UPDATE1 = {"method": "PUT", "path": "/container/cont-1", "status": 200, "body": CONT1}
UPDATE2 = {"method": "PUT", "path": "/container/cont-2", "status": 200, "body": CONT2}
case("update", ["container", "update", "-n", "my-container"], [UPDATE1])
case("update image and tag", ["container", "update", "-n", "my-container", "--image-name", "httpd", "--tag", "2.4"],
     [UPDATE1])
case("update rich container", ["container", "update", "-n", "other-container", "--tag", "5"], [UPDATE2])
case("update auto-deploy", ["container", "update", "-n", "my-container", "--auto-deploy"], [UPDATE1])
case("update auto-deploy false", ["container", "update", "-n", "other-container", "--auto-deploy=false"], [UPDATE2])
case("update auto-preview", ["container", "update", "-n", "my-container", "--auto-preview"], [UPDATE1])
case("update auto-preview false", ["container", "update", "-n", "other-container", "--auto-preview=false"],
     [UPDATE2])
case("update auto-deploy and auto-preview", ["container", "update", "-n", "other-container", "--auto-deploy",
                                             "--auto-preview=false", "--tag", "5"], [UPDATE2])
case("update api error", ["container", "update", "-n", "my-container"], [err("PUT", "/container/cont-1", ERR400)])
case("update api error 500", ["container", "update", "-n", "my-container"], [err("PUT", "/container/cont-1")])
case("update unknown container", ["container", "update", "-n", "nope"])
case("update missing --container", ["container", "update"])
case("update -t is not a flag", ["container", "update", "-n", "my-container", "-t", "1"])

# ---- registry list -------------------------------------------------------------------------------------------
case("registry list", ["container", "registry", "list"], [REGISTRIES])
case("registry list --json", ["container", "registry", "list", "--json"], [REGISTRIES])
case("registry list --organization", ["container", "registry", "list", "--organization", "my-org"], [REGISTRIES])
case("registry list unknown organization", ["container", "registry", "list", "--organization", "nope"], [REGISTRIES])
case("registry list empty --json", ["container", "registry", "list", "--json"], [
    {"method": "GET", "path": "/organization/org-1/containerRegistry", "status": 200, "body": {"results": []}}])
case("registry list api error", ["container", "registry", "list"],
     [err("GET", "/organization/org-1/containerRegistry")])

# ---- domain --------------------------------------------------------------------------------------------------
case("domain list", ["container", "domain", "list", "-n", "my-container"], [DOMAINS, LINKS])
case("domain list --json", ["container", "domain", "list", "-n", "my-container", "--json"], [DOMAINS, LINKS])
case("domain list empty", ["container", "domain", "list", "-n", "my-container"], [
    {"method": "GET", "path": "/container/cont-1/customDomain", "status": 200, "body": {"results": []}},
    {"method": "GET", "path": "/container/cont-1/link", "status": 200, "body": {"results": []}}])
case("domain list --json empty", ["container", "domain", "list", "-n", "my-container", "--json"], [
    {"method": "GET", "path": "/container/cont-1/customDomain", "status": 200, "body": {"results": []}},
    {"method": "GET", "path": "/container/cont-1/link", "status": 200, "body": {"results": []}}])
case("domain list domains api error", ["container", "domain", "list", "-n", "my-container"],
     [err("GET", "/container/cont-1/customDomain"), LINKS])
case("domain list links api error", ["container", "domain", "list", "-n", "my-container"],
     [DOMAINS, err("GET", "/container/cont-1/link", ERR400)])
case("domain list without validation domain (Go nil pointer panic)", ["container", "domain", "list", "-n", "my-container"],
     [{"method": "GET", "path": "/container/cont-1/customDomain", "status": 200, "body": {"results": [
         {"id": "cd-1", "created_at": TS, "domain": "a.b.c", "generate_certificate": True}]}}, LINKS],
     ignore=["stderr"])
case("domain list unknown container", ["container", "domain", "list", "-n", "nope"])
case("domain list missing --container", ["container", "domain", "list"])

DCREATE = post("/container/cont-1/customDomain", domain_resp("new.custom.com", True))
case("domain create", ["container", "domain", "create", "-n", "my-container", "--domain", "new.custom.com"],
     [DOMAINS, DCREATE])
case("domain create no cert cdn", ["container", "domain", "create", "-n", "my-container", "--domain",
                                   "new.custom.com", "--do-not-generate-certificate", "--is-behind-a-cdn"],
     [DOMAINS, post("/container/cont-1/customDomain", domain_resp("new.custom.com", False))])
case("domain create already exists", ["container", "domain", "create", "-n", "my-container", "--domain",
                                      "my.custom.com"], [DOMAINS, DCREATE])
case("domain create api error", ["container", "domain", "create", "-n", "my-container", "--domain", "x.y.z"],
     [DOMAINS, err("POST", "/container/cont-1/customDomain", ERR400)])
case("domain create missing --domain", ["container", "domain", "create", "-n", "my-container"])

DEDIT = {"method": "PUT", "path": "/container/cont-1/customDomain/cd-1", "status": 200,
         "body": {"id": "cd-1", "created_at": TS, "domain": "my.custom.com", "generate_certificate": False}}
case("domain edit", ["container", "domain", "edit", "-n", "my-container", "--domain", "my.custom.com",
                     "--do-not-generate-certificate"], [DOMAINS, DEDIT])
case("domain edit cdn", ["container", "domain", "edit", "-n", "my-container", "--domain", "my.custom.com",
                         "--is-behind-a-cdn"], [DOMAINS, DEDIT])
case("domain edit unknown domain", ["container", "domain", "edit", "-n", "my-container", "--domain", "nope.com"],
     [DOMAINS, DEDIT])
case("domain edit api error", ["container", "domain", "edit", "-n", "my-container", "--domain", "my.custom.com"],
     [DOMAINS, err("PUT", "/container/cont-1/customDomain/cd-1")])
case("domain edit missing all", ["container", "domain", "edit"])

DDEL = {"method": "DELETE", "path": "/container/cont-1/customDomain/cd-2", "status": 204}
case("domain delete", ["container", "domain", "delete", "-n", "my-container", "--domain", "other.custom.com"],
     [DOMAINS, DDEL])
case("domain delete unknown domain", ["container", "domain", "delete", "-n", "my-container", "--domain", "nope.com"],
     [DOMAINS, DDEL])
case("domain delete api error", ["container", "domain", "delete", "-n", "my-container", "--domain",
                                 "other.custom.com"], [DOMAINS, err("DELETE", "/container/cont-1/customDomain/cd-2")])
case("domain delete unknown container", ["container", "domain", "delete", "-n", "nope", "--domain", "a"], [DOMAINS])

# ---- env -----------------------------------------------------------------------------------------------------
EL = ["container", "env", "list", "-n", "my-container"]
# without --sort, Go iterates a map (random order): unsorted tables only have one key group
VARS_ONE = copy.deepcopy(VARS)
VARS_ONE["body"]["results"] = [v for v in VARS_ONE["body"]["results"] if v["key"] in ("PROJ_VAR", "ALIAS_FOO")]
case("env list", EL, [VARS_ONE])
case("env list --show-values", EL + ["--show-values"], [VARS_ONE])
case("env list --pretty-print", EL + ["--pretty-print"], [VARS_ONE])
case("env list --sort --show-values", EL + ["--sort", "--show-values"], [VARS])
case("env list --sort --pretty-print", EL + ["--sort", "--pretty-print"], [VARS])
case("env list --sort", EL + ["--sort"], [VARS])
case("env list all flags", EL + ["--sort", "--pretty-print", "--show-values"], [VARS])
case("env list --json", EL + ["--json"], [VARS])
case("env list --json --sort", EL + ["--json", "--sort"], [VARS])
case("env list empty", EL, [{"method": "GET", "path": "/variable", "status": 200, "body": {"results": []}}])
case("env list --json empty", EL + ["--json"],
     [{"method": "GET", "path": "/variable", "status": 200, "body": {"results": []}}])
case("env list api error", EL, [err("GET", "/variable")])
case("env list unknown container", ["container", "env", "list", "-n", "nope"], [VARS])
case("env list missing --container", ["container", "env", "list"])

VAR_POST = post("/variable", VAR_RESP)
case("env create", ["container", "env", "create", "-n", "my-container", "-k", "NEW", "-v", "val"], [VAR_POST])
case("env create --secret project scope", ["container", "env", "create", "-n", "my-container", "-k", "NEW", "-v",
                                           "val", "--secret", "--scope", "PROJECT"], [VAR_POST])
case("env create environment scope override", ["container", "env", "create", "-n", "my-container", "-k", "NEW",
                                                "-v", "val", "--scope", "ENVIRONMENT", "--override-if-exists"],
     [VAR_POST])
case("env create bad scope", ["container", "env", "create", "-n", "my-container", "-k", "NEW", "-v", "val",
                              "--scope", "NOPE"], [VAR_POST])
case("env create api error", ["container", "env", "create", "-n", "my-container", "-k", "NEW", "-v", "val"],
     [err("POST", "/variable", {"status": 400, "body": {"status": 400, "message": "key already exists"}})])
case("env create missing --value", ["container", "env", "create", "-n", "my-container", "-k", "NEW"])
case("env create unknown container", ["container", "env", "create", "-n", "nope", "-k", "NEW", "-v", "v"])

VAR_PUT = {"method": "PUT", "path": "/variable/var-1", "status": 200, "body": VAR_RESP["body"]}
case("env update", ["container", "env", "update", "-n", "my-container", "-k", "FOO", "-v", "baz"], [VARS, VAR_PUT])
case("env update unknown key", ["container", "env", "update", "-n", "my-container", "-k", "NOPE", "-v", "baz"],
     [VARS, VAR_PUT])
case("env update api error", ["container", "env", "update", "-n", "my-container", "-k", "FOO", "-v", "baz"],
     [VARS, err("PUT", "/variable/var-1")])
case("env update missing --key", ["container", "env", "update", "-n", "my-container", "-v", "baz"])

VAR_DEL = {"method": "DELETE", "path": "/variable/var-1", "status": 204}
case("env delete", ["container", "env", "delete", "-n", "my-container", "-k", "FOO"], [VARS, VAR_DEL])
case("env delete unknown key", ["container", "env", "delete", "-n", "my-container", "-k", "NOPE"], [VARS, VAR_DEL])
case("env delete api error", ["container", "env", "delete", "-n", "my-container", "-k", "FOO"],
     [VARS, err("DELETE", "/variable/var-1", ERR400)])
case("env delete missing --key", ["container", "env", "delete", "-n", "my-container"])

ALIAS = post("/variable/var-3/alias", VAR_RESP)
case("env alias create", ["container", "env", "alias", "create", "-n", "my-container", "-k", "PROJ_VAR",
                          "--alias", "MY_ALIAS"], [VARS, ALIAS])
case("env alias create env scope", ["container", "env", "alias", "create", "-n", "my-container", "-k", "PROJ_VAR",
                                    "--alias", "MY_ALIAS", "--scope", "ENVIRONMENT"], [VARS, ALIAS])
case("env alias create unknown key", ["container", "env", "alias", "create", "-n", "my-container", "-k", "NOPE",
                                      "--alias", "A"], [VARS, ALIAS])
case("env alias create api error", ["container", "env", "alias", "create", "-n", "my-container", "-k", "PROJ_VAR",
                                    "--alias", "A"], [VARS, err("POST", "/variable/var-3/alias", ERR400)])
case("env alias create missing --alias", ["container", "env", "alias", "create", "-n", "my-container", "-k", "X"])

OVERRIDE = post("/variable/var-3/override", VAR_RESP)
case("env override create", ["container", "env", "override", "create", "-n", "my-container", "-k", "PROJ_VAR",
                             "--value", "ov"], [VARS, OVERRIDE])
case("env override create env scope", ["container", "env", "override", "create", "-n", "my-container", "-k",
                                       "PROJ_VAR", "--value", "ov", "--scope", "ENVIRONMENT"], [VARS, OVERRIDE])
case("env override create unknown key", ["container", "env", "override", "create", "-n", "my-container", "-k",
                                         "NOPE", "--value", "ov"], [VARS, OVERRIDE])
case("env override create api error", ["container", "env", "override", "create", "-n", "my-container", "-k",
                                       "PROJ_VAR", "--value", "ov"], [VARS, err("POST", "/variable/var-3/override")])
case("env override -v is not a flag", ["container", "env", "override", "create", "-n", "my-container", "-k", "X",
                                       "-v", "ov"])

# ---- external-secret -----------------------------------------------------------------------------------------
ESC = ["container", "external-secret", "create", "-n", "my-container", "-k", "EXT", "-r", "aws/path",
       "--secret-manager-access-name", "aws-sm"]
case("external-secret create", ESC, [CLUSTERS_SMA, VAR_POST])
case("external-secret create mount path project scope", ESC + ["--mount-path", "/etc/secret", "--scope", "PROJECT"],
     [CLUSTERS_SMA, VAR_POST])
case("external-secret create unknown sma", ["container", "external-secret", "create", "-n", "my-container", "-k",
                                            "EXT", "-r", "x", "--secret-manager-access-name", "nope"],
     [CLUSTERS_SMA, VAR_POST])
case("external-secret create cluster without sma", ESC, [VAR_POST])
case("external-secret create clusters api error", ESC, [err("GET", "/organization/org-1/cluster")])
case("external-secret create api error", ESC, [CLUSTERS_SMA, err("POST", "/variable", ERR400)])
case("external-secret create missing flags", ["container", "external-secret", "create", "-n", "my-container"])

ESU = ["container", "external-secret", "update", "-n", "my-container", "-k", "EXT_SECRET"]
VAR5_PUT = {"method": "PUT", "path": "/variable/var-5", "status": 200, "body": VAR_RESP["body"]}
case("external-secret update reference", ESU + ["-r", "aws/new", "--secret-manager-access-name", "gcp-sm"],
     [CLUSTERS_SMA, VARS, VAR5_PUT])
case("external-secret update no sma name", ESU + ["-r", "aws/new"], [CLUSTERS_SMA, VARS, VAR5_PUT])
case("external-secret update unknown key", ["container", "external-secret", "update", "-n", "my-container", "-k",
                                            "NOPE", "--secret-manager-access-name", "aws-sm"],
     [CLUSTERS_SMA, VARS, VAR5_PUT])
case("external-secret update api error", ESU + ["--secret-manager-access-name", "aws-sm"],
     [CLUSTERS_SMA, VARS, err("PUT", "/variable/var-5")])
case("external-secret update missing --key", ["container", "external-secret", "update", "-n", "my-container"])

case("external-secret delete", ["container", "external-secret", "delete", "-n", "my-container", "-k", "EXT_SECRET"],
     [VARS, {"method": "DELETE", "path": "/variable/var-5", "status": 204}])
case("external-secret delete unknown key", ["container", "external-secret", "delete", "-n", "my-container", "-k",
                                            "NOPE"], [VARS])
case("external-secret delete api error", ["container", "external-secret", "delete", "-n", "my-container", "-k",
                                          "EXT_SECRET"], [VARS, err("DELETE", "/variable/var-5")])
case("external-secret delete unknown container", ["container", "external-secret", "delete", "-n", "nope", "-k", "X"])

json.dump({"fixtures": [CONTAINERS], "cases": cases}, open(OUT, "w"), indent=1)
print("%d cases" % len(cases))
