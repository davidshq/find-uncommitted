package main

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// repoOriginURL reads the configured origin remote URL for a repository.
// Missing origin is not an error — local-only repos simply have no correlation URL.
func repoOriginURL(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return NormalizeOriginURL(string(out))
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

// repoCorrelationKey is the stable identity used to match one project across
// machines. Prefer normalized origin; fall back to path basename for local-only
// repos (e.g. manuscripts with no remote).
func repoCorrelationKey(repo RepoSnapshot) string {
	if o := strings.TrimSpace(repo.Origin); o != "" {
		return "origin:" + o
	}
	base := filepath.Base(repo.Path)
	if base == "" || base == "." || base == string(filepath.Separator) || base == "…" {
		return "path:" + repo.Path
	}
	return "basename:" + base
}
