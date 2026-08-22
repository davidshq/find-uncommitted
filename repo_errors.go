package main

import (
	"context"
	"fmt"
	"strings"
)

// isEmptyRepositoryMessage reports whether git output indicates no commits yet.
func isEmptyRepositoryMessage(stderr string, err error) bool {
	combined := strings.ToLower(strings.TrimSpace(stderr))
	if err != nil {
		combined += " " + strings.ToLower(err.Error())
	}
	return strings.Contains(combined, "does not have any commits yet") ||
		strings.Contains(combined, "unknown revision") ||
		strings.Contains(combined, "invalid reference") ||
		strings.Contains(combined, "needed a single revision") ||
		strings.Contains(combined, "ambiguous argument") && strings.Contains(combined, "head")
}

// repoIsEmpty returns true when the repository has no commits yet.
func repoIsEmpty(ctx context.Context, repoPath string) bool {
	_, stderr, err := runGit(ctx, repoPath, "rev-parse", "--verify", "HEAD")
	if err == nil {
		return false
	}
	if isGitContextErr(ctx, err) {
		return false
	}
	return isEmptyRepositoryMessage(stderr, err)
}

// classifyUpstreamFailure interprets @{u} resolution failures.
// Callers must handle empty repositories before invoking this helper.
func classifyUpstreamFailure(stderr string, err error) (untrackedUpstream bool, repoErr string) {
	combined := strings.ToLower(stderr + " " + err.Error())
	if strings.Contains(combined, "no upstream configured") {
		return true, ""
	}
	return false, "Failed to check upstream tracking: " + formatGitError(stderr, err)
}

func invalidRepositoryError(stderr string, err error) string {
	detail := formatGitError(stderr, err)
	if detail == "" || detail == "unknown git error" {
		return "Not a valid git repository"
	}
	return "Not a valid git repository: " + detail
}

func appendRepoCheckError(status *RepoStatus, stderr string, err error, primary, followUp string) {
	label := primary
	if status.Error != "" {
		label = followUp
	}
	fragment := fmt.Sprintf("%s: %s", label, formatGitError(stderr, err))
	if status.Error == "" {
		status.Error = fragment
	} else {
		status.Error += "; " + fragment
	}
}
