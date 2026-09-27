package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

func printAgentStickyConfigHint() {
	fmt.Println("Agent settings (scan_root, interval, state_repo, …) come from sticky config.")
}
