package main

import (
	"strings"
	"testing"
)

func TestNormalizeOriginURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"git@github.com:acme/app.git", "github.com/acme/app"},
		{"https://github.com/acme/app.git", "github.com/acme/app"},
		{"https://github.com/acme/app/", "github.com/acme/app"},
		{"https://github.com/acme/app", "github.com/acme/app"},
		{"ssh://git@github.com/acme/app.git", "github.com/acme/app"},
		{"https://user:pass@github.com/acme/app.git", "github.com/acme/app"},
		{"git://github.com/acme/app.git", "github.com/acme/app"},
		{"https://gitlab.com/group/sub/proj.git", "gitlab.com/group/sub/proj"},
		{"git@gitlab.com:group/sub/proj.git", "gitlab.com/group/sub/proj"},
		{"SSH://Git@GitHub.COM/Acme/App.GIT", "github.com/Acme/App"},
	}
	for _, tc := range cases {
		got := NormalizeOriginURL(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeOriginURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeOriginURLSSHAndHTTPSMatch(t *testing.T) {
	ssh := NormalizeOriginURL("git@github.com:dave/find-uncommitted.git")
	https := NormalizeOriginURL("https://github.com/dave/find-uncommitted.git")
	if ssh == "" || ssh != https {
		t.Fatalf("SSH/HTTPS mismatch: %q vs %q", ssh, https)
	}
}

func TestRedactOriginPreservesCorrelation(t *testing.T) {
	a := redactOrigin("github.com/acme/app")
	b := redactOrigin("github.com/acme/app")
	c := redactOrigin("github.com/acme/other")
	if a == "" || a != b {
		t.Fatalf("same origin should redact identically: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("different origins should redact differently")
	}
	if a[:9] != "redacted:" {
		t.Fatalf("expected redacted: prefix, got %q", a)
	}
}

func TestRepoCorrelationKey(t *testing.T) {
	withOrigin := RepoSnapshot{Path: "/home/a/code/app", Origin: "github.com/acme/app"}
	otherPath := RepoSnapshot{Path: "D:\\work\\app", Origin: "github.com/acme/app"}
	if repoCorrelationKey(withOrigin) != repoCorrelationKey(otherPath) {
		t.Fatal("same origin should correlate across different paths")
	}

	// parent/basename: similarly laid-out local-only trees still match.
	localOnly := RepoSnapshot{Path: "/home/a/manuscripts/book"}
	otherLocal := RepoSnapshot{Path: "/Users/b/manuscripts/book"}
	if repoCorrelationKey(localOnly) != repoCorrelationKey(otherLocal) {
		t.Fatalf("parent/basename should correlate: %q vs %q",
			repoCorrelationKey(localOnly), repoCorrelationKey(otherLocal))
	}

	// Same leaf name under different parents must not collide.
	otherApp := RepoSnapshot{Path: "/home/a/archive/book"}
	if repoCorrelationKey(localOnly) == repoCorrelationKey(otherApp) {
		t.Fatal("different parent folders with same basename must not correlate")
	}

	if repoCorrelationKey(withOrigin) == repoCorrelationKey(localOnly) {
		t.Fatal("origin key must not collide with basename key")
	}
}

func TestMaybeRedactRepoSnapshotRedactsOrigin(t *testing.T) {
	status := RepoSnapshot{
		Path:    "/code/app",
		Origin:  "github.com/acme/app",
		Branch:  "main",
		IsClean: true,
	}
	redacted := maybeRedactRepoSnapshot(status, true)
	if redacted.Origin == status.Origin || redacted.Origin == "" {
		t.Fatalf("expected hashed origin when redacting, got %q", redacted.Origin)
	}
	if redacted.Origin != redactOrigin(status.Origin) {
		t.Fatalf("redacted origin mismatch: %q", redacted.Origin)
	}
}

// Y-4: a peer's published path uses the publisher's separators. Applying the
// reader's filepath rules left a Windows path whole on Linux, so local-only
// repos (no origin) never correlated between Windows and Linux machines.
func TestRepoCorrelationKeyAcrossWindowsAndUnixPaths(t *testing.T) {
	win := RepoSnapshot{Path: `C:\Users\dave\manuscripts\book`}
	unix := RepoSnapshot{Path: "/home/dave/manuscripts/book"}
	if got := pathBasenameIdentity(win.Path); got != "manuscripts/book" {
		t.Fatalf("pathBasenameIdentity(windows) = %q, want manuscripts/book", got)
	}
	if repoCorrelationKey(win) != repoCorrelationKey(unix) {
		t.Fatalf("windows %q vs unix %q keys differ", repoCorrelationKey(win), repoCorrelationKey(unix))
	}
	// Redacted paths published from Windows (…\book) read the same as from Unix.
	if pathBasenameIdentity(`…\book`) != pathBasenameIdentity("…/book") {
		t.Fatalf("redacted identities differ: %q vs %q", pathBasenameIdentity(`…\book`), pathBasenameIdentity("…/book"))
	}
	if got := redactPath(`C:\Users\dave\manuscripts\book`); got != "…/book" {
		t.Fatalf("redactPath(windows) = %q, want …/book", got)
	}
}

// Y-1: --redact-paths redacted Path/Origin but published Error verbatim, which
// carries absolute paths (dubious-ownership hint, git stderr).
func TestMaybeRedactRepoSnapshotScrubsErrorPaths(t *testing.T) {
	repo := "/home/dave/clients/acme-secret"
	cases := []RepoSnapshot{
		{Path: repo, Error: "Git ownership issue - run: git config --global --add safe.directory " + repo},
		{Path: repo, Error: "Not a valid git repository: not a git repository: '/home/dave/clients/.git'"},
		{Path: `C:\Users\dave\clients\acme-secret`, Error: `Git ownership issue - run: git config --global --add safe.directory C:/Users/dave/clients/acme-secret`},
	}
	for _, c := range cases {
		got := maybeRedactRepoSnapshot(c, true).Error
		if strings.Contains(got, "dave") || strings.Contains(got, "clients/") {
			t.Fatalf("redacted Error still leaks a path: %q", got)
		}
		if got == "" {
			t.Fatal("redaction must keep an error signal")
		}
	}
	plain := RepoSnapshot{Path: repo, Error: "x " + repo}
	if maybeRedactRepoSnapshot(plain, false).Error != plain.Error {
		t.Fatal("non-redacted publish must keep Error verbatim")
	}
}
