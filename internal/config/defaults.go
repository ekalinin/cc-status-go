package config

import _ "embed"

//go:embed default.yaml
var defaultYAML []byte

// DefaultYAML returns the embedded out-of-the-box configuration, used by the
// --print-config flag.
func DefaultYAML() []byte { return defaultYAML }

// defaults holds the built-in Template/When/Style for each known segment type.
// A segment's empty fields are filled from its entry here; anything set in the
// user config wins. The "text" type intentionally has no default template so
// users supply their own literal/template.
//
// "when" is a boolean expression of templates joined by infix "&&"/"||" ("&&"
// binds tighter), evaluated for truthiness with short-circuit: a sub-condition
// is falsy when its rendered output is empty, "false", or a nil/"<no value>"
// marker. Conditions read naturally, e.g. "{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}"
// or "{{ .Workspace.GitWorktree }} || {{ .Worktree }} || {{ .Workspace.Repo }}" — a
// leading guard drops the segment for absent JSON sections (nil pointers render
// as "<nil>") and stops later parts from dereferencing them.
var defaults = map[string]Segment{
	"model": {
		Template: "{{ .Model.DisplayName }}",
		When:     "{{ .Model.DisplayName }}",
		Style:    "bold cyan",
	},
	"cwd": {
		// Show just the project/folder name by default; use the shortpath
		// helper in your own config if you want a truncated path instead.
		Template: "{{ basename .Workspace.CurrentDir }}",
		When:     "{{ .Workspace.CurrentDir }}",
		Style:    "blue",
	},
	"project": {
		// The launch directory's name, shown only when the cwd has moved away
		// from it during the session.
		Template: "{{ basename .Workspace.ProjectDir }}",
		When:     "{{ .Workspace.ProjectDir }} && {{ ne .Workspace.ProjectDir .Workspace.CurrentDir }}",
		Style:    "blue",
	},
	"repo": {
		Template: "{{ with .Workspace.Repo }}{{ .Owner }}/{{ .Name }}{{ end }}",
		When:     "{{ .Workspace.Repo }}",
		Style:    "purple",
	},
	"git": {
		// Prefer an explicit worktree branch, then a Claude worktree branch,
		// then the repo name.
		Template: "{{ with .Workspace.GitWorktree }}{{ . }}{{ else }}{{ with .Worktree }}{{ .Branch }}{{ else }}{{ with .Workspace.Repo }}{{ .Name }}{{ end }}{{ end }}{{ end }}",
		When:     "{{ .Workspace.GitWorktree }} || {{ .Worktree }} || {{ .Workspace.Repo }}",
		Style:    "purple",
	},
	"cost": {
		Template: "💰 ${{ printf \"%.2f\" .Cost.TotalCostUSD }} (+{{ .Cost.TotalLinesAdded }}/-{{ .Cost.TotalLinesRemoved }})",
		When:     "{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}",
		Style:    "green",
	},
	"duration": {
		Template: "{{ duration .Cost.TotalDurationMS }}",
		When:     "{{ .Cost }} && {{ gt .Cost.TotalDurationMS 0 }}",
		Style:    "dim",
	},
	"lines": {
		// The edit volume from "cost" on its own, without the dollar amount.
		Template: "+{{ .Cost.TotalLinesAdded }}/-{{ .Cost.TotalLinesRemoved }}",
		When:     "{{ .Cost }}",
		Style:    "green",
	},
	"context": {
		Template: "{{ printf \"%.0f\" .ContextWindow.UsedPercentage }}% ctx",
		When:     "{{ .ContextWindow }}",
		Style:    "yellow",
	},
	"tokens": {
		Template: "{{ humantokens .ContextWindow.TotalInputTokens }} in / {{ humantokens .ContextWindow.TotalOutputTokens }} out",
		When:     "{{ .ContextWindow }}",
		Style:    "dim",
	},
	"cache": {
		// Cache-read volume and hit ratio from the last API response. The hit
		// ratio is cache reads over all input tokens (fresh + cache write +
		// cache read), matching how used_percentage counts input.
		Template: "{{ with .ContextWindow.CurrentUsage }}{{ humantokens .CacheReadInputTokens }} cached ({{ pct .CacheReadInputTokens (add .InputTokens .CacheCreationInputTokens .CacheReadInputTokens) }}%){{ end }}",
		When:     "{{ .ContextWindow }} && {{ .ContextWindow.CurrentUsage }}",
		Style:    "dim",
	},
	"rate_limit": {
		// Each window is guarded with "with" so a missing five_hour or
		// seven_day doesn't break the segment; " / " joins them only when both
		// are present.
		Template: "{{ with .RateLimits.FiveHour }}5h: {{ printf \"%.0f\" .UsedPercentage }}%{{ end }}{{ if and .RateLimits.FiveHour .RateLimits.SevenDay }} / {{ end }}{{ with .RateLimits.SevenDay }}7d: {{ printf \"%.0f\" .UsedPercentage }}%{{ end }}",
		When:     "{{ .RateLimits }}",
		Style:    "red",
	},
	"rate_limit_full": {
		// Like rate_limit, but also shows when each window resets.
		Template: "{{ with .RateLimits.FiveHour }}5h: {{ printf \"%.0f\" .UsedPercentage }}% (in {{ reltime .ResetsAt }}){{ end }}{{ if and .RateLimits.FiveHour .RateLimits.SevenDay }} / {{ end }}{{ with .RateLimits.SevenDay }}7d: {{ printf \"%.0f\" .UsedPercentage }}% (in {{ reltime .ResetsAt }}){{ end }}",
		When:     "{{ .RateLimits }}",
		Style:    "red",
	},
	"output_style": {
		Template: "{{ .OutputStyle.Name }}",
		When:     "{{ .OutputStyle }} && {{ ne .OutputStyle.Name \"default\" }}",
		Style:    "dim",
	},
	"effort": {
		Template: "effort:{{ .Effort.Level }}",
		When:     "{{ .Effort }}",
		Style:    "magenta",
	},
	"thinking": {
		Template: "{{ if .Thinking.Enabled }}thinking{{ end }}",
		When:     "{{ .Thinking }} && {{ .Thinking.Enabled }}",
		Style:    "italic magenta",
	},
	"vim": {
		Template: "{{ .Vim.Mode }}",
		When:     "{{ .Vim }} && {{ .Vim.Mode }}",
		Style:    "bold yellow",
	},
	"agent": {
		Template: "\U0001F916 {{ .Agent.Name }}",
		When:     "{{ .Agent }} && {{ .Agent.Name }}",
		Style:    "bold magenta",
	},
	"pr": {
		Template: "PR #{{ .PR.Number }}{{ with .PR.ReviewState }} {{ . }}{{ end }}",
		When:     "{{ .PR }} && {{ gt .PR.Number 0 }}",
		Style:    "cyan",
	},
	"worktree": {
		Template: "{{ .Worktree.Name }}",
		When:     "{{ .Worktree }} && {{ .Worktree.Name }}",
		Style:    "blue",
	},
	"session": {
		Template: "{{ .SessionName }}",
		When:     "{{ .SessionName }}",
		Style:    "dim",
	},
	"version": {
		Template: "v{{ .Version }}",
		When:     "{{ .Version }}",
		Style:    "dim",
	},
	"fast_mode": {
		Template: "⚡ fast",
		When:     "{{ .FastMode }}",
		Style:    "bold yellow",
	},
	"exceeds_200k": {
		Template: "⚠ 200k+",
		When:     "{{ .Exceeds200K }}",
		Style:    "bold red",
	},
	"text": {
		// No default template: the user supplies one.
		When: "true",
	},
}
