package main

import (
	"fmt"
	"time"
)

// Duration defaults are defined once as Go duration strings; typed values are
// derived so flags, sticky config, and runtime cannot drift.
// Per-command git timeouts live in internal/gitexec.

const (
	DefaultIntervalString    = "2m"
	DefaultHeartbeatString   = "15m"
	DefaultStaleTTLString    = "30m"
	DefaultTickTimeoutString = "2m"
)

var (
	// DefaultAgentInterval is the default check cadence (scan + publish decision).
	DefaultAgentInterval = mustDuration(DefaultIntervalString)

	// DefaultHeartbeat is the liveness commit interval when snapshot content is unchanged.
	DefaultHeartbeat = mustDuration(DefaultHeartbeatString)

	// DefaultStaleTTL marks remote snapshots stale when older than this.
	// Kept at roughly 2× DefaultHeartbeat so a quiet healthy agent is not marked stale.
	DefaultStaleTTL = mustDuration(DefaultStaleTTLString)

	// DefaultAgentTickTimeout bounds one agent publish tick (pull + scan + publish).
	DefaultAgentTickTimeout = mustDuration(DefaultTickTimeoutString)
)

func mustDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic("invalid default duration " + s + ": " + err.Error())
	}
	return d
}

func parseDurationFlag(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("Invalid %s: %v", name, err)
	}
	return d, nil
}

func parsePositiveDurationFlag(name, value string) (time.Duration, error) {
	d, err := parseDurationFlag(name, value)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("Invalid %s: must be positive", name)
	}
	return d, nil
}

// formatLocalWallClock renders t in the local zone as YYYY-MM-DD HH:MM:SS.
func formatLocalWallClock(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}

// formatCompactAge renders a short relative age (e.g. "45s", "12m", "3h20m", "2d").
func formatCompactAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	d = d.Truncate(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// formatPublishedSuffix annotates a remote snapshot with wall-clock + relative age.
// Returns "" when updatedAt is zero (live local rows).
func formatPublishedSuffix(updatedAt time.Time, now time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf(" · published %s (%s ago)",
		formatLocalWallClock(updatedAt), formatCompactAge(now.Sub(updatedAt)))
}

// formatCheckedSuffix annotates a live local row with this check’s wall-clock time.
func formatCheckedSuffix(checkedAt time.Time) string {
	if checkedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf(" · checked %s", formatLocalWallClock(checkedAt))
}
