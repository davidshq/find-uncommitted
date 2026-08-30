package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Exit codes for check mode (full scan still exits 0 on success).
const (
	exitCheckOK        = 0
	exitCheckError     = 1
	exitCheckAttention = 2
)

// resolveGitToplevel returns the git work-tree root for path (file or directory).
// It does not discover nested repos under a non-git directory.
func resolveGitToplevel(ctx context.Context, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("path %q: %w", path, err)
	}
	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	out, stderr, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%q is not inside a git work tree: %s", path, detail)
	}
	top := strings.TrimSpace(out)
	if top == "" {
		return "", fmt.Errorf("%q is not inside a git work tree", path)
	}
	return top, nil
}

// runCheckMode live-checks one repo, correlates remotes, prints a compact
// summary plus Attention nudges, and returns an exit code (0 / 1 / 2).
func runCheckMode(ctx context.Context, path, machineID, stateRepo string, skipRemote bool, staleTTL time.Duration) int {
	if strings.TrimSpace(path) == "" {
		fmt.Fprintln(os.Stderr, "Error: check requires a path")
		fmt.Fprintln(os.Stderr, "Usage: find-uncommitted [flags] check <path>")
		return exitCheckError
	}

	top, err := resolveGitToplevel(ctx, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return exitCheckError
	}

	status := checkRepoStatus(ctx, top)
	localResults := []RepoStatus{status}

	var remote []LoadedSnapshot
	if stateRepo != "" && !skipRemote {
		if abs, err := filepath.Abs(stateRepo); err == nil {
			stateRepo = abs
		}
		if err := validateStateRepo(stateRepo); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintln(os.Stderr, "Fix state_repo in sticky config / env / --state-repo, or pass --no-remote for a local-only check.")
			return exitCheckError
		}
		if err := PullStateRepoReadOnly(ctx, SyncConfig{StateRepoDir: stateRepo, MachineID: machineID}); err != nil {
			if errors.Is(err, ErrStateRepoBusy) {
				fmt.Fprintln(os.Stderr, "warning: agent is syncing state repo; using on-disk snapshots")
			} else {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}
		remote, err = LoadAllMachineSnapshots(stateRepo, staleTTL, time.Now().UTC())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed loading remote snapshots: %v\n", err)
			remote = nil
		} else {
			warnCorruptSnapshots(remote)
		}
	}

	var situations []Situation
	var projectRows []AggregateRow

	if len(remote) > 0 {
		rows := BuildAggregateRows(machineID, localResults, remote)
		key := repoCorrelationKey(RepoStatusToSnapshot(status, false))
		projectRows = FilterRowsByProjectKeys(rows, map[string]bool{key: true})
		situations = DetectSituations(projectRows)
	} else {
		situations = DetectLocalSituations(machineID, localResults)
		projectRows = []AggregateRow{{
			Machine: machineID,
			Local:   true,
			Repo:    RepoStatusToSnapshot(status, false),
		}}
	}

	label := projectLabelForCheck(projectRows, status)
	printCheckSummary(label, projectRows)
	printCheckNudges(situations)

	if len(situations) > 0 {
		return exitCheckAttention
	}
	return exitCheckOK
}

func projectLabelForCheck(rows []AggregateRow, status RepoStatus) string {
	for _, row := range rows {
		if row.LoadError != "" {
			continue
		}
		return projectLabel(row.Repo)
	}
	return projectLabel(RepoStatusToSnapshot(status, false))
}

// printCheckSummary prints one compact project × machine line.
func printCheckSummary(label string, rows []AggregateRow) {
	var parts []string
	for _, row := range rows {
		if row.LoadError != "" {
			continue
		}
		machine := row.Machine
		if row.Local {
			machine += "*"
		}
		if row.Stale {
			machine += " (stale)"
		}
		st, ch := repoStatusText(row.Repo, true)
		cell := st
		if row.Repo.Branch != "" {
			cell = fmt.Sprintf("%s on %s", st, row.Repo.Branch)
		}
		if ch != "" && ch != "-" {
			cell = fmt.Sprintf("%s (%s)", cell, ch)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", machine, cell))
	}
	if len(parts) == 0 {
		fmt.Printf("%s  (no status)\n", label)
		return
	}
	fmt.Printf("%s  %s\n", label, strings.Join(parts, "  ·  "))
}

// printCheckNudges prints Attention cues without repeating the project label.
func printCheckNudges(situations []Situation) {
	if len(situations) == 0 {
		fmt.Println("→ ok")
		return
	}
	for _, s := range situations {
		nudge := strings.TrimSpace(s.Nudge)
		if nudge == "" {
			continue
		}
		fmt.Printf("→ %s\n", nudge)
	}
}
