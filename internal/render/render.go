// Package render executes the configured segments against the parsed input and
// joins them into the final status line.
package render

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/shorrty/cc-status-go/internal/config"
	"github.com/shorrty/cc-status-go/internal/input"
	"github.com/shorrty/cc-status-go/internal/style"
)

// Options tweaks rendering behavior.
type Options struct {
	// Debug writes per-segment template errors to stderr instead of swallowing
	// them.
	Debug bool
}

// Render evaluates every segment and joins the non-empty results with the
// configured separator. It never panics on a bad template: a failing segment is
// skipped (and reported to stderr when Options.Debug is set).
func Render(in *input.Input, cfg *config.Config, sty style.Renderer, opts Options) string {
	parts := make([]string, 0, len(cfg.Segments))
	for i, seg := range cfg.Segments {
		out, ok := renderSegment(in, seg, sty, opts, i)
		if ok {
			parts = append(parts, out)
		}
	}
	sep := cfg.Separator
	if sep == "" {
		sep = " "
	}
	return strings.Join(parts, sep)
}

// renderSegment evaluates one segment's when/template/style. It returns the
// styled string and true only when the segment should be shown.
func renderSegment(in *input.Input, seg config.Segment, sty style.Renderer, opts Options, idx int) (string, bool) {
	if seg.Template == "" {
		return "", false
	}
	if !visible(in, seg, opts, idx) {
		return "", false
	}
	body, err := execTemplate(fmt.Sprintf("seg%d", idx), seg.Template, in)
	if err != nil {
		debugf(opts, "segment %d (%s) template error: %v", idx, seg.Type, err)
		return "", false
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", false
	}
	return sty.Apply(seg.Style, body), true
}

// visible reports whether the segment's "when" condition holds. An empty
// condition is always visible. Otherwise the condition is a boolean expression
// of sub-templates joined by the infix operators "&&" and "||", with "&&"
// binding tighter than "||" (so "A && B || C && D" means "(A && B) || (C && D)").
// Each sub-template is rendered and checked for truthiness; both operators
// short-circuit left to right. This lets conditions read naturally, e.g.
//
//	"{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}"
//	"{{ .Workspace.GitWorktree }} || {{ .Worktree }} || {{ .Workspace.Repo }}"
//
// instead of the prefix forms "{{ and ... }}" / "{{ or ... }}". Short-circuiting
// keeps later parts safe from nil pointers: the guard "{{ .Cost }}" fails first,
// so "{{ gt .Cost.TotalCostUSD 0.0 }}" never runs.
func visible(in *input.Input, seg config.Segment, opts Options, idx int) bool {
	if strings.TrimSpace(seg.When) == "" {
		return true
	}
	// OR of AND-groups: visible when any group is fully truthy.
	for _, group := range splitOn(seg.When, "||") {
		conds := splitOn(group, "&&")
		if len(conds) == 0 {
			continue
		}
		if allTruthy(in, seg, opts, idx, conds) {
			return true
		}
	}
	return false
}

// allTruthy renders each AND sub-condition in order and returns false at the
// first falsy (or failing) one, leaving the rest unevaluated.
func allTruthy(in *input.Input, seg config.Segment, opts Options, idx int, conds []string) bool {
	for _, cond := range conds {
		out, err := execTemplate(fmt.Sprintf("seg%d-when", idx), cond, in)
		if err != nil {
			debugf(opts, "segment %d (%s) when error: %v", idx, seg.Type, err)
			return false
		}
		if !truthy(strings.TrimSpace(out)) {
			return false
		}
	}
	return true
}

// splitOn splits a "when" string on sep, trimming blanks. A string without sep
// yields a single element.
func splitOn(when, sep string) []string {
	var parts []string
	for part := range strings.SplitSeq(when, sep) {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// falsyTokens are the rendered "when" results treated as not-visible, on top of
// the empty string.
var falsyTokens = map[string]bool{
	"false":      true,
	"<nil>":      true, // nil pointer (e.g. {{ .Cost }} when absent)
	"<no value>": true, // nil interface / missing key
}

func truthy(s string) bool {
	return s != "" && !falsyTokens[s]
}

// execTemplate compiles and runs a single template against the input. Missing
// keys render as their zero value rather than erroring.
func execTemplate(name, text string, in *input.Input) (string, error) {
	tmpl, err := template.New(name).Funcs(funcMap).Option("missingkey=zero").Parse(text)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, in); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func debugf(opts Options, format string, args ...any) {
	if opts.Debug {
		fmt.Fprintf(os.Stderr, "cc-status: "+format+"\n", args...)
	}
}
