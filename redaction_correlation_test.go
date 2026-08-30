package main

import (
	"strings"
	"testing"
	"time"
)

// A machine publishing with --redact-paths must still correlate with a machine
// publishing plain origins. Comparing a plain URL against a hash never matched,
// which silently split one project into two unrelated groups.
func TestRedactedOriginCorrelatesWithPlainOrigin(t *testing.T) {
	plain := RepoStatusToSnapshot(RepoStatus{Path: "/laptop/app", Origin: "github.com/acme/app"}, false)
	redacted := RepoStatusToSnapshot(RepoStatus{Path: "/desktop/app", Origin: "github.com/acme/app"}, true)

	if got, want := repoCorrelationKey(redacted), repoCorrelationKey(plain); got != want {
		t.Fatalf("redacted key %q != plain key %q", got, want)
	}
	if !strings.HasPrefix(redacted.Origin, redactedOriginPrefix) {
		t.Fatalf("expected redacted origin to be hashed, got %q", redacted.Origin)
	}
	if redacted.Origin == "github.com/acme/app" {
		t.Fatal("redaction must not publish the plain origin URL")
	}
}

// Different projects must not collide just because both sides are hashed.
func TestRedactedOriginKeepsDistinctProjectsApart(t *testing.T) {
	a := RepoStatusToSnapshot(RepoStatus{Path: "/m/app", Origin: "github.com/acme/app"}, true)
	b := RepoStatusToSnapshot(RepoStatus{Path: "/m/other", Origin: "github.com/acme/other"}, true)
	if repoCorrelationKey(a) == repoCorrelationKey(b) {
		t.Fatal("distinct origins must not share a correlation key")
	}
}

// End-to-end: a redacting remote and a plain local land in one group, and the
// group adopts the legible label rather than the bare hash.
func TestAggregateGroupsRedactedRemoteWithPlainLocal(t *testing.T) {
	local := []RepoStatus{{Path: "/laptop/app", Origin: "github.com/acme/app", Branch: "main", IsClean: true}}
	remote := []LoadedSnapshot{{
		Snapshot: MachineSnapshot{
			MachineID: "desktop",
			UpdatedAt: time.Now().UTC(),
			Repos: []RepoSnapshot{
				RepoStatusToSnapshot(RepoStatus{
					Path: "/desktop/app", Origin: "github.com/acme/app",
					Branch: "main", IsDirty: true, HasUnstaged: true,
				}, true),
			},
		},
	}}

	groups := GroupRowsByProject(BuildAggregateRows("laptop", local, remote))
	if len(groups) != 1 {
		t.Fatalf("expected 1 correlated group, got %d: %+v", len(groups), groups)
	}
	if len(groups[0].Rows) != 2 {
		t.Fatalf("expected 2 rows in the group, got %d", len(groups[0].Rows))
	}
	if groups[0].Label != "github.com/acme/app" {
		t.Fatalf("group should adopt the legible label, got %q", groups[0].Label)
	}

	// The cross-machine cue must actually fire across the redaction boundary.
	situations := DetectSituations(BuildAggregateRows("laptop", local, remote))
	var found bool
	for _, s := range situations {
		if s.Kind == SituationOtherMachineWork {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected other_machine_work across redaction boundary, got %+v", situations)
	}
}

func TestProjectLabelRankPrefersLegible(t *testing.T) {
	if projectLabelRank("github.com/acme/app") >= projectLabelRank("app (redacted origin)") {
		t.Fatal("plain origin should outrank a redacted-basename label")
	}
	if projectLabelRank("app (redacted origin)") >= projectLabelRank("redacted:deadbeef") {
		t.Fatal("redacted-basename label should outrank a bare hash")
	}
}
