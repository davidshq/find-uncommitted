//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// CREATE_NO_WINDOW prevents git (and helpers like taskkill) from allocating a
// console, which would flash a terminal window during background agent ticks.
const createNoWindow = 0x08000000

func hideWindowSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

func configureGitCmdCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = hideWindowSysProcAttr()
	// Windows TerminateProcess (CommandContext default) does not kill children.
	// Git often spawns a nested git.exe; those orphans accumulate across agent
	// tick cancellations. taskkill /T kills the tree; CREATE_NO_WINDOW avoids a
	// visible console flash on every cancel.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		pid := cmd.Process.Pid
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
		kill.SysProcAttr = hideWindowSysProcAttr()
		_ = kill.Run()
		_ = cmd.Process.Kill()
		return nil
	}
	cmd.WaitDelay = 100 * time.Millisecond
}
