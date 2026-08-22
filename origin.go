package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// repoOriginURL reads the configured origin remote URL for a repository.
// Missing origin is not an error — local-only repos simply have no correlation URL.
func repoOriginURL(ctx context.Context, repoPath string) string {
	out, _, err := runGit(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return NormalizeOriginURL(out)
}

// NormalizeOriginURL canonicalizes git remote URLs so SSH and HTTPS forms of the
// same repository compare equal across machines.
//
// Examples that all become "github.com/acme/app":
//
//	git@github.com:acme/app.git
//	https://github.com/acme/app.git
//	ssh://git@github.com/acme/app
func NormalizeOriginURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// SCP-like shorthand: git@host:path/to/repo.git
	if !strings.Contains(raw, "://") {
		if at := strings.Index(raw, "@"); at >= 0 {
			rest := raw[at+1:]
			if colon := strings.Index(rest, ":"); colon >= 0 && !strings.Contains(rest[:colon], "/") {
				host := rest[:colon]
				repoPath := rest[colon+1:]
				return canonicalizeHostPath(host, repoPath)
			}
		}
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// Last resort: strip .git and trailing slash from opaque strings.
		return stripGitSuffix(strings.Trim(raw, "/"))
	}

	host := u.Hostname()
	repoPath := u.Path
	if u.Opaque != "" && repoPath == "" {
		// Rare: scheme:opaque forms
		repoPath = u.Opaque
	}
	return canonicalizeHostPath(host, repoPath)
}

func canonicalizeHostPath(host, repoPath string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	repoPath = strings.TrimSpace(repoPath)
	repoPath = strings.TrimPrefix(repoPath, "/")
	repoPath = stripGitSuffix(repoPath)
	repoPath = path.Clean("/" + repoPath)
	repoPath = strings.TrimPrefix(repoPath, "/")
	if host == "" {
		return repoPath
	}
	if repoPath == "" || repoPath == "." {
		return host
	}
	return host + "/" + repoPath
}

func stripGitSuffix(s string) string {
	s = strings.TrimSuffix(s, "/")
	if strings.HasSuffix(strings.ToLower(s), ".git") {
		s = s[:len(s)-len(".git")]
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

// redactOrigin replaces a normalized origin with a short hash so snapshots can
// still correlate the same project across machines without publishing the URL.
func redactOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(origin))
	return fmt.Sprintf("redacted:%x", sum[:8])
}

// pathBasenameIdentity returns parent/base for repos without origin correlation.
// Empty means the path cannot be identified from basename alone.
func pathBasenameIdentity(path string) string {
	base := filepath.Base(path)
	if base == "" || base == "." || base == string(filepath.Separator) || base == "…" {
		return ""
	}
	parent := filepath.Base(filepath.Dir(path))
	if parent != "" && parent != "." && parent != string(filepath.Separator) && parent != "…" {
		return parent + "/" + base
	}
	return base
}

// repoCorrelationKey is the stable identity used to match one project across
// machines. Prefer normalized origin; for local-only repos fall back to
// parent/basename (e.g. manuscripts/book) so same-named folders under different
// parents do not collide, while similarly laid-out trees on two machines still match.
func repoCorrelationKey(repo RepoSnapshot) string {
	if o := strings.TrimSpace(repo.Origin); o != "" {
		return "origin:" + o
	}
	if id := pathBasenameIdentity(repo.Path); id != "" {
		return "basename:" + id
	}
	return "path:" + repo.Path
}
