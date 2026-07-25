package main

import (
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

	localOnly := RepoSnapshot{Path: "/home/a/manuscripts/book"}
	otherLocal := RepoSnapshot{Path: "/Users/b/docs/book"}
	if repoCorrelationKey(localOnly) != repoCorrelationKey(otherLocal) {
		t.Fatal("basename fallback should correlate local-only repos with same folder name")
	}

	if repoCorrelationKey(withOrigin) == repoCorrelationKey(localOnly) {
		t.Fatal("origin key must not collide with basename key")
	}
}

func TestRepoStatusToSnapshotIncludesOrigin(t *testing.T) {
	status := RepoStatus{
		Path:   "/code/app",
		Origin: "github.com/acme/app",
		Branch: "main",
		IsClean: true,
	}
	snap := RepoStatusToSnapshot(status, false)
	if snap.Origin != status.Origin {
		t.Fatalf("origin not copied: %q", snap.Origin)
	}
	redacted := RepoStatusToSnapshot(status, true)
	if redacted.Origin == status.Origin || redacted.Origin == "" {
		t.Fatalf("expected hashed origin when redacting, got %q", redacted.Origin)
	}
	if redacted.Origin != redactOrigin(status.Origin) {
		t.Fatalf("redacted origin mismatch: %q", redacted.Origin)
	}
}
