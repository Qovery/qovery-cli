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
