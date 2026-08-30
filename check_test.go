package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGitToplevelRepoRootAndNested(t *testing.T) {
	dir := t.TempDir()
	run := exec.Command("git", "init")
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	nested := filepath.Join(dir, "src", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	top, err := resolveGitToplevel(ctx, dir)
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(top)
	if got != want {
		t.Fatalf("toplevel=%q want %q", got, want)
	}

	top2, err := resolveGitToplevel(ctx, nested)
	if err != nil {
		t.Fatalf("nested: %v", err)
	}
	got2, _ := filepath.EvalSymlinks(top2)
	if got2 != want {
		t.Fatalf("nested toplevel=%q want %q", got2, want)
	}
}

func TestResolveGitToplevelNotARepo(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveGitToplevel(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for non-git path")
	}
	if !strings.Contains(err.Error(), "not inside a git work tree") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveGitToplevelEmptyPath(t *testing.T) {
	_, err := resolveGitToplevel(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestFilterRowsByProjectKeysForCheck(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{Origin: "github.com/acme/app", Path: "/a/app", Branch: "main"}},
		{Machine: "desktop", Repo: RepoSnapshot{Origin: "github.com/acme/app", Path: "/b/app", Branch: "main"}},
		{Machine: "desktop", Repo: RepoSnapshot{Origin: "github.com/acme/other", Path: "/b/other", Branch: "main"}},
	}
	key := repoCorrelationKey(rows[0].Repo)
	filtered := FilterRowsByProjectKeys(rows, map[string]bool{key: true})
	if len(filtered) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(filtered), filtered)
	}
	for _, r := range filtered {
		if repoCorrelationKey(r.Repo) != key {
			t.Fatalf("unexpected row: %+v", r)
		}
	}
}

func TestPrintCheckNudges(t *testing.T) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printCheckNudges(nil)
	printCheckNudges([]Situation{{Nudge: "commit or stash local changes"}})

	w.Close()
	os.Stdout = old
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "→ ok") {
		t.Fatalf("missing ok line: %q", out)
	}
	if !strings.Contains(out, "→ commit or stash local changes") {
		t.Fatalf("missing nudge: %q", out)
	}
}

func TestRunCheckModeMissingPath(t *testing.T) {
	code := runCheckMode(context.Background(), "", "laptop", "", true, 0, false)
	if code != exitCheckError {
		t.Fatalf("exit=%d want %d", code, exitCheckError)
	}
}

func TestRunCheckModeLocalDirtyExitAttention(t *testing.T) {
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "t"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := runCheckMode(context.Background(), dir, "test-machine", "", true, 0, false)
	if code != exitCheckAttention {
		t.Fatalf("exit=%d want %d (dirty tree should need attention)", code, exitCheckAttention)
	}
}
