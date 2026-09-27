//go:build windows

package main

import "testing"

func TestWindowsTaskStatusLine(t *testing.T) {
	out := "Folder: \\\nHostName: DESK\nTaskName: \\FindUncommittedAgent\nStatus: Running\n"
	if got := windowsTaskStatusLine(out); got != "Running" {
		t.Fatalf("got %q want Running", got)
	}
	if got := windowsTaskStatusLine("no status here"); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
