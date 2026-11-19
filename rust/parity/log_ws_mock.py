#!/usr/bin/env python3
"""Minimal websocket server scripted per connection, used to compare `qovery log` streaming (Go vs Rust).

Usage: log_ws_mock.py <port-file>

The behaviour is chosen by the `service` query parameter of the upgrade request (`qovery log --service-id <name>`):
see SCRIPTS below. Each step is ("text", str) | ("binary", bytes) | ("ping", bytes) | ("frag", [str, ...])
| ("close", code or None, reason) | ("drop",) | ("echo",) | ("sleep", seconds).
`text:<percent-encoded payload>` sends that single text message.
`reject:<status>` answers the upgrade request with that HTTP status.
"""

import base64
import hashlib
import socket
import struct
import sys
import threading
import time
from urllib.parse import parse_qs, urlsplit

GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"


def log(created_at, message, pod="app-1-pod-abc", version="v1"):
    import json

    d = {}
    if created_at is not None:
        d["created_at"] = created_at
    if message is not None:
        d["message"] = message
    if version is not None:
        d["version"] = version
    if pod is not None:
        d["pod_name"] = pod
    return json.dumps(d)


SCRIPTS = {
    "basic": [
        ("text", log(1700000000123, "Starting server on :8080")),
        ("text", log(1700000001000, "GET /health 200", pod="app-1-pod-def")),
        ("text", '{"message": "only a message"}'),
        ("text", '{"MESSAGE": "case insensitive key", "Pod_Name": "pod-x", "unknown": [1, 2]}'),
        ("text", "null"),
        ("text", log(0, "epoch", pod="")),
        ("text", log(-1500, "before epoch")),
        ("text", '{"created_at": null, "message": "null timestamp"}'),
        ("text", log(1700000002000, "unicode: éà ☃ \"quoted\" <tag> & \\back")),
        ("close", 1000, ""),
    ],
    "binary": [("binary", log(1700000000123, "from a binary frame").encode()), ("close", 1000, "bye")],
    "ping": [("ping", b"hi"), ("text", log(1700000000123, "after ping")), ("close", 1001, "")],
    "fragmented": [("frag", ['{"created_at": 1700000000', '123, "message": "frag', 'mented", "pod_name": "p"}']), ("close", 1000, "")],
    "close-policy": [("text", log(1700000000123, "one")), ("close", 1008, "invalid token")],
    "close-permission": [("close", 1007, "permission denied")],
    "close-internal": [("close", 1011, "Deadline of 90s exceeded for receiving agent response")],
    "close-custom": [("close", 4000, "custom code")],
    "close-nostatus": [("text", log(1700000000123, "one")), ("close", None, "")],
    "drop": [("text", log(1700000000123, "one")), ("drop",)],
    "bad-json": [("text", log(1700000000123, "one")), ("text", "not json")],
    "truncated-json": [("text", '{"message": "x"')],
    "bad-ts": [("text", log(1700000000123, "one")), ("text", '{"message": "two", "created_at": "yesterday"}')],
    "bad-ts-float": [("text", '{"created_at": 1.5, "message": "x"}')],
    "bad-ts-bool": [("text", '{"created_at": true}')],
    "type-error": [("text", '{"message": 12, "pod_name": "p"}')],
    "type-error-then-ts": [("text", '{"message": 12, "created_at": "x"}')],
    "array": [("text", "[1, 2]")],
    "string": [("text", '"hello"')],
    "number": [("text", "42")],
    "syntax-errors": [("text", '{"message": "x" "y"}')],
    "syntax-top-level": [("text", '{"message": "x"} {}')],
    "syntax-escape": [("text", '{"message": "\\q"}')],
    "syntax-number": [("text", '{"created_at": 1.e5}')],
    "syntax-quote": [("text", "{'message': 1}")],
    "dup-ts-errors": [("text", '{"created_at": "a", "Created_At": false, "message": 1}')],
    "echo": [("echo",), ("close", 1000, "")],
    "empty": [("close", 1000, "")],
}


def frame(opcode, payload, fin=True):
    head = bytes([(0x80 if fin else 0) | opcode])
    n = len(payload)
    if n < 126:
        head += bytes([n])
    elif n < 65536:
        head += bytes([126]) + struct.pack("!H", n)
    else:
        head += bytes([127]) + struct.pack("!Q", n)
    return head + payload


def read_request(conn):
    data = b""
    while b"\r\n\r\n" not in data:
        chunk = conn.recv(4096)
        if not chunk:
            return None, {}
        data += chunk
        if data[:1] == b"\x16":
            # TLS ClientHello (wss:// URL): answer like a plain HTTP server would
            conn.sendall(b"HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
            return None, {}
    lines = data.split(b"\r\n\r\n")[0].decode("latin-1").split("\r\n")
    headers = {}
    for line in lines[1:]:
        if ":" in line:
            k, v = line.split(":", 1)
            headers[k.strip().lower()] = v.strip()
    return lines[0], headers


def drain(conn, seconds=2.0):
    conn.settimeout(seconds)
    try:
        while conn.recv(4096):
            pass
    except Exception:
        pass


def handle(conn):
    try:
        request_line, headers = read_request(conn)
        if request_line is None:
            return
        target = request_line.split(" ")[1]
        service = parse_qs(urlsplit(target).query).get("service", [""])[0]
        if service.startswith("reject:"):
            status = int(service.split(":")[1])
            body = b'{"message": "rejected"}'
            conn.sendall(b"HTTP/1.1 %d Rejected\r\nContent-Length: %d\r\nContent-Type: application/json\r\n\r\n%s" % (status, len(body), body))
            drain(conn, 0.5)
            return
        accept = base64.b64encode(hashlib.sha1((headers.get("sec-websocket-key", "") + GUID).encode()).digest()).decode()
        conn.sendall(
            ("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n" % accept).encode()
        )
        script = SCRIPTS.get(service, SCRIPTS["empty"])
        if service.startswith("text:"):
            # text:<percent-encoded payload>: send that single text message, then wait for the client
            script = [("text", service[len("text:"):])]
        for step in script:
            kind = step[0]
            if kind == "text":
                conn.sendall(frame(0x1, step[1].encode()))
            elif kind == "binary":
                conn.sendall(frame(0x2, step[1]))
            elif kind == "ping":
                conn.sendall(frame(0x9, step[1]))
            elif kind == "frag":
                parts = step[1]
                for i, p in enumerate(parts):
                    conn.sendall(frame(0x1 if i == 0 else 0x0, p.encode(), fin=(i == len(parts) - 1)))
            elif kind == "echo":
                msg = "%s | authorization=%s" % (request_line, headers.get("authorization", ""))
                conn.sendall(frame(0x1, msg.encode()))
            elif kind == "sleep":
                time.sleep(step[1])
            elif kind == "drop":
                time.sleep(0.2)
                conn.shutdown(socket.SHUT_RDWR)
                return
            elif kind == "close":
                code, reason = step[1], step[2]
                payload = b"" if code is None else struct.pack("!H", code) + reason.encode()
                conn.sendall(frame(0x8, payload))
                drain(conn)
                return
            time.sleep(0.05)
        # script without close: wait for the client to go away
        drain(conn, 5.0)
    except Exception as e:  # noqa: BLE001
        print("ws mock error: %r" % e, file=sys.stderr)
    finally:
        try:
            conn.close()
        except Exception:
            pass


def main():
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("127.0.0.1", 0))
    srv.listen(16)
    with open(sys.argv[1], "w") as f:
        f.write(str(srv.getsockname()[1]))
    while True:
        conn, _ = srv.accept()
        threading.Thread(target=handle, args=(conn,), daemon=True).start()


if __name__ == "__main__":
    main()
