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

	execStart := quoteSystemd(exePath) + " --agent"

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
	fmt.Println("Agent settings (scan_root, interval, state_repo, …) come from sticky config.")
	fmt.Println("Ensure lingering is enabled if the agent should run without an active login: loginctl enable-linger $USER")
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

func quoteSystemd(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"'\\") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}
