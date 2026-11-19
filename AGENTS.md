# AGENTS.md

Guidance for AI coding agents working in this repository.

## Go conventions

### Do not write `panic("unreachable")` after `os.Exit`

Commands in `cmd/` exit on unrecoverable errors with:

```go
if service == nil {
    utils.PrintlnError(fmt.Errorf("service %s not found", name))
    os.Exit(1)
}
```

Nothing goes after `os.Exit(1)`. In particular, do **not** add a trailing
`panic("unreachable")`.

Older revisions of this repository carried 736 such panics, 706 of them tagged
`// staticcheck false positive: https://staticcheck.io/docs/checks#SA5011`.
That workaround is obsolete: current staticcheck (2026.2.x, and the
`staticcheck` linter bundled in golangci-lint v2) understands that `os.Exit`
does not return, and reports no SA5011 on this pattern. Both the tagged and the
bare panics were removed; do not reintroduce them, and do not add them to new
code.

If a linter ever does flag such a line, fix it with a scoped
`//lint:ignore SA5011 <reason>` directive rather than with dead code.

Verify with:

```bash
rg -n 'panic\("unreachable"\)' --glob '*.go' .   # expect no hits
golangci-lint run ./...
```

## Rust rewrite (`rust/`)

`rust/` is a Rust rewrite of this CLI whose user interface is **byte-identical**
to the Go one: same commands, aliases, flags, help texts, stdout/stderr bytes
(colors included), exit codes, API requests and `~/.qovery/context.json`
writes. Both CLIs are released side by side, so the two implementations must
stay in sync. Read `rust/README.md` (architecture, harness, CI) and
`rust/PORTING.md` (Go → Rust conventions) before touching it.

### Every user-visible Go change is ported in the same change

Any change to `cmd/`, `pkg/` or `utils/` that a user can observe (new or
renamed command or flag, help text, output, error message, API call, context
handling) must be mirrored in `rust/` in the same PR. The `rust-parity` job of
`.github/workflows/build.yml` runs both CLIs side by side and fails on any
difference, so a Go-only change breaks CI.

Where the Go code lives in Rust:

| Go | Rust |
|---|---|
| `cmd/<command>*.go` | `rust/crates/commands/<command>/` (`admin cluster ...` → `admin-cluster`) |
| cobra / pflag behaviour | `rust/crates/cobra` (do not re-implement help, usage or cobra errors) |
| `utils/printer.go`, pterm, logrus, Go formatting | `rust/crates/log` |
| API client, models | `rust/crates/api` |
| `utils/context.go`, auth, prompts, lookups, telemetry | `rust/crates/core` |
| `utils/qovery.go`, `utils/watch.go` | `rust/crates/services` |
| `utils/env_var.go` | `rust/crates/envvars` |

A new top-level command is a new crate in `rust/crates/commands/<name>`, a
`cmd-<name>` feature in `rust/crates/cli/Cargo.toml` (added to `default`) and
its registration in `rust/crates/cli/src/main.rs`.

### Prove parity with a scenario

1. Make the Go change, then the Rust one.
2. Add or update cases in `rust/parity/scenarios/<command>.json` (fixtures in
   `rust/parity/fixtures/`). When the file has a generator in
   `rust/parity/generators/`, edit the generator and re-run it; never edit the
   generated file by hand. Never put a path of your machine in a scenario:
   use `$PARITY` (absolute path of `rust/parity/`, e.g. the fake tools of
   `rust/parity/fake-tools/`) and `$MOCK` (URL of the API mock).
3. Build both binaries and compare:

```bash
cp rust/parity/go_dump_spec.go.txt cmd/zz_dump.go   # spec dump used by help_diff.py
go build -o rust/target/qovery-go .
rm cmd/zz_dump.go                                   # never commit it
cd rust
cargo build -p qovery-cli
python3 parity/help_diff.py                         # expect N/N commands identical
python3 parity/compare.py parity/scenarios/<command>.json --retries 2
bash parity/run_all.sh                              # everything, what CI runs
cargo fmt --all && cargo clippy --workspace --all-targets -- -D warnings && cargo test --workspace
```

The harness only talks to its local mock (`QOVERY_API_URL`): **never** run it,
or any Rust test, against the real Qovery API.

### Rules

- Reproduce the Go output exactly, including its quirks and bugs. Fix a Go bug
  in both CLIs or in neither. The only intended difference: `qovery upgrade`
  downloads the Rust archive (`qovery-cli-rust_*`) instead of the Go one.
- When a drift really cannot be removed (Go map order, panics, third-party
  error texts...), mark only the affected fields with `"ignore"` in the
  scenario case, give the reason in a `note`, and list the drift in
  `rust/README.md`/`rust/PORTING.md`. Do not ignore a field to make a failing
  case pass.
- Platform code: the CLI ships for linux, darwin and windows (amd64, arm64).
  Put Unix-only APIs (permission bits, signals, termios, `libc`) behind
  `#[cfg(unix)]` with what Go does on Windows in the other branch, and mirror
  Go build tags (`cmd/upgrade.go` is `//go:build !windows`, so `upgrade` is
  not registered on Windows). Use Go's home directory rules
  (`os.UserHomeDir()`: `%USERPROFILE%` on Windows), not `$HOME` directly.
- Dependencies go in `[workspace.dependencies]` of `rust/Cargo.toml`; commit
  `rust/Cargo.lock` (CI builds with `--locked`). The toolchain is pinned in
  `rust/rust-toolchain.toml`.
- The version is embedded at build time from `QOVERY_CLI_VERSION` (the
  `-ldflags -X .../utils.Version=` of Go); do not hard-code it.
- Releases: `.github/workflows/rust-build.yml` publishes
  `qovery-cli-rust_<version>_<os>_<arch>` archives, `checksums-rust.txt` and
  the `<tag>-rust`/`latest-rust` images next to the Go artifacts. Keep these
  names and the archive layout stable: users and scripts download them.
