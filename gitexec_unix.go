//go:build unix

package main

import (
	"os/exec"
	"syscall"
	"time"
)

func configureGitCmdCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Kill the whole process group so credential helpers / pager children
	// cannot keep stdout pipes open after the parent is cancelled.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 100 * time.Millisecond
}
