package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsEmptyRepositoryMessage(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   bool
	}{
		{"no commits yet", "fatal: your current branch 'main' does not have any commits yet", true},
		{"needed single revision", "fatal: Needed a single revision", true},
		{"ambiguous HEAD", "fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree", true},
		{"unknown revision alone", "fatal: ambiguous argument 'origin/main': unknown revision or path not in the working tree", false},
		{"invalid reference alone", "fatal: invalid reference: refs/remotes/origin/gone", false},
		{"deleted upstream shape", "fatal: no such ref: 'refs/remotes/origin/feature'", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isEmptyRepositoryMessage(tc.stderr, errors.New("exit status 128"))
			if got != tc.want {
				t.Fatalf("isEmptyRepositoryMessage(%q)=%v, want %v", tc.stderr, got, tc.want)
			}
		})
	}
}

func TestDeletedUpstreamNotMarkedEmpty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main", dir},
		{"-C", dir, "config", "user.email", "test@example.com"},
		{"-C", dir, "config", "user.name", "test"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "readme.txt"},
		{"commit", "-m", "init"},
		{"remote", "add", "origin", "https://example.com/org/app.git"},
		{"config", "branch.main.remote", "origin"},
		{"config", "branch.main.merge", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	st := checkRepoStatus(context.Background(), dir)
	if st.IsEmpty {
		t.Fatalf("non-empty repo with missing upstream must not be IsEmpty: %+v", st)
	}
	if st.Error == "" && !st.HasUntrackedUpstream {
		t.Fatalf("expected upstream error or untracked-upstream, got %+v", st)
	}
}

func TestClassifyUpstreamFailure(t *testing.T) {
	t.Run("no upstream", func(t *testing.T) {
		untracked, errMsg := classifyUpstreamFailure("fatal: no upstream configured\n", errors.New("exit status 128"))
		if !untracked || errMsg != "" {
			t.Fatalf("got untracked=%v err=%q", untracked, errMsg)
		}
	})
	t.Run("unknown fatal includes detail", func(t *testing.T) {
		untracked, errMsg := classifyUpstreamFailure("fatal: refusing to merge unrelated histories\n", errors.New("exit status 128"))
		if untracked || !strings.Contains(errMsg, "refusing to merge unrelated histories") {
			t.Fatalf("got untracked=%v err=%q", untracked, errMsg)
		}
	})
}

func TestInvalidRepositoryErrorIncludesDetail(t *testing.T) {
	msg := invalidRepositoryError("fatal: not a git repository\n", errors.New("exit status 128"))
	if !strings.Contains(msg, "not a git repository") {
		t.Fatalf("expected detail in %q", msg)
	}
	if strings.Contains(msg, "exit status 128") {
		t.Fatalf("expected stderr detail, got exec code in %q", msg)
	}
}

func TestCheckRepoStatusEmptyRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}

	st := checkRepoStatus(context.Background(), dir)
	if !st.IsEmpty {
		t.Fatalf("expected IsEmpty, got %+v", st)
	}
	if st.Error != "" {
		t.Fatalf("expected no error for empty repo, got %q", st.Error)
	}
	if snapshotNeedsAttention(st) {
		t.Fatal("empty repo should not need attention")
	}
}

func TestRevListCountFailureReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, _, err := revListCount(ctx, t.TempDir(), "@{u}..HEAD")
	if err == nil {
		t.Fatal("expected error from cancelled rev-list")
	}
	if n != 0 {
		t.Fatalf("count=%d, want 0 on failure", n)
	}
}

func TestFillAheadBehindFailureSetsErrorNotCleanFlags(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st := RepoSnapshot{}
	fillAheadBehind(ctx, &st, t.TempDir())
	if st.Error == "" {
		t.Fatal("expected Error when rev-list fails")
	}
	if !strings.Contains(st.Error, "timed out or cancelled") && !strings.Contains(st.Error, "ahead") {
		t.Fatalf("unexpected Error %q", st.Error)
	}
	if st.HasUnpushed || st.HasBehind || st.AheadCount != 0 || st.BehindCount != 0 {
		t.Fatalf("failure must not invent ahead/behind signal: %+v", st)
	}
	st.IsClean = st.Error == "" && !st.HasUnpushed && !st.HasBehind
	if st.IsClean {
		t.Fatal("failed rev-list must not leave IsClean true")
	}
}

func TestCheckRepoStatusWaitDelayNotInvalidRepo(t *testing.T) {
	st := RepoSnapshot{}
	if !setGitCancelled(context.Background(), &st, exec.ErrWaitDelay) {
		t.Fatal("expected WaitDelay to be classified as timeout")
	}
	if strings.Contains(st.Error, "Not a valid git repository") {
		t.Fatalf("unexpected invalid repo message: %q", st.Error)
	}
}

func TestDetectSituationsSkipsEmptyRepoError(t *testing.T) {
	rows := []AggregateRow{{
		Machine: "local",
		Local:   true,
		Repo: RepoSnapshot{
			Path:    filepath.Join("hutsell", "won"),
			Branch:  "main",
			IsEmpty: true,
		},
	}}
	situations := DetectSituations(rows)
	for _, s := range situations {
		if s.Kind == SituationLocalError {
			t.Fatalf("unexpected local error situation: %+v", s)
		}
	}
}

func TestDetectSituationsUpstreamFatalDetail(t *testing.T) {
	rows := []AggregateRow{{
		Machine: "local",
		Local:   true,
		Repo: RepoSnapshot{
			Path:   "/code/app",
			Origin: "github.com/org/app",
			Error:  "Failed to check upstream tracking: refusing to merge unrelated histories",
		},
	}}
	situations := DetectSituations(rows)
	if len(situations) != 1 || situations[0].Kind != SituationLocalError {
		t.Fatalf("expected one local error, got %+v", situations)
	}
	if !strings.Contains(situations[0].Nudge, "refusing to merge unrelated histories") {
		t.Fatalf("expected stderr detail in nudge: %q", situations[0].Nudge)
	}
}

func TestRepoIsEmptyIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	if !repoIsEmpty(context.Background(), dir) {
		t.Fatal("expected empty repo")
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Local identity so commits work on CI runners with no global user.*.
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "readme.txt"},
		{"commit", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if repoIsEmpty(context.Background(), dir) {
		t.Fatal("expected non-empty repo after commit")
	}
}

func gitRepoWithCommit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "readme.txt")
	run("commit", "-q", "-m", "init")
	return dir
}

// S-2: `git branch --show-current` exits 0 with empty output on a detached HEAD
// (every submodule, mid-rebase/bisect). That used to fall through to @{u} and
// become an error, which also hid real dirty state.
func TestCheckRepoStatusDetachedHeadIsNotError(t *testing.T) {
	dir := gitRepoWithCommit(t)
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "--detach").CombinedOutput(); err != nil {
		t.Fatalf("detach: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := checkRepoStatus(context.Background(), dir)
	if st.Error != "" {
		t.Fatalf("detached HEAD must not be an error, got %q", st.Error)
	}
	if !strings.HasPrefix(st.Branch, "detached HEAD (") {
		t.Fatalf("expected detached HEAD (sha) branch label, got %q", st.Branch)
	}
	if !st.IsDirty {
		t.Fatalf("dirty detached repo must report dirty: %+v", st)
	}
	if st.HasUnpushed {
		t.Fatalf("detached HEAD on a branch tip has nothing unreachable: %+v", st)
	}
}

// Commits made on a detached HEAD that no branch, remote ref or tag contains
// are lost on the next checkout; they must not read as clean.
func TestCheckRepoStatusDetachedHeadUnreachableCommitsAreUnpushed(t *testing.T) {
	dir := gitRepoWithCommit(t)
	for _, args := range [][]string{
		{"checkout", "-q", "--detach"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "orphan 1"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "orphan 2"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	st := checkRepoStatus(context.Background(), dir)
	if st.Error != "" {
		t.Fatalf("unexpected error %q", st.Error)
	}
	if !st.HasUnpushed || st.AheadCount != 2 {
		t.Fatalf("expected 2 unreachable commits as unpushed, got %+v", st)
	}
	if st.IsClean {
		t.Fatalf("detached HEAD with unreachable commits must not be clean: %+v", st)
	}
}

// S-3: a branch whose upstream was merged and pruned ("gone") is the normal end
// state of a feature branch. It is a no-usable-upstream cue, not a git error.
func TestCheckRepoStatusGoneUpstreamIsCueNotError(t *testing.T) {
	dir := gitRepoWithCommit(t)
	for _, args := range [][]string{
		{"remote", "add", "origin", "https://example.com/org/app.git"},
		{"config", "branch.main.remote", "origin"},
		{"config", "branch.main.merge", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	st := checkRepoStatus(context.Background(), dir)
	if st.Error != "" {
		t.Fatalf("gone upstream must not be an error, got %q", st.Error)
	}
	if !st.HasUntrackedUpstream {
		t.Fatalf("gone upstream should surface as untracked-upstream cue: %+v", st)
	}
	if st.IsClean {
		t.Fatal("gone upstream must not read as clean")
	}
}
