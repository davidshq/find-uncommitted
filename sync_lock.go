package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrStateRepoBusy means another process holds the state-repo sync lock (agent publishing).
var ErrStateRepoBusy = errors.New("state repo sync in progress")

const stateRepoSyncLockName = "find-uncommitted-sync.lock"

// legacyStateRepoSyncLockName is the pre-2026-10 lock in the worktree root.
// It showed up as an untracked file in every clone, which led to a manual
// .gitignore commit that blocked XPS's rebase for 31h. Kept as a fallback for
// clones whose .git is not a directory, and so doctor can ignore leftovers.
const legacyStateRepoSyncLockName = ".find-uncommitted-sync.lock"

// stateRepoSyncLockPoll is how often a context-aware wait retries LOCK_NB.
const stateRepoSyncLockPoll = 50 * time.Millisecond

type stateRepoSyncLock struct {
	file *os.File
	path string
}

// stateRepoSyncLockPath keeps the lock inside .git so the agent never leaves
// untracked files in the worktree. Falls back to the worktree root when .git
// is missing or a file (linked worktree/submodule), matching rebaseInProgress.
func stateRepoSyncLockPath(stateRepoDir string) string {
	gitDir := filepath.Join(stateRepoDir, ".git")
	if st, err := os.Stat(gitDir); err == nil && st.IsDir() {
		return filepath.Join(gitDir, stateRepoSyncLockName)
	}
	return filepath.Join(stateRepoDir, legacyStateRepoSyncLockName)
}

// tryAcquireStateRepoSyncLock grabs the lock without blocking.
// Returns ErrStateRepoBusy when another process is syncing the clone.
func tryAcquireStateRepoSyncLock(stateRepoDir string) (*stateRepoSyncLock, error) {
	path := stateRepoSyncLockPath(stateRepoDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state repo lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open state repo lock: %w", err)
	}
	if lockErr := lockFileExclusive(f); lockErr != nil {
		_ = f.Close()
		if lockWouldBlock(lockErr) {
			return nil, ErrStateRepoBusy
		}
		return nil, fmt.Errorf("acquire state repo lock: %w", lockErr)
	}
	return &stateRepoSyncLock{file: f, path: path}, nil
}

// acquireStateRepoSyncLockBlocking waits until the lock is available or ctx is done.
// Polls LOCK_NB so tick-timeout and SIGTERM cancel the wait instead of hanging
// past systemd Restart=on-failure expectations.
func acquireStateRepoSyncLockBlocking(ctx context.Context, stateRepoDir string) (*stateRepoSyncLock, error) {
	for {
		lock, err := tryAcquireStateRepoSyncLock(stateRepoDir)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, ErrStateRepoBusy) {
			return nil, err
		}
		timer := time.NewTimer(stateRepoSyncLockPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("acquire state repo lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func (l *stateRepoSyncLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = unlockFile(l.file)
	_ = l.file.Close()
}
