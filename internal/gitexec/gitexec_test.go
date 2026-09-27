package gitexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecGitRunnerCancelsOnContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep-based cancel test is unix-oriented")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	// Fake git that leaves a child holding pipes — exercises process-group kill.
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	started := time.Now()
	_, _, err := ExecGitRunner{Timeout: 5 * time.Second}.Run(ctx, dir, "status")
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("want context deadline/cancel, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("cancel took too long: %s", elapsed)
	}
}

func TestExecGitRunnerPerCommandTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep-based timeout test is unix-oriented")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	started := time.Now()
	_, _, err := ExecGitRunner{Timeout: 80 * time.Millisecond}.Run(context.Background(), dir, "status")
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
}

func TestExecGitRunnerRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatal(err)
	}
	_, _, err := ExecGitRunner{}.Run(context.Background(), dir, "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
}

func TestIsContextErrWaitDelay(t *testing.T) {
	if !IsContextErr(context.Background(), exec.ErrWaitDelay) {
		t.Fatal("exec.ErrWaitDelay should be treated as timeout")
	}
}

func TestResolvedMaxWorkers(t *testing.T) {
	if got := ResolvedMaxWorkers(0); got != DefaultMaxWorkers {
		t.Fatalf("default workers = %d, want %d", got, DefaultMaxWorkers)
	}
	if got := ResolvedMaxWorkers(4); got != 4 {
		t.Fatalf("explicit workers = %d, want 4", got)
	}
}

func TestRepoCheckWorkerCount(t *testing.T) {
	if got := RepoCheckWorkerCount(0, 100); got != DefaultMaxWorkers {
		t.Fatalf("default workers = %d, want %d", got, DefaultMaxWorkers)
	}
	if got := RepoCheckWorkerCount(4, 100); got != 4 {
		t.Fatalf("explicit workers = %d, want 4", got)
	}
	if got := RepoCheckWorkerCount(20, 5); got != 5 {
		t.Fatalf("workers capped to repo count = %d, want 5", got)
	}
}

func TestDefaultCommandTimeoutStringMatchesTypedDefault(t *testing.T) {
	parsed, err := time.ParseDuration(DefaultCommandTimeoutString)
	if err != nil {
		t.Fatalf("DefaultCommandTimeoutString %q: %v", DefaultCommandTimeoutString, err)
	}
	if parsed != 30*time.Second {
		t.Fatalf("DefaultCommandTimeoutString = %q (→ %s), want 30s", DefaultCommandTimeoutString, parsed)
	}
	if DefaultCommandTimeout != 30*time.Second {
		t.Fatalf("DefaultCommandTimeout = %s, want 30s", DefaultCommandTimeout)
	}
}

func TestExecGitRunnerSkipsStartWhenContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := ExecGitRunner{}.Run(ctx, t.TempDir(), "status")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestFormatError(t *testing.T) {
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
			if got := FormatError(tt.stderr, tt.err); got != tt.want {
				t.Fatalf("FormatError() = %q, want %q", got, tt.want)
			}
		})
	}
}
