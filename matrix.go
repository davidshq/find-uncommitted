package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// matrixColumn is one machine axis for the Project × Machine view.
type matrixColumn struct {
	ID    string
	Local bool
}

// formatMatrixStatus returns a compact status token for a matrix cell
// (no branch, no emoji). Multiple signals compose with " · " so dirty does
// not hide ahead/behind (e.g. "dirty · ↑3", "dirty · ↓2").
func formatMatrixStatus(repo RepoSnapshot) string {
	if repo.Error != "" {
		return "error"
	}
	if repo.IsEmpty && !repo.IsDirty {
		return "empty"
	}

	var parts []string
	if repo.IsDirty {
		parts = append(parts, "dirty")
	}
	if repo.HasUntrackedUpstream {
		parts = append(parts, "no upstream")
	} else if repo.HasBehind && repo.HasUnpushed {
		parts = append(parts, aheadToken(repo)+"/"+behindToken(repo))
	} else if repo.HasBehind {
		parts = append(parts, behindToken(repo))
	} else if repo.HasUnpushed {
		parts = append(parts, aheadToken(repo))
	}

	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, " · ")
}

func aheadToken(repo RepoSnapshot) string {
	if repo.AheadCount > 0 {
		return fmt.Sprintf("↑%d", repo.AheadCount)
	}
	return "↑"
}

func behindToken(repo RepoSnapshot) string {
	if repo.BehindCount > 0 {
		return fmt.Sprintf("↓%d", repo.BehindCount)
	}
	return "↓"
}

// matrixStatusRank orders severity for collapsing multiple clones on one machine
// (higher = worse).
func matrixStatusRank(repo RepoSnapshot) int {
	if repo.Error != "" {
		return 70
	}
	if repo.IsDirty {
		return 60
	}
	if repo.HasBehind && repo.HasUnpushed {
		return 50
	}
	if repo.HasBehind {
		return 40
	}
	if repo.HasUnpushed {
		return 30
	}
	if repo.HasUntrackedUpstream {
		return 20
	}
	if repo.IsEmpty {
		return 10
	}
	return 0
}

// collapseRowsForMachine picks the worst-status clone when one machine has
// multiple work trees for the same project. Stale is sticky if any row is stale.
func collapseRowsForMachine(rows []AggregateRow) AggregateRow {
	if len(rows) == 0 {
		return AggregateRow{}
	}
	best := rows[0]
	for _, r := range rows[1:] {
		if matrixStatusRank(r.Repo) > matrixStatusRank(best.Repo) {
			stale := best.Stale || r.Stale
			local := best.Local || r.Local
			best = r
			best.Stale = stale
			best.Local = local
			continue
		}
		if r.Stale {
			best.Stale = true
		}
		if r.Local {
			best.Local = true
		}
	}
	return best
}

// projectBranchesDiffer reports whether non-empty branch names disagree across cells.
func projectBranchesDiffer(byMachine map[string]AggregateRow) bool {
	var seen string
	for _, row := range byMachine {
		b := strings.TrimSpace(row.Repo.Branch)
		if b == "" {
			continue
		}
		if seen == "" {
			seen = b
			continue
		}
		if b != seen {
			return true
		}
	}
	return false
}

// formatMatrixCellForProject builds one matrix cell. Branch is shown when
// branches differ across the project or the status is not clean (glance aid).
func formatMatrixCellForProject(row AggregateRow, branchesDiffer bool) string {
	status := formatMatrixStatus(row.Repo)
	showBranch := branchesDiffer || status != "clean"
	cell := status
	if showBranch {
		if b := strings.TrimSpace(row.Repo.Branch); b != "" {
			cell = status + " · " + b
		}
	}
	if row.Stale {
		cell += " stale"
	}
	return cell
}

// matrixColumnsFromRows returns stable machine columns: locals first (marked
// later with *), then remote machine ids sorted.
func matrixColumnsFromRows(rows []AggregateRow) []matrixColumn {
	localSeen := map[string]bool{}
	remoteSeen := map[string]bool{}
	var locals []matrixColumn
	var remotes []string
	for _, row := range rows {
		if row.LoadError != "" {
			if row.Machine == "" {
				continue
			}
			if !remoteSeen[row.Machine] && !localSeen[row.Machine] {
				remoteSeen[row.Machine] = true
				remotes = append(remotes, row.Machine)
			}
			continue
		}
		if row.Machine == "" {
			continue
		}
		if row.Local {
			if !localSeen[row.Machine] {
				localSeen[row.Machine] = true
				locals = append(locals, matrixColumn{ID: row.Machine, Local: true})
			}
			continue
		}
		if localSeen[row.Machine] {
			continue
		}
		if !remoteSeen[row.Machine] {
			remoteSeen[row.Machine] = true
			remotes = append(remotes, row.Machine)
		}
	}
	sort.Strings(remotes)
	out := append([]matrixColumn{}, locals...)
	for _, id := range remotes {
		out = append(out, matrixColumn{ID: id, Local: false})
	}
	return out
}

func columnHeader(c matrixColumn) string {
	if c.Local {
		return c.ID + "*"
	}
	return c.ID
}

// displayProjectMachineMatrix prints the default Project × Machine morning view.
func displayProjectMachineMatrix(rows []AggregateRow) {
	cols := matrixColumnsFromRows(rows)
	groups := GroupRowsByProject(rows)

	// Load-error lines first.
	loadErrs := 0
	for _, row := range rows {
		if row.LoadError == "" {
			continue
		}
		if loadErrs == 0 {
			fmt.Println("Load errors:")
		}
		fmt.Printf("  %s: %s\n", row.Machine, row.LoadError)
		loadErrs++
	}
	if loadErrs > 0 {
		fmt.Println()
	}

	if len(cols) == 0 && len(groups) == 0 {
		fmt.Println("(no repositories to display)")
		return
	}

	localHint := ""
	for _, c := range cols {
		if c.Local {
			localHint = c.ID + "*"
			break
		}
	}

	if localHint != "" {
		fmt.Printf("Projects  (you are %s)\n", localHint)
	} else {
		fmt.Println("Projects")
	}

	const maxProjectWidth = 40
	const maxCellWidth = 48
	const minCellWidth = 8

	type matrixLine struct {
		label string
		cells []string
	}
	lines := make([]matrixLine, 0, len(groups))
	cellWidths := make([]int, len(cols))
	for i, c := range cols {
		cellWidths[i] = utf8.RuneCountInString(columnHeader(c))
		if cellWidths[i] < minCellWidth {
			cellWidths[i] = minCellWidth
		}
	}

	projectWidth := len("Project")
	for _, g := range groups {
		if n := utf8.RuneCountInString(g.Label); n > projectWidth {
			projectWidth = n
		}

		byMachine := map[string][]AggregateRow{}
		for _, row := range g.Rows {
			byMachine[row.Machine] = append(byMachine[row.Machine], row)
		}
		collapsed := map[string]AggregateRow{}
		for mid, list := range byMachine {
			collapsed[mid] = collapseRowsForMachine(list)
		}
		branchesDiffer := projectBranchesDiffer(collapsed)

		cells := make([]string, len(cols))
		for i, c := range cols {
			cell := "—"
			if row, ok := collapsed[c.ID]; ok {
				cell = formatMatrixCellForProject(row, branchesDiffer)
			}
			cells[i] = cell
			if n := utf8.RuneCountInString(cell); n > cellWidths[i] {
				cellWidths[i] = n
			}
		}
		lines = append(lines, matrixLine{label: g.Label, cells: cells})
	}
	if projectWidth > maxProjectWidth {
		projectWidth = maxProjectWidth
	}
	for i := range cellWidths {
		if cellWidths[i] > maxCellWidth {
			cellWidths[i] = maxCellWidth
		}
	}

	// Header
	fmt.Printf("%-*s", projectWidth, "Project")
	for i, c := range cols {
		fmt.Printf("  %-*s", cellWidths[i], truncateRunes(columnHeader(c), cellWidths[i]))
	}
	fmt.Println()
	totalWidth := projectWidth
	for _, w := range cellWidths {
		totalWidth += w + 2
	}
	if totalWidth < 40 {
		totalWidth = 40
	}
	fmt.Println(strings.Repeat("-", totalWidth))

	if len(lines) == 0 {
		fmt.Println("(no projects to display)")
		return
	}

	for _, line := range lines {
		fmt.Printf("%-*s", projectWidth, truncateRunes(line.label, projectWidth))
		for i, cell := range line.cells {
			fmt.Printf("  %-*s", cellWidths[i], truncateRunes(cell, cellWidths[i]))
		}
		fmt.Println()
	}
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	if max <= 3 {
		r, _ := utf8.DecodeRuneInString(s)
		return string(r)
	}
	runes := []rune(s)
	return string(runes[:max-3]) + "..."
}
