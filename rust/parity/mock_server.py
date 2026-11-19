#!/usr/bin/env python3
"""Fixture driven mock of the Qovery API, used to compare the Go and Rust CLIs.

Usage: mock_server.py <fixtures.json> <request-log.jsonl> <port-file>

fixtures.json is a list of routes:
  {"method": "GET", "path": "/organization", "status": 200, "body": {...}}
  {"method": "GET", "path_regex": "^/environment/[^/]+/status$", "responses": [{"status": 200, "body": ...}, ...]}
  {"method": "POST", "path": "/x", "query": {"k": "v"}, "status": 202, "body_raw": "text", "headers": {"Content-Type": "text/plain"}}

* `responses` is a sequence: each matching request consumes the next one, the last one repeats.
* Routes are tried in order; the first match wins. Unmatched requests get a 404 and are logged with "unmatched": true.
* Every request is appended to the request log as one JSON line: method, path, query, body.
"""

import json
import re
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

FIXTURES = json.load(open(sys.argv[1]))
LOG = open(sys.argv[2], "a", buffering=1)
LOCK = threading.Lock()
COUNTERS = {}


def match(route, method, path, query):
    if route.get("method", "GET").upper() != method:
        return False
    if "path" in route and route["path"] != path:
        return False
    if "path_regex" in route and not re.search(route["path_regex"], path):
        return False
    for k, v in (route.get("query") or {}).items():
        if query.get(k) != ([v] if isinstance(v, str) else v):
            return False
    return True


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *args):
        pass

    def handle_any(self):
        url = urlsplit(self.path)
        query = parse_qs(url.query, keep_blank_values=True)
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        try:
            body = json.loads(raw) if raw else None
        except Exception:
            body = raw.decode("utf-8", "replace")
        entry = {"method": self.command, "path": url.path, "query": {k: sorted(v) for k, v in sorted(query.items())}, "body": body}
        response = None
        with LOCK:
            for i, route in enumerate(FIXTURES):
                if match(route, self.command, url.path, query):
                    if "responses" in route:
                        n = COUNTERS.get(i, 0)
                        COUNTERS[i] = n + 1
                        response = route["responses"][min(n, len(route["responses"]) - 1)]
                    else:
                        response = route
                    break
            if response is None:
                entry["unmatched"] = True
            LOG.write(json.dumps(entry, sort_keys=True) + "\n")
        if response is None:
            response = {"status": 404, "body": {"status": 404, "error": "no fixture for " + self.command + " " + url.path}}
        status = response.get("status", 200)
        headers = dict(response.get("headers") or {})
        if "body_raw" in response:
            payload = response["body_raw"].encode()
            headers.setdefault("Content-Type", "text/plain")
        elif "body" in response:
            payload = json.dumps(response["body"]).encode()
            headers.setdefault("Content-Type", "application/json")
        else:
            payload = b""
        self.send_response(status)
        for k, v in headers.items():
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(payload)

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = handle_any


class Server(ThreadingHTTPServer):
    daemon_threads = True

    def handle_error(self, request, client_address):
        pass  # clients closing the connection early (e.g. on exit) are not errors here


server = Server(("127.0.0.1", 0), Handler)
with open(sys.argv[3], "w") as f:
    f.write(str(server.server_address[1]))
server.serve_forever()
