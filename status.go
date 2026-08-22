package main

import (
	"fmt"
	"strings"
)

// needsAttention is the shared predicate for dirty-only filtering.
func needsAttention(err string, isEmpty, isDirty, hasUnpushed, hasBehind, hasUntrackedUpstream bool) bool {
	if isEmpty && err == "" && !isDirty {
		return false
	}
	return err != "" || isDirty || hasUnpushed || hasBehind || hasUntrackedUpstream
}

// snapshotNeedsAttention reports whether a repo snapshot should appear in
// dirty-only views and attention summaries.
func snapshotNeedsAttention(repo RepoSnapshot) bool {
	return needsAttention(repo.Error, repo.IsEmpty, repo.IsDirty, repo.HasUnpushed, repo.HasBehind, repo.HasUntrackedUpstream)
}

// repoNeedsAttention is the live-scan equivalent of snapshotNeedsAttention.
func repoNeedsAttention(status RepoStatus) bool {
	return needsAttention(status.Error, status.IsEmpty, status.IsDirty, status.HasUnpushed, status.HasBehind, status.HasUntrackedUpstream)
}

// snapshotChangesText lists change tags for a repository snapshot.
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
		if repo.AheadCount > 0 {
			changes = append(changes, fmt.Sprintf("unpushed:%d", repo.AheadCount))
		} else {
			changes = append(changes, "unpushed")
		}
	}
	if repo.HasBehind {
		if repo.BehindCount > 0 {
			changes = append(changes, fmt.Sprintf("behind:%d", repo.BehindCount))
		} else {
			changes = append(changes, "behind")
		}
	}
	if repo.HasUntrackedUpstream {
		changes = append(changes, "untracked-upstream")
	}
	return changes
}

// repoStatusText returns display labels for status and changes columns.
// When plain is true, labels omit emoji (for CSV export).
func repoStatusText(repo RepoSnapshot, plain bool) (statusText, changesText string) {
	if repo.Error != "" {
		if plain {
			return "Error", repo.Error
		}
		return "❌ Error", repo.Error
	}

	changes := strings.Join(snapshotChangesText(repo), ", ")
	if repo.IsDirty {
		if plain {
			return "Dirty", changes
		}
		return "⚠️  Dirty", changes
	}
	if repo.IsEmpty {
		if plain {
			return "Empty", "no commits yet"
		}
		return "📭 Empty", "no commits yet"
	}
	if repo.HasUntrackedUpstream {
		if plain {
			return "UntrackedUpstream", "untracked-upstream"
		}
		return "🔗 Untracked Upstream", "untracked-upstream"
	}
	if repo.HasBehind && repo.HasUnpushed {
		if plain {
			return "Diverged", changes
		}
		return "↕️ Diverged", changes
	}
	if repo.HasBehind {
		if plain {
			return "Behind", changes
		}
		return "⬇️ Behind", changes
	}
	if repo.HasUnpushed {
		if plain {
			return "Unpushed", changes
		}
		return "⬆️ Unpushed", changes
	}
	if plain {
		return "Clean", "-"
	}
	return "✅ Clean", "-"
}
