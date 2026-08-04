package config

import _ "embed"

//go:embed default.yaml
var defaultYAML []byte

// DefaultYAML returns the embedded out-of-the-box configuration, used by the
// --print-config flag.
func DefaultYAML() []byte { return defaultYAML }

// defaults holds the built-in Template/When/Style/Symbol for each known segment
// type. A segment's empty fields are filled from its entry here; anything set in
// the user config wins. The "text" type intentionally has no default template so
// users supply their own literal/template.
//
// A type's glyphs belong in Symbol, not in the template text: the template
// reaches them with {{ symbol }} (or {{ symbol "name" }} when a type picks
// between several), which is what lets a user change one glyph without
// restating the template. Every template opens with {{ symbol }} and a space,
// even the types that ship no glyph of their own - a symbol nobody set renders
// as "" and the leading space is trimmed off the segment, so the default output
// is unchanged while "symbol: ⏳" on any type works without touching its
// template.
//
// The one caveat: a template that self-drops by rendering "" needs its "when" to
// cover that case too, or the symbol would be left behind as the whole segment.
// That is why rate_limit demands a window rather than just a rate_limits object,
// and why git keeps its symbols inside its branches instead of up front.
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
		Template: "{{ symbol }} {{ .Model.DisplayName }}",
		When:     "{{ .Model.DisplayName }}",
		Style:    "bold cyan",
	},
	"cwd": {
		// Show just the project/folder name by default; use the shortpath
		// helper in your own config if you want a truncated path instead.
		Template: "{{ symbol }} {{ basename .Workspace.CurrentDir }}",
		When:     "{{ .Workspace.CurrentDir }}",
		Style:    "blue",
	},
	"project": {
		// The launch directory's name, shown only when the cwd has moved away
		// from it during the session.
		Template: "{{ symbol }} {{ basename .Workspace.ProjectDir }}",
		When:     "{{ .Workspace.ProjectDir }} && {{ ne .Workspace.ProjectDir .Workspace.CurrentDir }}",
		Style:    "blue",
	},
	"repo": {
		Template: "{{ symbol }} {{ with .Workspace.Repo }}{{ .Owner }}/{{ .Name }}{{ end }}",
		When:     "{{ .Workspace.Repo }}",
		Style:    "purple",
	},
	"git": {
		// Where we are, with the leading symbol saying what kind of place:
		// "worktree" (🌳) for a worktree (its name from workspace.git_worktree,
		// or the branch of a --worktree session), "branch" (🌿) for a branch in
		// the main working tree. Neither field is present in an ordinary
		// session, so the last case reads the branch from .git/HEAD.
		//
		// No "when": the template yields "" outside a git repository (and for a
		// detached HEAD), which drops the segment on its own. Guarding on
		// .Workspace.Repo instead would hide the branch in repositories with no
		// "origin" remote, where that field is absent.
		Template: "{{ $wt := .Workspace.GitWorktree }}" +
			"{{ with .Worktree }}{{ $wt = or $wt .Branch }}{{ end }}" +
			"{{ if $wt }}{{ symbol \"worktree\" }} {{ $wt }}" +
			"{{ else }}{{ with branch .Workspace.CurrentDir }}{{ symbol \"branch\" }} {{ . }}{{ end }}{{ end }}",
		Style:  "purple",
		Symbol: Symbols{"branch": "🌿", "worktree": "🌳"},
	},
	"cost": {
		Template: "{{ symbol }} ${{ printf \"%.2f\" .Cost.TotalCostUSD }} (+{{ .Cost.TotalLinesAdded }}/-{{ .Cost.TotalLinesRemoved }})",
		When:     "{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}",
		Style:    "green",
		Symbol:   Symbols{"": "💰"},
	},
	"duration": {
		Template: "{{ symbol }} {{ duration .Cost.TotalDurationMS }}",
		When:     "{{ .Cost }} && {{ gt .Cost.TotalDurationMS 0 }}",
		Style:    "dim",
	},
	"lines": {
		// The edit volume from "cost" on its own, without the dollar amount.
		Template: "{{ symbol }} +{{ .Cost.TotalLinesAdded }}/-{{ .Cost.TotalLinesRemoved }}",
		When:     "{{ .Cost }}",
		Style:    "green",
	},
	"context": {
		// Just the percentage, for a line where the number speaks for itself;
		// use context_full when it needs naming.
		Template: "{{ symbol }} {{ printf \"%.0f\" .ContextWindow.UsedPercentage }}%",
		When:     "{{ .ContextWindow }}",
		Style:    "yellow",
	},
	"context_full": {
		// Like context, but labelled.
		Template: "{{ symbol }} {{ printf \"%.0f\" .ContextWindow.UsedPercentage }}% ctx",
		When:     "{{ .ContextWindow }}",
		Style:    "yellow",
	},
	"tokens": {
		// Input / output, in that order; use tokens_full to say which is which.
		Template: "{{ symbol }} {{ humantokens .ContextWindow.TotalInputTokens }} / {{ humantokens .ContextWindow.TotalOutputTokens }}",
		When:     "{{ .ContextWindow }}",
		Style:    "dim",
	},
	"tokens_full": {
		// Like tokens, but labelled.
		Template: "{{ symbol }} {{ humantokens .ContextWindow.TotalInputTokens }} in / {{ humantokens .ContextWindow.TotalOutputTokens }} out",
		When:     "{{ .ContextWindow }}",
		Style:    "dim",
	},
	"cache": {
		// Cache-read volume and hit ratio from the last API response. The hit
		// ratio is cache reads over all input tokens (fresh + cache write +
		// cache read), matching how used_percentage counts input.
		Template: "{{ symbol }} {{ with .ContextWindow.CurrentUsage }}{{ humantokens .CacheReadInputTokens }} ({{ pct .CacheReadInputTokens (add .InputTokens .CacheCreationInputTokens .CacheReadInputTokens) }}%){{ end }}",
		When:     "{{ .ContextWindow }} && {{ .ContextWindow.CurrentUsage }}",
		Style:    "dim",
	},
	"cache_full": {
		// Like cache, but labelled.
		Template: "{{ symbol }} {{ with .ContextWindow.CurrentUsage }}{{ humantokens .CacheReadInputTokens }} cached ({{ pct .CacheReadInputTokens (add .InputTokens .CacheCreationInputTokens .CacheReadInputTokens) }}%){{ end }}",
		When:     "{{ .ContextWindow }} && {{ .ContextWindow.CurrentUsage }}",
		Style:    "dim",
	},
	"rate_limit": {
		// Each window is guarded with "with" so a missing five_hour or
		// seven_day doesn't break the segment; " / " joins them only when both
		// are present. The "when" demands at least one window, because a
		// rate_limits with neither would otherwise render a lone symbol.
		Template: "{{ symbol }} {{ with .RateLimits.FiveHour }}5h: {{ printf \"%.0f\" .UsedPercentage }}%{{ end }}{{ if and .RateLimits.FiveHour .RateLimits.SevenDay }} / {{ end }}{{ with .RateLimits.SevenDay }}7d: {{ printf \"%.0f\" .UsedPercentage }}%{{ end }}",
		When:     "{{ .RateLimits }} && {{ or .RateLimits.FiveHour .RateLimits.SevenDay }}",
		Style:    "red",
	},
	"rate_limit_full": {
		// Like rate_limit, but also shows when each window resets.
		Template: "{{ symbol }} {{ with .RateLimits.FiveHour }}5h: {{ printf \"%.0f\" .UsedPercentage }}% (in {{ reltime .ResetsAt }}){{ end }}{{ if and .RateLimits.FiveHour .RateLimits.SevenDay }} / {{ end }}{{ with .RateLimits.SevenDay }}7d: {{ printf \"%.0f\" .UsedPercentage }}% (in {{ reltime .ResetsAt }}){{ end }}",
		When:     "{{ .RateLimits }} && {{ or .RateLimits.FiveHour .RateLimits.SevenDay }}",
		Style:    "red",
	},
	"output_style": {
		Template: "{{ symbol }} {{ .OutputStyle.Name }}",
		When:     "{{ .OutputStyle }} && {{ ne .OutputStyle.Name \"default\" }}",
		Style:    "dim",
	},
	"effort": {
		// Just the level, for a line that already reads unambiguously; use
		// effort_full when the value needs naming.
		Template: "{{ symbol }} {{ .Effort.Level }}",
		When:     "{{ .Effort }} && {{ .Effort.Level }}",
		Style:    "magenta",
	},
	"effort_full": {
		// Like effort, but labelled.
		Template: "{{ symbol }} effort:{{ .Effort.Level }}",
		When:     "{{ .Effort }} && {{ .Effort.Level }}",
		Style:    "magenta",
	},
	"thinking": {
		Template: "{{ symbol }} {{ if .Thinking.Enabled }}thinking{{ end }}",
		When:     "{{ .Thinking }} && {{ .Thinking.Enabled }}",
		Style:    "italic magenta",
	},
	"vim": {
		Template: "{{ symbol }} {{ .Vim.Mode }}",
		When:     "{{ .Vim }} && {{ .Vim.Mode }}",
		Style:    "bold yellow",
	},
	"agent": {
		Template: "{{ symbol }} {{ .Agent.Name }}",
		When:     "{{ .Agent }} && {{ .Agent.Name }}",
		Style:    "bold magenta",
		Symbol:   Symbols{"": "\U0001F916"},
	},
	"pr": {
		// Just the number: the "#" says what it is. Use pr_full for the label
		// and the review state.
		Template: "{{ symbol }} #{{ .PR.Number }}",
		When:     "{{ .PR }} && {{ gt .PR.Number 0 }}",
		Style:    "cyan",
	},
	"pr_full": {
		// Like pr, but labelled and with the review state.
		Template: "{{ symbol }} PR #{{ .PR.Number }}{{ with .PR.ReviewState }} {{ . }}{{ end }}",
		When:     "{{ .PR }} && {{ gt .PR.Number 0 }}",
		Style:    "cyan",
	},
	"worktree": {
		Template: "{{ symbol }} {{ .Worktree.Name }}",
		When:     "{{ .Worktree }} && {{ .Worktree.Name }}",
		Style:    "blue",
	},
	"session": {
		Template: "{{ symbol }} {{ .SessionName }}",
		When:     "{{ .SessionName }}",
		Style:    "dim",
	},
	"version": {
		Template: "{{ symbol }} v{{ .Version }}",
		When:     "{{ .Version }}",
		Style:    "dim",
	},
	"fast_mode": {
		Template: "{{ symbol }} fast",
		When:     "{{ .FastMode }}",
		Style:    "bold yellow",
		Symbol:   Symbols{"": "⚡"},
	},
	"exceeds_200k": {
		Template: "{{ symbol }} 200k+",
		When:     "{{ .Exceeds200K }}",
		Style:    "bold red",
		Symbol:   Symbols{"": "⚠"},
	},
	"text": {
		// No default template: the user supplies one.
		When: "true",
	},
}
