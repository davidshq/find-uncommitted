## 1. Shared State Data Model

- [ ] 1.1 Define machine snapshot schema (machine id, updated timestamp, repo statuses, scan metadata).
- [ ] 1.2 Add JSON read/write helpers for per-machine files in the state repo.
- [ ] 1.3 Add aggregate loader that reads all machine snapshots and validates/parses with graceful error handling.

## 2. Git Sync Engine

- [ ] 2.1 Implement pull/commit/rebase/push workflow for state repo with bounded retries.
- [ ] 2.2 Implement change detection so commits are created only when local machine state file changed.
- [ ] 2.3 Add offline-tolerant error handling and warnings (non-fatal loop behavior).

## 3. Agent Mode

- [ ] 3.1 Add `agent` mode loop with configurable interval and state repo location.
- [ ] 3.2 Add single-instance lock to prevent duplicate concurrent agent loops per machine.
- [ ] 3.3 Wire scan + snapshot write + sync flow into each loop tick.

## 4. Scheduling Integration

- [ ] 4.1 Add Windows scheduler install/uninstall commands (Task Scheduler, logon + repeat interval).
- [ ] 4.2 Add Linux scheduler install/uninstall commands (systemd user service + timer).
- [ ] 4.3 Add docs and safety checks for scheduler setup prerequisites.

## 5. CLI Aggregate View

- [ ] 5.1 Add command/flag flow that loads all machine snapshots during normal runs.
- [ ] 5.2 Add stale-state detection with configurable TTL and clear status labeling.
- [ ] 5.3 Merge and display local + remote machine views clearly in table/summary output.

## 6. Security and Privacy Guardrails

- [ ] 6.1 Document private-repository requirement and metadata exposure risks.
- [ ] 6.2 Add optional redaction mode for path display in shared snapshots (future-compatible switch).
- [ ] 6.3 Ensure logs avoid printing sensitive state content by default.

## 7. Validation

- [ ] 7.1 Add tests for snapshot parsing, staleness, and merge behavior.
- [ ] 7.2 Add tests for git-sync retry flow using controlled command failures.
- [ ] 7.3 Run `go build ./...` and verify no regression in existing scan behavior.
