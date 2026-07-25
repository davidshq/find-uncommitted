package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadMachineSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := SnapshotFilePath(dir, "desk/top")
	snap := MachineSnapshot{
		MachineID: "desk/top",
		UpdatedAt: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
		Repos: []RepoSnapshot{
			{Path: "/code/a", Branch: "main", IsDirty: true, HasUnstaged: true},
		},
		Meta: ScanMetadata{RepoCount: 1, DirtyCount: 1, ScanRoot: "/code"},
	}
	if err := WriteMachineSnapshot(path, snap); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadMachineSnapshot(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.MachineID != snap.MachineID {
		t.Fatalf("machine id: got %q", got.MachineID)
	}
	if len(got.Repos) != 1 || !got.Repos[0].IsDirty {
		t.Fatalf("repos: %+v", got.Repos)
	}
	if filepath.Base(path) != snapshotFileName("desk/top") {
		t.Fatalf("expected hashed sanitized filename, got %s", path)
	}
}

func TestSnapshotFileNameAvoidsSanitizationCollision(t *testing.T) {
	a := snapshotFileName("a/b")
	b := snapshotFileName("a_b")
	if a == b {
		t.Fatalf("expected distinct filenames, both %s", a)
	}
}

func TestLoadAllMachineSnapshotsStaleAndInvalid(t *testing.T) {
	dir := t.TempDir()
	machines := filepath.Join(dir, machinesDirName)
	if err := os.MkdirAll(machines, 0o755); err != nil {
		t.Fatal(err)
	}

	fresh := MachineSnapshot{
		MachineID: "fresh",
		UpdatedAt: time.Now().UTC(),
		Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
	}
	stale := MachineSnapshot{
		MachineID: "stale-box",
		UpdatedAt: time.Now().UTC().Add(-10 * time.Minute),
		Repos:     []RepoSnapshot{{Path: "/b", Branch: "dev", IsDirty: true, HasStaged: true}},
	}
	if err := WriteMachineSnapshot(SnapshotFilePath(dir, "fresh"), fresh); err != nil {
		t.Fatal(err)
	}
	if err := WriteMachineSnapshot(SnapshotFilePath(dir, "stale-box"), stale); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(machines, "bad.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadAllMachineSnapshots(dir, 5*time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded))
	}

	byID := map[string]LoadedSnapshot{}
	var badCount int
	for _, item := range loaded {
		if item.LoadErr != "" {
			badCount++
			continue
		}
		byID[item.Snapshot.MachineID] = item
	}
	if badCount != 1 {
		t.Fatalf("expected 1 bad file, got %d", badCount)
	}
	if byID["fresh"].Stale {
		t.Fatal("fresh should not be stale")
	}
	if !byID["stale-box"].Stale {
		t.Fatal("stale-box should be stale")
	}
}

func TestWarnCorruptSnapshots(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	remote := []LoadedSnapshot{
		{FilePath: badPath, LoadErr: "parse snapshot: invalid"},
		{
			Snapshot: MachineSnapshot{
				MachineID: "ok",
				UpdatedAt: time.Now().UTC(),
				Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
			},
			FilePath: filepath.Join(dir, "ok.json"),
		},
	}
	warnCorruptSnapshots(remote)
	rows := BuildAggregateRows("local", nil, remote)
	var sawErr, sawOK bool
	for _, r := range rows {
		if r.LoadError != "" {
			sawErr = true
		}
		if r.Machine == "ok" && r.Repo.Path == "/a" {
			sawOK = true
		}
	}
	if !sawErr || !sawOK {
		t.Fatalf("expected corrupt skipped with warning path and valid sibling kept: %+v", rows)
	}
}

func TestBuildAggregateRowsPreservesLoadError(t *testing.T) {
	remote := []LoadedSnapshot{
		{FilePath: "/state/machines/broken.json", LoadErr: "parse failed"},
		{
			Snapshot: MachineSnapshot{
				MachineID: "other",
				UpdatedAt: time.Now().UTC(),
				Repos:     []RepoSnapshot{{Path: "/r", Branch: "main", IsClean: true}},
			},
		},
	}
	rows := BuildAggregateRows("me", nil, remote)
	if len(rows) != 2 {
		t.Fatalf("expected error row + other repo, got %d: %+v", len(rows), rows)
	}
	var sawErr bool
	for _, r := range rows {
		if r.LoadError == "parse failed" {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatalf("LoadError not preserved: %+v", rows)
	}
}

func TestBuildAggregateRowsSkipsLocalDuplicate(t *testing.T) {
	local := []RepoStatus{{Path: "/local/a", Branch: "main", IsDirty: true, HasUnstaged: true}}
	remote := []LoadedSnapshot{
		{
			Snapshot: MachineSnapshot{
				MachineID: "this-machine",
				UpdatedAt: time.Now().UTC(),
				Repos:     []RepoSnapshot{{Path: "/local/a", Branch: "main", IsDirty: true}},
			},
		},
		{
			Snapshot: MachineSnapshot{
				MachineID: "other",
				UpdatedAt: time.Now().UTC().Add(-time.Hour),
				Repos:     []RepoSnapshot{{Path: "/other/b", Branch: "feat", IsClean: true}},
			},
			Stale: true,
		},
	}
	rows := BuildAggregateRows("this-machine", local, remote)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (local + other), got %d: %+v", len(rows), rows)
	}
	var sawOther, sawLocal bool
	for _, r := range rows {
		if r.Machine == "other" && r.Stale {
			sawOther = true
		}
		if r.Local && r.Machine == "this-machine" {
			sawLocal = true
		}
	}
	if !sawOther || !sawLocal {
		t.Fatalf("missing expected rows: %+v", rows)
	}
}

func TestBuildAggregateRowsSortsByOrigin(t *testing.T) {
	local := []RepoStatus{
		{Path: "/laptop/other", Origin: "github.com/acme/other", Branch: "main", IsClean: true},
		{Path: "/laptop/app", Origin: "github.com/acme/app", Branch: "feat", IsDirty: true, HasUnstaged: true},
	}
	remote := []LoadedSnapshot{
		{
			Snapshot: MachineSnapshot{
				MachineID: "desktop",
				UpdatedAt: time.Now().UTC(),
				Repos: []RepoSnapshot{
					{Path: "/desktop/app", Origin: "github.com/acme/app", Branch: "main", IsClean: true},
				},
			},
		},
	}
	rows := BuildAggregateRows("laptop", local, remote)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(rows), rows)
	}
	// Same origin should be adjacent regardless of machine/path.
	if rows[0].Repo.Origin != "github.com/acme/app" || rows[1].Repo.Origin != "github.com/acme/app" {
		t.Fatalf("expected app origin rows first (correlated), got %+v", rows)
	}
	if rows[0].Machine == rows[1].Machine {
		t.Fatalf("expected app rows from different machines: %+v", rows)
	}
	if rows[2].Repo.Origin != "github.com/acme/other" {
		t.Fatalf("expected other last, got %+v", rows)
	}
}

func TestSnapshotContentEqualIgnoresTimestamps(t *testing.T) {
	a := MachineSnapshot{
		MachineID: "m",
		UpdatedAt: time.Now(),
		Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
		Meta:      ScanMetadata{DurationMs: 10, RepoCount: 1, ScanRoot: "/"},
	}
	b := MachineSnapshot{
		MachineID: a.MachineID,
		UpdatedAt: a.UpdatedAt.Add(time.Minute),
		Repos:     []RepoSnapshot{{Path: "/a", Branch: "main", IsClean: true}},
		Meta:      ScanMetadata{DurationMs: 99, RepoCount: 1, ScanRoot: "/"},
	}
	if !SnapshotContentEqual(a, b) {
		t.Fatal("expected equal ignoring timestamps/duration")
	}
	b.Repos[0].IsDirty = true
	b.Repos[0].IsClean = false
	if SnapshotContentEqual(a, b) {
		t.Fatal("expected inequality when dirty status changes")
	}
}

func TestRedactPaths(t *testing.T) {
	status := RepoStatus{Path: filepath.Join("C:", "Users", "me", "proj"), Branch: "main", IsClean: true}
	snap := RepoStatusToSnapshot(status, true)
	if snap.Path == status.Path {
		t.Fatalf("expected redacted path, got %q", snap.Path)
	}
	if filepath.Base(snap.Path) != "proj" && filepath.Base(snap.Path) != "…" {
		// basename should be preserved in common case
		if !containsBasename(snap.Path, "proj") {
			t.Fatalf("redacted path should keep basename: %q", snap.Path)
		}
	}
}

func containsBasename(path, base string) bool {
	return filepath.Base(path) == base || path == filepath.Join("…", base)
}
