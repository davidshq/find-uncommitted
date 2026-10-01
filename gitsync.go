package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davidshq/find-uncommitted/internal/gitexec"
)

// SyncConfig controls state-repo git operations.
type SyncConfig struct {
	StateRepoDir string
	MachineID    string
	MaxRetries   int
	RetryDelay   time.Duration
	// Heartbeat forces a commit when status is unchanged but the last
	// published UpdatedAt is older than this, so remote views stay fresh.
	// Zero means DefaultHeartbeat. Sticky config key: heartbeat.
	Heartbeat time.Duration
	Runner    gitexec.GitRunner
}

func (c SyncConfig) runner() gitexec.GitRunner {
	if c.Runner != nil {
		return c.Runner
	}
	return gitexec.ExecGitRunner{}
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
		return DefaultHeartbeat
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

func (w SyncWarning) Unwrap() error { return w.Err }

// PullStateRepo fetches latest remote state with rebase (agent publish path).
func PullStateRepo(ctx context.Context, cfg SyncConfig) error {
	lock, err := acquireStateRepoSyncLockBlocking(ctx, cfg.StateRepoDir)
	if err != nil {
		return SyncWarning{
			Message: "could not acquire state repo sync lock",
			Err:     err,
		}
	}
	defer lock.Release()
	return pullStateRepoLocked(ctx, cfg)
}

func pullStateRepoLocked(ctx context.Context, cfg SyncConfig) error {
	r := cfg.runner()
	if err := abortRebaseIfInProgress(ctx, cfg); err != nil {
		return err
	}
	_, stderr, err := r.Run(ctx, cfg.StateRepoDir, "pull", "--rebase", "--autostash")
	if err != nil {
		// Leave the clone usable for the next tick when pull --rebase stalls mid-rebase.
		_ = abortRebaseIfInProgress(ctx, cfg)
		return SyncWarning{
			Message: "state repo pull failed (will retry later)",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}
	return nil
}

// PullStateRepoReadOnly updates the state clone for viewing without rewrite-heavy rebase.
// Skips the pull when the sync lock is held (agent publishing) and returns ErrStateRepoBusy.
func PullStateRepoReadOnly(ctx context.Context, cfg SyncConfig) error {
	lock, err := tryAcquireStateRepoSyncLock(cfg.StateRepoDir)
	if err != nil {
		return err
	}
	defer lock.Release()
	return pullStateRepoReadOnlyLocked(ctx, cfg)
}

// readOnlyPullTimeout bounds the viewer pull (check, cd hook, extension). A
// slightly stale answer beats a hung `cd`; the agent keeps the clone fresh.
const readOnlyPullTimeout = 10 * time.Second

// viewerSSHCommand makes ssh fail fast instead of prompting on /dev/tty
// (GIT_TERMINAL_PROMPT does not stop ssh's own passphrase/host-key prompts) or
// waiting on an unreachable host.
const viewerSSHCommand = "ssh -o BatchMode=yes -o ConnectTimeout=5"

// viewerSSHEnv returns GIT_SSH_COMMAND for viewer pulls unless the user already
// chose an ssh command (GIT_SSH_COMMAND, GIT_SSH or core.sshCommand), which wins.
func viewerSSHEnv(ctx context.Context, stateRepo string) []string {
	if os.Getenv("GIT_SSH_COMMAND") != "" || os.Getenv("GIT_SSH") != "" {
		return nil
	}
	if out, _, err := gitexec.Run(ctx, stateRepo, "config", "--get", "core.sshCommand"); err == nil && strings.TrimSpace(out) != "" {
		return nil
	}
	return []string{"GIT_SSH_COMMAND=" + viewerSSHCommand}
}

func pullStateRepoReadOnlyLocked(ctx context.Context, cfg SyncConfig) error {
	r := cfg.runner()
	if cfg.Runner == nil {
		r = gitexec.ExecGitRunner{Timeout: readOnlyPullTimeout, ExtraEnv: viewerSSHEnv(ctx, cfg.StateRepoDir)}
	}
	_, stderr, err := r.Run(ctx, cfg.StateRepoDir, "pull", "--ff-only")
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
//
// The on-disk file is the freshness clock consumers trust. A write that advances
// updated_at without a successful commit would make the next tick skip (content
// equal + heartbeat not due) while pushIfAhead ignores a dirty worktree — remotes
// stay stale. So: restore the previous snapshot if commit fails after write, and
// force a commit when the snapshot path is already dirty (orphan recovery).
// When content is unchanged and the worktree is clean, still push if local
// commits are ahead of upstream (e.g. previous tick committed but failed to push).
//
// Backlog gate: when local is known to be ahead, flush before committing and
// skip the commit if the flush fails. Otherwise a deterministically stuck push
// adds one local commit per heartbeat (742 in the 2026-09-29 XPS incident).
// The scan is rebuilt every tick, so skipping a commit loses nothing.
func PublishLocalSnapshot(ctx context.Context, cfg SyncConfig, snap MachineSnapshot) (published bool, err error) {
	path := SnapshotFilePath(cfg.StateRepoDir, cfg.MachineID)
	prev, readErr := ReadMachineSnapshot(path)
	hadPrev := readErr == nil
	needsCommit := true
	if hadPrev {
		contentSame := SnapshotContentEqual(prev, snap)
		heartbeatDue := time.Since(prev.UpdatedAt) >= cfg.heartbeat()
		needsCommit = !contentSame || heartbeatDue
	}

	if !needsCommit {
		dirty, dirtyErr := snapshotPathDirty(ctx, cfg, path)
		if dirtyErr != nil {
			return false, SyncWarning{
				Message: "could not check snapshot worktree state",
				Err:     dirtyErr,
			}
		}
		if dirty {
			// Prior write left an uncommitted snapshot; commit it this tick.
			needsCommit = true
		} else {
			pushed, err := pushIfAhead(ctx, cfg)
			return pushed, err
		}
	}

	// Only gate on a known backlog: rev-list also fails on a fresh clone of an
	// empty remote, and that first publish must still commit.
	if n, aheadErr := aheadOfUpstreamCount(ctx, cfg); aheadErr == nil && n > 0 {
		if err := rebaseAndPush(ctx, cfg); err != nil {
			return false, SyncWarning{
				Message: fmt.Sprintf("state repo publish blocked: %d local commit(s) not pushed; not adding another until push succeeds", n),
				Err:     err,
			}
		}
	}

	if err := WriteMachineSnapshot(path, snap); err != nil {
		return false, err
	}
	if err := commitSnapshot(ctx, cfg, path); err != nil {
		if restoreErr := restorePublishedSnapshot(path, prev, hadPrev); restoreErr != nil {
			return false, fmt.Errorf("%w (also failed to restore snapshot: %v)", err, restoreErr)
		}
		return false, err
	}
	if err := rebaseAndPush(ctx, cfg); err != nil {
		// Commit landed; leave the new file. Next tick's pushIfAhead retries.
		return false, err
	}
	return true, nil
}

// snapshotPathDirty reports whether the machine snapshot has uncommitted changes.
func snapshotPathDirty(ctx context.Context, cfg SyncConfig, absPath string) (bool, error) {
	rel, err := filepath.Rel(cfg.StateRepoDir, absPath)
	if err != nil {
		rel = absPath
	}
	out, stderr, err := cfg.runner().Run(ctx, cfg.StateRepoDir, "status", "--porcelain", "--", rel)
	if err != nil {
		return false, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(out) != "", nil
}

// restorePublishedSnapshot puts the last successfully-read snapshot back on disk
// after a failed commit so updated_at does not claim a publish that never landed.
// When there was no prior file, remove the orphan write.
func restorePublishedSnapshot(path string, prev MachineSnapshot, hadPrev bool) error {
	if hadPrev {
		return WriteMachineSnapshot(path, prev)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func aheadOfUpstreamCount(ctx context.Context, cfg SyncConfig) (int, error) {
	r := cfg.runner()
	out, stderr, err := r.Run(ctx, cfg.StateRepoDir, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("unexpected rev-list output %q: %w", out, err)
	}
	return n, nil
}

// isNoUpstreamRevListError reports @{u} resolution failures that mean there is
// nothing to flush (no tracking branch), as opposed to transient remote errors.
func isNoUpstreamRevListError(err error) bool {
	if err == nil {
		return false
	}
	combined := strings.ToLower(err.Error())
	return strings.Contains(combined, "no upstream configured") ||
		strings.Contains(combined, "does not point to a branch")
}

// pushIfAhead rebases and pushes when local commits are not on the remote yet.
// No-upstream is treated as nothing to flush. Other rev-list failures surface
// as SyncWarning so the next tick retries instead of silently leaving commits
// unpublished after a prior commit-ok/push-fail.
func pushIfAhead(ctx context.Context, cfg SyncConfig) (bool, error) {
	n, err := aheadOfUpstreamCount(ctx, cfg)
	if err != nil {
		if isNoUpstreamRevListError(err) {
			return false, nil
		}
		return false, SyncWarning{
			Message: "could not check if state repo is ahead of upstream",
			Err:     err,
		}
	}
	if n == 0 {
		return false, nil
	}
	if err := rebaseAndPush(ctx, cfg); err != nil {
		return false, err
	}
	return true, nil
}

func commitAndPush(ctx context.Context, cfg SyncConfig, absPath string) error {
	if err := commitSnapshot(ctx, cfg, absPath); err != nil {
		return err
	}
	return rebaseAndPush(ctx, cfg)
}

// commitSnapshot stages and commits the machine snapshot file. A clean
// "nothing to commit" race is treated as success (caller may still push).
func commitSnapshot(ctx context.Context, cfg SyncConfig, absPath string) error {
	r := cfg.runner()
	addPath, err := filepath.Rel(cfg.StateRepoDir, absPath)
	if err != nil {
		addPath = absPath
	}

	if _, stderr, err := r.Run(ctx, cfg.StateRepoDir, "add", "--", addPath); err != nil {
		return SyncWarning{
			Message: "state repo git add failed",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}

	msg := fmt.Sprintf("update snapshot for %s", sanitizeMachineID(cfg.MachineID))
	if _, stderr, err := r.Run(ctx, cfg.StateRepoDir, "commit", "-m", msg); err != nil {
		combined := strings.ToLower(stderr + err.Error())
		if strings.Contains(combined, "nothing to commit") {
			return nil
		}
		return SyncWarning{
			Message: "state repo commit failed",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}
	return nil
}

func rebaseAndPush(ctx context.Context, cfg SyncConfig) error {
	r := cfg.runner()
	var lastErr error
	for attempt := 1; attempt <= cfg.retries(); attempt++ {
		if err := ctx.Err(); err != nil {
			_ = abortRebaseIfInProgress(ctx, cfg)
			return SyncWarning{
				Message: "state repo sync cancelled",
				Err:     err,
			}
		}
		// A prior failed pull --rebase can leave the state clone mid-rebase;
		// abort before retrying so publishes self-recover instead of sticking.
		if err := abortRebaseIfInProgress(ctx, cfg); err != nil {
			lastErr = err
			if gitexec.IsContextErr(ctx, err) {
				return lastErr
			}
			time.Sleep(cfg.delay())
			continue
		}
		if _, stderr, err := r.Run(ctx, cfg.StateRepoDir, "pull", "--rebase", "--autostash"); err != nil {
			lastErr = SyncWarning{
				Message: "state repo rebase before push failed",
				Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
			}
			_ = abortRebaseIfInProgress(ctx, cfg)
			if gitexec.IsContextErr(ctx, err) {
				return lastErr
			}
			time.Sleep(cfg.delay())
			continue
		}
		if _, stderr, err := r.Run(ctx, cfg.StateRepoDir, "push"); err != nil {
			lastErr = SyncWarning{
				Message: "state repo push failed",
				Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
			}
			if gitexec.IsContextErr(ctx, err) {
				return lastErr
			}
			time.Sleep(cfg.delay())
			continue
		}
		return nil
	}
	_ = abortRebaseIfInProgress(ctx, cfg)
	return lastErr
}

// rebaseInProgress reports whether the state clone has an unfinished rebase.
// Uses on-disk rebase-merge/rebase-apply markers under .git (normal clone layout).
func rebaseInProgress(stateRepoDir string) bool {
	gitDir := filepath.Join(stateRepoDir, ".git")
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		if st, err := os.Stat(filepath.Join(gitDir, name)); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

// abortRebaseIfInProgress clears a stuck rebase so the next pull --rebase can proceed.
// No-op when the clone is not mid-rebase.
func abortRebaseIfInProgress(ctx context.Context, cfg SyncConfig) error {
	if !rebaseInProgress(cfg.StateRepoDir) {
		return nil
	}
	r := cfg.runner()
	if _, stderr, err := r.Run(ctx, cfg.StateRepoDir, "rebase", "--abort"); err != nil {
		return SyncWarning{
			Message: "state repo rebase abort failed",
			Err:     fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
		}
	}
	return nil
}
