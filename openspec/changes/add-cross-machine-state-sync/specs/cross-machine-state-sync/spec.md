## ADDED Requirements

### Requirement: Automatic machine state publishing
The system SHALL support an autonomous background mode that periodically scans local repositories and publishes a machine snapshot to shared state storage without requiring manual command invocation.

#### Scenario: Periodic publish tick
- **WHEN** agent mode is running and the configured interval elapses
- **THEN** the system runs a local repository scan and writes an updated machine snapshot

#### Scenario: Startup publish
- **WHEN** agent mode starts
- **THEN** the system performs an immediate publish attempt before entering periodic waits

### Requirement: Cross-machine aggregate visibility
The CLI SHALL load latest available snapshots from all machines in shared state storage and present a combined view during normal runs.

#### Scenario: Aggregate display includes remote machine state
- **WHEN** multiple machine snapshot files exist in shared storage
- **THEN** command output includes status entries from each machine snapshot

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
