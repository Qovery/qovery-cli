#!/usr/bin/env python3
"""Generates parity/scenarios/env.json.

Usage: python3 parity/generators/env.py [OUTPUT.json]   (default output next to the scenarios, from any working directory;
an explicit OUTPUT.json is relative to the current working directory)
"""
import os
import sys

PARITY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # rust/parity
RUST = os.path.dirname(PARITY)
import json
out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(PARITY, "scenarios", "env.json")
def c(name, args, **kw):
    d = {"name": name, "args": args}
    d.update(kw)
    return d
GOOD = "# comment\nexport FOO=bar\nBAZ=\"multi\\nline\"\nQUX='single $FOO'\nEXP=${FOO}-x # trailing comment\n"
J = ["env", "parse", "-j"]
cases = [
    c("env (group)", ["env"]),
    c("env unknown sub", ["env", "foo"]),
    c("env import too many args", ["env", "import", "a.env", "b.env"]),
    c("env import too many args --sort", ["env", "import", "--sort", "a.env", "b.env", "c"]),
    c("env import missing file", ["env", "import", "nope.env"]),
    c("env import directory", ["env", "import", ".qovery"]),
    c("env import bad key", ["env", "import", "bad.env"], files={"bad.env": "GOOD=1\nfoo-bar=1\n"}),
    c("env import key without value at EOL", ["env", "import", "bad.env"], files={"bad.env": "FOO\nBAR=1\n"}),
    c("env import unterminated quote", ["env", "import", "bad.env"], files={"bad.env": "A=1\nB=\"abc\nC=2\n"}),
    c("env import unicode key error", ["env", "import", "bad.env"], files={"bad.env": "CLÉ-X=1\n"}),
    c("env import database service", ["env", "import", ".env"], files={".env": GOOD},
      context={"service_id": "db-1", "service_name": "my-db", "service_type": "database"}),
    c("env import job service", ["env", "import", ".env"], files={".env": GOOD},
      context={"service_id": "cron-1", "service_name": "my-cron", "service_type": "job"}),
    c("env import no context file", ["env", "import", ".env"], files={".env": GOOD}, no_context_file=True),
    # interactive (survey prompt fails on a non-tty stdin): only exit code / requests are comparable
    c("env import application (survey prompt)", ["env", "import", ".env"], files={".env": GOOD}, ignore=["stdout"]),
    c("env import container (survey prompt)", ["env", "import", "--sort", ".env"], files={".env": GOOD},
      context={"service_id": "cont-1", "service_name": "my-container", "service_type": "container"}, ignore=["stdout"]),
    c("env import empty file (survey prompt)", ["env", "import", "empty.env"], files={"empty.env": ""}, ignore=["stdout"]),
    c("env import no args, no .env file", ["env", "import"]),
    c("env import no args (promptui select)", ["env", "import"], files={"app/.env": GOOD, "app/.env.local": "A=1\n"}),
    c("env parse without option", ["env", "parse"]),
    c("env parse without option with arg", ["env", "parse", "file"]),
    c("env parse too many args", ["env", "parse", "-j", "a", "b"]),
    c("env parse heroku json", J, stdin='{"DATABASE_URL":"postgres://u:p@h/db"}'),
    c("env parse heroku json long flag + arg", ["env", "parse", "--heroku-json", "ignored"], stdin='{"A":"1"}\n'),
    c("env parse heroku json null value", J, stdin='{"A":null}'),
    c("env parse heroku json escapes", J, stdin='{"A":"x\\ny\\u00e9\\"q"}'),
    c("env parse heroku json duplicate key", J, stdin='{"A":"1","A":"2"}'),
    c("env parse heroku json trailing data", J, stdin='{"A":"1"} garbage'),
    c("env parse heroku json null", J, stdin='null'),
    c("env parse heroku json empty object", J, stdin='{}'),
    c("env parse heroku json empty stdin", J, stdin=''),
    c("env parse heroku json whitespace stdin", J, stdin='  \n'),
    c("env parse heroku json truncated", J, stdin='{"A":"1"'),
    c("env parse heroku json truncated literal", J, stdin='tru'),
    c("env parse heroku json syntax error", J, stdin='{"A" "1"}'),
    c("env parse heroku json invalid", J, stdin='hello'),
    c("env parse heroku json trailing comma", J, stdin='{"A":"1",}'),
    c("env parse heroku json number value", J, stdin='{"A":1}'),
    c("env parse heroku json bool then number", J, stdin='{"A":true,"B":2}'),
    c("env parse heroku json object value", J, stdin='{"A":{"b":"c"}}'),
    c("env parse heroku json array", J, stdin='["a"]'),
    c("env parse heroku json string", J, stdin='"a"'),
    c("env parse heroku json number", J, stdin='12'),
    c("env parse heroku json control char", J, stdin='{"A":"a\tb"}'),
    c("env parse heroku json invalid escape", J, stdin='{"A":"\\x"}'),
    c("env parse heroku json invalid unicode escape", J, stdin='{"A":"\\u12g4"}'),
    c("env parse heroku json lone surrogate", J, stdin='{"A":"\\ud800x"}'),
    c("env parse heroku json bad number", J, stdin='{"A":1.}'),
    c("env parse heroku json BOM", J, stdin='\ufeff{"A":"1"}'),
    c("env parse heroku json bad literal", J, stdin='{"A":nul}'),
    c("env parse unknown flag", ["env", "parse", "--nope"]),
]
json.dump({"cases": cases}, open(out, "w"), indent=1)
