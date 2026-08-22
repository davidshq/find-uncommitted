package main

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDisplayPathDot(t *testing.T) {
	wd := "/home/user/projects/find-uncommitted"
	got := displayPath(wd, wd)
	if got != "." {
		t.Fatalf("displayPath(wd, wd) = %q, want '.'", got)
	}
}

func TestDisplayPathRelative(t *testing.T) {
	wd := "/home/user/projects"
	got := displayPath(wd, "/home/user/projects/foo/bar")
	if got != "foo/bar" {
		t.Fatalf("got %q, want foo/bar", got)
	}
}

func TestCheckRepoStatusesSortsByPath(t *testing.T) {
	ctx := contextWithCancelledGit()
	repos := []string{
		filepath.Join("z", "repo"),
		filepath.Join("a", "repo"),
		filepath.Join("m", "repo"),
	}
	results := checkRepoStatuses(ctx, repos, false)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Path > results[i].Path {
			t.Fatalf("results not sorted: %q before %q", results[i-1].Path, results[i].Path)
		}
	}
}

func contextWithCancelledGit() context.Context {
	// checkRepoStatus will error quickly on invalid paths; sorting is independent.
	return context.Background()
}
