package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// SituationKind identifies a cross-machine or local awareness cue.
// Nudges suggest verbs only; the tool never runs git mutations on user repos.
type SituationKind string

const (
	SituationLocalError              SituationKind = "local_error"
	SituationLocalDirty              SituationKind = "local_dirty"
	SituationLocalUnpushed           SituationKind = "local_unpushed"
	SituationLocalBehind             SituationKind = "local_behind"
	SituationLocalUntrackedUpstream  SituationKind = "local_untracked_upstream"
	SituationBranchMismatch          SituationKind = "branch_mismatch"
	SituationTipMismatch             SituationKind = "tip_mismatch"
	SituationOtherMachineWork        SituationKind = "other_machine_work"
	SituationStaleEvidence           SituationKind = "stale_evidence"
)

// situationPriority orders Attention output (lower = higher priority).
func situationPriority(k SituationKind) int {
	switch k {
	case SituationLocalError:
		return 1
	case SituationLocalDirty:
		return 2
	case SituationLocalUnpushed:
		return 3
	case SituationLocalBehind:
		return 4
	case SituationLocalUntrackedUpstream:
		return 5
	case SituationBranchMismatch:
		return 6
	case SituationTipMismatch:
		return 7
	case SituationOtherMachineWork:
		return 8
	case SituationStaleEvidence:
		return 9
	default:
		return 99
	}
}

// Situation is one actionable awareness cue for a correlated project.
type Situation struct {
	Kind         SituationKind
	ProjectKey   string
	ProjectLabel string
	Nudge        string
	Machines     []string
	Stale        bool // true when remote evidence behind the nudge may be stale
}

// ProjectGroup collects aggregate rows that share a correlation key.
type ProjectGroup struct {
	Key   string
	Label string
	Rows  []AggregateRow
}

// GroupRowsByProject clusters aggregate rows by origin/basename identity.
func GroupRowsByProject(rows []AggregateRow) []ProjectGroup {
	index := map[string]int{}
	var groups []ProjectGroup
	for _, row := range rows {
		if row.LoadError != "" {
			continue
		}
		key := repoCorrelationKey(row.Repo)
		label := projectLabel(row.Repo)
		if i, ok := index[key]; ok {
			groups[i].Rows = append(groups[i].Rows, row)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, ProjectGroup{
			Key:   key,
			Label: label,
			Rows:  []AggregateRow{row},
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Label < groups[j].Label
	})
	return groups
}

func projectLabel(repo RepoSnapshot) string {
	if o := strings.TrimSpace(repo.Origin); o != "" {
		if strings.HasPrefix(o, "redacted:") {
			base := filepath.Base(repo.Path)
			if base != "" && base != "." && base != "…" {
				return base + " (redacted origin)"
			}
			return o
		}
		return o
	}
	base := filepath.Base(repo.Path)
	parent := filepath.Base(filepath.Dir(repo.Path))
	if base == "" || base == "." || base == string(filepath.Separator) || base == "…" {
		return repo.Path
	}
	if parent != "" && parent != "." && parent != string(filepath.Separator) && parent != "…" {
		return parent + "/" + base
	}
	return base
}

// DetectSituations builds soft-advice cues from project-grouped aggregate rows.
// Advice is text-only; callers must not execute suggested git commands.
func DetectSituations(rows []AggregateRow) []Situation {
	groups := GroupRowsByProject(rows)
	var out []Situation
	for _, g := range groups {
		out = append(out, detectGroupSituations(g)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := situationPriority(out[i].Kind), situationPriority(out[j].Kind)
		if pi != pj {
			return pi < pj
		}
		if out[i].ProjectLabel != out[j].ProjectLabel {
			return out[i].ProjectLabel < out[j].ProjectLabel
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func detectGroupSituations(g ProjectGroup) []Situation {
	var local *AggregateRow
	var remote []AggregateRow
	for i := range g.Rows {
		row := g.Rows[i]
		if row.Local {
			local = &g.Rows[i]
		} else {
			remote = append(remote, row)
		}
	}

	var situations []Situation
	anyRemoteStale := false
	for _, r := range remote {
		if r.Stale {
			anyRemoteStale = true
			break
		}
	}

	if local != nil {
		repo := local.Repo
		if repo.Error != "" {
			situations = append(situations, Situation{
				Kind:         SituationLocalError,
				ProjectKey:   g.Key,
				ProjectLabel: g.Label,
				Nudge:        "fix local git error: " + repo.Error,
				Machines:     []string{local.Machine},
			})
		}
		if repo.Error == "" && repo.IsDirty {
			situations = append(situations, Situation{
				Kind:         SituationLocalDirty,
				ProjectKey:   g.Key,
				ProjectLabel: g.Label,
				Nudge:        "commit or stash local changes before switching machines",
				Machines:     []string{local.Machine},
			})
		}
		if repo.Error == "" && repo.HasUnpushed {
			nudge := "push local commits so other machines can pull"
			if repo.AheadCount > 0 {
				nudge = fmt.Sprintf("push %d local commit(s) so other machines can pull", repo.AheadCount)
			}
			situations = append(situations, Situation{
				Kind:         SituationLocalUnpushed,
				ProjectKey:   g.Key,
				ProjectLabel: g.Label,
				Nudge:        nudge,
				Machines:     []string{local.Machine},
			})
		}
		if repo.Error == "" && repo.HasBehind {
			nudge := "pull before continuing (local branch is behind upstream)"
			if repo.BehindCount > 0 {
				nudge = fmt.Sprintf("pull before continuing (behind upstream by %d commit(s))", repo.BehindCount)
			}
			situations = append(situations, Situation{
				Kind:         SituationLocalBehind,
				ProjectKey:   g.Key,
				ProjectLabel: g.Label,
				Nudge:        nudge,
				Machines:     []string{local.Machine},
			})
		}
		if repo.Error == "" && repo.HasUntrackedUpstream {
			situations = append(situations, Situation{
				Kind:         SituationLocalUntrackedUpstream,
				ProjectKey:   g.Key,
				ProjectLabel: g.Label,
				Nudge:        "set upstream tracking (or push -u) so ahead/behind status is meaningful",
				Machines:     []string{local.Machine},
			})
		}
	}

	branchMismatch := false
	if local != nil && len(remote) > 0 {
		localBranch := strings.TrimSpace(local.Repo.Branch)
		var mismatchParts []string
		var mismatchStale bool
		for _, r := range remote {
			other := strings.TrimSpace(r.Repo.Branch)
			if localBranch == "" || other == "" {
				continue
			}
			if other != localBranch {
				mismatchParts = append(mismatchParts, fmt.Sprintf("%s on %s", other, formatMachineLabel(r)))
				if r.Stale {
					mismatchStale = true
				}
			}
		}
		if len(mismatchParts) > 0 {
			branchMismatch = true
			nudge := fmt.Sprintf("branch differs from local %q — other machine(s): %s", localBranch, strings.Join(mismatchParts, ", "))
			if mismatchStale {
				nudge += " (some snapshots stale — verify before switching)"
			}
			situations = append(situations, Situation{
				Kind:         SituationBranchMismatch,
				ProjectKey:   g.Key,
				ProjectLabel: g.Label,
				Nudge:        nudge,
				Machines:     append([]string{local.Machine}, machineNames(remote)...),
				Stale:        mismatchStale,
			})
		}
	}

	// Same branch, different HEAD tip (uses published short SHAs; no object fetch).
	if local != nil && len(remote) > 0 && !branchMismatch {
		localSHA := strings.TrimSpace(local.Repo.HeadSHA)
		localBranch := strings.TrimSpace(local.Repo.Branch)
		if localSHA != "" && localBranch != "" && !strings.HasPrefix(localBranch, "detached HEAD") {
			var tipParts []string
			var tipStale bool
			for _, r := range remote {
				otherBranch := strings.TrimSpace(r.Repo.Branch)
				otherSHA := strings.TrimSpace(r.Repo.HeadSHA)
				if otherBranch != localBranch || otherSHA == "" {
					continue
				}
				if otherSHA != localSHA {
					tipParts = append(tipParts, fmt.Sprintf("%s on %s", otherSHA, formatMachineLabel(r)))
					if r.Stale {
						tipStale = true
					}
				}
			}
			if len(tipParts) > 0 {
				nudge := fmt.Sprintf("same branch %q but different tip (local %s vs %s) — pull/push or inspect divergence",
					localBranch, localSHA, strings.Join(tipParts, ", "))
				if tipStale {
					nudge += " (some snapshots stale)"
				}
				situations = append(situations, Situation{
					Kind:         SituationTipMismatch,
					ProjectKey:   g.Key,
					ProjectLabel: g.Label,
					Nudge:        nudge,
					Machines:     append([]string{local.Machine}, machineNames(remote)...),
					Stale:        tipStale,
				})
			}
		}
	}

	if local != nil {
		localClean := local.Repo.Error == "" && !snapshotNeedsAttention(local.Repo)
		if localClean {
			var dirtyMachines []string
			var unpushedMachines []string
			var workStale bool
			for _, r := range remote {
				if r.Repo.Error != "" {
					continue
				}
				if r.Repo.IsDirty {
					dirtyMachines = append(dirtyMachines, formatMachineLabel(r))
					if r.Stale {
						workStale = true
					}
				} else if r.Repo.HasUnpushed {
					unpushedMachines = append(unpushedMachines, formatMachineLabel(r))
					if r.Stale {
						workStale = true
					}
				}
			}
			var parts []string
			if len(dirtyMachines) > 0 {
				parts = append(parts, fmt.Sprintf("uncommitted work on %s (pull will not get those changes — commit/push there or continue on that machine)",
					strings.Join(dirtyMachines, ", ")))
			}
			if len(unpushedMachines) > 0 {
				parts = append(parts, fmt.Sprintf("unpushed commits on %s — pull after they push, or continue there",
					strings.Join(unpushedMachines, ", ")))
			}
			if len(parts) > 0 {
				nudge := "other machine has " + strings.Join(parts, "; ")
				if workStale {
					nudge += " (snapshot may be stale)"
				}
				machines := append([]string{}, dirtyMachines...)
				machines = append(machines, unpushedMachines...)
				situations = append(situations, Situation{
					Kind:         SituationOtherMachineWork,
					ProjectKey:   g.Key,
					ProjectLabel: g.Label,
					Nudge:        nudge,
					Machines:     machines,
					Stale:        workStale,
				})
			}
		}
	}

	// Standalone stale cue only when a remote row itself needs attention and no
	// stronger situation already carried a stale qualifier (avoids noisy clean lists).
	if anyRemoteStale {
		alreadyQualified := false
		for _, s := range situations {
			if s.Stale {
				alreadyQualified = true
				break
			}
		}
		if !alreadyQualified {
			var staleMachines []string
			for _, r := range remote {
				if r.Stale && snapshotNeedsAttention(r.Repo) {
					staleMachines = append(staleMachines, formatMachineLabel(r))
				}
			}
			if len(staleMachines) > 0 {
				situations = append(situations, Situation{
					Kind:         SituationStaleEvidence,
					ProjectKey:   g.Key,
					ProjectLabel: g.Label,
					Nudge:        fmt.Sprintf("remote snapshot(s) stale for: %s — treat other-machine advice cautiously", strings.Join(staleMachines, ", ")),
					Machines:     staleMachines,
					Stale:        true,
				})
			}
		}
	}

	return situations
}

func formatMachineLabel(row AggregateRow) string {
	name := row.Machine
	if row.Stale {
		return name + " (stale)"
	}
	return name
}

func machineNames(rows []AggregateRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Machine)
	}
	return out
}

// ProjectKeysWithSituations returns correlation keys that produced at least one situation.
func ProjectKeysWithSituations(situations []Situation) map[string]bool {
	keys := make(map[string]bool, len(situations))
	for _, s := range situations {
		keys[s.ProjectKey] = true
	}
	return keys
}

// FilterRowsByProjectKeys keeps aggregate rows whose correlation key is in keys.
// Load-error rows are always retained. When keys is empty, only load-error rows remain.
func FilterRowsByProjectKeys(rows []AggregateRow, keys map[string]bool) []AggregateRow {
	var out []AggregateRow
	for _, row := range rows {
		if row.LoadError != "" {
			out = append(out, row)
			continue
		}
		if len(keys) > 0 && keys[repoCorrelationKey(row.Repo)] {
			out = append(out, row)
		}
	}
	return out
}

// DisplayAttention prints the primary Attention / nudge list.
// Suggestions are advisory only and must never trigger git writes on user repos.
func DisplayAttention(situations []Situation) {
	fmt.Println("Attention (suggestions only — no commands are run):")
	if len(situations) == 0 {
		fmt.Println("  (nothing needing attention)")
		fmt.Println()
		return
	}
	for _, s := range situations {
		fmt.Printf("  • %s\n", s.ProjectLabel)
		fmt.Printf("      → %s\n", s.Nudge)
	}
	fmt.Println()
}

// DetectLocalSituations builds Attention cues from a local-only scan (no state bus).
func DetectLocalSituations(machineID string, results []RepoStatus) []Situation {
	if machineID == "" {
		machineID = "local"
	}
	var rows []AggregateRow
	for _, status := range results {
		rows = append(rows, AggregateRow{
			Machine: machineID,
			Local:   true,
			Repo:    RepoStatusToSnapshot(status, false),
		})
	}
	return DetectSituations(rows)
}
