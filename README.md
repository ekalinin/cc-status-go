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

`model`, `cwd`, `git`, `cost`, `duration`, `context`, `tokens`, `rate_limit`,
`rate_limit_full`, `output_style`, `effort`, `thinking`, `vim`, `agent`, `pr`,
`worktree`, `session`, `version`, and `text` (no defaults — bring your own template).

`rate_limit` shows usage percentages; `rate_limit_full` additionally shows when each
window resets, e.g. `5h 24% (resets 2h13m) / 7d 41% (resets 3d4h)`.

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
