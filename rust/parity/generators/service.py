#!/usr/bin/env python3
"""Generates parity/scenarios/service.json.

Usage: python3 parity/generators/service.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "service.json")
ENV_STATUS = {"id": "env-1", "state": "DEPLOYING", "last_deployment_state": "DEPLOYED"}
DEPLOY_OK = {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202, "body": ENV_STATUS}
DEPLOY_500 = {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 500, "body": {"message": "boom"}}
def err(method, path, status=500):
    return {"method": method, "path": path, "status": status, "body": {"status": status, "error": "boom"}}
def empty(path):
    return {"method": "GET", "path": path, "status": 200, "body": {"results": []}}

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
def link(kind, id, url):
    return {"method": "GET", "path": "/%s/%s/link" % (kind, id), "status": 200, "body": {"results": [
        {"service_id": id, "service_type": kind.upper(), "url": url, "internal_port": 80, "external_port": 443, "is_qovery_domain": True, "is_default": True},
        {"service_id": id, "service_type": kind.upper(), "url": url + "/second", "internal_port": 80, "external_port": 443, "is_qovery_domain": True, "is_default": False}]}}
LINKS = [link("application", "app-1", "https://app.example.com"), empty("/container/cont-1/link")]

def st(state, exe):
    return {"id": "app-1", "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": exe}
def sws(state, exe):
    return {"body": {"environment": {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED",
            "last_deployment_date": "2024-06-02T11:30:00Z", "last_deployment_id": "dep-1", "total_deployment_duration_in_seconds": 42},
            "stages": [{"applications": [st(state, exe)]}]}}
WATCH = {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [sws("DEPLOYED", "exec-old"), sws("DEPLOYING", "exec-new"), sws("DEPLOYED", "exec-new")]}
WATCH_FAIL = {"method": "GET", "path": "/environment/env-1/statusesWithStages", "responses": [sws("DEPLOYED", "exec-old"), sws("DEPLOYMENT_ERROR", "exec-new")]}

ALL_EMPTY = [empty("/environment/env-1/" + k) for k in ("application", "container", "database", "job", "helm")]

DEF = json.load(open(os.path.join(PARITY, "fixtures", "default.json")))
APP = [r for r in DEF if r["path"] == "/environment/env-1/application"][0]["body"]["results"][0]
cases = []
def c(name, args, **kw):
    d = {"name": name, "args": args}
    d.update(kw)
    cases.append(d)

c("service group", ["service"])
c("service list", ["service", "list"])
c("service list json", ["service", "list", "--json"])
c("service list markdown", ["service", "list", "--markdown"], fixtures=LINKS)
c("service list jira", ["service", "list", "--jira"], fixtures=LINKS)
c("service list markdown no links", ["service", "list", "--markdown"], fixtures=[empty("/application/app-1/link"), empty("/container/cont-1/link")])
c("service list markdown links error", ["service", "list", "--markdown"], fixtures=[err("GET", "/application/app-1/link")])
c("service list jira container links error", ["service", "list", "--jira"], fixtures=[empty("/application/app-1/link"), err("GET", "/container/cont-1/link", 403)])
c("service list markdown env error", ["service", "list", "--markdown"], fixtures=[err("GET", "/environment/env-1", 404)])
c("service list jira env error", ["service", "list", "--jira"], fixtures=[err("GET", "/environment/env-1")])
c("service list markdown and json", ["service", "list", "--markdown", "--json"], fixtures=LINKS)
c("service list by names", ["service", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
c("service list by names json", ["service", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env", "--json"])
c("service list unknown org", ["service", "list", "--organization", "nope", "--project", "my-project", "--environment", "my-env"])
c("service list unknown project", ["service", "list", "--organization", "my-org", "--project", "nope", "--environment", "my-env"])
c("service list unknown env", ["service", "list", "--environment", "nope"])
c("service list empty json", ["service", "list", "--json"], fixtures=ALL_EMPTY)
c("service list empty table", ["service", "list"], fixtures=ALL_EMPTY)
c("service list empty markdown", ["service", "list", "--markdown"], fixtures=ALL_EMPTY)
for k in ("application", "database", "container", "job", "helm"):
    c("service list %s error" % k, ["service", "list"], fixtures=[err("GET", "/environment/env-1/" + k)])
c("service list statuses error", ["service", "list", "--json"], fixtures=[err("GET", "/environment/env-1/statuses", 401)])
c("service list no token", ["service", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)
c("service list bad arg flag", ["service", "list", "--nope"])

c("deploy no flag", ["service", "deploy"])
c("deploy both flags", ["service", "deploy", "-n", "my-app", "--services", "my-db"])
c("deploy app", ["service", "deploy", "-n", "my-app"], fixtures=[DEPLOY_OK])
c("deploy app version", ["service", "deploy", "-n", "my-app", "--version", "c0ffee"], fixtures=[DEPLOY_OK])
c("deploy container", ["service", "deploy", "-n", "my-container"], fixtures=[DEPLOY_OK])
c("deploy container version", ["service", "deploy", "--service", "my-container", "-v", "2.0"], fixtures=[DEPLOY_OK])
c("deploy git container", ["service", "deploy", "-n", "git-container"], fixtures=[GIT_CONTAINERS, DEPLOY_OK])
c("deploy git container version", ["service", "deploy", "--services", "my-app,git-container", "-v", "v2"],
  fixtures=[GIT_CONTAINERS, DEPLOY_OK])
c("deploy git container api error", ["service", "deploy", "-n", "git-container"], fixtures=[GIT_CONTAINERS, DEPLOY_500])
IMAGE_APP = {k: v for k, v in APP.items() if k not in ("git_repository", "build_mode", "dockerfile_path")}
IMAGE_APPS = {"method": "GET", "path": "/environment/env-1/application", "status": 200, "body": {"results": [IMAGE_APP]}}
c("deploy app without git repository", ["service", "deploy", "-n", "my-app"], fixtures=[IMAGE_APPS, DEPLOY_OK])
c("deploy app without git repository version", ["service", "deploy", "-n", "my-app", "-v", "c0ffee"], fixtures=[IMAGE_APPS, DEPLOY_OK])
c("deploy services app without git repository version", ["service", "deploy", "--services", "my-container,my-app,my-db", "-v", "v2"],
  fixtures=[IMAGE_APPS, DEPLOY_OK])
c("deploy services app without git repository version watch", ["service", "deploy", "--services", "my-container,my-app", "-v", "v2", "-w"],
  fixtures=[IMAGE_APPS, DEPLOY_OK, WATCH], timeout=60)
c("deploy app without git repository latest", ["service", "deploy", "-n", "my-app", "-v", "latest"], fixtures=[IMAGE_APPS, DEPLOY_OK])
c("deploy database", ["service", "deploy", "-n", "my-db", "-v", "ignored"], fixtures=[DEPLOY_OK])
c("deploy cron", ["service", "deploy", "-n", "my-cron"], fixtures=[DEPLOY_OK])
c("deploy cron version", ["service", "deploy", "-n", "my-cron", "-v", "abcdef"], fixtures=[DEPLOY_OK])
c("deploy lifecycle version", ["service", "deploy", "-n", "my-lifecycle", "-v", "1.2"], fixtures=[DEPLOY_OK])
c("deploy helm", ["service", "deploy", "-n", "my-helm"], fixtures=[DEPLOY_OK])
c("deploy helm versions", ["service", "deploy", "-n", "my-helm", "-v", "19.0.0", "--values-override-version", "abc"], fixtures=[DEPLOY_OK])
c("deploy helm values override latest", ["service", "deploy", "-n", "my-helm", "--values-override-version", "latest"], fixtures=[DEPLOY_OK])
c("deploy app latest no commits", ["service", "deploy", "-n", "my-app", "-v", "latest"], fixtures=[DEPLOY_OK])
c("deploy services", ["service", "deploy", "--services", "my-helm, my-app,my-container ,my-db,my-cron,my-lifecycle", "-v", "v9"], fixtures=[DEPLOY_OK])
c("deploy services by names", ["service", "deploy", "--organization", "my-org", "--project", "my-project", "--environment", "my-env", "--services", "my-app,my-db"], fixtures=[DEPLOY_OK])
c("deploy trimmed name", ["service", "deploy", "-n", "  my-app "], fixtures=[DEPLOY_OK])
c("deploy unknown", ["service", "deploy", "-n", "nope"], fixtures=[DEPLOY_OK])
c("deploy services one unknown", ["service", "deploy", "--services", "my-app,nope"], fixtures=[DEPLOY_OK])
c("deploy services comma", ["service", "deploy", "--services", ","], fixtures=[DEPLOY_OK])
c("deploy terraform not supported", ["service", "deploy", "-n", "my-terraform"], fixtures=[DEPLOY_OK])
c("deploy unknown env", ["service", "deploy", "-n", "my-app", "--environment", "nope"], fixtures=[DEPLOY_OK])
c("deploy unknown org", ["service", "deploy", "-n", "my-app", "--organization", "nope", "--project", "x", "--environment", "y"], fixtures=[DEPLOY_OK])
c("deploy error", ["service", "deploy", "-n", "my-app"], fixtures=[DEPLOY_500])
c("deploy services error", ["service", "deploy", "--services", "my-app,my-db"], fixtures=[DEPLOY_500])
for k in ("application", "container", "database", "job", "helm"):
    c("deploy %s list error" % k, ["service", "deploy", "-n", "my-app"], fixtures=[err("GET", "/environment/env-1/" + k), DEPLOY_OK])
c("deploy same name app and db", ["service", "deploy", "-n", "my-db"], fixtures=[DEPLOY_OK, {"method": "GET", "path": "/environment/env-1/application", "status": 200, "body": {"results": [dict(APP, id="app-x", name="my-db")]}}])
c("deploy watch", ["service", "deploy", "-n", "my-app", "-w"], fixtures=[DEPLOY_OK, WATCH], timeout=60)
c("deploy watch failure", ["service", "deploy", "-n", "my-app", "--watch"], fixtures=[DEPLOY_OK, WATCH_FAIL], timeout=60)
c("deploy no token", ["service", "deploy", "-n", "my-app"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)
c("deploy extra arg", ["service", "deploy", "extra", "-n", "my-app"], fixtures=[DEPLOY_OK])

json.dump({"cases": cases}, open(out, "w"), indent=1)
print(len(cases))
