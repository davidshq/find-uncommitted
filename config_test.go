package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	if filepath.Base(path) != configFileName {
		t.Fatalf("expected basename %q, got %q", configFileName, filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != configAppDirName {
		t.Fatalf("expected parent dir %q, got %q", configAppDirName, filepath.Base(filepath.Dir(path)))
	}
}

func TestLoadSaveUserConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if cfg.StateRepo != "" {
		t.Fatalf("expected empty config for missing file")
	}

	want := UserConfig{
		StateRepo:   "/tmp/state",
		ScanRoot:    "/tmp/repos",
		MachineID:   "test-machine",
		Interval:    "2m",
		Heartbeat:   "15m",
		StaleTTL:    "30m",
		RedactPaths: true,
	}
	if err := SaveUserConfig(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if !ConfigFileExists(path) {
		t.Fatal("expected config file to exist")
	}
}

func TestLoadUserConfigCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("state_repo = [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil {
		t.Fatal("expected parse error for corrupt TOML")
	}
}

func TestResolveSettingsPrecedence(t *testing.T) {
	file := UserConfig{
		StateRepo: "/from/file",
		ScanRoot:  "/scan/file",
		MachineID: "file-id",
		Interval:  "45s",
		Heartbeat: "20m",
		StaleTTL:  "10m",
	}
	env := map[string]string{
		envStateRepo: "/from/env",
		envMachineID: "env-id",
		envHeartbeat: "25m",
	}
	getenv := func(k string) string { return env[k] }

	// Flag wins over env and file.
	res := ResolveSettings(FlagOverrides{
		StateRepo:    "/from/flag",
		StateRepoSet: true,
		Interval:     "60s",
		IntervalSet:  true,
	}, file, getenv)
	if res.StateRepo != "/from/flag" || res.StateRepoSource != SourceFlag {
		t.Fatalf("state_repo flag: got %q (%s)", res.StateRepo, res.StateRepoSource)
	}
	if res.MachineID != "env-id" || res.MachineIDSource != SourceEnv {
		t.Fatalf("machine_id env: got %q (%s)", res.MachineID, res.MachineIDSource)
	}
	if res.ScanRoot != "/scan/file" || res.ScanRootSource != SourceConfig {
		t.Fatalf("scan_root file: got %q (%s)", res.ScanRoot, res.ScanRootSource)
	}
	if res.Interval != "60s" || res.IntervalSource != SourceFlag {
		t.Fatalf("interval flag: got %q (%s)", res.Interval, res.IntervalSource)
	}
	if res.Heartbeat != "25m" || res.HeartbeatSource != SourceEnv {
		t.Fatalf("heartbeat env: got %q (%s)", res.Heartbeat, res.HeartbeatSource)
	}
	if res.StaleTTL != "10m" || res.StaleTTLSource != SourceConfig {
		t.Fatalf("stale_ttl file: got %q (%s)", res.StaleTTL, res.StaleTTLSource)
	}

	// Heartbeat flag wins over env and file.
	res = ResolveSettings(FlagOverrides{
		Heartbeat:    "10m",
		HeartbeatSet: true,
	}, file, getenv)
	if res.Heartbeat != "10m" || res.HeartbeatSource != SourceFlag {
		t.Fatalf("heartbeat flag: got %q (%s)", res.Heartbeat, res.HeartbeatSource)
	}

	// Env wins over file when flag unset.
	res = ResolveSettings(FlagOverrides{}, file, getenv)
	if res.StateRepo != "/from/env" || res.StateRepoSource != SourceEnv {
		t.Fatalf("state_repo env: got %q (%s)", res.StateRepo, res.StateRepoSource)
	}

	// File only (including heartbeat).
	res = ResolveSettings(FlagOverrides{}, file, func(string) string { return "" })
	if res.StateRepo != "/from/file" || res.StateRepoSource != SourceConfig {
		t.Fatalf("state_repo file: got %q (%s)", res.StateRepo, res.StateRepoSource)
	}
	if res.Heartbeat != "20m" || res.HeartbeatSource != SourceConfig {
		t.Fatalf("heartbeat file: got %q (%s)", res.Heartbeat, res.HeartbeatSource)
	}

	// Heartbeat unset when neither env nor config provides it.
	res = ResolveSettings(FlagOverrides{}, UserConfig{}, func(string) string { return "" })
	if res.Heartbeat != "" || res.HeartbeatSource != SourceNone {
		t.Fatalf("heartbeat unset: got %q (%s)", res.Heartbeat, res.HeartbeatSource)
	}
}

func TestResolveRedactPaths(t *testing.T) {
	file := UserConfig{RedactPaths: true}
	res := ResolveSettings(FlagOverrides{}, file, func(string) string { return "" })
	if !res.RedactPaths || res.RedactPathsSource != SourceConfig {
		t.Fatalf("expected redact from config")
	}

	res = ResolveSettings(FlagOverrides{
		RedactPaths:    false,
		RedactPathsSet: true,
	}, file, func(string) string { return "" })
	if res.RedactPaths || res.RedactPathsSource != SourceFlag {
		t.Fatalf("expected flag to clear redact")
	}

	res = ResolveSettings(FlagOverrides{}, UserConfig{}, func(k string) string {
		if k == envRedactPaths {
			return "true"
		}
		return ""
	})
	if !res.RedactPaths || res.RedactPathsSource != SourceEnv {
		t.Fatalf("expected env redact")
	}
}

func TestEnsureConfigFromAgent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := EnsureConfigFromAgent(path, "/state", "/scan", "m1", "2m", "15m", "30m", false); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.StateRepo != "/state" || cfg.ScanRoot != "/scan" {
		t.Fatalf("unexpected cfg %+v", cfg)
	}
	if cfg.Interval != "2m" || cfg.Heartbeat != "15m" || cfg.StaleTTL != "30m" {
		t.Fatalf("expected cadence fields, got %+v", cfg)
	}

	// Second call must not overwrite.
	if err := os.WriteFile(path, []byte("state_repo = \"/kept\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureConfigFromAgent(path, "/other", "/scan2", "m2", "2m", "15m", "30m", true); err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	cfg, err = LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load again: %v", err)
	}
	if cfg.StateRepo != "/kept" {
		t.Fatalf("expected existing config preserved, got %q", cfg.StateRepo)
	}
}

func TestStaleTTLTooShort(t *testing.T) {
	if !staleTTLTooShort(15*time.Minute, 20*time.Minute) {
		t.Fatal("expected 20m stale with 15m heartbeat to be too short")
	}
	if staleTTLTooShort(15*time.Minute, 30*time.Minute) {
		t.Fatal("expected 30m stale with 15m heartbeat to be OK")
	}
	if !staleTTLTooShort(15*time.Minute, 29*time.Minute) {
		t.Fatal("expected 29m stale with 15m heartbeat to be too short")
	}
	if staleTTLTooShort(0, 5*time.Minute) || staleTTLTooShort(15*time.Minute, 0) {
		t.Fatal("zero durations should not report too short")
	}
}
