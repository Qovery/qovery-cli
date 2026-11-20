#!/usr/bin/env python3
"""Compare the websocket sessions of `qovery shell` / `list-pods` between the Go and Rust CLIs.

Starts parity/shell_ws_mock.py, replaces "$WS" in the scenario file (default parity/scenarios/shell_ws.json) with
ws://127.0.0.1:<ws mock port>, then runs parity/compare.py on it.

Usage: shell_ws_compare.py [scenario.json] [compare.py options, e.g. -v --only NAME]
"""

import json
import os
import subprocess
import sys
import tempfile
import time

PARITY_DIR = os.path.dirname(os.path.abspath(__file__))


def start_ws_mock(work):
    portfile = os.path.join(work, "ws-port")
    mock = subprocess.Popen([sys.executable, "-I", os.path.join(PARITY_DIR, "shell_ws_mock.py"), portfile])
    for _ in range(200):
        if os.path.exists(portfile) and open(portfile).read().strip():
            return mock, "ws://127.0.0.1:%s" % open(portfile).read().strip()
        time.sleep(0.02)
    # never leave the mock running when it did not start properly
    mock.terminate()
    mock.wait()
    raise RuntimeError("the websocket mock did not write its port to " + portfile)


def main():
    args = sys.argv[1:]
    scenario = os.path.join(PARITY_DIR, "scenarios", "shell_ws.json")
    if args and args[0].endswith(".json"):
        scenario, args = args[0], args[1:]
    work = tempfile.mkdtemp(prefix="shell-ws-")
    mock, ws = start_ws_mock(work)
    try:
        text = open(scenario).read().replace("$WS", ws)
        tmp = os.path.join(work, os.path.basename(scenario))
        with open(tmp, "w") as f:
            f.write(text)
        json.load(open(tmp))
        rc = subprocess.call([sys.executable, os.path.join(PARITY_DIR, "compare.py"), tmp] + args)
    finally:
        mock.terminate()
        mock.wait()
    sys.exit(rc)


if __name__ == "__main__":
    main()
