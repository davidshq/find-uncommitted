package main

import (
	"fmt"
	"time"
)

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
