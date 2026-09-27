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
