package main

import (
	"fmt"
	"io"
	"os"
)

// printResolvedConfig writes resolved settings with value sources to w.
// Effective built-in defaults are labeled "default" when no flag/env/config supplied them.
func printResolvedConfig(w io.Writer, configPath string, configExists bool, r ResolvedSettings, machineID, interval, heartbeat, staleTTL string, maxWorkers int, tickTimeout string, tickTimeoutFromFlag bool) {
	fmt.Fprintf(w, "config_path: %s", configPath)
	switch {
	case configPath == "":
		fmt.Fprint(w, " (unavailable)")
	case configExists:
		fmt.Fprint(w, " (exists)")
	default:
		fmt.Fprint(w, " (missing)")
	}
	fmt.Fprintln(w)

	printConfigString(w, "state_repo", r.StateRepo, r.StateRepoSource, "")
	printConfigString(w, "scan_root", r.ScanRoot, r.ScanRootSource, "")
	printConfigString(w, "machine_id", machineID, r.MachineIDSource, "hostname")
	printConfigString(w, "interval", interval, r.IntervalSource, "default")
	printConfigString(w, "heartbeat", heartbeat, r.HeartbeatSource, "default")
	printConfigString(w, "stale_ttl", staleTTL, r.StaleTTLSource, "default")
	tickSrc := SourceNone
	if tickTimeoutFromFlag {
		tickSrc = SourceFlag
	}
	printConfigString(w, "tick_timeout", tickTimeout, tickSrc, "default")
	printConfigBool(w, "redact_paths", r.RedactPaths, r.RedactPathsSource)
	printConfigInt(w, "max_workers", resolvedMaxWorkers(maxWorkers), r.MaxWorkersSource, DefaultMaxWorkers)
}

func printConfigString(w io.Writer, key, value string, source ConfigSource, defaultLabel string) {
	src := formatConfigSource(source, defaultLabel)
	if value == "" && source == SourceNone {
		fmt.Fprintf(w, "%s: (unset) (%s)\n", key, src)
		return
	}
	fmt.Fprintf(w, "%s: %s (%s)\n", key, value, src)
}

func printConfigBool(w io.Writer, key string, value bool, source ConfigSource) {
	src := formatConfigSource(source, "default")
	fmt.Fprintf(w, "%s: %v (%s)\n", key, value, src)
}

func printConfigInt(w io.Writer, key string, value int, source ConfigSource, builtinDefault int) {
	src := formatConfigSource(source, "default")
	if source == SourceNone {
		fmt.Fprintf(w, "%s: %d (%s)\n", key, builtinDefault, src)
		return
	}
	fmt.Fprintf(w, "%s: %d (%s)\n", key, value, src)
}

func formatConfigSource(source ConfigSource, defaultLabel string) string {
	switch source {
	case SourceFlag:
		return "flag"
	case SourceEnv:
		return "env"
	case SourceConfig:
		return "config"
	default:
		if defaultLabel != "" {
			return defaultLabel
		}
		return "default"
	}
}

// printConfigToStdout is the CLI entry for --print-config.
func printConfigToStdout(configPath string, configExists bool, r ResolvedSettings, machineID, interval, heartbeat, staleTTL string, maxWorkers int, tickTimeout string, tickTimeoutFromFlag bool) {
	printResolvedConfig(os.Stdout, configPath, configExists, r, machineID, interval, heartbeat, staleTTL, maxWorkers, tickTimeout, tickTimeoutFromFlag)
}
