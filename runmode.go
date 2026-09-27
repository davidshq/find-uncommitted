package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Soft command verbs (positional after flags), matching check/doctor.
const (
	cmdCheck              = "check"
	cmdDoctor             = "doctor"
	cmdAgent              = "agent"
	cmdInstallScheduler   = "install-scheduler"
	cmdUninstallScheduler = "uninstall-scheduler"
)

// flagsWhoseNextArgIsValue are string/int flags where the following argv token
// is a value, not a soft command (e.g. --machine-id agent).
var flagsWhoseNextArgIsValue = map[string]bool{
	"--state-repo": true, "-state-repo": true,
	"--interval": true, "-interval": true,
	"--heartbeat": true, "-heartbeat": true,
	"--stale-ttl": true, "-stale-ttl": true,
	"--tick-timeout": true, "-tick-timeout": true,
	"--machine-id": true, "-machine-id": true,
	"--output": true, "-output": true,
	"--max-workers": true, "-max-workers": true,
}

// argsHasAgentMode reports whether argv requests background agent mode
// (soft command agent). Used to detach the Windows console before flag
// parsing so Task Scheduler launches do not flash a terminal window.
func argsHasAgentMode(args []string) bool {
	for i, a := range args {
		if a != cmdAgent {
			continue
		}
		if i > 0 && flagsWhoseNextArgIsValue[args[i-1]] {
			continue
		}
		return true
	}
	return false
}

// softCommandMode is a positional verb after flags (or empty for a normal scan).
type softCommandMode string

const (
	softNone               softCommandMode = ""
	softCheck              softCommandMode = cmdCheck
	softDoctor             softCommandMode = cmdDoctor
	softAgent              softCommandMode = cmdAgent
	softInstallScheduler   softCommandMode = cmdInstallScheduler
	softUninstallScheduler softCommandMode = cmdUninstallScheduler
)

// parseSoftCommand detects check/doctor/agent/install-scheduler/uninstall-scheduler
// as the first positional arg (same pattern as check). Remaining positionals follow.
func parseSoftCommand(args []string) (mode softCommandMode, rest []string, err error) {
	if len(args) == 0 {
		return softNone, nil, nil
	}
	switch args[0] {
	case cmdCheck:
		return softCheck, args, nil // check keeps its own arg parser including the verb
	case cmdDoctor:
		if len(args) > 1 {
			return softDoctor, nil, fmt.Errorf("doctor takes no arguments")
		}
		return softDoctor, nil, nil
	case cmdAgent:
		rest = args[1:]
		if len(rest) > 1 {
			return softAgent, nil, fmt.Errorf("agent accepts at most one scan-root argument")
		}
		return softAgent, rest, nil
	case cmdInstallScheduler:
		rest = args[1:]
		if len(rest) > 1 {
			return softInstallScheduler, nil, fmt.Errorf("install-scheduler accepts at most one scan-root argument")
		}
		return softInstallScheduler, rest, nil
	case cmdUninstallScheduler:
		if len(args) > 1 {
			return softUninstallScheduler, nil, fmt.Errorf("uninstall-scheduler takes no arguments")
		}
		return softUninstallScheduler, nil, nil
	default:
		return softNone, args, nil
	}
}

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
