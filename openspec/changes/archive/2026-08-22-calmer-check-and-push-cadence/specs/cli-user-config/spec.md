## ADDED Requirements

### Requirement: Heartbeat sticky setting
The system SHALL resolve an optional sticky setting `heartbeat` (duration string) from the user TOML config with the same precedence as other sticky string settings: explicit CLI flags (if any), then environment variables (if defined for this key), then the TOML config file, then the built-in default. When `heartbeat` is unset, the built-in default SHALL be `15m`. The system SHALL persist `heartbeat` when writing sticky config from `--install-scheduler` or first-time agent config creation.

#### Scenario: Heartbeat from config
- **WHEN** the config file contains `heartbeat = "30m"` and no higher-precedence override is set
- **THEN** the agent uses a 30-minute liveness heartbeat

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

## MODIFIED Requirements

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
