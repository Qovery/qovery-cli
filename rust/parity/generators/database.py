#!/usr/bin/env python3
"""Generates parity/scenarios/database.json.

Usage: python3 parity/generators/database.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "database.json")

def status(id, state, exec_id):
    return {"id": id, "state": state, "service_deployment_status": "UP_TO_DATE", "is_part_last_deployment": True,
            "status_details": {"action": "DEPLOY", "status": "SUCCESS", "sub_action": "NONE"},
            "deployment_request_id": None, "deployment_requests_count": 0, "execution_id": exec_id}

def stages(dbs):
    return {"stages": [{"databases": [status(*d) for d in dbs]}]}

def swst(*seq):
    return {"method": "GET", "path": "/environment/env-1/statusesWithStages",
            "responses": [{"status": 200, "body": stages(s)} for s in seq]}

def db(id, name, typ="POSTGRESQL", **kw):
    d = {"id": id, "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": "2024-06-02T11:30:00Z", "name": name,
         "type": typ, "version": "16", "mode": "CONTAINER", "icon_uri": "app://qovery-console/database",
         "environment": {"id": "env-1"}, "service_type": "DATABASE"}
    d.update(kw)
    return d

def tf(id, name, blueprint_id=None, updated="2024-07-03T08:09:10.5+02:00"):
    t = {"id": id, "created_at": "2024-05-01T10:00:00.123456Z", "updated_at": updated, "name": name,
         "timeout_sec": 3600, "auto_deploy": False, "icon_uri": "app://qovery-console/terraform", "service_type": "TERRAFORM",
         "terraform_variables_source": {"tf_var_file_paths": [], "tf_vars": []}, "engine": "TERRAFORM",
         "backend": {"kubernetes": {}}, "provider_version": {"read_from_terraform_block": False, "explicit_version": "1.9.0"},
         "job_resources": {"cpu_milli": 500, "ram_mib": 512, "gpu": 0, "storage_gib": 1}, "environment": {"id": "env-1"},
         "use_cluster_credentials": False, "action_extra_arguments": {}}
    if blueprint_id is not None:
        t["blueprint_id"] = blueprint_id
    return t

DBS = [db("db-1", "my-db", host="db-1.internal", port=5432), db("db-2", "other-db", "MYSQL")]
TFS = [tf("tf-1", "my-terraform"), tf("tf-bp-1", "bp-pg", "bp-1"), tf("tf-bp-2", "bp-bucket", "bp-2"),
       tf("tf-bp-3", "bp-redis", "bp-3"), tf("tf-bp-4", "bp-mongo", "bp-4", updated="2024-01-01T00:00:00Z"),
       tf("tf-bp-5", "bp-null", None)]

STATUSES = {"environment": {"id": "env-1", "state": "DEPLOYED", "last_deployment_state": "DEPLOYED"},
            "applications": [], "containers": [], "jobs": [], "helms": [],
            "databases": [status("db-1", "DEPLOYED", "e1"), status("db-2", "STOPPED", "e2")],
            "terraforms": [status("tf-1", "DEPLOYED", "e3"), status("tf-bp-1", "DEPLOYMENT_ERROR", "e4"), status("tf-bp-3", "DEPLOYING", "e5")]}

def cred(host, port, login, pw):
    return {"host": host, "port": port, "login": login, "password": pw}

fixtures = [
    {"method": "GET", "path": "/environment/env-1/database", "body": {"results": DBS}},
    {"method": "GET", "path": "/environment/env-1/terraform", "body": {"results": TFS}},
    {"method": "GET", "path": "/environment/env-1/statuses", "body": STATUSES},
    {"method": "GET", "path": "/blueprint/bp-1/database", "body": {"kind": "POSTGRESQL", "service_id": "tf-bp-1", "endpoint": {"host": "pg.example.com", "port": 5433}}},
    {"method": "GET", "path": "/blueprint/bp-2/database", "status": 404, "body": {"status": 404, "error": "not a database"}},
    {"method": "GET", "path": "/blueprint/bp-3/database", "body": {"kind": "REDIS", "service_id": "tf-bp-3", "endpoint": {"host": "redis.example.com", "port": None}}},
    {"method": "GET", "path": "/blueprint/bp-4/database", "body": {"kind": "MONGODB", "service_id": None, "endpoint": None}},
    {"method": "GET", "path": "/blueprint/bp-1/database/masterCredentials", "body": cred("pg.example.com", 5433, "admin", "s3cr<t>&")},
    {"method": "GET", "path": "/blueprint/bp-3/database/masterCredentials", "status": 403, "body": {"status": 403}},
    {"method": "GET", "path": "/blueprint/bp-4/database/masterCredentials", "status": 404, "body": {"status": 404}},
    {"method": "GET", "path": "/database/db-1/masterCredentials", "body": cred("db-1.internal", 5432, "postgres", "pw1")},
    {"method": "GET", "path": "/database/db-2/masterCredentials", "body": cred("db-2.internal", 3306, "", "pw2")},
    {"method": "POST", "path": "/environment/env-1/service/deploy", "status": 202, "body": {"id": "env-1", "state": "DEPLOYMENT_QUEUED", "last_deployment_state": "DEPLOYED"}},
    {"method": "POST", "path": "/environment/env-1/service/delete", "status": 202},
    {"method": "POST", "path": "/environment/env-1/service/stop", "status": 202},
    {"method": "POST", "path": "/environment/env-1/service/restart-service", "status": 202, "body": {}},
    {"method": "POST", "path": "/database/db-1/deploy", "status": 202, "body": status("db-1", "DEPLOYMENT_QUEUED", "e1")},
]

def err(method, path, code=500, body=None):
    return {"method": method, "path": path, "status": code, "body": body if body is not None else {"status": code, "error": "boom"}}

cases = []
def case(name, args, **kw):
    c = {"name": name, "args": args}
    c.update(kw)
    cases.append(c)

case("group help", ["database"])
case("list table", ["database", "list"])
case("list json", ["database", "list", "--json"])
case("list show-credentials", ["database", "list", "--show-credentials"])
case("list show-credentials json", ["database", "list", "--show-credentials", "--json"])
case("list explicit env", ["database", "list", "--organization", "my-org", "--project", "my-project", "--environment", "my-env"])
case("list env not found", ["database", "list", "--environment", "nope"])
case("list empty table", ["database", "list"], fixtures=[{"method": "GET", "path": "/environment/env-1/database", "body": {"results": []}}, {"method": "GET", "path": "/environment/env-1/terraform", "body": {"results": []}}])
case("list empty json", ["database", "list", "--json"], fixtures=[{"method": "GET", "path": "/environment/env-1/database", "body": {"results": []}}, {"method": "GET", "path": "/environment/env-1/terraform", "body": {"results": []}}])
case("list database error", ["database", "list"], fixtures=[err("GET", "/environment/env-1/database")])
case("list statuses error", ["database", "list", "--json"], fixtures=[err("GET", "/environment/env-1/statuses", 403)])
case("list terraform error", ["database", "list"], fixtures=[err("GET", "/environment/env-1/terraform", 401)])
case("list blueprint error", ["database", "list"], fixtures=[err("GET", "/blueprint/bp-1/database")])
case("list blueprint creds error", ["database", "list", "--show-credentials"], fixtures=[err("GET", "/blueprint/bp-1/database/masterCredentials")])
case("list creds error", ["database", "list", "--show-credentials"], fixtures=[err("GET", "/database/db-2/masterCredentials", 404)])
case("list creds error json", ["database", "list", "--show-credentials", "--json"], fixtures=[err("GET", "/database/db-1/masterCredentials", 500)])
case("list no token", ["database", "list"], env={"QOVERY_CLI_ACCESS_TOKEN": ""}, no_context_file=True)
case("list bad flag", ["database", "list", "--nope"])

case("cancel", ["database", "cancel"])

for cmd, method, path, state_word in [("deploy", "POST", "/environment/env-1/service/deploy", "DEPLOYED"),
                                      ("delete", "POST", "/environment/env-1/service/delete", "DELETED"),
                                      ("stop", "POST", "/environment/env-1/service/stop", "STOPPED"),
                                      ("restart", "POST", "/environment/env-1/service/restart-service", "RESTARTED")]:
    case(cmd, ["database", cmd, "-n", "my-db"])
    case(cmd + " multiple", ["database", cmd, "--databases", "my-db, other-db"])
    case(cmd + " no name", ["database", cmd])
    case(cmd + " both", ["database", cmd, "-n", "my-db", "--databases", "other-db"])
    case(cmd + " not found", ["database", cmd, "-n", "nope"])
    case(cmd + " multiple not found", ["database", cmd, "--databases", "my-db, nope"])
    case(cmd + " api error", ["database", cmd, "-n", "my-db"], fixtures=[err(method, path, 400)])
    case(cmd + " list error", ["database", cmd, "-n", "my-db"], fixtures=[err("GET", "/environment/env-1/database", 500)])
    case(cmd + " env not found", ["database", cmd, "-n", "my-db", "--project", "nope"])
    if state_word == "DELETED":
        seq = ([("db-1", "DEPLOYED", "x1")], [("db-1", "DELETE_QUEUED", "x2")], [])
    else:
        seq = ([("db-1", "DEPLOYED", "x1")], [("db-1", "QUEUED", "x2")], [("db-1", state_word, "x2")])
    case(cmd + " watch", ["database", cmd, "-n", "my-db", "-w"], fixtures=[swst(*seq)], timeout=60)
    fail = "DEPLOYMENT_ERROR" if state_word in ("DEPLOYED", "RESTARTED") else ("DELETE_ERROR" if state_word == "DELETED" else "STOP_ERROR")
    case(cmd + " watch error", ["database", cmd, "-n", "my-db", "--watch"],
         fixtures=[swst([("db-1", "DEPLOYED", "x1")], [("db-1", fail, "x2")])], timeout=60)

case("deploy watch snapshot error", ["database", "deploy", "-n", "my-db", "-w"],
     fixtures=[err("GET", "/environment/env-1/statusesWithStages", 500)], timeout=60)

case("redeploy", ["database", "redeploy", "-n", "my-db"])
case("redeploy missing flag", ["database", "redeploy"])
case("redeploy not found", ["database", "redeploy", "--database", "nope"])
case("redeploy api error", ["database", "redeploy", "-n", "my-db"], fixtures=[err("POST", "/database/db-1/deploy", 409)])
case("redeploy watch", ["database", "redeploy", "-n", "my-db", "-w"],
     fixtures=[swst([("db-1", "DEPLOYED", "x1")], [("db-1", "DEPLOYING", "x2")], [("db-1", "DEPLOYED", "x2")])], timeout=60)
case("redeploy databases flag unknown", ["database", "redeploy", "-n", "my-db", "--databases", "x"])

json.dump({"fixtures": fixtures, "cases": cases}, open(OUT, "w"), indent=1)
print(len(cases), "cases")
