# cc-status

A fast, configurable status line for [Claude Code](https://claude.com/claude-code),
written in Go. It reads the [session JSON](https://code.claude.com/docs/en/statusline)
Claude Code provides on **stdin** and prints a single formatted line to **stdout**, shaped
by a YAML config — in the spirit of [starship](https://starship.rs) and
[oh-my-posh](https://ohmyposh.dev).

```
Opus │ cc-status-go │ 🌿 main │ 8% ctx │ 💰 $0.01 (+156/-23) │ 5h: 24% / 7d: 41% │ NORMAL │ 🤖 security-reviewer │ PR #1234 pending
```

## Install

```sh
go build -o cc-status ./cmd/cc-status
# then move cc-status somewhere on your PATH
```

Wire it into Claude Code via `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "/path/to/cc-status"
  }
}
```

## Try it

`cc-status` is just a filter: feed it a session payload on stdin and it prints the line.
Run this from inside any git repository to check your build without touching Claude Code:

```sh
cat <<EOF | cc-status
{
  "model": { "display_name": "Opus" },
  "workspace": { "current_dir": "$PWD" },
  "context_window": { "used_percentage": 8 },
  "cost": { "total_cost_usd": 0.0123, "total_lines_added": 156, "total_lines_removed": 23 }
}
EOF
```

```
Opus | cc-status-go | 🌿 main | 8% ctx | 💰 $0.01 (+156/-23)
```

The payload deliberately omits `rate_limits`, `vim`, `agent` and `pr`: those segments drop
out along with their separators, which is exactly what happens in a real session. Add
`--no-color` to compare output byte for byte, and `--debug` to surface template errors.

A fuller payload with every optional section present lives in `testdata/sample.json`:

```sh
cc-status --config testdata/config.yaml < testdata/sample.json
```

For the complete list of fields Claude Code sends, see the
[status line documentation](https://code.claude.com/docs/en/statusline).

## Configuration

The config is **hybrid**: a flat list of segments. Each segment's `type` supplies smart
defaults for its `template`, `when` (visibility), `style` and `symbol`; set any of those
fields to override the default. Segments whose data is absent (e.g. no `cost`, no `pr`) drop
out automatically — together with their separator.

Config is discovered in this order:

1. `--config <path>` flag
2. `CC_STATUS_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/cc-status/config.yaml` (or `.yml`)
4. `~/.config/cc-status/config.yaml` (or `.yml`)
5. the built-in default (see `cc-status --print-config`)

```yaml
separator: " │ "
no_color: false
segments:
  - type: model
    style: "bold cyan"
  - type: cwd
    style: blue
    # default shows just the folder name; for a truncated path use:
    # template: "{{ shortpath .Workspace.CurrentDir 3 }}"
  - type: git
  - type: context
  - type: cost
    symbol: "$"         # swap the 💰 glyph without restating the template
    when: "{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}"
  - type: rate_limit
  - type: vim
  - type: agent
  - type: pr
  - type: text          # free-form: no default template, supply your own
    style: dim
    template: "v{{ .Version }}"
```

### Segment types

Every type ships smart defaults for its `template`, `when`, and `style`. A segment drops
out automatically when its data is absent, so you can list all of them and only the
relevant ones show. The default output and example for each type:

| type | outputs | example |
|------|---------|---------|
| `model` | active model display name | `Opus` |
| `cwd` | current directory name (folder only) | `cc-status-go` |
| `project` | launch directory name, only when it differs from `cwd` | `my-app` |
| `repo` | `owner/name` from the git `origin` remote | `anthropics/claude-code` |
| `git` | current branch, or the worktree you are in (see below) | `🌿 main` |
| `cost` | session cost with lines added/removed | `💰 $0.01 (+156/-23)` |
| `lines` | lines added/removed on their own (the diff from `cost`) | `+156/-23` |
| `duration` | session wall-clock time | `1m30s` |
| `context` | context window used, in percent | `8%` |
| `context_full` | the same, labelled | `8% ctx` |
| `tokens` | input / output tokens, human-readable | `15.5k / 2.1k` |
| `tokens_full` | the same, labelled | `15.5k in / 2.1k out` |
| `cache` | cache-read tokens and hit ratio from the last response | `2.0k (13%)` |
| `cache_full` | the same, labelled | `2.0k cached (13%)` |
| `rate_limit` | 5h and 7d usage percentages | `5h: 24% / 7d: 41%` |
| `rate_limit_full` | usage percentages plus each window's reset time | `5h: 24% (in 2h13m) / 7d: 41% (in 3d4h)` |
| `output_style` | active output style name (hidden when `default`) | `explanatory` |
| `effort` | reasoning effort level | `high` |
| `effort_full` | reasoning effort level, labelled | `effort:high` |
| `thinking` | literal `thinking` when extended thinking is on | `thinking` |
| `vim` | current vim mode | `NORMAL` |
| `agent` | active subagent name | `🤖 security-reviewer` |
| `pr` | pull request number | `#1234` |
| `pr_full` | pull request number, labelled, with its review state | `PR #1234 pending` |
| `worktree` | Claude Code worktree name | `fix-login` |
| `session` | session name | `my-session` |
| `version` | Claude Code version | `v1.2.3` |
| `fast_mode` | badge shown when fast mode is on | `⚡ fast` |
| `exceeds_200k` | badge shown when the last response exceeds 200k tokens | `⚠ 200k+` |
| `text` | nothing by default - supply your own `template` | (yours) |

Each type's visibility guard is built in: e.g. `cost` shows only when spend is above `$0`,
`output_style` hides the `default` style, and `vim`/`agent`/`pr` appear only when their
data is present. Override any type's `template`, `when`, `style`, or `symbol` in your config;
`--print-config` dumps the full set of defaults.

Where the same data has a compact and a detailed rendering, the plain name is the compact
one and the detailed one is `<type>_full`: `context`, `tokens`, `cache`, `pr`, `rate_limit`
and `effort` all come in a pair. The short form prints the numbers alone, `_full` spells
them out - the words (`ctx`, `in`/`out`, `cached`, `PR`), the review state for `pr`, each
window's reset time for `rate_limit_full`. A status line is only a few columns wide, so the
shorter form is the default and you opt into the longer one. The built-in config is the
exception: it lists `context_full`, `pr_full` and `rate_limit_full`, since a first-run line
should read without a legend.

#### Symbols

A segment's leading glyph lives in its own `symbol` field, so you can change it without
restating the whole template. **Every type accepts one**, including the types that ship no
glyph of their own:

```yaml
segments:
  - type: cost
    symbol: "$"        # a scalar sets the segment's one symbol
  - type: rate_limit_full
    symbol: "⏳"       # a type with no glyph by default - no template needed
  - type: agent
    symbol: ""         # empty drops the glyph, leaving no stray space
  - type: git
    symbol:            # a mapping sets symbols by name
      worktree: "🌲"   # branch keeps its default 🌿
```

These are the types that come with a glyph already; everywhere else the default is no symbol:

| type | default | names |
|------|---------|-------|
| `git` | 🌿 / 🌳 | `branch`, `worktree` |
| `cost` | 💰 | — |
| `agent` | 🤖 | — |
| `fast_mode` | ⚡ | — |
| `exceeds_200k` | ⚠ | — |

A mapping overrides only the names it lists and the rest keep their defaults; a scalar
replaces the whole set, so `symbol: "⎇"` on `git` uses that one glyph for both cases.

In your own `template` the symbol is `{{ symbol }}`, or `{{ symbol "worktree" }}` for a named
one. Put it first and follow it with a space: an empty symbol then disappears along with that
space, since the rendered segment is trimmed.

#### The `git` segment

The leading symbol tells you *where* you are, which the branch name alone does not:

| you are in | shows | example |
|------------|-------|---------|
| the main working tree | 🌿 and the current branch | `🌿 main` |
| a `git worktree add` checkout | 🌳 and the worktree name | `🌳 feature-xyz` |
| a `claude --worktree` session | 🌳 and the worktree's branch | `🌳 worktree-my-feature` |
| a detached HEAD, or no repository | nothing — the segment drops out | |

Both glyphs are configurable — `symbol: {branch: "⎇", worktree: "🌲"}`, see
[Symbols](#symbols) above.

Claude Code sends no field for the current branch: `workspace.git_worktree` is a worktree
*name* and is absent in the main working tree, and `worktree.branch` only appears in
`--worktree` sessions. So the last case reads `.git/HEAD` directly — the one place this
tool touches the filesystem. It never shells out to `git`, which would cost milliseconds
on every render; reading the file takes microseconds.

### Templates

`template` and `when` are Go [`text/template`](https://pkg.go.dev/text/template) strings.
The root context (`.`) is the full session object, so you can reference any field
Claude Code [documents](https://code.claude.com/docs/en/statusline#json-input-structure):
`{{ .Model.DisplayName }}`, `{{ .Cost.TotalCostUSD }}`, `{{ with .PR }}#{{ .Number }}{{ end }}`.
Unknown fields are ignored, so a newer Claude Code never breaks the line.

A segment is shown when its `when` renders to a truthy value — anything that isn't empty,
`false`, or a nil/missing marker. Conditions combine with infix `&&` and `||` (`&&` binds
tighter than `||`), evaluated left to right with short-circuit:

```yaml
when: "{{ .Model.DisplayName }}"                        # single condition
when: "{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}"  # AND; leading guard is nil-safe
when: "{{ .Workspace.GitWorktree }} || {{ .Worktree }}" # OR
when: "{{ .Vim }} && {{ .Vim.Mode }} || {{ .Agent }} && {{ .Agent.Name }}"  # (A&&B)||(C&&D)
```

No `{{ if … }}true{{ end }}` wrapper and no prefix `{{ and … }}` / `{{ or … }}` are needed.

Built-in helpers, on top of the standard `printf`/`eq`/`gt`/`and`/`or`/`with`/…:

| func | example | result |
|------|---------|--------|
| `basename path` | `{{ basename .Workspace.CurrentDir }}` | final component, e.g. `cc-status-go` (cwd default) |
| `shortpath path n` | `{{ shortpath .Cwd 3 }}` | collapses `$HOME`→`~`, keeps last 3 parts |
| `branch dir` | `{{ branch .Workspace.CurrentDir }}` | branch from `.git/HEAD`, empty outside a repo |
| `symbol [name]` | `{{ symbol }}`, `{{ symbol "worktree" }}` | this segment's `symbol`, see [Symbols](#symbols) |
| `duration ms` | `{{ duration .Cost.TotalDurationMS }}` | `45s`, `1m30s` |
| `reltime unixSec` | `{{ reltime .RateLimits.FiveHour.ResetsAt }}` | `13m`, `2h13m`, `3d4h`, `now` |
| `humantokens n` | `{{ humantokens .ContextWindow.TotalInputTokens }}` | `15.5k` |
| `add nums…` | `{{ add .Foo .Bar .Baz }}` | integer sum (build a denominator inline) |
| `pct part whole` | `{{ pct .Read (add .In .Create .Read) }}` | rounded percentage, `0` when `whole ≤ 0` |
| `trunc n s` | `{{ trunc 20 .SessionName }}` | truncates with `…` |
| `default fb v` | `{{ default "?" .Agent.Name }}` | fallback if empty |
| `upper` / `lower` | `{{ upper .Vim.Mode }}` | case folding |

### Styling

`style` is a space-separated list of tokens (simple ANSI, no powerline fill):

- attributes: `bold`, `dim`, `italic`, `underline`, `reverse`, `strikethrough`
- named colors: `black red green yellow blue magenta/purple cyan white`, plus `bright_*`
- `fg:<color>` / `bg:<color>`
- 256-color palette index: `240`
- truecolor hex: `#ff8800`

Colors are disabled when `NO_COLOR` is set, `--no-color` is passed, or `no_color: true`.

## Flags

```
--config <path>    use a specific config file
--no-color         disable ANSI colors
--print-config     print the built-in default config and exit
--version          print version and exit
--debug            report per-segment template errors to stderr
```

## Development

```sh
make check    # gofmt -l, go vet, go test ./...  (run before committing)
make run      # render testdata/sample.json with testdata/config.yaml
make cover    # tests with a coverage summary

go run ./cmd/cc-status --config testdata/config.yaml < testdata/sample.json
```

Adding a segment type is one entry in the `defaults` map in `internal/config/defaults.go`
(plus a helper in `internal/render/funcs.go` if the template needs one) — there is no
registration anywhere else.

Whatever you add, keep the crash-safety contract: no path in the render pipeline may panic
out of `main` or exit non-zero, or Claude Code treats the status line as fatal. New segment
logic must degrade to "skip the segment", never to an error.
