package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultMaxWorkers caps parallel repo status checks to reduce git/disk contention.
const DefaultMaxWorkers = 8

// DefaultGitCommandTimeout bounds a single git subprocess.
const DefaultGitCommandTimeout = 30 * time.Second

// maxGitErrorDetailLen caps stderr included in user-facing repo errors.
const maxGitErrorDetailLen = 200

// DefaultAgentTickTimeout bounds one agent publish tick (pull + scan + publish).
const DefaultAgentTickTimeout = 2 * time.Minute

// runGit executes git under ctx with a per-command deadline, no TTY credential
// prompts, and cancelled subprocesses when the deadline expires.
func runGit(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error) {
	return ExecGitRunner{}.Run(ctx, dir, args...)
}

// isGitContextErr reports whether err (or ctx) indicates timeout/cancellation.
// exec.ErrWaitDelay is treated as a timeout so killed git children are not
// misreported as invalid repositories.
func isGitContextErr(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, exec.ErrWaitDelay)
}

// GitRunner abstracts git command execution for tests.
type GitRunner interface {
	Run(ctx context.Context, dir string, args ...string) (stdout string, stderr string, err error)
}

// ExecGitRunner runs real git commands with deadlines and non-interactive env.
type ExecGitRunner struct {
	// Timeout overrides DefaultGitCommandTimeout when > 0.
	Timeout time.Duration
}

func (r ExecGitRunner) commandTimeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultGitCommandTimeout
}

func (r ExecGitRunner) Run(ctx context.Context, dir string, args ...string) (string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmdCtx, cancel := context.WithTimeout(ctx, r.commandTimeout())
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	configureGitCmdCancel(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if cmdCtx.Err() != nil {
			// Prefer the context error so callers can detect timeout/cancel reliably.
			err = cmdCtx.Err()
		} else if errors.Is(err, exec.ErrWaitDelay) {
			err = context.DeadlineExceeded
		}
	}
	return stdout.String(), stderr.String(), err
}

// formatGitError prefers trimmed git stderr (first fatal line when present) and
// falls back to the execution error. Callers handling timeouts should check
// isGitContextErr before formatting so timeout wording stays distinct.
func formatGitError(stderr string, err error) string {
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		if line := firstGitErrorLine(stderr); line != "" {
			return truncateGitErrorDetail(line)
		}
	}
	if err != nil {
		return err.Error()
	}
	return "unknown git error"
}

func firstGitErrorLine(stderr string) string {
	var fallback string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if fallback == "" {
			fallback = line
		}
		if strings.HasPrefix(strings.ToLower(line), "fatal:") {
			return strings.TrimSpace(line[len("fatal:"):])
		}
	}
	return fallback
}

func truncateGitErrorDetail(s string) string {
	if len(s) <= maxGitErrorDetailLen {
		return s
	}
	return s[:maxGitErrorDetailLen-3] + "..."
}

// resolvedMaxWorkers returns the configured worker count or the built-in default.
func resolvedMaxWorkers(requested int) int {
	if requested > 0 {
		return requested
	}
	return DefaultMaxWorkers
}

// repoCheckWorkerCount returns the worker pool size for concurrent repo checks.
func repoCheckWorkerCount(requested, repoCount int) int {
	maxWorkers := resolvedMaxWorkers(requested)
	if maxWorkers > repoCount {
		maxWorkers = repoCount
	}
	return maxWorkers
}
