#!/usr/bin/env python3
"""Run the Go (reference) and Rust CLIs on the same scenarios and diff their behaviour.

Usage:
  compare.py SCENARIO.json [SCENARIO.json ...] [--only NAME_SUBSTRING] [-v]
  compare.py --args application list --json          # ad-hoc case with the default fixtures

Environment:
  QOVERY_GO_BIN    reference binary (default: $PARITY_DIR/../target/qovery-go, built from the Go sources)
  QOVERY_RUST_BIN  rust binary      (default: $PARITY_DIR/../target/debug/qovery)

A scenario file:
{
  "fixtures": [ ...routes, see mock_server.py... ],     # shared by all cases of the file
  "context": { "project_id": "..." },                   # overrides of ~/.qovery/context.json (see DEFAULT_CONTEXT)
  "no_context_file": false,                             # true: start without ~/.qovery/context.json
  "env": { "QOVERY_CLI_ACCESS_TOKEN": "" },             # env overrides ("" removes the variable)
  "files": { "relative/path/in/home": "content" },      # extra files written in $HOME before each run
  "cases": [
    {"name": "list json", "args": ["application", "list", "--json"], "stdin": "", "fixtures": [...], "env": {...},
     "ignore": ["stderr"], "timeout": 20}
  ]
}

Each binary runs in a fresh sandbox $HOME (same path for both), with QOVERY_API_URL pointing to a fresh mock
server, QOVERY_TELEMETRY=false, NO tty. Compared: exit code, stdout, stderr, the API requests (method, path,
query, JSON body) and the final content of ~/.qovery/context.json. `ignore` can list any of
"exit", "stdout", "stderr", "requests", "context" for a case.
A case can also assert what both CLIs must do, on top of parity:
  "expect": {"exit": 1, "stdout_contains": "...", "requests": ["GET /environment/env-1/application"]}
"requests" is the exact ordered list of "METHOD /path" the CLI must send.
Volatile values are normalized: logrus timestamps, durations like "1.234s", the sandbox paths.
"""

import argparse
import difflib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

PARITY_DIR = os.path.dirname(os.path.abspath(__file__))
RUST_DIR = os.path.dirname(PARITY_DIR)
GO_BIN = os.environ.get("QOVERY_GO_BIN", os.path.join(RUST_DIR, "target", "qovery-go"))
RUST_BIN = os.environ.get("QOVERY_RUST_BIN", os.path.join(RUST_DIR, "target", "debug", "qovery"))

DEFAULT_CONTEXT = {
    "access_token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEiLCJhenAiOiJjbGllbnQtMSJ9.c2ln",
    "access_token_expiration": "2099-01-01T00:00:00Z",
    "refresh_token": "refresh-token",
    "organization_id": "org-1",
    "organization_name": "my-org",
    "project_id": "proj-1",
    "project_name": "my-project",
    "environment_id": "env-1",
    "environment_name": "my-env",
    "service_id": "app-1",
    "service_name": "my-app",
    "service_type": "application",
    "user": "user-1",
}

DEFAULT_ENV = {
    "QOVERY_CLI_ACCESS_TOKEN": "qov_testtoken",
    "QOVERY_TELEMETRY": "false",
    "TERM": "xterm",
    "LANG": "C.UTF-8",
    "TZ": "UTC",
}

DEFAULT_FIXTURES = json.load(open(os.path.join(PARITY_DIR, "fixtures", "default.json")))

SANDBOX = os.path.join(tempfile.gettempdir(), "qovery-parity-%d" % os.getpid())


def normalize(text, home):
    text = text.replace(home, "$HOME")
    text = re.sub(r'time="[^"]*"', 'time="<T>"', text)
    text = re.sub(r"\[\d{4}-\d\d-\d\dT[\d:]+[+-]\d\d:\d\d\]", "[<T>]", text)
    text = re.sub(r"\b\d+(\.\d+)?(ms|µs|ns|s)\b", "<DUR>", text)
    text = re.sub(r"(?m)^\d{4}/\d\d/\d\d \d\d:\d\d:\d\d ", "<T> ", text)
    text = re.sub(r"127\.0\.0\.1:\d+", "127.0.0.1:<PORT>", text)
    text = re.sub(r"(?m)^Date: .*GMT\r?$", "Date: <DATE>", text)
    return text


def start_mock(fixtures, workdir):
    fx = os.path.join(workdir, "fixtures.json")
    log = os.path.join(workdir, "requests.jsonl")
    portfile = os.path.join(workdir, "port")
    for p in (log, portfile):
        if os.path.exists(p):
            os.remove(p)
    json.dump(fixtures, open(fx, "w"))
    proc = subprocess.Popen([sys.executable, "-I", os.path.join(PARITY_DIR, "mock_server.py"), fx, log, portfile])
    for _ in range(200):
        if os.path.exists(portfile) and open(portfile).read().strip():
            return proc, int(open(portfile).read()), log
        time.sleep(0.02)
    raise RuntimeError("mock server did not start")


def run_one(binary, scenario, case):
    home = os.path.join(SANDBOX, "home")
    work = os.path.join(SANDBOX, "work")
    shutil.rmtree(SANDBOX, ignore_errors=True)
    os.makedirs(home)
    os.makedirs(work)
    if not scenario.get("no_context_file") and not case.get("no_context_file"):
        os.makedirs(os.path.join(home, ".qovery"))
        ctx = dict(DEFAULT_CONTEXT)
        ctx.update(scenario.get("context") or {})
        ctx.update(case.get("context") or {})
        with open(os.path.join(home, ".qovery", "context.json"), "w") as f:
            json.dump(ctx, f)
        os.chmod(os.path.join(home, ".qovery", "context.json"), 0o600)
    for rel, content in {**(scenario.get("files") or {}), **(case.get("files") or {})}.items():
        p = os.path.join(home, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w") as f:
            f.write(content)
    fixtures = (case.get("fixtures") or []) + (scenario.get("fixtures") or []) + DEFAULT_FIXTURES
    mock, port, log = start_mock(fixtures, work)
    env = {"PATH": os.environ.get("PATH", ""), "HOME": home}
    env.update(DEFAULT_ENV)
    env["QOVERY_API_URL"] = "http://127.0.0.1:%d" % port
    env["QOVERY_WS_URL"] = "ws://127.0.0.1:%d" % port
    for k, v in {**(scenario.get("env") or {}), **(case.get("env") or {})}.items():
        if v == "" or v is None:
            env.pop(k, None)
        else:
            env[k] = v.replace("$MOCK", "http://127.0.0.1:%d" % port).replace("$PARITY", PARITY_DIR)
    try:
        p = subprocess.run(
            [binary] + case["args"],
            input=(case.get("stdin") or "").encode(),
            capture_output=True,
            env=env,
            cwd=home,
            timeout=case.get("timeout", 30),
        )
        out, err, code = p.stdout.decode("utf-8", "replace"), p.stderr.decode("utf-8", "replace"), p.returncode
    except subprocess.TimeoutExpired as e:
        out = (e.stdout or b"").decode("utf-8", "replace")
        err = (e.stderr or b"").decode("utf-8", "replace") + "\n<TIMEOUT>"
        code = "timeout"
    finally:
        mock.terminate()
        mock.wait()
    reqs = [json.loads(l) for l in open(log)] if os.path.exists(log) else []
    ctx_path = os.path.join(home, ".qovery", "context.json")
    try:
        context = json.load(open(ctx_path))
        context.pop("access_token_expiration", None)
    except Exception:
        context = None
    return {
        "exit": str(code),
        "stdout": normalize(out, home),
        "stderr": normalize(err, home),
        "requests": "\n".join(json.dumps(r, sort_keys=True) for r in reqs),
        "context": json.dumps(context, sort_keys=True, indent=1),
    }


def show(s):
    return s.replace("\x1b", "\\e")


def unmet_expectations(expect, side, result):
    unmet = []
    if "exit" in expect and result["exit"] != str(expect["exit"]):
        unmet.append("%s/expect: exit %s, got %s" % (side, expect["exit"], result["exit"]))
    if "stdout_contains" in expect and expect["stdout_contains"] not in result["stdout"]:
        unmet.append("%s/expect: stdout without %r, got %r" % (side, expect["stdout_contains"], result["stdout"]))
    if "requests" in expect:
        sent = [r["method"] + " " + r["path"] for r in map(json.loads, filter(None, result["requests"].split("\n")))]
        if sent != expect["requests"]:
            unmet.append("%s/expect: requests %s, got %s" % (side, expect["requests"], sent))
    return unmet


def diff_case(scenario, case):
    go = run_one(GO_BIN, scenario, case)
    rs = run_one(RUST_BIN, scenario, case)
    ignore = set(case.get("ignore") or []) | set(scenario.get("ignore") or [])
    diffs = []
    for key in ("exit", "stdout", "stderr", "requests", "context"):
        if key in ignore:
            continue
        if go[key] != rs[key]:
            d = difflib.unified_diff(
                show(go[key]).splitlines(), show(rs[key]).splitlines(), "go/" + key, "rust/" + key, lineterm="", n=2
            )
            diffs.append("\n".join(d))
    for side, result in (("go", go), ("rust", rs)):
        diffs.extend(unmet_expectations(case.get("expect") or {}, side, result))
    return go, diffs


def compare_case(scenario, case, verbose, retries=0):
    # a few Go outputs are not deterministic (promptui redraw races): a case only drifts when every attempt differs
    for attempt in range(retries + 1):
        go, diffs = diff_case(scenario, case)
        if not diffs:
            break
    name = case.get("name") or " ".join(case["args"])
    if diffs:
        print("DIFF  %s   [qovery %s]" % (name, " ".join(case["args"])))
        for d in diffs:
            print("\n".join("      " + l for l in d.splitlines()))
        return False
    print("SAME  %s%s" % (name, "   (attempt %d)" % (attempt + 1) if attempt else ""))
    if verbose:
        print("      stdout: " + show(go["stdout"])[:2000].replace("\n", "\n              "))
        print("      stderr: " + show(go["stderr"])[:2000].replace("\n", "\n              "))
        print("      requests: " + go["requests"][:2000].replace("\n", "\n                "))
    return True


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("scenarios", nargs="*")
    ap.add_argument("--only")
    ap.add_argument("-v", "--verbose", action="store_true")
    ap.add_argument("--retries", type=int, default=0, help="re-run a differing case up to N times before reporting it")
    ap.add_argument("--args", nargs=argparse.REMAINDER)
    a = ap.parse_args()
    for b in (GO_BIN, RUST_BIN):
        if not os.path.exists(b):
            sys.exit("missing binary: " + b)
    ok = total = 0
    jobs = []
    if a.args is not None:
        jobs.append(({}, {"args": a.args}))
    for path in a.scenarios:
        sc = json.load(open(path))
        for case in sc.get("cases", []):
            name = case.get("name") or " ".join(case["args"])
            if a.only and a.only not in name:
                continue
            jobs.append((sc, case))
    for sc, case in jobs:
        total += 1
        ok += compare_case(sc, case, a.verbose, a.retries)
    shutil.rmtree(SANDBOX, ignore_errors=True)
    print("\n%d/%d identical" % (ok, total))
    sys.exit(0 if ok == total else 1)


if __name__ == "__main__":
    main()
