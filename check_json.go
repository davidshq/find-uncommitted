package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// checkJSONSchemaVersion is the stable contract for editor/script consumers.
// Additive fields may appear without a bump; renames/removals require a new version.
const checkJSONSchemaVersion = 1

// CheckJSONResult is the machine-readable check mode payload (--json).
type CheckJSONResult struct {
	SchemaVersion int                  `json:"schemaVersion"`
	OK            bool                 `json:"ok"`
	Attention     bool                 `json:"attention"`
	Error         string               `json:"error,omitempty"`
	Project       string               `json:"project,omitempty"`
	Machines      []CheckJSONMachine   `json:"machines,omitempty"`
	Situations    []CheckJSONSituation `json:"situations,omitempty"`
}

// CheckJSONMachine is one project × machine cell for clients.
type CheckJSONMachine struct {
	ID                   string `json:"id"`
	Local                bool   `json:"local"`
	Stale                bool   `json:"stale,omitempty"`
	Path                 string `json:"path,omitempty"`
	Origin               string `json:"origin,omitempty"`
	Branch               string `json:"branch,omitempty"`
	HasUnstaged          bool   `json:"has_unstaged,omitempty"`
	HasStaged            bool   `json:"has_staged,omitempty"`
	HasUntracked         bool   `json:"has_untracked,omitempty"`
	HasUnpushed          bool   `json:"has_unpushed,omitempty"`
	HasBehind            bool   `json:"has_behind,omitempty"`
	HasUntrackedUpstream bool   `json:"has_untracked_upstream,omitempty"`
	AheadCount           int    `json:"ahead_count,omitempty"`
	BehindCount          int    `json:"behind_count,omitempty"`
	HeadSHA              string `json:"head_sha,omitempty"`
	IsDirty              bool   `json:"is_dirty,omitempty"`
	IsClean              bool   `json:"is_clean,omitempty"`
	IsEmpty              bool   `json:"is_empty,omitempty"`
	Error                string `json:"error,omitempty"`
}

// CheckJSONSituation is one Attention cue for clients.
type CheckJSONSituation struct {
	Kind     string   `json:"kind"`
	Nudge    string   `json:"nudge"`
	Machines []string `json:"machines,omitempty"`
	Stale    bool     `json:"stale,omitempty"`
}

func buildCheckJSONResult(label string, rows []AggregateRow, situations []Situation) CheckJSONResult {
	out := CheckJSONResult{
		SchemaVersion: checkJSONSchemaVersion,
		OK:            len(situations) == 0,
		Attention:     len(situations) > 0,
		Project:       label,
		Machines:      make([]CheckJSONMachine, 0, len(rows)),
		Situations:    make([]CheckJSONSituation, 0, len(situations)),
	}
	ordered := orderCheckRows(rows)
	for _, row := range ordered {
		if row.LoadError != "" {
			continue
		}
		out.Machines = append(out.Machines, checkJSONMachineFromRow(row))
	}
	for _, s := range situations {
		out.Situations = append(out.Situations, CheckJSONSituation{
			Kind:     string(s.Kind),
			Nudge:    s.Nudge,
			Machines: s.Machines,
			Stale:    s.Stale,
		})
	}
	return out
}

func checkJSONMachineFromRow(row AggregateRow) CheckJSONMachine {
	r := row.Repo
	return CheckJSONMachine{
		ID:                   row.Machine,
		Local:                row.Local,
		Stale:                row.Stale,
		Path:                 r.Path,
		Origin:               r.Origin,
		Branch:               r.Branch,
		HasUnstaged:          r.HasUnstaged,
		HasStaged:            r.HasStaged,
		HasUntracked:         r.HasUntracked,
		HasUnpushed:          r.HasUnpushed,
		HasBehind:            r.HasBehind,
		HasUntrackedUpstream: r.HasUntrackedUpstream,
		AheadCount:           r.AheadCount,
		BehindCount:          r.BehindCount,
		HeadSHA:              r.HeadSHA,
		IsDirty:              r.IsDirty,
		IsClean:              r.IsClean,
		IsEmpty:              r.IsEmpty,
		Error:                r.Error,
	}
}

func printCheckJSON(result CheckJSONResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}

func printCheckJSONError(msg string) {
	_ = printCheckJSON(CheckJSONResult{
		SchemaVersion: checkJSONSchemaVersion,
		OK:            false,
		Attention:     false,
		Error:         msg,
	})
}

func parseCheckArgs(args []string) (path string, jsonOut bool, err error) {
	if len(args) == 0 || args[0] != "check" {
		return "", false, fmt.Errorf("not check mode")
	}
	for _, a := range args[1:] {
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--":
			continue
		case strings.HasPrefix(a, "-"):
			return "", false, fmt.Errorf("unknown check flag %q (use global flags before check, or --json)", a)
		case path == "":
			path = a
		default:
			return "", false, fmt.Errorf("unexpected argument %q", a)
		}
	}
	return path, jsonOut, nil
}
