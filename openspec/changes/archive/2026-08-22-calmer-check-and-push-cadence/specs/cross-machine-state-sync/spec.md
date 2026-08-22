## ADDED Requirements

### Requirement: Configurable liveness heartbeat
When published snapshot content is unchanged, the system SHALL commit and push a freshness update only when the time since the last published `updated_at` meets or exceeds the configured heartbeat duration (default `15m` when unset). Skipping an unchanged tick MUST NOT rewrite `updated_at` on disk. When snapshot content differs from the last publish, the system SHALL commit and push on that check tick.

#### Scenario: Heartbeat due while unchanged
- **WHEN** a check tick finds snapshot content equal to the last publish and the last `updated_at` is older than the configured heartbeat
- **THEN** the system commits and pushes an updated snapshot that refreshes `updated_at`

#### Scenario: Heartbeat not due while unchanged
- **WHEN** a check tick finds snapshot content equal to the last publish and the last `updated_at` is within the heartbeat window
- **THEN** the system does not create a new commit for that tick

#### Scenario: Immediate publish on content change
- **WHEN** a check tick detects snapshot content different from the last publish
- **THEN** the system commits and pushes the new snapshot on that tick

## MODIFIED Requirements

### Requirement: Automatic machine state publishing
The system SHALL support an autonomous background mode that periodically scans local repositories on the configured check interval and publishes per the heartbeat policy without requiring manual command invocation. The default check interval when unset SHALL be `2m`.

#### Scenario: Periodic publish tick
- **WHEN** agent mode is running and the configured check interval elapses
- **THEN** the system runs a local repository scan and evaluates whether to write/commit a machine snapshot (content change or heartbeat due)

#### Scenario: Startup publish
- **WHEN** agent mode starts
- **THEN** the system performs an immediate publish attempt before entering periodic waits

#### Scenario: Unchanged status skips commit
- **WHEN** a check tick finds snapshot content unchanged and the heartbeat window is not due
- **THEN** the system does not create a commit solely because the check interval elapsed

### Requirement: Snapshot freshness signaling
The CLI SHALL mark machine snapshots as stale when their last update time exceeds a configurable staleness threshold. When the threshold is unset, the built-in default SHALL be `30m`.

#### Scenario: Snapshot exceeds threshold
- **WHEN** a machine snapshot timestamp is older than the configured stale threshold
- **THEN** the machine is labeled stale in output

#### Scenario: Unset threshold uses calm default
- **WHEN** no stale threshold is configured
- **THEN** snapshots older than 30 minutes are labeled stale
