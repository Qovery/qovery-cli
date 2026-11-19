#!/usr/bin/env python3
"""Generates parity/scenarios/list-pods.json, list-pods_ws.json and port-forward.json.

Usage: python3 parity/generators/list-pods.py   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
DEF = json.load(open(os.path.join(PARITY, "fixtures", "default.json")))
def fx(path):
    return [r for r in DEF if r["path"] == path][0]
def err(method, path, status=500):
    return {"method": method, "path": path, "status": status, "body": {"status": status, "error": "boom"}}
URL = "https://console.qovery.com/organization/org-1/project/proj-1/environment/env-1/%s/%s"
PROMPT = {}  # promptui prompts are byte-identical on a non-tty stdin: compared
TF = fx("/terraform/tf-1")
TF_BP = dict(TF, body=dict(TF["body"], blueprint_id="bp-1"))

def common(cmd, extra=()):
    cases = []
    def c(name, args, **kw):
        d = {"name": "%s %s" % (cmd, name), "args": [cmd] + list(extra) + args}; d.update(kw); cases.append(d)
    c("url too short", ["https://console.qovery.com/organization/org-1"])
    c("url unknown org", [URL.replace("org-1", "org-x") % ("application", "app-1")])
    c("url unknown project", [URL.replace("proj-1", "proj-x") % ("application", "app-1")])
    c("url unknown env", [URL.replace("env-1", "env-x") % ("application", "app-1")])
    c("url statuses error", [URL % ("application", "app-1")], fixtures=[err("GET", "/environment/env-1/statuses", 502)])
    c("url service not found", [URL % ("application", "nope")])
    c("url app error", [URL % ("application", "app-1")], fixtures=[err("GET", "/application/app-1", 404)])
    c("url terraform error", [URL % ("terraform", "tf-1")], fixtures=[err("GET", "/terraform/tf-1", 404)])
    c("url job error", [URL % ("job", "life-1")], fixtures=[err("GET", "/job/life-1", 401)])
    c("no context file", [], no_context_file=True, **PROMPT)
    c("full context prompt", [])
    c("context env error", [], stdin="yes\n", fixtures=[err("GET", "/environment/env-1", 404)])
    c("context terraform error", [], stdin="yes\n", context={"service_id": "tf-x", "service_type": "TERRAFORM"})
    c("context no", [], stdin="no\n", **PROMPT)
    c("partial context select", [], context={"service_name": ""}, **PROMPT)
    c("select orgs error", [], context={"service_name": ""}, fixtures=[err("GET", "/organization", 500)])
    c("unknown flag", ["--nope"])
    return cases

def ws_common(cmd, extra=()):
    cases = []
    def c(name, args, **kw):
        d = {"name": "%s ws %s" % (cmd, name), "args": [cmd] + list(extra) + args, "timeout": 20}; d.update(kw); cases.append(d)
    c("url app", [URL % ("application", "app-1")])
    c("url new console container", [(URL % ("container", "cont-1")).replace("https://console", "https://new.console")])
    c("url db", [URL % ("database", "db-1")])
    c("url job", [URL % ("job", "cron-1")])
    c("url helm", [URL % ("helm", "helm-1")])
    c("url terraform blueprint", [URL % ("terraform", "tf-1")], fixtures=[TF_BP])
    c("context terraform blueprint", [], stdin="yes\n", context={"service_id": "tf-1", "service_type": "terraform"}, fixtures=[TF_BP])
    c("context terraform upper", [], stdin="yes\n", context={"service_id": "tf-1", "service_type": "TERRAFORM"})
    c("context token", [URL % ("application", "app-1")], env={"QOVERY_CLI_ACCESS_TOKEN": ""})
    c("connection refused", [URL % ("application", "app-1")], env={"QOVERY_WS_URL": "ws://127.0.0.1:1"})
    c("malformed ws url", [URL % ("application", "app-1")], env={"QOVERY_WS_URL": "wss://user@127.0.0.1:1"})
    return cases

def pods_ws():
    cases = ws_common("list-pods")
    for key in ["pods", "pods-list", "pods-empty", "pods-null", "pods-case", "pods-binary", "pods-badjson", "pods-truncated",
                "pods-badport", "pods-badtype", "pods-badport-case", "pods-floatport", "pods-stringport", "pods-ports-object", "pods-string", "pods-pod-string", "pods-name-array", "pods-array", "pods-two-errors", "pods-dup", "pods-null-items", "pods-close", "pods-drop", "pf-reject"]:
        cases.append({"name": "list-pods ws context " + key, "args": ["list-pods"], "stdin": "yes\n", "context": {"service_id": key}, "timeout": 20})
    return cases

json.dump({"cases": common("list-pods")}, open(os.path.join(PARITY, "scenarios", "list-pods.json"), "w"), indent=1)
json.dump({"_comment": "Run with parity/shell_ws_compare.py parity/scenarios/list-pods_ws.json (starts parity/shell_ws_mock.py and substitutes $WS).",
           "env": {"QOVERY_WS_URL": "$WS"}, "cases": pods_ws()}, open(os.path.join(PARITY, "scenarios", "list-pods_ws.json"), "w"), indent=1)

pf = common("port-forward", ["-p", "8080:80"])
def c(name, args, **kw):
    d = {"name": "port-forward " + name, "args": ["port-forward"] + args}; d.update(kw); pf.append(d)
c("missing port", [])
c("missing port with url", [URL % ("application", "app-1")])
c("empty port", ["-p", ""])
c("empty port list", ["--port="])
c("invalid local port", ["-p", "80a80:80", URL % ("application", "app-1")])
c("invalid remote port", ["-p", "8080:http", URL % ("application", "app-1")])
c("out of range port", ["-p", "70000", URL % ("application", "app-1")])
c("empty local port", ["-p", ":80", URL % ("application", "app-1")])
c("invalid port before resolution error", ["-p", "x", URL % ("application", "nope")])
c("pod flag unknown service", ["-p", "1", "--pod", "p", URL % ("application", "nope")])
json.dump({"cases": pf}, open(os.path.join(PARITY, "scenarios", "port-forward.json"), "w"), indent=1)
