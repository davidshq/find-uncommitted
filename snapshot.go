package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RepoSnapshot is the serializable form of a single repository's status.
// Origin is the normalized remote.origin.url used to correlate the same project
// across machines (empty when the repo has no origin remote).
// Newer fields (HasBehind, counts, HeadSHA) are omitempty so older snapshots
// without them still load and simply report no behind/SHA evidence.
type RepoSnapshot struct {
	Path                 string `json:"path"`
	Origin               string `json:"origin,omitempty"`
	Branch               string `json:"branch"`
	HasUnstaged          bool   `json:"has_unstaged"`
	HasStaged            bool   `json:"has_staged"`
	HasUntracked         bool   `json:"has_untracked"`
	HasUnpushed          bool   `json:"has_unpushed"`
	HasBehind            bool   `json:"has_behind,omitempty"`
	HasUntrackedUpstream bool   `json:"has_untracked_upstream"`
	AheadCount           int    `json:"ahead_count,omitempty"`
	BehindCount          int    `json:"behind_count,omitempty"`
	HeadSHA              string `json:"head_sha,omitempty"`
	IsDirty              bool   `json:"is_dirty"`
	IsClean              bool   `json:"is_clean"`
	IsEmpty              bool   `json:"is_empty,omitempty"`
	Error                string `json:"error,omitempty"`
}

// ScanMetadata captures high-level stats from one publish tick.
type ScanMetadata struct {
	RepoCount  int    `json:"repo_count"`
	DirtyCount int    `json:"dirty_count"`
	ErrorCount int    `json:"error_count"`
	DurationMs int64  `json:"duration_ms"`
	ScanRoot   string `json:"scan_root"`
}

// MachineSnapshot is one machine's published state file content.
type MachineSnapshot struct {
	MachineID string         `json:"machine_id"`
	UpdatedAt time.Time      `json:"updated_at"`
	Repos     []RepoSnapshot `json:"repos"`
	Meta      ScanMetadata   `json:"meta"`
}

// LoadedSnapshot is a parsed machine snapshot plus load-time annotations.
type LoadedSnapshot struct {
	Snapshot MachineSnapshot
	FilePath string
	Stale    bool
	LoadErr  string
}

const machinesDirName = "machines"

// SnapshotFilePath returns the per-machine JSON path inside a state repo.
// Filenames include a short hash of the raw machine id to avoid collisions
// after character sanitization (e.g. "a/b" vs "a_b").
func SnapshotFilePath(stateRepoDir, machineID string) string {
	return filepath.Join(stateRepoDir, machinesDirName, snapshotFileName(machineID))
}

func snapshotFileName(machineID string) string {
	safe := sanitizeMachineID(machineID)
	sum := sha256.Sum256([]byte(machineID))
	return fmt.Sprintf("%s-%x.json", safe, sum[:4])
}

func sanitizeMachineID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(id)
}

// RepoStatusToSnapshot converts a live scan result into snapshot form.
// When redactPaths is set, filesystem paths are basename-only and origin URLs
// are replaced with a stable hash so cross-machine correlation still works.
func RepoStatusToSnapshot(status RepoStatus, redactPaths bool) RepoSnapshot {
	path := status.Path
	origin := status.Origin
	if redactPaths {
		path = redactPath(path)
		origin = redactOrigin(origin)
	}
	return RepoSnapshot{
		Path:                 path,
		Origin:               origin,
		Branch:               status.Branch,
		HasUnstaged:          status.HasUnstaged,
		HasStaged:            status.HasStaged,
		HasUntracked:         status.HasUntracked,
		HasUnpushed:          status.HasUnpushed,
		HasBehind:            status.HasBehind,
		HasUntrackedUpstream: status.HasUntrackedUpstream,
		AheadCount:           status.AheadCount,
		BehindCount:          status.BehindCount,
		HeadSHA:              status.HeadSHA,
		IsDirty:              status.IsDirty,
		IsClean:              status.IsClean,
		IsEmpty:              status.IsEmpty,
		Error:                status.Error,
	}
}

// BuildMachineSnapshot creates a snapshot from scan results.
// Repos are sorted by path so concurrent scans don't churn commits.
func BuildMachineSnapshot(machineID, scanRoot string, results []RepoStatus, started time.Time, redactPaths bool) MachineSnapshot {
	repos := make([]RepoSnapshot, 0, len(results))
	dirty := 0
	errs := 0
	for _, r := range results {
		repos = append(repos, RepoStatusToSnapshot(r, redactPaths))
		if r.Error != "" {
			errs++
		} else if r.IsDirty {
			dirty++
		}
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Path < repos[j].Path
	})
	root := scanRoot
	if redactPaths {
		root = redactPath(scanRoot)
	}
	return MachineSnapshot{
		MachineID: machineID,
		UpdatedAt: time.Now().UTC(),
		Repos:     repos,
		Meta: ScanMetadata{
			RepoCount:  len(results),
			DirtyCount: dirty,
			ErrorCount: errs,
			DurationMs: time.Since(started).Milliseconds(),
			ScanRoot:   root,
		},
	}
}

func redactPath(path string) string {
	base := filepath.Base(path)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "[redacted]"
	}
	return filepath.Join("…", base)
}

// WriteMachineSnapshot writes the machine snapshot JSON (pretty-printed).
func WriteMachineSnapshot(path string, snap MachineSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// ReadMachineSnapshot loads and validates a single machine snapshot file.
func ReadMachineSnapshot(path string) (MachineSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MachineSnapshot{}, err
	}
	var snap MachineSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return MachineSnapshot{}, fmt.Errorf("parse snapshot %s: %w", path, err)
	}
	if strings.TrimSpace(snap.MachineID) == "" {
		return MachineSnapshot{}, fmt.Errorf("snapshot %s missing machine_id", path)
	}
	if snap.UpdatedAt.IsZero() {
		return MachineSnapshot{}, fmt.Errorf("snapshot %s missing updated_at", path)
	}
	if snap.Repos == nil {
		snap.Repos = []RepoSnapshot{}
	}
	return snap, nil
}

// LoadAllMachineSnapshots reads every *.json under machines/ in the state repo.
// Invalid files are skipped for aggregate merge but returned with LoadErr set
// so callers can warn without blanking the rest of the aggregate.
func LoadAllMachineSnapshots(stateRepoDir string, staleTTL time.Duration, now time.Time) ([]LoadedSnapshot, error) {
	dir := filepath.Join(stateRepoDir, machinesDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read machines directory: %w", err)
	}

	var loaded []LoadedSnapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		snap, err := ReadMachineSnapshot(path)
		if err != nil {
			loaded = append(loaded, LoadedSnapshot{
				FilePath: path,
				LoadErr:  err.Error(),
			})
			continue
		}
		item := LoadedSnapshot{
			Snapshot: snap,
			FilePath: path,
		}
		if staleTTL > 0 && now.Sub(snap.UpdatedAt) > staleTTL {
			item.Stale = true
		}
		loaded = append(loaded, item)
	}
	return loaded, nil
}

// warnCorruptSnapshots prints a stderr warning for each snapshot that failed to parse.
// One corrupt file must not blank the aggregate; callers still load valid siblings.
func warnCorruptSnapshots(remote []LoadedSnapshot) {
	for _, item := range remote {
		if item.LoadErr == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "warning: skipping corrupt snapshot %s: %s\n", item.FilePath, item.LoadErr)
	}
}

// SnapshotContentEqual compares JSON content for change detection (ignores formatting).
func SnapshotContentEqual(a, b MachineSnapshot) bool {
	aj, errA := json.Marshal(normalizeForCompare(a))
	bj, errB := json.Marshal(normalizeForCompare(b))
	if errA != nil || errB != nil {
		return false
	}
	return string(aj) == string(bj)
}

func normalizeForCompare(snap MachineSnapshot) MachineSnapshot {
	// Exclude UpdatedAt so churn timestamps alone do not force commits when
	// repository status is unchanged. Meta.DurationMs is likewise noisy.
	out := snap
	out.UpdatedAt = time.Time{}
	out.Meta.DurationMs = 0
	if out.Repos == nil {
		out.Repos = []RepoSnapshot{}
	}
	return out
}
