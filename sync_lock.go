package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrStateRepoBusy means another process holds the state-repo sync lock (agent publishing).
var ErrStateRepoBusy = errors.New("state repo sync in progress")

const stateRepoSyncLockName = ".find-uncommitted-sync.lock"

type stateRepoSyncLock struct {
	file *os.File
	path string
}

func stateRepoSyncLockPath(stateRepoDir string) string {
	return filepath.Join(stateRepoDir, stateRepoSyncLockName)
}

// tryAcquireStateRepoSyncLock grabs the lock without blocking.
// Returns ErrStateRepoBusy when another process is syncing the clone.
func tryAcquireStateRepoSyncLock(stateRepoDir string) (*stateRepoSyncLock, error) {
	return acquireStateRepoSyncLock(stateRepoDir, true)
}

// acquireStateRepoSyncLockBlocking waits until the lock is available.
func acquireStateRepoSyncLockBlocking(stateRepoDir string) (*stateRepoSyncLock, error) {
	return acquireStateRepoSyncLock(stateRepoDir, false)
}

func acquireStateRepoSyncLock(stateRepoDir string, nonBlocking bool) (*stateRepoSyncLock, error) {
	path := stateRepoSyncLockPath(stateRepoDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state repo lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open state repo lock: %w", err)
	}
	var lockErr error
	if nonBlocking {
		lockErr = lockFileExclusive(f)
	} else {
		lockErr = lockFileExclusiveBlocking(f)
	}
	if lockErr != nil {
		_ = f.Close()
		if nonBlocking && lockWouldBlock(lockErr) {
			return nil, ErrStateRepoBusy
		}
		return nil, fmt.Errorf("acquire state repo lock: %w", lockErr)
	}
	return &stateRepoSyncLock{file: f, path: path}, nil
}

func (l *stateRepoSyncLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = unlockFile(l.file)
	_ = l.file.Close()
}
