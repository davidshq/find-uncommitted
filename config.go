package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	configAppDirName = "find-uncommitted"
	configFileName   = "config.toml"

	envStateRepo   = "FIND_UNCOMMITTED_STATE_REPO"
	envScanRoot    = "FIND_UNCOMMITTED_SCAN_ROOT"
	envMachineID   = "FIND_UNCOMMITTED_MACHINE_ID"
	envInterval    = "FIND_UNCOMMITTED_INTERVAL"
	envHeartbeat   = "FIND_UNCOMMITTED_HEARTBEAT"
	envStaleTTL    = "FIND_UNCOMMITTED_STALE_TTL"
	envRedactPaths = "FIND_UNCOMMITTED_REDACT_PATHS"
	envMaxWorkers  = "FIND_UNCOMMITTED_MAX_WORKERS"
)

// UserConfig is the sticky TOML settings shared by CLI and agent.
type UserConfig struct {
	StateRepo   string `toml:"state_repo"`
	ScanRoot    string `toml:"scan_root,omitempty"`
	MachineID   string `toml:"machine_id,omitempty"`
	Interval    string `toml:"interval,omitempty"`
	Heartbeat   string `toml:"heartbeat,omitempty"`
	StaleTTL    string `toml:"stale_ttl,omitempty"`
	RedactPaths bool   `toml:"redact_paths,omitempty"`
	MaxWorkers  int    `toml:"max_workers,omitempty"`
}

// ConfigSource identifies where a resolved value came from.
type ConfigSource string

const (
	SourceNone   ConfigSource = ""
	SourceFlag   ConfigSource = "flag"
	SourceEnv    ConfigSource = "env"
	SourceConfig ConfigSource = "config"
)

// FlagOverrides captures CLI values and whether each flag was explicitly set.
type FlagOverrides struct {
	StateRepo      string
	StateRepoSet   bool
	ScanRoot       string
	ScanRootSet    bool
	MachineID      string
	MachineIDSet   bool
	Interval       string
	IntervalSet    bool
	Heartbeat      string
	HeartbeatSet   bool
	StaleTTL       string
	StaleTTLSet    bool
	RedactPaths    bool
	RedactPathsSet bool
	MaxWorkers     int
	MaxWorkersSet  bool
}

// ResolvedSettings is the effective configuration after precedence resolution.
type ResolvedSettings struct {
	StateRepo         string
	StateRepoSource   ConfigSource
	ScanRoot          string
	ScanRootSource    ConfigSource
	MachineID         string
	MachineIDSource   ConfigSource
	Interval          string
	IntervalSource    ConfigSource
	Heartbeat         string
	HeartbeatSource   ConfigSource
	StaleTTL          string
	StaleTTLSource    ConfigSource
	RedactPaths       bool
	RedactPathsSource ConfigSource
	MaxWorkers        int
	MaxWorkersSource  ConfigSource
}

// DefaultConfigPath returns the platform user config file path.
// Unix: $XDG_CONFIG_HOME/find-uncommitted/config.toml (via os.UserConfigDir).
// Windows: %AppData%\find-uncommitted\config.toml.
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, configAppDirName, configFileName), nil
}

// LoadUserConfig reads the TOML config. Missing file returns an empty config and nil error.
func LoadUserConfig(path string) (UserConfig, error) {
	var cfg UserConfig
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

// SaveUserConfig writes the TOML config, creating parent directories as needed.
func SaveUserConfig(path string, cfg UserConfig) error {
	if path == "" {
		return fmt.Errorf("config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config %q: %w", path, err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode config %q: %w", path, err)
	}
	return nil
}

// ConfigFileExists reports whether path exists as a regular file.
func ConfigFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ResolveSettings applies precedence: flags > env > config file > defaults (empty).
func ResolveSettings(flags FlagOverrides, file UserConfig, getenv func(string) string) ResolvedSettings {
	if getenv == nil {
		getenv = os.Getenv
	}
	var out ResolvedSettings

	out.StateRepo, out.StateRepoSource = resolveString(flags.StateRepo, flags.StateRepoSet, getenv(envStateRepo), file.StateRepo)
	out.ScanRoot, out.ScanRootSource = resolveString(flags.ScanRoot, flags.ScanRootSet, getenv(envScanRoot), file.ScanRoot)
	out.MachineID, out.MachineIDSource = resolveString(flags.MachineID, flags.MachineIDSet, getenv(envMachineID), file.MachineID)
	out.Interval, out.IntervalSource = resolveString(flags.Interval, flags.IntervalSet, getenv(envInterval), file.Interval)
	out.Heartbeat, out.HeartbeatSource = resolveString(flags.Heartbeat, flags.HeartbeatSet, getenv(envHeartbeat), file.Heartbeat)
	out.StaleTTL, out.StaleTTLSource = resolveString(flags.StaleTTL, flags.StaleTTLSet, getenv(envStaleTTL), file.StaleTTL)

	if flags.RedactPathsSet {
		out.RedactPaths = flags.RedactPaths
		out.RedactPathsSource = SourceFlag
	} else if v, ok := parseBoolEnv(getenv(envRedactPaths)); ok {
		out.RedactPaths = v
		out.RedactPathsSource = SourceEnv
	} else if file.RedactPaths {
		out.RedactPaths = true
		out.RedactPathsSource = SourceConfig
	}

	out.MaxWorkers, out.MaxWorkersSource = resolveInt(flags.MaxWorkers, flags.MaxWorkersSet, getenv(envMaxWorkers), file.MaxWorkers)

	return out
}

func resolveString(flagVal string, flagSet bool, envVal, fileVal string) (string, ConfigSource) {
	if flagSet && strings.TrimSpace(flagVal) != "" {
		return flagVal, SourceFlag
	}
	if flagSet && strings.TrimSpace(flagVal) == "" {
		// Explicit empty flag wins over env/config (clears sticky value for this run).
		return "", SourceFlag
	}
	if env := strings.TrimSpace(envVal); env != "" {
		return env, SourceEnv
	}
	if file := strings.TrimSpace(fileVal); file != "" {
		return file, SourceConfig
	}
	return "", SourceNone
}

func resolveInt(flagVal int, flagSet bool, envVal string, fileVal int) (int, ConfigSource) {
	if flagSet && flagVal > 0 {
		return flagVal, SourceFlag
	}
	if env := strings.TrimSpace(envVal); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			return n, SourceEnv
		}
	}
	if fileVal > 0 {
		return fileVal, SourceConfig
	}
	return 0, SourceNone
}

func parseBoolEnv(v string) (bool, bool) {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return false, false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, false
	}
	return b, true
}

// GenerateStableMachineID returns a hostname-based id with a random suffix so cloned VMs
// do not silently share the same machine_id.
func GenerateStableMachineID(hostname string) string {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		suffix = sha256Suffix(hostname)
	}
	host := sanitizeMachineID(strings.TrimSpace(hostname))
	if host == "" || host == "unknown" {
		host = "machine"
	}
	return fmt.Sprintf("%s-%x", host, suffix)
}

func sha256Suffix(s string) [4]byte {
	sum := sha256.Sum256([]byte(s))
	var out [4]byte
	copy(out[:], sum[:4])
	return out
}

// EnsureMachineIDInConfig writes machine_id when sticky config exists but the field is empty.
func EnsureMachineIDInConfig(path, machineID string) error {
	if path == "" || machineID == "" || !ConfigFileExists(path) {
		return nil
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.MachineID) != "" {
		return nil
	}
	cfg.MachineID = machineID
	return SaveUserConfig(path, cfg)
}

// EnsureConfigFromAgent writes a sticky config if missing when agent has an explicit state repo.
func EnsureConfigFromAgent(path string, stateRepo, scanRoot, machineID, interval, heartbeat, staleTTL string, redactPaths bool) error {
	if path == "" || stateRepo == "" {
		return nil
	}
	if ConfigFileExists(path) {
		return nil
	}
	return SaveUserConfig(path, UserConfig{
		StateRepo:   stateRepo,
		ScanRoot:    scanRoot,
		MachineID:   machineID,
		Interval:    interval,
		Heartbeat:   heartbeat,
		StaleTTL:    staleTTL,
		RedactPaths: redactPaths,
	})
}

// minimumStaleTTL is the recommended lower bound for stale_ttl (2× heartbeat).
func minimumStaleTTL(heartbeat time.Duration) time.Duration {
	if heartbeat <= 0 {
		return 0
	}
	return 2 * heartbeat
}

// staleTTLTooShort reports whether staleTTL is below 2× heartbeat.
func staleTTLTooShort(heartbeat, staleTTL time.Duration) bool {
	min := minimumStaleTTL(heartbeat)
	if min == 0 || staleTTL <= 0 {
		return false
	}
	return staleTTL < min
}

// warnStaleTTLMismatch logs when stale_ttl is likely to mark healthy agents stale.
func warnStaleTTLMismatch(heartbeat, staleTTL time.Duration) {
	if !staleTTLTooShort(heartbeat, staleTTL) {
		return
	}
	min := minimumStaleTTL(heartbeat)
	fmt.Fprintf(os.Stderr,
		"warning: stale_ttl (%s) is less than 2× heartbeat (%s); healthy machines may appear stale between heartbeats (consider stale_ttl >= %s)\n",
		staleTTL, heartbeat, min)
}
