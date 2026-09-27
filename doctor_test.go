package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectDoctorReportLocalOnly(t *testing.T) {
	// Decouple from the live OS scheduler so an installed-but-stopped unit
	// on a developer machine cannot FAIL this local-only assertion.
	prev := probeSchedulerStatus
	probeSchedulerStatus = func() (SchedulerStatus, error) {
		return SchedulerStatus{Supported: true, Name: "test-scheduler", Detail: "not installed"}, nil
	}
	t.Cleanup(func() { probeSchedulerStatus = prev })

	in := DoctorInput{
		ConfigPath:   "/tmp/no-such-find-uncommitted-config.toml",
		ConfigExists: false,
		Resolved:     ResolvedSettings{},
		MachineID:    "test-machine",
		Interval:     DefaultIntervalString,
		Heartbeat:    DefaultHeartbeatString,
		StaleTTL:     DefaultStaleTTLString,
		StaleTTLDur:  DefaultStaleTTL,
		TickTimeout:  DefaultTickTimeoutString,
		Now:          time.Now().UTC(),
	}
	report := collectDoctorReport(context.Background(), in)
	joined := strings.Join(report.lines, "\n")
	if report.failed {
		t.Fatalf("local-only doctor should not FAIL:\n%s", joined)
	}
	if !strings.Contains(joined, "config path missing") {
		t.Fatalf("expected missing config warn:\n%s", joined)
	}
	if !strings.Contains(joined, "state_repo unset") {
		t.Fatalf("expected state_repo unset warn:\n%s", joined)
	}
}

func TestCollectDoctorReportWithStateRepo(t *testing.T) {
	state := initTempGitRepo(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	machineID := "doctor-test"
	snapPath := SnapshotFilePath(state, machineID)
	if err := WriteMachineSnapshot(snapPath, MachineSnapshot{
		MachineID: machineID,
		UpdatedAt: now.Add(-5 * time.Minute),
		Repos:     nil,
	}); err != nil {
		t.Fatal(err)
	}

	in := DoctorInput{
		ConfigPath:   filepath.Join(t.TempDir(), "config.toml"),
		ConfigExists: false,
		Resolved: ResolvedSettings{
			StateRepo:       state,
			StateRepoSource: SourceConfig,
			ScanRoot:        "/repos",
			ScanRootSource:  SourceConfig,
			MachineID:       machineID,
			MachineIDSource: SourceConfig,
		},
		MachineID:   machineID,
		StateRepo:   state,
		ScanRoot:    "/repos",
		Interval:    DefaultIntervalString,
		Heartbeat:   DefaultHeartbeatString,
		StaleTTL:    DefaultStaleTTLString,
		StaleTTLDur: DefaultStaleTTL,
		TickTimeout: DefaultTickTimeoutString,
		Now:         now,
	}
	report := collectDoctorReport(context.Background(), in)
	joined := strings.Join(report.lines, "\n")
	if !strings.Contains(joined, "OK   state_repo:") {
		t.Fatalf("expected state_repo OK:\n%s", joined)
	}
	if !strings.Contains(joined, "OK   last publish:") {
		t.Fatalf("expected last publish OK:\n%s", joined)
	}
	if !strings.Contains(joined, "OK   state-repo sync lock: free") {
		t.Fatalf("expected sync lock free:\n%s", joined)
	}
}

func TestCollectDoctorReportInvalidStateRepo(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "not-a-repo")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	in := DoctorInput{
		MachineID:   "m",
		StateRepo:   bad,
		Interval:    DefaultIntervalString,
		Heartbeat:   DefaultHeartbeatString,
		StaleTTL:    DefaultStaleTTLString,
		StaleTTLDur: DefaultStaleTTL,
		TickTimeout: DefaultTickTimeoutString,
		Now:         time.Now().UTC(),
	}
	report := collectDoctorReport(context.Background(), in)
	if !report.failed {
		t.Fatalf("expected FAIL for invalid state repo:\n%s", strings.Join(report.lines, "\n"))
	}
}

func TestCollectDoctorReportStalePublish(t *testing.T) {
	state := initTempGitRepo(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	machineID := "stale-machine"
	if err := WriteMachineSnapshot(SnapshotFilePath(state, machineID), MachineSnapshot{
		MachineID: machineID,
		UpdatedAt: now.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	in := DoctorInput{
		MachineID:   machineID,
		StateRepo:   state,
		ScanRoot:    "/repos",
		Interval:    DefaultIntervalString,
		Heartbeat:   DefaultHeartbeatString,
		StaleTTL:    DefaultStaleTTLString,
		StaleTTLDur: DefaultStaleTTL,
		TickTimeout: DefaultTickTimeoutString,
		Now:         now,
	}
	report := collectDoctorReport(context.Background(), in)
	joined := strings.Join(report.lines, "\n")
	if !strings.Contains(joined, "WARN last publish:") || !strings.Contains(joined, "stale") {
		t.Fatalf("expected stale publish warn:\n%s", joined)
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-m", "init")
	return dir
}
