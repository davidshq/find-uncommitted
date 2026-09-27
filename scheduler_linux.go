//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	systemdUnitName = "find-uncommitted-agent"
	systemdService  = systemdUnitName + ".service"
)

// installScheduler writes a systemd user service that keeps the agent running.
// Scan root, state_repo, interval, and related settings come from sticky config.
func installScheduler(exePath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("create systemd user unit dir: %w", err)
	}

	execStart := quoteSystemd(exePath) + " agent"

	content := fmt.Sprintf(`[Unit]
Description=Find Uncommitted cross-machine state agent
After=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
`, execStart)

	unitPath := filepath.Join(unitDir, systemdService)
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", systemdService).CombinedOutput(); err != nil {
		return fmt.Errorf("enable service: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	fmt.Printf("Installed and started systemd user service %q.\n", systemdService)
	printAgentStickyConfigHint()
	fmt.Println("Default: agent runs while you have a login session.")
	fmt.Println("For always-on / headless: loginctl enable-linger $USER")
	fmt.Println("(Linger is per-user: all enabled systemd user services for this account can stay up, not only this agent.)")
	return nil
}

func uninstallScheduler() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", systemdService).Run()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	unitPath := filepath.Join(home, ".config", "systemd", "user", systemdService)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	fmt.Printf("Removed systemd user service %q.\n", systemdService)
	return nil
}

func schedulerStatus() (SchedulerStatus, error) {
	status := SchedulerStatus{
		Supported:        true,
		Name:             systemdService,
		NotRunningIsFail: true,
	}
	enabledOut, enabledErr := exec.Command("systemctl", "--user", "is-enabled", systemdService).CombinedOutput()
	activeOut, activeErr := exec.Command("systemctl", "--user", "is-active", systemdService).CombinedOutput()
	enabledText := strings.TrimSpace(string(enabledOut))
	activeText := strings.TrimSpace(string(activeOut))
	if systemdUserBusUnavailable(enabledText, activeText) {
		unitPath := linuxSystemdUnitPath()
		status.Installed = unitPath != "" && ConfigFileExists(unitPath)
		status.Running = false
		status.NotRunningIsFail = false // cannot probe without a user bus
		if status.Installed {
			status.Detail = "unit present; systemd user bus unavailable"
		} else {
			status.Detail = "systemd user bus unavailable"
		}
		return status, nil
	}
	enabled := enabledErr == nil && enabledText == "enabled"
	active := activeErr == nil && activeText == "active"
	unitPath := linuxSystemdUnitPath()
	unitExists := unitPath != "" && ConfigFileExists(unitPath)
	status.Installed = enabled || active || unitExists
	status.Running = active
	switch {
	case !status.Installed:
		status.Detail = "unit not found"
	case active:
		status.Detail = fmt.Sprintf("enabled=%v active=active", enabled)
	default:
		activeState := activeText
		if activeState == "" {
			activeState = "unknown"
		}
		status.Detail = fmt.Sprintf("enabled=%v active=%s", enabled, activeState)
	}
	return status, nil
}

func linuxSystemdUnitPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdService)
}

func systemdUserBusUnavailable(texts ...string) bool {
	for _, t := range texts {
		if strings.Contains(t, "Failed to connect to bus") {
			return true
		}
	}
	return false
}

func quoteSystemd(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"'\\") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
