#!/usr/bin/env python3
"""Generates parity/scenarios/helm.json.

Usage: python3 parity/generators/helm.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import copy
import json

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "helm.json")
T = "2024-05-01T10:00:00.123456Z"
U = "2024-06-02T11:30:00Z"


def helm(hid, name, source, values_override, **extra):
    h = {
        "id": hid, "created_at": T, "updated_at": U, "environment": {"id": "env-1"}, "name": name,
        "auto_preview": False, "auto_deploy": False, "source": source, "arguments": [],
        "allow_cluster_wide_resources": False, "values_override": values_override,
        "icon_uri": "app://qovery-console/helm", "service_type": "HELM", "ports": [],
    }
    h.update(extra)
    return h


REPO_SOURCE = {"repository": {"repository": {"id": "hr-1", "name": "bitnami", "url": "https://charts.bitnami.com/bitnami"},
                              "chart_name": "redis", "chart_version": "18.0.0"}}
GIT_SOURCE = {"git": {"git_repository": {"provider": "GITHUB", "owner": "me", "url": "https://github.com/me/charts.git",
                                         "name": "me/charts", "branch": "main", "root_path": "/charts/app",
                                         "deployed_commit_id": "abc123", "has_access": True, "git_token_id": None}}}
HELM1 = helm("helm-1", "my-helm", REPO_SOURCE, {"set": [], "set_string": [], "set_json": [], "file": None})
HELM2 = helm(
    "helm-2", "git-helm", GIT_SOURCE,
    {"set": [["a", "b"]], "set_string": [], "set_json": [["j", "{}"]],
     "file": {"git": {"git_repository": {"provider": "GITLAB", "owner": "me", "url": "https://gitlab.com/me/values.git",
                                         "name": "me/values", "branch": "prod", "root_path": "/",
                                         "git_token_id": "tok-1", "has_access": True},
                      "paths": ["/values.yaml", "/values-prod.yaml"]}}},
    description="git based chart", timeout_sec=600, auto_deploy=True, arguments=["--wait"],
    allow_cluster_wide_resources=True,
    ports=[
        {"id": "p1", "port_type": "SERVICE_NAME", "name": "http", "internal_port": 80, "external_port": 443,
         "namespace": "apps", "protocol": "HTTP", "is_default": True, "service_name": "web"},
        {"id": "p2", "port_type": "SERVICE_SELECTORS", "internal_port": 8080, "protocol": "GRPC",
         "service_selectors": [{"key": "app", "value": "x"}]},
        {"id": "p3", "port_type": "SERVICE_NAME", "internal_port": 9090, "protocol": "GRPC", "service_name": "metrics",
         "name": None},
    ],
)
HELM3 = helm("helm-3", "raw-helm", REPO_SOURCE,
             {"set_string": [["s", "t"]], "file": {"raw": {"values": [{"name": "override", "content": "replicas: 2\n"},
                                                                       {"name": "extra", "content": "a: b"}]}}},
             )
HELMS = [HELM1, HELM2, HELM3]

HELM_LIST = {"method": "GET", "path": "/environment/env-1/helm", "body": {"results": HELMS}}


def status(sid, state, execution_id):
    return {"id": sid, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": execution_id}


ENV_STATUS = {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED",
              "last_deployment_date": U, "last_deployment_id": "dep-1", "total_deployment_duration_in_seconds": 42}


def stages(*helm_statuses):
    return {"body": {"environment": ENV_STATUS, "stages": [{"helms": list(helm_statuses)}]}}


def env_statuses(env_state, last_state, helms):
    return {"body": {"environment": dict(ENV_STATUS, state=env_state, last_deployment_state=last_state),
                     "applications": [], "containers": [], "jobs": [], "databases": [], "helms": helms,
                     "terraforms": []}}


SERVICE_DEPLOY = {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202,
                  "body": {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYED"}}
SERVICE_STOP = {"method": "POST", "path": "/environment/env-1/service/stop", "status": 202,
                "body": {"id": "env-1", "state": "STOPPING", "last_deployment_state": "DEPLOYED"}}
SERVICE_DELETE = {"method": "POST", "path": "/environment/env-1/service/delete", "status": 202,
                  "body": {"id": "env-1", "state": "DELETING", "last_deployment_state": "DEPLOYED"}}
HELM_DEPLOY = {"method": "POST", "path": "/helm/helm-1/deploy", "status": 202, "body": status("helm-1", "DEPLOYING", "e")}
COMMITS = {"method": "GET", "path": "/helm/helm-2/commit", "body": {"results": [
    {"created_at": "2024-06-01T10:00:00Z", "git_commit_id": "old111", "tag": "", "message": "old", "author_name": "me"},
    {"created_at": "2024-06-03T10:00:00Z", "git_commit_id": "new222", "tag": "", "message": "newest\nbody",
     "author_name": "me"}]}}

CUSTOM_DOMAINS = {"method": "GET", "path": "/helm/helm-1/customDomain", "body": {"results": [
    {"id": "cd-1", "created_at": T, "domain": "app.example.com", "generate_certificate": True,
     "validation_domain": "z1.qovery.io", "status": "VALIDATION_PENDING", "use_cdn": False},
    {"id": "cd-2", "created_at": T, "domain": "cdn.example.com", "generate_certificate": False,
     "validation_domain": "z2.qovery.io", "use_cdn": True}]}}
LINKS = {"method": "GET", "path": "/helm/helm-1/link", "body": {"results": [
    {"service_id": "helm-1", "service_type": "HELM", "url": "https://p80-zabc.qovery.io", "internal_port": 80,
     "external_port": 443, "is_qovery_domain": True, "is_default": True},
    {"service_id": "helm-1", "service_type": "HELM", "url": "https://app.example.com", "internal_port": 80,
     "external_port": 443, "is_qovery_domain": False, "is_default": True}]}}
EMPTY_DOMAINS = {"method": "GET", "path": "/helm/helm-1/customDomain", "body": {"results": []}}
EMPTY_LINKS = {"method": "GET", "path": "/helm/helm-1/link", "body": {"results": []}}
CREATED_DOMAIN = {"method": "POST", "path": "/helm/helm-1/customDomain", "status": 201,
                  "body": {"id": "cd-3", "created_at": T, "domain": "new.example.com", "generate_certificate": False}}
EDITED_DOMAIN = {"method": "PUT", "path": "/helm/helm-1/customDomain/cd-1",
                 "body": {"id": "cd-1", "created_at": T, "domain": "app.example.com", "generate_certificate": True}}
DELETED_DOMAIN = {"method": "DELETE", "path": "/helm/helm-1/customDomain/cd-2", "status": 204}


def var(vid, key, value, scope, vtype="VALUE", is_secret=False, **extra):
    v = {"id": vid, "created_at": "2024-01-02T15:04:05.123456Z", "updated_at": "2024-03-04T05:06:07Z", "key": key,
         "value": value, "mount_path": None, "scope": scope, "variable_type": vtype, "is_secret": is_secret}
    v.update(extra)
    return v


VARS = {"method": "GET", "path": "/variable", "query": {"parent_id": ["helm-1"], "scope": ["HELM"]}, "body": {"results": [
    var("var-1", "FOO", "bar", "HELM", service_id="helm-1", service_name="my-helm", service_type="HELM"),
    var("var-2", "API_KEY", None, "HELM", is_secret=True, service_id="helm-1", service_name="my-helm",
        service_type="HELM"),
    var("var-3", "DB_HOST", "db.local", "ENVIRONMENT"),
    var("var-4", "ALIAS_HOST", "db.local", "HELM", "ALIAS",
        aliased_variable={"id": "var-3", "key": "DB_HOST", "value": "db.local", "mount_path": "",
                          "scope": "ENVIRONMENT", "variable_type": "VALUE"}),
    var("var-5", "PROJECT_VAR", "p", "PROJECT"),
    var("var-6", "EXT", "/path/in/sm", "HELM", "EXTERNAL_SECRET", service_id="helm-1", service_name="my-helm",
        service_type="HELM"),
]}}
VAR_POST = {"method": "POST", "path": "/variable", "status": 201, "body": var("var-9", "NEW", "val", "HELM")}
VAR_PUT1 = {"method": "PUT", "path": "/variable/var-1", "body": var("var-1", "FOO", "baz", "HELM")}
VAR_PUT6 = {"method": "PUT", "path": "/variable/var-6", "body": var("var-6", "EXT", "/sm/ref", "HELM", "EXTERNAL_SECRET")}
VAR_DEL1 = {"method": "DELETE", "path": "/variable/var-1", "status": 204}
VAR_DEL6 = {"method": "DELETE", "path": "/variable/var-6", "status": 204}
VAR_ALIAS = {"method": "POST", "path": "/variable/var-3/alias", "status": 201,
             "body": var("var-10", "MY_ALIAS", "db.local", "HELM", "ALIAS")}
VAR_OVERRIDE = {"method": "POST", "path": "/variable/var-3/override", "status": 201,
                "body": var("var-11", "DB_HOST", "other", "HELM", "OVERRIDE")}
CLUSTERS = {"method": "GET", "path": "/organization/org-1/cluster", "body": {"results": [{
    "id": "cluster-1", "created_at": T, "updated_at": U, "organization": {"id": "org-1"}, "name": "my-cluster",
    "region": "us-east-2", "cloud_provider": "AWS", "kubernetes": "MANAGED", "status": "DEPLOYED", "version": "1.30",
    "is_default": True, "production": False, "instance_type": "t3a.large", "min_running_nodes": 3,
    "max_running_nodes": 10, "disk_size": 50,
    "secret_manager_accesses": [{"id": "sma-1", "name": "my-sm", "created_at": "2024-05-01T10:00:00Z",
                                 "updated_at": "2024-05-01T10:00:00Z",
                                 "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"},
                                 "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}]}]}}

ERR500 = {"status": 500, "body": {"status": 500, "error": "boom"}}
ERR400 = {"status": 400, "body": {"status": 400, "error": "Bad Request", "detail": "invalid chart"}}


def err(method, path, e=ERR500, **kw):
    r = {"method": method, "path": path}
    r.update(e)
    r.update(kw)
    return r


cases = []


def case(name, args, fixtures=None, **kw):
    c = {"name": name, "args": args}
    if fixtures:
        c["fixtures"] = fixtures
    c.update(kw)
    cases.append(c)


# --- group / help
case("helm no args", ["helm"])
case("helm domain no args", ["helm", "domain"])
case("helm env no args", ["helm", "env"])
case("helm env alias no args", ["helm", "env", "alias"])
case("helm env override no args", ["helm", "env", "override"])
case("helm external-secret no args", ["helm", "external-secret"])
case("helm unknown sub", ["helm", "foo"])

# --- list
case("list table", ["helm", "list"])
case("list json", ["helm", "list", "--json"])
case("list default fixture", ["helm", "list"], fixtures=[], ignore=[])
case("list empty json", ["helm", "list", "--json"],
     fixtures=[{"method": "GET", "path": "/environment/env-1/helm", "body": {"results": []}}])
case("list empty table", ["helm", "list"],
     fixtures=[{"method": "GET", "path": "/environment/env-1/helm", "body": {"results": []}}])
case("list with names", ["helm", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
case("list environment not found", ["helm", "list", "--organization", "my-org", "--project", "my-project",
                                     "--environment", "nope"])
case("list project not found", ["helm", "list", "--organization", "my-org", "--project", "nope"])
case("list api error", ["helm", "list"], fixtures=[err("GET", "/environment/env-1/helm")])
case("list statuses error", ["helm", "list"], fixtures=[err("GET", "/environment/env-1/statuses", ERR400)])
case("list no token", ["helm", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)

# --- deploy
case("deploy one", ["helm", "deploy", "-n", "my-helm"], fixtures=[SERVICE_DEPLOY])
case("deploy chart version", ["helm", "deploy", "-n", "my-helm", "--chart_version", "19.0.0"], fixtures=[SERVICE_DEPLOY])
case("deploy multi", ["helm", "deploy", "--helms", "my-helm, git-helm,raw-helm", "--chart_version", "2.0.0",
                      "--chart_git_commit_id", "deadbeef", "--values_override_git_commit_id", "cafe"],
     fixtures=[SERVICE_DEPLOY])
case("deploy git commit latest", ["helm", "deploy", "-n", "git-helm", "--chart_git_commit_id", "latest"],
     fixtures=[SERVICE_DEPLOY, COMMITS])
case("deploy git commit latest no commits", ["helm", "deploy", "-n", "git-helm", "--chart_git_commit_id", "latest"],
     fixtures=[SERVICE_DEPLOY, {"method": "GET", "path": "/helm/helm-2/commit", "body": {"results": []}}])
case("deploy git commit latest api error", ["helm", "deploy", "-n", "git-helm", "--chart_git_commit_id", "latest"],
     fixtures=[SERVICE_DEPLOY, err("GET", "/helm/helm-2/commit")])
case("deploy latest on repository helm", ["helm", "deploy", "-n", "my-helm", "--chart_git_commit_id", "latest"],
     fixtures=[SERVICE_DEPLOY])
case("deploy values override latest", ["helm", "deploy", "-n", "my-helm", "--values_override_git_commit_id", "latest"],
     fixtures=[SERVICE_DEPLOY])
case("deploy not found", ["helm", "deploy", "-n", "nope"])
case("deploy multi not found", ["helm", "deploy", "--helms", "my-helm, nope"])
case("deploy no helm flag", ["helm", "deploy"])
case("deploy both flags", ["helm", "deploy", "-n", "my-helm", "--helms", "git-helm"])
case("deploy api error", ["helm", "deploy", "-n", "my-helm"], fixtures=[err("POST", "/environment/env-1/service/deploy")])
case("deploy list error", ["helm", "deploy", "-n", "my-helm"], fixtures=[err("GET", "/environment/env-1/helm")])
case("deploy watch", ["helm", "deploy", "-n", "my-helm", "-w"], timeout=60, fixtures=[
    SERVICE_DEPLOY,
    {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        stages(status("helm-1", "DEPLOYED", "exec-old")),
        stages(status("helm-1", "DEPLOYING", "exec-new")),
        stages(status("helm-1", "DEPLOYED", "exec-new"))]}])
case("deploy watch multi failure", ["helm", "deploy", "--helms", "my-helm,git-helm", "--watch"], timeout=60, fixtures=[
    SERVICE_DEPLOY,
    {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        stages(status("helm-1", "DEPLOYED", "a0"), status("helm-2", "DEPLOYED", "b0")),
        stages(status("helm-1", "DEPLOYED", "a1"), status("helm-2", "DEPLOYMENT_ERROR", "b1"))]}])
case("deploy watch statuses error", ["helm", "deploy", "-n", "my-helm", "-w"], timeout=60, fixtures=[
    SERVICE_DEPLOY, err("GET", "/environment/env-1/statusesWithStages")])

# --- redeploy
case("redeploy", ["helm", "redeploy", "-n", "my-helm"], fixtures=[HELM_DEPLOY])
case("redeploy missing flag", ["helm", "redeploy"])
case("redeploy not found", ["helm", "redeploy", "-n", "nope"])
case("redeploy api error", ["helm", "redeploy", "-n", "my-helm"], fixtures=[err("POST", "/helm/helm-1/deploy", ERR400)])
case("redeploy watch", ["helm", "redeploy", "-n", "my-helm", "--watch"], timeout=60, fixtures=[
    HELM_DEPLOY,
    {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        stages(status("helm-1", "DEPLOYED", "x0")), stages(status("helm-1", "DEPLOYED", "x1"))]}])

# --- stop
case("stop", ["helm", "stop", "-n", "my-helm"], fixtures=[SERVICE_STOP])
case("stop multi", ["helm", "stop", "--helms", "my-helm,raw-helm"], fixtures=[SERVICE_STOP])
case("stop no flag", ["helm", "stop"])
case("stop both flags", ["helm", "stop", "-n", "a", "--helms", "b"])
case("stop not found", ["helm", "stop", "--helms", "my-helm,ghost"])
case("stop api error", ["helm", "stop", "-n", "my-helm"], fixtures=[err("POST", "/environment/env-1/service/stop")])
case("stop watch", ["helm", "stop", "-n", "my-helm", "-w"], timeout=60, fixtures=[
    SERVICE_STOP,
    {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        stages(status("helm-1", "DEPLOYED", "s0")), stages(status("helm-1", "STOPPED", "s1"))]}])
case("stop watch wrong state", ["helm", "stop", "-n", "my-helm", "-w"], timeout=60, fixtures=[
    SERVICE_STOP,
    {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        stages(status("helm-1", "DEPLOYED", "s0")), stages(status("helm-1", "DEPLOYED", "s1"))]}])

# --- delete
case("delete", ["helm", "delete", "-n", "my-helm"], fixtures=[SERVICE_DELETE])
case("delete multi", ["helm", "delete", "--helms", "my-helm,git-helm"], fixtures=[SERVICE_DELETE])
case("delete no flag", ["helm", "delete"])
case("delete not found", ["helm", "delete", "-n", "ghost"])
case("delete api error", ["helm", "delete", "-n", "my-helm"], fixtures=[err("POST", "/environment/env-1/service/delete")])
case("delete watch", ["helm", "delete", "-n", "my-helm", "-w"], timeout=60, fixtures=[
    SERVICE_DELETE,
    {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [
        stages(status("helm-1", "DEPLOYED", "d0")), {"body": {"environment": ENV_STATUS, "stages": [{"helms": []}]}}]}])

# --- cancel
CANCEL_POST = {"method": "POST", "path": "/environment/env-1/cancelDeployment",
               "body": {"id": "env-1", "state": "CANCELING", "last_deployment_state": "DEPLOYING"}}
case("cancel nothing to cancel", ["helm", "cancel", "-n", "my-helm"])
case("cancel", ["helm", "cancel", "-n", "my-helm"], fixtures=[
    CANCEL_POST, {"method": "GET", "path": "/environment/env-1/statuses",
                  "body": env_statuses("DEPLOYING", "DEPLOYING", [status("helm-1", "DEPLOYING", "e1")])["body"]}])
case("cancel watch", ["helm", "cancel", "-n", "my-helm", "--watch"], timeout=60, fixtures=[
    CANCEL_POST, {"method": "GET", "path": "/environment/env-1/statuses", "responses": [
        env_statuses("DEPLOYING", "DEPLOYING", [status("helm-1", "DEPLOYING", "e1")]),
        env_statuses("DEPLOYED", "CANCELED", [status("helm-1", "CANCELED", "e1")])]}])
case("cancel waiting previous", ["helm", "cancel", "-n", "my-helm"], timeout=60, fixtures=[
    {"method": "GET", "path": "/environment/env-1/statuses", "responses": [
        env_statuses("DEPLOYING", "DEPLOYING", [status("helm-1", "DEPLOYED", "e1")]),
        env_statuses("DEPLOYED", "DEPLOYED", [status("helm-1", "DEPLOYED", "e1")])]}])
case("cancel api error", ["helm", "cancel", "-n", "my-helm"], fixtures=[
    err("POST", "/environment/env-1/cancelDeployment"),
    {"method": "GET", "path": "/environment/env-1/statuses",
     "body": env_statuses("DEPLOYING", "DEPLOYING", [status("helm-1", "DEPLOYING", "e1")])["body"]}])
case("cancel statuses error", ["helm", "cancel", "-n", "my-helm"], fixtures=[err("GET", "/environment/env-1/statuses")])
case("cancel missing flag", ["helm", "cancel"])
case("cancel not found", ["helm", "cancel", "-n", "ghost"])

# --- clone
CLONE_OK = {"method": "POST", "path": "/helm/helm-1/clone", "status": 201,
            "body": dict(copy.deepcopy(HELM1), id="helm-9", name="my-helm-copy")}
case("clone", ["helm", "clone", "-n", "my-helm"], fixtures=[CLONE_OK])
case("clone target name", ["helm", "clone", "-n", "my-helm", "--target-helm-name", "copy"], fixtures=[CLONE_OK])
case("clone target project env", ["helm", "clone", "-n", "my-helm", "--target-project", "my-project",
                                  "--target-environment", "my-env"], fixtures=[CLONE_OK])
case("clone target env not found", ["helm", "clone", "-n", "my-helm", "--target-environment", "nope"])
case("clone target project not found", ["helm", "clone", "-n", "my-helm", "--target-project", "nope"])
case("clone not found", ["helm", "clone", "-n", "ghost"])
case("clone list error", ["helm", "clone", "-n", "my-helm"], fixtures=[err("GET", "/environment/env-1/helm")])
case("clone api error", ["helm", "clone", "-n", "my-helm"], fixtures=[err("POST", "/helm/helm-1/clone", ERR400)])
# successful responses the Go client cannot decode (generated client + HelmResponse.UnmarshalJSON)
for name, resp in [
    ("syntax error", {"body_raw": "{bad", "headers": {"Content-Type": "application/json"}}),
    ("not an object", {"body": "x"}),
    ("not json", {"body_raw": "hello"}),
    ("missing required", {"body": {"id": "helm-9", "name": "x"}}),
    ("empty", {"body_raw": "", "headers": {"Content-Type": "application/json"}}),
    ("null", {"body_raw": "null", "headers": {"Content-Type": "application/json"}}),
]:
    case("clone response " + name, ["helm", "clone", "-n", "my-helm"],
         fixtures=[dict({"method": "POST", "path": "/helm/helm-1/clone", "status": 201}, **resp)])
case("clone missing flag", ["helm", "clone"])

# --- update
HELM_PUT = lambda hid, h: {"method": "PUT", "path": "/helm/" + hid, "body": h}
case("update repository helm without values file", ["helm", "update", "-n", "my-helm"], fixtures=[HELM_PUT("helm-1", HELM1)])
case("update git helm", ["helm", "update", "-n", "git-helm"], fixtures=[HELM_PUT("helm-2", HELM2)])
case("update git helm branches", ["helm", "update", "-n", "git-helm", "--chart_git_commit_branch", "dev",
                                  "--values_override_git_commit_branch", "staging", "--chart_name", "ignored",
                                  "--chart_version", "9"], fixtures=[HELM_PUT("helm-2", HELM2)])
case("update raw helm", ["helm", "update", "-n", "raw-helm"], fixtures=[HELM_PUT("helm-3", HELM3)])
case("update raw helm chart", ["helm", "update", "-n", "raw-helm", "--chart_name", "nginx", "--chart_version", "1.2.3"],
     fixtures=[HELM_PUT("helm-3", HELM3)])
case("update api error", ["helm", "update", "-n", "git-helm"], fixtures=[err("PUT", "/helm/helm-2", ERR400)])
case("update api error 500", ["helm", "update", "-n", "raw-helm"], fixtures=[err("PUT", "/helm/helm-3")])
case("update not found", ["helm", "update", "-n", "ghost"])
case("update missing flag", ["helm", "update"])
NO_UPDATED_AT = copy.deepcopy(HELM3)
del NO_UPDATED_AT["updated_at"]
# Go panics (nil *time.Time); the goroutine trace cannot be reproduced
case("list missing updated_at panics", ["helm", "list"], ignore=["stderr"],
     fixtures=[{"method": "GET", "path": "/environment/env-1/helm", "body": {"results": [NO_UPDATED_AT]}}])
case("list json missing updated_at panics", ["helm", "list", "--json"], ignore=["stderr"],
     fixtures=[{"method": "GET", "path": "/environment/env-1/helm", "body": {"results": [NO_UPDATED_AT]}}])
NO_VALIDATION = copy.deepcopy(CUSTOM_DOMAINS)
del NO_VALIDATION["body"]["results"][1]["validation_domain"]
case("domain list missing validation domain panics", ["helm", "domain", "list", "-n", "my-helm"], ignore=["stderr"],
     fixtures=[NO_VALIDATION, LINKS])

# --- domain
case("domain list", ["helm", "domain", "list", "-n", "my-helm"], fixtures=[CUSTOM_DOMAINS, LINKS])
case("domain list json", ["helm", "domain", "list", "-n", "my-helm", "--json"], fixtures=[CUSTOM_DOMAINS, LINKS])
case("domain list empty json", ["helm", "domain", "list", "-n", "my-helm", "--json"], fixtures=[EMPTY_DOMAINS, EMPTY_LINKS])
case("domain list empty", ["helm", "domain", "list", "-n", "my-helm"], fixtures=[EMPTY_DOMAINS, EMPTY_LINKS])
case("domain list links error", ["helm", "domain", "list", "-n", "my-helm"], fixtures=[CUSTOM_DOMAINS, err("GET", "/helm/helm-1/link")])
case("domain list domains error", ["helm", "domain", "list", "-n", "my-helm"], fixtures=[err("GET", "/helm/helm-1/customDomain")])
case("domain list not found", ["helm", "domain", "list", "-n", "ghost"])
case("domain list missing flag", ["helm", "domain", "list"])
case("domain create", ["helm", "domain", "create", "-n", "my-helm", "--domain", "new.example.com",
                       "--do-not-generate-certificate"], fixtures=[CUSTOM_DOMAINS, CREATED_DOMAIN])
case("domain create cdn", ["helm", "domain", "create", "-n", "my-helm", "--domain", "new.example.com",
                           "--is-behind-a-cdn"], fixtures=[CUSTOM_DOMAINS, CREATED_DOMAIN])
case("domain create exists", ["helm", "domain", "create", "-n", "my-helm", "--domain", "app.example.com"],
     fixtures=[CUSTOM_DOMAINS])
case("domain create api error", ["helm", "domain", "create", "-n", "my-helm", "--domain", "x.example.com"],
     fixtures=[CUSTOM_DOMAINS, err("POST", "/helm/helm-1/customDomain", ERR400)])
case("domain create missing domain", ["helm", "domain", "create", "-n", "my-helm"])
case("domain create missing all", ["helm", "domain", "create"])
case("domain edit", ["helm", "domain", "edit", "-n", "my-helm", "--domain", "app.example.com", "--is-behind-a-cdn"],
     fixtures=[CUSTOM_DOMAINS, EDITED_DOMAIN])
case("domain edit missing", ["helm", "domain", "edit", "-n", "my-helm", "--domain", "nope.example.com"],
     fixtures=[CUSTOM_DOMAINS])
case("domain edit api error", ["helm", "domain", "edit", "-n", "my-helm", "--domain", "app.example.com"],
     fixtures=[CUSTOM_DOMAINS, err("PUT", "/helm/helm-1/customDomain/cd-1")])
case("domain edit helm not found", ["helm", "domain", "edit", "-n", "ghost", "--domain", "app.example.com"])
case("domain delete", ["helm", "domain", "delete", "-n", "my-helm", "--domain", "cdn.example.com"],
     fixtures=[CUSTOM_DOMAINS, DELETED_DOMAIN])
case("domain delete missing", ["helm", "domain", "delete", "-n", "my-helm", "--domain", "nope.example.com"],
     fixtures=[CUSTOM_DOMAINS])
case("domain delete api error", ["helm", "domain", "delete", "-n", "my-helm", "--domain", "cdn.example.com"],
     fixtures=[CUSTOM_DOMAINS, err("DELETE", "/helm/helm-1/customDomain/cd-2")])
case("domain delete missing flags", ["helm", "domain", "delete", "--domain", "x"])

# --- env
case("env list sorted", ["helm", "env", "list", "-n", "my-helm", "--sort"], fixtures=[VARS])
case("env list show values", ["helm", "env", "list", "-n", "my-helm", "--show-values", "--sort"], fixtures=[VARS])
case("env list pretty", ["helm", "env", "list", "-n", "my-helm", "--pretty-print", "--sort"], fixtures=[VARS])
# without --sort Go iterates over a map (random order): only one key here
ONE_VAR = copy.deepcopy(VARS)
ONE_VAR["body"]["results"] = [v for v in ONE_VAR["body"]["results"] if v["key"] in ("DB_HOST", "ALIAS_HOST")]
case("env list unsorted single key", ["helm", "env", "list", "-n", "my-helm", "--pretty-print"], fixtures=[ONE_VAR])
case("env list empty", ["helm", "env", "list", "-n", "my-helm"],
     fixtures=[{"method": "GET", "path": "/variable", "body": {"results": []}}])
case("env list empty json", ["helm", "env", "list", "-n", "my-helm", "--json"],
     fixtures=[{"method": "GET", "path": "/variable", "body": {"results": []}}])
case("env list show values pretty sort", ["helm", "env", "list", "-n", "my-helm", "--show-values", "--pretty-print",
                                          "--sort"], fixtures=[VARS])
case("env list json", ["helm", "env", "list", "-n", "my-helm", "--json"], fixtures=[VARS])
case("env list json sort", ["helm", "env", "list", "-n", "my-helm", "--json", "--sort"], fixtures=[VARS])
case("env list api error", ["helm", "env", "list", "-n", "my-helm"], fixtures=[err("GET", "/variable")])
case("env list not found", ["helm", "env", "list", "-n", "ghost"])
case("env list missing flag", ["helm", "env", "list"])
case("env create", ["helm", "env", "create", "-n", "my-helm", "-k", "NEW", "-v", "val"], fixtures=[VAR_POST])
case("env create secret env scope", ["helm", "env", "create", "-n", "my-helm", "-k", "NEW", "-v", "val", "--secret",
                                     "--scope", "ENVIRONMENT", "--override-if-exists"], fixtures=[VAR_POST])
case("env create project scope", ["helm", "env", "create", "-n", "my-helm", "-k", "NEW", "-v", "val",
                                  "--scope", "PROJECT"], fixtures=[VAR_POST])
case("env create bad scope", ["helm", "env", "create", "-n", "my-helm", "-k", "NEW", "-v", "val", "--scope", "helm"])
case("env create conflict", ["helm", "env", "create", "-n", "my-helm", "-k", "NEW", "-v", "val"],
     fixtures=[err("POST", "/variable", {"status": 409, "body": {"status": 409, "error": "Conflict",
                                                                 "detail": "key NEW already exists at ENVIRONMENT"}})])
case("env create missing value", ["helm", "env", "create", "-n", "my-helm", "-k", "NEW"])
case("env create helm not found", ["helm", "env", "create", "-n", "ghost", "-k", "NEW", "-v", "x"])
case("env update", ["helm", "env", "update", "-n", "my-helm", "-k", "FOO", "-v", "baz"], fixtures=[VARS, VAR_PUT1])
case("env update unknown key", ["helm", "env", "update", "-n", "my-helm", "-k", "NOPE", "-v", "baz"], fixtures=[VARS])
case("env update api error", ["helm", "env", "update", "-n", "my-helm", "-k", "FOO", "-v", "baz"],
     fixtures=[VARS, err("PUT", "/variable/var-1")])
case("env update missing", ["helm", "env", "update", "-n", "my-helm", "-k", "FOO"])
case("env delete", ["helm", "env", "delete", "-n", "my-helm", "-k", "FOO"], fixtures=[VARS, VAR_DEL1])
case("env delete unknown key", ["helm", "env", "delete", "-n", "my-helm", "-k", "NOPE"], fixtures=[VARS])
case("env delete api error", ["helm", "env", "delete", "-n", "my-helm", "-k", "FOO"],
     fixtures=[VARS, err("DELETE", "/variable/var-1")])
case("env delete missing key", ["helm", "env", "delete", "-n", "my-helm"])
case("env alias create", ["helm", "env", "alias", "create", "-n", "my-helm", "-k", "DB_HOST", "--alias", "MY_ALIAS"],
     fixtures=[VARS, VAR_ALIAS])
case("env alias create env scope", ["helm", "env", "alias", "create", "-n", "my-helm", "-k", "DB_HOST", "--alias",
                                    "MY_ALIAS", "--scope", "ENVIRONMENT"], fixtures=[VARS, VAR_ALIAS])
case("env alias create project scope", ["helm", "env", "alias", "create", "-n", "my-helm", "-k", "DB_HOST", "--alias",
                                        "MY_ALIAS", "--scope", "PROJECT"], fixtures=[VARS, VAR_ALIAS])
case("env alias create unknown key", ["helm", "env", "alias", "create", "-n", "my-helm", "-k", "NOPE", "--alias", "A"],
     fixtures=[VARS])
case("env alias create bad scope", ["helm", "env", "alias", "create", "-n", "my-helm", "-k", "DB_HOST", "--alias", "A",
                                    "--scope", "X"], fixtures=[VARS])
case("env alias create missing alias", ["helm", "env", "alias", "create", "-n", "my-helm", "-k", "DB_HOST"])
case("env override create", ["helm", "env", "override", "create", "-n", "my-helm", "-k", "DB_HOST", "--value", "other"],
     fixtures=[VARS, VAR_OVERRIDE])
case("env override create env scope", ["helm", "env", "override", "create", "-n", "my-helm", "-k", "DB_HOST",
                                       "--value", "other", "--scope", "ENVIRONMENT"], fixtures=[VARS, VAR_OVERRIDE])
case("env override create no value", ["helm", "env", "override", "create", "-n", "my-helm", "-k", "DB_HOST"],
     fixtures=[VARS, VAR_OVERRIDE])
case("env override create unknown key", ["helm", "env", "override", "create", "-n", "my-helm", "-k", "NOPE",
                                         "--value", "x"], fixtures=[VARS])
case("env override create api error", ["helm", "env", "override", "create", "-n", "my-helm", "-k", "DB_HOST",
                                       "--value", "x"], fixtures=[VARS, err("POST", "/variable/var-3/override")])
case("env override create missing key", ["helm", "env", "override", "create", "-n", "my-helm"])

# --- external-secret
case("external-secret create", ["helm", "external-secret", "create", "-n", "my-helm", "-k", "EXT2", "-r", "/sm/ref",
                                "--secret-manager-access-name", "my-sm"], fixtures=[CLUSTERS, VAR_POST])
case("external-secret create mount path env scope", ["helm", "external-secret", "create", "-n", "my-helm", "-k", "EXT2",
                                                     "-r", "/sm/ref", "--secret-manager-access-name", "my-sm",
                                                     "--mount-path", "/etc/secret", "--scope", "ENVIRONMENT"],
     fixtures=[CLUSTERS, VAR_POST])
case("external-secret create unknown sma", ["helm", "external-secret", "create", "-n", "my-helm", "-k", "EXT2", "-r",
                                            "/sm/ref", "--secret-manager-access-name", "nope"], fixtures=[CLUSTERS])
case("external-secret create clusters error", ["helm", "external-secret", "create", "-n", "my-helm", "-k", "EXT2", "-r",
                                               "/sm/ref", "--secret-manager-access-name", "my-sm"],
     fixtures=[err("GET", "/organization/org-1/cluster")])
case("external-secret create api error", ["helm", "external-secret", "create", "-n", "my-helm", "-k", "EXT2", "-r",
                                          "/sm/ref", "--secret-manager-access-name", "my-sm"],
     fixtures=[CLUSTERS, err("POST", "/variable", ERR400)])
case("external-secret create missing flags", ["helm", "external-secret", "create", "-n", "my-helm", "-k", "EXT2"])
case("external-secret create helm not found", ["helm", "external-secret", "create", "-n", "ghost", "-k", "EXT2", "-r",
                                               "/sm/ref", "--secret-manager-access-name", "my-sm"], fixtures=[CLUSTERS])
case("external-secret update", ["helm", "external-secret", "update", "-n", "my-helm", "-k", "EXT", "-r", "/sm/ref",
                                "--secret-manager-access-name", "my-sm"], fixtures=[CLUSTERS, VARS, VAR_PUT6])
case("external-secret update no sma", ["helm", "external-secret", "update", "-n", "my-helm", "-k", "EXT", "-r",
                                       "/sm/ref"], fixtures=[CLUSTERS, VARS, VAR_PUT6])
case("external-secret update unknown key", ["helm", "external-secret", "update", "-n", "my-helm", "-k", "NOPE",
                                            "--secret-manager-access-name", "my-sm"], fixtures=[CLUSTERS, VARS])
case("external-secret update missing key", ["helm", "external-secret", "update", "-n", "my-helm"])
case("external-secret delete", ["helm", "external-secret", "delete", "-n", "my-helm", "-k", "EXT"],
     fixtures=[VARS, VAR_DEL6])
case("external-secret delete unknown", ["helm", "external-secret", "delete", "-n", "my-helm", "-k", "NOPE"],
     fixtures=[VARS])
case("external-secret delete missing", ["helm", "external-secret", "delete", "-k", "EXT"])

scenario = {"fixtures": [HELM_LIST], "cases": cases}
# the "default fixture" list case runs against the default helm list only
for c in cases:
    if c["name"] == "list default fixture":
        c["fixtures"] = [{"method": "GET", "path": "/environment/env-1/helm",
                          "body": {"results": [HELM1]}}]
        c.pop("ignore")
json.dump(scenario, open(OUT, "w"), indent=1)
print(len(cases), "cases")
