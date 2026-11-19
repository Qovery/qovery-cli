#!/usr/bin/env python3
"""Generates parity/scenarios/shell.json and shell_ws.json.

Usage: python3 parity/generators/shell.py   (default output next to the scenarios, from any working directory;
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
def single(path, item):
    return {"method": "GET", "path": path, "status": 200, "body": {"results": [item]}}
ORG1 = fx("/organization")["body"]["results"][0]
SINGLE_ORG = single("/organization", ORG1)
URL = "https://console.qovery.com/organization/org-1/project/proj-1/environment/env-1/%s/%s"
FLAGS = ["shell", "--organization", "my-org", "--project", "my-project", "--environment", "my-env", "--service"]

def shell_cases():
    cases = []
    def c(name, args, **kw):
        d = {"name": name, "args": args}; d.update(kw); cases.append(d)
    c("shell org only", ["shell", "--organization", "my-org"])
    c("shell project only", ["shell", "--project", "my-project"])
    c("shell blank org", ["shell", "--organization", "  ", "--project", "p", "--environment", "e", "--service", "s"])
    c("shell no env", ["shell", "--organization", "my-org", "--project", "my-project", "--service", "my-app"])
    c("shell no service", ["shell", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
    c("shell unknown org", ["shell", "--organization", "nope", "--project", "my-project", "--environment", "my-env", "--service", "my-app"])
    c("shell unknown project", ["shell", "--organization", "my-org", "--project", "nope", "--environment", "my-env", "--service", "my-app"])
    c("shell unknown env", ["shell", "--organization", "my-org", "--project", "my-project", "--environment", "nope", "--service", "my-app"])
    c("shell unknown service", FLAGS + ["nope"])
    c("shell env get error", FLAGS + ["my-app"], fixtures=[err("GET", "/environment/env-1", 404)])
    c("shell orgs error", FLAGS + ["my-app"], fixtures=[err("GET", "/organization", 500)])
    c("shell flags no token", FLAGS + ["my-app"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)
    c("shell url too short", ["shell", "https://console.qovery.com/organization/org-1/project/proj-1"])
    c("shell url new console too short", ["shell", "https://new.console.qovery.com/organization/org-1"])
    c("shell url unknown org", ["shell", URL.replace("org-1", "org-x") % ("application", "app-1")])
    c("shell url unknown project", ["shell", URL.replace("proj-1", "proj-x") % ("application", "app-1")])
    c("shell url unknown env", ["shell", URL.replace("env-1", "env-x") % ("application", "app-1")])
    c("shell url statuses error", ["shell", URL % ("application", "app-1")], fixtures=[err("GET", "/environment/env-1/statuses", 503)])
    c("shell url terraform", ["shell", URL % ("terraform", "tf-1")])
    c("shell url app get error", ["shell", URL % ("application", "app-1")], fixtures=[err("GET", "/application/app-1", 404)])
    c("shell url job get error", ["shell", URL % ("job", "cron-1")], fixtures=[err("GET", "/job/cron-1", 500)])
    c("shell url db get error", ["shell", URL % ("database", "db-1")], fixtures=[err("GET", "/database/db-1", 403)])
    c("shell url helm get error", ["shell", URL % ("helm", "helm-1")], fixtures=[err("GET", "/helm/helm-1", 404)])
    c("shell url container get error", ["shell", URL % ("container", "cont-1")], fixtures=[err("GET", "/container/cont-1", 404)])
    c("shell no context file", ["shell"], no_context_file=True)
    c("shell full context prompt", ["shell"])
    c("shell full context prompt yes", ["shell"], stdin="yes\n")
    c("shell partial context select", ["shell"], context={"service_id": "", "service_name": ""})
    c("shell empty context select", ["shell"], context={"organization_id": "", "organization_name": "", "project_id": "", "project_name": "", "environment_id": "", "environment_name": "", "service_id": "", "service_name": "", "service_type": ""})
    c("shell select orgs error", ["shell"], context={"service_id": ""}, fixtures=[err("GET", "/organization", 500)])
    c("shell select no orgs", ["shell"], context={"service_id": ""}, fixtures=[{"method": "GET", "path": "/organization", "status": 200, "body": {"results": []}}])
    c("shell select project prompt", ["shell"], context={"service_id": ""}, fixtures=[SINGLE_ORG, {"method": "GET", "path": "/organization/org-1/project", "status": 200, "body": {"results": [fx("/organization/org-1/project")["body"]["results"][0], dict(fx("/organization/org-1/project")["body"]["results"][0], id="proj-2", name="other")]}}])
    c("shell select service prompt", ["shell"], context={"service_id": ""}, fixtures=[SINGLE_ORG])
    c("shell ephemeral bad mode", FLAGS + ["my-app", "--ephemeral", "--mode", "nope"])
    c("shell unknown flag", ["shell", "--nope"])
    return cases

def ws_cases():
    cases = []
    def c(name, args, **kw):
        d = {"name": name, "args": args, "timeout": 30}; d.update(kw); cases.append(d)
    c("shell ws basic flags", FLAGS + ["my-app", "--pod", "basic"], stdin="ls -l\n")
    c("shell ws basic empty stdin", FLAGS + ["my-app", "--pod", "basic"])
    c("shell ws command container", FLAGS + ["my-container", "--pod", "basic", "-c", "bash,-c,echo a&b", "--container", "main c"], stdin="x")
    c("shell ws empty command", FLAGS + ["my-db", "--pod", "basic", "--command", ""])
    c("shell ws repeated command", FLAGS + ["my-helm", "--pod", "basic", "-c", "a", "-c", "b,c"])
    c("shell ws ephemeral clone", FLAGS + ["my-app", "--pod", "basic", "--ephemeral", "--cpu", "500m", "--memory", "2Gi"])
    c("shell ws ephemeral debug", FLAGS + ["my-app", "--pod", "basic", "--ephemeral", "--mode", "debug", "--cpu", "1"])
    c("shell ws ephemeral debug no override", FLAGS + ["my-cron", "--pod", "basic", "--ephemeral", "--mode=debug"])
    c("shell ws mode without ephemeral", FLAGS + ["my-app", "--pod", "basic", "--mode", "debug"])
    c("shell ws url app", ["shell", URL % ("application", "app-1"), "--pod", "basic"], stdin="hello")
    c("shell ws url new console container", ["shell", (URL % ("container", "cont-1")).replace("https://console", "https://new.console"), "--pod", "basic"])
    c("shell ws url job", ["shell", URL % ("job", "life-1"), "--pod", "basic"])
    c("shell ws url db", ["shell", URL % ("database", "db-1"), "--pod", "basic"])
    c("shell ws url helm", ["shell", URL % ("helm", "helm-1"), "--pod", "basic"])
    c("shell ws url unknown service", ["shell", URL % ("application", "nope"), "--pod", "basic"])
    c("shell ws context yes", ["shell", "--pod", "basic"], stdin="yes\n", _comment="promptui buffers the whole piped stdin: the input after the confirmation is lost in Go, kept in Rust")
    c("shell ws select single", ["shell", "--pod", "basic"], context={"service_id": ""}, fixtures=[SINGLE_ORG, single("/environment/env-1/container", fx("/environment/env-1/container")["body"]["results"][0])] + [{"method": "GET", "path": "/environment/env-1/" + k, "status": 200, "body": {"results": []}} for k in ("application", "database", "job", "helm", "terraform")])
    c("shell ws text ignored", FLAGS + ["my-app", "--pod", "text-ignored"], stdin="abc\x1b[20")
    c("shell ws paste fragments", FLAGS + ["my-app", "--pod", "basic"], stdin="\x1b[200~pasted\x1b[201~\x1b")
    c("shell ws policy", FLAGS + ["my-app", "--pod", "policy"])
    c("shell ws permission", FLAGS + ["my-app", "--pod", "permission"])
    c("shell ws agent timeout", FLAGS + ["my-app", "--pod", "agent-timeout"], stdin="late\n")
    c("shell ws internal", FLAGS + ["my-app", "--pod", "internal"])
    c("shell ws custom close", FLAGS + ["my-app", "--pod", "custom"])
    c("shell ws no status close", FLAGS + ["my-app", "--pod", "nostatus"])
    c("shell ws drop", FLAGS + ["my-app", "--pod", "drop"])
    c("shell ws reject", FLAGS + ["my-app", "--pod", "reject"])
    c("shell ws reject 500", FLAGS + ["my-app", "--pod", "reject-500"])
    c("shell ws ping", FLAGS + ["my-app", "--pod", "ping"])
    c("shell ws big", FLAGS + ["my-app", "--pod", "big"])
    c("shell ws context token", FLAGS + ["my-app", "--pod", "basic"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, stdin="id\n")
    c("shell ws connection refused", FLAGS + ["my-app", "--pod", "basic"], env={"QOVERY_WS_URL": "ws://127.0.0.1:1"}, timeout=7)
    c("shell ws malformed url", FLAGS + ["my-app", "--pod", "basic"], env={"QOVERY_WS_URL": "http://127.0.0.1:1"}, timeout=7)
    return cases

json.dump({"cases": shell_cases()}, open(os.path.join(PARITY, "scenarios", "shell.json"), "w"), indent=1)
json.dump({"_comment": "Run with parity/shell_ws_compare.py parity/scenarios/shell_ws.json (starts parity/shell_ws_mock.py and substitutes $WS).", "env": {"QOVERY_WS_URL": "$WS"}, "cases": ws_cases()}, open(os.path.join(PARITY, "scenarios", "shell_ws.json"), "w"), indent=1)
