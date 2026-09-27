## 1. Shared State Data Model

- [x] 1.1 Define machine snapshot schema (machine id, updated timestamp, repo statuses, scan metadata).
- [x] 1.2 Add JSON read/write helpers for per-machine files in the state repo.
- [x] 1.3 Add aggregate loader that reads all machine snapshots and validates/parses with graceful error handling.

## 2. Git Sync Engine

- [x] 2.1 Implement pull/commit/rebase/push workflow for state repo with bounded retries.
- [x] 2.2 Implement change detection so commits are created only when local machine state file changed.
- [x] 2.3 Add offline-tolerant error handling and warnings (non-fatal loop behavior).

## 3. Agent Mode

- [x] 3.1 Add `agent` mode loop with configurable interval and state repo location.
- [x] 3.2 Add single-instance lock to prevent duplicate concurrent agent loops per machine.
- [x] 3.3 Wire scan + snapshot write + sync flow into each loop tick.

## 4. Scheduling Integration

- [x] 4.1 Add Windows scheduler install/uninstall commands (Task Scheduler, logon + repeat interval).
- [x] 4.2 Add Linux scheduler install/uninstall commands (systemd user service + timer).
- [x] 4.3 Add docs and safety checks for scheduler setup prerequisites.

## 5. CLI Aggregate View

- [x] 5.1 Add command/flag flow that loads all machine snapshots during normal runs.
- [x] 5.2 Add stale-state detection with configurable TTL and clear status labeling.
- [x] 5.3 Merge and display local + remote machine views clearly in table/summary output.

## 6. Security and Privacy Guardrails

- [x] 6.1 Document private-repository requirement and metadata exposure risks.
- [x] 6.2 Add optional redaction mode for path display in shared snapshots (future-compatible switch).
- [x] 6.3 Ensure logs avoid printing sensitive state content by default.

## 7. Validation

- [x] 7.1 Add tests for snapshot parsing, staleness, and merge behavior.
- [x] 7.2 Add tests for git-sync retry flow using controlled command failures.
- [x] 7.3 Run `go build ./...` and verify no regression in existing scan behavior.
