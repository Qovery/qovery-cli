#!/usr/bin/env python3
"""Minimal scripted websocket server used to compare `qovery shell`, `port-forward` and `list-pods` (Go vs Rust).

Usage: shell_ws_mock.py <port-file>

The script is chosen by the `pod_name` query parameter of the upgrade request (`--pod <key>`), or the `service`
parameter when there is no pod name. SCRIPTS[key] is a list of connection scripts: the n-th connection of a
client (clients are told apart by their User-Agent, i.e. Go vs Rust) runs the n-th one, the last one repeats.
Every upgrade response carries `X-Qovery-Request-Id: req-<n>`.

Steps: ("reject", status) | ("echo_request",) | ("binary", bytes) | ("text", str) | ("ping",)
| ("read_until_eot",)  echo each binary frame received as `recv:<repr>` until a frame ending with 0x04
| ("echo",)            echo binary frames back until the client goes away (port-forward)
| ("close", code or None, reason) | ("drop",) | ("sleep", seconds)
"""

import base64
import hashlib
import json
import socket
import struct
import sys
import threading
import time
from urllib.parse import parse_qs, unquote, urlsplit

GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

PODS = json.dumps({"pods": [{"name": "app-1-pod-b", "ports": [8080, 80, 443]}, {"name": "app-1-pod-a", "ports": []}, {"name": "app-1-pod-c"}]})

SCRIPTS = {
    # shell
    "basic": [[("echo_request",), ("binary", b"$ "), ("read_until_eot",), ("binary", b"bye\r\n"), ("close", 1000, "")]],
    "text-ignored": [[("text", "ignored text"), ("binary", b"visible\n"), ("read_until_eot",), ("close", 1000, "done")]],
    "policy": [[("binary", b"partial"), ("close", 1008, "invalid token")]],
    "permission": [[("close", 1007, "permission denied")]],
    "agent-timeout": [[("close", 1011, "Deadline of 90s exceeded for receiving agent response")], [("echo_request",), ("read_until_eot",), ("close", 1000, "")]],
    "internal": [[("close", 1011, "boom")], [("read_until_eot",), ("close", 1000, "")]],
    "custom": [[("close", 4000, "custom")], [("close", 1000, "")]],
    "nostatus": [[("close", None, "")], [("close", 1000, "")]],
    "drop": [[("binary", b"before drop\n"), ("drop",)], [("echo_request",), ("close", 1000, "")]],
    "reject": [[("reject", 403)], [("echo_request",), ("close", 1000, "")]],
    "reject-500": [[("reject", 500)], [("close", 1008, "")]],
    "ping": [[("ping",), ("binary", b"after ping\n"), ("read_until_eot",), ("close", 1000, "")]],
    "big": [[("binary", b"x" * 70000 + b"\n"), ("read_until_eot",), ("close", 1000, "")]],
    # list-pods
    "pods": [[("echo_request_text",), ("close", 1000, "")]],
    "pods-list": [[("text", PODS), ("close", 1000, "")]],
    "pods-empty": [[("text", '{"pods": []}'), ("close", 1000, "")]],
    "pods-null": [[("text", "null"), ("close", 1000, "")]],
    "pods-case": [[("text", '{"PODS": [{"NAME": "p", "Ports": [1]}], "other": 1}'), ("close", 1000, "")]],
    "pods-binary": [[("binary", b"not text"), ("close", 1000, "")]],
    "pods-badjson": [[("text", "not json"), ("close", 1000, "")]],
    "pods-truncated": [[("text", '{"pods": ['), ("close", 1000, "")]],
    "pods-badport": [[("text", '{"pods": [{"name": "p", "ports": [70000]}]}'), ("close", 1000, "")]],
    "pods-badtype": [[("text", '{"pods": [{"name": 1, "ports": [1]}]}'), ("close", 1000, "")]],
    "pods-badport-case": [[("text", '{"Pods": [{"name": "p", "Ports": [1, -1]}]}'), ("close", 1000, "")]],
    "pods-floatport": [[("text", '{"pods": [{"ports": [1.5]}]}'), ("close", 1000, "")]],
    "pods-stringport": [[("text", '{"pods": [{"name": "a"}, {"ports": [80, "x"]}]}'), ("close", 1000, "")]],
    "pods-ports-object": [[("text", '{"pods": [{"ports": {"a": 1}}]}'), ("close", 1000, "")]],
    "pods-string": [[("text", '{"pods": "x"}'), ("close", 1000, "")]],
    "pods-pod-string": [[("text", '{"pods": ["x", {"name": 2}]}'), ("close", 1000, "")]],
    "pods-name-array": [[("text", '{"pods": [{"NAME": [1]}]}'), ("close", 1000, "")]],
    "pods-array": [[("text", '[1]'), ("close", 1000, "")]],
    "pods-two-errors": [[("text", '{"pods": [{"name": true, "ports": [99999]}]}'), ("close", 1000, "")]],
    "pods-dup": [[("text", '{"pods": [{"name": "a"}], "PODS": [{"name": "b"}]}'), ("close", 1000, "")]],
    "pods-null-items": [[("text", '{"pods": [null, {"name": null, "ports": [null, 3]}]}'), ("close", 1000, "")]],
    "pods-close": [[("close", 1008, "denied")]],
    "pods-drop": [[("drop",)]],
    # port-forward
    "pf-echo": [[("echo_request",), ("echo",)]],
    "pf-big": [[("binary", b"y" * 70000), ("binary", b"z" * 10), ("close", 1000, "")]],
    "pf-banner": [[("binary", b"banner\n"), ("echo",)]],
    "pf-policy": [[("close", 1008, "invalid token")]],
    "pf-permission": [[("close", 1007, "permission denied")]],
    "pf-timeout": [[("close", 1011, "Timed out after 45s while setting up port forward")]],
    "pf-internal": [[("close", 1011, "boom")]],
    "pf-normal": [[("binary", b"one shot\n"), ("close", 1000, "")]],
    "pf-custom": [[("close", 4001, "custom")]],
    "pf-drop": [[("binary", b"x\n"), ("drop",)]],
    "pf-reject": [[("reject", 401)]],
    "pf-text": [[("text", "ignored"), ("binary", b"bin\n"), ("echo",)]],
}

COUNTERS = {}
LOCK = threading.Lock()


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


def recv_exact(conn, n):
    data = b""
    while len(data) < n:
        chunk = conn.recv(n - len(data))
        if not chunk:
            raise EOFError()
        data += chunk
    return data


def read_frame(conn):
    b0, b1 = recv_exact(conn, 2)
    opcode = b0 & 0x0F
    n = b1 & 0x7F
    if n == 126:
        n = struct.unpack("!H", recv_exact(conn, 2))[0]
    elif n == 127:
        n = struct.unpack("!Q", recv_exact(conn, 8))[0]
    mask = recv_exact(conn, 4) if b1 & 0x80 else b"\0\0\0\0"
    payload = bytearray(recv_exact(conn, n))
    for i in range(n):
        payload[i] ^= mask[i % 4]
    return opcode, bytes(payload)


def read_data_frame(conn):
    """Next binary/text frame (answers pings, ignores pongs). None on close/EOF."""
    while True:
        opcode, payload = read_frame(conn)
        if opcode == 0x9:
            conn.sendall(frame(0xA, payload))
            continue
        if opcode == 0xA:
            continue
        if opcode == 0x8:
            return None
        return opcode, payload


def read_request(conn):
    data = b""
    while b"\r\n\r\n" not in data:
        chunk = conn.recv(4096)
        if not chunk:
            return None, {}
        data += chunk
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
        qs = parse_qs(urlsplit(target).query)
        key = qs.get("pod_name", qs.get("service", [""]))[0]
        client = "go" if "Go-http-client" in headers.get("user-agent", "") else "other"
        with LOCK:
            n = COUNTERS.get((key, client), 0)
            COUNTERS[(key, client)] = n + 1
        scripts = SCRIPTS.get(key, [[("echo_request",), ("close", 1000, "")]])
        script = scripts[min(n, len(scripts) - 1)]
        request_id = "req-%d" % (n + 1)
        if script and script[0][0] == "reject":
            body = b'{"message": "rejected"}'
            conn.sendall(
                b"HTTP/1.1 %d Rejected\r\nX-Qovery-Request-Id: %s\r\nContent-Length: %d\r\nContent-Type: application/json\r\n\r\n%s"
                % (script[0][1], request_id.encode(), len(body), body)
            )
            drain(conn, 0.5)
            return
        accept = base64.b64encode(hashlib.sha1((headers.get("sec-websocket-key", "") + GUID).encode()).digest()).decode()
        conn.sendall(
            (
                "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
                "Sec-WebSocket-Accept: %s\r\nX-Qovery-Request-Id: %s\r\n\r\n" % (accept, request_id)
            ).encode()
        )
        described = "url=%s auth=%s\r\n" % (unquote(target), headers.get("authorization", ""))
        for step in script:
            kind = step[0]
            if kind == "echo_request":
                conn.sendall(frame(0x2, described.encode()))
            elif kind == "echo_request_text":
                conn.sendall(frame(0x1, json.dumps({"pods": [{"name": described.strip(), "ports": [1]}]}).encode()))
            elif kind == "text":
                conn.sendall(frame(0x1, step[1].encode()))
            elif kind == "binary":
                conn.sendall(frame(0x2, step[1]))
            elif kind == "ping":
                conn.sendall(frame(0x9, b"hi"))
            elif kind == "read_until_eot":
                while True:
                    f = read_data_frame(conn)
                    if f is None:
                        return
                    conn.sendall(frame(0x2, ("recv:%r\r\n" % f[1]).encode()))
                    if f[1].endswith(b"\x04"):
                        break
            elif kind == "echo":
                while True:
                    try:
                        f = read_data_frame(conn)
                    except (EOFError, OSError):
                        return
                    if f is None:
                        return
                    conn.sendall(frame(0x2, b"echo:" + f[1]))
            elif kind == "sleep":
                time.sleep(step[1])
            elif kind == "drop":
                time.sleep(0.2)
                conn.shutdown(socket.SHUT_RDWR)
                return
            elif kind == "close":
                code, reason = step[1], step[2]
                payload = b"" if code is None else struct.pack("!H", code) + reason.encode()
                time.sleep(0.1)
                conn.sendall(frame(0x8, payload))
                drain(conn)
                return
            time.sleep(0.05)
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
