package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GitRunner abstracts git command execution for tests.
type GitRunner interface {
	Run(dir string, args ...string) (stdout string, stderr string, err error)
}

// ExecGitRunner runs real git commands.
type ExecGitRunner struct{}

func (ExecGitRunner) Run(dir string, args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// SyncConfig controls state-repo git operations.
type SyncConfig struct {
	StateRepoDir string
	MachineID    string
	MaxRetries   int
	RetryDelay   time.Duration
	// Heartbeat forces a commit when status is unchanged but the last
	// published UpdatedAt is older than this, so remote views stay fresh.
	Heartbeat time.Duration
	Runner    GitRunner
}

func (c SyncConfig) runner() GitRunner {
	if c.Runner != nil {
		return c.Runner
	}
	return ExecGitRunner{}
}

func (c SyncConfig) retries() int {
	if c.MaxRetries <= 0 {
		return 3
	}
	return c.MaxRetries
}

func (c SyncConfig) delay() time.Duration {
	if c.RetryDelay <= 0 {
		return time.Second
	}
	return c.RetryDelay
}

func (c SyncConfig) heartbeat() time.Duration {
	if c.Heartbeat <= 0 {
		return 2 * time.Minute
	}
	return c.Heartbeat
}

// SyncWarning is a non-fatal sync issue suitable for agent loop logging.
type SyncWarning struct {
	Message string
	Err     error
}

func (w SyncWarning) Error() string {
	if w.Err == nil {
		return w.Message
	}
	return fmt.Sprintf("%s: %v", w.Message, w.Err)
}

// PullStateRepo fetches latest remote state with rebase (agent publish path).
func PullStateRepo(cfg SyncConfig) error {
	r := cfg.runner()
	_, stderr, err := r.Run(cfg.StateRepoDir, "pull", "--rebase", "--autostash")
	if err != nil {
		return SyncWarning{
			Message: "state repo pull failed (will retry later)",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}
	return nil
}

// PullStateRepoReadOnly updates the state clone for viewing without rewrite-heavy rebase.
func PullStateRepoReadOnly(cfg SyncConfig) error {
	r := cfg.runner()
	_, stderr, err := r.Run(cfg.StateRepoDir, "pull", "--ff-only")
	if err != nil {
		return SyncWarning{
			Message: "state repo fast-forward pull failed (using local snapshots)",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}
	return nil
}

// PublishLocalSnapshot writes/commits the machine file only when content changed
// or a heartbeat publish is due. Skipping must not rewrite updated_at on disk,
// or remote staleness detection breaks while the agent is still healthy.
// When content is unchanged, still push if local commits are ahead of upstream
// (e.g. previous tick committed but failed to push).
func PublishLocalSnapshot(cfg SyncConfig, snap MachineSnapshot) (published bool, err error) {
	path := SnapshotFilePath(cfg.StateRepoDir, cfg.MachineID)
	prev, readErr := ReadMachineSnapshot(path)
	needsCommit := true
	if readErr == nil {
		contentSame := SnapshotContentEqual(prev, snap)
		heartbeatDue := time.Since(prev.UpdatedAt) >= cfg.heartbeat()
		needsCommit = !contentSame || heartbeatDue
	}

	if !needsCommit {
		pushed, err := pushIfAhead(cfg)
		return pushed, err
	}

	if err := WriteMachineSnapshot(path, snap); err != nil {
		return false, err
	}
	if err := commitAndPush(cfg, path); err != nil {
		return false, err
	}
	return true, nil
}

func aheadOfUpstreamCount(cfg SyncConfig) (int, error) {
	r := cfg.runner()
	out, stderr, err := r.Run(cfg.StateRepoDir, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("unexpected rev-list output %q: %w", out, err)
	}
	return n, nil
}

// pushIfAhead rebases and pushes when local commits are not on the remote yet.
func pushIfAhead(cfg SyncConfig) (bool, error) {
	n, err := aheadOfUpstreamCount(cfg)
	if err != nil || n == 0 {
		// No upstream / not ahead: nothing to flush.
		return false, nil
	}
	if err := rebaseAndPush(cfg); err != nil {
		return false, err
	}
	return true, nil
}

func commitAndPush(cfg SyncConfig, relPath string) error {
	r := cfg.runner()
	addPath, err := filepath.Rel(cfg.StateRepoDir, relPath)
	if err != nil {
		addPath = relPath
	}

	if _, stderr, err := r.Run(cfg.StateRepoDir, "add", "--", addPath); err != nil {
		return SyncWarning{
			Message: "state repo git add failed",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}

	msg := fmt.Sprintf("update snapshot for %s", sanitizeMachineID(cfg.MachineID))
	if _, stderr, err := r.Run(cfg.StateRepoDir, "commit", "-m", msg); err != nil {
		// Nothing to commit is treated as success (race with equal content).
		combined := strings.ToLower(stderr + err.Error())
		if strings.Contains(combined, "nothing to commit") {
			return rebaseAndPush(cfg)
		}
		return SyncWarning{
			Message: "state repo commit failed",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}

	return rebaseAndPush(cfg)
}

func rebaseAndPush(cfg SyncConfig) error {
	r := cfg.runner()
	var lastErr error
	for attempt := 1; attempt <= cfg.retries(); attempt++ {
		if _, stderr, err := r.Run(cfg.StateRepoDir, "pull", "--rebase", "--autostash"); err != nil {
			lastErr = SyncWarning{
				Message: "state repo rebase before push failed",
				Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
			}
			time.Sleep(cfg.delay())
			continue
		}
		if _, stderr, err := r.Run(cfg.StateRepoDir, "push"); err != nil {
			lastErr = SyncWarning{
				Message: "state repo push failed",
				Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
			}
			time.Sleep(cfg.delay())
			continue
		}
		return nil
	}
	return lastErr
}
