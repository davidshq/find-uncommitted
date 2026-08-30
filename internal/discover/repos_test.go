package discover

import (
	"os"
	"os/exec"
	"path/filepath"
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

	repos := FindGitRepos(root, WalkOptions{})
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

	repos := FindGitRepos(root, WalkOptions{})
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

	repos := FindGitRepos(root, WalkOptions{})
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

	repos := FindGitRepos(root, WalkOptions{Excludes: []string{state}})
	if contains(repos, state) {
		t.Errorf("excluded repo %q should not appear in %v", state, repos)
	}
	if !contains(repos, other) {
		t.Errorf("expected sibling %q in %v", other, repos)
	}
}
