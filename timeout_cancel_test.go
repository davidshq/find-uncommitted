package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidshq/find-uncommitted/internal/gitexec"
)

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
	if !gitexec.IsContextErr(ctx, err) && !strings.Contains(strings.ToLower(err.Error()), "cancel") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// S-1: an unmounted/missing scan root must not publish an empty snapshot that
// peers read as "all clear" — the tick fails and nothing is written.
func TestPublishAgentSnapshotMissingRootDoesNotPublish(t *testing.T) {
	state := t.TempDir()
	cfg := AgentConfig{
		ScanRoot:     filepath.Join(t.TempDir(), "unmounted"),
		StateRepoDir: state,
		MachineID:    "test-machine",
		Sync: SyncConfig{
			StateRepoDir: state,
			MachineID:    "test-machine",
			Runner:       newScriptedGit(),
			RetryDelay:   time.Millisecond,
		},
	}
	_, committed, err := publishAgentSnapshot(context.Background(), cfg)
	if err == nil || committed {
		t.Fatalf("expected scan-root error and no publish, got committed=%v err=%v", committed, err)
	}
	if _, statErr := os.Stat(SnapshotFilePath(state, "test-machine")); !os.IsNotExist(statErr) {
		t.Fatalf("snapshot file must not be written for a missing root (stat err %v)", statErr)
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
	if !gitexec.IsContextErr(context.Background(), err) {
		t.Fatal("IsContextErr should detect wrapped deadline")
	}
}

func TestSetGitCancelledWaitDelayNotInvalidRepo(t *testing.T) {
	st := RepoSnapshot{}
	if !setGitCancelled(context.Background(), &st, exec.ErrWaitDelay) {
		t.Fatal("expected WaitDelay to be classified as timeout")
	}
	if strings.Contains(st.Error, "Not a valid git repository") {
		t.Fatalf("WaitDelay must not be reported as invalid repo: %q", st.Error)
	}
	if !strings.Contains(st.Error, "timed out") {
		t.Fatalf("expected timeout wording, got %q", st.Error)
	}
}

func TestCheckRepoStatusesStopsSchedulingOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repos := make([]string, 64)
	for i := range repos {
		repos[i] = t.TempDir()
	}
	started := time.Now()
	results := checkRepoStatuses(ctx, repos, false, 8)
	elapsed := time.Since(started)
	if elapsed > 2*time.Second {
		t.Fatalf("cancelled pool took too long: %s", elapsed)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results when cancelled before scheduling, got %d", len(results))
	}
}

func TestDefaultDurationStringsMatchTypedDefaults(t *testing.T) {
	// Pin product defaults here so renaming a string const can't silently change behavior.
	cases := []struct {
		name string
		str  string
		got  time.Duration
		want time.Duration
	}{
		{"interval", DefaultIntervalString, DefaultAgentInterval, 2 * time.Minute},
		{"heartbeat", DefaultHeartbeatString, DefaultHeartbeat, 15 * time.Minute},
		{"stale-ttl", DefaultStaleTTLString, DefaultStaleTTL, 30 * time.Minute},
		{"tick-timeout", DefaultTickTimeoutString, DefaultAgentTickTimeout, 2 * time.Minute},
	}
	for _, tc := range cases {
		parsed, err := time.ParseDuration(tc.str)
		if err != nil {
			t.Errorf("%s string %q: %v", tc.name, tc.str, err)
			continue
		}
		if parsed != tc.want {
			t.Errorf("%s string = %q (→ %s), want %s", tc.name, tc.str, parsed, tc.want)
		}
		if tc.got != tc.want {
			t.Errorf("%s duration = %s, want %s", tc.name, tc.got, tc.want)
		}
	}
}
