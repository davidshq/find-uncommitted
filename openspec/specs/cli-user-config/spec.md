# cli-user-config Specification

## Purpose

Persist and resolve sticky user settings (TOML config path, precedence, write-on-install) shared by the interactive CLI and background agent.

## Requirements

### Requirement: User-scoped TOML config resolution
The system SHALL resolve sticky settings from a user-scoped TOML config file at the platform config path (`$XDG_CONFIG_HOME/find-uncommitted/config.toml` or `~/.config/find-uncommitted/config.toml` on Unix; `%AppData%\find-uncommitted\config.toml` on Windows).

#### Scenario: Config file present
- **WHEN** the config file exists and contains a valid `state_repo` path
- **THEN** the system loads that value for use when the corresponding flag is unset

#### Scenario: Config file absent
- **WHEN** the config file does not exist
- **THEN** the system behaves as today for unset flags (local-only interactive scan; agent/scheduler still require explicit `--state-repo`)

### Requirement: Configuration precedence
The system SHALL apply settings in this order: explicit CLI flags, then environment variables, then the TOML config file, then built-in defaults.

#### Scenario: Flag overrides config
- **WHEN** both `--state-repo` and a config `state_repo` are available
- **THEN** the flag value is used

#### Scenario: Env overrides config
- **WHEN** `FIND_UNCOMMITTED_STATE_REPO` is set and no `--state-repo` flag is passed
- **THEN** the environment value is used instead of the config file value

### Requirement: Heartbeat sticky setting
The system SHALL resolve an optional sticky setting `heartbeat` (duration string) from the user TOML config with the same precedence as other sticky string settings: explicit CLI flags (if any), then environment variables (if defined for this key), then the TOML config file, then the built-in default. When `heartbeat` is unset, the built-in default SHALL be `15m`. The system SHALL persist `heartbeat` when writing sticky config from `--install-scheduler` or first-time agent config creation.

#### Scenario: Heartbeat from config
- **WHEN** the config file contains `heartbeat = "30m"` and no higher-precedence override is set
- **THEN** the agent uses a 30-minute liveness heartbeat

#### Scenario: Heartbeat flag overrides config
- **WHEN** the user passes `--heartbeat 10m` and the config file contains `heartbeat = "30m"`
- **THEN** the agent uses a 10-minute liveness heartbeat

#### Scenario: Unset heartbeat uses calm default
- **WHEN** no flag, env, or config provides `heartbeat`
- **THEN** the effective heartbeat is 15 minutes

#### Scenario: Install writes heartbeat
- **WHEN** `--install-scheduler` succeeds
- **THEN** the sticky config includes a `heartbeat` value

### Requirement: Calmer default check interval and stale TTL
When `interval` is unset (no flag, env, or config value), the system SHALL default the agent check interval to `2m`. When `stale_ttl` is unset, the system SHALL default the staleness threshold to `30m`. Explicit sticky, env, or flag values SHALL override these defaults.

#### Scenario: Unset interval uses calm default
- **WHEN** agent mode starts with no resolved `interval`
- **THEN** the check ticker uses a 2-minute interval

#### Scenario: Explicit aggressive interval preserved
- **WHEN** config or `--interval` sets `interval` to `30s`
- **THEN** the agent checks every 30 seconds

#### Scenario: Unset stale_ttl uses calm default
- **WHEN** no flag, env, or config provides `stale_ttl`
- **THEN** the effective stale threshold is 30 minutes

### Requirement: Stale TTL vs heartbeat warning
When a state repository is in use and both `heartbeat` and `stale_ttl` resolve to positive durations, the system SHALL emit a stderr warning if `stale_ttl` is less than 2× `heartbeat`, because healthy quiet agents may be labeled stale between liveness commits.

#### Scenario: Mismatched stale_ttl warns
- **WHEN** agent or interactive mode uses a state repo with `heartbeat = 15m` and `stale_ttl = 5m`
- **THEN** stderr includes a warning recommending `stale_ttl >= 30m`

#### Scenario: Balanced cadence is silent
- **WHEN** `stale_ttl` is at least 2× `heartbeat`
- **THEN** no stale/heartbeat mismatch warning is printed

### Requirement: Install and agent persist config
The system SHALL write or update the user TOML config when `--install-scheduler` succeeds, including at least `state_repo` and `scan_root` from the install invocation, plus the resolved `interval`, `stale_ttl`, and `heartbeat` values (including built-in defaults when those knobs were unset). Agent mode SHALL create the config file if it is missing and `--state-repo` was provided, including the same cadence fields when available.

#### Scenario: Install writes config
- **WHEN** the user runs `--install-scheduler` with a state repo and scan root
- **THEN** the config file is created or updated with those values before or as part of scheduler registration

#### Scenario: Install writes cadence knobs
- **WHEN** `--install-scheduler` succeeds
- **THEN** the sticky config includes `interval`, `stale_ttl`, and `heartbeat`

#### Scenario: Agent creates missing config
- **WHEN** agent mode starts with `--state-repo` and no config file exists
- **THEN** the system writes a config file containing at least that `state_repo` and the resolved cadence settings

### Requirement: Default aggregate when configured
When a `state_repo` is resolved from config or environment and `--no-remote` is not set, interactive scans SHALL pull shared state (fast-forward only when possible) and present the cross-machine aggregate view.

#### Scenario: Bare scan after install
- **WHEN** a valid sticky `state_repo` is configured and the user runs an interactive scan without `--state-repo` or `--no-remote`
- **THEN** the output includes remote machine snapshot rows using the aggregate view

#### Scenario: Opt out of remotes
- **WHEN** the user passes `--no-remote` while a sticky `state_repo` is configured
- **THEN** the scan remains local-only

### Requirement: Transparent config use and invalid-repo failures
When sticky config supplies `state_repo` for an interactive scan, the system SHALL emit a brief stderr notice identifying that config was used. If the configured state repo path is missing or invalid (not a usable git clone), the system SHALL fail the interactive scan with a non-zero exit instead of silently falling back to local-only output. Transient pull failures SHALL still warn and continue with local results and any readable on-disk snapshots. Unreadable or corrupt sticky config files SHALL also fail loudly rather than ignoring the file.

#### Scenario: Notice on config-backed aggregate
- **WHEN** interactive mode uses `state_repo` from the config file
- **THEN** stderr includes a short notice that config was used and that `--no-remote` forces local-only

#### Scenario: Invalid state repo from sticky config
- **WHEN** the configured `state_repo` path cannot be validated as a git repository and `--no-remote` is unset
- **THEN** the system prints an error and exits non-zero without presenting a local-only table as if remotes were intentionally disabled

#### Scenario: Offline pull with valid state repo
- **WHEN** a valid state repository is configured for an interactive run and `git pull` fails
- **THEN** the system prints a warning, attempts to use on-disk snapshots if present, and still shows local scan results

#### Scenario: Corrupt sticky config file
- **WHEN** the sticky config file exists but cannot be read or parsed
- **THEN** the system prints an error and exits non-zero

### Requirement: Post-install smoke publish
When `--install-scheduler` succeeds at writing sticky config, the system SHALL perform one publish attempt to the configured state repository and report the resulting snapshot path before (or as part of) completing install, so the operator can confirm a machine file landed. If that smoke publish fails, the system SHALL exit non-zero without treating the install as fully successful.

#### Scenario: Install smoke publish succeeds
- **WHEN** the user runs `--install-scheduler` with a valid state repo and scan root
- **THEN** a machine snapshot file is written under `machines/` and the CLI prints its path

#### Scenario: Install smoke publish fails
- **WHEN** smoke publish cannot write or push the machine snapshot during `--install-scheduler`
- **THEN** the system reports an error and does not claim a successful install completion