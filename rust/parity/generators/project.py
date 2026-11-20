#!/usr/bin/env python3
"""Generates parity/scenarios/project.json.

Usage: python3 parity/generators/project.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "project.json")
VARS = [
    {"id": "var-1", "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": "2024-06-02T11:30:00Z", "key": "FOO",
     "value": "bar", "scope": "PROJECT", "variable_type": "VALUE", "is_secret": False, "owned_by": "QOVERY"},
    {"id": "var-2", "created_at": "2024-05-02T10:00:00Z", "updated_at": "2024-05-02T10:00:00Z", "key": "DB_PASSWORD", "value": None, "scope": "PROJECT",
     "variable_type": "VALUE", "is_secret": True, "description": "the db password"},
    {"id": "var-3", "created_at": "2024-05-03T10:00:00Z", "updated_at": "2024-12-25T23:59:59.5+02:00", "key": "ALIAS_FOO", "value": None, "scope": "PROJECT",
     "variable_type": "ALIAS", "is_secret": False,
     "aliased_variable": {"id": "var-1", "key": "FOO", "value": "bar", "mount_path": "", "scope": "PROJECT", "variable_type": "VALUE"}},
    {"id": "var-4", "created_at": "2024-05-04T10:00:00Z", "updated_at": "2024-05-04T10:00:00Z", "key": "CONFIG_FILE", "value": "a=b\nc=d", "mount_path": "/etc/config",
     "scope": "PROJECT", "variable_type": "FILE", "is_secret": False},
    {"id": "var-5", "created_at": "2024-05-05T10:00:00Z", "updated_at": "2023-01-05T08:00:00Z", "service_name": "my-app", "service_id": "app-1", "service_type": "APPLICATION", "key": "A_INTERP", "value": "{{FOO}}-x", "scope": "PROJECT",
     "variable_type": "VALUE", "is_secret": False},
]
LIST_Q = {"parent_id": "proj-1", "scope": "PROJECT"}
created = {"id": "var-9", "created_at": "2024-07-01T10:00:00Z", "key": "NEW_KEY", "value": "val", "scope": "PROJECT",
           "variable_type": "VALUE", "is_secret": False}
fixtures = [
    {"method": "GET", "path": "/variable", "query": LIST_Q, "status": 200, "body": {"results": VARS}},
    {"method": "POST", "path": "/variable", "status": 201, "body": created},
    {"method": "PUT", "path": "/variable/var-1", "status": 200, "body": dict(VARS[0], value="new")},
    {"method": "DELETE", "path": "/variable/var-1", "status": 204},
    {"method": "POST", "path": "/variable/var-1/alias", "status": 201, "body": dict(VARS[2], id="var-10", key="FOO2")},
]
P = ["--project", "my-project"]
# Go iterates over a map of keys when --sort is not set: a single key group keeps the order deterministic
SINGLE = [{"method": "GET", "path": "/variable", "query": LIST_Q, "status": 200, "body": {"results": [VARS[0], VARS[2]]}}]
def c(name, args, **kw):
    d = {"name": name, "args": args}
    d.update(kw)
    return d
err500 = lambda method, path, **q: [dict({"method": method, "path": path, "status": 500, "body": {"message": "boom"}}, **q)]
cases = [
    c("project (group)", ["project"]),
    c("project list", ["project", "list"]),
    c("project list --json", ["project", "list", "--json"]),
    c("project list --organization", ["project", "list", "--organization", "my-org"]),
    c("project list --organization other (no projects)", ["project", "list", "--organization", "other-org"],
      fixtures=[{"method": "GET", "path": "/organization/org-2/project", "status": 200, "body": {"results": []}}]),
    c("project list --organization other --json empty", ["project", "list", "--organization", "other-org", "--json"],
      fixtures=[{"method": "GET", "path": "/organization/org-2/project", "status": 200, "body": {"results": []}}]),
    c("project list unknown organization", ["project", "list", "--organization", "nope"]),
    c("project list api error", ["project", "list"], fixtures=err500("GET", "/organization/org-1/project")),
    c("project list org api error", ["project", "list", "--organization", "my-org"], fixtures=err500("GET", "/organization")),
    c("project list no organization in context", ["project", "list"], context={"organization_id": "", "organization_name": ""}),
    c("project list extra args", ["project", "list", "foo"]),
    c("project list unknown flag", ["project", "list", "--nope"]),
    c("project env (group)", ["project", "env"]),
    c("project env alias (group)", ["project", "env", "alias"]),
    c("project env unknown sub", ["project", "env", "foo"]),
    c("env list", ["project", "env", "list"] + P, fixtures=SINGLE),
    c("env list --show-values", ["project", "env", "list", "--show-values"] + P, fixtures=SINGLE),
    c("env list --pretty-print", ["project", "env", "list", "--pretty-print"] + P, fixtures=SINGLE),
    c("env list --sort", ["project", "env", "list", "--sort"] + P),
    c("env list --sort --show-values", ["project", "env", "list", "--sort", "--show-values"] + P),
    c("env list --sort --pretty-print", ["project", "env", "list", "--sort", "--pretty-print"] + P),
    c("env list all flags", ["project", "env", "list", "--sort", "--show-values", "--pretty-print"] + P),
    c("env list --json", ["project", "env", "list", "--json"] + P),
    c("env list --json --sort", ["project", "env", "list", "--json", "--sort"] + P),
    c("env list --organization", ["project", "env", "list", "--organization", "my-org"] + P, fixtures=SINGLE),
    c("env list empty", ["project", "env", "list"] + P,
      fixtures=[{"method": "GET", "path": "/variable", "query": LIST_Q, "status": 200, "body": {"results": []}}]),
    c("env list --json empty", ["project", "env", "list", "--json"] + P,
      fixtures=[{"method": "GET", "path": "/variable", "query": LIST_Q, "status": 200, "body": {"results": []}}]),
    c("env list api error", ["project", "env", "list"] + P, fixtures=err500("GET", "/variable")),
    c("env list projects api error", ["project", "env", "list"] + P, fixtures=err500("GET", "/organization/org-1/project")),
    c("env list unknown project", ["project", "env", "list", "--project", "nope"]),
    c("env list unknown organization", ["project", "env", "list", "--organization", "nope"] + P),
    c("env list variable without updated_at (Go nil pointer panic)", ["project", "env", "list"] + P, ignore=["stderr"],
      fixtures=[{"method": "GET", "path": "/variable", "query": LIST_Q, "status": 200, "body": {"results": [dict(VARS[0], updated_at=None)]}}]),
    c("env list missing --project", ["project", "env", "list"]),
    c("env list blank --project (Go nil pointer panic)", ["project", "env", "list", "--project", " "], ignore=["stderr"]),
    c("env list no environment in context (auto-selected)", ["project", "env", "list"] + P,
      context={"environment_id": "", "environment_name": ""}),
    c("env create", ["project", "env", "create", "-k", "NEW_KEY", "-v", "val"] + P),
    c("env create --secret", ["project", "env", "create", "--key", "NEW_KEY", "--value", "s3cr3t", "--secret"] + P),
    c("env create api error", ["project", "env", "create", "-k", "NEW_KEY", "-v", "val"] + P,
      fixtures=[{"method": "POST", "path": "/variable", "status": 400, "body": {"status": 400, "message": "key already exists"}}]),
    c("env create api error 500", ["project", "env", "create", "-k", "NEW_KEY", "-v", "val"] + P, fixtures=err500("POST", "/variable")),
    c("env create missing --value", ["project", "env", "create", "-k", "NEW_KEY"] + P),
    c("env create missing all", ["project", "env", "create"]),
    c("env create unknown project", ["project", "env", "create", "-k", "A", "-v", "b", "--project", "nope"]),
    c("env update", ["project", "env", "update", "-k", "FOO", "-v", "new"] + P),
    c("env update unknown key", ["project", "env", "update", "-k", "NOPE", "-v", "new"] + P),
    c("env update api error", ["project", "env", "update", "-k", "FOO", "-v", "new"] + P, fixtures=err500("PUT", "/variable/var-1")),
    c("env update list api error", ["project", "env", "update", "-k", "FOO", "-v", "new"] + P, fixtures=err500("GET", "/variable")),
    c("env update missing --key", ["project", "env", "update", "-v", "new"] + P),
    c("env delete", ["project", "env", "delete", "-k", "FOO"] + P),
    c("env delete unknown key", ["project", "env", "delete", "--key", "NOPE"] + P),
    c("env delete api error", ["project", "env", "delete", "-k", "FOO"] + P,
      fixtures=[{"method": "DELETE", "path": "/variable/var-1", "status": 404, "body": {"message": "not found"}}]),
    c("env delete missing --key", ["project", "env", "delete"] + P),
    c("env alias create", ["project", "env", "alias", "create", "-k", "FOO", "--alias", "FOO2"] + P),
    c("env alias create unknown key", ["project", "env", "alias", "create", "-k", "NOPE", "--alias", "FOO2"] + P),
    c("env alias create api error", ["project", "env", "alias", "create", "-k", "FOO", "--alias", "FOO2"] + P,
      fixtures=[{"method": "POST", "path": "/variable/var-1/alias", "status": 400, "body": {"message": "invalid alias"}}]),
    c("env alias create missing --alias", ["project", "env", "alias", "create", "-k", "FOO"] + P),
    c("env alias create no token", ["project", "env", "alias", "create", "-k", "FOO", "--alias", "X"] + P,
      env={"QOVERY_CLI_ACCESS_TOKEN": ""}, context={"access_token": ""}),
]
json.dump({"fixtures": fixtures, "cases": cases}, open(out, "w"), indent=1)
