package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

// DefaultGitCommandTimeout bounds a single git subprocess.
const DefaultGitCommandTimeout = 30 * time.Second

// DefaultAgentTickTimeout bounds one agent publish tick (pull + scan + publish).
const DefaultAgentTickTimeout = 2 * time.Minute

// runGit executes git under ctx with a per-command deadline, no TTY credential
// prompts, and cancelled subprocesses when the deadline expires.
func runGit(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error) {
	return ExecGitRunner{}.Run(ctx, dir, args...)
}

// isGitContextErr reports whether err (or ctx) indicates timeout/cancellation.
func isGitContextErr(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
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
	if err != nil && cmdCtx.Err() != nil {
		// Prefer the context error so callers can detect timeout/cancel reliably.
		err = cmdCtx.Err()
	}
	return stdout.String(), stderr.String(), err
}
