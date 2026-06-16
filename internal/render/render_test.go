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
	want := "Opus │ directory │ feature-xyz │ 8% ctx │ 💰 $0.01 (+156/-23) │ 5h: 24% / 7d: 41% │ NORMAL │ 🤖 security-reviewer │ PR #1234 pending │ v2.1.90"
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
	if strings.Contains(got, "PR") || strings.Contains(got, "$") {
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
