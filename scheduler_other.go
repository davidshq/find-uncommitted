//go:build !windows && !linux

package main

import (
	"fmt"
	"runtime"
)

func installScheduler(_ string) error {
	return fmt.Errorf("scheduler install is not supported on %s (Windows and Linux only in this release)", runtime.GOOS)
}

func uninstallScheduler() error {
	return fmt.Errorf("scheduler uninstall is not supported on %s (Windows and Linux only in this release)", runtime.GOOS)
}
