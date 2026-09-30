package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateStableMachineID(t *testing.T) {
	a := GenerateStableMachineID("laptop")
	b := GenerateStableMachineID("laptop")
	if a == b {
		t.Fatal("expected distinct random suffixes")
	}
	if !strings.HasPrefix(a, "laptop-") {
		t.Fatalf("expected laptop- prefix, got %q", a)
	}
	if GenerateStableMachineID("") == "" {
		t.Fatal("expected non-empty id for empty hostname")
	}
}

func TestEnsureMachineIDInConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := SaveUserConfig(path, UserConfig{StateRepo: "/state"}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureMachineIDInConfig(path, "box-abc1"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MachineID != "box-abc1" {
		t.Fatalf("machine_id = %q", cfg.MachineID)
	}
	if err := EnsureMachineIDInConfig(path, "other"); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MachineID != "box-abc1" {
		t.Fatalf("expected existing machine_id preserved, got %q", cfg.MachineID)
	}
}

// C-1: an agent configured only via env (no config file) used to skip
// persistence, so every start minted a new random id (ghost machines, broken
// agent lock). The id must be written to a new config file and resolve back.
func TestEnsureMachineIDInConfigCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "find-uncommitted", "config.toml")
	if err := EnsureMachineIDInConfig(path, "box-1234abcd"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MachineID != "box-1234abcd" {
		t.Fatalf("machine_id = %q", cfg.MachineID)
	}
	// Next start: env supplies state repo, file supplies the id — no regeneration.
	resolved := ResolveSettings(FlagOverrides{}, cfg, func(k string) string {
		if k == envStateRepo {
			return "/state"
		}
		return ""
	})
	if shouldPersistStableMachineID(resolved, cfg, map[string]bool{}, false, true) {
		t.Fatal("agent must reuse the persisted id instead of generating a new one")
	}
	if resolved.MachineID != "box-1234abcd" || resolved.StateRepo != "/state" {
		t.Fatalf("resolved = %+v", resolved)
	}
}
