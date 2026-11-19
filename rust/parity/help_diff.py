#!/usr/bin/env python3
"""Compare the command-line interface of the Go and Rust CLIs, for every command:

* the command spec (use, short, long, example, aliases, hidden, flags with type/default/usage/shorthand/required)
  dumped by both binaries with QOVERY_DUMP_SPEC=1
* the output of `<command> --help`

Usage: help_diff.py [--prefix "application env"] [--no-help] [-q]
"""
import argparse, difflib, json, os, subprocess, sys, tempfile

D = os.path.dirname(os.path.abspath(__file__))
GO = os.environ.get("QOVERY_GO_BIN", os.path.join(D, "..", "target", "qovery-go"))
RS = os.environ.get("QOVERY_RUST_BIN", os.path.join(D, "..", "target", "debug", "qovery"))
HOME = tempfile.mkdtemp(prefix="qovery-help-")
ENV = {"PATH": os.environ["PATH"], "HOME": HOME, "QOVERY_TELEMETRY": "false", "TERM": "xterm"}


def spec(binary):
    out = subprocess.run([binary], env={**ENV, "QOVERY_DUMP_SPEC": "1"}, capture_output=True, text=True).stdout
    flat = {}

    def walk(c):
        c = dict(c)
        children = c.pop("Children", None) or []
        c["Flags"] = sorted(c.get("Flags") or [], key=lambda f: (f["Persistent"], f["Name"]))
        c["Aliases"] = c.get("Aliases") or []
        flat[c["Path"]] = c
        for ch in children:
            walk(ch)

    walk(json.loads(out))
    return flat


def run(binary, args):
    p = subprocess.run([binary] + args, env=ENV, capture_output=True, text=True, timeout=30)
    return "exit=%d\n--stdout--\n%s--stderr--\n%s" % (p.returncode, p.stdout, p.stderr)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--prefix", default="")
    ap.add_argument("--no-help", action="store_true")
    ap.add_argument("-q", action="store_true", help="only print the summary")
    a = ap.parse_args()
    go, rs = spec(GO), spec(RS)
    prefix = ("qovery " + a.prefix).strip()
    paths = sorted(p for p in go if p == prefix or p.startswith(prefix + " "))
    extra = sorted(p for p in rs if (p == prefix or p.startswith(prefix + " ")) and p not in go)
    bad = 0
    for p in paths:
        problems = []
        if p not in rs:
            problems.append("missing in rust")
        else:
            for k in sorted(set(go[p]) | set(rs[p])):
                if go[p].get(k) != rs[p].get(k):
                    gv = json.dumps(go[p].get(k), indent=1, sort_keys=True).splitlines()
                    rv = json.dumps(rs[p].get(k), indent=1, sort_keys=True).splitlines()
                    problems.append("spec %s:\n%s" % (k, "\n".join(difflib.unified_diff(gv, rv, "go", "rust", lineterm="", n=1))))
            if not a.no_help:
                args = p.split()[1:] + ["--help"]
                g, r = run(GO, args), run(RS, args)
                if g != r:
                    problems.append("--help output:\n" + "\n".join(difflib.unified_diff(g.splitlines(), r.splitlines(), "go", "rust", lineterm="", n=1)))
        if problems:
            bad += 1
            print("DIFF  " + p)
            if not a.q:
                for pr in problems:
                    print("\n".join("      " + l for l in pr.splitlines()))
    for p in extra:
        bad += 1
        print("EXTRA " + p + " (not in Go)")
    print("\n%d/%d commands identical" % (len(paths) - (bad - len(extra)), len(paths)))
    sys.exit(1 if bad else 0)


main()
