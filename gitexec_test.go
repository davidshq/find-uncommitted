package main

import (
	"context"
	"errors"
	"fmt"
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

func TestCheckRepoStatusRespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st := checkRepoStatus(ctx, t.TempDir())
	if st.Error == "" {
		t.Fatal("expected error on cancelled context")
	}
	lower := strings.ToLower(st.Error)
	if !strings.Contains(lower, "cancel") && !strings.Contains(lower, "timed out") {
		t.Fatalf("unexpected error: %q", st.Error)
	}
}

func TestPublishAgentSnapshotAbortsOnTickDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := AgentConfig{
		ScanRoot:     t.TempDir(),
		StateRepoDir: t.TempDir(),
		MachineID:    "test-machine",
		Sync: SyncConfig{
			StateRepoDir: t.TempDir(),
			MachineID:    "test-machine",
			Runner:       newScriptedGit(),
			RetryDelay:   time.Millisecond,
		},
	}
	_, _, err := publishAgentSnapshot(ctx, cfg)
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if !isGitContextErr(ctx, err) && !strings.Contains(strings.ToLower(err.Error()), "cancel") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncWarningUnwrapsContext(t *testing.T) {
	err := SyncWarning{
		Message: "state repo pull failed (will retry later)",
		Err:     fmt.Errorf("%w: hung", context.DeadlineExceeded),
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Unwrap broken: %v", err)
	}
	if !isGitContextErr(context.Background(), err) {
		t.Fatal("isGitContextErr should detect wrapped deadline")
	}
}
