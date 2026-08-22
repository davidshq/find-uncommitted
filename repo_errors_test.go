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

func TestFormatGitError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		err    error
		want   string
	}{
		{
			name:   "fatal line trimmed",
			stderr: "fatal: no such branch: 'main'\n",
			err:    errors.New("exit status 128"),
			want:   "no such branch: 'main'",
		},
		{
			name:   "multiline prefers first fatal",
			stderr: "hint: something\nfatal: refusing to merge\nfatal: second\n",
			err:    errors.New("exit status 128"),
			want:   "refusing to merge",
		},
		{
			name:   "empty stderr falls back to err",
			stderr: "",
			err:    errors.New("exit status 128"),
			want:   "exit status 128",
		},
		{
			name:   "non fatal first line",
			stderr: "error: something failed\n",
			err:    errors.New("exit status 1"),
			want:   "error: something failed",
		},
		{
			name:   "long stderr truncated",
			stderr: "fatal: " + strings.Repeat("x", 300),
			err:    errors.New("exit status 128"),
			want:   strings.Repeat("x", 197) + "...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatGitError(tt.stderr, tt.err); got != tt.want {
				t.Fatalf("formatGitError() = %q, want %q", got, tt.want)
			}
		})
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
	if repoNeedsAttention(st) {
		t.Fatal("empty repo should not need attention")
	}
}

func TestCheckRepoStatusWaitDelayNotInvalidRepo(t *testing.T) {
	st := RepoStatus{}
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
	if out, err := exec.Command("git", "-C", dir, "add", "readme.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (%s)", err, out)
	}
	if repoIsEmpty(context.Background(), dir) {
		t.Fatal("expected non-empty repo after commit")
	}
}
