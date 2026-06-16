package render

import (
	"testing"
	"time"
)

func TestHumanUntil(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{-time.Hour, "now"},
		{0, "now"},
		{30 * time.Second, "30s"},
		{13 * time.Minute, "13m"},
		{2*time.Hour + 13*time.Minute, "2h13m"},
		{3 * time.Hour, "3h"},
		{3*24*time.Hour + 4*time.Hour, "3d4h"},
		{2 * 24 * time.Hour, "2d"},
	}
	for _, tt := range cases {
		if got := humanUntil(tt.in); got != tt.want {
			t.Errorf("humanUntil(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHumantokens(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		999:     "999",
		15500:   "15.5k",
		1200:    "1.2k",
		2300000: "2.3M",
	}
	for in, want := range cases {
		if got := humantokens(in); got != want {
			t.Errorf("humantokens(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	cases := map[int64]string{
		0:     "0s",
		45000: "45s",
		90000: "1m30s",
	}
	for in, want := range cases {
		if got := duration(in); got != want {
			t.Errorf("duration(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestShortpath(t *testing.T) {
	if got := shortpath("/a/b/c/d", 3); got != "…/b/c/d" {
		t.Errorf("shortpath truncate = %q, want %q", got, "…/b/c/d")
	}
	if got := shortpath("/a/b/c", 3); got != "/a/b/c" {
		t.Errorf("shortpath no-truncate = %q, want %q", got, "/a/b/c")
	}
	if got := shortpath("", 3); got != "" {
		t.Errorf("shortpath empty = %q, want empty", got)
	}
}

func TestBasename(t *testing.T) {
	cases := map[string]string{
		"/Users/me/Projects/cc-status-go": "cc-status-go",
		"/current/working/directory":      "directory",
		"single":                          "single",
		"":                                "",
	}
	for in, want := range cases {
		if got := basename(in); got != want {
			t.Errorf("basename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTrunc(t *testing.T) {
	if got := trunc(3, "hello"); got != "hel…" {
		t.Errorf("trunc = %q, want %q", got, "hel…")
	}
	if got := trunc(10, "hi"); got != "hi" {
		t.Errorf("trunc no-cut = %q, want %q", got, "hi")
	}
}
