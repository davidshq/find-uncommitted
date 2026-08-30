package main

import (
	"strings"
	"testing"
)

func TestDetectSituationsLocalDirtyUnpushedBehind(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "laptop",
			Local:   true,
			Repo: RepoSnapshot{
				Path:        "/code/app",
				Origin:      "github.com/acme/app",
				Branch:      "main",
				IsDirty:     true,
				HasUnstaged: true,
				HasUnpushed: true,
				HasBehind:   true,
				AheadCount:  2,
				BehindCount: 3,
			},
		},
	}
	got := DetectSituations(rows)
	kinds := map[SituationKind]Situation{}
	for _, s := range got {
		kinds[s.Kind] = s
	}
	if _, ok := kinds[SituationLocalDirty]; !ok {
		t.Fatalf("missing local_dirty: %+v", got)
	}
	if s, ok := kinds[SituationLocalUnpushed]; !ok || !strings.Contains(s.Nudge, "2") {
		t.Fatalf("missing/bad local_unpushed: %+v", got)
	}
	if s, ok := kinds[SituationLocalBehind]; !ok || !strings.Contains(s.Nudge, "3") {
		t.Fatalf("missing/bad local_behind: %+v", got)
	}
}

func TestDetectSituationsLocalErrorAndUntrackedUpstream(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "laptop",
			Local:   true,
			Repo: RepoSnapshot{
				Path:                 "/code/a",
				Origin:               "github.com/acme/a",
				Branch:               "main",
				Error:                "dubious ownership",
				HasUntrackedUpstream: true, // ignored when Error set for dirty/unpushed, but error cue wins
			},
		},
		{
			Machine: "laptop",
			Local:   true,
			Repo: RepoSnapshot{
				Path:                 "/code/b",
				Origin:               "github.com/acme/b",
				Branch:               "main",
				HasUntrackedUpstream: true,
				IsClean:              false,
			},
		},
	}
	// Two locals with different origins → two groups; BuildAggregateRows wouldn't
	// put two Local=true on same origin. Pass as separate detection via full list.
	got := DetectSituations(rows)
	kinds := map[SituationKind]bool{}
	for _, s := range got {
		kinds[s.Kind] = true
	}
	if !kinds[SituationLocalError] {
		t.Fatalf("expected local_error: %+v", got)
	}
	if !kinds[SituationLocalUntrackedUpstream] {
		t.Fatalf("expected local_untracked_upstream: %+v", got)
	}
}

func TestDetectSituationsBranchMismatchAndOtherMachineWork(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "laptop",
			Local:   true,
			Repo: RepoSnapshot{
				Path:    "/laptop/app",
				Origin:  "github.com/acme/app",
				Branch:  "main",
				IsClean: true,
			},
		},
		{
			Machine: "desktop",
			Repo: RepoSnapshot{
				Path:        "/desktop/app",
				Origin:      "github.com/acme/app",
				Branch:      "feat/x",
				IsDirty:     true,
				HasUnstaged: true,
			},
			Stale: true,
		},
	}
	got := DetectSituations(rows)
	kinds := map[SituationKind]bool{}
	var branch, other Situation
	for _, s := range got {
		kinds[s.Kind] = true
		if s.Kind == SituationBranchMismatch {
			branch = s
		}
		if s.Kind == SituationOtherMachineWork {
			other = s
		}
	}
	if !kinds[SituationBranchMismatch] {
		t.Fatalf("expected branch mismatch: %+v", got)
	}
	if !strings.Contains(branch.Nudge, "feat/x") || !branch.Stale {
		t.Fatalf("bad branch mismatch nudge: %+v", branch)
	}
	if !kinds[SituationOtherMachineWork] {
		t.Fatalf("expected other_machine_work: %+v", got)
	}
	if !strings.Contains(other.Nudge, "uncommitted") || !strings.Contains(other.Nudge, "pull will not") {
		t.Fatalf("expected dirty-specific other-machine nudge: %+v", other)
	}
	if !other.Stale || !strings.Contains(other.Nudge, "stale") {
		t.Fatalf("expected stale-qualified other-machine nudge: %+v", other)
	}
	if kinds[SituationStaleEvidence] {
		t.Fatalf("did not expect standalone stale when stronger cues exist: %+v", got)
	}
}

func TestDetectSituationsOtherMachineUnpushedOnly(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "laptop",
			Local:   true,
			Repo:    RepoSnapshot{Path: "/l/app", Origin: "github.com/acme/app", Branch: "main", IsClean: true},
		},
		{
			Machine: "desktop",
			Repo: RepoSnapshot{
				Path:        "/d/app",
				Origin:      "github.com/acme/app",
				Branch:      "main",
				HasUnpushed: true,
				AheadCount:  2,
			},
		},
	}
	got := DetectSituations(rows)
	var other Situation
	for _, s := range got {
		if s.Kind == SituationOtherMachineWork {
			other = s
		}
	}
	if other.Kind == "" {
		t.Fatalf("expected other_machine_work: %+v", got)
	}
	if !strings.Contains(other.Nudge, "unpushed") || strings.Contains(other.Nudge, "pull will not") {
		t.Fatalf("expected unpushed-specific nudge: %+v", other)
	}
}

func TestDetectSituationsTipMismatch(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "laptop",
			Local:   true,
			Repo: RepoSnapshot{
				Path:    "/l/app",
				Origin:  "github.com/acme/app",
				Branch:  "main",
				HeadSHA: "aaa1111",
				IsClean: true,
			},
		},
		{
			Machine: "desktop",
			Repo: RepoSnapshot{
				Path:    "/d/app",
				Origin:  "github.com/acme/app",
				Branch:  "main",
				HeadSHA: "bbb2222",
				IsClean: true,
			},
		},
	}
	got := DetectSituations(rows)
	var tip Situation
	for _, s := range got {
		if s.Kind == SituationTipMismatch {
			tip = s
		}
	}
	if tip.Kind == "" || !strings.Contains(tip.Nudge, "aaa1111") || !strings.Contains(tip.Nudge, "bbb2222") {
		t.Fatalf("expected tip mismatch: %+v", got)
	}
}

func TestDetectSituationsStaleEvidenceWhenRemoteNeedsAttention(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "desktop",
			Stale:   true,
			Repo: RepoSnapshot{
				Path:        "/desktop/notes",
				Origin:      "github.com/acme/notes",
				Branch:      "main",
				IsDirty:     true,
				HasUnstaged: true,
			},
		},
	}
	got := DetectSituations(rows)
	var sawStale bool
	for _, s := range got {
		if s.Kind == SituationStaleEvidence {
			sawStale = true
		}
	}
	if !sawStale {
		t.Fatalf("expected stale_evidence for remote-only dirty stale snapshot: %+v", got)
	}
}

func TestDetectSituationsNoStaleForCleanRemote(t *testing.T) {
	rows := []AggregateRow{
		{
			Machine: "laptop",
			Local:   true,
			Repo: RepoSnapshot{
				Path:    "/laptop/app",
				Origin:  "github.com/acme/app",
				Branch:  "main",
				IsClean: true,
			},
		},
		{
			Machine: "desktop",
			Stale:   true,
			Repo: RepoSnapshot{
				Path:    "/desktop/app",
				Origin:  "github.com/acme/app",
				Branch:  "main",
				IsClean: true,
			},
		},
	}
	got := DetectSituations(rows)
	for _, s := range got {
		if s.Kind == SituationStaleEvidence {
			t.Fatalf("clean stale remote should not emit stale_evidence: %+v", got)
		}
	}
}

func TestFilterRowsByProjectKeys(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "a", Local: true, Repo: RepoSnapshot{Path: "/a/app", Origin: "github.com/acme/app", Branch: "main"}},
		{Machine: "b", Repo: RepoSnapshot{Path: "/b/other", Origin: "github.com/acme/other", Branch: "main"}},
		{LoadError: "bad", Machine: "broken.json"},
	}
	// Derive the key rather than hardcoding its shape: it is an opaque hash.
	keys := map[string]bool{
		repoCorrelationKey(RepoSnapshot{Path: "/a/app", Origin: "github.com/acme/app"}): true,
	}
	got := FilterRowsByProjectKeys(rows, keys)
	if len(got) != 2 {
		t.Fatalf("expected app row + load error, got %d: %+v", len(got), got)
	}

	onlyErrs := FilterRowsByProjectKeys(rows, nil)
	if len(onlyErrs) != 1 || onlyErrs[0].LoadError == "" {
		t.Fatalf("empty keys should retain only load errors: %+v", onlyErrs)
	}
}

func TestGroupRowsByProject(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{Path: "/l/app", Origin: "github.com/acme/app"}},
		{Machine: "desktop", Repo: RepoSnapshot{Path: "/d/app", Origin: "github.com/acme/app"}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{Path: "/l/book", Branch: "main"}},
	}
	groups := GroupRowsByProject(rows)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(groups), groups)
	}
}

func TestRepoStatusToSnapshotIncludesBehindAndSHA(t *testing.T) {
	status := RepoStatus{
		Path:        "/code/app",
		Origin:      "github.com/acme/app",
		Branch:      "main",
		HasBehind:   true,
		BehindCount: 4,
		HasUnpushed: true,
		AheadCount:  1,
		HeadSHA:     "abc1234",
		IsClean:     false,
	}
	snap := RepoStatusToSnapshot(status, false)
	if !snap.HasBehind || snap.BehindCount != 4 || snap.AheadCount != 1 || snap.HeadSHA != "abc1234" {
		t.Fatalf("snapshot missing behind/SHA fields: %+v", snap)
	}
}

func TestSnapshotNeedsAttentionIncludesBehind(t *testing.T) {
	if !snapshotNeedsAttention(RepoSnapshot{HasBehind: true, BehindCount: 1}) {
		t.Fatal("behind should need attention")
	}
	if !repoNeedsAttention(RepoStatus{HasBehind: true}) {
		t.Fatal("behind should need attention")
	}
}

func TestOldSnapshotWithoutBehindLoadsClean(t *testing.T) {
	old := RepoSnapshot{Path: "/a", Branch: "main", IsClean: true}
	if snapshotNeedsAttention(old) {
		t.Fatal("legacy clean snapshot should not need attention")
	}
	rows := []AggregateRow{{Machine: "m", Local: true, Repo: old}}
	if sit := DetectSituations(rows); len(sit) != 0 {
		t.Fatalf("legacy clean should yield no situations: %+v", sit)
	}
}

func TestDetectLocalSituationsUsesMachineID(t *testing.T) {
	got := DetectLocalSituations("my-laptop", []RepoStatus{
		{Path: "/a", Branch: "main", IsDirty: true, HasUnstaged: true},
	})
	if len(got) == 0 || got[0].Machines[0] != "my-laptop" {
		t.Fatalf("expected machine id in situation: %+v", got)
	}
}
