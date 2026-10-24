package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/shorrty/cc-status-go/internal/config"
)

// funcMap is the set of helpers available to segment templates, on top of the
// built-ins text/template provides (printf, eq, gt, lt, and, or, with, if...).
var funcMap = template.FuncMap{
	"shortpath":   shortpath,
	"basename":    basename,
	"branch":      branch,
	"sessions":    sessions,
	"duration":    duration,
	"reltime":     reltime,
	"humantokens": humantokens,
	"default":     defaultVal,
	"trunc":       trunc,
	"trimprefix":  trimprefix,
	"upper":       strings.ToUpper,
	"lower":       strings.ToLower,
	"add":         add,
	"pct":         pct,
	"env":         os.Getenv,
}

// symbolFunc binds one segment's symbols to the "symbol" helper, which is why
// it is built per template instead of living in funcMap: {{ symbol }} yields the
// unnamed symbol and {{ symbol "worktree" }} a named one. An unknown name falls
// back to the unnamed symbol, so a scalar override in the config covers every
// symbol of a multi-symbol segment. A segment with no symbols yields "", which
// the caller's TrimSpace then strips along with the space that followed it.
func symbolFunc(sym config.Symbols) func(...string) string {
	return func(name ...string) string {
		if len(name) > 0 {
			if s, ok := sym[name[0]]; ok {
				return s
			}
		}
		return sym[""]
	}
}

// shortpath collapses $HOME to "~" and keeps only the last n path components,
// prefixing an ellipsis when the path was truncated. n <= 0 keeps the full
// (home-collapsed) path.
func shortpath(path string, n int) string {
	if path == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if path == home {
			return "~"
		}
		if rest, ok := strings.CutPrefix(path, home+string(os.PathSeparator)); ok {
			path = "~" + string(os.PathSeparator) + rest
		}
	}
	if n <= 0 {
		return path
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	// Drop the leading empty element from an absolute path so counting is
	// intuitive (e.g. n=3 on /a/b/c/d -> b/c/d).
	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}
	if len(parts) <= n {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-n:], "/")
}

// basename returns the final path component (the project/folder name), e.g.
// "/Users/me/Projects/cc-status-go" -> "cc-status-go". Empty input yields "".
func basename(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

// branch returns the git branch checked out at dir, e.g. "main". The session
// JSON carries no current-branch field (workspace.git_worktree is a worktree
// name and absent in the main working tree; worktree.branch exists only in
// --worktree sessions), so it is read from .git/HEAD.
//
// Reading the file rather than shelling out to "git branch --show-current"
// keeps the segment in the microsecond range instead of the milliseconds a
// subprocess costs on every render. A detached HEAD has no branch and yields ""
// (as does "git branch --show-current"), and so does every failure path, which
// simply drops the segment.
func branch(dir string) string {
	gitDir := findGitDir(dir)
	if gitDir == "" {
		return ""
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	// "ref: refs/heads/<branch>" when on a branch, a raw commit sha when not.
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: refs/heads/")
	if !ok {
		return ""
	}
	return ref
}

// findGitDir walks up from dir to the filesystem root looking for ".git" and
// returns the directory holding that working tree's HEAD, or "" when dir is not
// inside a repository. A ".git" directory is the main working tree; a ".git"
// file holds "gitdir: <path>" pointing at the git directory of a linked
// worktree or submodule, which keeps its own HEAD.
func findGitDir(dir string) string {
	if dir == "" {
		return ""
	}
	for {
		dotGit := filepath.Join(dir, ".git")
		if info, err := os.Stat(dotGit); err == nil {
			if info.IsDir() {
				return dotGit
			}
			return resolveGitFile(dir, dotGit)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// resolveGitFile reads a ".git" file and returns the path its "gitdir:" line
// points at, resolving a relative target against the working tree holding it.
func resolveGitFile(worktree, dotGit string) string {
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return ""
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return ""
	}
	if target = strings.TrimSpace(target); target == "" {
		return ""
	}
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(worktree, target)
}

// sessions counts the Claude Code sessions running on this machine, or only
// those in a given status: {{ sessions }} for all of them, {{ sessions "busy" }}
// for the ones working right now. The session JSON describes the current
// session alone, so the count comes from the registry Claude Code keeps in its
// config directory ($CLAUDE_CONFIG_DIR, or ~/.claude): sessions/<pid>.json per
// running process, carrying its pid and its status ("busy" or "idle").
//
// A record counts only while its process is alive, since a session that crashed
// leaves its file behind. Every failure (no directory, an unreadable or
// malformed file) counts as no session, which simply drops the segment.
func sessions(status ...string) int {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return 0
		}
		dir = filepath.Join(home, ".claude")
	}
	dir = filepath.Join(dir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		// Skip the <pid>.<hash>.key files that sit next to the records.
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec struct {
			PID    int    `json:"pid"`
			Status string `json:"status"`
		}
		// A non-positive pid would make the liveness check address a process
		// group (or every process) instead of one process.
		if json.Unmarshal(data, &rec) != nil || rec.PID <= 0 || !alive(rec.PID) {
			continue
		}
		if len(status) > 0 && rec.Status != status[0] {
			continue
		}
		n++
	}
	return n
}

// duration formats a millisecond count as a compact human string (e.g. 45s,
// 1m30s, 2h5m).
func duration(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Minute:
		return d.Round(time.Second).String()
	case d < time.Hour:
		return d.Round(time.Second).Truncate(time.Second).String()
	default:
		return d.Round(time.Minute).String()
	}
}

// reltime formats the time remaining until a unix timestamp as a compact
// string ("45s", "13m", "2h13m", "3d4h"). Past timestamps render as "now".
func reltime(unixSec int64) string {
	return humanUntil(time.Until(time.Unix(unixSec, 0)))
}

// humanUntil renders a remaining duration compactly; non-positive durations are
// "now".
func humanUntil(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Round(time.Second).Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Round(time.Minute).Minutes()))
	case d < 24*time.Hour:
		d = d.Round(time.Minute)
		h := int(d.Hours())
		if m := int(d.Minutes()) - h*60; m != 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	default:
		d = d.Round(time.Hour)
		days := int(d.Hours()) / 24
		if h := int(d.Hours()) % 24; h != 0 {
			return fmt.Sprintf("%dd%dh", days, h)
		}
		return fmt.Sprintf("%dd", days)
	}
}

// humantokens renders a token count as 1.5k / 2.3M.
func humantokens(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}

// defaultVal returns fallback when val is empty/zero-ish, otherwise val.
func defaultVal(fallback string, val any) string {
	s := fmt.Sprintf("%v", val)
	if s == "" || s == "<nil>" {
		return fallback
	}
	return s
}

// add sums its arguments. With no arguments it returns 0. It is handy for
// building a denominator inline, since text/template has no arithmetic
// built-ins, e.g. pct .CacheRead (add .Input .CacheCreation .CacheRead).
func add(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// pct returns part/whole as a rounded integer percentage. A non-positive whole
// yields 0 rather than dividing by zero.
func pct(part, whole int) int {
	if whole <= 0 {
		return 0
	}
	return int(float64(part)/float64(whole)*100 + 0.5)
}

// trimprefix drops prefix from the front of s, leaving s alone when it does not
// start with prefix. The value comes last so templates can pipe into it, as with
// trunc and default; this is why strings.TrimPrefix is not used directly.
func trimprefix(prefix, s string) string {
	return strings.TrimPrefix(s, prefix)
}

// trunc shortens s to at most n runes, appending "…" when cut.
func trunc(n int, s string) string {
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
