## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Offline-tolerant interactive aggregate loading
Interactive runs that attempt to load shared state SHALL tolerate missing clones and pull failures without aborting the local scan.

#### Scenario: Pull failure during interactive aggregate
- **WHEN** a state repository is configured for an interactive run and `git pull` fails
- **THEN** the CLI warns, attempts to use on-disk snapshots if present, and still shows local scan results
