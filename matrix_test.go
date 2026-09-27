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
	got := formatMatrixCellForProject(dirty, false, false)
	if !strings.Contains(got, "dirty") || !strings.Contains(got, "feature/x") {
		t.Fatalf("non-clean should show branch: %q", got)
	}

	clean := AggregateRow{Repo: RepoSnapshot{IsClean: true, Branch: "main"}}
	got = formatMatrixCellForProject(clean, false, false)
	if got != "clean" {
		t.Fatalf("matching clean should omit branch: %q", got)
	}

	got = formatMatrixCellForProject(clean, true, false)
	if !strings.Contains(got, "main") {
		t.Fatalf("branch mismatch should show branch on clean: %q", got)
	}

	stale := AggregateRow{Stale: true, Repo: RepoSnapshot{HasUnpushed: true, AheadCount: 2, Branch: "main"}}
	got = formatMatrixCellForProject(stale, false, false)
	if !strings.Contains(got, "stale") || !strings.Contains(got, "↑2") {
		t.Fatalf("stale unpushed: %q", got)
	}
}

func TestProjectTipsDiffer(t *testing.T) {
	same := map[string]AggregateRow{
		"laptop":  {Repo: RepoSnapshot{Branch: "main", HeadSHA: "aaa1111"}},
		"desktop": {Repo: RepoSnapshot{Branch: "main", HeadSHA: "aaa1111"}},
	}
	if projectTipsDiffer(same) {
		t.Fatal("matching tips should not differ")
	}

	abbrevLen := map[string]AggregateRow{
		"laptop":  {Repo: RepoSnapshot{Branch: "main", HeadSHA: "abcdef1"}},
		"desktop": {Repo: RepoSnapshot{Branch: "main", HeadSHA: "abcdef123456"}},
	}
	if projectTipsDiffer(abbrevLen) {
		t.Fatal("shared-prefix abbrevs of same tip should not differ")
	}

	diverged := map[string]AggregateRow{
		"laptop":  {Repo: RepoSnapshot{Branch: "main", HeadSHA: "aaa1111"}},
		"desktop": {Repo: RepoSnapshot{Branch: "main", HeadSHA: "bbb2222"}},
	}
	if !projectTipsDiffer(diverged) {
		t.Fatal("same branch different SHA should differ")
	}

	branchMismatch := map[string]AggregateRow{
		"laptop":  {Repo: RepoSnapshot{Branch: "main", HeadSHA: "aaa1111"}},
		"desktop": {Repo: RepoSnapshot{Branch: "feature", HeadSHA: "bbb2222"}},
	}
	if projectTipsDiffer(branchMismatch) {
		t.Fatal("different branches are not tip mismatch")
	}

	missingSHA := map[string]AggregateRow{
		"laptop":  {Repo: RepoSnapshot{Branch: "main", HeadSHA: "aaa1111"}},
		"desktop": {Repo: RepoSnapshot{Branch: "main"}},
	}
	if projectTipsDiffer(missingSHA) {
		t.Fatal("missing SHA should be ignored")
	}
}

func TestFormatMatrixCellTipMismatch(t *testing.T) {
	clean := AggregateRow{Repo: RepoSnapshot{IsClean: true, Branch: "main", HeadSHA: "aaa1111"}}
	got := formatMatrixCellForProject(clean, false, true)
	if got != "clean · tip≠aaa1111" {
		t.Fatalf("clean tip mismatch: %q", got)
	}

	dirty := AggregateRow{Repo: RepoSnapshot{IsDirty: true, Branch: "main", HeadSHA: "bbb2222"}}
	got = formatMatrixCellForProject(dirty, false, true)
	if !strings.Contains(got, "dirty") || !strings.Contains(got, "main") || !strings.Contains(got, "tip≠bbb2222") {
		t.Fatalf("dirty tip mismatch should keep branch+tip: %q", got)
	}

	noSHA := AggregateRow{Repo: RepoSnapshot{IsClean: true, Branch: "main"}}
	got = formatMatrixCellForProject(noSHA, false, true)
	if got != "clean · tip≠" {
		t.Fatalf("tip cue without SHA: %q", got)
	}
}

func TestDisplayProjectMachineMatrixTipMismatchVisible(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Origin: "github.com/you/app", Path: "/repos/app", Branch: "main",
			IsClean: true, HeadSHA: "aaa1111",
		}},
		{Machine: "desktop", Repo: RepoSnapshot{
			Origin: "github.com/you/app", Path: "/Users/you/app", Branch: "main",
			IsClean: true, HeadSHA: "bbb2222",
		}},
	}
	out := captureStdout(t, func() {
		displayProjectMachineMatrix(rows)
	})
	if !strings.Contains(out, "tip≠aaa1111") || !strings.Contains(out, "tip≠bbb2222") {
		t.Fatalf("diverged tips must appear in matrix cells: %s", out)
	}
	if strings.Contains(out, "Attention") {
		t.Fatalf("matrix-only view should not print Attention: %s", out)
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
