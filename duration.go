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
