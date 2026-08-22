package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// requireStateRepo exits when no state repo is configured for agent/scheduler modes.
func requireStateRepo(stateRepo, modeFlag string) {
	if stateRepo != "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s requires --state-repo (or sticky config / FIND_UNCOMMITTED_STATE_REPO)\n", modeFlag)
	os.Exit(1)
}

// validateStateRepoOrExit prints a fatal error when the state clone path is invalid.
func validateStateRepoOrExit(stateRepo string) {
	if err := validateStateRepo(stateRepo); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// shouldPersistStableMachineID reports whether to generate and save a stable machine_id
// (install or agent only — not bare scans).
func shouldPersistStableMachineID(resolved ResolvedSettings, file UserConfig, flagSet map[string]bool, installSched, agentMode bool) bool {
	if resolved.MachineIDSource != SourceNone || flagSet["machine-id"] || strings.TrimSpace(file.MachineID) != "" {
		return false
	}
	return installSched || agentMode
}

func stickyConfigFromRun(stateRepo, scanRoot, machineID, intervalStr, heartbeatStr, staleTTLStr string, redactPaths bool, maxWorkers int) UserConfig {
	return UserConfig{
		StateRepo:   stateRepo,
		ScanRoot:    scanRoot,
		MachineID:   machineID,
		Interval:    intervalStr,
		Heartbeat:   heartbeatStr,
		StaleTTL:    staleTTLStr,
		RedactPaths: redactPaths,
		MaxWorkers:  maxWorkers,
	}
}

func newAgentConfig(scanRoot, stateRepo, machineID string, interval, tickTimeout time.Duration, maxWorkers int, redactPaths bool, heartbeat time.Duration, dirtyOnly bool) AgentConfig {
	return AgentConfig{
		ScanRoot:     scanRoot,
		StateRepoDir: stateRepo,
		MachineID:    machineID,
		Interval:     interval,
		TickTimeout:  tickTimeout,
		MaxWorkers:   maxWorkers,
		RedactPaths:  redactPaths,
		DirtyOnly:    dirtyOnly,
		Sync: SyncConfig{
			StateRepoDir: stateRepo,
			MachineID:    machineID,
			Heartbeat:    heartbeat,
		},
	}
}
