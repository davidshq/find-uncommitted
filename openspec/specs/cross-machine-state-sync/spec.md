# cross-machine-state-sync Specification

## Purpose

Synchronize uncommitted repository status across machines using shared Git-backed per-machine snapshots with freshness indicators.

## Requirements

### Requirement: Automatic machine state publishing
The system SHALL support an autonomous background mode that periodically scans local repositories and publishes a machine snapshot to shared state storage without requiring manual command invocation.

#### Scenario: Periodic publish tick
- **WHEN** agent mode is running and the configured interval elapses
- **THEN** the system runs a local repository scan and writes an updated machine snapshot

#### Scenario: Startup publish
- **WHEN** agent mode starts
- **THEN** the system performs an immediate publish attempt before entering periodic waits

### Requirement: Cross-machine aggregate visibility
The CLI SHALL load latest available snapshots from all machines in shared state storage and present a combined view during normal runs when a state repository is resolved from an explicit flag, environment variable, or sticky user config, unless remote loading is disabled.

#### Scenario: Aggregate display includes remote machine state
- **WHEN** multiple machine snapshot files exist in shared storage and remote loading is enabled
- **THEN** command output includes status entries from each machine snapshot

#### Scenario: Aggregate from sticky config without flag
- **WHEN** `state_repo` is resolved from sticky user config (no `--state-repo` flag) and `--no-remote` is unset
- **THEN** the CLI loads shared snapshots and presents the combined aggregate view

#### Scenario: Remote loading disabled
- **WHEN** the user passes `--no-remote`
- **THEN** the CLI does not load other machines' snapshots even if a state repository is configured

### Requirement: Origin-based project identity in snapshots
Each published repository entry SHALL include a normalized `origin` remote URL when the repository has an `origin` remote, so the same project can be correlated across machines despite differing local filesystem paths. SSH and HTTPS forms of the same remote SHALL canonicalize to the same identity. Repositories without an `origin` remote MAY omit the field; aggregate sorting SHALL fall back to path basename in that case.

#### Scenario: Snapshot carries normalized origin
- **WHEN** a scanned repository has `remote.origin.url` set
- **THEN** the published snapshot entry includes a normalized `origin` field (for example both `git@github.com:acme/app.git` and `https://github.com/acme/app.git` become `github.com/acme/app`)

#### Scenario: Aggregate groups by origin across machines
- **WHEN** two machines publish snapshots for clones of the same origin at different local paths
- **THEN** the aggregate view sorts those entries adjacent by shared origin identity

#### Scenario: Redacted origin still correlates
- **WHEN** `--redact-paths` is enabled and a repository has an origin
- **THEN** the published `origin` is a stable hash of the normalized URL so machines can still correlate without exposing the raw remote

### Requirement: Snapshot freshness signaling
The CLI SHALL mark machine snapshots as stale when their last update time exceeds a configurable staleness threshold.

#### Scenario: Snapshot exceeds threshold
- **WHEN** a machine snapshot timestamp is older than the configured stale threshold
- **THEN** the machine is labeled stale in output

### Requirement: Conflict-minimized state writes
Each machine SHALL write only its own snapshot file in shared storage to reduce multi-writer conflicts.

#### Scenario: Concurrent machine updates
- **WHEN** two machines publish around the same time
- **THEN** each machine updates only its own file path and sync retries resolve transient push races

### Requirement: Offline-tolerant sync behavior
The background publisher SHALL continue operating when network or remote Git access is unavailable and retry on subsequent cycles.

#### Scenario: Remote unavailable
- **WHEN** pull or push fails due to connectivity or remote access error
- **THEN** the agent logs a non-fatal warning and retries on a later cycle

### Requirement: Offline-tolerant interactive aggregate loading
Interactive runs that attempt to load shared state SHALL tolerate pull failures without aborting the local scan when the state repository path itself is valid. Missing or invalid state repository paths SHALL fail loudly when remote loading is enabled. Individual corrupt snapshot files SHALL be skipped with a warning so valid sibling snapshots still appear in the aggregate.

#### Scenario: Pull failure during interactive aggregate
- **WHEN** a valid state repository is configured for an interactive run and `git pull` fails
- **THEN** the CLI warns, attempts to use on-disk snapshots if present, and still shows local scan results

#### Scenario: Invalid state repository path
- **WHEN** remote loading is enabled and the configured state repository path is missing or not a git repository
- **THEN** the CLI exits with an error instead of silently showing a local-only table

#### Scenario: Corrupt snapshot file among valid siblings
- **WHEN** one machine snapshot JSON file is corrupt and others are valid
- **THEN** the CLI warns about the corrupt file, skips it, and still displays aggregate rows from valid snapshots
