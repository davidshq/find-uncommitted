package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFormatMatrixStatus(t *testing.T) {
	cases := []struct {
		name string
		repo RepoSnapshot
		want string
	}{
		{"clean", RepoSnapshot{IsClean: true, Branch: "main"}, "clean"},
		{"dirty", RepoSnapshot{IsDirty: true, Branch: "main"}, "dirty"},
		{"unpushed", RepoSnapshot{HasUnpushed: true, AheadCount: 3}, "↑3"},
		{"behind", RepoSnapshot{HasBehind: true, BehindCount: 2}, "↓2"},
		{"diverged", RepoSnapshot{HasBehind: true, HasUnpushed: true, AheadCount: 1, BehindCount: 2}, "↑1/↓2"},
		{"dirty+unpushed", RepoSnapshot{IsDirty: true, HasUnpushed: true, AheadCount: 3}, "dirty · ↑3"},
		{"dirty+behind", RepoSnapshot{IsDirty: true, HasBehind: true, BehindCount: 2}, "dirty · ↓2"},
		{"dirty+diverged", RepoSnapshot{IsDirty: true, HasBehind: true, HasUnpushed: true, AheadCount: 1, BehindCount: 2}, "dirty · ↑1/↓2"},
		{"empty", RepoSnapshot{IsEmpty: true}, "empty"},
		{"error", RepoSnapshot{Error: "boom"}, "error"},
		{"no upstream", RepoSnapshot{HasUntrackedUpstream: true, Branch: "main"}, "no upstream"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatMatrixStatus(tc.repo); got != tc.want {
				t.Fatalf("formatMatrixStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatMatrixCellBranchRules(t *testing.T) {
	dirty := AggregateRow{Repo: RepoSnapshot{IsDirty: true, Branch: "feature/x"}}
	got := formatMatrixCellForProject(dirty, false)
	if !strings.Contains(got, "dirty") || !strings.Contains(got, "feature/x") {
		t.Fatalf("non-clean should show branch: %q", got)
	}

	clean := AggregateRow{Repo: RepoSnapshot{IsClean: true, Branch: "main"}}
	got = formatMatrixCellForProject(clean, false)
	if got != "clean" {
		t.Fatalf("matching clean should omit branch: %q", got)
	}

	got = formatMatrixCellForProject(clean, true)
	if !strings.Contains(got, "main") {
		t.Fatalf("branch mismatch should show branch on clean: %q", got)
	}

	stale := AggregateRow{Stale: true, Repo: RepoSnapshot{HasUnpushed: true, AheadCount: 2, Branch: "main"}}
	got = formatMatrixCellForProject(stale, false)
	if !strings.Contains(got, "stale") || !strings.Contains(got, "↑2") {
		t.Fatalf("stale unpushed: %q", got)
	}
}

func TestCollapseRowsForMachineWorstWins(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{IsClean: true, Branch: "main", Path: "/a"}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{IsDirty: true, Branch: "main", Path: "/b"}},
	}
	got := collapseRowsForMachine(rows)
	if !got.Repo.IsDirty {
		t.Fatalf("expected dirty collapse, got %+v", got.Repo)
	}
	staleMix := []AggregateRow{
		{Machine: "desk", Stale: true, Repo: RepoSnapshot{IsClean: true}},
		{Machine: "desk", Repo: RepoSnapshot{HasUnpushed: true, AheadCount: 1}},
	}
	got = collapseRowsForMachine(staleMix)
	if !got.Stale || !got.Repo.HasUnpushed {
		t.Fatalf("expected stale+unpushed: %+v", got)
	}
}

func TestMatrixColumnsLocalFirst(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "zebra", Repo: RepoSnapshot{Origin: "o", Path: "/z"}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{Origin: "o", Path: "/l"}},
		{Machine: "alpha", Repo: RepoSnapshot{Origin: "o", Path: "/a"}},
	}
	cols := matrixColumnsFromRows(rows)
	if len(cols) != 3 || !cols[0].Local || cols[0].ID != "laptop" {
		t.Fatalf("local first: %+v", cols)
	}
	if cols[1].ID != "alpha" || cols[2].ID != "zebra" {
		t.Fatalf("remotes sorted: %+v", cols)
	}
}

func TestDisplayProjectMachineMatrixDirtyOnlyShape(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Origin: "github.com/you/work", Path: "/repos/work", Branch: "feature/pay",
			IsDirty: true, HasUnstaged: true,
		}},
		{Machine: "desktop", Repo: RepoSnapshot{
			Origin: "github.com/you/work", Path: "/Users/you/work", Branch: "feature/pay",
			IsDirty: true, HasUnstaged: true,
		}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Origin: "github.com/you/clean", Path: "/repos/clean", Branch: "main", IsClean: true,
		}},
		{Machine: "desktop", Repo: RepoSnapshot{
			Origin: "github.com/you/clean", Path: "/Users/you/clean", Branch: "main", IsClean: true,
		}},
	}
	situations := DetectSituations(rows)
	keys := ProjectKeysWithSituations(situations)
	filtered := FilterRowsByProjectKeys(rows, keys)

	out := captureStdout(t, func() {
		displayProjectMachineMatrix(filtered)
	})
	if !strings.Contains(out, "Projects") {
		t.Fatalf("expected Projects header: %s", out)
	}
	if !strings.Contains(out, "github.com/you/work") {
		t.Fatalf("expected dirty project: %s", out)
	}
	if strings.Contains(out, "github.com/you/clean") {
		t.Fatalf("clean project should be filtered out: %s", out)
	}
	if !strings.Contains(out, "dirty") {
		t.Fatalf("expected dirty cells: %s", out)
	}
	if strings.Contains(out, "Attention") || strings.Contains(out, "Full inventory") {
		t.Fatalf("matrix must not include Attention/inventory heroes: %s", out)
	}
}

func TestDisplayProjectMachineMatrixSizesToCellContent(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Origin: "github.com/you/work", Path: "/repos/work",
			Branch: "feature/payment-refactor", IsDirty: true, HasUnpushed: true, AheadCount: 3,
		}},
		{Machine: "desktop", Repo: RepoSnapshot{
			Origin: "github.com/you/work", Path: "/Users/you/work",
			Branch: "feature/payment-refactor", IsClean: true,
		}},
	}
	out := captureStdout(t, func() {
		displayProjectMachineMatrix(rows)
	})
	if !strings.Contains(out, "dirty · ↑3") {
		t.Fatalf("expected composed dirty+unpushed: %s", out)
	}
	if !strings.Contains(out, "feature/payment-refactor") {
		t.Fatalf("column should be wide enough for branch: %s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
