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

func TestArgsHasAgentMode(t *testing.T) {
	if argsHasAgentMode([]string{"--agent"}) {
		t.Fatal("--agent is not a mode flag anymore")
	}
	if argsHasAgentMode([]string{"--state-repo", "x", "--agent", "C:\\code"}) {
		t.Fatal("--agent among flags is not agent mode")
	}
	if argsHasAgentMode([]string{"install-scheduler", "C:\\code"}) {
		t.Fatal("install-scheduler is not agent mode")
	}
	if !argsHasAgentMode([]string{"agent"}) {
		t.Fatal("expected soft command agent")
	}
	if !argsHasAgentMode([]string{"--state-repo", "/state", "agent", "/scan"}) {
		t.Fatal("expected soft command agent after flags")
	}
	if argsHasAgentMode([]string{"--machine-id", "agent"}) {
		t.Fatal("machine-id value agent must not trigger agent mode")
	}
	if argsHasAgentMode([]string{"check", "."}) {
		t.Fatal("check is not agent mode")
	}
}

func TestParseSoftCommand(t *testing.T) {
	mode, rest, err := parseSoftCommand(nil)
	if err != nil || mode != softNone || rest != nil {
		t.Fatalf("empty: mode=%q rest=%v err=%v", mode, rest, err)
	}

	mode, rest, err = parseSoftCommand([]string{"/scan"})
	if err != nil || mode != softNone || len(rest) != 1 || rest[0] != "/scan" {
		t.Fatalf("scan root: mode=%q rest=%v err=%v", mode, rest, err)
	}

	mode, rest, err = parseSoftCommand([]string{"agent", "/scan"})
	if err != nil || mode != softAgent || len(rest) != 1 || rest[0] != "/scan" {
		t.Fatalf("agent: mode=%q rest=%v err=%v", mode, rest, err)
	}

	mode, rest, err = parseSoftCommand([]string{"install-scheduler", "/scan"})
	if err != nil || mode != softInstallScheduler || len(rest) != 1 || rest[0] != "/scan" {
		t.Fatalf("install: mode=%q rest=%v err=%v", mode, rest, err)
	}

	mode, rest, err = parseSoftCommand([]string{"uninstall-scheduler"})
	if err != nil || mode != softUninstallScheduler || rest != nil {
		t.Fatalf("uninstall: mode=%q rest=%v err=%v", mode, rest, err)
	}

	if _, _, err := parseSoftCommand([]string{"agent", "/a", "/b"}); err == nil {
		t.Fatal("expected error for extra agent args")
	}
	if _, _, err := parseSoftCommand([]string{"doctor", "x"}); err == nil {
		t.Fatal("expected error for doctor args")
	}
	if _, _, err := parseSoftCommand([]string{"uninstall-scheduler", "/x"}); err == nil {
		t.Fatal("expected error for uninstall args")
	}

	mode, rest, err = parseSoftCommand([]string{"check", "--json", "."})
	if err != nil || mode != softCheck || len(rest) != 3 || rest[0] != "check" {
		t.Fatalf("check passthrough: mode=%q rest=%v err=%v", mode, rest, err)
	}
}

func TestNewAgentConfig(t *testing.T) {
	cfg := newAgentConfig("/scan", "/state", "box", 2*time.Minute, 3*time.Minute, 8, true, 15*time.Minute, false)
	if cfg.ScanRoot != "/scan" || cfg.StateRepoDir != "/state" || cfg.MachineID != "box" {
		t.Fatalf("unexpected top-level fields: %+v", cfg)
	}
	if cfg.Interval != 2*time.Minute || cfg.TickTimeout != 3*time.Minute || cfg.MaxWorkers != 8 || !cfg.RedactPaths || cfg.DirtyOnly {
		t.Fatalf("unexpected cadence/redact/dirty: %+v", cfg)
	}
	if cfg.Sync.StateRepoDir != "/state" || cfg.Sync.MachineID != "box" || cfg.Sync.Heartbeat != 15*time.Minute {
		t.Fatalf("unexpected sync fields: %+v", cfg.Sync)
	}
}

func TestStickyConfigFromRun(t *testing.T) {
	cfg := stickyConfigFromRun("/state", "/scan", "m1", "2m", "15m", "30m", "15m", true, 8)
	if cfg.StateRepo != "/state" || cfg.ScanRoot != "/scan" || cfg.MachineID != "m1" {
		t.Fatalf("unexpected paths/id: %+v", cfg)
	}
	if cfg.Interval != "2m" || cfg.Heartbeat != "15m" || cfg.StaleTTL != "30m" || !cfg.RedactPaths || cfg.MaxWorkers != 8 {
		t.Fatalf("unexpected cadence/redact/workers: %+v", cfg)
	}
	// C-3: the installed agent runs bare `agent`, so --tick-timeout only
	// reaches it through sticky config.
	if cfg.TickTimeout != "15m" {
		t.Fatalf("tick_timeout not persisted: %+v", cfg)
	}
}
