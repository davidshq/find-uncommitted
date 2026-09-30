package main

import (
	"testing"
	"time"
)

func TestFormatCompactAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{12 * time.Minute, "12m"},
		{3*time.Hour + 20*time.Minute, "3h20m"},
		{5 * time.Hour, "5h"},
		{50 * time.Hour, "2d"},
	}
	for _, tc := range cases {
		if got := formatCompactAge(tc.d); got != tc.want {
			t.Fatalf("formatCompactAge(%v)=%q want %q", tc.d, got, tc.want)
		}
	}
}

func TestFormatPublishedSuffix(t *testing.T) {
	now := time.Date(2026, 9, 29, 15, 0, 0, 0, time.Local)
	updated := now.Add(-90 * time.Minute)
	got := formatPublishedSuffix(updated, now)
	want := " · published " + formatLocalWallClock(updated) + " (1h30m ago)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if formatPublishedSuffix(time.Time{}, now) != "" {
		t.Fatal("zero UpdatedAt should omit suffix")
	}
	checked := formatCheckedSuffix(now)
	wantChecked := " · checked " + formatLocalWallClock(now)
	if checked != wantChecked {
		t.Fatalf("checked suffix: got %q want %q", checked, wantChecked)
	}
	if formatCheckedSuffix(time.Time{}) != "" {
		t.Fatal("zero checkedAt should omit suffix")
	}
}
