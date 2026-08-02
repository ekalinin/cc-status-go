package render

import (
	"os"
	"path/filepath"
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

func TestAdd(t *testing.T) {
	if got := add(); got != 0 {
		t.Errorf("add() = %d, want 0", got)
	}
	if got := add(8500, 5000, 2000); got != 15500 {
		t.Errorf("add(8500, 5000, 2000) = %d, want 15500", got)
	}
}

func TestPct(t *testing.T) {
	cases := []struct {
		part, whole, want int
	}{
		{2000, 15500, 13}, // rounds 12.9 up
		{1, 3, 33},        // rounds 33.3 down
		{2, 3, 67},        // rounds 66.6 up
		{5, 0, 0},         // divide-by-zero guard
		{0, 100, 0},
	}
	for _, tt := range cases {
		if got := pct(tt.part, tt.whole); got != tt.want {
			t.Errorf("pct(%d, %d) = %d, want %d", tt.part, tt.whole, got, tt.want)
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

// writeFile creates path with the given contents, making parent directories.
func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBranch(t *testing.T) {
	// Main working tree: .git is a directory holding HEAD.
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/chore/ci\n")
	if got := branch(repo); got != "chore/ci" {
		t.Errorf("branch(main tree) = %q, want %q", got, "chore/ci")
	}

	// A nested directory finds the repository by walking up.
	nested := filepath.Join(repo, "internal", "render")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := branch(nested); got != "chore/ci" {
		t.Errorf("branch(nested) = %q, want %q", got, "chore/ci")
	}

	// Detached HEAD has no branch.
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "9fceb02b0b4e1e2c0e5b2a1d3f4a5b6c7d8e9f00\n")
	if got := branch(repo); got != "" {
		t.Errorf("branch(detached) = %q, want %q", got, "")
	}

	// Outside a repository, and on empty input.
	if got := branch(t.TempDir()); got != "" {
		t.Errorf("branch(no repo) = %q, want %q", got, "")
	}
	if got := branch(""); got != "" {
		t.Errorf("branch(empty) = %q, want %q", got, "")
	}
}

// TestBranchLinkedWorktree covers a ".git" file pointing at the worktree's own
// git directory, which holds a HEAD distinct from the main working tree's.
func TestBranchLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "repo")
	writeFile(t, filepath.Join(main, ".git", "HEAD"), "ref: refs/heads/main\n")
	wtGitDir := filepath.Join(main, ".git", "worktrees", "feature")
	writeFile(t, filepath.Join(wtGitDir, "HEAD"), "ref: refs/heads/feature-xyz\n")

	// Absolute gitdir target.
	abs := filepath.Join(root, "wt-abs")
	writeFile(t, filepath.Join(abs, ".git"), "gitdir: "+wtGitDir+"\n")
	if got := branch(abs); got != "feature-xyz" {
		t.Errorf("branch(abs gitdir) = %q, want %q", got, "feature-xyz")
	}

	// Relative gitdir target, resolved against the working tree holding it.
	rel := filepath.Join(root, "wt-rel")
	writeFile(t, filepath.Join(rel, ".git"), "gitdir: ../repo/.git/worktrees/feature\n")
	if got := branch(rel); got != "feature-xyz" {
		t.Errorf("branch(rel gitdir) = %q, want %q", got, "feature-xyz")
	}

	// A ".git" file that is not a gitdir pointer must not crash or resolve.
	junk := filepath.Join(root, "wt-junk")
	writeFile(t, filepath.Join(junk, ".git"), "not a gitdir line\n")
	if got := branch(junk); got != "" {
		t.Errorf("branch(junk .git) = %q, want %q", got, "")
	}
}
