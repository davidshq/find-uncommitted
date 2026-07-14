package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AggregateRow is one repo row in the combined local + remote view.
type AggregateRow struct {
	Machine   string
	Stale     bool
	Local     bool
	Repo      RepoSnapshot
	LoadError string
}

// BuildAggregateRows merges local scan results with remote machine snapshots.
func BuildAggregateRows(localMachine string, localResults []RepoStatus, remote []LoadedSnapshot) []AggregateRow {
	var rows []AggregateRow
	for _, status := range localResults {
		rows = append(rows, AggregateRow{
			Machine: localMachine,
			Local:   true,
			Repo:    RepoStatusToSnapshot(status, false),
		})
	}
	for _, item := range remote {
		if item.LoadErr != "" {
			rows = append(rows, AggregateRow{
				Machine:   filepath.Base(item.FilePath),
				LoadError: item.LoadErr,
			})
			continue
		}
		if item.Snapshot.MachineID == localMachine {
			// Local live scan already shown; skip duplicate remote copy.
			continue
		}
		for _, repo := range item.Snapshot.Repos {
			rows = append(rows, AggregateRow{
				Machine: item.Snapshot.MachineID,
				Stale:   item.Stale,
				Repo:    repo,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Machine != rows[j].Machine {
			return rows[i].Machine < rows[j].Machine
		}
		return rows[i].Repo.Path < rows[j].Repo.Path
	})
	return rows
}

func displayAggregateTable(rows []AggregateRow, dirtyOnlyFilter bool) {
	const machineCol = 16
	const pathCol = 42
	const branchCol = 16
	const statusCol = 18

	fmt.Printf("%-*s %-*s %-*s %-*s %s\n",
		machineCol, "Machine", pathCol, "Repository", branchCol, "Branch", statusCol, "Status", "Changes")
	fmt.Println(strings.Repeat("-", 110))

	wd, _ := os.Getwd()
	shown := 0
	for _, row := range rows {
		if row.LoadError != "" {
			fmt.Printf("%-*s %-*s %-*s %-*s %s\n",
				machineCol, truncate(row.Machine, machineCol-1),
				pathCol, "-",
				branchCol, "-",
				statusCol, "❌ Error",
				row.LoadError)
			shown++
			continue
		}

		statusText, changesText := repoSnapshotStatusText(row.Repo)
		if dirtyOnlyFilter && !snapshotNeedsAttention(row.Repo) {
			continue
		}

		machine := row.Machine
		if row.Local {
			machine = machine + "*"
		}
		if row.Stale {
			machine = machine + " (stale)"
		}

		path := displayPath(wd, row.Repo.Path)
		branch := row.Repo.Branch
		if len(branch) > branchCol-3 {
			branch = branch[:branchCol-3] + "..."
		}

		fmt.Printf("%-*s %-*s %-*s %-*s %s\n",
			machineCol, truncate(machine, machineCol),
			pathCol, truncate(path, pathCol),
			branchCol, branch,
			statusCol, statusText,
			changesText)
		shown++
	}
	if shown == 0 {
		fmt.Println("(no repositories to display)")
	}
}

func repoSnapshotStatusText(repo RepoSnapshot) (statusText, changesText string) {
	if repo.Error != "" {
		return "❌ Error", repo.Error
	}
	if repo.IsDirty {
		return "⚠️  Dirty", strings.Join(snapshotChangesText(repo), ", ")
	}
	if repo.HasUntrackedUpstream {
		return "🔗 Untracked Upstream", "untracked-upstream"
	}
	if repo.HasUnpushed {
		return "⬆️ Unpushed", "unpushed"
	}
	return "✅ Clean", "-"
}

func snapshotChangesText(repo RepoSnapshot) []string {
	var changes []string
	if repo.HasUnstaged {
		changes = append(changes, "unstaged")
	}
	if repo.HasStaged {
		changes = append(changes, "staged")
	}
	if repo.HasUntracked {
		changes = append(changes, "untracked")
	}
	if repo.HasUnpushed {
		changes = append(changes, "unpushed")
	}
	if repo.HasUntrackedUpstream {
		changes = append(changes, "untracked-upstream")
	}
	return changes
}

func displayPath(wd, path string) string {
	rel, err := filepath.Rel(wd, path)
	if err != nil || rel == "." {
		return path
	}
	return rel
}

func truncate(s string, max int) string {
	if max <= 3 || len(s) <= max {
		return s
	}
	return "..." + s[len(s)-(max-3):]
}

func printStaleMachineSummary(remote []LoadedSnapshot, staleTTL time.Duration) {
	var stale []string
	for _, item := range remote {
		if item.LoadErr == "" && item.Stale {
			age := time.Since(item.Snapshot.UpdatedAt).Truncate(time.Second)
			stale = append(stale, fmt.Sprintf("%s (updated %s ago)", item.Snapshot.MachineID, age))
		}
	}
	if len(stale) == 0 {
		return
	}
	fmt.Printf("\nStale machines (no publish within %s): %s\n", staleTTL, strings.Join(stale, ", "))
}

func printAggregateSummary(rows []AggregateRow, dirtyOnlyFilter bool) {
	localAttention, remoteAttention := 0, 0
	localTotal, remoteTotal := 0, 0
	staleRows, loadErrs := 0, 0
	for _, row := range rows {
		if row.LoadError != "" {
			loadErrs++
			continue
		}
		if row.Local {
			localTotal++
			if snapshotNeedsAttention(row.Repo) {
				localAttention++
			}
		} else {
			remoteTotal++
			if row.Stale {
				staleRows++
			}
			if snapshotNeedsAttention(row.Repo) {
				remoteAttention++
			}
		}
	}
	if dirtyOnlyFilter {
		fmt.Printf("\nSummary: %d local needing attention, %d remote needing attention, %d load errors\n",
			localAttention, remoteAttention, loadErrs)
		return
	}
	fmt.Printf("\nSummary: %d local repos (%d need attention), %d remote repos (%d need attention, %d stale rows), %d load errors\n",
		localTotal, localAttention, remoteTotal, remoteAttention, staleRows, loadErrs)
}

func exportAggregateToCSV(rows []AggregateRow, filename string, dirtyOnlyFilter bool) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Machine", "Local", "Stale", "Repository", "Branch", "Status", "Changes"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header to CSV: %v", err)
	}

	for _, row := range rows {
		if row.LoadError != "" {
			if err := writer.Write([]string{
				row.Machine, "false", "false", "", "", "Error", row.LoadError,
			}); err != nil {
				return err
			}
			continue
		}
		if dirtyOnlyFilter && !snapshotNeedsAttention(row.Repo) {
			continue
		}
		statusText, changesText := repoSnapshotStatusTextPlain(row.Repo)
		if err := writer.Write([]string{
			row.Machine,
			fmt.Sprintf("%t", row.Local),
			fmt.Sprintf("%t", row.Stale),
			row.Repo.Path,
			row.Repo.Branch,
			statusText,
			changesText,
		}); err != nil {
			return err
		}
	}
	return nil
}

func repoSnapshotStatusTextPlain(repo RepoSnapshot) (statusText, changesText string) {
	if repo.Error != "" {
		return "Error", repo.Error
	}
	if repo.IsDirty {
		return "Dirty", strings.Join(snapshotChangesText(repo), ", ")
	}
	if repo.HasUntrackedUpstream {
		return "UntrackedUpstream", "untracked-upstream"
	}
	if repo.HasUnpushed {
		return "Unpushed", "unpushed"
	}
	return "Clean", "-"
}
