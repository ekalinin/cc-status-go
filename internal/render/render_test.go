package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shorrty/cc-status-go/internal/config"
	"github.com/shorrty/cc-status-go/internal/input"
	"github.com/shorrty/cc-status-go/internal/style"
)

// repoRoot walks up from this test file to the module root so testdata is found
// regardless of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestRenderGolden(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "testdata", "sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := input.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "testdata", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
	want := "Opus │ directory │ 🌳 feature-xyz │ 8% │ 💰 $0.01 (+156/-23) │ 5h: 24% / 7d: 41% │ NORMAL │ 🤖 security-reviewer │ #1234 │ v2.1.90"
	if got != want {
		t.Errorf("Render mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestRenderDropsAbsentSegments verifies that segments whose data is missing
// disappear together with their separators.
func TestRenderDropsAbsentSegments(t *testing.T) {
	in, _ := input.Parse([]byte(`{"model":{"display_name":"Sonnet"},"workspace":{"current_dir":"/x/y"},"version":"2.0"}`))
	cfg := &config.Config{
		Separator: " │ ",
		Segments: []config.Segment{
			{Type: "model"}, {Type: "cwd"}, {Type: "cost"}, {Type: "pr"}, {Type: "vim"},
		},
	}
	// Mimic config.Load's default-merge for a hand-built config.
	applyDefaultsForTest(cfg)

	got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
	if strings.Contains(got, "#") || strings.Contains(got, "$") {
		t.Errorf("absent segments leaked into output: %q", got)
	}
	if strings.Count(got, "│") != 1 { // only model │ cwd
		t.Errorf("unexpected separator count in %q", got)
	}
}

// TestWhenInfixAnd checks the "&&" infix conditions: a segment shows only when
// every part is truthy, and the leading nil-guard short-circuits so later parts
// never dereference an absent pointer.
func TestWhenInfixAnd(t *testing.T) {
	cfg := &config.Config{
		Separator: " ",
		Segments: []config.Segment{{
			Type:     "cost",
			Template: "${{ printf \"%.2f\" .Cost.TotalCostUSD }}",
			When:     "{{ .Cost }} && {{ gt .Cost.TotalCostUSD 0.0 }}",
		}},
	}
	cases := map[string]string{
		`{}`:                              "",      // Cost nil: hidden, no error
		`{"cost":{"total_cost_usd":0}}`:   "",      // present but zero: hidden
		`{"cost":{"total_cost_usd":0.5}}`: "$0.50", // present and > 0: shown
	}
	for payload, want := range cases {
		in, _ := input.Parse([]byte(payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != want {
			t.Errorf("payload %s => %q, want %q", payload, got, want)
		}
	}
}

// TestWhenInfixOr checks "||" groups and "&&"-over-"||" precedence: the segment
// shows when any AND-group is fully truthy.
func TestWhenInfixOr(t *testing.T) {
	seg := config.Segment{
		Type:     "text",
		Template: "X",
		// (vim mode) OR (agent present AND named): "A && B || C"-style precedence.
		When: "{{ .Vim }} && {{ .Vim.Mode }} || {{ .Agent }} && {{ .Agent.Name }}",
	}
	cfg := &config.Config{Separator: " ", Segments: []config.Segment{seg}}
	cases := map[string]bool{
		`{}`:                            false, // neither group
		`{"vim":{"mode":"NORMAL"}}`:     true,  // first group
		`{"agent":{"name":"reviewer"}}`: true,  // second group
		`{"vim":{"mode":""}}`:           false, // first group present but empty mode
		`{"agent":{}}`:                  false, // agent present but no name
	}
	for payload, wantVisible := range cases {
		in, _ := input.Parse([]byte(payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if (got == "X") != wantVisible {
			t.Errorf("payload %s => %q, wantVisible=%v", payload, got, wantVisible)
		}
	}
}

// TestRateLimitFullPartial guards against a regression where a present
// rate_limits with only one window crashed the template (nil pointer on the
// missing window) and silently dropped the whole segment.
func TestRateLimitFullPartial(t *testing.T) {
	cfg, err := config.Load(writeTempConfig(t, "segments:\n  - type: rate_limit_full\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		`{"rate_limits":{"five_hour":{"used_percentage":50,"resets_at":0}}}`: "5h: 50% (in now)",
		`{"rate_limits":{"seven_day":{"used_percentage":40,"resets_at":0}}}`: "7d: 40% (in now)",
		`{"rate_limits":{}}`: "", // present but no windows
		`{}`:                 "", // absent
	}
	for payload, want := range cases {
		in, _ := input.Parse([]byte(payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != want {
			t.Errorf("payload %s => %q, want %q", payload, got, want)
		}
	}
}

// TestEffortSegments checks the bare and labelled variants, and that both drop
// out when "effort" is absent or carries no level.
func TestEffortSegments(t *testing.T) {
	cfg, err := config.Load(writeTempConfig(t, "separator: \" | \"\nsegments:\n  - type: effort\n  - type: effort_full\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		`{"effort":{"level":"high"}}`: "high | effort:high",
		`{"effort":{"level":""}}`:     "", // present but unset
		`{}`:                          "", // absent
	}
	for payload, want := range cases {
		in, _ := input.Parse([]byte(payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != want {
			t.Errorf("payload %s => %q, want %q", payload, got, want)
		}
	}
}

// TestShortAndFullSegments checks the bare/labelled pairs: the plain type
// renders the numbers alone, "<type>_full" spells them out, and both drop on the
// same absent data.
func TestShortAndFullSegments(t *testing.T) {
	cases := []struct{ segments, payload, want string }{
		{
			"  - type: context\n  - type: context_full\n",
			`{"context_window":{"used_percentage":8}}`,
			"8% | 8% ctx",
		},
		{
			"  - type: tokens\n  - type: tokens_full\n",
			`{"context_window":{"total_input_tokens":15500,"total_output_tokens":1200}}`,
			"15.5k / 1.2k | 15.5k in / 1.2k out",
		},
		{
			"  - type: cache\n  - type: cache_full\n",
			`{"context_window":{"current_usage":{"input_tokens":8500,"cache_creation_input_tokens":5000,"cache_read_input_tokens":2000}}}`,
			"2.0k (13%) | 2.0k cached (13%)",
		},
		{
			"  - type: pr\n  - type: pr_full\n",
			`{"pr":{"number":1234,"review_state":"pending"}}`,
			"#1234 | PR #1234 pending",
		},
		// No context_window / no pr: every variant drops out.
		{"  - type: context\n  - type: context_full\n", `{}`, ""},
		{"  - type: tokens\n  - type: tokens_full\n", `{}`, ""},
		{"  - type: cache\n  - type: cache_full\n", `{}`, ""},
		{"  - type: pr\n  - type: pr_full\n", `{}`, ""},
	}
	for _, c := range cases {
		cfg, err := config.Load(writeTempConfig(t, "separator: \" | \"\nsegments:\n"+c.segments))
		if err != nil {
			t.Fatalf("config %q: %v", c.segments, err)
		}
		in, _ := input.Parse([]byte(c.payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != c.want {
			t.Errorf("segments %q on %s => %q, want %q", c.segments, c.payload, got, c.want)
		}
	}
}

// TestNewSegments covers the segment types added on top of the original set:
// the fast_mode/exceeds_200k badges, repo/project/lines, and the cache segment
// with its volume-and-hit-ratio template.
func TestNewSegments(t *testing.T) {
	cfg, err := config.Load(writeTempConfig(t, "separator: \" | \"\nsegments:\n"+
		"  - type: fast_mode\n  - type: exceeds_200k\n  - type: repo\n"+
		"  - type: project\n  - type: lines\n  - type: cache\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		// Everything present and truthy.
		`{"fast_mode":true,"exceeds_200k_tokens":true,` +
			`"workspace":{"current_dir":"/a/app","project_dir":"/a/proj","repo":{"owner":"anthropics","name":"claude-code"}},` +
			`"cost":{"total_lines_added":156,"total_lines_removed":23},` +
			`"context_window":{"current_usage":{"input_tokens":8500,"cache_creation_input_tokens":5000,"cache_read_input_tokens":2000}}}`: "⚡ fast | ⚠ 200k+ | anthropics/claude-code | proj | +156/-23 | 2.0k (13%)",
		// Badges off; project_dir == current_dir hides project; no repo/usage.
		`{"fast_mode":false,"exceeds_200k_tokens":false,` +
			`"workspace":{"current_dir":"/a/app","project_dir":"/a/app"},` +
			`"cost":{"total_lines_added":0,"total_lines_removed":0}}`: "+0/-0",
	}
	for payload, want := range cases {
		in, _ := input.Parse([]byte(payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != want {
			t.Errorf("payload %s =>\n got: %q\nwant: %q", payload, got, want)
		}
	}
}

// TestSegmentSymbolOverride checks the "symbol" field end to end: both YAML
// shapes reach the template, an empty symbol leaves no stray space, and a scalar
// on a multi-symbol type covers every one of its symbols.
func TestSegmentSymbolOverride(t *testing.T) {
	// git_worktree keeps the git segment off the filesystem: it never falls
	// through to reading .git/HEAD.
	const payload = `{"workspace":{"git_worktree":"feature-xyz"},"cost":{"total_cost_usd":0.5}}`
	cases := map[string]string{
		"  - type: cost\n":                                "💰 $0.50 (+0/-0)", // default
		"  - type: cost\n    symbol: \"$\"\n":             "$ $0.50 (+0/-0)", // scalar
		"  - type: cost\n    symbol: \"\"\n":              "$0.50 (+0/-0)",   // dropped, no leading space
		"  - type: git\n":                                 "🌳 feature-xyz",   // default
		"  - type: git\n    symbol:\n      worktree: W\n": "W feature-xyz",   // one named symbol
		"  - type: git\n    symbol: \"@\"\n":              "@ feature-xyz",   // scalar covers every name
	}
	for segment, want := range cases {
		cfg, err := config.Load(writeTempConfig(t, "segments:\n"+segment))
		if err != nil {
			t.Fatalf("config %q: %v", segment, err)
		}
		in, _ := input.Parse([]byte(payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != want {
			t.Errorf("segment %q => %q, want %q", segment, got, want)
		}
	}
}

// TestSymbolOnGlyphlessType checks that "symbol" works on the types that ship
// no glyph of their own, without the user having to restate the template - and
// that a type whose template can render "" does not leave the symbol behind as
// the whole segment.
func TestSymbolOnGlyphlessType(t *testing.T) {
	cases := []struct{ segment, payload, want string }{
		{"  - type: model\n", `{"model":{"display_name":"Opus"}}`, "Opus"},
		{"  - type: model\n    symbol: \"🧠\"\n", `{"model":{"display_name":"Opus"}}`, "🧠 Opus"},
		{
			"  - type: rate_limit_full\n    symbol: \"⏳\"\n",
			`{"rate_limits":{"five_hour":{"used_percentage":24,"resets_at":0}}}`,
			"⏳ 5h: 24% (in now)",
		},
		{
			// rate_limits present but with no window: the segment drops whole,
			// symbol included.
			"  - type: rate_limit_full\n    symbol: \"⏳\"\n",
			`{"rate_limits":{}}`,
			"",
		},
	}
	for _, c := range cases {
		cfg, err := config.Load(writeTempConfig(t, "segments:\n"+c.segment))
		if err != nil {
			t.Fatalf("config %q: %v", c.segment, err)
		}
		in, _ := input.Parse([]byte(c.payload))
		got := Render(in, cfg, style.Renderer{Disabled: true}, Options{})
		if got != c.want {
			t.Errorf("segment %q on %s => %q, want %q", c.segment, c.payload, got, c.want)
		}
	}
}

// TestSymbolDefaultsMerge covers how a partial override meets the type's
// defaults: named symbols merge key by key, while a scalar replaces the set.
func TestSymbolDefaultsMerge(t *testing.T) {
	cfg, err := config.Load(writeTempConfig(t, "segments:\n"+
		"  - type: git\n    symbol:\n      worktree: W\n"+
		"  - type: git\n    symbol: \"@\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Segments[0].Symbol["branch"]; got != "🌿" {
		t.Errorf("named override dropped the default branch symbol: %q", got)
	}
	if _, ok := cfg.Segments[1].Symbol["branch"]; ok {
		t.Error("scalar override should replace the named defaults, not merge with them")
	}
}

// TestSymbolBadShape checks that a "symbol" that is neither a scalar nor a
// mapping is reported as a config error rather than panicking.
func TestSymbolBadShape(t *testing.T) {
	_, err := config.Load(writeTempConfig(t, "segments:\n  - type: cost\n    symbol: [a, b]\n"))
	if err == nil {
		t.Fatal("want an error for a sequence symbol, got nil")
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// applyDefaultsForTest reuses the public Load path by writing a temp file would
// be overkill; instead we round-trip through DefaultYAML-derived defaults by
// loading an equivalent in-memory config. Since applyDefaults is unexported in
// config, we reconstruct via config.Load on a temp file.
func applyDefaultsForTest(cfg *config.Config) {
	tmp, err := os.CreateTemp("", "cc-*.yaml")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("separator: \" │ \"\nsegments:\n  - type: model\n  - type: cwd\n  - type: cost\n  - type: pr\n  - type: vim\n")
	tmp.Close()
	loaded, err := config.Load(tmp.Name())
	if err != nil {
		panic(err)
	}
	cfg.Segments = loaded.Segments
}
