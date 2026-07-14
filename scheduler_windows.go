//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
func installScheduler(exePath string, scanRoot, stateRepo, machineID string, interval time.Duration, redactPaths bool) error {
	launcher, err := agentLauncherPath()
	if err != nil {
		return fmt.Errorf("resolve launcher path: %w", err)
	}

	args := []string{
		quoteCmdArg(exePath),
		"--agent",
		"--state-repo", quoteCmdArg(stateRepo),
		"--machine-id", quoteCmdArg(machineID),
		"--interval", quoteCmdArg(interval.String()),
	}
	if redactPaths {
		args = append(args, "--redact-paths")
	}
	args = append(args, quoteCmdArg(scanRoot))

	content := "@echo off\r\n" + strings.Join(args, " ") + "\r\n"
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
	fmt.Printf("Agent interval once running: %s\n", interval)
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

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}
