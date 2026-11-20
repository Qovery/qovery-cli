#!/usr/bin/env python3
"""Compare `qovery log` websocket streaming between the Go and Rust CLIs.

Starts parity/log_ws_mock.py, replaces "$WS" ("$WSS" for a wss:// URL) in the env values of the scenario file (default
parity/scenarios/log_ws.json) with ws://127.0.0.1:<ws mock port>, then runs parity/compare.py on it.

Usage: log_ws_compare.py [scenario.json] [compare.py options, e.g. -v --only NAME]
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import time

PARITY_DIR = os.path.dirname(os.path.abspath(__file__))


def main():
    args = sys.argv[1:]
    scenario = os.path.join(PARITY_DIR, "scenarios", "log_ws.json")
    if args and args[0].endswith(".json"):
        scenario, args = args[0], args[1:]
    work = tempfile.mkdtemp(prefix="log-ws-")
    portfile = os.path.join(work, "port")
    mock = subprocess.Popen([sys.executable, "-I", os.path.join(PARITY_DIR, "log_ws_mock.py"), portfile])
    try:
        for _ in range(200):
            if os.path.exists(portfile) and open(portfile).read().strip():
                break
            time.sleep(0.02)
        ws = "ws://127.0.0.1:%s" % open(portfile).read().strip()
        text = open(scenario).read().replace("$WSS", ws.replace("ws://", "wss://")).replace("$WS", ws)
        tmp = os.path.join(work, os.path.basename(scenario))
        with open(tmp, "w") as f:
            f.write(text)
        json.load(open(tmp))
        rc = subprocess.call([sys.executable, os.path.join(PARITY_DIR, "compare.py"), tmp] + args)
    finally:
        mock.terminate()
        mock.wait()
        shutil.rmtree(work, ignore_errors=True)
    sys.exit(rc)


if __name__ == "__main__":
    main()
