## Context

The tool currently reports repository status on a single machine when run manually. The primary user risk is creating incompatible work on one machine while forgetting uncommitted changes exist on another machine.

A practical solution should avoid standing up a custom service and work on Windows and Linux first. A shared Git repository can act as a lightweight transport and durable history store for machine snapshots.

## Goals / Non-Goals

**Goals:**
- Publish machine state automatically without requiring manual command runs.
- Show cross-machine latest known state on any local run.
- Keep merge/conflict risk low.
- Work on Windows and Linux first.
- Tolerate intermittent network availability.

**Non-Goals:**
- Real-time by-second synchronization.
- Public multi-tenant service or complex backend infrastructure.
- Full secret management solution beyond private-repo usage guidance.
- macOS scheduler integration in the first iteration.

## Decisions

- Use a dedicated Git state repository as the sync bus.
- Store one JSON file per machine, named by machine ID.
- Agent loop runs periodically (default 30 seconds) and on startup.
- Sync sequence each tick:
  1. Pull latest state repo.
  2. Scan local repositories.
  3. Write local machine snapshot file.
  4. Commit only when content changed.
  5. Pull --rebase and push with bounded retries.
- On CLI runs, load all machine snapshot files and mark stale entries using a configurable TTL.
- Use scheduler integrations for autonomous operation:
  - Windows: Task Scheduler task repeating every interval.
  - Linux: systemd user service/timer.
- Use single-machine ownership of each state file to minimize write contention.

## Risks / Trade-offs

- Risk: stale data when a machine is powered off or offline.
  - Mitigation: explicit stale indicators in output using last-updated timestamp.
- Risk: metadata leakage (repo paths, branch names).
  - Mitigation: private repository requirement, optional path redaction strategy later.
- Risk: Git push races between machines.
  - Mitigation: one-file-per-machine model plus pull-rebase-push retry logic.
- Trade-off: eventual consistency over real-time correctness.
  - Reasonable for this use case because frequent periodic updates are sufficient.
