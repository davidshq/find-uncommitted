package main

import (
	"strings"
	"testing"
)

func collisionRows(localDirty bool, remote RepoSnapshot) []AggregateRow {
	local := RepoSnapshot{
		Path: "/laptop/app", Origin: "github.com/acme/app", Branch: "main",
		IsDirty: localDirty, HasUnstaged: localDirty, IsClean: !localDirty,
	}
	return []AggregateRow{
		{Machine: "laptop", Local: true, Repo: local},
		{Machine: "desktop", Repo: remote},
	}
}

func findSituation(situations []Situation, kind SituationKind) (Situation, bool) {
	for _, s := range situations {
		if s.Kind == kind {
			return s, true
		}
	}
	return Situation{}, false
}

// The collision the tool exists to prevent: both machines dirty in one project.
// This cue used to be suppressed whenever the local copy was dirty, so the user
// was told to stash their own work and never heard about the other machine.
func TestDirtyLocalStillReportsDirtyRemote(t *testing.T) {
	remote := RepoSnapshot{
		Path: "/desktop/app", Origin: "github.com/acme/app", Branch: "main",
		IsDirty: true, HasUnstaged: true,
	}
	situations := DetectSituations(collisionRows(true, remote))

	s, ok := findSituation(situations, SituationOtherMachineWork)
	if !ok {
		t.Fatalf("expected other_machine_work when both sides are dirty, got %+v", situations)
	}
	if !strings.Contains(s.Nudge, "BOTH sides") {
		t.Errorf("expected an explicit both-sides collision nudge, got %q", s.Nudge)
	}
	if !strings.Contains(s.Nudge, "desktop") {
		t.Errorf("nudge should name the other machine, got %q", s.Nudge)
	}
	if _, ok := findSituation(situations, SituationLocalDirty); !ok {
		t.Error("local dirty nudge should still be present alongside the collision")
	}
}

// A dirty local copy plus unpushed (not dirty) remote commits is not a
// working-tree collision, so it keeps the ordinary other-machine wording.
func TestDirtyLocalWithUnpushedRemoteUsesOrdinaryNudge(t *testing.T) {
	remote := RepoSnapshot{
		Path: "/desktop/app", Origin: "github.com/acme/app", Branch: "main",
		HasUnpushed: true, AheadCount: 2,
	}
	situations := DetectSituations(collisionRows(true, remote))

	s, ok := findSituation(situations, SituationOtherMachineWork)
	if !ok {
		t.Fatalf("expected other_machine_work, got %+v", situations)
	}
	if strings.Contains(s.Nudge, "BOTH sides") {
		t.Errorf("unpushed-only remote is not a working-tree collision: %q", s.Nudge)
	}
	if !strings.Contains(s.Nudge, "unpushed commits on desktop") {
		t.Errorf("expected unpushed wording, got %q", s.Nudge)
	}
}

// Clean local + dirty remote keeps its original behaviour.
func TestCleanLocalStillReportsOtherMachineWork(t *testing.T) {
	remote := RepoSnapshot{
		Path: "/desktop/app", Origin: "github.com/acme/app", Branch: "main",
		IsDirty: true, HasUnstaged: true,
	}
	situations := DetectSituations(collisionRows(false, remote))

	s, ok := findSituation(situations, SituationOtherMachineWork)
	if !ok {
		t.Fatalf("expected other_machine_work, got %+v", situations)
	}
	if !strings.HasPrefix(s.Nudge, "other machine has") {
		t.Errorf("expected the original wording for a clean local, got %q", s.Nudge)
	}
}

// A clean remote must not produce a cue at all, whatever the local state.
func TestDirtyLocalWithCleanRemoteEmitsNoCrossMachineCue(t *testing.T) {
	remote := RepoSnapshot{
		Path: "/desktop/app", Origin: "github.com/acme/app", Branch: "main", IsClean: true,
	}
	situations := DetectSituations(collisionRows(true, remote))
	if s, ok := findSituation(situations, SituationOtherMachineWork); ok {
		t.Fatalf("clean remote should emit no cross-machine cue, got %q", s.Nudge)
	}
}

// Linked worktrees and submodules share an origin, so they correlate into one
// group with several Local rows. Keeping only the last one made a dirty worktree
// beside a clean clone report "nothing needing attention" while the inventory
// showed it dirty — contradictory output that trains distrust.
func TestMultipleLocalWorkingTreesEachGetCues(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Path: "/w/main", Origin: "github.com/acme/app", Branch: "main", IsClean: true,
		}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Path: "/w/feature", Origin: "github.com/acme/app", Branch: "feature",
			IsDirty: true, HasUnstaged: true,
		}},
	}
	situations := DetectSituations(rows)

	s, ok := findSituation(situations, SituationLocalDirty)
	if !ok {
		t.Fatalf("dirty worktree must still raise a cue, got %+v", situations)
	}
	if !strings.Contains(s.Nudge, "/w/feature") {
		t.Errorf("nudge should name the working tree, got %q", s.Nudge)
	}
}

// Both trees dirty: two distinguishable cues, not one swallowed by the other.
func TestTwoDirtyLocalTreesBothReported(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Path: "/w/main", Origin: "github.com/acme/app", Branch: "main",
			IsDirty: true, HasUnstaged: true,
		}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Path: "/w/feature", Origin: "github.com/acme/app", Branch: "feature",
			IsDirty: true, HasUnstaged: true,
		}},
	}
	var dirty []string
	for _, s := range DetectSituations(rows) {
		if s.Kind == SituationLocalDirty {
			dirty = append(dirty, s.Nudge)
		}
	}
	if len(dirty) != 2 {
		t.Fatalf("expected one cue per dirty tree, got %d: %v", len(dirty), dirty)
	}
	if dirty[0] == dirty[1] {
		t.Errorf("cues must be distinguishable, both were %q", dirty[0])
	}
}

// A single local tree keeps the original wording: no path suffix.
func TestSingleLocalTreeHasNoPathSuffix(t *testing.T) {
	rows := []AggregateRow{{Machine: "laptop", Local: true, Repo: RepoSnapshot{
		Path: "/w/app", Origin: "github.com/acme/app", Branch: "main",
		IsDirty: true, HasUnstaged: true,
	}}}
	s, ok := findSituation(DetectSituations(rows), SituationLocalDirty)
	if !ok {
		t.Fatal("expected local dirty cue")
	}
	if s.Nudge != "commit or stash local changes before switching machines" {
		t.Errorf("single tree should keep original wording, got %q", s.Nudge)
	}
}

// A dirty worktree on this machine plus a dirty remote is still a collision.
func TestCollisionDetectedFromNonRepresentativeLocalTree(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Path: "/w/main", Origin: "github.com/acme/app", Branch: "main", IsClean: true,
		}},
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{
			Path: "/w/feature", Origin: "github.com/acme/app", Branch: "feature",
			IsDirty: true, HasUnstaged: true,
		}},
		{Machine: "desktop", Repo: RepoSnapshot{
			Path: "/d/app", Origin: "github.com/acme/app", Branch: "main",
			IsDirty: true, HasUnstaged: true,
		}},
	}
	s, ok := findSituation(DetectSituations(rows), SituationOtherMachineWork)
	if !ok {
		t.Fatalf("expected other_machine_work, got %+v", DetectSituations(rows))
	}
	if !strings.Contains(s.Nudge, "BOTH sides") {
		t.Errorf("dirt in any local tree is still a collision, got %q", s.Nudge)
	}
}
