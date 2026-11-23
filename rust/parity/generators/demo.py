#!/usr/bin/env python3
"""Generates parity/scenarios/demo.json (fake docker/k3d/helm/kubectl come from parity/fake-tools/*: `$PARITY` in the
PATH is replaced by compare.py with the absolute path of parity/, as Go refuses executables found through a relative
PATH entry).

Usage: python3 parity/generators/demo.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
ROOT = RUST
BIN = "$PARITY/fake-tools/all"
BIN_NO_HELM = "$PARITY/fake-tools/no-helm"
BIN_DOCKER_ONLY = "$PARITY/fake-tools/docker-only"
BIN_HELM_KUBECTL = "$PARITY/fake-tools/helm-kubectl"  # no docker nor k3d: an existing kube context needs neither
BIN_HELM_ONLY = "$PARITY/fake-tools/helm-only"
J = lambda v: json.dumps(v, separators=(",", ":"))

def cluster(id, name, is_demo=None):
    c = {"id": id, "created_at": "2024-05-01T10:00:00Z", "organization": {"id": "org-1"}, "name": name,
         "region": "on-premise", "cloud_provider": "ON_PREMISE"}
    if is_demo is not None:
        c["is_demo"] = is_demo
    return c

CLUSTERS = {"method": "GET", "path": "/organization/org-1/cluster", "status": 200, "body": {"results": [
    cluster("cluster-1", "my-cluster"), cluster("demo-1", "my-demo", True), cluster("prod-1", "production", False)]}}
DELETE_OK = {"method": "DELETE", "path": "/organization/org-1/cluster/demo-1", "status": 204}

# local state of a demo cluster "my-demo" with its registry
REG = J([{"name": "k3d-qovery-registry.lan"}])
LOCAL_EXISTING = {
    "FAKE_K3D_CLUSTER_LIST": J([{"name": "my-demo"}, {"name": "other"}]),
    "FAKE_K3D_REGISTRY_LIST": REG,
    "FAKE_DOCKER_INSPECT": J([{"NetworkSettings": {"Networks": {"k3d-my-demo": {"DNSNames": ["qovery-registry.lan"]}}}}]),
}

def env(*dicts, path=BIN):
    e = {"PATH": path}
    for d in dicts:
        e.update(d)
    return e

cases = []
def case(name, args, **kw):
    c = {"name": name, "args": args}
    c.update(kw)
    cases.append(c)

# --- group / help / args ---
case("demo (group)", ["demo"])
case("demo unknown sub-command", ["demo", "foo"])
case("demo --help", ["demo", "--help"])
case("demo up --help", ["demo", "up", "--help"])
case("demo destroy -h", ["demo", "destroy", "-h"])
case("demo up extra args", ["demo", "up", "foo"])
case("demo destroy extra args", ["demo", "destroy", "foo"])
case("demo destroy unknown flag", ["demo", "destroy", "--nope"])

# --- destroy ---
case("destroy invalid cluster name", ["demo", "destroy", "-c", "1-bad"], env=env())
case("destroy invalid cluster name (one letter)", ["demo", "destroy", "--cluster-name=a"], env=env())
case("destroy without docker", ["demo", "destroy", "-c", "my-demo"], env=env(path="/nonexistent"))
case("destroy without k3d", ["demo", "destroy", "-c", "my-demo"], env=env(path=BIN_DOCKER_ONLY))
case("destroy local cluster and registry", ["demo", "destroy", "-c", "my-demo"], env=env(LOCAL_EXISTING))
case("destroy default cluster name", ["demo", "destroy"], env=env({"FAKE_K3D_CLUSTER_LIST": "[]", "FAKE_K3D_REGISTRY_LIST": "[]"}))
case("destroy missing local cluster, no registry", ["demo", "destroy", "-c", "my-demo"],
     env=env({"FAKE_K3D_CLUSTER_LIST": "null", "FAKE_K3D_REGISTRY_LIST": "[]"}))
case("destroy keeps a shared registry", ["demo", "destroy", "-c", "my-demo"],
     env=env(LOCAL_EXISTING, {"FAKE_DOCKER_INSPECT": J([{"NetworkSettings": {"Networks": {"k3d-my-demo": {}, "k3d-other": {}}}}])}))
case("destroy k3d cluster list fails", ["demo", "destroy", "-c", "my-demo"],
     env=env({"FAKE_K3D_CLUSTER_LIST": "boom: cannot list", "FAKE_K3D_CLUSTER_LIST_EXIT": "2"}))
case("destroy invalid k3d cluster list", ["demo", "destroy", "-c", "my-demo"], env=env({"FAKE_K3D_CLUSTER_LIST": "not json"}))
case("destroy k3d registry list fails", ["demo", "destroy", "-c", "my-demo"],
     env=env({"FAKE_K3D_CLUSTER_LIST": "[]", "FAKE_K3D_REGISTRY_LIST": "registry boom", "FAKE_K3D_REGISTRY_LIST_EXIT": "1"}))
case("destroy invalid registry list", ["demo", "destroy", "-c", "my-demo"],
     env=env({"FAKE_K3D_CLUSTER_LIST": "[]", "FAKE_K3D_REGISTRY_LIST": "{}"}))
case("destroy docker inspect fails", ["demo", "destroy", "-c", "my-demo"],
     env=env(LOCAL_EXISTING, {"FAKE_DOCKER_INSPECT": "Error: no such object", "FAKE_DOCKER_INSPECT_EXIT": "1"}))
case("destroy invalid docker inspect", ["demo", "destroy", "-c", "my-demo"], env=env(LOCAL_EXISTING, {"FAKE_DOCKER_INSPECT": "[]"}))
case("destroy k3d cluster delete fails", ["demo", "destroy", "-c", "my-demo"],
     env=env(LOCAL_EXISTING, {"FAKE_K3D_CLUSTER_DELETE": "deleting...\nfailed", "FAKE_K3D_CLUSTER_DELETE_EXIT": "5"}))
case("destroy k3d registry delete fails", ["demo", "destroy", "-c", "my-demo"],
     env=env(LOCAL_EXISTING, {"FAKE_K3D_REGISTRY_DELETE": "nope", "FAKE_K3D_REGISTRY_DELETE_EXIT": "3"}))
case("destroy -d deletes the Qovery cluster", ["demo", "destroy", "-c", "my-demo", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[CLUSTERS, DELETE_OK])
case("destroy --delete-qovery-config cluster missing in Qovery", ["demo", "destroy", "-c", "other", "--delete-qovery-config"],
     env=env(LOCAL_EXISTING), fixtures=[CLUSTERS])
case("destroy -d refuses a cluster that is not a demo", ["demo", "destroy", "-c", "production", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[CLUSTERS])
case("destroy -d refuses an unflagged cluster", ["demo", "destroy", "-c", "my-cluster", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[CLUSTERS])
case("destroy -d lookup fails", ["demo", "destroy", "-c", "my-demo", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[{"method": "GET", "path": "/organization/org-1/cluster", "status": 503, "body": {"message": "down"}}])
case("destroy -d lookup unauthorized is retried once", ["demo", "destroy", "-c", "my-demo", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[{"method": "GET", "path": "/organization/org-1/cluster", "status": 401, "body": {"message": "expired"}}])
case("destroy -d lookup unauthorized then ok", ["demo", "destroy", "-c", "my-demo", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[{"method": "GET", "path": "/organization/org-1/cluster", "responses": [
         {"status": 401, "body": {"message": "expired"}}, CLUSTERS]}, DELETE_OK])
case("destroy -d delete fails", ["demo", "destroy", "-c", "my-demo", "-d"], env=env(LOCAL_EXISTING),
     fixtures=[CLUSTERS, {"method": "DELETE", "path": "/organization/org-1/cluster/demo-1", "status": 400, "body": {"message": "bad"}}])
case("destroy -d without token", ["demo", "destroy", "-c", "my-demo", "-d"],
     env=env(LOCAL_EXISTING, {"QOVERY_CLI_ACCESS_TOKEN": ""}), context={"access_token": ""})
case("destroy -d without organization (interactive prompt)", ["demo", "destroy", "-c", "my-demo", "-d"], env=env(LOCAL_EXISTING),
     context={"organization_id": "", "organization_name": ""})

# --- up ---
UPLOAD_OK = {"method": "POST", "path": "/admin/demoDebugLog", "status": 200}
CREDS = {"method": "GET", "path": "/organization/org-1/onPremise/credentials", "status": 200,
         "body": {"results": [{"id": "cred-1", "name": "on-premise", "object_type": "OTHER"}]}}
CREDS_EMPTY = {"method": "GET", "path": "/organization/org-1/onPremise/credentials", "status": 200, "body": {"results": []}}
CREDS_CREATE = {"method": "POST", "path": "/organization/org-1/onPremise/credentials", "status": 201,
                "body": {"id": "cred-2", "name": "on-premise", "object_type": "OTHER"}}
CREATE_CLUSTER = {"method": "POST", "path": "/organization/org-1/cluster", "status": 201, "body": cluster("demo-2", "new-demo", True)}
def layer(key, mandatory, enabled):
    return {"key": key, "mandatory": mandatory, "enabledByDefault": enabled, "modes": ["CUSTOMER_MANAGED"], "componentKeys": [], "components": []}
CATALOG = {"method": "GET", "path": "/organization/org-1/platformTemplate", "status": 200, "body": {"results": [
    {"key": "qovery-cluster-v0", "version": "0.1.0", "status": "PUBLISHED", "layers": []},
    {"key": "qovery-demo-v0", "version": "0.2.0", "status": "PUBLISHED",
     "layers": [layer("qovery-stack", False, True), layer("observability", False, False), layer("dns-certificates", True, True)]}]}}
def config_route(cid, status=404, platform=None, inputs=None):
    if platform is None:
        return {"method": "GET", "path": "/v1/cluster/%s/platformConfiguration" % cid, "status": status, "body": {"message": "not found"}}
    return {"method": "GET", "path": "/v1/cluster/%s/platformConfiguration" % cid, "status": 200,
            "body": {"clusterId": cid, "organizationId": "org-1", "platform": platform, "clusterInputs": inputs or {}, "layers": []}}
def save_route(cid, status=200):
    return {"method": "PUT", "path": "/v1/cluster/%s/platformConfiguration" % cid, "status": status,
            "body": {"clusterId": cid, "organizationId": "org-1", "platform": {"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0"},
                     "clusterInputs": {}, "layers": []} if status < 300 else {"type": "about:blank", "title": "Conflict", "status": status, "detail": "owned by another cluster"}}
def bootstrap(cid, chart="oci://registry.example/qovery-operator"):
    return {"method": "GET", "path": "/organization/org-1/cluster/%s/operator/bootstrap" % cid, "status": 200, "body": {
        "chart_repository": "oci://registry.example", "chart_name": "qovery-operator", "chart_version": "1.2.3",
        "chart_reference": chart, "namespace": "qovery", "release_name": "qovery-operator", "values": {},
        "values_yaml": "token: secret\n", "helm_command": "ignored helm command"}}
def attach(cid, status=204):
    return {"method": "POST", "path": "/organization/org-1/cluster/%s/operator/attach" % cid, "status": status}
def operator_status(cid, connected=True, heartbeat="2099-01-01T00:00:00Z"):
    body = {"organization_id": "org-1", "cluster_id": cid, "operator_connected": connected, "status": "CURRENT", "reported_runtime": "KUBERNETES"}
    if heartbeat:
        body["last_heartbeat"] = heartbeat
    return {"method": "GET", "path": "/organization/org-1/cluster/%s/operator/status" % cid, "status": 200, "body": body}
def cstatus(cid, s):
    return {"status": 200, "body": {"cluster_id": cid, "status": s, "is_deployed": s == "DEPLOYED", "reason": "UNSPECIFIED"}}
def status_route(cid, *statuses):
    return {"method": "GET", "path": "/organization/org-1/cluster/%s/status" % cid, "responses": [cstatus(cid, s) for s in statuses]}
def deploy_route(cid, s="DEPLOYED", status=None):
    if status:
        return {"method": "POST", "path": "/organization/org-1/cluster/%s/deploy" % cid, "status": status, "body": {"message": "no"}}
    r = cstatus(cid, s); r.update({"method": "POST", "path": "/organization/org-1/cluster/%s/deploy" % cid}); return r

def api_ok(cid="demo-1", **over):
    routes = {
        "creds": CREDS, "clusters": CLUSTERS, "create": CREATE_CLUSTER, "catalog": CATALOG, "config": config_route(cid),
        "save": save_route(cid), "bootstrap": bootstrap(cid), "attach": attach(cid), "operator": operator_status(cid),
        "status": status_route(cid, "READY"), "deploy": deploy_route(cid), "upload": UPLOAD_OK,
    }
    routes.update(over)
    return [r for r in routes.values() if r is not None]

NETWORKS_FREE = J([{"Name": "bridge", "IPAM": {"Config": [{"Subnet": "172.17.0.0/16"}]}}])
LOCAL_NEW = {
    "FAKE_DOCKER_INFO": "Server: fake",
    "FAKE_K3D_CLUSTER_LIST": "[]",
    "FAKE_DOCKER_NETWORK_LS": "bridge-id",
    "FAKE_DOCKER_NETWORK_INSPECT": NETWORKS_FREE,
    "FAKE_K3D_CLUSTER_CREATE": "INFO[0000] cluster created",
    "FAKE_K3D_REGISTRY_LIST": REG,
    "FAKE_DOCKER_INSPECT": J([{"NetworkSettings": {"Networks": {"k3d-new-demo": {"DNSNames": ["qovery-registry.lan"]}}}}]),
    "FAKE_HELM_LIST": "[]",
    "FAKE_HELM_UPGRADE": "Release \"qovery-operator\" has been upgraded.",
    "FAKE_KUBECTL": J({"items": [{"metadata": {"name": "qovery-operator"}}]}),
}
LOCAL_RERUN = dict(LOCAL_NEW, **{
    "FAKE_K3D_CLUSTER_LIST": J([{"name": "my-demo"}]),
    "FAKE_K3D_CLUSTER_START": "INFO[0000] started",
    "FAKE_DOCKER_INSPECT": J([{"NetworkSettings": {"Networks": {"k3d-my-demo": {"Aliases": ["qovery-registry.lan"]}}}}]),
})
NEW = ["demo", "up", "-c", "new-demo"]
RERUN = ["demo", "up", "-c", "my-demo"]
# The failure summary on stderr contains a random diagnostic ID; the uploaded debug log contains it too, with a timestamp.
UP_FAIL = ["stderr", "requests"]

case("up invalid cluster name", ["demo", "up", "-c", "bad_name"], env=env(), ignore=["stderr"])
case("up without token", NEW, env=env({"QOVERY_CLI_ACCESS_TOKEN": ""}), context={"access_token": ""}, ignore=["stderr"])
case("up without organization (interactive prompt)", NEW, env=env(), context={"organization_id": "", "organization_name": ""},
     ignore=["stderr"])  # random diagnostic id
case("up debug log directory cannot be created", NEW, env=env({"TMPDIR": "/proc/qovery-demo-parity"}), ignore=["stderr"])
case("up debug log directory under a file", NEW, env=env({"TMPDIR": "/etc/hostname"}), ignore=["stderr"])
case("up credentials unauthorized is retried once", NEW, env=env(LOCAL_NEW),
     fixtures=api_ok(creds={"method": "GET", "path": "/organization/org-1/onPremise/credentials", "status": 401, "body": {}}), ignore=UP_FAIL)
case("up without any dependency (default cluster name)", ["demo", "up"], env=env(path="/nonexistent"), fixtures=[UPLOAD_OK], ignore=UP_FAIL)
case("up without helm", NEW, env=env(path=BIN_NO_HELM), fixtures=[UPLOAD_OK], ignore=UP_FAIL)
case("up docker not running", NEW, env=env(LOCAL_NEW, {"FAKE_DOCKER_INFO": "Cannot connect to the Docker daemon", "FAKE_DOCKER_INFO_EXIT": "1"}),
     fixtures=[UPLOAD_OK], ignore=UP_FAIL)
case("up debug log upload fails", NEW, env=env(path="/nonexistent"),
     fixtures=[{"method": "POST", "path": "/admin/demoDebugLog", "status": 400, "body": {"message": "bad log"}}], ignore=UP_FAIL)
case("up debug log upload forbidden", NEW, env=env(path="/nonexistent"),
     fixtures=[{"method": "POST", "path": "/admin/demoDebugLog", "status": 403, "body_raw": "forbidden"}], ignore=UP_FAIL)
case("up credentials API error", NEW, env=env(LOCAL_NEW),
     fixtures=api_ok(creds={"method": "GET", "path": "/organization/org-1/onPremise/credentials", "status": 500, "body": {}}), ignore=UP_FAIL)
case("up invalid credentials", NEW, env=env(LOCAL_NEW),
     fixtures=api_ok(creds={"method": "GET", "path": "/organization/org-1/onPremise/credentials", "status": 200,
                            "body": {"results": [{"id": "cred-1", "name": "aws", "object_type": "AWS", "access_key_id": "x"}]}}), ignore=UP_FAIL)
case("up cluster name used by a cluster that is not a demo", ["demo", "up", "-c", "production"], env=env(LOCAL_NEW), fixtures=api_ok(), ignore=UP_FAIL)
case("up cluster lookup API error", NEW, env=env(LOCAL_NEW),
     fixtures=api_ok(clusters={"method": "GET", "path": "/organization/org-1/cluster", "status": 502, "body": {}}), ignore=UP_FAIL)
case("up demo subnet used by an old k3d cluster", NEW, env=env(LOCAL_NEW, {"FAKE_DOCKER_NETWORK_INSPECT": J([
    {"Name": "bridge", "IPAM": {"Config": [{"Subnet": "172.17.0.0/16"}]}},
    {"Name": "k3d-old demo", "IPAM": {"Config": [{"Subnet": "172.42.0.0/24"}]}}])}), fixtures=api_ok(), ignore=UP_FAIL)
case("up demo subnet used by another network", NEW, env=env(LOCAL_NEW, {"FAKE_DOCKER_NETWORK_INSPECT": J([
    {"Name": "vpn", "IPAM": {"Config": [{"Subnet": "172.40.0.0/13"}, {"Subnet": "fd00::/64"}]}}])}), fixtures=api_ok(), ignore=UP_FAIL)
case("up k3d cluster list fails", NEW, env=env(LOCAL_NEW, {"FAKE_K3D_CLUSTER_LIST": "k3d: boom", "FAKE_K3D_CLUSTER_LIST_EXIT": "4"}),
     fixtures=api_ok(), ignore=UP_FAIL)
case("up k3d cluster create fails", NEW, env=env(LOCAL_NEW, {"FAKE_K3D_CLUSTER_CREATE": "ERRO[0001] failed", "FAKE_K3D_CLUSTER_CREATE_EXIT": "1"}),
     fixtures=api_ok(), ignore=UP_FAIL)
case("up registry created and connected", NEW, env=env(LOCAL_NEW, {
    "FAKE_K3D_REGISTRY_LIST_1": "[]",
    "FAKE_DOCKER_INSPECT": J([{"NetworkSettings": {"Networks": {"k3d-new-demo": {"Aliases": ["other"]}}}}]),
    "FAKE_HELM_LIST": "not json"}), fixtures=api_ok(), ignore=UP_FAIL)
case("up registry never appears", NEW, env=env(LOCAL_NEW, {"FAKE_K3D_REGISTRY_LIST": "[]"}), fixtures=api_ok(), ignore=UP_FAIL)
case("up registry network connect fails", RERUN, env=env(LOCAL_RERUN, {
    "FAKE_DOCKER_INSPECT": J([{"NetworkSettings": {"Networks": {"bridge": {}}}}]),
    "FAKE_DOCKER_NETWORK_CONNECT": "Error response from daemon", "FAKE_DOCKER_NETWORK_CONNECT_EXIT": "1"}), fixtures=api_ok(), ignore=UP_FAIL)
case("up legacy qovery release", RERUN, env=env(LOCAL_RERUN, {"FAKE_HELM_LIST": J([{"name": "qovery", "namespace": "qovery"}])}),
     fixtures=api_ok(), ignore=UP_FAIL)
case("up helm list fails", RERUN, env=env(LOCAL_RERUN, {"FAKE_HELM_LIST": "Error: kubernetes cluster unreachable", "FAKE_HELM_LIST_EXIT": "1"}),
     fixtures=api_ok(), ignore=UP_FAIL)
case("up cluster creation fails", NEW, env=env(LOCAL_NEW),
     fixtures=api_ok(create={"method": "POST", "path": "/organization/org-1/cluster", "status": 400, "body": {"message": "bad"}}), ignore=UP_FAIL)
case("up no demo platform template", RERUN, env=env(LOCAL_RERUN),
     fixtures=api_ok(catalog={"method": "GET", "path": "/organization/org-1/platformTemplate", "status": 200, "body": {"results": []}}), ignore=UP_FAIL)
case("up platform configuration unreadable", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(config=config_route("demo-1", 500)), ignore=UP_FAIL)
case("up platform configuration conflict", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(save=save_route("demo-1", 409)), ignore=UP_FAIL)
case("up platform configuration redacted", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(config=config_route("demo-1", platform={
    "templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": True},
    "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64", "token": "<redacted>"}, "cert-manager": {"email": "<redacted>"}}})),
     ignore=UP_FAIL)
# Go's typed decoding refuses a non-string cluster input instead of sending it back cleared
case("up platform configuration with a non-string cluster input", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(config=config_route("demo-1", platform={
    "templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64"}}},
    inputs={"qovery-operator": {"input": "value", "replicas": 2}})), ignore=UP_FAIL)
case("up operator bootstrap fails", RERUN, env=env(LOCAL_RERUN),
     fixtures=api_ok(bootstrap={"method": "GET", "path": "/organization/org-1/cluster/demo-1/operator/bootstrap", "status": 500, "body": {}}), ignore=UP_FAIL)
case("up operator attach fails", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(attach=attach("demo-1", 403)), ignore=UP_FAIL)
case("up umbrella chart refused", RERUN, env=env(LOCAL_RERUN),
     fixtures=api_ok(bootstrap=bootstrap("demo-1", "oci://public.ecr.aws/qovery/charts/qovery/")), ignore=UP_FAIL)
# the following cases print the random name of the temporary helm values file on stdout
UP_FAIL_OUT = UP_FAIL + ["stdout"]
case("up helm install fails", RERUN, env=env(LOCAL_RERUN, {"FAKE_HELM_UPGRADE": "Error: timed out", "FAKE_HELM_UPGRADE_EXIT": "1"}),
     fixtures=api_ok(), ignore=UP_FAIL_OUT)
# The node read finds disk pressure, then none 10s later: both CLIs wait once and go on.
NODE_LINE = "k3d-my-demo-server-0\t2026-01-01T00:00:00Z\t%s\t%s"
case("up disk pressure clears", RERUN, env=env(LOCAL_RERUN, {
    "FAKE_KUBECTL_1": NODE_LINE % ("node.kubernetes.io/disk-pressure", "True"), "FAKE_KUBECTL_2": NODE_LINE % ("", "False")}),
    fixtures=api_ok(), ignore=["stdout"], timeout=60)
case("up operator status error", RERUN, env=env(LOCAL_RERUN),
     fixtures=api_ok(operator={"method": "GET", "path": "/organization/org-1/cluster/demo-1/operator/status", "status": 500, "body": {}}),
     ignore=UP_FAIL_OUT)
case("up cluster status error", RERUN, env=env(LOCAL_RERUN),
     fixtures=api_ok(status={"method": "GET", "path": "/organization/org-1/cluster/demo-1/status", "status": 500, "body": {}}),
     ignore=UP_FAIL_OUT)
case("up deploy fails", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(deploy=deploy_route("demo-1", status=400)), ignore=UP_FAIL_OUT)
case("up terminal deployment status", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(deploy=deploy_route("demo-1", "DEPLOYMENT_ERROR")),
     ignore=UP_FAIL_OUT)
case("up workloads without operator", RERUN, env=env(LOCAL_RERUN, {"FAKE_KUBECTL": J({"items": []})}), fixtures=api_ok(), ignore=UP_FAIL_OUT)
case("up workloads with a permanent engine", RERUN, env=env(LOCAL_RERUN, {"FAKE_KUBECTL": J({"items": [
    {"metadata": {"name": "qovery-operator"}}, {"metadata": {"name": "qovery-engine"}}]})}), fixtures=api_ok(), ignore=UP_FAIL_OUT)
# success paths (stdout: random values file name; stderr is empty)
case("up creates everything", NEW, env=env(LOCAL_NEW), fixtures=api_ok("demo-2", creds=CREDS_EMPTY, creds_create=CREDS_CREATE),
     ignore=["stdout"])
case("up rerun reuses the demo cluster", RERUN, env=env(LOCAL_RERUN), fixtures=api_ok(config=config_route("demo-1", platform={
    "templateKey": "qovery-demo-v0", "templateVersion": "0.2.0", "layerSelections": {"qovery-stack": False},
    "managedConfig": {"qovery-operator": {"cpuArchitectures": "ARM64", "logLevel": "debug", "replicas": 2.0}}},
    inputs={"qovery-operator": {"input": "value"}})), ignore=["stdout"])
case("up rerun with an up-to-date configuration, deployment in progress", RERUN + ["--debug"], env=env(LOCAL_RERUN), fixtures=api_ok(
    config=config_route("demo-1", platform={"templateKey": "qovery-demo-v0", "templateVersion": "0.2.0",
                                            "managedConfig": {"qovery-operator": {"cpuArchitectures": "AMD64"}}}),
    status=status_route("demo-1", "DEPLOYING", "READY", "DEPLOYED"), deploy=deploy_route("demo-1", "DEPLOYMENT_QUEUED")),
    ignore=["stdout"], timeout=60)

# --- existing kube context (`--kube-context`): only helm and kubectl are on the PATH ---
KUBE = ["--kube-context", "vcluster_qls-a", "--kubeconfig", "/kube/qls-a.yaml"]
KUBE_UP = ["demo", "up", "-c", "my-demo"] + KUBE
LOCAL_KUBE = {
    "FAKE_HELM_LIST": "[]",
    "FAKE_HELM_UPGRADE": "Release \"qovery-operator\" has been upgraded.",
    "FAKE_KUBECTL": J({"items": [{"metadata": {"name": "qovery-operator"}}]}),
}
NO_ORG = {"organization_id": "", "organization_name": ""}
case("up --kubeconfig without --kube-context", ["demo", "up", "-c", "my-demo", "--kubeconfig", "/kube/qls-a.yaml"], env=env(),
     ignore=["stderr"])
case("up --kube-context without organization never prompts", KUBE_UP, env=env(path=BIN_HELM_KUBECTL), context=NO_ORG,
     ignore=["stderr"])
case("up --kube-context without helm", KUBE_UP, env=env(path="/nonexistent"), fixtures=[UPLOAD_OK], ignore=UP_FAIL)
case("up --kube-context, debug log endpoint missing", KUBE_UP, env=env(path="/nonexistent"),
     fixtures=[{"method": "POST", "path": "/admin/demoDebugLog", "status": 404, "body": {"message": "not found"}}], ignore=UP_FAIL)
case("up --kube-context helm install fails", KUBE_UP, env=env(LOCAL_KUBE, {"FAKE_HELM_UPGRADE": "Error: timed out", "FAKE_HELM_UPGRADE_EXIT": "1"},
     path=BIN_HELM_KUBECTL), fixtures=api_ok(), ignore=UP_FAIL_OUT)
case("up --kube-context --skip-catalog", KUBE_UP + ["--skip-catalog"], env=env(LOCAL_KUBE, path=BIN_HELM_KUBECTL),
     fixtures=api_ok(status=None, deploy=None, upload=None), ignore=["stdout"])
case("up --kube-context --organization-id without organization", KUBE_UP + ["--organization-id", "org-1"],
     env=env(LOCAL_KUBE, path=BIN_HELM_KUBECTL), context=NO_ORG, fixtures=api_ok(), ignore=["stdout"])
case("up --skip-catalog on k3d", RERUN + ["--skip-catalog"], env=env(LOCAL_RERUN),
     fixtures=api_ok(status=None, deploy=None, upload=None), ignore=["stdout"])

OPERATOR_RELEASES = J([{"name": "qovery-operator", "namespace": "qovery", "chart": "qovery-operator-1.2.3"},
                       {"name": "vcluster", "namespace": "qls", "chart": "vcluster-0.20.0"}])
KUBE_DESTROY = ["demo", "destroy", "-c", "my-demo"] + KUBE
case("destroy --kubeconfig without --kube-context", ["demo", "destroy", "-c", "my-demo", "--kubeconfig", "/kube/qls-a.yaml"], env=env())
case("destroy --kube-context without helm", KUBE_DESTROY, env=env(path=BIN_DOCKER_ONLY))
case("destroy --kube-context uninstalls the Operator", KUBE_DESTROY, env=env({"FAKE_HELM_LIST": OPERATOR_RELEASES}, path=BIN_HELM_ONLY))
case("destroy --kube-context without an Operator release", KUBE_DESTROY, env=env({"FAKE_HELM_LIST": "null"}, path=BIN_HELM_ONLY))
case("destroy --kube-context helm list fails", KUBE_DESTROY,
     env=env({"FAKE_HELM_LIST": "Error: cluster unreachable", "FAKE_HELM_LIST_EXIT": "1"}, path=BIN_HELM_ONLY))
case("destroy --kube-context invalid release list", KUBE_DESTROY, env=env({"FAKE_HELM_LIST": "{}"}, path=BIN_HELM_ONLY))
case("destroy --kube-context helm uninstall fails", KUBE_DESTROY,
     env=env({"FAKE_HELM_LIST": OPERATOR_RELEASES, "FAKE_HELM_UNINSTALL": "Error: timed out", "FAKE_HELM_UNINSTALL_EXIT": "1"}, path=BIN_HELM_ONLY))
case("destroy --kube-context -d deletes the Qovery cluster", KUBE_DESTROY + ["-d"], env=env({"FAKE_HELM_LIST": OPERATOR_RELEASES}, path=BIN_HELM_ONLY),
     fixtures=[CLUSTERS, DELETE_OK])
case("destroy --kube-context -d cluster missing in Qovery", ["demo", "destroy", "-c", "other", "-d"] + KUBE,
     env=env({"FAKE_HELM_LIST": "[]"}, path=BIN_HELM_ONLY), fixtures=[CLUSTERS])
case("destroy --kube-context -d lookup fails", KUBE_DESTROY + ["-d"], env=env({"FAKE_HELM_LIST": OPERATOR_RELEASES}, path=BIN_HELM_ONLY),
     fixtures=[{"method": "GET", "path": "/organization/org-1/cluster", "status": 503, "body": {"message": "down"}}])
case("destroy --kube-context -d without organization never prompts", KUBE_DESTROY + ["-d"], env=env(path=BIN_HELM_ONLY), context=NO_ORG)
case("destroy --kube-context -d --organization-id", KUBE_DESTROY + ["-d", "--organization-id", "org-1"],
     env=env({"FAKE_HELM_LIST": OPERATOR_RELEASES}, path=BIN_HELM_ONLY), context=NO_ORG, fixtures=[CLUSTERS, DELETE_OK])

json.dump({"cases": cases}, open(sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "demo.json"), "w"), indent=1)
print(len(cases), "cases")
