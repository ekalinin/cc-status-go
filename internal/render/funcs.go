package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// funcMap is the set of helpers available to segment templates, on top of the
// built-ins text/template provides (printf, eq, gt, lt, and, or, with, if...).
var funcMap = template.FuncMap{
	"shortpath":   shortpath,
	"basename":    basename,
	"duration":    duration,
	"reltime":     reltime,
	"humantokens": humantokens,
	"default":     defaultVal,
	"trunc":       trunc,
	"upper":       strings.ToUpper,
	"lower":       strings.ToLower,
	"add":         add,
	"pct":         pct,
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

// trunc shortens s to at most n runes, appending "…" when cut.
func trunc(n int, s string) string {
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
