// Package gitexec runs git subprocesses with per-command deadlines, cancellation,
// and non-interactive environment settings shared by scans and state-repo sync.
package gitexec

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

// DefaultCommandTimeoutString is the canonical duration string for a single git
// subprocess deadline. DefaultCommandTimeout is derived from it.
const DefaultCommandTimeoutString = "30s"

// DefaultCommandTimeout bounds a single git subprocess.
var DefaultCommandTimeout = mustDuration(DefaultCommandTimeoutString)

// maxErrorDetailLen caps stderr included in user-facing repo errors.
const maxErrorDetailLen = 200

func mustDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic("invalid default duration " + s + ": " + err.Error())
	}
	return d
}

// Run executes git under ctx with a per-command deadline, no TTY credential
// prompts, and cancelled subprocesses when the deadline expires.
func Run(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error) {
	return ExecGitRunner{}.Run(ctx, dir, args...)
}

// IsContextErr reports whether err (or ctx) indicates timeout/cancellation.
// exec.ErrWaitDelay is treated as a timeout so killed git children are not
// misreported as invalid repositories.
func IsContextErr(ctx context.Context, err error) bool {
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
	// Timeout overrides DefaultCommandTimeout when > 0.
	Timeout time.Duration
	// ExtraEnv is appended to the inherited environment (e.g. GIT_SSH_COMMAND).
	ExtraEnv []string
}

func (r ExecGitRunner) commandTimeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultCommandTimeout
}

func (r ExecGitRunner) Run(ctx context.Context, dir string, args ...string) (string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Do not Start a process that would be cancelled immediately — avoids a
	// spawn storm when a tick deadline has already fired.
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	cmdCtx, cancel := context.WithTimeout(ctx, r.commandTimeout())
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), r.ExtraEnv...)
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

// FormatError prefers trimmed git stderr (first fatal line when present) and
// falls back to the execution error. Callers handling timeouts should check
// IsContextErr before formatting so timeout wording stays distinct.
func FormatError(stderr string, err error) string {
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		if line := firstErrorLine(stderr); line != "" {
			return truncateErrorDetail(line)
		}
	}
	if err != nil {
		return err.Error()
	}
	return "unknown git error"
}

func firstErrorLine(stderr string) string {
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

func truncateErrorDetail(s string) string {
	if len(s) <= maxErrorDetailLen {
		return s
	}
	return s[:maxErrorDetailLen-3] + "..."
}

// ResolvedMaxWorkers returns the configured worker count or the built-in default.
func ResolvedMaxWorkers(requested int) int {
	if requested > 0 {
		return requested
	}
	return DefaultMaxWorkers
}

// RepoCheckWorkerCount returns the worker pool size for concurrent repo checks.
func RepoCheckWorkerCount(requested, repoCount int) int {
	maxWorkers := ResolvedMaxWorkers(requested)
	if maxWorkers > repoCount {
		maxWorkers = repoCount
	}
	return maxWorkers
}
