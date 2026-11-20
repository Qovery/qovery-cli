#!/usr/bin/env python3
"""Print the required JSON properties of Go client models (the Go CLI fails to decode a response lacking them).

Usage: required_fields.py model_name [model_name ...]    e.g. required_fields.py application container_response
       required_fields.py --grep cron                     list model names containing 'cron'
"""
import glob, os, re, subprocess, sys

modcache = subprocess.run(["go", "env", "GOMODCACHE"], capture_output=True, text=True).stdout.strip()
roots = sorted(glob.glob(os.path.join(modcache, "github.com/qovery/qovery-client-go@*")))
root = [r for r in roots if "20261030144809" in r] or roots
root = root[-1]
if len(sys.argv) > 2 and sys.argv[1] == "--grep":
    for f in sorted(glob.glob(os.path.join(root, "model_*%s*.go" % sys.argv[2]))):
        print(os.path.basename(f)[6:-3])
    sys.exit(0)
for m in sys.argv[1:]:
    f = os.path.join(root, "model_%s.go" % m)
    if not os.path.exists(f):
        print("%s: no such model" % m)
        continue
    src = open(f).read()
    mm = re.search(r"requiredProperties := \[\]string\{(.*?)\}", src, re.S)
    props = re.findall(r'"([^"]+)"', mm.group(1)) if mm else []
    print("%s: %s" % (m, ", ".join(props)))
print("(client source: %s)" % root, file=sys.stderr)
