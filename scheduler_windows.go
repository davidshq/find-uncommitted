//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const schedulerTaskName = "FindUncommittedAgent"

// legacyAgentLauncherPath is the old .cmd wrapper path; removed on install/uninstall
// after switching the task to invoke the exe directly.
func legacyAgentLauncherPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "find-uncommitted", "agent-launcher.cmd"), nil
}

func removeLegacyAgentLauncher() {
	if p, err := legacyAgentLauncherPath(); err == nil {
		_ = os.Remove(p)
	}
}

// installScheduler registers an at-logon task that runs the exe with --agent.
// The agent detaches its console when it owns it (see detachAgentConsoleIfOwned),
// so no visible cmd window stays open. Scan settings come from sticky config.
func installScheduler(exePath string) error {
	removeLegacyAgentLauncher()

	tr := quoteCmdArg(exePath) + " --agent"
	cmd := exec.Command("schtasks", "/Create", "/TN", schedulerTaskName, "/SC", "ONLOGON", "/RL", "LIMITED", "/F", "/TR", tr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install Windows task: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Printf("Installed Windows scheduled task %q (starts agent at logon, no console window).\n", schedulerTaskName)
	fmt.Printf("Task runs: %s\n", tr)
	printAgentStickyConfigHint()
	return nil
}

func uninstallScheduler() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", schedulerTaskName, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("uninstall Windows task: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	removeLegacyAgentLauncher()
	fmt.Printf("Removed Windows scheduled task %q.\n", schedulerTaskName)
	return nil
}

func quoteCmdArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"&<>|()^%") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
