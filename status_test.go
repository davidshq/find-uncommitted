package main

import "testing"

func TestRepoStatusText(t *testing.T) {
	tests := []struct {
		name       string
		repo       RepoSnapshot
		plain      bool
		wantStatus string
		wantChange string
	}{
		{
			name:       "error emoji",
			repo:       RepoSnapshot{Error: "not a git repo"},
			plain:      false,
			wantStatus: "❌ Error",
			wantChange: "not a git repo",
		},
		{
			name:       "error plain",
			repo:       RepoSnapshot{Error: "not a git repo"},
			plain:      true,
			wantStatus: "Error",
			wantChange: "not a git repo",
		},
		{
			name:       "dirty emoji",
			repo:       RepoSnapshot{IsDirty: true, HasUnstaged: true, HasStaged: true},
			plain:      false,
			wantStatus: "⚠️  Dirty",
			wantChange: "unstaged, staged",
		},
		{
			name:       "dirty plain",
			repo:       RepoSnapshot{IsDirty: true, HasUntracked: true},
			plain:      true,
			wantStatus: "Dirty",
			wantChange: "untracked",
		},
		{
			name:       "untracked upstream emoji",
			repo:       RepoSnapshot{HasUntrackedUpstream: true},
			plain:      false,
			wantStatus: "🔗 Untracked Upstream",
			wantChange: "untracked-upstream",
		},
		{
			name:       "untracked upstream plain",
			repo:       RepoSnapshot{HasUntrackedUpstream: true},
			plain:      true,
			wantStatus: "UntrackedUpstream",
			wantChange: "untracked-upstream",
		},
		{
			name:       "diverged emoji",
			repo:       RepoSnapshot{HasBehind: true, HasUnpushed: true, AheadCount: 2, BehindCount: 3},
			plain:      false,
			wantStatus: "↕️ Diverged",
			wantChange: "unpushed:2, behind:3",
		},
		{
			name:       "diverged plain",
			repo:       RepoSnapshot{HasBehind: true, HasUnpushed: true, AheadCount: 1, BehindCount: 1},
			plain:      true,
			wantStatus: "Diverged",
			wantChange: "unpushed:1, behind:1",
		},
		{
			name:       "behind emoji",
			repo:       RepoSnapshot{HasBehind: true, BehindCount: 4},
			plain:      false,
			wantStatus: "⬇️ Behind",
			wantChange: "behind:4",
		},
		{
			name:       "unpushed plain",
			repo:       RepoSnapshot{HasUnpushed: true, AheadCount: 5},
			plain:      true,
			wantStatus: "Unpushed",
			wantChange: "unpushed:5",
		},
		{
			name:       "clean emoji",
			repo:       RepoSnapshot{IsClean: true},
			plain:      false,
			wantStatus: "✅ Clean",
			wantChange: "-",
		},
		{
			name:       "clean plain",
			repo:       RepoSnapshot{IsClean: true},
			plain:      true,
			wantStatus: "Clean",
			wantChange: "-",
		},
		{
			name:       "empty emoji",
			repo:       RepoSnapshot{IsEmpty: true},
			plain:      false,
			wantStatus: "📭 Empty",
			wantChange: "no commits yet",
		},
		{
			name:       "empty plain",
			repo:       RepoSnapshot{IsEmpty: true},
			plain:      true,
			wantStatus: "Empty",
			wantChange: "no commits yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotChange := repoStatusText(tt.repo, tt.plain)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if gotChange != tt.wantChange {
				t.Fatalf("changes = %q, want %q", gotChange, tt.wantChange)
			}
		})
	}
}

func TestNeedsAttentionEquivalent(t *testing.T) {
	cases := []struct {
		name   string
		status RepoStatus
		snap   RepoSnapshot
		want   bool
	}{
		{
			name:   "clean",
			status: RepoStatus{IsClean: true},
			snap:   RepoSnapshot{IsClean: true},
			want:   false,
		},
		{
			name:   "dirty",
			status: RepoStatus{IsDirty: true},
			snap:   RepoSnapshot{IsDirty: true},
			want:   true,
		},
		{
			name:   "behind",
			status: RepoStatus{HasBehind: true},
			snap:   RepoSnapshot{HasBehind: true, BehindCount: 1},
			want:   true,
		},
		{
			name:   "error",
			status: RepoStatus{Error: "boom"},
			snap:   RepoSnapshot{Error: "boom"},
			want:   true,
		},
		{
			name:   "empty",
			status: RepoStatus{IsEmpty: true},
			snap:   RepoSnapshot{IsEmpty: true},
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := repoNeedsAttention(tc.status); got != tc.want {
				t.Fatalf("repoNeedsAttention = %v, want %v", got, tc.want)
			}
			if got := snapshotNeedsAttention(tc.snap); got != tc.want {
				t.Fatalf("snapshotNeedsAttention = %v, want %v", got, tc.want)
			}
		})
	}
}
