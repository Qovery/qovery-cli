# Qovery CLI — Rust rewrite

A Rust rewrite of the Go `qovery` CLI (`../cmd`, `../pkg`, `../utils`) whose user interface is **byte-identical** to
the Go one: same commands, aliases, flags, help texts, outputs (stdout/stderr bytes, colors), exit codes, API calls
and `~/.qovery/context.json` handling. [`PORTING.md`](PORTING.md) has the porting rules and the Go → Rust mapping.

## Architecture

```
rust/
  Cargo.toml                 workspace (shared dependency versions in [workspace.dependencies])
  crates/
    cobra     qovery-cobra     cobra/pflag compatible command layer on top of clap: command tree, flags, help/usage,
                               cobra error messages, `QOVERY_DUMP_SPEC`; pflag value parsers (`__complete` and the
                               completion scripts are in `commands/completion`)
    log       qovery-log       printers (utils.Println*), pterm / fatih colors, pterm tables, logrus logger,
                               `gofmt` (Go formatting: %q, time.Time, durations, JSON, RFC3339/1123/822 ...),
                               `gojson` (encoding/json syntax errors),
                               `tabwriter` (text/tabwriter)
    api       qovery-api       blocking HTTP client of the Qovery API, minimal models, Go client error texts,
                               `gourl` (url.PathEscape / QueryEscape / Values.Encode), `go_url_error_op`
    core      qovery-core      context file, auth & token refresh, API client creation, interactive selection
                               (`select`, promptui emulation), name lookups, statuses, telemetry, version,
                               `gosys` (GOOS/GOARCH, errno & PathError texts, exec.LookPath, os.WriteFile,
                               user.Current, Go panics), `browser` (pkg/browser.OpenURL)
    services  qovery-services  utils/qovery.go: deploy/stop/delete/cancel helpers, services watch, getters,
                               jobs, token selection, utils/file_handler.go, utils/date.go ...
    envvars   qovery-envvars   utils/env_var.go + cmd/external_secret_helpers.go (env var / secret commands)
    commands/<name>  qovery-cmd-<name>  one crate per top-level command (`application`, `cluster`, ...)
    cli       qovery-cli       the `qovery` binary; each command crate is behind a `cmd-<name>` feature (all on by default)
  parity/                      Go-vs-Rust comparison harness (see below)
```

Dependencies only go downwards: `cobra`/`log` ← `api` ← `core` ← `services`/`envvars` ← `commands/*` ← `cli`. A few
command crates reuse another command crate's public helpers (`shell`/`port-forward`/`list-pods` use `log`'s
websocket client, `terraform` uses `cluster`'s kubeconfig download).

## Building

```sh
cd rust
cargo build -p qovery-cli                 # debug binary: target/debug/qovery
cargo build -p qovery-cli --release       # release binary: target/release/qovery

# release builds embed the version like Go's `-ldflags -X github.com/qovery/qovery-cli/utils.Version=...`
QOVERY_CLI_VERSION=1.175.1 cargo build -p qovery-cli --release

# a subset of the commands
cargo build -p qovery-cli --no-default-features --features "cmd-application cmd-container"
```

Without `QOVERY_CLI_VERSION` the version is `unknown` (like a plain `go build`), which some commands report as
an invalid semver (exactly like Go).

Checks: `cargo clippy --workspace --all-targets`, `cargo test --workspace`, `cargo fmt --all`.

The toolchain is pinned in `rust-toolchain.toml` (rustup installs it on first use).

Platforms: the same as the Go release (linux, darwin, windows × amd64, arm64). As in Go (`cmd/upgrade.go` is
`//go:build !windows`), the `upgrade` command does not exist on Windows; Unix-only calls (permission bits, signals,
`$PWD`) are behind `cfg(unix)` with the behaviour of Go on Windows otherwise. The parity harness only runs on Linux.

## Parity harness

Everything runs against a local mock of the API (`parity/mock_server.py`); **never** against the real Qovery API.
Both binaries run in a fresh sandbox `$HOME`, with no tty, `QOVERY_TELEMETRY=false`, `TZ=UTC`.

### Reference Go binary

The harness compares with `rust/target/qovery-go` (override with `QOVERY_GO_BIN`). `help_diff.py` needs it to
support `QOVERY_DUMP_SPEC=1` (dump of the command tree as JSON). The Go CLI does not have it: it comes from a small
patch, [`parity/go_dump_spec.go.txt`](parity/go_dump_spec.go.txt), to drop in `cmd/` while building:

```sh
cd ..                                            # the Go sources
cp rust/parity/go_dump_spec.go.txt cmd/zz_dump.go
go build -o rust/target/qovery-go .              # Go 1.27, CGO enabled (default)
rm cmd/zz_dump.go
```

The Rust binary to compare is `QOVERY_RUST_BIN` (default `rust/target/debug/qovery`).

### Help / spec (`help_diff.py`)

```sh
export QOVERY_RUST_BIN=$PWD/target/debug/qovery
python3 parity/help_diff.py --prefix ""            # every command: spec (flags, defaults, usages...) + --help
python3 parity/help_diff.py --prefix "application"  # a subtree
```

### Behaviour scenarios (`compare.py`)

`parity/scenarios/<name>.json` lists cases (args, stdin, env, context, files, fixtures) run with both binaries;
exit code, stdout, stderr, API requests and the final context file are compared (format: docstring of
`parity/compare.py`; default fixtures: `parity/fixtures/default.json`).

```sh
python3 parity/compare.py parity/scenarios/application.json          # N/M identical
python3 parity/compare.py parity/scenarios/application.json --only "list json" -v
python3 parity/compare.py --args application list --json             # ad-hoc case, default fixtures
```

Env values may use `$MOCK` (URL of the API mock) and `$PARITY` (absolute path of `parity/`, e.g. the fake
`docker`/`k3d`/`helm`/`kubectl` of `parity/fake-tools/` used by the `demo` cases). Never write a path of your
machine in a scenario: `run_all.sh` rejects them, since the CI checks out the repository elsewhere.

A case can `"ignore"` some of `exit`, `stdout`, `stderr`, `requests`, `context` when the Go output cannot be
reproduced (Go map iteration order, Go panic stack traces, random ids, timestamps): the reason is in the case name
or in a `note`/`_comment` field.

Parity only proves both CLIs agree. A case that must refuse something can also assert what both do with
`"expect": {"exit": 1, "stdout_contains": "...", "requests": ["GET /environment/env-1/application"]}`, where
`requests` is the exact ordered list of `METHOD /path` sent.

Websocket commands use their own runners (they start a websocket mock and substitute `$WS` in the scenario):

```sh
python3 parity/log_ws_compare.py parity/scenarios/log_ws.json
python3 parity/shell_ws_compare.py parity/scenarios/shell_ws.json
python3 parity/shell_ws_compare.py parity/scenarios/list-pods_ws.json
python3 parity/port-forward_ws_compare.py parity/scenarios/port-forward_ws.json
```

The whole suite takes ~15 minutes sequentially. `parity/run_all.sh` runs everything (help, scenario files in
parallel, websocket files) and prints the drifting cases, exiting 1 on any drift. The CI runs it:

```sh
parity/run_all.sh                          # PARITY_JOBS (default: CPUs), PARITY_RETRIES (default 2)
```

`compare.py --retries N` re-runs a differing case up to N times before reporting it (see the Go races below).

Known nondeterminism on the Go side: promptui redraws the prompt from a goroutine; under heavy load a few % of the
Go runs print the last redraw after the final line (or tear it) for inputs ending with `\n` (see the `note` of the
`core-select` cases). Such a one-off `DIFF` on a prompt case is a Go race: re-run the case.

A 409 on `POST /variable` whose body carries a `title` makes Go print a `*string` pointer address
(`409 Conflict %!s(*string=0xc000…) (<detail>)`), different on every run: the Rust CLI prints a fixed address
(`crates/envvars/src/api.rs`, `variable_conflict_error`). No scenario sends a `title`.

### Scenario generators

Big scenario files are generated by `parity/generators/<name>.py` (they write `parity/scenarios/<name>.json`, or
the path given as first argument; `list-pods.py` writes `list-pods.json`, `list-pods_ws.json` and
`port-forward.json`, `shell.py` writes `shell.json` and `shell_ws.json`). Edit the generator, not the generated
file, then re-run it:

```sh
python3 parity/generators/cluster.py
```

Files without a generator (`admin.json`, `context.json`, `token.json`, ...) are edited by hand.

## CI and releases

* `.github/workflows/build.yml` (every push): `rust-check` (fmt, clippy `-D warnings`, tests) and `rust-parity`
  (builds the Go CLI with the dump-spec patch and the Rust CLI, then `parity/run_all.sh`). A Go change without
  its Rust counterpart fails `rust-parity`.
* `.github/workflows/rust-build.yml` (reusable): release binaries for the 6 platforms of `.goreleaser.yml`
  (static musl binaries on Linux), archived like GoReleaser does (`qovery-cli-rust_<version>_<os>_<arch>.tar.gz`,
  `.zip` on Windows, binary + README + LICENSE) with `checksums-rust.txt`.
* `.github/workflows/release.yml` (tags): attaches those archives to the GitHub release GoReleaser creates,
  uploads them to the same R2 folder, and pushes `qovery-cli:<tag>-rust` / `qovery-cli:latest-rust` images
  (ECR public + GHCR, from `rust/Dockerfile`). Homebrew, Scoop and AUR still install the Go CLI.
* `.github/workflows/release_latest.yml` (main): builds the release binaries, publishes nothing.

`qovery upgrade` downloads the Rust archive (`qovery-cli-rust_*`) where Go downloads `qovery-cli_*`, so the
Rust CLI upgrades to the Rust CLI (intentional difference with Go). It also stops with an error when the
download is not a 200 (the Rust archives are attached a few minutes after GoReleaser publishes the release).
