package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestStateRepoSyncLockBlocksSecondNonBlockingAcquire(t *testing.T) {
	dir := t.TempDir()
	first, err := acquireStateRepoSyncLockBlocking(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	_, err = tryAcquireStateRepoSyncLock(dir)
	if !errors.Is(err, ErrStateRepoBusy) {
		t.Fatalf("expected ErrStateRepoBusy, got %v", err)
	}
}

func TestStateRepoSyncLockBlockingWaits(t *testing.T) {
	dir := t.TempDir()
	first, err := acquireStateRepoSyncLockBlocking(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		lock, err := acquireStateRepoSyncLockBlocking(context.Background(), dir)
		if err != nil {
			done <- err
			return
		}
		lock.Release()
		done <- nil
	}()

	time.Sleep(50 * time.Millisecond)
	first.Release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second acquire failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocking acquire did not complete after release")
	}
}

func TestStateRepoSyncLockBlockingRespectsContext(t *testing.T) {
	dir := t.TempDir()
	first, err := acquireStateRepoSyncLockBlocking(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err = acquireStateRepoSyncLockBlocking(ctx, dir)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("timed-out acquire hung too long: %v", elapsed)
	}
}

func TestStateRepoSyncLockConcurrentCLIAndAgent(t *testing.T) {
	dir := t.TempDir()
	agentLock, err := acquireStateRepoSyncLockBlocking(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := PullStateRepoReadOnly(context.Background(), SyncConfig{StateRepoDir: dir})
		if !errors.Is(err, ErrStateRepoBusy) {
			t.Errorf("CLI pull: expected ErrStateRepoBusy, got %v", err)
		}
	}()
	wg.Wait()
	agentLock.Release()
}
