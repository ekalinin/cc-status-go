# cc-status

A fast, configurable status line for [Claude Code](https://claude.com/claude-code),
written in Go. It reads the session JSON Claude Code provides on **stdin** and prints a
single formatted line to **stdout**, shaped by a YAML config — in the spirit of
[starship](https://starship.rs) and [oh-my-posh](https://ohmyposh.dev).

```
Opus │ cc-status-go │ feature-xyz │ 8% ctx │ $0.01 (+156/-23) │ 5h 24% / 7d 41% │ NORMAL │ 🤖 security-reviewer │ PR #1234 pending
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

## Configuration

The config is **hybrid**: a flat list of segments. Each segment's `type` supplies smart
defaults for its `template`, `when` (visibility), and `style`; set any of those fields to
override the default. Segments whose data is absent (e.g. no `cost`, no `pr`) drop out
automatically — together with their separator.

Config is discovered in this order:

1. `--config <path>` flag
2. `CC_STATUS_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/cc-status/config.yaml`
4. `~/.config/cc-status/config.yaml`
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
    when: "{{ if .Cost }}{{ gt .Cost.TotalCostUSD 0.0 }}{{ end }}"
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
| `git` | worktree branch, else Claude worktree branch, else repo name | `feature-xyz` |
| `cost` | session cost with lines added/removed | `💰 $0.01 (+156/-23)` |
| `lines` | lines added/removed on their own (the diff from `cost`) | `+156/-23` |
| `duration` | session wall-clock time | `1m30s` |
| `context` | context window used, in percent | `8% ctx` |
| `tokens` | input / output tokens, human-readable | `15.5k in / 2.1k out` |
| `cache` | cache-read tokens and hit ratio from the last response | `2.0k cached (13%)` |
| `rate_limit` | 5h and 7d usage percentages | `5h: 24% / 7d: 41%` |
| `rate_limit_full` | usage percentages plus each window's reset time | `5h: 24% (in 2h13m) / 7d: 41% (in 3d4h)` |
| `output_style` | active output style name (hidden when `default`) | `explanatory` |
| `effort` | reasoning effort level | `effort:high` |
| `thinking` | literal `thinking` when extended thinking is on | `thinking` |
| `vim` | current vim mode | `NORMAL` |
| `agent` | active subagent name | `🤖 security-reviewer` |
| `pr` | pull request number and review state | `PR #1234 pending` |
| `worktree` | Claude Code worktree name | `fix-login` |
| `session` | session name | `my-session` |
| `version` | Claude Code version | `v1.2.3` |
| `fast_mode` | badge shown when fast mode is on | `⚡ fast` |
| `exceeds_200k` | badge shown when the last response exceeds 200k tokens | `⚠ 200k+` |
| `text` | nothing by default - supply your own `template` | (yours) |

Each type's visibility guard is built in: e.g. `cost` shows only when spend is above `$0`,
`output_style` hides the `default` style, and `git`/`vim`/`agent`/`pr` appear only when
their data is present. Override any type's `template`, `when`, or `style` in your config;
`--print-config` dumps the full set of defaults.

### Templates

`template` and `when` are Go [`text/template`](https://pkg.go.dev/text/template) strings.
The root context (`.`) is the full session object, so you can reference any field:
`{{ .Model.DisplayName }}`, `{{ .Cost.TotalCostUSD }}`, `{{ with .PR }}#{{ .Number }}{{ end }}`.

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
go test ./...
echo '<session json>' | go run ./cmd/cc-status --config testdata/config.yaml
```
