package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func backlogTestSnapshot(at time.Time, branch string) MachineSnapshot {
	return MachineSnapshot{
		MachineID: "box",
		UpdatedAt: at,
		Repos:     []RepoSnapshot{{Path: "/a", Branch: branch, IsClean: true}},
		Meta:      ScanMetadata{RepoCount: 1, ScanRoot: "/"},
	}
}

// Heartbeat due while a previous commit is unpushed and the flush still fails:
// no new commit, disk untouched, error says the publish is blocked.
func TestPublishDoesNotStackCommitsWhileFlushFails(t *testing.T) {
	dir := t.TempDir()
	g := newScriptedGit()
	g.enqueue("rev-list", gitResult{stdout: "1\n"})
	g.enqueue("pull", gitResult{
		err:    errors.New("exit status 1"),
		stderr: "error: The following untracked working tree files would be overwritten by checkout:\n\t.gitignore",
	})

	cfg := SyncConfig{
		StateRepoDir: dir,
		MachineID:    "box",
		Heartbeat:    time.Second,
		MaxRetries:   1,
		RetryDelay:   time.Millisecond,
		Runner:       g,
	}
	path := SnapshotFilePath(dir, "box")
	prev := backlogTestSnapshot(time.Now().UTC().Add(-time.Minute), "main")
	if err := WriteMachineSnapshot(path, prev); err != nil {
		t.Fatal(err)
	}
	next := backlogTestSnapshot(time.Now().UTC(), "feature")

	published, err := PublishLocalSnapshot(context.Background(), cfg, next)
	if published {
		t.Fatal("blocked publish must not report success")
	}
	var warn SyncWarning
	if !errors.As(err, &warn) || !strings.Contains(warn.Message, "publish blocked") {
		t.Fatalf("expected publish-blocked warning, got %v", err)
	}
	for _, c := range g.calls {
		if c[0] == "add" || c[0] == "commit" {
			t.Fatalf("stacked a commit while ahead and flush failing: %v", g.calls)
		}
	}
	got, err := ReadMachineSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(prev.UpdatedAt) || !SnapshotContentEqual(got, prev) {
		t.Fatalf("blocked publish rewrote snapshot on disk: %+v", got)
	}
}

// Once the backlog flushes, the same tick commits and pushes the new snapshot.
func TestPublishFlushesBacklogThenCommits(t *testing.T) {
	dir := t.TempDir()
	g := newScriptedGit()
	g.enqueue("rev-list", gitResult{stdout: "1\n"})
	g.enqueue("pull", gitResult{}) // flush
	g.enqueue("push", gitResult{})
	g.enqueue("add", gitResult{})
	g.enqueue("commit", gitResult{})
	g.enqueue("pull", gitResult{}) // publish
	g.enqueue("push", gitResult{})

	cfg := SyncConfig{
		StateRepoDir: dir,
		MachineID:    "box",
		Heartbeat:    time.Hour,
		RetryDelay:   time.Millisecond,
		Runner:       g,
	}
	path := SnapshotFilePath(dir, "box")
	if err := WriteMachineSnapshot(path, backlogTestSnapshot(time.Now().UTC().Add(-time.Minute), "main")); err != nil {
		t.Fatal(err)
	}
	next := backlogTestSnapshot(time.Now().UTC(), "feature")

	published, err := PublishLocalSnapshot(context.Background(), cfg, next)
	if err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Fatal("expected publish after backlog flush")
	}
	got, err := ReadMachineSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if !SnapshotContentEqual(got, next) {
		t.Fatalf("new snapshot not written: %+v", got)
	}
}

// Real-git repro of the 2026-09-29 XPS incident: an untracked .gitignore in the
// agent's clone collides with a .gitignore commit pushed from another clone, so
// every pull --rebase refuses to start. Before the gate, each heartbeat added a
// local commit (742 in 31h). Now the backlog stays at one until the collision
// is cleared, then publishing resumes.
func TestPublishBacklogStaysBoundedOnUntrackedCollision(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	agent := filepath.Join(root, "agent")
	human := filepath.Join(root, "human")
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "-q", "--bare", "-b", "main", bare)
	seed := filepath.Join(root, "seed")
	git(root, "init", "-q", "-b", "main", seed)
	git(seed, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	git(seed, "push", "-q", bare, "main")
	for _, dir := range []string{agent, human} {
		git(root, "clone", "-q", bare, dir)
		git(dir, "config", "user.name", "t")
		git(dir, "config", "user.email", "t@t")
	}

	cfg := SyncConfig{
		StateRepoDir: agent,
		MachineID:    "box",
		Heartbeat:    time.Nanosecond, // every call is a heartbeat
		MaxRetries:   1,
		RetryDelay:   time.Millisecond,
	}
	ahead := func() int {
		n, err := aheadOfUpstreamCount(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Healthy first publish.
	if _, err := PublishLocalSnapshot(context.Background(), cfg, backlogTestSnapshot(time.Now().UTC(), "main")); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	// The collision: same path untracked in the agent clone, tracked upstream.
	if err := os.WriteFile(filepath.Join(agent, ".gitignore"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(human, "pull", "-q", "--ff-only")
	if err := os.WriteFile(filepath.Join(human, ".gitignore"), []byte("upstream\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(human, "add", ".gitignore")
	git(human, "commit", "-q", "-m", "add gitignore")
	git(human, "push", "-q")
	git(agent, "fetch", "-q")

	for i := 0; i < 5; i++ {
		_, err := PublishLocalSnapshot(context.Background(), cfg, backlogTestSnapshot(time.Now().UTC(), "main"))
		if err == nil {
			t.Fatalf("tick %d: expected stuck publish to fail", i)
		}
		if n := ahead(); n > 1 {
			t.Fatalf("tick %d: backlog grew to %d commits", i, n)
		}
	}

	// Operator clears the collision; the next tick flushes and publishes.
	if err := os.Remove(filepath.Join(agent, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	published, err := PublishLocalSnapshot(context.Background(), cfg, backlogTestSnapshot(time.Now().UTC(), "main"))
	if err != nil || !published {
		t.Fatalf("expected recovery publish, got published=%v err=%v", published, err)
	}
	if n := ahead(); n != 0 {
		t.Fatalf("still %d ahead after recovery", n)
	}
	if out := git(agent, "status", "--porcelain"); out != "" {
		t.Fatalf("agent left worktree dirty or with untracked files:\n%s", out)
	}
}
