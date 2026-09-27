## Why

Every git subprocess is a bare `exec.Command` with no deadline. A hung credential prompt or stuck git blocks an agent tick forever while the process still looks healthy, so the aggregate goes silently stale. This is the reframe item that fails trust under normal use (`developer-review-findings.md`, `strategic-directions.md`).

## What Changes

- Bound every git subprocess with `exec.CommandContext` and a per-command deadline
- Give each agent publish tick a cancellable deadline so a hung scan/sync cannot stall the loop indefinitely
- Replace `time.After` in the agent wait loop with `time.Ticker` so interval timing stays correct across ticks
- Surface timeout failures as non-fatal per-repo or per-tick warnings (agent continues; interactive scan records errors)
- Document the timeout behavior and defaults

## Capabilities

### New Capabilities

- `git-command-timeouts`: All git subprocesses used for scanning and state-repo sync run under a context deadline; timeouts cancel the process and surface as errors rather than hanging forever

### Modified Capabilities

- `cross-machine-state-sync`: Agent publish ticks become deadline-bounded and interruptible; the wait loop uses a ticker; offline-tolerant warnings cover tick/git timeouts

## Impact

- Affected code: `gitsync.go` (`ExecGitRunner`), `main.go` / scan helpers (`checkRepoStatus` and related git calls), `origin.go`, `agent.go` (tick context + ticker)
- Tests: timeout/cancel coverage for git runner and agent tick behavior
- Docs: README agent section notes timeouts
- No **BREAKING** CLI flag changes; optional timeout overrides may be added if useful, but defaults alone close the trust gap
