#!/usr/bin/env python3
"""Generates parity/scenarios/port-forward_ws.json.

Usage: python3 parity/generators/port-forward_ws.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
URL = "https://console.qovery.com/organization/org-1/project/proj-1/environment/env-1/%s/%s"
cases = []
def c(name, args, session=None, **kw):
    d = {"name": "port-forward ws " + name, "args": ["port-forward"] + args, "stdin": "yes\n"}
    if session is not None:
        d["session"] = session
    d.update(kw); cases.append(d)
ECHO = [{"connect": "a"}, {"recv": "a", "until": "auth="}, {"send": "a", "data": "hello"}, {"recv": "a", "until": "echo:hello"}, {"close": "a"}]
EOF_ONLY = [{"connect": "a"}, {"recv": "a"}]
c("echo context", ["-p", "0:80", "--pod", "pf-echo"], ECHO)
c("echo url", [URL % ("application", "app-1"), "-p", "0:8080", "--pod", "pf-echo"], ECHO)
c("echo url terraform", [URL % ("terraform", "tf-1"), "--port", "0:5432", "--pod", "pf-echo"], ECHO)
c("echo same port", ["-p", "0", "--pod", "pf-echo"], [{"connect": "a"}, {"recv": "a", "until": "auth="}])
c("echo two connections", ["-p", "0:80", "--pod", "pf-echo"], ECHO + [{"connect": "b"}, {"recv": "b", "until": "auth="}, {"send": "b", "data": "second"}, {"recv": "b", "until": "echo:second"}])
c("banner", ["-p", "0:80", "--pod", "pf-banner"], [{"connect": "a"}, {"recv": "a", "until": "banner"}, {"send": "a", "data": "x"}, {"recv": "a", "until": "echo:x"}])
c("half close", ["-p", "0:80", "--pod", "pf-echo"], [{"connect": "a"}, {"recv": "a", "until": "auth="}, {"send": "a", "data": "bye"}, {"shutdown_write": "a"}, {"recv": "a", "until": "echo:bye"}])
c("text ignored", ["-p", "0:80", "--pod", "pf-text"], [{"connect": "a"}, {"recv": "a", "until": "bin"}, {"send": "a", "data": "q"}, {"recv": "a", "until": "echo:q"}])
for key in ["pf-normal", "pf-policy", "pf-permission", "pf-timeout", "pf-internal", "pf-custom", "pf-drop", "pf-reject", "pf-big"]:
    c(key, ["-p", "0:80", "--pod", key], EOF_ONLY)
c("connection refused", ["-p", "0:80", "--pod", "pf-echo"], EOF_ONLY, env={"QOVERY_WS_URL": "ws://127.0.0.1:1"})
c("no token at connection", ["-p", "0:80", "--pod", "pf-echo"], EOF_ONLY, env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""})
c("address in use", ["-p", "$BUSY:80", "--pod", "pf-echo"], [], busy_port=True)
c("two listeners", ["-p", "0:80", "-p", "0:81", "--pod", "pf-echo"], [], listeners=2, sort_stdout=True)
c("no connection", ["-p", "0:80"], [])
json.dump({"_comment": "Run with parity/port-forward_ws_compare.py (TCP sessions against the local port, websocket mock parity/shell_ws_mock.py).", "cases": cases}, open(sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "port-forward_ws.json"), "w"), indent=1)
