package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTickFailStreakEscalatesThenRepeatsThenRecovers(t *testing.T) {
	var s tickFailStreak
	start := time.Date(2026, 9, 29, 19, 23, 0, 0, time.UTC)
	fail := errors.New("state repo rebase before push failed: exit status 1: error: The following untracked working tree files would be overwritten by checkout:\n\t.gitignore\nPlease move or remove them")

	var escalations []int
	for i := 1; i <= agentStuckEscalateAfter+agentStuckRepeatEvery; i++ {
		if msg := s.record(fail, start.Add(time.Duration(i)*time.Minute)); msg != "" {
			escalations = append(escalations, i)
			if !strings.HasPrefix(msg, "ERROR: agent has failed") || strings.Contains(msg, "\n") || !strings.Contains(msg, ".gitignore Please move") {
				t.Fatalf("tick %d: unexpected message %q", i, msg)
			}
		}
	}
	want := []int{agentStuckEscalateAfter, agentStuckEscalateAfter + agentStuckRepeatEvery}
	if len(escalations) != len(want) || escalations[0] != want[0] || escalations[1] != want[1] {
		t.Fatalf("escalated at ticks %v, want %v", escalations, want)
	}

	msg := s.record(nil, start.Add(2*time.Hour))
	if !strings.Contains(msg, "recovered after 65 consecutive failed ticks") {
		t.Fatalf("recovery message: %q", msg)
	}
	if msg := s.record(nil, start.Add(3*time.Hour)); msg != "" {
		t.Fatalf("healthy tick should be quiet, got %q", msg)
	}
}

func TestTickFailStreakQuietBelowThreshold(t *testing.T) {
	var s tickFailStreak
	for i := 1; i < agentStuckEscalateAfter; i++ {
		if msg := s.record(errors.New("offline"), time.Now()); msg != "" {
			t.Fatalf("tick %d escalated early: %q", i, msg)
		}
	}
}

func TestOneLineCapsLength(t *testing.T) {
	got := oneLine(strings.Repeat("x", 500), 300)
	if len([]rune(got)) != 302 || !strings.HasSuffix(got, " …") {
		t.Fatalf("not capped: len=%d", len([]rune(got)))
	}
}
