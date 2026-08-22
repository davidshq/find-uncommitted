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
