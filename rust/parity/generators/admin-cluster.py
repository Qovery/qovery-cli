#!/usr/bin/env python3
"""Generates parity/scenarios/admin-cluster.json.

Usage: python3 parity/generators/admin-cluster.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "admin-cluster.json")

def fx(method, path, status=200, body=None, **kw):
    r = {"method": method, "path": path, "status": status}
    if body is not None:
        r["body"] = body
    r.update(kw)
    return r

def details(cid, name, org, orgname, ctype, prod, status, **kw):
    d = {"organization_id": org, "organization_name": orgname, "organization_plan": "TEAM", "cluster_id": cid,
         "cluster_name": name, "cluster_type": ctype, "cluster_created_at": "2024-01-01T00:00:00Z",
         "cluster_last_deployed_at": "2024-02-01T00:00:00Z", "cluster_k8s_version": "1.30", "mode": "MANAGED",
         "is_production": prod, "current_status": status, "has_karpenter": False, "has_pending_update": False,
         "has_metrics_feature": True}
    d.update(kw)
    return d

ELIGIBLE = {"results": [
    details("c-1", "alpha", "org-1", "my-org", "AWS", True, "DEPLOYED"),
    details("c-2", "beta", "org-1", "my-org", "GCP", False, "DEPLOYED", has_karpenter=True),
    details("c-3", "gamma", "org-2", "other-org", "AWS", False, "STOPPED", cluster_k8s_version="1.29"),
]}

def cstatus(cid, status):
    return {"cluster_id": cid, "status": status, "is_deployed": True, "reason": "UNSPECIFIED"}

def op(org, cid, name, runtime, status, connected=True, attached=True, kind="SELF_MANAGED", **kw):
    d = {"organization_id": org, "cluster_id": cid, "cluster_name": name, "cluster_kind": kind, "attached": attached,
         "connected": connected, "reported_runtime": runtime, "status": status}
    d.update(kw)
    return d

FLEET = {"results": [
    op("org-1", "op-1", "k8s-current", "KUBERNETES", "CURRENT", last_heartbeat="2026-08-18T12:28:46.120Z",
       reported_image_version="v1", desired_image_version="v1", reported_chart_version="0.1.0", desired_chart_version="0.1.0"),
    op("org-1", "op-2", "docker-old", "DOCKER", "OUTDATED_IMAGE", kind="EKS_ANYWHERE", last_heartbeat="2026-08-18T14:28:46+02:00",
       reported_image_version="v1", desired_image_version="v2", desired_chart_version=None),
    op("org-2", "op-3", "k8s-old-chart", "KUBERNETES", "OUTDATED_CHART", reported_image_version="v2",
       desired_image_version="v2", reported_chart_version="0.1.0", desired_chart_version="0.2.0", extra_field={"a": 1}),
    op("org-1", "op-4", "detached", "UNKNOWN", "NOT_ATTACHED", connected=False, attached=False),
    op("org-3", "op-5", "lost", "UNKNOWN", "DISCONNECTED", connected=False, last_heartbeat=None),
]}
FLEET_UPDATED = {"results": [op("org-1", "op-2", "docker-old", "DOCKER", "CURRENT", kind="EKS_ANYWHERE",
                                 reported_image_version="v2", desired_image_version="v2")]}

def history(cid, exec_id, status):
    return {"page": 1, "page_size": 10, "results": [{
        "identifier": {"deployment_id": "d-1", "cluster_id": cid, "execution_id": exec_id},
        "auditing_data": {"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
        "status": status, "action_status": "SUCCESS", "trigger_action": "DEPLOY", "operation": "", "is_dry_run": False, "reason": ""}]}

def full_cluster(**kw):
    c = {"id": "cluster-1", "created_at": "2024-05-01T10:00:00.123456Z", "organization": {"id": "org-1"}, "name": "my-cluster",
         "region": "us-east-2", "cloud_provider": "AWS", "kubernetes": "MANAGED", "max_running_nodes": 10,
         "metrics_parameters": {"enabled": True, "configuration": {"kind": "MANAGED_BY_QOVERY", "resource_profile": "LOW", "high_availability": True}},
         "keda": {"enabled": True, "resource_profile": "NORMAL", "availability_profile": "NORMAL"}}
    c.update(kw)
    return c

def clusters(*cs):
    return {"results": list(cs)}

PROFILE_CLUSTER = full_cluster()
SAVED = full_cluster(metrics_parameters={"enabled": True, "configuration": {"kind": "MANAGED_BY_QOVERY", "resource_profile": "NORMAL", "high_availability": True}})
NO_KEDA = full_cluster(keda={"enabled": False})

NOPATH = {"PATH": "/nonexistent"}
cases = []
def case(name, args, **kw):
    c = {"name": name, "args": args}
    c.update(kw)
    cases.append(c)

A = ["admin"]
C = A + ["cluster"]
# --- cluster group
case("cluster group", C)
case("cluster group with args", C + ["foo"])
case("cluster list", C + ["list"])
case("cluster list filter type", C + ["list", "-f", "ClusterType=AWS"])
case("cluster list filters and", C + ["list", "-f", "IsProduction=false", "-f", "CurrentStatus=DEPLOYED"])
case("cluster list filter bool karpenter", C + ["list", "--filters", "HasKarpenter=true"])
case("cluster list filter OR (comma values)", C + ["list", "-f", "ClusterType=AWS,GCP"])
case("cluster list filter no match", C + ["list", "-f", "ClusterName=nope"])
case("cluster list unknown filter (random key order in Go)", C + ["list", "-f", "Foo=bar"], ignore=["stdout"])
case("cluster list no ADMIN_URL", C + ["list"], env={"ADMIN_URL": ""})
case("cluster list api error", C + ["list"], fixtures=[fx("GET", "/listClustersEligibleToUpdate", 500, {"message": "boom"})])
case("cluster list no token", C + ["list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
case("cluster list connection refused", C + ["list"], env={"ADMIN_URL": "http://127.0.0.1:1"})
case("cluster list bad filter format", C + ["list", "-f", "nope"])
# deploy
ST1 = [fx("GET", "/organization/org-1/cluster/c-1/status", body=cstatus("c-1", "DEPLOYED"))]
case("cluster deploy refused", C + ["deploy", "-f", "ClusterId=c-1"], stdin="no\n", fixtures=ST1)
case("cluster deploy prompt eof", C + ["deploy", "-f", "ClusterId=c-1"], fixtures=ST1)
case("cluster deploy dry run", C + ["deploy", "-c", "-r", "0", "-f", "ClusterId=c-1"],
     fixtures=ST1 + [fx("POST", "/cluster/deploy/c-1", 200, {})])
case("cluster deploy confirmed", C + ["deploy", "-r", "0", "-y", "-f", "ClusterId=c-1"], stdin="yes\n",
     fixtures=[fx("GET", "/organization/org-1/cluster/c-1/status", responses=[{"status": 200, "body": cstatus("c-1", "DEPLOYED")}, {"status": 200, "body": cstatus("c-1", "DEPLOYMENT_ERROR")}]),
               fx("POST", "/cluster/deploy/c-1", 200, {})])
case("cluster deploy upgrade", C + ["deploy", "-c", "-r", "0", "-y", "-k", "1.31", "-f", "ClusterId=c-1"],
     fixtures=ST1 + [fx("POST", "/cluster/c-1/upgrade", 200, cstatus("c-1", "DEPLOYING"))])
case("cluster deploy upgrade dry run", C + ["deploy", "-c", "-r", "0", "-k", "1.31", "-f", "ClusterId=c-1"], fixtures=ST1)
case("cluster deploy pending cluster", C + ["deploy", "-c", "-r", "0", "-f", "ClusterId=c-1"],
     fixtures=[fx("GET", "/organization/org-1/cluster/c-1/status", body=cstatus("c-1", "DEPLOYING"))])
case("cluster deploy request error", C + ["deploy", "-c", "-r", "0", "-y", "-f", "ClusterId=c-1"],
     fixtures=ST1 + [fx("POST", "/cluster/deploy/c-1", 500, {"message": "boom"})])
case("cluster deploy status error", C + ["deploy", "-c", "-r", "0", "-f", "ClusterId=c-1"],
     fixtures=[fx("GET", "/organization/org-1/cluster/c-1/status", 404, {"message": "nope"})])
case("cluster deploy status missing property", C + ["deploy", "-c", "-r", "0", "-f", "ClusterId=c-1"],
     fixtures=[fx("GET", "/organization/org-1/cluster/c-1/status", 200, {"cluster_id": "c-1", "status": "DEPLOYED"})])
case("cluster deploy status 401 env token", C + ["deploy", "-c", "-r", "0", "-f", "ClusterId=c-1"],
     fixtures=[fx("GET", "/organization/org-1/cluster/c-1/status", 401, {})])
case("cluster deploy two batches", C + ["deploy", "-r", "0", "-n", "1", "-f", "OrganizationId=org-1"], stdin="yes\nno\n",
     fixtures=ST1 + [fx("GET", "/organization/org-1/cluster/c-2/status", body=cstatus("c-2", "DEPLOYED")), fx("POST", "/cluster/deploy/c-1", 200, {})])
case("cluster deploy on the fly", C + ["deploy", "-c", "-r", "0", "-n", "0", "-e", "on-the-fly", "-f", "OrganizationId=org-1"],
     fixtures=ST1 + [fx("GET", "/organization/org-1/cluster/c-2/status", body=cstatus("c-2", "STOPPED")), fx("POST", "/cluster/deploy/c-1", 200, {}), fx("POST", "/cluster/deploy/c-2", 200, {})])
case("cluster deploy too many parallel runs", C + ["deploy", "-n", "25", "-f", "ClusterId=c-1"])
case("cluster deploy no cluster", C + ["deploy", "-c", "-r", "0", "-f", "ClusterId=none"])
case("cluster deploy unknown filter", C + ["deploy", "-f", "Nope=x"], ignore=["stdout"])
case("cluster deploy no token", C + ["deploy"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
# operator
O = C + ["operator"]
case("operator group", O)
case("operator group args", O + ["typo"])
case("operator list", O + ["list"])
case("operator list json", O + ["list", "--json"])
case("operator list outdated", O + ["list", "--outdated"])
case("operator list filters", O + ["list", "-f", "Connected=TRUE", "-f", "Runtime=KUBERNETES"])
case("operator list filter kind", O + ["list", "-f", "ClusterKind=EKS_ANYWHERE", "--json"])
case("operator list invalid value", O + ["list", "-f", "Runtime=docker"])
case("operator list invalid connected", O + ["list", "-f", "Connected=yes"])
case("operator list unknown key", O + ["list", "-f", "CurrentStatus=DEPLOYED"])
case("operator list api error", O + ["list"], fixtures=[fx("GET", "/operator/clusters", 503, body_raw="maintenance  \n")])
case("operator list missing results", O + ["list"], fixtures=[fx("GET", "/operator/clusters", 200, {})])
case("operator list invalid enum", O + ["list"], fixtures=[fx("GET", "/operator/clusters", 200, {"results": [op("o", "c", "n", "PODMAN", "CURRENT")]})])
case("operator list filter OR (comma values)", O + ["list", "-f", "Status=CURRENT,OUTDATED_CHART"])
case("operator list empty json", O + ["list", "--json", "-f", "ClusterId=none"])
case("operator list no ADMIN_URL", O + ["list"], env={"ADMIN_URL": ""})
case("operator update dry run", O + ["update"])
case("operator update explicit versions", O + ["update", "--image-version", "v3.0.0", "--chart-version", "0.3.0"])
case("operator update bad parallel", O + ["update", "-n", "0"])
case("operator update bad refresh", O + ["update", "-r", "200"])
case("operator update zero timeout", O + ["update", "--timeout", "0s"])
case("operator update negative timeout", O + ["update", "--timeout=-5m"])
case("operator update bad filter", O + ["update", "-f", "Status=OLD"])
case("operator update nothing to do", O + ["update", "-f", "ClusterId=op-1"])
case("operator update refused", O + ["update", "-y"], stdin="no\n")
case("operator update fleet 401", O + ["update"], fixtures=[fx("GET", "/operator/clusters", 401, {"message": "unauthorized"})])
case("operator update run", O + ["update", "-y", "-c", "-r", "5", "-f", "ClusterId=op-2"], timeout=40,
     fixtures=[fx("GET", "/operator/clusters", responses=[{"status": 200, "body": FLEET}, {"status": 200, "body": FLEET_UPDATED}]),
               fx("POST", "/cluster/operator/update/op-2", 202, {"execution_id": "exec-1", "image_version": "v2", "chart_version": None}),
               fx("GET", "/organization/org-1/cluster/op-2/deploymentHistoryV2", body=history("op-2", "exec-1", "DEPLOYED"))])
case("operator update run failed execution", O + ["update", "-y", "-c", "-r", "5", "-f", "ClusterId=op-2"], timeout=40,
     fixtures=[fx("POST", "/cluster/operator/update/op-2", 202, {"execution_id": "exec-1", "image_version": "v2"}),
               fx("GET", "/organization/org-1/cluster/op-2/deploymentHistoryV2", body=history("op-2", "exec-1", "DEPLOYMENT_ERROR"))])
case("operator update request rejected", O + ["update", "-y", "-c"],
     fixtures=[fx("POST", "/cluster/operator/update/op-2", 412, body_raw="operator is disconnected")])
# profile
P = C + ["profile"]
PF = [fx("GET", "/organization/org-1/cluster", body=clusters(PROFILE_CLUSTER))]
T = ["--organization-id", "org-1", "--cluster-id", "cluster-1"]
case("profile group", P)
case("profile get missing flags", P + ["get"])
case("profile get", P + ["get"] + T, fixtures=PF)
case("profile get without addons", P + ["get"] + T)
case("profile get not found", P + ["get", "--organization-id", "org-1", "--cluster-id", "nope"], fixtures=PF)
case("profile get api error", P + ["get", "--organization-id", "org-9", "--cluster-id", "x"])
case("profile set nothing", P + ["set"] + T)
case("profile set invalid", P + ["set", "--keda-availability-profile", "low"] + T)
case("profile set keda disabled", P + ["set", "--keda-resource-profile", "HIGH"] + T, fixtures=[fx("GET", "/organization/org-1/cluster", body=clusters(NO_KEDA))])
case("profile set refused", P + ["set", "--observability-resource-profile", "normal"] + T, stdin="no\n", fixtures=PF)
case("profile set deploy", P + ["set", "-c", "--deploy", "--observability-resource-profile", "normal"] + T,
     fixtures=[fx("GET", "/organization/org-1/cluster", responses=[{"status": 200, "body": clusters(PROFILE_CLUSTER)}, {"status": 200, "body": clusters(PROFILE_CLUSTER)}, {"status": 200, "body": clusters(SAVED)}]),
               fx("PUT", "/organization/org-1/cluster/cluster-1", 200, SAVED), fx("POST", "/organization/org-1/cluster/cluster-1/deploy", 202, {})])
case("profile set confirmed not saved", P + ["set", "--observability-high-availability=false"] + T, stdin="yes\n",
     fixtures=PF + [fx("PUT", "/organization/org-1/cluster/cluster-1", 200, PROFILE_CLUSTER)])
case("profile set put error", P + ["set", "-c", "--keda-availability-profile", "high"] + T,
     fixtures=PF + [fx("PUT", "/organization/org-1/cluster/cluster-1", 400, {"message": "bad"})])
# status
case("status websocket error", C + ["status", "--organization-id", "org-1", "--cluster-id", "cluster-1"])
case("status no flags", C + ["status"])
# update-domain
case("update-domain no flags", C + ["update-domain"])
case("update-domain no domain", C + ["update-domain", "--cluster-id", "c-1"])
case("update-domain", C + ["update-domain", "--cluster-id", "c-1", "--domain", "new.example.com"], fixtures=[fx("PUT", "/cluster/c-1/domain", 204)])
case("update-domain error", C + ["update-domain", "--cluster-id", "c-1", "--domain", "x y"])
case("update-domain admin url with path", C + ["update-domain", "--cluster-id", "c-1", "--domain", "d"], env={"ADMIN_URL": "$MOCK/api/"}, fixtures=[fx("PUT", "/api/cluster/c-1/domain", 200)])
# update-dns-provider
D = C + ["update-dns-provider", "--cluster-id", "c-1", "--domain", "example.com"]
case("dns missing flags", C + ["update-dns-provider"])
case("dns invalid provider", D + ["--provider", "ovh"])
case("dns cloudflare missing", D + ["--provider", "cloudflare", "--cloudflare-email", "a@b.c"])
case("dns qovery missing", D + ["--provider", "qovery"])
case("dns route53 missing", D + ["--provider", "route53", "--route53-region", "eu-west-3"])
case("dns cloudflare", D + ["--provider", "cloudflare", "--cloudflare-email", "a@b.c", "--cloudflare-token", "t", "--cloudflare-proxied"], fixtures=[fx("PUT", "/cluster/c-1/updateDnsProvider", 200)])
case("dns route53", D + ["--provider", "route53", "--route53-access-key-id", "k", "--route53-secret-access-key", "s", "--route53-region", "eu-west-3", "--route53-hosted-zone-id", "z"], fixtures=[fx("PUT", "/cluster/c-1/updateDnsProvider", 204)])
case("dns route53 no zone", D + ["--provider", "route53", "--route53-access-key-id", "k", "--route53-secret-access-key", "s", "--route53-region", "eu-west-3"], fixtures=[fx("PUT", "/cluster/c-1/updateDnsProvider", 200)])
case("dns qovery error", D + ["--provider", "qovery", "--qovery-api-url", "https://dns"], fixtures=[fx("PUT", "/cluster/c-1/updateDnsProvider", 400, body_raw="bad domain")])
# kubeconfig
K = C + ["kubeconfig", "--organization-id", "org-1", "--cluster-id"]
SM = [fx("GET", "/organization/org-1/cluster", body=clusters(full_cluster(kubernetes="SELF_MANAGED")))]
case("kubeconfig not found", K + ["nope"])
case("kubeconfig managed cluster", K + ["cluster-1"])
case("kubeconfig self managed", K + ["cluster-1", "--kubeconfig", "apiVersion: v1"], fixtures=SM + [fx("PUT", "/organization/org-1/cluster/cluster-1/kubeconfig", 200)])
case("kubeconfig empty", K + ["cluster-1"], fixtures=SM)
case("kubeconfig api error", K + ["cluster-1", "--kubeconfig", "x"], fixtures=SM)
case("kubeconfig list error", C + ["kubeconfig"])
# delete-cluster-undeployed-in-error
case("delete undeployed eof", A + ["delete-cluster-undeployed-in-error"])
case("delete undeployed refused", A + ["delete-cluster-undeployed-in-error"], stdin="no\n")
case("delete undeployed", A + ["delete-cluster-undeployed-in-error"], stdin="yes\n", fixtures=[fx("POST", "/cluster/deleteNotDeployedInErrorClusters", 200, ["c-1", "c-2"])])
case("delete undeployed error", A + ["delete-cluster-undeployed-in-error"], stdin="yes\n")
case("delete undeployed empty line then yes", A + ["delete-cluster-undeployed-in-error"], stdin="\nyes\n", fixtures=[fx("POST", "/cluster/deleteNotDeployedInErrorClusters", 200, [])])
case("delete undeployed no ADMIN_URL", A + ["delete-cluster-undeployed-in-error"], stdin="yes\n", env={"ADMIN_URL": ""})
# force-delete-cluster
case("force-delete-cluster missing flag", A + ["force-delete-cluster"])
case("force-delete-cluster dry run", A + ["force-delete-cluster", "-c", "c-1"], stdin="yes\n")
case("force-delete-cluster", A + ["force-delete-cluster", "-c", "c-1", "-y"], stdin="yes\n", fixtures=[fx("DELETE", "/cluster/c-1", 200)])
case("force-delete-cluster error", A + ["force-delete-cluster", "-c", "c-1", "-y"], stdin="yes\n")
case("force-delete-cluster no token", A + ["force-delete-cluster", "-c", "c-1"], stdin="yes\n", env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
# force-delete-old-invalid-credentials-clusters
F = A + ["force-delete-old-invalid-credentials-clusters"]
case("force-delete-old dry run", F, stdin="yes\n", fixtures=[fx("POST", "/cluster/deleteOldClustersWithInvalidCredentials", 200, ["c-9"])])
case("force-delete-old", F + ["-y", "-d", "10"], stdin="yes\n", fixtures=[fx("POST", "/cluster/deleteOldClustersWithInvalidCredentials", 200, ["c-9"])])
case("force-delete-old error", F + ["-y"], stdin="yes\n")
case("force-delete-old refused", F, stdin="nope\n")
# locked
LOCKS = {"results": [
    {"cluster_id": "c-1", "owner_sub": "s", "owner_name": "Alice", "reason": "migration", "locked_at": "2026-01-02T03:04:05Z", "ttl_in_days": 3},
    {"cluster_id": "cluster-long-id", "owner_name": "Bob Builder", "reason": "x", "locked_at": "2026-01-02T03:04:05.5+02:00", "ttl_in_days": None},
]}
case("locked", A + ["locked"], fixtures=[fx("GET", "/cluster/lock", body=LOCKS)])
case("locked empty", A + ["locked"], fixtures=[fx("GET", "/cluster/lock", body={"results": []})])
case("locked error", A + ["locked"])
case("locked no ADMIN_URL", A + ["locked"], env={"ADMIN_URL": ""})
# notify
case("notify cluster", A + ["notify-users-cluster-failure", "-c", "c-1"], fixtures=[fx("POST", "/cluster/notifyFailedClustersAdmins", 200, ["c-1"])])
case("notify all", A + ["notify-users-cluster-failure"], fixtures=[fx("POST", "/cluster/notifyFailedClustersAdmins", 200, [])])
case("notify error", A + ["notify-users-cluster-failure", "-c", "c-1"])
case("notify no token", A + ["notify-users-cluster-failure"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
# update
case("update missing flags", A + ["update"])
case("update dry run", A + ["update", "-c", "c-1", "-v", "1.31"], stdin="yes\n", fixtures=[fx("POST", "/cluster/update/c-1", 200, {})])
case("update", A + ["update", "-c", "c-1", "-v", "1.31", "-y"], stdin="yes\n", fixtures=[fx("POST", "/cluster/update/c-1", 200, {})])
case("update error", A + ["update", "-c", "c-1", "-v", "1.31", "-y"], stdin="yes\n")
case("update refused", A + ["update", "-c", "c-1", "-v", "1.31"], stdin="no\n")
# update-bulk
case("update-bulk missing flags", A + ["update-bulk"])
case("update-bulk too many", A + ["update-bulk", "-v", "1.31", "-k", "AWS", "-p", "21"])
case("update-bulk dry run", A + ["update-bulk", "-v", "1.31", "-k", "AWS"], stdin="yes\n", fixtures=[fx("POST", "/cluster/update", 200, ["c-1"])])
case("update-bulk", A + ["update-bulk", "-v", "1.31", "-k", "GCP", "-p", "4", "-y"], stdin="yes\n", fixtures=[fx("POST", "/cluster/update", 202, ["c-1"])])
case("update-bulk error", A + ["update-bulk", "-v", "1.31", "-k", "AWS"], stdin="yes\n")
# Go builds the body with fmt.Sprintf: a quote in a flag value yields the same invalid JSON
case("update-bulk quoted values", A + ["update-bulk", "-v", '1.31"x', "-k", 'A\\WS'], stdin="yes\n", fixtures=[fx("POST", "/cluster/update", 200, ["c-1"])])
case("update-bulk connection refused", A + ["update-bulk", "-v", "1.31", "-k", "AWS"], stdin="yes\n", env={"ADMIN_URL": "http://127.0.0.1:1"})
# k9s
KC = [fx("GET", "/organization/00000000-0000-0000-000000000000/cluster/k9s-test/kubeconfig", 200, body_raw="apiVersion: v1\n")]
case("k9s no bastion addr", A + ["k9s", "k9s-test"])
# Go's checkEnv requires BASTION_ADDR even with --no-bastion
case("k9s no-bastion no bastion addr", A + ["k9s", "-n", "k9s-test"], fixtures=KC)
case("k9s no args", A + ["k9s"], env={"BASTION_ADDR": "127.0.0.1"})
case("k9s missing binary", A + ["k9s", "-n", "k9s-test"], env={"BASTION_ADDR": "127.0.0.1", **NOPATH}, fixtures=KC)
case("k9s read-write missing binary", A + ["k9s", "-n", "-w", "k9s-test"], env={"BASTION_ADDR": "127.0.0.1", **NOPATH}, fixtures=KC)
case("k9s ssh missing", A + ["k9s", "k9s-test"], env={"BASTION_ADDR": "127.0.0.1", **NOPATH}, fixtures=KC)
case("k9s kubeconfig error", A + ["k9s", "-n", "k9s-test"], env={"BASTION_ADDR": "127.0.0.1", **NOPATH})
# load-aws-credentials
case("load-aws missing org", A + ["load-aws-credentials"])
case("load-aws", A + ["load-aws-credentials", "-r", "arn:aws:iam::1:role/x", "-o", "org-1", "-c", "c-1"], env={"SHELL": "/bin/true"},
     fixtures=[fx("POST", "/aws/credentials/assume-role", 200, {"access_key_id": "a", "secret_access_key": "s", "session_token": "t"})])
case("load-aws shell fails", A + ["load-aws-credentials", "-o", "org-1"], env={"SHELL": "/bin/false"},
     fixtures=[fx("POST", "/aws/credentials/assume-role", 200, {"access_key_id": "a"})])
case("load-aws shell missing", A + ["load-aws-credentials", "-o", "org-1"], env={"SHELL": "/nonexistent/sh"},
     fixtures=[fx("POST", "/aws/credentials/assume-role", 200, {"access_key_id": "a"})])
case("load-aws error", A + ["load-aws-credentials", "-o", "org-1"], fixtures=[fx("POST", "/aws/credentials/assume-role", 403, body_raw="denied")])
# load-credentials
LC = [fx("GET", "/organization/00000000-0000-0000-000000000000/cluster/lc-test/kubeconfig", 200, body_raw="users:\n- exec:\n    command: qovery\n")]
case("load-credentials", A + ["load-credentials", "-n", "-c", "lc-test"], env={"SHELL": "/bin/true", **NOPATH},
     fixtures=LC + [fx("GET", "/cluster/lc-test/credential", 200, {"access_key_id": "a"})])
case("load-credentials gcp", A + ["load-credentials", "-n", "-c", "lc-test"], env={"SHELL": "/bin/true", **NOPATH},
     fixtures=LC + [fx("GET", "/cluster/lc-test/credential", 200, {"gcp_project_id": "p"})])
case("load-credentials empty", A + ["load-credentials", "-n", "-c", "lc-test"], fixtures=[fx("GET", "/cluster/lc-test/credential", 200, {})])
case("load-credentials error", A + ["load-credentials", "-n", "-c", "lc-test"])
case("load-credentials no bastion addr", A + ["load-credentials", "-c", "lc-test"])
case("load-credentials ssh missing", A + ["load-credentials", "-c", "lc-test"], env={"BASTION_ADDR": "127.0.0.1", **NOPATH},
     fixtures=[fx("GET", "/cluster/lc-test/credential", 200, {})])

scenario = {
    "env": {"ADMIN_URL": "$MOCK"},
    "fixtures": [
        fx("GET", "/listClustersEligibleToUpdate", body=ELIGIBLE),
        fx("GET", "/operator/clusters", body=FLEET),
    ],
    "cases": cases,
}
json.dump(scenario, open(OUT, "w"), indent=1)
print(len(cases), "cases")
