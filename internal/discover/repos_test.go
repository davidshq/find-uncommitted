package discover

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, out)
	}
	// Local identity so commits work on CI runners with no global user.*.
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
}

func mustFind(t *testing.T, root string, opts WalkOptions) []string {
	t.Helper()
	repos, err := FindGitRepos(root, opts)
	if err != nil {
		t.Fatalf("FindGitRepos(%q): %v", root, err)
	}
	return repos
}

func contains(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// A .git *file* marks a linked worktree or submodule. Only matching .git
// directories made those invisible — exactly where unfinished work hides.
func TestFindGitReposDetectsWorktreeGitFile(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	gitInit(t, main)
	if out, err := exec.Command("git", "-C", main, "commit", "-q", "--allow-empty", "-m", "x").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v (%s)", err, out)
	}
	wt := filepath.Join(root, "wt2")
	if out, err := exec.Command("git", "-C", main, "worktree", "add", "-q", wt, "-b", "feature").CombinedOutput(); err != nil {
		t.Skipf("git worktree unavailable: %v (%s)", err, out)
	}

	info, err := os.Stat(filepath.Join(wt, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Skip("this git version uses a .git directory for worktrees")
	}

	repos := mustFind(t, root, WalkOptions{})
	if !contains(repos, main) {
		t.Errorf("expected main clone %q in %v", main, repos)
	}
	if !contains(repos, wt) {
		t.Errorf("expected linked worktree %q in %v", wt, repos)
	}
}

// A hidden scan root was explicitly requested by the user, so it must not be
// skipped by the leading-dot rule — that silently reported zero repositories.
func TestFindGitReposScansHiddenRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".dotfiles")
	proj := filepath.Join(root, "proj")
	gitInit(t, proj)

	repos := mustFind(t, root, WalkOptions{})
	if !contains(repos, proj) {
		t.Fatalf("expected %q under hidden root, got %v", proj, repos)
	}
}

// Hidden directories *below* the root are still skipped.
func TestFindGitReposStillSkipsNestedHiddenDirs(t *testing.T) {
	root := t.TempDir()
	visible := filepath.Join(root, "visible")
	hidden := filepath.Join(root, ".cache", "buried")
	gitInit(t, visible)
	gitInit(t, hidden)

	repos := mustFind(t, root, WalkOptions{})
	if !contains(repos, visible) {
		t.Errorf("expected %q in %v", visible, repos)
	}
	if contains(repos, hidden) {
		t.Errorf("did not expect nested hidden repo %q in %v", hidden, repos)
	}
}

// Excluding the state repo must not stop the walk from finding its siblings.
func TestFindGitReposExcludeKeepsSiblings(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	other := filepath.Join(root, "other")
	gitInit(t, state)
	gitInit(t, other)

	repos := mustFind(t, root, WalkOptions{Excludes: []string{state}})
	if contains(repos, state) {
		t.Errorf("excluded repo %q should not appear in %v", state, repos)
	}
	if !contains(repos, other) {
		t.Errorf("expected sibling %q in %v", other, repos)
	}
}

func TestFindGitReposStopsOnCancel(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		gitInit(t, filepath.Join(root, "r"+strconv.Itoa(i)))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repos := mustFind(t, root, WalkOptions{Context: ctx})
	if len(repos) != 0 {
		t.Fatalf("expected no repos when cancelled before walk, got %d", len(repos))
	}
}

// A missing root used to be swallowed as "zero repos", which the agent then
// published as an empty snapshot that peers read as "all clear".
func TestFindGitReposMissingRootIsError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "unmounted")
	repos, err := FindGitRepos(root, WalkOptions{})
	if err == nil {
		t.Fatalf("expected error for missing root, got repos %v", repos)
	}
}

func TestFindGitReposFileRootIsError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FindGitRepos(root, WalkOptions{}); err == nil {
		t.Fatal("expected error for non-directory root")
	}
}

// filepath.Walk Lstats the root, so a symlinked root (~/repos -> /data/repos)
// was never descended. Repos must be found and reported under the link path,
// and exclusions given under the link path must still apply.
func TestFindGitReposFollowsSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "data")
	gitInit(t, filepath.Join(real, "proj"))
	gitInit(t, filepath.Join(real, "state"))
	link := filepath.Join(base, "repos")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	repos := mustFind(t, link, WalkOptions{Excludes: []string{filepath.Join(link, "state")}})
	if !contains(repos, filepath.Join(link, "proj")) {
		t.Fatalf("expected %q under symlinked root, got %v", filepath.Join(link, "proj"), repos)
	}
	if len(repos) != 1 {
		t.Fatalf("expected only proj (state excluded), got %v", repos)
	}
}

// A root that exists but can't be listed (permissions, stale mount) passes
// Stat, then fails inside Walk — it must still be an error, not zero repos.
func TestFindGitReposUnreadableRootIsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	lockDirListing(t, root)
	if repos, err := FindGitRepos(root, WalkOptions{}); err == nil {
		t.Fatalf("expected error for unreadable root, got repos %v", repos)
	}
}
