#!/usr/bin/env python3
"""Generates parity/scenarios/terraform.json.

Usage: python3 parity/generators/terraform.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "terraform.json")

def status(id, state, exec_id):
    return {"id": id, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": exec_id}

def swst(*seq):
    return {"method": "GET", "path": "/environment/env-1/statusesWithStages",
            "responses": [{"status": 200, "body": {"stages": [{"terraforms": [status(*d) for d in s]}]}} for s in seq]}

def tf(id, name, updated="2024-07-03T08:09:10.5+02:00", engine="TERRAFORM"):
    t = {"id": id, "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": updated, "name": name,
         "timeout_sec": 3600, "auto_deploy": False, "icon_uri": "app://qovery-console/terraform", "service_type": "TERRAFORM",
         "terraform_variables_source": {"tf_var_file_paths": [], "tf_vars": []}, "engine": engine,
         "backend": {"kubernetes": {}}, "provider_version": {"read_from_terraform_block": False, "explicit_version": "1.9.0"},
         "job_resources": {"cpu_milli": 500, "ram_mib": 512, "gpu": 0, "storage_gib": 1}, "environment": {"id": "env-1"},
         "use_cluster_credentials": False, "action_extra_arguments": {}}
    return t

TFS = [tf("tf-1", "my-terraform", "2024-06-02T11:30:00Z"), tf("tf-2", "infra <net>", engine="OPEN_TOFU")]
STATUSES = {"environment": {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED"},
            "applications": [], "containers": [], "jobs": [], "helms": [], "databases": [],
            "terraforms": [status("tf-1", "DEPLOYED", "e3")]}

def var(id, key, value, scope="TERRAFORM", **kw):
    v = {"id": id, "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": "2024-06-02T11:30:00Z", "key": key,
         "value": value, "scope": scope, "variable_type": "VALUE", "is_secret": False}
    v.update(kw)
    return v

SMA = {"id": "sma-1", "name": "my-sma", "created_at": "2024-05-01T10:00:00Z", "updated_at": "2024-05-01T10:00:00Z",
       "endpoint": {"mode": "AWS_SECRET_MANAGER", "region": "us-east-2"}, "authentication": {"mode": "AUTOMATICALLY_CONFIGURED"}}
CLUSTERS = {"results": [
    {"id": "cluster-0", "created_at": "2024-05-01T10:00:00Z", "organization": {"id": "org-1"}, "name": "other", "region": "eu-west-3", "cloud_provider": "AWS"},
    {"id": "cluster-1", "created_at": "2024-05-01T10:00:00Z", "organization": {"id": "org-1"}, "name": "my-cluster", "region": "us-east-2", "cloud_provider": "AWS", "secret_manager_accesses": [SMA]}]}

KUBECONFIG = """apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:1
    insecure-skip-tls-verify: true
  name: c
contexts:
- context:
    cluster: c
    user: u
  name: ctx
current-context: ctx
users:
- name: u
  user:
    token: abc
"""

fixtures = [
    {"method": "GET", "path": "/environment/env-1/terraform", "body": {"results": TFS}},
    {"method": "GET", "path": "/environment/env-1/statuses", "body": STATUSES},
    {"method": "GET", "path": "/organization/org-1/cluster", "body": CLUSTERS},
    {"method": "GET", "path": "/variable", "query": {"parent_id": "tf-1", "scope": "TERRAFORM"},
     "body": {"results": [var("var-1", "MY_SECRET", "aws:ref", variable_type="VALUE"), var("var-2", "OTHER", "x")]}},
    {"method": "POST", "path": "/variable", "status": 201, "body": var("var-9", "NEW_SECRET", "aws:new")},
    {"method": "PUT", "path": "/variable/var-1", "status": 200, "body": var("var-1", "MY_SECRET", "aws:new")},
    {"method": "DELETE", "path": "/variable/var-1", "status": 204},
    {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202, "body": {"id": "env-1", "state": "DEPLOYMENT_QUEUED", "last_deployment_state": "DEPLOYED"}},
    {"method": "POST", "path_regex": "^/terraform/[^/]+/deploy$", "status": 202, "body": status("tf-1", "DEPLOYMENT_QUEUED", "e3")},
    {"method": "GET", "path": "/terraform/tf-1/commit", "body": {"results": [
        {"created_at": "2024-05-01T10:00:00Z", "git_commit_id": "old111", "tag": "", "message": "old", "author_name": "me"},
        {"created_at": "2024-06-01T10:00:00Z", "git_commit_id": "new222", "tag": "v1", "message": "newest commit\nbody", "author_name": "me"}]}},
    {"method": "GET", "path": "/terraform/tf-2/commit", "body": {"results": []}},
    {"method": "DELETE", "path_regex": "^/terraform/[^/]+$", "status": 204},
    {"method": "GET", "path": "/terraform/tf-2", "body": tf("tf-2", "infra <net>", engine="OPEN_TOFU")},
    {"method": "GET", "path": "/organization/00000000-0000-0000-000000000000/cluster/cluster-1/kubeconfig", "body_raw": KUBECONFIG, "headers": {"Content-Type": "text/plain"}},
]

def err(method, path, code=500, body=None):
    return {"method": method, "path": path, "status": code, "body": body if body is not None else {"status": code, "error": "boom"}}

cases = []
def case(name, args, **kw):
    c = {"name": name, "args": args}
    c.update(kw)
    cases.append(c)

case("group help", ["terraform"])
case("external-secret group help", ["terraform", "external-secret"])
case("list table", ["terraform", "list"])
case("list json", ["terraform", "list", "--json"])
case("list explicit env", ["terraform", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env", "--json"])
case("list env not found", ["terraform", "list", "--environment", "nope"])
case("list empty table", ["terraform", "list"], fixtures=[{"method": "GET", "path": "/environment/env-1/terraform", "body": {"results": []}}])
case("list empty json", ["terraform", "list", "--json"], fixtures=[{"method": "GET", "path": "/environment/env-1/terraform", "body": {"results": []}}])
case("list error", ["terraform", "list"], fixtures=[err("GET", "/environment/env-1/terraform")])
case("list statuses error", ["terraform", "list"], fixtures=[err("GET", "/environment/env-1/statuses", 404)])
case("list no token", ["terraform", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)

DEPLOYS = [("plan-and-apply", None), ("plan", "PLAN"), ("force-unlock", "FORCE_UNLOCK"), ("migrate-state", "MIGRATE_STATE")]
for cmd, action in DEPLOYS:
    post_path = "/environment/env-1/service/deploy" if action is None else "/terraform/tf-1/deploy"
    case(cmd, ["terraform", cmd, "-n", "my-terraform"])
    case(cmd + " multiple", ["terraform", cmd, "--terraforms", "my-terraform, infra <net>"])
    case(cmd + " commit", ["terraform", cmd, "-n", "my-terraform", "-c", "abc123"])
    case(cmd + " commit latest", ["terraform", cmd, "-n", "my-terraform", "--commit-id", " LATEST "])
    case(cmd + " commit latest none", ["terraform", cmd, "--terraforms", "my-terraform,infra <net>", "--commit-id", "latest"])
    case(cmd + " commit latest error", ["terraform", cmd, "-n", "my-terraform", "--commit-id", "latest"], fixtures=[err("GET", "/terraform/tf-1/commit", 403)])
    case(cmd + " no name", ["terraform", cmd])
    case(cmd + " both", ["terraform", cmd, "-n", "my-terraform", "--terraforms", "x"])
    case(cmd + " not found", ["terraform", cmd, "-n", "nope"])
    case(cmd + " multiple not found", ["terraform", cmd, "--terraforms", "my-terraform, nope"])
    case(cmd + " api error", ["terraform", cmd, "-n", "my-terraform"], fixtures=[err("POST", post_path, 400)])
    case(cmd + " env not found", ["terraform", cmd, "-n", "my-terraform", "--environment", "nope"])
    case(cmd + " watch", ["terraform", cmd, "-n", "my-terraform", "-w"],
         fixtures=[swst([("tf-1", "DEPLOYED", "x1")], [("tf-1", "DEPLOYING", "x2")], [("tf-1", "DEPLOYED", "x2")])], timeout=60)
    case(cmd + " watch error", ["terraform", cmd, "-n", "my-terraform", "-w"],
         fixtures=[swst([("tf-1", "DEPLOYED", "x1")], [("tf-1", "DEPLOYMENT_ERROR", "x2")])], timeout=60)

case("delete", ["terraform", "delete", "-n", "my-terraform"])
case("delete multiple", ["terraform", "delete", "--terraforms", "my-terraform,infra <net>"])
case("delete skip-destroy", ["terraform", "delete", "-n", "my-terraform", "--skip-destroy"])
case("delete resources-only", ["terraform", "delete", "-n", "my-terraform", "--resources-only"])
case("delete both flags", ["terraform", "delete", "-n", "my-terraform", "--resources-only", "--skip-destroy"])
case("delete no name", ["terraform", "delete"])
case("delete both names", ["terraform", "delete", "-n", "a", "--terraforms", "b"])
case("delete not found", ["terraform", "delete", "-n", "nope"])
case("delete api error", ["terraform", "delete", "-n", "my-terraform"], fixtures=[err("DELETE", "/terraform/tf-1", 409)])
case("delete watch", ["terraform", "delete", "-n", "my-terraform", "-w"],
     fixtures=[swst([("tf-1", "DEPLOYED", "x1")], [("tf-1", "DELETING", "x2")], [])], timeout=60)
case("delete resources-only watch", ["terraform", "delete", "-n", "my-terraform", "-w", "--resources-only"],
     fixtures=[swst([("tf-1", "DEPLOYED", "x1")], [("tf-1", "DELETED", "x2")])], timeout=60)
case("delete watch error", ["terraform", "delete", "-n", "my-terraform", "-w"],
     fixtures=[swst([("tf-1", "DEPLOYED", "x1")], [("tf-1", "DELETE_ERROR", "x2")])], timeout=60)

ES = ["terraform", "external-secret"]
case("es create", ES + ["create", "-n", "my-terraform", "-k", "NEW_SECRET", "-r", "aws:new", "--secret-manager-access-name", "my-sma"])
case("es create mount-path scope env", ES + ["create", "-n", "my-terraform", "-k", "NEW_SECRET", "-r", "aws:new", "--secret-manager-access-name", "my-sma", "--scope", "ENVIRONMENT", "--mount-path", "/etc/s"])
case("es create scope project", ES + ["create", "-n", "my-terraform", "-k", "NEW_SECRET", "-r", "aws:new", "--secret-manager-access-name", "my-sma", "--scope", "PROJECT"])
case("es create bad scope", ES + ["create", "-n", "my-terraform", "-k", "NEW_SECRET", "-r", "aws:new", "--secret-manager-access-name", "my-sma", "--scope", "NOPE"])
case("es create missing flags", ES + ["create"])
case("es create missing key", ES + ["create", "-n", "my-terraform", "-r", "x", "--secret-manager-access-name", "my-sma"])
case("es create tf not found", ES + ["create", "-n", "nope", "-k", "K", "-r", "x", "--secret-manager-access-name", "my-sma"])
case("es create sma not found", ES + ["create", "-n", "my-terraform", "-k", "K", "-r", "x", "--secret-manager-access-name", "nope"])
case("es create cluster not found", ES + ["create", "-n", "my-terraform", "-k", "K", "-r", "x", "--secret-manager-access-name", "my-sma"],
     fixtures=[{"method": "GET", "path": "/organization/org-1/cluster", "body": {"results": []}}])
case("es create clusters error", ES + ["create", "-n", "my-terraform", "-k", "K", "-r", "x", "--secret-manager-access-name", "my-sma"],
     fixtures=[err("GET", "/organization/org-1/cluster", 500)])
case("es create conflict", ES + ["create", "-n", "my-terraform", "-k", "K", "-r", "x", "--secret-manager-access-name", "my-sma"],
     fixtures=[err("POST", "/variable", 409, {"status": 409, "detail": "variable K already exists in scope ENVIRONMENT"})])
case("es create list error", ES + ["create", "-n", "my-terraform", "-k", "K", "-r", "x", "--secret-manager-access-name", "my-sma"],
     fixtures=[err("GET", "/environment/env-1/terraform", 500)])
case("es create env not found", ES + ["create", "-n", "my-terraform", "-k", "K", "-r", "x", "--secret-manager-access-name", "my-sma", "--project", "nope"])

case("es update", ES + ["update", "-n", "my-terraform", "-k", "MY_SECRET", "-r", "aws:new", "--secret-manager-access-name", "my-sma"])
case("es update reference only", ES + ["update", "-n", "my-terraform", "-k", "MY_SECRET", "-r", "aws:new"],
     fixtures=[{"method": "GET", "path": "/organization/org-1/cluster", "body": {"results": [dict(CLUSTERS["results"][1], secret_manager_accesses=[dict(SMA, name="")])]}}])
case("es update sma empty not found", ES + ["update", "-n", "my-terraform", "-k", "MY_SECRET", "-r", "aws:new"])
case("es update key not found", ES + ["update", "-n", "my-terraform", "-k", "NOPE", "-r", "x", "--secret-manager-access-name", "my-sma"])
case("es update missing flags", ES + ["update", "-k", "X"])
case("es update tf not found", ES + ["update", "-n", "nope", "-k", "X"])
case("es update api error", ES + ["update", "-n", "my-terraform", "-k", "MY_SECRET", "-r", "x", "--secret-manager-access-name", "my-sma"],
     fixtures=[err("PUT", "/variable/var-1", 400)])
case("es update list vars error", ES + ["update", "-n", "my-terraform", "-k", "MY_SECRET", "-r", "x", "--secret-manager-access-name", "my-sma"],
     fixtures=[err("GET", "/variable", 500)])

case("es delete", ES + ["delete", "-n", "my-terraform", "-k", "MY_SECRET"])
case("es delete key not found", ES + ["delete", "-n", "my-terraform", "-k", "NOPE"])
case("es delete missing flags", ES + ["delete"])
case("es delete tf not found", ES + ["delete", "--terraform", "nope", "--key", "MY_SECRET"])
case("es delete api error", ES + ["delete", "-n", "my-terraform", "-k", "MY_SECRET"], fixtures=[err("DELETE", "/variable/var-1", 403)])

SB = ["terraform", "setup-backend"]
case("setup-backend connection refused", SB + ["-t", "tf-1"], timeout=60)
case("setup-backend no flag", SB, timeout=60)
case("setup-backend tf not found", SB + ["--terraform", "nope"])
case("setup-backend env error", SB + ["-t", "tf-1"], fixtures=[err("GET", "/environment/env-1", 500)])
case("setup-backend kubeconfig 404", SB + ["-t", "tf-1"], fixtures=[err("GET", "/organization/00000000-0000-0000-000000000000/cluster/cluster-1/kubeconfig", 404)])
case("setup-backend kubeconfig 204", SB + ["-t", "tf-1"], fixtures=[{"method": "GET", "path": "/organization/00000000-0000-0000-000000000000/cluster/cluster-1/kubeconfig", "status": 204}])
case("setup-backend no token", SB + ["-t", "tf-1"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)

KP = "/organization/00000000-0000-0000-000000000000/cluster/cluster-1/kubeconfig"
def kc(body): return [{"method": "GET", "path": KP, "body_raw": body, "headers": {"Content-Type": "text/plain"}}]
case("setup-backend kubeconfig empty", SB + ["-t", "tf-1"], fixtures=kc(""))
case("setup-backend kubeconfig no context", SB + ["-t", "tf-1"], fixtures=kc("apiVersion: v1\nkind: Config\nclusters: []\n"))
case("setup-backend http connection refused", SB + ["-t", "tf-2"], fixtures=kc(KUBECONFIG.replace("https://127.0.0.1:1", "http://127.0.0.1:9").replace("    insecure-skip-tls-verify: true\n", "")))

json.dump({"fixtures": fixtures, "cases": cases}, open(OUT, "w"), indent=1)
print(len(cases), "cases")
