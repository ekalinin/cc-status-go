# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`cc-status` is a status line for Claude Code, written in Go. It reads a session JSON
payload from **stdin** and writes a single formatted line to **stdout**, shaped by a YAML
config (in the spirit of starship / oh-my-posh). It is invoked by Claude Code on every
render, so it must be fast and must never crash the host UI.

## Commands

```sh
make build        # build with version ldflags into ./bin/cc-status
make test         # go test ./...
make check        # fmt-check + vet + test (run before committing)
make run          # render testdata/sample.json with testdata/config.yaml
make cover        # tests with coverage summary
make fmt          # gofmt -w .

go test ./internal/render -run TestRender      # single test / package
echo '<session json>' | go run ./cmd/cc-status --config testdata/config.yaml
go run ./cmd/cc-status --print-config          # dump the embedded default config
```

## Architecture

Pipeline: `main` reads stdin → `input.Parse` → `config.Load` → `render.Render` → styled
line to stdout. Each stage lives in its own `internal/` package.

- **`cmd/cc-status/main.go`** - CLI entry, flag parsing, and the crash-safety contract:
  a top-level `recover()` and every error path exits **0** so Claude Code never treats the
  status line as fatal. `version` is injected via `-ldflags "-X main.version=..."`.

- **`internal/input`** - the session JSON schema. `Input` is the template root context
  (`.`), so field names are chosen to read well in templates (`.Model.DisplayName`,
  `.Cost.TotalCostUSD`). **Optional sections are pointers** so templates can guard them
  with `{{ if .PR }}` / `{{ with .Vim }}` and `when` defaults can test for nil. Parsing is
  forgiving: unknown fields are ignored (forward-compatible) and empty input yields a
  zero-valued `*Input`, not an error.

- **`internal/config`** - loads YAML and merges each segment with per-type defaults.
  Config discovery order: `--config` flag → `CC_STATUS_CONFIG` env → `$XDG_CONFIG_HOME` /
  `~/.config/cc-status/config.{yaml,yml}` → embedded `default.yaml`. `defaults.go` holds
  the **per-type registry** (`model`, `git`, `cost`, `rate_limit`, `pr`, `repo`, `project`,
  `lines`, `cache`, `fast_mode`, `exceeds_200k`, `text`, …): a
  segment's empty `Template`/`When`/`Style` are filled from its type's entry, and anything
  set in user config wins. `default.yaml` is embedded via `//go:embed`.

- **`internal/render`** - executes segments and joins non-empty results with the
  separator. A segment renders only if `when` is truthy and its `template` produces
  non-empty output; absent-data segments (and their separators) drop out automatically.
  Templates are `text/template` with `Option("missingkey=zero")`. A failing segment is
  **skipped, never fatal** (reported to stderr only under `--debug`). `funcs.go` defines
  the custom template helpers (`basename`, `shortpath`, `duration`, `reltime`,
  `humantokens`, `trunc`, `default`, `upper`/`lower`).

- **`internal/style`** - parses style strings (`"bold cyan"`, `"fg:#ff8800 bg:240"`) into
  ANSI SGR codes. Disabled by `NO_COLOR` env, `--no-color`, or `no_color: true` in config;
  unknown tokens are ignored.

### The `when` mini-language

`when` is **not** raw `text/template` boolean logic. `render.visible` splits the string on
infix `&&` / `||` (`&&` binds tighter), renders each sub-template independently, and tests
each for truthiness with short-circuit evaluation. Falsy = empty, `false`, `<nil>`, or
`<no value>`. This is why the idiomatic guard `"{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}"`
works: the nil guard short-circuits before the second part dereferences a nil pointer. When
adding a default in `defaults.go`, follow this pattern - lead with a nil guard for any
optional (pointer) section.

## Conventions

- **Never break the crash-safety contract**: no code path in the render pipeline should
  panic out of `main` or exit non-zero. New segment logic must degrade to "skip the
  segment", not error.
- Adding a segment type = one entry in the `defaults` map in `config/defaults.go` (plus a
  helper in `render/funcs.go` if the template needs one). No registration elsewhere.
- Specs in `.spec.md` (behavior) and `.spec.perf.md` (performance) capture the original
  design intent; consult them for the "why".
