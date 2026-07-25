package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptedGit struct {
	mu    sync.Mutex
	calls [][]string
	// map of first-arg -> queue of results
	queue map[string][]gitResult
}

type gitResult struct {
	stdout string
	stderr string
	err    error
}

func newScriptedGit() *scriptedGit {
	return &scriptedGit{queue: map[string][]gitResult{}}
}

func (s *scriptedGit) enqueue(cmd string, res gitResult) {
	s.queue[cmd] = append(s.queue[cmd], res)
}

func (s *scriptedGit) Run(dir string, args ...string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := append([]string{}, args...)
	s.calls = append(s.calls, copied)
	key := args[0]
	q := s.queue[key]
	if len(q) == 0 {
		return "", "", fmt.Errorf("unexpected git %v", args)
	}
	res := q[0]
	s.queue[key] = q[1:]
	return res.stdout, res.stderr, res.err
}

func TestPullStateRepoOfflineWarning(t *testing.T) {
	g := newScriptedGit()
	g.enqueue("pull", gitResult{stderr: "network down", err: errors.New("exit 1")})
	err := PullStateRepo(SyncConfig{StateRepoDir: t.TempDir(), Runner: g, RetryDelay: time.Millisecond})
	var warn SyncWarning
	if !errors.As(err, &warn) {
		t.Fatalf("expected SyncWarning, got %T %v", err, err)
	}
	if !strings.Contains(warn.Message, "pull failed") {
		t.Fatalf("message: %s", warn.Message)
	}
}

func TestCommitAndPushRetriesThenSucceeds(t *testing.T) {
	dir := t.TempDir()
	g := newScriptedGit()
	g.enqueue("add", gitResult{})
	g.enqueue("commit", gitResult{})
	// first rebase+push fail, second succeed
	g.enqueue("pull", gitResult{err: errors.New("conflict"), stderr: "rebase failed"})
	g.enqueue("pull", gitResult{})
	g.enqueue("push", gitResult{})

	cfg := SyncConfig{
		StateRepoDir: dir,
		MachineID:    "box",
		MaxRetries:   3,
		RetryDelay:   time.Millisecond,
		Runner:       g,
	}
	path := SnapshotFilePath(dir, "box")
	if err := commitAndPush(cfg, path); err != nil {
		t.Fatalf("commitAndPush: %v", err)
	}

	var pushes, pulls int
	for _, c := range g.calls {
		switch c[0] {
		case "push":
			pushes++
		case "pull":
			pulls++
		}
	}
	if pulls < 2 || pushes != 1 {
		t.Fatalf("expected retry then success; pulls=%d pushes=%d calls=%v", pulls, pushes, g.calls)
	}
}

func TestPublishSkipsCommitWhenUnchangedWithinHeartbeat(t *testing.T) {
	dir := t.TempDir()
	g := newScriptedGit()
	g.enqueue("rev-list", gitResult{stdout: "0\n"})
	cfg := SyncConfig{
		StateRepoDir: dir,
		MachineID:    "box",
		Heartbeat:    time.Hour,
		Runner:       g,
	}
	path := SnapshotFilePath(dir, "box")
	snap := MachineSnapshot{
		MachineID: "box",
		UpdatedAt: time.Now().UTC().Add(-time.Minute),
		Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
		Meta:      ScanMetadata{RepoCount: 1, ScanRoot: "/"},
	}
	if err := WriteMachineSnapshot(path, snap); err != nil {
		t.Fatal(err)
	}
	next := snap
	next.UpdatedAt = time.Now().UTC()

	committed, err := PublishLocalSnapshot(cfg, next)
	if err != nil {
		t.Fatal(err)
	}
	if committed {
		t.Fatal("expected no commit when content unchanged and heartbeat not due")
	}
	if len(g.calls) != 1 || g.calls[0][0] != "rev-list" {
		t.Fatalf("expected ahead-check only, got %v", g.calls)
	}
	// Disk timestamp must stay unchanged so heartbeat can still fire later.
	got, err := ReadMachineSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(snap.UpdatedAt) {
		t.Fatalf("skip publish rewrote updated_at: got %v want %v", got.UpdatedAt, snap.UpdatedAt)
	}
}

func TestPublishPushesWhenAheadWithoutNewCommit(t *testing.T) {
	dir := t.TempDir()
	g := newScriptedGit()
	g.enqueue("rev-list", gitResult{stdout: "1\n"})
	g.enqueue("pull", gitResult{})
	g.enqueue("push", gitResult{})

	cfg := SyncConfig{
		StateRepoDir: dir,
		MachineID:    "box",
		Heartbeat:    time.Hour,
		RetryDelay:   time.Millisecond,
		Runner:       g,
	}
	path := SnapshotFilePath(dir, "box")
	snap := MachineSnapshot{
		MachineID: "box",
		UpdatedAt: time.Now().UTC().Add(-time.Minute),
		Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
		Meta:      ScanMetadata{RepoCount: 1, ScanRoot: "/"},
	}
	if err := WriteMachineSnapshot(path, snap); err != nil {
		t.Fatal(err)
	}
	next := snap
	next.UpdatedAt = time.Now().UTC()

	published, err := PublishLocalSnapshot(cfg, next)
	if err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Fatal("expected push of pending ahead commits")
	}
	got, err := ReadMachineSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(snap.UpdatedAt) {
		t.Fatal("push-only path should not rewrite snapshot updated_at")
	}
}

func TestPublishCommitsOnHeartbeat(t *testing.T) {
	dir := t.TempDir()
	g := newScriptedGit()
	g.enqueue("add", gitResult{})
	g.enqueue("commit", gitResult{})
	g.enqueue("pull", gitResult{})
	g.enqueue("push", gitResult{})

	cfg := SyncConfig{
		StateRepoDir: dir,
		MachineID:    "box",
		Heartbeat:    time.Second,
		RetryDelay:   time.Millisecond,
		Runner:       g,
	}
	snap := MachineSnapshot{
		MachineID: "box",
		UpdatedAt: time.Now().UTC().Add(-2 * time.Second),
		Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
		Meta:      ScanMetadata{RepoCount: 1, ScanRoot: "/"},
	}
	if err := WriteMachineSnapshot(SnapshotFilePath(dir, "box"), snap); err != nil {
		t.Fatal(err)
	}
	next := snap
	next.UpdatedAt = time.Now().UTC()

	committed, err := PublishLocalSnapshot(cfg, next)
	if err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("expected commit due to heartbeat")
	}
}

func TestBuildMachineSnapshotSortsByPath(t *testing.T) {
	results := []RepoStatus{
		{Path: "/z", Branch: "main", IsClean: true},
		{Path: "/a", Branch: "main", IsClean: true},
	}
	snap := BuildMachineSnapshot("m", "/", results, time.Now(), false)
	if snap.Repos[0].Path != "/a" || snap.Repos[1].Path != "/z" {
		t.Fatalf("unsorted: %+v", snap.Repos)
	}
}

func TestSmokePublishOnceWritesSnapshot(t *testing.T) {
	state := t.TempDir()
	scan := t.TempDir()
	g := newScriptedGit()
	g.enqueue("pull", gitResult{}) // PullStateRepo
	g.enqueue("add", gitResult{})
	g.enqueue("commit", gitResult{})
	g.enqueue("pull", gitResult{}) // rebaseAndPush
	g.enqueue("push", gitResult{})

	path, err := smokePublishOnce(AgentConfig{
		ScanRoot:     scan,
		StateRepoDir: state,
		MachineID:    "smoke-box",
		Sync: SyncConfig{
			StateRepoDir: state,
			MachineID:    "smoke-box",
			Runner:       g,
			RetryDelay:   time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("smokePublishOnce: %v", err)
	}
	want := SnapshotFilePath(state, "smoke-box")
	if path != want {
		t.Fatalf("path: got %q want %q", path, want)
	}
	if _, err := ReadMachineSnapshot(path); err != nil {
		t.Fatalf("snapshot not readable after smoke: %v", err)
	}
}
