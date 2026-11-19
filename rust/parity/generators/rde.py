#!/usr/bin/env python3
"""Generates parity/scenarios/rde.json.

Usage: python3 parity/generators/rde.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import copy
import json

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "rde.json")
T = "2024-05-01T10:00:00Z"


def project(pid, name):
    return {"id": pid, "created_at": T, "name": name, "associated_environments_count": 1, "organization": {"id": "org-1"}}


def env(eid, name, pid, mode="DEVELOPMENT"):
    return {"id": eid, "created_at": T, "name": name, "organization": {"id": "org-1"}, "project": {"id": pid},
            "cloud_provider": {"provider": "AWS", "cluster": "us-east-2"}, "mode": mode, "cluster_id": "cluster-1",
            "cluster_name": "my-cluster"}


def var(vid, key, value, scope):
    return {"id": vid, "created_at": T, "key": key, "value": value, "scope": scope, "variable_type": "VALUE",
            "is_secret": False}


def vars_fx(parent, scope, items, status=200):
    r = {"method": "GET", "path": "/variable", "query": {"parent_id": parent, "scope": scope}, "status": status}
    r["body"] = {"results": items} if status == 200 else {"message": "boom"}
    return r


def get(path, body, status=200):
    return {"method": "GET", "path": path, "status": status, "body": body}


def results(items):
    return {"results": items}


def st(sid, state):
    return {"id": sid, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0}


def statuses(eid, state, apps=(), containers=(), jobs=(), dbs=(), helms=(), last=None):
    return {"environment": {"id": eid, "state": state, "last_deployment_state": last or state},
            "applications": list(apps), "containers": list(containers), "jobs": list(jobs), "databases": list(dbs),
            "helms": list(helms), "terraforms": []}


ENV_ACTION = {"id": "x", "state": "DEPLOYING", "last_deployment_state": "DEPLOYING",
              "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
              "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
              "deployment_request_id": None, "deployment_requests_count": 0}

HC = {"readiness_probe": {"type": {"http": {"path": "/", "port": 8080, "scheme": "HTTP"}}, "initial_delay_seconds": 30,
                          "period_seconds": 10, "timeout_seconds": 5, "success_threshold": 1, "failure_threshold": 3},
      "liveness_probe": None}
HC2 = {"readiness_probe": None, "liveness_probe": {"type": {"tcp": {"port": 3000}}, "initial_delay_seconds": 5}}


def app(aid, eid, name, branch="main", cpu=500, hc=HC, ports=None, storage=None):
    return {"id": aid, "created_at": T, "environment": {"id": eid}, "name": name, "healthchecks": hc,
            "icon_uri": "app://qovery-console/application", "service_type": "APPLICATION",
            "git_repository": {"provider": "GITHUB", "owner": "me", "url": "https://github.com/me/ws.git",
                               "name": "me/ws", "branch": branch, "root_path": "/", "has_access": True,
                               "git_token_id": None},
            "build_mode": "DOCKER", "dockerfile_path": "Dockerfile", "cpu": cpu, "memory": 512,
            "min_running_instances": 1, "max_running_instances": 1, "auto_preview": False, "auto_deploy": True,
            "ports": ports if ports is not None else [
                {"id": "p-" + aid, "name": "http", "internal_port": 8080, "external_port": 443,
                 "publicly_accessible": True, "is_default": True, "protocol": "HTTP", "public_path": None}],
            "arguments": [], "storage": storage or []}


def container(cid, eid, name, tag="1.0", cpu=250, storage=None, ports=None, hc=HC, autoscaling=None):
    c = {"id": cid, "created_at": T, "image_name": "code-server", "tag": tag,
         "registry": {"id": "reg-" + eid, "name": "Docker Hub", "url": "https://docker.io", "kind": "DOCKER_HUB"},
         "environment": {"id": eid}, "maximum_cpu": 4000, "maximum_memory": 8192, "maximum_gpu": 0, "name": name,
         "cpu": cpu, "memory": 512, "gpu": 0, "min_running_instances": 1, "max_running_instances": 1,
         "healthchecks": hc, "auto_preview": False, "icon_uri": "app://qovery-console/container",
         "service_type": "CONTAINER", "auto_deploy": None, "entrypoint": "/bin/sh", "arguments": ["-c", "run"],
         "description": "ide container",
         "ports": ports if ports is not None else [
             {"id": "cp-" + cid, "name": "ide", "internal_port": 8443, "external_port": 443,
              "publicly_accessible": True, "is_default": True, "protocol": "HTTP"}],
         "storage": storage if storage is not None else [
             {"id": "s-" + cid, "type": "FAST_SSD", "size": 10, "mount_point": "/home"}]}
    if autoscaling:
        c["autoscaling"] = autoscaling
    return c


def cron(jid, eid, name, cpu=250, source=None, hc=None):
    return {"id": jid, "created_at": T, "environment": {"id": eid}, "maximum_cpu": 4000, "maximum_memory": 8192,
            "maximum_gpu": 0, "name": name, "cpu": cpu, "memory": 256, "gpu": 0, "auto_preview": False,
            "source": source or {"image": {"image_name": "curl", "tag": "8", "registry": {
                "id": "reg-" + eid, "name": "Docker Hub", "url": "https://docker.io", "kind": "DOCKER_HUB"}}},
            "healthchecks": hc or {}, "icon_uri": "app://qovery-console/cron-job", "service_type": "JOB",
            "job_type": "CRON",
            "schedule": {"cronjob": {"scheduled_at": "0 * * * *", "arguments": [], "timezone": "Etc/UTC"}},
            "auto_deploy": True, "max_nb_restart": 0, "max_duration_seconds": 300, "port": None}


def lifecycle(jid, eid, name, cpu=250, source=None):
    return {"id": jid, "created_at": T, "environment": {"id": eid}, "maximum_cpu": 4000, "maximum_memory": 8192,
            "maximum_gpu": 0, "name": name, "cpu": cpu, "memory": 256, "gpu": 0, "auto_preview": True,
            "source": source or {"docker": {"git_repository": {
                "provider": "GITHUB", "owner": "me", "url": "https://github.com/me/seed.git", "name": "me/seed",
                "branch": "main", "root_path": "/"}, "dockerfile_path": "Dockerfile", "dockerfile_raw": None}},
            "healthchecks": {}, "icon_uri": "app://qovery-console/lifecycle-job", "service_type": "JOB",
            "job_type": "LIFECYCLE",
            "schedule": {"on_start": {"arguments": ["seed"], "entrypoint": "/seed.sh"}, "on_stop": None},
            "auto_deploy": None, "description": "seed db"}


def helm(hid, eid, name, source=None, args=None):
    return {"id": hid, "created_at": T, "environment": {"id": eid}, "name": name, "auto_preview": False,
            "auto_deploy": False, "timeout_sec": 600,
            "source": source or {"repository": {"repository": {"id": "hr-" + eid, "name": "bitnami",
                                                                "url": "https://charts.bitnami.com/bitnami"},
                                                 "chart_name": "redis", "chart_version": "18.0.0"}},
            "arguments": args if args is not None else ["--wait"], "allow_cluster_wide_resources": False,
            "values_override": {"set": [["a", "b"]], "set_string": [], "file": None},
            "icon_uri": "app://qovery-console/helm", "service_type": "HELM", "ports": []}


PROJECTS = [project("proj-1", "my-project"), project("bp-1", "my-blueprint"), project("bp-2", "empty-blueprint"),
            project("rde-1", "rde-alice"), project("rde-2", "rde-bob")]

BP_APP = app("bapp-1", "bpenv-1", "workspace-app", branch="develop", cpu=1000, hc=HC2,
             ports=[{"id": "bp-port", "name": "web", "internal_port": 3000, "publicly_accessible": False,
                     "protocol": "HTTP", "is_default": None}],
             storage=[{"id": "bs-1", "type": "FAST_SSD", "size": 20, "mount_point": "/data"}])
BP_CONT = container("bcont-1", "bpenv-1", "ide", tag="2.0", cpu=1000, hc=HC2,
                    storage=[{"id": "bst-1", "type": "FAST_SSD", "size": 50, "mount_point": "/work"}],
                    ports=[{"id": "bport", "internal_port": 9000, "publicly_accessible": False, "protocol": "TCP"}],
                    autoscaling={"id": "as-1", "created_at": T, "service_id": "bcont-1", "mode": "KEDA",
                                 "polling_interval_seconds": 30, "cooldown_period_seconds": 300,
                                 "scalers": [{"id": "sc-1", "created_at": T, "scaler_type": "cpu", "enabled": True,
                                              "role": "PRIMARY", "config_json": {"value": "80"}}]})
BP_CRON = cron("bcron-1", "bpenv-1", "ttl-auto-shutdown", cpu=500, hc=HC2,
               source={"image": {"image_name": "curl", "tag": "9", "registry": {
                   "id": "reg-bp", "name": "Docker Hub", "url": "https://docker.io", "kind": "DOCKER_HUB"}}})
BP_LIFE = lifecycle("blife-1", "bpenv-1", "seed", cpu=1000,
                    source={"docker": {"git_repository": {
                        "provider": "GITHUB", "owner": "me", "url": "https://github.com/me/seed.git",
                        "name": "me/seed", "branch": "v2", "root_path": "/db"}, "dockerfile_path": "Dockerfile.seed",
                        "docker_target_build_stage": None}})
BP_HELM = helm("bhelm-1", "bpenv-1", "redis",
               source={"repository": {"repository": {"id": "hr-bp", "name": "bitnami",
                                                     "url": "https://charts.bitnami.com/bitnami"},
                                      "chart_name": "redis", "chart_version": "19.0.0"}})
BP_HELM_GIT = helm("bhelm-2", "bpenv-1", "charts",
                   source={"git": {"git_repository": {"provider": "GITHUB", "owner": "me",
                                                      "url": "https://github.com/me/charts.git", "name": "me/charts",
                                                      "branch": "main", "root_path": "/charts"}}})

R_APP = app("rapp-1", "rdeenv-1", "workspace-app")
R_CONT = container("rcont-1", "rdeenv-1", "ide")
R_CONT_UNMATCHED = container("rcont-2", "rdeenv-1", "sidecar")
R_CRON = cron("rcron-1", "rdeenv-1", "ttl-auto-shutdown")
R_LIFE = lifecycle("rlife-1", "rdeenv-1", "seed")
R_HELM = helm("rhelm-1", "rdeenv-1", "redis")
R_HELM_GIT = helm("rhelm-2", "rdeenv-1", "charts", args=None)
R_HELM_GIT["arguments"] = None

FIXTURES = [
    get("/organization/org-1/project", results(PROJECTS)),
    vars_fx("proj-1", "PROJECT", []),
    vars_fx("bp-1", "PROJECT", [var("v-bp1", "BLUEPRINT_PROJECT_ID", "bp-1", "PROJECT")]),
    vars_fx("bp-2", "PROJECT", [var("v-bp2", "BLUEPRINT_PROJECT_ID", "bp-2", "PROJECT")]),
    vars_fx("rde-1", "PROJECT", []),
    vars_fx("rde-2", "PROJECT", []),
    get("/project/bp-1/environment", results([env("bpenv-1", "blueprint-env", "bp-1")])),
    get("/project/bp-2/environment", results([])),
    get("/project/rde-1/environment", results([env("rdeenv-1", "workspace", "rde-1")])),
    get("/project/rde-2/environment", results([env("rdeenv-2", "workspace", "rde-2")])),
    vars_fx("env-1", "ENVIRONMENT", []),
    vars_fx("bpenv-1", "ENVIRONMENT", [var("v-bk", "BLUEPRINT_KEY", "bp-1", "ENVIRONMENT")]),
    vars_fx("rdeenv-1", "ENVIRONMENT", [var("v-bk1", "BLUEPRINT_KEY", "bp-1", "ENVIRONMENT"),
                                       var("v-own1", "RDE_OWNER_EMAIL", "alice@example.com", "ENVIRONMENT")]),
    vars_fx("rdeenv-2", "ENVIRONMENT", [var("v-bk2", "BLUEPRINT_KEY", "bp-1", "ENVIRONMENT")]),
    get("/environment/bpenv-1/statuses",
        statuses("bpenv-1", "DEPLOYED", apps=[st("bapp-1", "DEPLOYED")], containers=[st("bcont-1", "STOPPED")],
                 jobs=[st("bcron-1", "DEPLOYED"), st("unknown-job", "DEPLOYMENT_ERROR")],
                 helms=[st("bhelm-1", "DEPLOYING")])),
    get("/environment/rdeenv-1/statuses",
        statuses("rdeenv-1", "DEPLOYED", apps=[st("rapp-1", "DEPLOYED")], containers=[st("rcont-1", "DEPLOYED")])),
    get("/environment/rdeenv-2/statuses", statuses("rdeenv-2", "STOPPED")),
    get("/project/bp-1", project("bp-1", "my-blueprint")),
    get("/project/bp-2", project("bp-2", "empty-blueprint")),
    get("/environment/bpenv-1", env("bpenv-1", "blueprint-env", "bp-1")),
    get("/environment/bpenv-1/application", results([BP_APP])),
    get("/environment/bpenv-1/container", results([BP_CONT])),
    get("/environment/bpenv-1/job", results([BP_CRON, BP_LIFE])),
    get("/environment/bpenv-1/database", results([])),
    get("/environment/bpenv-1/helm", results([BP_HELM, BP_HELM_GIT])),
    get("/environment/rdeenv-1/application", results([R_APP])),
    get("/environment/rdeenv-1/container", results([R_CONT, R_CONT_UNMATCHED])),
    get("/environment/rdeenv-1/job", results([R_CRON, R_LIFE])),
    get("/environment/rdeenv-1/database", results([])),
    get("/environment/rdeenv-1/helm", results([R_HELM, R_HELM_GIT])),
    get("/environment/rdeenv-2/application", results([])),
    get("/environment/rdeenv-2/container", results([])),
    get("/environment/rdeenv-2/job", results([])),
    get("/environment/rdeenv-2/helm", results([])),
    get("/application/rapp-1/link", results([{"service_id": "rapp-1", "service_type": "APPLICATION",
                                              "url": "https://alice.rde.example.com", "internal_port": 8080,
                                              "external_port": 443, "is_qovery_domain": True, "is_default": True}])),
    get("/environment/rdeenv-1/deploymentHistory",
        {"page": 1, "page_size": 10, "results": [{"id": "dep-1", "created_at": "2024-01-01T00:00:00Z"}]}),
    get("/organization/org-1/customRole", results([{"id": "role-a", "name": "RDE-alice"}])),
    get("/organization/org-1/apiToken", results([{"id": "tok-1", "created_at": T, "name": "ttl-alice"},
                                                 {"id": "tok-2", "created_at": T, "name": "other"}])),
    {"method": "POST", "path_regex": "^/environment/[^/]+/(deploy|stop)$", "status": 202, "body": ENV_ACTION},
    {"method": "DELETE", "path_regex": "^/(environment|project|variable)/[^/]+$", "status": 204},
    {"method": "DELETE", "path_regex": "^/organization/org-1/(customRole|apiToken)/[^/]+$", "status": 204},
    {"method": "PUT", "path": "/container/rcont-1", "body": R_CONT},
    {"method": "PUT", "path": "/application/rapp-1", "body": R_APP},
    {"method": "PUT", "path": "/job/rcron-1", "body": R_CRON},
    {"method": "PUT", "path": "/job/rlife-1", "body": R_LIFE},
    {"method": "PUT", "path": "/helm/rhelm-1", "body": R_HELM},
    {"method": "PUT", "path": "/helm/rhelm-2", "body": R_HELM_GIT},
    {"method": "POST", "path": "/variable", "status": 201, "body": var("v-new", "K", "V", "ENVIRONMENT")},
]

NEW_PROJECT = project("rde-new", "rde-carol")
NEW_ENV = env("newenv-1", "workspace", "rde-new")
CREATE_FX = [
    {"method": "POST", "path": "/organization/org-1/project", "status": 201, "body": NEW_PROJECT},
    {"method": "POST", "path": "/organization/org-1/customRole", "status": 201, "body": {"id": "role-1", "name": "RDE-carol"}},
    {"method": "PUT", "path": "/organization/org-1/customRole/role-1", "body": {"id": "role-1", "name": "RDE-carol"}},
    {"method": "POST", "path": "/environment/bpenv-1/clone", "status": 201, "body": NEW_ENV},
    get("/environment/newenv-1/job", results([cron("ncron-1", "newenv-1", "ttl-auto-shutdown")])),
    {"method": "POST", "path": "/organization/org-1/inviteMember", "status": 201,
     "body": {"id": "inv-1", "created_at": T, "email": "carol@example.com", "role": "DEVELOPER",
              "invitation_link": "https://x", "invitation_status": "PENDING", "inviter": "me"}},
]

# Reclone: the child env is deleted (statuses 404), then cloned
RECLONE_FX = [
    get("/environment/rdeenv-1/statuses", {"message": "gone"}, status=404),
    get("/environment/rdeenv-2/statuses", {"message": "gone"}, status=404),
    {"method": "POST", "path": "/environment/bpenv-1/clone", "status": 201, "body": NEW_ENV},
    get("/environment/newenv-1/job", results([])),
]

ERR500 = {"message": "boom"}


def case(name, args, fixtures=None, **kw):
    c = {"name": name, "args": args}
    if fixtures:
        c["fixtures"] = fixtures
    c.update(kw)
    return c


def watch_statuses(env_id, seq):
    return {"method": "GET", "path": "/environment/%s/statuses" % env_id,
            "responses": [{"status": 200, "body": statuses(env_id, s, apps=[st("rapp-1", s)], last=s)} for s in seq]}


CASES = [
    # groups / args
    case("rde (group)", ["rde"]),
    case("rde blueprint (group)", ["rde", "blueprint"]),
    case("rde extra arg", ["rde", "foo"]),
    case("rde unknown flag", ["rde", "list", "--nope"]),

    # blueprint list
    case("blueprint list", ["rde", "blueprint", "list"]),
    case("blueprint list --json", ["rde", "blueprint", "list", "--json"]),
    case("blueprint list -o my-org", ["rde", "blueprint", "list", "-o", "my-org"]),
    case("blueprint list org not found", ["rde", "blueprint", "list", "-o", "nope"]),
    case("blueprint list empty", ["rde", "blueprint", "list"],
         [get("/organization/org-1/project", results([project("proj-1", "my-project")]))]),
    case("blueprint list projects api error", ["rde", "blueprint", "list"],
         [get("/organization/org-1/project", ERR500, 500)]),
    case("blueprint list status error", ["rde", "blueprint", "list"],
         [get("/environment/bpenv-1/statuses", ERR500, 500)]),
    case("blueprint list json status error", ["rde", "blueprint", "list", "--json"],
         [get("/environment/bpenv-1/statuses", ERR500, 500)]),
    case("blueprint list vars error skipped", ["rde", "blueprint", "list"],
         [vars_fx("bp-1", "PROJECT", [], status=500)]),
    case("blueprint list wrong var value", ["rde", "blueprint", "list"],
         [vars_fx("bp-1", "PROJECT", [var("v-bp1", "BLUEPRINT_PROJECT_ID", "other", "PROJECT")])]),
    case("blueprint list no token", ["rde", "blueprint", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""},
         context={"access_token": ""}),

    # blueprint status
    case("blueprint status", ["rde", "blueprint", "status", "-p", "my-blueprint"]),
    case("blueprint status case-insensitive", ["rde", "blueprint", "status", "--project", "MY-BLUEPRINT"]),
    case("blueprint status no env", ["rde", "blueprint", "status", "-p", "empty-blueprint"]),
    case("blueprint status not found", ["rde", "blueprint", "status", "-p", "nope"]),
    case("blueprint status not a blueprint", ["rde", "blueprint", "status", "-p", "my-project"]),
    case("blueprint status invalid var", ["rde", "blueprint", "status", "-p", "my-blueprint"],
         [vars_fx("bp-1", "PROJECT", [var("v-bp1", "BLUEPRINT_PROJECT_ID", "zzz", "PROJECT")])]),
    case("blueprint status vars error", ["rde", "blueprint", "status", "-p", "my-blueprint"],
         [vars_fx("bp-1", "PROJECT", [], status=500)]),
    case("blueprint status env list error", ["rde", "blueprint", "status", "-p", "my-blueprint"],
         [get("/project/bp-1/environment", ERR500, 500)]),
    case("blueprint status statuses error", ["rde", "blueprint", "status", "-p", "my-blueprint"],
         [get("/environment/bpenv-1/statuses", ERR500, 500)]),
    case("blueprint status no services", ["rde", "blueprint", "status", "-p", "my-blueprint"],
         [get("/environment/bpenv-1/statuses", statuses("bpenv-1", "STOPPED"))]),
    case("blueprint status missing project", ["rde", "blueprint", "status"]),

    # blueprint deploy / stop
    case("blueprint deploy", ["rde", "blueprint", "deploy", "-p", "my-blueprint"]),
    case("blueprint deploy no env", ["rde", "blueprint", "deploy", "-p", "empty-blueprint"]),
    case("blueprint deploy not found", ["rde", "blueprint", "deploy", "-p", "nope"]),
    case("blueprint deploy api error", ["rde", "blueprint", "deploy", "-p", "my-blueprint"],
         [{"method": "POST", "path": "/environment/bpenv-1/deploy", "status": 400, "body": ERR500}]),
    case("blueprint deploy --watch", ["rde", "blueprint", "deploy", "-p", "my-blueprint", "--watch"],
         [watch_statuses("bpenv-1", ["DEPLOYING", "DEPLOYED"])], timeout=40),
    case("blueprint deploy --watch error", ["rde", "blueprint", "deploy", "-p", "my-blueprint", "-w"],
         [watch_statuses("bpenv-1", ["DEPLOYMENT_ERROR"])], timeout=40),
    case("blueprint deploy missing project", ["rde", "blueprint", "deploy"]),
    case("blueprint stop", ["rde", "blueprint", "stop", "-p", "my-blueprint"]),
    case("blueprint stop no env", ["rde", "blueprint", "stop", "-p", "empty-blueprint"]),
    case("blueprint stop api error", ["rde", "blueprint", "stop", "-p", "my-blueprint"],
         [{"method": "POST", "path": "/environment/bpenv-1/stop", "status": 500, "body": ERR500}]),
    case("blueprint stop --watch", ["rde", "blueprint", "stop", "-p", "my-blueprint", "--watch"],
         [watch_statuses("bpenv-1", ["STOPPING", "STOPPED"])], timeout=40),
    case("blueprint stop missing project", ["rde", "blueprint", "stop"]),

    # blueprint register / unregister
    case("blueprint register", ["rde", "blueprint", "register", "-p", "my-project"]),
    case("blueprint register prefers development env", ["rde", "blueprint", "register", "-p", "my-project"],
         [get("/project/proj-1/environment", results([env("e-prod", "prod", "proj-1", mode="PRODUCTION"),
                                                      env("e-dev", "dev", "proj-1")]))]),
    case("blueprint register fallback first env", ["rde", "blueprint", "register", "-p", "my-project"],
         [get("/project/proj-1/environment", results([env("e-prod", "prod", "proj-1", mode="PRODUCTION"),
                                                      env("e-stg", "stg", "proj-1", mode="STAGING")]))]),
    case("blueprint register already", ["rde", "blueprint", "register", "-p", "my-blueprint"]),
    case("blueprint register not found", ["rde", "blueprint", "register", "-p", "nope"]),
    case("blueprint register no env", ["rde", "blueprint", "register", "-p", "my-project"],
         [get("/project/proj-1/environment", results([]))]),
    case("blueprint register env list error", ["rde", "blueprint", "register", "-p", "my-project"],
         [get("/project/proj-1/environment", ERR500, 500)]),
    case("blueprint register create var error", ["rde", "blueprint", "register", "-p", "my-project"],
         [{"method": "POST", "path": "/variable", "status": 409, "body": {"detail": "already exists"}}]),
    case("blueprint register missing project", ["rde", "blueprint", "register"]),
    case("blueprint unregister", ["rde", "blueprint", "unregister", "-p", "my-blueprint"]),
    case("blueprint unregister no env", ["rde", "blueprint", "unregister", "-p", "empty-blueprint"]),
    case("blueprint unregister delete errors", ["rde", "blueprint", "unregister", "-p", "my-blueprint"],
         [{"method": "DELETE", "path_regex": "^/variable/", "status": 500, "body": ERR500}]),
    case("blueprint unregister not found", ["rde", "blueprint", "unregister", "-p", "nope"]),
    case("blueprint unregister missing project", ["rde", "blueprint", "unregister"]),

    # list
    case("list", ["rde", "list"]),
    case("list --json", ["rde", "list", "--json"]),
    case("list -b my-blueprint", ["rde", "list", "-b", "my-blueprint"]),
    case("list -b empty-blueprint", ["rde", "list", "-b", "empty-blueprint"]),
    case("list -b not found", ["rde", "list", "--blueprint", "nope"]),
    case("list status error", ["rde", "list"], [get("/environment/rdeenv-2/statuses", ERR500, 500)]),
    case("list json status error", ["rde", "list", "--json"], [get("/environment/rdeenv-1/statuses", ERR500, 500)]),
    case("list no url no history", ["rde", "list"],
         [get("/application/rapp-1/link", results([])), get("/environment/rdeenv-1/deploymentHistory", ERR500, 500),
          get("/project/bp-1", ERR500, 500)]),
    case("list error state", ["rde", "list"],
         [get("/environment/rdeenv-2/statuses", statuses("rdeenv-2", "DEPLOYMENT_ERROR"))]),
    case("list projects error", ["rde", "list"], [get("/organization/org-1/project", ERR500, 500)]),

    # info
    case("info", ["rde", "info"]),
    case("info no blueprints", ["rde", "info", "--organization", "my-org"],
         [get("/organization/org-1/project", results([project("proj-1", "my-project")]))]),
    case("info org name fallback", ["rde", "info"], [get("/organization", ERR500, 500)]),
    case("info projects error", ["rde", "info"], [get("/organization/org-1/project", ERR500, 500)]),
    case("info status errors", ["rde", "info"],
         [get("/environment/rdeenv-2/statuses", ERR500, 500),
          get("/environment/rdeenv-1/statuses", statuses("rdeenv-1", "DEPLOYING"))]),

    # urls
    case("urls", ["rde", "urls"]),
    case("urls -b", ["rde", "urls", "-b", "my-blueprint"]),
    case("urls none running", ["rde", "urls"],
         [get("/environment/rdeenv-1/statuses", statuses("rdeenv-1", "STOPPED"))]),
    case("urls no link", ["rde", "urls"], [get("/application/rapp-1/link", ERR500, 500)]),
    case("urls no children", ["rde", "urls", "-b", "empty-blueprint"]),

    # start / stop / status
    case("start", ["rde", "start", "-n", "alice"]),
    case("start not found", ["rde", "start", "-n", "nobody"]),
    case("start not a child", ["rde", "start", "-n", "alice"],
         [vars_fx("rdeenv-1", "ENVIRONMENT", [var("v-bk1", "BLUEPRINT_KEY", "rde-1", "ENVIRONMENT")])]),
    case("start env list error", ["rde", "start", "-n", "alice"], [get("/project/rde-1/environment", ERR500, 500)]),
    case("start api error", ["rde", "start", "--name", "alice"],
         [{"method": "POST", "path": "/environment/rdeenv-1/deploy", "status": 403, "body": ERR500}]),
    case("start --watch", ["rde", "start", "-n", "alice", "-w"],
         [watch_statuses("rdeenv-1", ["QUEUED", "DEPLOYED"])], timeout=40),
    case("start missing name", ["rde", "start"]),
    case("stop", ["rde", "stop", "-n", "bob"]),
    case("stop api error", ["rde", "stop", "-n", "bob"],
         [{"method": "POST", "path": "/environment/rdeenv-2/stop", "status": 500, "body": ERR500}]),
    case("stop --watch", ["rde", "stop", "-n", "alice", "--watch"],
         [watch_statuses("rdeenv-1", ["STOPPED"])], timeout=40),
    case("stop missing name", ["rde", "stop"]),
    case("status alice", ["rde", "status", "-n", "alice"]),
    case("status bob", ["rde", "status", "-n", "bob"]),
    case("status statuses error", ["rde", "status", "-n", "bob"],
         [get("/environment/rdeenv-2/statuses", ERR500, 500)]),
    case("status not found", ["rde", "status", "-n", "nobody"]),
    case("status missing name", ["rde", "status"]),

    # start-all / stop-all
    case("start-all", ["rde", "start-all"]),
    case("start-all partial failure", ["rde", "start-all"],
         [{"method": "POST", "path": "/environment/rdeenv-2/deploy", "status": 500, "body": ERR500}]),
    case("start-all -b empty", ["rde", "start-all", "-b", "empty-blueprint"]),
    case("start-all -b not found", ["rde", "start-all", "-b", "nope"]),
    case("stop-all", ["rde", "stop-all"]),
    case("stop-all -b", ["rde", "stop-all", "-b", "my-blueprint"]),
    case("stop-all partial failure", ["rde", "stop-all"],
         [{"method": "POST", "path": "/environment/rdeenv-1/stop", "status": 404, "body": ERR500}]),

    # create
    case("create", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "-e", "carol@example.com"], CREATE_FX),
    case("create no email", ["rde", "create", "-b", "my-blueprint", "-n", "carol"], CREATE_FX),
    case("create skip all", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "--skip-rbac", "--skip-invite",
                             "--skip-deploy", "-e", "carol@example.com"], CREATE_FX),
    case("create --cluster", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "-c", "MY-CLUSTER",
                              "--skip-invite"], CREATE_FX),
    case("create --cluster not found", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "-c", "nope"],
         CREATE_FX),
    case("create --cluster other gets viewer", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "-c", "c2"],
         [get("/organization/org-1/cluster", results([
             {"id": "cluster-1", "created_at": T, "organization": {"id": "org-1"}, "name": "my-cluster",
              "region": "us-east-2", "cloud_provider": "AWS"},
             {"id": "cluster-2", "created_at": T, "organization": {"id": "org-1"}, "name": "c2",
              "region": "eu-west-3", "cloud_provider": "AWS"}]))] + CREATE_FX),
    case("create blueprint not found", ["rde", "create", "-b", "nope", "-n", "carol"], CREATE_FX),
    case("create blueprint no env", ["rde", "create", "-b", "empty-blueprint", "-n", "carol"], CREATE_FX),
    case("create empty blueprint flag", ["rde", "create", "-b", "", "-n", "carol"], CREATE_FX),
    case("create empty name flag", ["rde", "create", "-b", "my-blueprint", "--name="], CREATE_FX),
    case("create missing flags", ["rde", "create"]),
    case("create missing name", ["rde", "create", "-b", "my-blueprint"]),
    case("create project exists", ["rde", "create", "-b", "my-blueprint", "-n", "alice", "--skip-rbac"],
         [{"method": "POST", "path": "/organization/org-1/project", "status": 400, "body": ERR500}] + CREATE_FX),
    case("create project error", ["rde", "create", "-b", "my-blueprint", "-n", "zed"],
         [{"method": "POST", "path": "/organization/org-1/project", "status": 500, "body": ERR500}] + CREATE_FX),
    case("create role exists", ["rde", "create", "-b", "my-blueprint", "-n", "alice"],
         [{"method": "POST", "path": "/organization/org-1/customRole", "status": 409, "body": ERR500},
          {"method": "PUT", "path": "/organization/org-1/customRole/role-a", "status": 500, "body": ERR500}]
         + CREATE_FX),
    case("create role error", ["rde", "create", "-b", "my-blueprint", "-n", "carol"],
         [{"method": "POST", "path": "/organization/org-1/customRole", "status": 500, "body": ERR500}] + CREATE_FX),
    case("create role no id", ["rde", "create", "-b", "my-blueprint", "-n", "carol"],
         [{"method": "POST", "path": "/organization/org-1/customRole", "status": 201, "body": {"name": "x"}}]
         + CREATE_FX),
    case("create clusters error", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "-c", "x"],
         [get("/organization/org-1/cluster", ERR500, 500)] + CREATE_FX),
    case("create clone error", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "--skip-rbac"],
         [{"method": "POST", "path": "/environment/bpenv-1/clone", "status": 500, "body": ERR500},
          get("/project/rde-new/environment", results([]))] + CREATE_FX),
    case("create bp env error and failures", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "-e",
                                              "carol@example.com"],
         [get("/environment/bpenv-1", ERR500, 500),
          {"method": "POST", "path": "/organization/org-1/inviteMember", "status": 400, "body": ERR500},
          {"method": "POST", "path": "/environment/newenv-1/deploy", "status": 500, "body": ERR500},
          get("/environment/newenv-1/job", ERR500, 500)] + CREATE_FX),
    case("create no ttl job", ["rde", "create", "-b", "my-blueprint", "-n", "carol", "--skip-rbac"],
         [get("/environment/newenv-1/job", results([]))] + CREATE_FX),

    # delete
    case("delete", ["rde", "delete", "-n", "alice"]),
    case("delete stopped", ["rde", "delete", "-n", "bob"]),
    case("delete not found", ["rde", "delete", "-n", "nobody"]),
    case("delete not found cleans role and token", ["rde", "delete", "-n", "nobody"],
         [get("/organization/org-1/customRole", results([{"id": "role-n", "name": "rde-NOBODY"}])),
          get("/organization/org-1/apiToken", results([{"id": "tok-n", "created_at": T, "name": "ttl-nobody"}]))]),
    case("delete api errors", ["rde", "delete", "-n", "bob"],
         [{"method": "DELETE", "path": "/environment/rdeenv-2", "status": 500, "body": ERR500},
          {"method": "DELETE", "path": "/project/rde-2", "status": 403, "body": ERR500},
          get("/organization/org-1/customRole", ERR500, 500), get("/organization/org-1/apiToken", ERR500, 500)]),
    case("delete missing name", ["rde", "delete"]),
    case("delete --watch (no-op flag)", ["rde", "delete", "-n", "bob", "--watch"]),

    # delete-all
    case("delete-all without confirm", ["rde", "delete-all"]),
    case("delete-all", ["rde", "delete-all", "--confirm"], timeout=40),
    case("delete-all -b empty", ["rde", "delete-all", "--confirm", "-b", "empty-blueprint"]),
    case("delete-all -b not found", ["rde", "delete-all", "--confirm", "-b", "nope"]),

    # upgrade
    case("upgrade bad strategy", ["rde", "upgrade", "-s", "foo"]),
    case("upgrade empty strategy", ["rde", "upgrade", "--strategy=", "-n", "bob"]),
    case("upgrade single image", ["rde", "upgrade", "-n", "alice"]),
    case("upgrade single image --sync-all", ["rde", "upgrade", "-n", "alice", "--sync-all"]),
    case("upgrade single image sync flags", ["rde", "upgrade", "-n", "alice", "--sync-resources", "--sync-ports"]),
    case("upgrade single image sync hc storage", ["rde", "upgrade", "-n", "alice", "--sync-healthchecks",
                                                  "--sync-storage"]),
    case("upgrade single image put errors", ["rde", "upgrade", "-n", "alice"],
         [{"method": "PUT", "path_regex": "^/(container|application|job|helm)/", "status": 400, "body": ERR500},
          {"method": "POST", "path": "/environment/rdeenv-1/deploy", "status": 500, "body": ERR500}]),
    case("upgrade single image nothing matched", ["rde", "upgrade", "-n", "bob"]),
    case("upgrade single no blueprint env", ["rde", "upgrade", "-n", "alice"],
         [vars_fx("bpenv-1", "ENVIRONMENT", [])]),
    case("upgrade single not found", ["rde", "upgrade", "-n", "nobody"]),
    case("upgrade all image", ["rde", "upgrade"]),
    case("upgrade all -b empty", ["rde", "upgrade", "-b", "empty-blueprint"]),
    case("upgrade all -b not found", ["rde", "upgrade", "-b", "nope"]),
    case("upgrade single reclone", ["rde", "upgrade", "-n", "alice", "-s", "reclone"], RECLONE_FX),
    case("upgrade single reclone clone error", ["rde", "upgrade", "-n", "alice", "-s", "reclone"],
         [{"method": "POST", "path": "/environment/bpenv-1/clone", "status": 500, "body": ERR500}] + RECLONE_FX),
    case("upgrade single reclone deploy error", ["rde", "upgrade", "-n", "bob", "-s", "reclone"],
         [{"method": "POST", "path": "/environment/newenv-1/deploy", "status": 500, "body": ERR500},
          get("/environment/bpenv-1", ERR500, 500)] + RECLONE_FX),
    case("upgrade single reclone no blueprint env", ["rde", "upgrade", "-n", "alice", "-s", "reclone"],
         [vars_fx("bpenv-1", "ENVIRONMENT", [])] + RECLONE_FX),
    case("upgrade all reclone (one child)", ["rde", "upgrade", "-s", "reclone"],
         [get("/project/rde-2/environment", results([]))] + RECLONE_FX),
    case("upgrade all reclone no blueprint env", ["rde", "upgrade", "-s", "reclone"],
         [get("/project/rde-2/environment", results([])), vars_fx("bpenv-1", "ENVIRONMENT", [])] + RECLONE_FX),
    case("upgrade all reclone failures", ["rde", "upgrade", "-s", "reclone"],
         [get("/project/rde-2/environment", results([])),
          {"method": "POST", "path": "/environment/bpenv-1/clone", "status": 500, "body": ERR500}] + RECLONE_FX),
]

# A blueprint whose child environment does not exist anymore
json.dump({"fixtures": FIXTURES, "cases": CASES}, open(OUT, "w"), indent=1)
print(len(CASES), "cases")
