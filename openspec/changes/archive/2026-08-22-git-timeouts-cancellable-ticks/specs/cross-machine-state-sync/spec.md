## ADDED Requirements

### Requirement: Cancellable deadline-bounded agent ticks
Each autonomous publish tick SHALL run under a cancellable context with a per-tick deadline derived from the agent’s parent context (signal cancellation). When the tick deadline expires, the system SHALL abort in-flight git work for that tick, log a non-fatal warning, and continue the agent loop. The wait between ticks SHALL use a reusable interval ticker rather than a one-shot timer recreated each cycle.

#### Scenario: Tick exceeds deadline
- **WHEN** an agent publish tick (pull, scan, or publish) does not finish before the per-tick deadline
- **THEN** the tick is aborted, a non-fatal warning is logged, and the agent continues waiting for subsequent ticks

#### Scenario: Signal stops mid-tick
- **WHEN** the agent receives an interrupt or termination signal during a publish tick
- **THEN** the tick context is cancelled, in-flight git work is terminated, and the agent loop exits cleanly

#### Scenario: Interval wait uses ticker
- **WHEN** agent mode is running between publish ticks
- **THEN** the wait is driven by a `Ticker` at the configured interval (not a fresh one-shot timer each cycle)

## MODIFIED Requirements

### Requirement: Offline-tolerant sync behavior
The background publisher SHALL continue operating when network or remote Git access is unavailable and retry on subsequent cycles. Tick or git-command timeouts SHALL be treated as non-fatal for the agent loop in the same way as connectivity failures.

#### Scenario: Remote unavailable
- **WHEN** pull or push fails due to connectivity or remote access error
- **THEN** the agent logs a non-fatal warning and retries on a later cycle

#### Scenario: Tick or git timeout during publish
- **WHEN** a publish tick fails because the tick deadline or a git command deadline is exceeded
- **THEN** the agent logs a non-fatal warning and retries on a later cycle
