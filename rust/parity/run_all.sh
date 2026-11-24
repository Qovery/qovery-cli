#!/usr/bin/env bash
# Whole Go-vs-Rust parity suite: help/spec comparison, then every scenario file. Prints one line per file and the
# drifting cases; exits 1 on any drift. Used by the CI (.github/workflows/build.yml).
#
#   cd rust && QOVERY_GO_BIN=... QOVERY_RUST_BIN=... parity/run_all.sh
#
# PARITY_JOBS:    scenario files run in parallel (default: number of CPUs)
# PARITY_RETRIES: re-runs of a differing case before it counts as a drift (default 2; a few Go outputs are racy)
set -uo pipefail
cd "$(dirname "$0")/.."

jobs="${PARITY_JOBS:-$(nproc 2>/dev/null || sysctl -n hw.ncpu)}"
retries="${PARITY_RETRIES:-2}"
logs="$(mktemp -d)"
trap 'rm -rf "$logs"' EXIT
export logs retries

# scenarios run on other machines (CI): a path of the machine that generated them would only exist there
if grep -lE '"/(home|Users)/|[" :]/(home|Users)/[a-z]' parity/scenarios/*.json parity/fixtures/*.json; then
    echo "machine-specific paths in the files above: use \$PARITY (see parity/compare.py) instead" >&2
    exit 1
fi

run_scenario() { # <scenario file>
    local file="$1" name
    name="$(basename "$file" .json)"
    local cmd=(python3 parity/compare.py "$file" --retries "$retries")
    case "$name" in
    log_ws) cmd=(python3 parity/log_ws_compare.py "$file" --retries "$retries") ;;
    shell_ws | list-pods_ws) cmd=(python3 parity/shell_ws_compare.py "$file" --retries "$retries") ;;
    port-forward_ws) cmd=(python3 parity/port-forward_ws_compare.py "$file" --retries "$retries") ;;
    esac
    local result=ok
    "${cmd[@]}" >"$logs/$name.log" 2>&1 || result=DRIFT
    printf '%-24s %-5s %s\n' "$name" "$result" "$(tail -n 1 "$logs/$name.log")"
    [ "$result" = ok ] || touch "$logs/$name.failed"
}
export -f run_scenario

python3 parity/help_diff.py >"$logs/help_diff.log" 2>&1 || touch "$logs/help_diff.failed"
printf '%-24s %-5s %s\n' help_diff "$([ -e "$logs/help_diff.failed" ] && echo DRIFT || echo ok)" "$(tail -n 1 "$logs/help_diff.log")"

# HTTP scenarios in parallel (each run has its own sandbox and mock port), websocket ones one at a time
# (port-forward cases listen on local ports)
find parity/scenarios -name '*.json' ! -name '*_ws.json' | sort |
    xargs -P "$jobs" -I{} bash -c 'run_scenario "$1"' _ {}
for file in $(find parity/scenarios -name '*_ws.json' | sort); do
    run_scenario "$file"
done

failed=("$logs"/*.failed)
if [ ! -e "${failed[0]}" ]; then
    echo "no drift"
    exit 0
fi
for marker in "${failed[@]}"; do
    name="$(basename "$marker" .failed)"
    echo
    echo "=== $name"
    grep -v '^SAME ' "$logs/$name.log"
done
exit 1
