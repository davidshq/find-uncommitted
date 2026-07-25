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

### Requirement: Install and agent persist config
The system SHALL write or update the user TOML config when `--install-scheduler` succeeds, including at least `state_repo` and `scan_root` from the install invocation. Agent mode SHALL create the config file if it is missing and `--state-repo` was provided.

#### Scenario: Install writes config
- **WHEN** the user runs `--install-scheduler` with `--state-repo` and a scan root
- **THEN** the config file is created or updated with those values before or as part of scheduler registration

#### Scenario: Agent creates missing config
- **WHEN** agent mode starts with `--state-repo` and no config file exists
- **THEN** the system writes a config file containing at least that `state_repo`

### Requirement: Default aggregate when configured
When a `state_repo` is resolved from config or environment and `--no-remote` is not set, interactive scans SHALL pull shared state (fast-forward only when possible) and present the cross-machine aggregate view.

#### Scenario: Bare scan after install
- **WHEN** a valid sticky `state_repo` is configured and the user runs an interactive scan without `--state-repo` or `--no-remote`
- **THEN** the output includes remote machine snapshot rows using the aggregate view

#### Scenario: Opt out of remotes
- **WHEN** the user passes `--no-remote` while a sticky `state_repo` is configured
- **THEN** the scan remains local-only

### Requirement: Transparent config use and degraded failures
When sticky config supplies `state_repo` for an interactive scan, the system SHALL emit a brief stderr notice identifying that config was used. If the configured state repo is missing, invalid, or pull fails, the system SHALL warn and continue with local results (and any readable on-disk snapshots) without failing the local scan.

#### Scenario: Notice on config-backed aggregate
- **WHEN** interactive mode uses `state_repo` from the config file
- **THEN** stderr includes a short notice that config was used and that `--no-remote` forces local-only

#### Scenario: Offline or invalid state repo
- **WHEN** the configured state repo cannot be validated or pulled
- **THEN** the system prints a warning and still displays local scan results
