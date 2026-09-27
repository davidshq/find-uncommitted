//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
)

// detachAgentConsoleIfOwned frees the console when this process is the only one
// attached (typical for Task Scheduler at-logon). That closes the console window
// so the agent runs without a visible cmd window. An inherited terminal (user ran
// `find-uncommitted agent` interactively) has multiple processes on the console,
// so logging stays visible.
func detachAgentConsoleIfOwned() {
	var buf [8]uint32
	n, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	count := uint32(n)
	if count != 1 {
		return
	}
	_, _, _ = procFreeConsole.Call()
}
