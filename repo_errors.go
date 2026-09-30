package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/davidshq/find-uncommitted/internal/gitexec"
)

// isEmptyRepositoryMessage reports whether git output indicates an unborn HEAD
// (no commits yet). It deliberately omits generic "unknown revision" /
// "invalid reference" phrases — those also appear for deleted upstreams and
// other real failures that must stay classified as errors.
func isEmptyRepositoryMessage(stderr string, err error) bool {
	combined := strings.ToLower(strings.TrimSpace(stderr))
	if err != nil {
		combined += " " + strings.ToLower(err.Error())
	}
	return strings.Contains(combined, "does not have any commits yet") ||
		strings.Contains(combined, "needed a single revision") ||
		(strings.Contains(combined, "ambiguous argument") && strings.Contains(combined, "head"))
}

// repoIsEmpty returns true when the repository has no commits yet.
func repoIsEmpty(ctx context.Context, repoPath string) bool {
	_, stderr, err := gitexec.Run(ctx, repoPath, "rev-parse", "--verify", "HEAD")
	if err == nil {
		return false
	}
	if gitexec.IsContextErr(ctx, err) {
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
	return false, "Failed to check upstream tracking: " + gitexec.FormatError(stderr, err)
}

// upstreamGone reports a configured upstream whose tracking ref no longer
// exists — the branch was merged and pruned (`git status` shows "[gone]").
// Checked structurally rather than by stderr text, which varies by git version.
func upstreamGone(ctx context.Context, repoPath, branch string) bool {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false
	}
	out, _, err := gitexec.Run(ctx, repoPath, "for-each-ref", "--format=%(upstream)", "refs/heads/"+branch)
	ref := strings.TrimSpace(out)
	if err != nil || ref == "" {
		return false
	}
	_, _, err = gitexec.Run(ctx, repoPath, "rev-parse", "--verify", "--quiet", ref)
	return err != nil && !gitexec.IsContextErr(ctx, err)
}

func invalidRepositoryError(stderr string, err error) string {
	detail := gitexec.FormatError(stderr, err)
	if detail == "" || detail == "unknown git error" {
		return "Not a valid git repository"
	}
	return "Not a valid git repository: " + detail
}

func appendRepoCheckError(status *RepoSnapshot, stderr string, err error, primary, followUp string) {
	label := primary
	if status.Error != "" {
		label = followUp
	}
	fragment := fmt.Sprintf("%s: %s", label, gitexec.FormatError(stderr, err))
	if status.Error == "" {
		status.Error = fragment
	} else {
		status.Error += "; " + fragment
	}
}
