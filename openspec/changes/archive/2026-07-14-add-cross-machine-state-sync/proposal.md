## Why

Developers working across multiple machines can forget about uncommitted work left on another machine and start conflicting changes elsewhere. Manual command runs do not eliminate this risk because the failure mode is forgetting to run the command.

This change adds automatic, frequent state publishing from each machine and aggregate visibility across machines so latest known uncommitted state is always available.

## What Changes

- Add autonomous background syncing for machine snapshot files representing repository status.
- Add background agent scheduling on Windows (Task Scheduler) and Linux (systemd user timer).
- Use one-file-per-machine storage in a shared Git state repository to minimize conflicts.
- Add stale-state detection for machines that have not reported recently.
- Add privacy-oriented guidance and defaults for shared metadata.

## Capabilities

### New Capabilities
- cross-machine-state-sync: Synchronize uncommitted repo status across machines using shared Git-backed snapshots with freshness indicators.

### Modified Capabilities
- repository-scan: Extend output to include aggregate state from other machines in addition to the local scan.

## Impact

- Affected code paths: CLI flags/commands, scan result rendering, and new sync/scheduler modules.
- New operational requirements: private shared state repository and local scheduler setup.
- Behavioral change: each machine publishes state automatically; manual runs consume aggregate state and highlight stale entries.
- Security posture: shared metadata can include repo identity details; defaults and docs must emphasize private repo usage.
