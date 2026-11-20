# Porting the Go `qovery` CLI to Rust

Goal: a Rust rewrite whose user interface is **strictly identical** to the Go CLI (`../cmd`, `../pkg`, `../utils`):
same commands, aliases, flags (names, shorthands, types, defaults, usages, required), help texts, outputs
(stdout/stderr bytes, colors), exit codes, API calls and `~/.qovery/context.json` handling. Users must not have
to change anything.

## Layout

```
rust/
  crates/cobra      qovery-cobra   cobra/pflag compatible command layer on top of clap (DONE, shared)
  crates/log        qovery-log     printers, pterm/fatih colors, pterm tables, logrus logger, Go formatting (DONE, shared)
  crates/api        qovery-api     blocking HTTP client of the Qovery API + minimal models (DONE, shared)
  crates/core       qovery-core    context, auth, client, selection prompts, lookups, statuses, telemetry (DONE, shared)
  crates/services   qovery-services  port of utils/qovery.go deploy/cancel/watch/getters, utils/watch.go ... (shared)
  crates/envvars    qovery-envvars   port of utils/env_var.go + cmd/external_secret_helpers.go (shared)
  crates/commands/<name>  qovery-cmd-<name>  one crate per top-level command (`application`, `cluster`, ...)
  crates/cli        qovery-cli     the `qovery` binary; each command crate is behind a `cmd-<name>` feature
  parity/           Go-vs-Rust comparison harness (mock API server, scenarios)
```

`crates/commands/organization` is the **reference example** of a ported command: read it first.

## Declaring commands (`qovery_cobra`)

Go:
```go
var applicationDeployCmd = &cobra.Command{Use: "deploy", Short: "Deploy an application", Run: ...}
applicationDeployCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
applicationDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch ...")
_ = applicationDeployCmd.MarkFlagRequired("application")
```
Rust:
```rust
Command::new("deploy").short("Deploy an application")
    .flag(Flag::string("application", "Application Name").short('n').required())
    .flag(Flag::bool("watch", "Watch ...").short('w'))
    .run(run_deploy)                       // fn run_deploy(ctx: &Ctx)
```
* `Flag::{bool,string,int,int32,int64,uint,float64,string_slice,string_array,string_to_string,duration}`;
  `.default_value("x")` takes pflag's **DefValue string** (`"30"`, `"true"`, `"15m0s"`, `"[sh]"`).
  `.persistent()`, `.hidden()`, `.deprecated(msg)`.
* `Command`: `.long()`, `.example()`, `.alias()`, `.hidden()`, `.deprecated()`, `.args(ArgsRule::Exact(1))`
  (`cobra.NoArgs` = `ArgsRule::NoArgs`, ...), `.subcommand()`, `.disable_flag_parsing()`.
  `Use` with args (`"get-token <cluster-id>"`) goes in `Command::new(...)`: the name is its first word.
* Group commands: Go groups that have a `Run` calling `cmd.Help()` when `len(args) == 0` must keep a `.run(...)`
  (it makes them "runnable", which changes their help/usage); groups without `Run` must not have one.
* `Ctx`: `ctx.string("name")`, `ctx.bool`, `ctx.int`, `ctx.int32`, `ctx.strings` (slice/array), `ctx.string_map`,
  `ctx.duration`, `ctx.changed("name")`, `ctx.args()`, `ctx.help()`, `ctx.command_path()`. Reading a flag the
  command does not define returns its zero value (Go shares package-level flag variables between commands).
* Help, usage, `--help`, unknown flags/commands, `required flag(s) ... not set`, args validation, the `Error: ...`
  printed twice and the exit code 0 on cobra errors are all handled by `qovery_cobra`: do not re-implement them.
* `QOVERY_DUMP_SPEC=1 qovery` dumps the command tree (used by `parity/help_diff.py`).

## Translating the Go code

| Go | Rust |
|---|---|
| `utils.Capture(cmd)` | `qovery_core::telemetry::capture(ctx)` |
| `utils.GetAccessToken(false)` + `utils.GetQoveryClient(...)` | `qovery_core::client::get_client()` -> `Result<Client, String>` |
| `utils.GetQoveryClientPanicInCaseOfError()` | `qovery_core::client::get_client_or_exit()` |
| `utils.PrintlnError(err); os.Exit(1)` / `checkError(err)` / `utils.CheckError(err)` | `qovery_core::exit_with_error(err)` / `qovery_core::check_error(result)` |
| `os.Exit(n)` | `qovery_core::exit(n)` (flushes stdout/stderr) |
| `utils.PrintlnInfo`, `utils.Println`, `utils.PrintTable(headers, rows)` | `qovery_log::{println_info, println, print_table}` |
| `fmt.Println`, `fmt.Printf` | `println!`, `print!` (stdout) — Go's builtin `println()` writes to **stderr**: `qovery_log::go_builtin_println` |
| `pterm.FgBlue.Sprintf(...)`, `pterm.Bold.Sprintf`, `pterm.DefaultTable`, `pterm.DefaultSection` | `qovery_log::pterm::{FG_BLUE.sprint(..), BOLD.sprint(..)}`, `qovery_log::table::Table`, `qovery_log::pterm::Section` (pterm ALWAYS emits colors) |
| `color.RedString(...)` (fatih) | `qovery_log::fatih::red(...)` (colors only when stdout is a tty) |
| `log.Infof`, `log.Warnf`, `log.Errorf`, `log.Fatal` (logrus, stderr) | `qovery_log::logrus::{info, warn, error, fatal}` / `log_info!` macros |
| `utils.FindStatusTextWithColor`, `utils.GetStatusTextWithColor` | `qovery_core::status::*` |
| `utils.FindByApplicationName(...)` & co | `qovery_core::find::find_by_name(&items, name)` |
| `getOrganizationProjectEnvironmentContextResourcesIds(client)` | `qovery_core::resources::ids_from_flags(&client, ctx)` (reads `--organization/--project/--environment`) |
| `getEnvironmentIdFromContextPanicInCaseOfError(client)` | `qovery_core::resources::environment_id_from_flags_or_exit(&client, ctx)` |
| `usercontext.GetOrganizationContextResourceId`, `getProjectContextResourceId`, ... | `qovery_core::resources::{organization_id, project_id, environment_id, service_by_name, application_by_name, ...}` |
| `utils.CurrentOrganization(true)`, `utils.GetCurrentContext()`, `utils.SetContext(...)` | `qovery_core::context::*` |
| `utils.SelectOrganization()` & co, promptui `Select`/`Prompt` | `qovery_core::select::*` (`prompt_select`, `prompt_select_with_label`, `prompt_select_with_searcher`, `prompt_text`, `prompt_text_with_default`) |
| `utils.Validate("delete")` (type-to-confirm) | `qovery_core::select::validate("delete")` |
| `time.Time.String()` (e.g. `application.UpdatedAt.String()`) | `qovery_log::gofmt::time_string_from_rfc3339(s)` |
| `fmt.Sprintf("%q", s)`, `time.Duration.String()` | `qovery_log::gofmt::{quote, duration_string}` |
| `json.Marshal(map[string]interface{}{...})` / of an API model | `qovery_log::gofmt::go_json(&serde_json::Value)` (sorted keys, Go HTML escaping) |
| `json.Marshal(structValue)` (field order matters) | `qovery_log::gofmt::go_marshal(&serde_struct)` |
| API calls `client.XxxAPI.Yyy(ctx, id).Execute()` | `client.get/list/post/put/patch/delete(path, ...)` on `qovery_api::Client`; errors display like Go (`404 Not Found`) |

* Endpoints / payloads: read the Go client in `$(go env GOMODCACHE)/github.com/qovery/qovery-client-go@v0.0.0-20261030144809-3aa18684b1e6/`
  (`api_*.go` for paths and methods, `model_*.go` for JSON field names). Typed minimal models live in
  `qovery_api::models`; otherwise use `serde_json::Value` or local serde structs.
* The Go generated models fail to decode responses lacking **required** properties
  (`parity/required_fields.py <model>`); the Rust side should be lenient, but fixtures must contain them.
* Go `json.Marshal` of an API model (e.g. `--json` outputs of a whole resource) sorts keys and drops unset optional
  fields: re-serialize the API `Value` with `go_json`.
* Keep Go behaviours even when they look odd (typos in messages, exit 0 on errors, messages on stdout instead of
  stderr, double newlines...). The target is byte-identical output.
* Interactive prompts: promptui `Select`/`Prompt` go through `qovery_core::select`. When stdin is **not** a terminal
  (pipes, scripts, the parity harness) promptui is emulated byte for byte (`qovery_core::{readline, promptui_select,
  promptui_prompt}`): same rendering on stdout, same keys read, same stdin consumption between prompts; on a terminal
  `inquire` is used (the rendering differs, the logic and surrounding prints do not). Never write a local prompt.
  survey prompts (`env import`) are not emulated (`inquire`, survey's non-tty rendering is not reproduced).
* Port the Go unit tests (`*_test.go`) of your area as Rust `#[cfg(test)]` tests when they test logic.

## Shared Go-compatibility helpers

Before writing a helper, look for it here (all of them reproduce the Go texts exactly):

| Go | Rust |
|---|---|
| `url.PathEscape`, `url.QueryEscape`, `url.Values.Encode()`, `URL.EscapedPath` | `qovery_api::gourl::{path_escape, query_escape, encode_query, escape_url_path}` |
| `*url.Error` op (`Get`, `Post`...) | `qovery_api::go_url_error_op` |
| `runtime.GOOS` / `GOARCH` | `qovery_core::gosys::{go_os, go_arch}` |
| errno text, `*fs.PathError` (`open /x: no such file or directory`) | `qovery_core::gosys::{errno_text, path_error}` |
| `exec.LookPath` (incl. `exec.ErrDot`) | `qovery_core::gosys::look_path` |
| `os.WriteFile(name, data, perm)` | `qovery_core::gosys::write_file` |
| `user.Current().Username` | `qovery_core::gosys::current_username` |
| unrecovered panic / nil pointer dereference (exit 2) | `qovery_core::gosys::{go_panic, go_nil_pointer_panic}` |
| `browser.OpenURL` | `qovery_core::browser::open_url` |
| `utils.GetAccessToken(false)` + `GetAuthorizationHeaderValue` | `qovery_core::auth::authorization()` |
| `t.Format(time.RFC3339 / RFC3339Nano / RFC1123 / RFC822)`, `utils.ToIso8601` | `qovery_log::gofmt::{time_rfc3339, time_rfc3339_nano, time_rfc1123, time_rfc822, time_iso8601}` |
| `text/tabwriter` (`NewWriter(w, minwidth, _, padding, ' ', 0)`) | `qovery_log::tabwriter::{TabWriter, tabwrite}` |
| `strconv.ParseBool/ParseInt/ParseUint`, pflag `readAsCSV`, `stringToString` | `qovery_cobra::{parse_go_bool, parse_go_int, parse_go_uint, read_as_csv, string_to_string_pairs}` |
| `StopSelectedServices` / `DeleteSelectedServices` | `qovery_services::{selected_services_action, ServiceIdsAllRequest}` |
| `utils.NewServicesWatch` / `Wait` | `qovery_services::new_services_watch` |
| `utils.IsTerminalState` / `IsTerminalClusterState` / `IsEmptyOrBlank` | `qovery_services::{is_terminal_state, is_terminal_cluster_state, is_empty_or_blank}` |
| `utils.WriteInFile` / `DeleteFolder` / `DeleteFile` | `qovery_services::{write_in_file, delete_folder, delete_file}` |
| `utils.SelectRole` / `SelectTokenInformation` | `qovery_services::{select_role, select_token_information}` |
| `downloadKubeconfig` / `GetKubeconfigByClusterId` | `qovery_cmd_cluster::{download_kubeconfig, get_kubeconfig_by_cluster_id}` |
| gorilla `websocket.Dialer.Dial` (+ response headers), `pkg/wserror.go` | `qovery_cmd_log::ws::{dial, dial_with_response, ...}` |
| `json.Unmarshal` syntax errors (`invalid character 'x' looking for beginning of value`) | `qovery_log::gojson::go_json_check_valid` (then decode with serde) |
| `os.UserHomeDir()` (`$HOME`, `%USERPROFILE%` on Windows, never the passwd database) | `qovery_core::gosys::user_home_dir` |
| `time.Parse(time.RFC3339, ...)` (and `time.Time` JSON decoding) | `qovery_log::gotime::{parse_rfc3339, parse_rfc3339_unix}` |
| `time.ParseDuration` (signed, Go error texts) | `qovery_log::gofmt::parse_duration_nanos` (flags: `ctx.duration_nanos`) |

Deliberately **not** shared (the Go behaviours differ): the other `encoding/json` scanners (`admin/gojson.rs` is
go-jose's fork of encoding/json, `env/gojson.rs` is a `json.Decoder` stream with its own `EOF`/type errors), and the
`net/http` transport error translations of `api` (CLI command), `admin`, `admin-cluster` and `qovery_api` (each was
matched against its own Go call site; they only agree on the common `connection refused` / timeout cases).

## Building only your crates

Several agents work in parallel: use **your own target dir** and build only your commands, so a crate being
edited by someone else cannot break your build:

```sh
cd rust
export CARGO_TARGET_DIR=$PWD/target/agents/<you>
cargo build -p qovery-cli --no-default-features --features "cmd-application"     # add your crates' features
```
`target/debug` (seeded build) can be copied to your target dir first to save time:
`mkdir -p target/agents/<you> && cp -r --reflink=auto target/debug target/agents/<you>/`.

Only edit: your own crates, `parity/scenarios/<your-crate>*.json`, `parity/fixtures/<your-crate>*.json`.
Never edit the shared crates (`cobra`, `log`, `api`, `core`, `cli`, `services`, `envvars`) or another agent's crate:
if you need a shared change (bug, missing helper), write a local helper in your crate and **report it** in your
final answer (the integrator will move/fix it centrally). New dependencies: add them to your crate's
`Cargo.toml` (prefer `workspace = true` entries of `rust/Cargo.toml`; otherwise a plain version requirement).

## Verifying parity (mandatory)

The reference Go binary is `rust/target/qovery-go` (built from `../` with the `QOVERY_DUMP_SPEC` patch, see
[README.md](README.md#reference-go-binary)). Never run anything against the real
Qovery API: the harness points both binaries to a local mock (`QOVERY_API_URL`).

```sh
export QOVERY_RUST_BIN=$CARGO_TARGET_DIR/debug/qovery
python3 parity/help_diff.py --prefix "application"              # spec + --help of every command, must be 100% identical
python3 parity/compare.py parity/scenarios/application.json     # behaviour on scenarios, must be identical
python3 parity/compare.py --args application list --json        # ad-hoc run with the default fixtures
python3 parity/compare.py -v ...                                # also show the Go output of identical cases
```
* `parity/fixtures/default.json` provides an organization `org-1`/`my-org`, project `proj-1`/`my-project`,
  environment `env-1`/`my-env` (cluster `cluster-1`), application `app-1`/`my-app`, container `cont-1`/`my-container`,
  cron job `cron-1`/`my-cron`, lifecycle job `life-1`/`my-lifecycle`, database `db-1`/`my-db`, helm `helm-1`/`my-helm`,
  terraform `tf-1`/`my-terraform`, statuses, cluster list. The default context points to them, and
  `QOVERY_CLI_ACCESS_TOKEN=qov_testtoken` is set (no token validation call). Scenario format: see `parity/compare.py`.
* Unmatched requests return 404 and appear in the request diff with `"unmatched": true`: add fixtures for every
  endpoint a command calls (POST/PUT/DELETE included) so the success paths are exercised.
* Write scenarios for each command: nominal output (table and `--json`), every flag that changes behaviour,
  name-not-found errors, API errors (4xx/5xx), missing required flags, bad args. Iterate until identical.
* Some things cannot be compared (prompts on a real terminal, browser opening, timing): test what can be (e.g.
  error paths, piped prompts) and list the rest in your report. Websockets have their own runners
  (`parity/*_ws_compare.py`, see README.md).
* Generated scenario files: edit `parity/generators/<name>.py` and re-run it rather than the JSON.

## Final report

List: commands ported; `help_diff` result; scenario results (N/M identical); every remaining drift with the reason
it cannot be resolved; shared-crate bugs or helpers you had to work around (with file:line of your local copy).
