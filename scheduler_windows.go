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

func agentLauncherPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "find-uncommitted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "agent-launcher.cmd"), nil
}

// installScheduler writes a .cmd launcher (avoids schtasks arg quoting issues)
// and registers an at-logon task that runs it.
// Scan root, state_repo, interval, and related settings come from sticky config.
func installScheduler(exePath string) error {
	launcher, err := agentLauncherPath()
	if err != nil {
		return fmt.Errorf("resolve launcher path: %w", err)
	}

	content := "@echo off\r\n" + quoteCmdArg(exePath) + " --agent\r\n"
	if err := os.WriteFile(launcher, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write launcher: %w", err)
	}

	cmd := exec.Command("schtasks", "/Create", "/TN", schedulerTaskName, "/SC", "ONLOGON", "/RL", "LIMITED", "/F", "/TR", quoteCmdArg(launcher))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install Windows task: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Printf("Installed Windows scheduled task %q (starts agent at logon).\n", schedulerTaskName)
	fmt.Printf("Launcher: %s\n", launcher)
	printAgentStickyConfigHint()
	return nil
}

func uninstallScheduler() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", schedulerTaskName, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("uninstall Windows task: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	if launcher, err := agentLauncherPath(); err == nil {
		_ = os.Remove(launcher)
	}
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
