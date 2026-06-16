// Package input models the JSON payload that Claude Code feeds to the
// statusline command on stdin and exposes a forgiving parser for it.
package input

import "encoding/json"

// Input is the root object decoded from stdin. It becomes the "." context
// for every segment template, so field names are chosen to read well there
// (e.g. {{ .Model.DisplayName }}, {{ with .Cost }}...{{ end }}).
//
// Optional nested objects are pointers so templates can guard them with
// {{ if .PR }} / {{ with .Vim }} and so segment "when" defaults can simply
// test for nil.
type Input struct {
	Cwd            string         `json:"cwd"`
	SessionID      string         `json:"session_id"`
	SessionName    string         `json:"session_name"`
	TranscriptPath string         `json:"transcript_path"`
	Model          Model          `json:"model"`
	Workspace      Workspace      `json:"workspace"`
	Version        string         `json:"version"`
	OutputStyle    *OutputStyle   `json:"output_style"`
	Cost           *Cost          `json:"cost"`
	ContextWindow  *ContextWindow `json:"context_window"`
	Exceeds200K    bool           `json:"exceeds_200k_tokens"`
	Effort         *Effort        `json:"effort"`
	Thinking       *Thinking      `json:"thinking"`
	RateLimits     *RateLimits    `json:"rate_limits"`
	Vim            *Vim           `json:"vim"`
	Agent          *Agent         `json:"agent"`
	PR             *PR            `json:"pr"`
	Worktree       *Worktree      `json:"worktree"`
}

// Model identifies the active model.
type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// Workspace describes the directories and repository for the session.
type Workspace struct {
	CurrentDir  string   `json:"current_dir"`
	ProjectDir  string   `json:"project_dir"`
	AddedDirs   []string `json:"added_dirs"`
	GitWorktree string   `json:"git_worktree"`
	Repo        *Repo    `json:"repo"`
}

// Repo is the remote repository the workspace points at.
type Repo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// OutputStyle is the active output style.
type OutputStyle struct {
	Name string `json:"name"`
}

// Cost aggregates spend and edit volume for the session.
type Cost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
}

// ContextWindow describes token usage against the model's context window.
type ContextWindow struct {
	TotalInputTokens    int           `json:"total_input_tokens"`
	TotalOutputTokens   int           `json:"total_output_tokens"`
	ContextWindowSize   int           `json:"context_window_size"`
	UsedPercentage      float64       `json:"used_percentage"`
	RemainingPercentage float64       `json:"remaining_percentage"`
	CurrentUsage        *CurrentUsage `json:"current_usage"`
}

// CurrentUsage breaks down the tokens of the latest request.
type CurrentUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Effort is the reasoning effort level.
type Effort struct {
	Level string `json:"level"`
}

// Thinking reports whether extended thinking is enabled.
type Thinking struct {
	Enabled bool `json:"enabled"`
}

// RateLimits holds the rolling usage windows.
type RateLimits struct {
	FiveHour *RateLimit `json:"five_hour"`
	SevenDay *RateLimit `json:"seven_day"`
}

// RateLimit is a single usage window.
type RateLimit struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

// Vim is the current vim mode (when vim mode is on).
type Vim struct {
	Mode string `json:"mode"`
}

// Agent is the active subagent, if any.
type Agent struct {
	Name string `json:"name"`
}

// PR describes the pull request associated with the session.
type PR struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	ReviewState string `json:"review_state"`
}

// Worktree describes a Claude Code worktree checkout.
type Worktree struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	OriginalCwd    string `json:"original_cwd"`
	OriginalBranch string `json:"original_branch"`
}

// Parse decodes the stdin payload. Unknown fields are ignored so the tool
// stays forward-compatible with newer Claude Code versions. An empty input
// yields a zero-valued *Input rather than an error.
func Parse(data []byte) (*Input, error) {
	in := &Input{}
	if len(data) == 0 {
		return in, nil
	}
	if err := json.Unmarshal(data, in); err != nil {
		return in, err
	}
	return in, nil
}
