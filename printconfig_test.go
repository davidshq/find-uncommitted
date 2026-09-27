package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintResolvedConfigSources(t *testing.T) {
	var buf bytes.Buffer
	r := ResolvedSettings{
		StateRepo:         "/state",
		StateRepoSource:   SourceConfig,
		ScanRoot:          "/scan",
		ScanRootSource:    SourceEnv,
		MachineID:         "desk",
		MachineIDSource:   SourceFlag,
		Interval:          "2m",
		IntervalSource:    SourceNone,
		Heartbeat:         "15m",
		HeartbeatSource:   SourceConfig,
		StaleTTL:          "30m",
		StaleTTLSource:    SourceNone,
		RedactPaths:       true,
		RedactPathsSource: SourceFlag,
		MaxWorkers:        4,
		MaxWorkersSource:  SourceEnv,
	}
	printResolvedConfig(&buf, "/tmp/config.toml", true, r, "desk", "2m", "15m", "30m", 4, "2m", false)
	out := buf.String()
	want := []string{
		"config_path: /tmp/config.toml (exists)",
		"state_repo: /state (config)",
		"scan_root: /scan (env)",
		"machine_id: desk (flag)",
		"interval: 2m (default)",
		"heartbeat: 15m (config)",
		"stale_ttl: 30m (default)",
		"tick_timeout: 2m (default)",
		"redact_paths: true (flag)",
		"max_workers: 4 (env)",
	}
	for _, line := range want {
		if !strings.Contains(out, line) {
			t.Fatalf("missing %q in:\n%s", line, out)
		}
	}
}

func TestPrintResolvedConfigMissingFileAndDefaults(t *testing.T) {
	var buf bytes.Buffer
	r := ResolvedSettings{}
	printResolvedConfig(&buf, "/missing.toml", false, r, "host1", DefaultIntervalString, DefaultHeartbeatString, DefaultStaleTTLString, 0, DefaultTickTimeoutString, false)
	out := buf.String()
	if !strings.Contains(out, "config_path: /missing.toml (missing)") {
		t.Fatalf("expected missing config path, got:\n%s", out)
	}
	if !strings.Contains(out, "state_repo: (unset) (default)") {
		t.Fatalf("expected unset state_repo, got:\n%s", out)
	}
	if !strings.Contains(out, "machine_id: host1 (hostname)") {
		t.Fatalf("expected hostname machine_id, got:\n%s", out)
	}
	if !strings.Contains(out, "max_workers: 8 (default)") {
		t.Fatalf("expected default max_workers, got:\n%s", out)
	}
}
