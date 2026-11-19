#!/usr/bin/env python3
"""Compare `qovery port-forward` tunnels between the Go and Rust CLIs.

Starts parity/shell_ws_mock.py (websocket gateway) and, per case and per binary, the API mock of compare.py, runs
`qovery port-forward ...` (with "$WS" substituted in env values), waits for its `Listening on` line, plays the
case's TCP `session` against the local port, sends SIGINT and compares exit code, stdout, stderr, API requests and
the bytes received by the TCP clients.

Usage: port-forward_ws_compare.py [scenario.json] [-v] [--only NAME]

Scenario: like compare.py, plus per case
  "session": [{"connect": "a"}, {"send": "a", "data": "hello"}, {"recv": "a", "until": "echo:hello"},
              {"recv": "a"} (until EOF), {"close": "a"}, {"shutdown_write": "a"}, {"sleep": 0.5}],
  "sort_stdout": true   (several listeners print in a random order)
  "busy_port": true     (the local port given as "$BUSY" in args is already bound)
"""

import argparse
import difflib
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time

PARITY_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, PARITY_DIR)
import compare  # noqa: E402
from shell_ws_compare import start_ws_mock  # noqa: E402


class Reader(threading.Thread):
    def __init__(self, stream):
        super().__init__(daemon=True)
        self.stream, self.data, self.lock = stream, b"", threading.Lock()

    def run(self):
        while True:
            chunk = self.stream.read1(4096) if hasattr(self.stream, "read1") else self.stream.read(4096)
            if not chunk:
                return
            with self.lock:
                self.data += chunk

    def text(self):
        with self.lock:
            return self.data.decode("utf-8", "replace")


def wait_for(pred, timeout):
    end = time.time() + timeout
    while time.time() < end:
        if pred():
            return True
        time.sleep(0.02)
    return False


def summarize(data):
    if len(data) > 200:
        return "%d bytes: %r...%r" % (len(data), data[:20], data[-20:])
    return repr(data)


def play(session, port, transcript):
    socks = {}
    for step in session:
        if "connect" in step:
            socks[step["connect"]] = socket.create_connection(("127.0.0.1", port), timeout=5)
            transcript.append("connect %s" % step["connect"])
            time.sleep(0.3)
        elif "send" in step:
            socks[step["send"]].sendall(step["data"].encode())
            transcript.append("send %s %r" % (step["send"], step["data"]))
        elif "recv" in step:
            s = socks[step["recv"]]
            s.settimeout(step.get("timeout", 5))
            data, until = b"", step.get("until")
            try:
                while not (until and until.encode() in data):
                    chunk = s.recv(65536)
                    if not chunk:
                        data += b"<EOF>"
                        break
                    data += chunk
            except socket.timeout:
                data += b"<TIMEOUT>"
            except OSError as e:
                data += ("<%s>" % e.__class__.__name__).encode()
            transcript.append("recv %s %s" % (step["recv"], summarize(data)))
        elif "shutdown_write" in step:
            socks[step["shutdown_write"]].shutdown(socket.SHUT_WR)
        elif "close" in step:
            socks.pop(step["close"]).close()
            time.sleep(0.3)
        elif "sleep" in step:
            time.sleep(step["sleep"])
    time.sleep(0.5)
    for s in socks.values():
        s.close()


def run_one(binary, scenario, case, ws):
    home = os.path.join(compare.SANDBOX, "home")
    work = os.path.join(compare.SANDBOX, "work")
    shutil.rmtree(compare.SANDBOX, ignore_errors=True)
    os.makedirs(os.path.join(home, ".qovery"))
    os.makedirs(work)
    ctx = dict(compare.DEFAULT_CONTEXT)
    ctx.update(scenario.get("context") or {})
    ctx.update(case.get("context") or {})
    json.dump(ctx, open(os.path.join(home, ".qovery", "context.json"), "w"))
    fixtures = (case.get("fixtures") or []) + (scenario.get("fixtures") or []) + compare.DEFAULT_FIXTURES
    mock, port, log = compare.start_mock(fixtures, work)
    env = {"PATH": os.environ.get("PATH", ""), "HOME": home}
    env.update(compare.DEFAULT_ENV)
    env["QOVERY_API_URL"] = "http://127.0.0.1:%d" % port
    env["QOVERY_WS_URL"] = ws
    for k, v in {**(scenario.get("env") or {}), **(case.get("env") or {})}.items():
        if v == "" or v is None:
            env.pop(k, None)
        else:
            env[k] = v.replace("$WS", ws)
    busy = None
    args = list(case["args"])
    if case.get("busy_port"):
        busy = socket.socket()
        busy.bind(("127.0.0.1", 0))
        busy.listen(1)
        args = [a.replace("$BUSY", str(busy.getsockname()[1])) for a in args]
    proc = subprocess.Popen([binary] + args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, cwd=home)
    proc.stdin.write((case.get("stdin") or "").encode())
    proc.stdin.close()
    out, err = Reader(proc.stdout), Reader(proc.stderr)
    out.start()
    err.start()
    transcript = []
    expected = case.get("listeners", 1)
    wait_for(lambda: len(re.findall(r"Listening on |listen tcp ", out.text())) >= expected or proc.poll() is not None, 15)
    m = re.search(r"Listening on 127\.0\.0\.1:(\d+)", out.text())
    if m and proc.poll() is None:
        play(case.get("session") or [], int(m.group(1)), transcript)
    if proc.poll() is None:
        proc.send_signal(signal.SIGINT)
    try:
        code = proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()
        code = "timeout"
    out.join(2)
    err.join(2)
    mock.terminate()
    mock.wait()
    if busy:
        busy.close()
    stdout = compare.normalize(out.text(), home)
    if case.get("sort_stdout"):
        stdout = "\n".join(sorted(stdout.splitlines()))
    reqs = [json.loads(l) for l in open(log)] if os.path.exists(log) else []
    return {
        "exit": str(code),
        "stdout": stdout,
        "stderr": compare.normalize(err.text(), home),
        "requests": "\n".join(json.dumps(r, sort_keys=True) for r in reqs),
        "tcp": "\n".join(transcript),
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("scenario", nargs="?", default=os.path.join(PARITY_DIR, "scenarios", "port-forward_ws.json"))
    ap.add_argument("--only")
    ap.add_argument("-v", "--verbose", action="store_true")
    a = ap.parse_args()
    sc = json.load(open(a.scenario))
    wsdir = tempfile.mkdtemp(prefix="pf-ws-")
    wsmock, ws = start_ws_mock(wsdir)
    ok = total = 0
    try:
        for case in sc["cases"]:
            if a.only and a.only not in case["name"]:
                continue
            total += 1
            go = run_one(compare.GO_BIN, sc, case, ws)
            rs = run_one(compare.RUST_BIN, sc, case, ws)
            ignore = set(case.get("ignore") or [])
            diffs = []
            for key in ("exit", "stdout", "stderr", "requests", "tcp"):
                if key not in ignore and go[key] != rs[key]:
                    d = difflib.unified_diff(compare.show(go[key]).splitlines(), compare.show(rs[key]).splitlines(), "go/" + key, "rust/" + key, lineterm="", n=2)
                    diffs.append("\n".join(d))
            if diffs:
                print("DIFF  %s   [qovery %s]" % (case["name"], " ".join(case["args"])))
                for d in diffs:
                    print("\n".join("      " + l for l in d.splitlines()))
            else:
                ok += 1
                print("SAME  %s" % case["name"])
                if a.verbose:
                    for key in ("stdout", "stderr", "tcp"):
                        print("      %s: %s" % (key, compare.show(go[key]).replace("\n", "\n              ")))
    finally:
        wsmock.terminate()
        wsmock.wait()
        shutil.rmtree(compare.SANDBOX, ignore_errors=True)
    print("\n%d/%d identical" % (ok, total))
    sys.exit(0 if ok == total else 1)


if __name__ == "__main__":
    main()
