//go:build windows

package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const schedulerTaskName = "FindUncommittedAgent"

// installScheduler registers an at-logon task that runs the exe with --agent.
// The agent detaches its console when it owns it (see detachAgentConsoleIfOwned),
// so no visible cmd window stays open. Scan settings come from sticky config.
//
// Task XML sets MultipleInstancesPolicy=IgnoreNew (so /Run is not stuck Queued
// behind a phantom instance), ExecutionTimeLimit=PT0S (no 72h kill), and
// RestartOnFailure (crash recovery while the logon session is still up).
func installScheduler(exePath string) error {
	userID, err := windowsTaskUserID()
	if err != nil {
		return fmt.Errorf("resolve task user: %w", err)
	}
	xmlPath, err := writeSchedulerTaskXML(exePath, userID)
	if err != nil {
		return err
	}
	defer os.Remove(xmlPath)

	cmd := exec.Command("schtasks", "/Create", "/TN", schedulerTaskName, "/XML", xmlPath, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install Windows task: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Printf("Installed Windows scheduled task %q (starts agent at logon, restart on failure, no console window).\n", schedulerTaskName)
	fmt.Printf("Task runs: %s --agent\n", exePath)
	printAgentStickyConfigHint()
	return nil
}

func uninstallScheduler() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", schedulerTaskName, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("uninstall Windows task: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Printf("Removed Windows scheduled task %q.\n", schedulerTaskName)
	return nil
}

func windowsTaskUserID() (string, error) {
	domain := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	name := strings.TrimSpace(os.Getenv("USERNAME"))
	if domain != "" && name != "" {
		return domain + `\` + name, nil
	}
	cu, err := user.Current()
	if err != nil {
		return "", err
	}
	return cu.Username, nil
}

type taskXML struct {
	XMLName          xml.Name `xml:"Task"`
	Xmlns            string   `xml:"xmlns,attr"`
	Version          string   `xml:"version,attr"`
	RegistrationInfo struct {
		Description string `xml:"Description"`
	} `xml:"RegistrationInfo"`
	Triggers struct {
		LogonTrigger struct {
			Enabled string `xml:"Enabled"`
			UserId  string `xml:"UserId"`
		} `xml:"LogonTrigger"`
	} `xml:"Triggers"`
	Principals struct {
		Principal struct {
			ID        string `xml:"id,attr"`
			UserId    string `xml:"UserId"`
			LogonType string `xml:"LogonType"`
			RunLevel  string `xml:"RunLevel"`
		} `xml:"Principal"`
	} `xml:"Principals"`
	Settings struct {
		MultipleInstancesPolicy    string `xml:"MultipleInstancesPolicy"`
		DisallowStartIfOnBatteries bool   `xml:"DisallowStartIfOnBatteries"`
		StopIfGoingOnBatteries     bool   `xml:"StopIfGoingOnBatteries"`
		AllowHardTerminate         bool   `xml:"AllowHardTerminate"`
		StartWhenAvailable         bool   `xml:"StartWhenAvailable"`
		AllowStartOnDemand         bool   `xml:"AllowStartOnDemand"`
		Enabled                    bool   `xml:"Enabled"`
		Hidden                     bool   `xml:"Hidden"`
		RunOnlyIfIdle              bool   `xml:"RunOnlyIfIdle"`
		WakeToRun                  bool   `xml:"WakeToRun"`
		ExecutionTimeLimit         string `xml:"ExecutionTimeLimit"`
		Priority                   int    `xml:"Priority"`
		IdleSettings               struct {
			StopOnIdleEnd bool `xml:"StopOnIdleEnd"`
			RestartOnIdle bool `xml:"RestartOnIdle"`
		} `xml:"IdleSettings"`
		// RestartOnFailure mirrors systemd Restart=on-failure for the agent process
		// while the logon session is still up (not a logged-out always-on mode).
		RestartOnFailure struct {
			Interval string `xml:"Interval"`
			Count    int    `xml:"Count"`
		} `xml:"RestartOnFailure"`
	} `xml:"Settings"`
	Actions struct {
		Context string `xml:"Context,attr"`
		Exec    struct {
			Command   string `xml:"Command"`
			Arguments string `xml:"Arguments"`
		} `xml:"Exec"`
	} `xml:"Actions"`
}

func writeSchedulerTaskXML(exePath, userID string) (string, error) {
	var t taskXML
	t.Xmlns = "http://schemas.microsoft.com/windows/2004/02/mit/task"
	t.Version = "1.2"
	t.RegistrationInfo.Description = "find-uncommitted background agent (publish machine snapshots)"
	t.Triggers.LogonTrigger.Enabled = "true"
	t.Triggers.LogonTrigger.UserId = userID
	t.Principals.Principal.ID = "Author"
	t.Principals.Principal.UserId = userID
	t.Principals.Principal.LogonType = "InteractiveToken"
	t.Principals.Principal.RunLevel = "LeastPrivilege"
	t.Settings.MultipleInstancesPolicy = "IgnoreNew"
	t.Settings.DisallowStartIfOnBatteries = false
	t.Settings.StopIfGoingOnBatteries = false
	t.Settings.AllowHardTerminate = true
	t.Settings.StartWhenAvailable = true
	t.Settings.AllowStartOnDemand = true
	t.Settings.Enabled = true
	t.Settings.Hidden = true
	t.Settings.RunOnlyIfIdle = false
	t.Settings.WakeToRun = false
	t.Settings.ExecutionTimeLimit = "PT0S"
	t.Settings.Priority = 7
	t.Settings.IdleSettings.StopOnIdleEnd = false
	t.Settings.IdleSettings.RestartOnIdle = false
	// Task Scheduler minimum restart interval is 1 minute; Count max is 999.
	t.Settings.RestartOnFailure.Interval = "PT1M"
	t.Settings.RestartOnFailure.Count = 999
	t.Actions.Context = "Author"
	t.Actions.Exec.Command = exePath
	t.Actions.Exec.Arguments = "--agent"

	body, err := xml.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal task xml: %w", err)
	}
	// schtasks /XML expects UTF-16 LE on disk; the declaration must match the bytes
	// (Go's xml.Header says UTF-8 and would fail import).
	doc := append([]byte(`<?xml version="1.0" encoding="UTF-16"?>`+"\n"), body...)

	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	outDir := filepath.Join(dir, "find-uncommitted")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, "FindUncommittedAgent.xml")
	if err := os.WriteFile(path, utf16LEBOM(doc), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func utf16LEBOM(utf8 []byte) []byte {
	u16 := utf16.Encode([]rune(string(utf8)))
	out := make([]byte, 2+len(u16)*2)
	out[0], out[1] = 0xFF, 0xFE
	for i, v := range u16 {
		out[2+i*2] = byte(v)
		out[2+i*2+1] = byte(v >> 8)
	}
	return out
}
