//go:build windows

package main

import (
	"os/exec"
	"time"
)

func configureGitCmdCancel(cmd *exec.Cmd) {
	// Windows has no process-group kill equivalent here; CommandContext's
	// default Cancel (Process.Kill) still applies when Cancel is unset.
	// WaitDelay helps release pipe copy goroutines after Kill.
	cmd.WaitDelay = 100 * time.Millisecond
}
