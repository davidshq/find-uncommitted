package main

import (
	"testing"
	"time"
)

func TestShouldPersistStableMachineID(t *testing.T) {
	flagSet := map[string]bool{}
	file := UserConfig{}
	resolved := ResolvedSettings{MachineIDSource: SourceNone}

	if !shouldPersistStableMachineID(resolved, file, flagSet, true, false) {
		t.Fatal("expected stable id for install")
	}
	if !shouldPersistStableMachineID(resolved, file, flagSet, false, true) {
		t.Fatal("expected stable id for agent")
	}
	if shouldPersistStableMachineID(resolved, file, flagSet, false, false) {
		t.Fatal("bare scan should not generate stable id")
	}

	file.MachineID = "kept"
	if shouldPersistStableMachineID(resolved, file, flagSet, true, false) {
		t.Fatal("config machine_id should skip generation")
	}

	flagSet["machine-id"] = true
	if shouldPersistStableMachineID(ResolvedSettings{MachineIDSource: SourceFlag}, file, flagSet, true, false) {
		t.Fatal("explicit flag should not auto-generate")
	}
}

func TestNewAgentConfig(t *testing.T) {
	cfg := newAgentConfig("/scan", "/state", "box", 2*time.Minute, 3*time.Minute, true, 15*time.Minute, false)
	if cfg.ScanRoot != "/scan" || cfg.StateRepoDir != "/state" || cfg.MachineID != "box" {
		t.Fatalf("unexpected top-level fields: %+v", cfg)
	}
	if cfg.Interval != 2*time.Minute || cfg.TickTimeout != 3*time.Minute || !cfg.RedactPaths || cfg.DirtyOnly {
		t.Fatalf("unexpected cadence/redact/dirty: %+v", cfg)
	}
	if cfg.Sync.StateRepoDir != "/state" || cfg.Sync.MachineID != "box" || cfg.Sync.Heartbeat != 15*time.Minute {
		t.Fatalf("unexpected sync fields: %+v", cfg.Sync)
	}
}

func TestStickyConfigFromRun(t *testing.T) {
	cfg := stickyConfigFromRun("/state", "/scan", "m1", "2m", "15m", "30m", true)
	if cfg.StateRepo != "/state" || cfg.ScanRoot != "/scan" || cfg.MachineID != "m1" {
		t.Fatalf("unexpected paths/id: %+v", cfg)
	}
	if cfg.Interval != "2m" || cfg.Heartbeat != "15m" || cfg.StaleTTL != "30m" || !cfg.RedactPaths {
		t.Fatalf("unexpected cadence/redact: %+v", cfg)
	}
}
