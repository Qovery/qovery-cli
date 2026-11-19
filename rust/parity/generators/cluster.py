#!/usr/bin/env python3
"""Generates parity/scenarios/cluster.json.

Usage: python3 parity/generators/cluster.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "cluster.json")

def status(state, next_version=None):
    b = {"cluster_id": "cluster-1", "status": state, "is_deployed": state == "DEPLOYED", "reason": "UNSPECIFIED"}
    if next_version is not None:
        b["next_k8s_available_version"] = next_version
    return b

def fx(method, path, body=None, status_code=200, **kw):
    r = {"method": method, "path": path, "status": status_code}
    if body is not None:
        r["body"] = body
    r.update(kw)
    return r

STATUS_PATH = "/organization/org-1/cluster/cluster-1/status"
analysis = lambda id_, status_, err=None: dict({
    "id": id_, "cluster_id": "cluster-1", "organization_id": "org-1", "kind": "COST_RECOMMENDATION", "status": status_,
    "output_format": "JSON", "created_at": "2024-07-01T10:00:00.123456Z", "updated_at": "2024-07-01T10:05:00Z",
    "triggered_by": "user@qovery.com"}, **({"error_message": err} if err else {}))
logs = {"results": [
    {"timestamp": "2024-07-01T10:00:01.5Z", "level": "INFO", "message": "line one", "line_order": 1},
    {"timestamp": "2024-07-01T10:00:02Z", "level": "INFO", "message": "line <two> & \"three\"", "line_order": 2},
]}
locks = {"results": [
    {"cluster_id": "cluster-1", "locked_at": "2024-07-01T10:00:00Z", "owner_name": "John Doe", "reason": "maintenance", "ttl_in_days": 3},
    {"cluster_id": "cluster-long-identifier-2", "locked_at": "2024-07-02T11:30:15.5Z", "owner_name": "Jane", "reason": "upgrade"},
]}
token = {"apiVersion": "client.authentication.k8s.io/v1", "kind": "ExecCredential", "spec": {"interactive": False},
         "status": {"expirationTimestamp": "2024-07-01T10:00:00Z", "token": "k8s-token"}}
operator = {"organization_id": "org-1", "cluster_id": "cluster-1", "operator_connected": True, "status": "OUTDATED_IMAGE",
            "reported_runtime": "KUBERNETES", "last_heartbeat": "2024-07-01T10:00:00.000Z", "operator_version": "1.2.3",
            "desired_image_version": "1.3.0", "reported_chart_version": None, "desired_chart_version": "", "extra_field": "<x>"}
KUBECONFIG_OK = """apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:1
  name: c1
contexts:
- context:
    cluster: c1
    user: u1
  name: ctx1
- context:
    cluster: missing
    user: u1
  name: no-cluster
- context:
    cluster: c2
    user: u1
  name: no-server
current-context: ctx1
users:
- name: u1
  user:
    token: abc
clusters2: []
"""
KUBECONFIG_OK = KUBECONFIG_OK.replace("clusters2: []\n", "") .replace("  name: c1\n", "  name: c1\n- cluster:\n    insecure-skip-tls-verify: true\n  name: c2\n")
BASE_VALUES = """services:
  qovery:
    qovery-engine:
      enabled: true
  ingress:
    ingress-nginx:
      enabled: true
external-dns:
  provider:
    name: pdns
qovery:
  clusterId: &clusterId set-by-customer
  domain: &domain set-by-customer
qovery-cluster-agent:
  environmentVariables:
    CLUSTER_ID: *clusterId
    ORGANIZATION_ID: "0123"
    LIST: [a, "yes", 1.0]
"""
INSTALL_VALUES = """qovery:
  clusterId: &clusterId new-c
  domain: &domain zabc.example.com
  acmeEmailAddr: acme@qovery.com
"""

cases = []
def case(name, args, **kw):
    c = {"name": name, "args": args}
    c.update(kw)
    cases.append(c)

# group
case("cluster (group)", ["cluster"])
case("cluster unknown sub", ["cluster", "nope"])
case("cluster analysis (group)", ["cluster", "analysis"])
case("cluster operator (group)", ["cluster", "operator"])
case("cluster operator extra arg", ["cluster", "operator", "foo"])

# list
case("list", ["cluster", "list"])
case("list json", ["cluster", "list", "--json"])
case("list org flag", ["cluster", "list", "--organization", "my-org", "--json"])
case("list org not found", ["cluster", "list", "--organization", "nope"])
case("list empty json", ["cluster", "list", "--json"], fixtures=[fx("GET", "/organization/org-1/cluster", {"results": []})])
case("list empty table", ["cluster", "list"], fixtures=[fx("GET", "/organization/org-1/cluster", {"results": []})])
case("list api error", ["cluster", "list"], fixtures=[fx("GET", "/organization/org-1/cluster", {"message": "boom"}, 500)])
case("list no token", ["cluster", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
case("list statuses", ["cluster", "list"], fixtures=[fx("GET", "/organization/org-1/cluster", {"results": [
    {"id": "c-%d" % i, "created_at": "2024-05-01T10:00:00Z", "updated_at": "2024-06-02T11:30:00+02:00", "organization": {"id": "org-1"},
     "name": "n%d" % i, "region": "r", "cloud_provider": "AWS", "status": s}
    for i, s in enumerate(["DEPLOYING", "DEPLOYMENT_ERROR", "STOPPED", "INVALID_CREDENTIALS", "DEPLOYMENT_QUEUED", "UNKNOWN"])]})])

# deploy
deploy_ok = fx("POST", "/organization/org-1/cluster/cluster-1/deploy", status("DEPLOYING"))
case("deploy", ["cluster", "deploy", "-n", "my-cluster"], fixtures=[deploy_ok])
case("deploy org flag", ["cluster", "deploy", "--organization", "my-org", "--cluster", "my-cluster"], fixtures=[deploy_ok])
case("deploy watch", ["cluster", "deploy", "-n", "my-cluster", "-w"], fixtures=[deploy_ok, fx("GET", STATUS_PATH, status("DEPLOYED"))])
case("deploy watch polling", ["cluster", "deploy", "-n", "my-cluster", "-w"],
     fixtures=[deploy_ok, {"method": "GET", "path": STATUS_PATH, "responses": [{"status": 200, "body": status("DEPLOYING")}, {"status": 200, "body": status("DEPLOYED")}]}])
case("deploy watch status error", ["cluster", "deploy", "-n", "my-cluster", "-w"], fixtures=[deploy_ok, fx("GET", STATUS_PATH, {"m": 1}, 500)])
case("deploy not found", ["cluster", "deploy", "-n", "nope"])
case("deploy missing cluster", ["cluster", "deploy"])
case("deploy api error", ["cluster", "deploy", "-n", "my-cluster"], fixtures=[fx("POST", "/organization/org-1/cluster/cluster-1/deploy", {"message": "bad"}, 400)])
case("deploy accepted", ["cluster", "deploy", "-n", "my-cluster"], fixtures=[fx("POST", "/organization/org-1/cluster/cluster-1/deploy", status("DEPLOYING"), 202)])
case("deploy trimmed name", ["cluster", "deploy", "-n", " my-cluster "], fixtures=[deploy_ok])

# stop
stop_ok = fx("POST", "/organization/org-1/cluster/cluster-1/stop", status("STOPPING"))
case("stop", ["cluster", "stop", "-n", "my-cluster"], fixtures=[stop_ok])
case("stop watch", ["cluster", "stop", "-n", "my-cluster", "--watch"], fixtures=[stop_ok, fx("GET", STATUS_PATH, status("STOPPED"))])
case("stop not found", ["cluster", "stop", "-n", "nope"])
case("stop api error", ["cluster", "stop", "-n", "my-cluster"], fixtures=[fx("POST", "/organization/org-1/cluster/cluster-1/stop", {"message": "bad"}, 400)])
case("stop org not found", ["cluster", "stop", "-n", "my-cluster", "--organization", "nope"])

# upgrade
up_status = fx("GET", STATUS_PATH, status("DEPLOYED", "1.31"))
up_ok = fx("POST", "/cluster/cluster-1/upgrade", status("DEPLOYING"))
case("upgrade skip confirmation", ["cluster", "upgrade", "-n", "my-cluster", "-y"], fixtures=[up_status, up_ok])
case("upgrade watch", ["cluster", "upgrade", "-n", "my-cluster", "-y", "-w"], fixtures=[up_status, up_ok])
case("upgrade no next version", ["cluster", "upgrade", "-n", "my-cluster", "-y"], fixtures=[fx("GET", STATUS_PATH, status("DEPLOYED"))])
case("upgrade null next version", ["cluster", "upgrade", "-n", "my-cluster", "-y"], fixtures=[fx("GET", STATUS_PATH, status("DEPLOYED", None) | {"next_k8s_available_version": None})])
case("upgrade not found", ["cluster", "upgrade", "-n", "nope", "-y"])
case("upgrade api error", ["cluster", "upgrade", "-n", "my-cluster", "-y"], fixtures=[up_status, fx("POST", "/cluster/cluster-1/upgrade", {"message": "nope"}, 400)])
case("upgrade status error", ["cluster", "upgrade", "-n", "my-cluster", "-y"], fixtures=[fx("GET", STATUS_PATH, {}, 404)])
case("upgrade prompt yes (piped)", ["cluster", "upgrade", "-n", "my-cluster"], stdin="\n", fixtures=[up_status, up_ok])
case("upgrade prompt eof (piped)", ["cluster", "upgrade", "-n", "my-cluster"], fixtures=[up_status, up_ok])
case("upgrade missing cluster", ["cluster", "upgrade"])

# get-token
case("get-token", ["cluster", "get-token", "-c", "cluster-1"], fixtures=[fx("GET", "/cluster/cluster-1/token", token)])
case("get-token read-only", ["cluster", "get-token", "--cluster-id", "cluster-1", "-r"], fixtures=[fx("GET", "/cluster/cluster-1/token", token, query={"read_only": "true"})])
case("get-token no cluster id", ["cluster", "get-token"])
case("get-token api error", ["cluster", "get-token", "-c", "cluster-1"], fixtures=[fx("GET", "/cluster/cluster-1/token", {"error": "x"}, 403)])
case("get-token missing property", ["cluster", "get-token", "-c", "cluster-1"], fixtures=[fx("GET", "/cluster/cluster-1/token", {"kind": "ExecCredential"})])
case("get-token no token", ["cluster", "get-token", "-c", "cluster-1"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

# kubeconfig
KC_PATH = "/organization/00000000-0000-0000-000000000000/cluster/cluster-1/kubeconfig"
case("kubeconfig", ["cluster", "kubeconfig", "-c", "cluster-1"], fixtures=[fx("GET", KC_PATH, body_raw="apiVersion: v1\nkind: Config\n", query={"with_token_from_cli": "true"})])
case("kubeconfig read-only", ["cluster", "kubeconfig", "-c", "cluster-1", "--read-only"], fixtures=[fx("GET", KC_PATH, body_raw="apiVersion: v1\n", query={"read_only": "true"})])
case("kubeconfig no cluster id", ["cluster", "kubeconfig"])
case("kubeconfig api error", ["cluster", "kubeconfig", "-c", "cluster-1"], fixtures=[fx("GET", KC_PATH, {"e": 1}, 404)])

# lock / unlock / locked
LOCK = "/cluster/cluster-1/lock"
lock_ok = fx("POST", LOCK, {"reason": "maint", "cluster_id": "cluster-1", "locked_at": "2024-07-01T10:00:00Z", "owner_name": "me"})
case("lock yes", ["cluster", "lock", "-c", "cluster-1", "-r", "maint"], stdin="yes\n", fixtures=[lock_ok])
case("lock ttl", ["cluster", "lock", "-c", "cluster-1", "-r", "maint", "-d", "3"], stdin="\nyes\n", fixtures=[lock_ok])
case("lock no", ["cluster", "lock", "-c", "cluster-1", "-r", "maint"], stdin="no\n", fixtures=[lock_ok])
case("lock eof", ["cluster", "lock", "-c", "cluster-1", "-r", "maint"])
case("lock api error detail", ["cluster", "lock", "-c", "cluster-1", "-r", "maint"], stdin="yes\n", fixtures=[fx("POST", LOCK, {"detail": "already locked"}, 400)])
case("lock api error no detail", ["cluster", "lock", "-c", "cluster-1", "-r", "maint"], stdin="yes\n", fixtures=[fx("POST", LOCK, body_raw="oops", status_code=500)])
case("lock missing flags", ["cluster", "lock"])
case("lock missing reason", ["cluster", "lock", "-c", "cluster-1"])
case("lock bad ttl", ["cluster", "lock", "-c", "cluster-1", "-r", "x", "-d", "abc"])
case("unlock yes", ["cluster", "unlock", "-c", "cluster-1"], stdin="yes\n", fixtures=[fx("DELETE", LOCK, status_code=204)])
case("unlock error", ["cluster", "unlock", "-c", "cluster-1"], stdin="yes\n", fixtures=[fx("DELETE", LOCK, {"status": 404}, 404)])
case("unlock no", ["cluster", "unlock"], stdin="nope\n")
case("locked", ["cluster", "locked", "-o", "org-1"], fixtures=[fx("GET", "/organization/org-1/lock", locks)])
case("locked empty", ["cluster", "locked", "--organization-id", "org-1"], fixtures=[fx("GET", "/organization/org-1/lock", {"results": []})])
case("locked api error", ["cluster", "locked", "-o", "org-1"], fixtures=[fx("GET", "/organization/org-1/lock", {"m": "x"}, 500)])
case("locked missing org", ["cluster", "locked"])

# analysis
A = "/clusters/cluster-1/analysis"
case("analysis list", ["cluster", "analysis", "list", "-c", "cluster-1"], fixtures=[fx("GET", A, {"results": [analysis("an-1", "SUCCEEDED"), analysis("an-2", "FAILED", "boom")]})])
case("analysis list json", ["cluster", "analysis", "list", "-c", "cluster-1", "--json"], fixtures=[fx("GET", A, {"results": [analysis("an-1", "SUCCEEDED"), analysis("an-2", "FAILED", "boom")]})])
case("analysis list empty json", ["cluster", "analysis", "list", "-c", "cluster-1", "--json"], fixtures=[fx("GET", A, {"results": []})])
case("analysis list api error", ["cluster", "analysis", "list", "-c", "cluster-1"])
case("analysis list missing cluster", ["cluster", "analysis", "list"])
case("analysis logs", ["cluster", "analysis", "logs", "-c", "cluster-1", "-a", "an-1"], fixtures=[fx("GET", A + "/an-1/logs", logs)])
case("analysis logs json", ["cluster", "analysis", "logs", "-c", "cluster-1", "-a", "an-1", "--json"], fixtures=[fx("GET", A + "/an-1/logs", logs)])
case("analysis logs api error", ["cluster", "analysis", "logs", "-c", "cluster-1", "-a", "an-1"], fixtures=[fx("GET", A + "/an-1/logs", {"message": "denied"}, 403)])
case("analysis logs missing analysis", ["cluster", "analysis", "logs", "-c", "cluster-1"])
case("cost-recommendation no watch", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1", "--watch=false"], fixtures=[fx("POST", A, analysis("an-9", "PENDING"))])
case("cost-recommendation flags", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1", "--watch=false", "--output", " TABLE ",
                                   "--prometheus-url", "http://prom:9090", "--cmd-arg=--history_duration", "--cmd-arg", "336"], fixtures=[fx("POST", A, analysis("an-9", "PENDING"))])
case("cost-recommendation watch succeeded", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1"],
     fixtures=[fx("POST", A, analysis("an-9", "SUCCEEDED")), fx("GET", A + "/an-9/logs", logs)])
case("cost-recommendation watch no-logs", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1", "--no-logs"],
     fixtures=[fx("POST", A, analysis("an-9", "SUCCEEDED"))])
case("cost-recommendation watch polling", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1"],
     fixtures=[fx("POST", A, analysis("an-9", "PENDING")), {"method": "GET", "path": A + "/an-9", "responses": [{"status": 200, "body": analysis("an-9", "FAILED", "engine crashed")}]},
               fx("GET", A + "/an-9/logs", logs)])
case("cost-recommendation invalid output", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1", "--output", "yaml"])
case("cost-recommendation api error", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1"], fixtures=[fx("POST", A, {"message": "nope"}, 400)])
case("cost-recommendation logs error", ["cluster", "analysis", "cost-recommendation", "-c", "cluster-1"], fixtures=[fx("POST", A, analysis("an-9", "SUCCEEDED"))])
case("deprecated-api failed", ["cluster", "analysis", "deprecated-api", "-c", "cluster-1", "--target-kubernetes-version", "1.33", "--output", "csv"],
     fixtures=[fx("POST", A, dict(analysis("an-7", "FAILED", "x"), kind="DEPRECATED_API_CHECK")), fx("GET", A + "/an-7/logs", {"results": []})])
case("deprecated-api missing cluster", ["cluster", "analysis", "deprecated-api"])

# operator status
OP = "/organization/org-1/cluster/cluster-1/operator/status"
case("operator status", ["cluster", "operator", "status", "--cluster", "my-cluster"], fixtures=[fx("GET", OP, operator)])
case("operator status json", ["cluster", "operator", "status", "--cluster", "my-cluster", "--json"], fixtures=[fx("GET", OP, operator)])
case("operator status never", ["cluster", "operator", "status", "--cluster", "my-cluster"],
     fixtures=[fx("GET", OP, {"organization_id": "org-1", "cluster_id": "cluster-1", "operator_connected": False, "status": "NOT_ATTACHED", "reported_runtime": "UNKNOWN"})])
case("operator status not found", ["cluster", "operator", "status", "--cluster", "nope"])
case("operator status api error", ["cluster", "operator", "status", "--cluster", "my-cluster", "--organization", "my-org"], fixtures=[fx("GET", OP, {"m": 1}, 500)])
case("operator status missing cluster", ["cluster", "operator", "status"])

# list-nodes / debug-pod (websocket: the mock refuses the upgrade)
case("list-nodes bad handshake", ["cluster", "list-nodes", "-c", "cluster-1"])
case("list-nodes ws refused", ["cluster", "list-nodes", "-c", "cluster-1"], env={"QOVERY_WS_URL": "ws://127.0.0.1:1"})
case("debug-pod reconnects", ["cluster", "debug-pod", "-c", "cluster-1", "-n", "pool=a b", "-p"], timeout=3)
case("debug-pod org id", ["cluster", "debug-pod", "-c", "cluster-1", "-o", "org-x"], timeout=3)
case("debug-pod missing cluster", ["cluster", "debug-pod"])

# preflight (no Kubernetes API reachable: 127.0.0.1:1)
files = {"kc": KUBECONFIG_OK}
case("preflight unreachable", ["cluster", "preflight", "--kubeconfig", "kc"], files=files)
case("preflight unreachable json", ["cluster", "preflight", "--kubeconfig", "kc", "--json", "--install-path", "CLI", "--loki-object-storage"], files=files)
case("preflight default kubeconfig", ["cluster", "preflight", "--context", "ctx1"], files={".kube/config": KUBECONFIG_OK})
case("preflight KUBECONFIG env", ["cluster", "preflight"], files={"a/kc": KUBECONFIG_OK}, env={"KUBECONFIG": "/nonexistent:a/kc"})
case("preflight missing kubeconfig", ["cluster", "preflight", "--kubeconfig", "nope"])
case("preflight unknown context", ["cluster", "preflight", "--kubeconfig", "kc", "--context", "nope"], files=files)
case("preflight context without cluster", ["cluster", "preflight", "--kubeconfig", "kc", "--context", "no-cluster"], files=files)
case("preflight context without server", ["cluster", "preflight", "--kubeconfig", "kc", "--context", "no-server"], files=files)
case("preflight no config", ["cluster", "preflight"])
case("preflight invalid install path", ["cluster", "preflight", "--install-path", "helm"])
case("preflight extra args", ["cluster", "preflight", "foo"])

# install
inst_fixtures = [
    fx("GET", "/aws/region", {"results": [{"name": "eu-west-3", "country_code": "FR", "country": "France", "city": "Paris"}, {"name": "us-east-1", "country_code": "US", "country": "USA", "city": "Virginia"}]}),
    fx("GET", "/organization/org-1/aws/credentials", {"results": [{"id": "cred-1", "name": "my-creds", "access_key_id": "x"}]}),
    fx("POST", "/organization/org-1/cluster", {"id": "new-c", "created_at": "2024-07-01T10:00:00Z", "organization": {"id": "org-1"}, "name": "My-Cluster", "region": "eu-west-3", "cloud_provider": "AWS", "kubernetes": "SELF_MANAGED"}, 201),
    fx("GET", "/organization/org-1/cluster/new-c/installationHelmValues", body_raw=INSTALL_VALUES),
]
case("install eof", ["cluster", "install"])
case("install aws flow (piped)", ["cluster", "install", "--base-values-file", "base.yaml"], stdin="\n" * 7, files={"base.yaml": BASE_VALUES}, fixtures=inst_fixtures)
# Go trims "~/" then "~"+separator: ~/~/x is $HOME/x
case("install base values ~/~/ prefix", ["cluster", "install", "--base-values-file", "~/~/nope.yaml"])
case("install missing base values file", ["cluster", "install", "--base-values-file", "nope.yaml"])
case("install base values dir", ["cluster", "install", "--base-values-file", "~/.qovery"])
case("install no token", ["cluster", "install"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})

json.dump({"cases": cases}, open(OUT, "w"), indent=1)
print(len(cases), "cases")
