# cross-machine-state-sync Specification

## Purpose

Synchronize repository status across machines using shared Git-backed per-machine snapshots with freshness indicators, and surface soft Attention nudges for local and cross-machine situations without mutating user repositories.

## Requirements

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

### Requirement: Behind-upstream and tip metadata in scans and snapshots
Each scanned repository with a configured upstream SHALL report whether it is behind that upstream using already-known tracking refs (no mandatory fetch). Published snapshots SHALL include behind status, optional ahead/behind counts, and a short HEAD SHA when available. Older snapshots missing these fields SHALL still load; absent behind/SHA fields SHALL be treated as unknown/false rather than failing parse.

#### Scenario: Local behind detection without fetch
- **WHEN** a repository has upstream tracking refs that are ahead of HEAD
- **THEN** the scan marks the repository behind and includes a behind count derived from `HEAD..@{u}`

#### Scenario: Snapshot publishes behind and HEAD SHA
- **WHEN** the agent publishes a repository that is behind upstream
- **THEN** the snapshot JSON includes `has_behind` (true), `behind_count`, and `head_sha` when resolvable

#### Scenario: Legacy snapshot without behind fields
- **WHEN** a machine snapshot JSON omits `has_behind`, counts, and `head_sha`
- **THEN** the CLI loads it successfully and treats behind as false / SHA empty

### Requirement: Attention nudges for project situations
When presenting results, the CLI SHALL print an Attention section before the full inventory. Attention entries SHALL suggest advisory verbs only and MUST NOT execute git mutations on user repositories. Situations SHALL include local error, local dirty, local unpushed, local behind, local untracked upstream, cross-machine branch mismatch, same-branch tip mismatch (via published short HEAD SHAs), other-machine dirty/unpushed while local is clean, and stale remote evidence when a remote attention-worthy row is stale and no stronger cue already qualifies staleness. The aggregate inventory SHALL group rows by project identity. Empty local repositories (no commits yet) SHALL NOT produce a local-error Attention nudge.

#### Scenario: Attention precedes inventory
- **WHEN** the CLI finishes a scan (local-only or aggregate)
- **THEN** output starts with an Attention list of soft suggestions, then a Full inventory section

#### Scenario: Local untracked upstream nudge
- **WHEN** a local repository has no upstream tracking configured
- **THEN** Attention includes a nudge to set upstream tracking

#### Scenario: Empty repository no error nudge
- **WHEN** a local repository has no commits yet
- **THEN** Attention does not include a fix-local-git-error nudge for that project

#### Scenario: Cross-machine branch mismatch nudge
- **WHEN** the local clone and another machine's snapshot for the same origin report different branch names
- **THEN** Attention includes a branch-mismatch nudge naming the other branch and machine

#### Scenario: Same-branch tip mismatch
- **WHEN** local and remote snapshots share a branch name but different short HEAD SHAs
- **THEN** Attention includes a tip-mismatch nudge

#### Scenario: Other machine has work
- **WHEN** the local clone is clean and another machine's snapshot shows dirty or unpushed work for the same origin
- **THEN** Attention includes a nudge that distinguishes uncommitted work (pull will not help) from unpushed commits

#### Scenario: Dirty-only is situation-aware
- **WHEN** `--dirty-only` is set and remote loading is enabled
- **THEN** Attention and inventory are limited to projects that produced at least one situation (clean locals are retained in the scan so cross-machine cues can still fire); load-error rows remain visible

#### Scenario: Nudges never mutate user repos
- **WHEN** Attention suggests pull, push, commit, stash, or branch switching
- **THEN** the tool prints text only and does not run those git commands against user repositories

### Requirement: Local-only project identity without origin
For repositories without an `origin` remote, correlation SHALL use parent-directory plus basename (for example `manuscripts/book`) so identically named repos under different parents do not collide, while similarly laid-out trees on two machines can still match.

#### Scenario: Different parents do not collide
- **WHEN** two local-only repositories are `/code/app` and `/archive/app`
- **THEN** they use distinct correlation keys

#### Scenario: Matching layout correlates across machines
- **WHEN** two machines have local-only clones at `.../manuscripts/book`
- **THEN** those entries share a correlation key

### Requirement: Snapshot freshness signaling
The CLI SHALL mark machine snapshots as stale when their last update time exceeds a configurable staleness threshold. When the threshold is unset, the built-in default SHALL be `30m`.

#### Scenario: Snapshot exceeds threshold
- **WHEN** a machine snapshot timestamp is older than the configured stale threshold
- **THEN** the machine is labeled stale in output

#### Scenario: Unset threshold uses calm default
- **WHEN** no stale threshold is configured
- **THEN** snapshots older than 30 minutes are labeled stale

### Requirement: Conflict-minimized state writes
Each machine SHALL write only its own snapshot file in shared storage to reduce multi-writer conflicts.

#### Scenario: Concurrent machine updates
- **WHEN** two machines publish around the same time
- **THEN** each machine updates only its own file path and sync retries resolve transient push races

### Requirement: Offline-tolerant sync behavior
The background publisher SHALL continue operating when network or remote Git access is unavailable and retry on subsequent cycles. Tick or git-command timeouts SHALL be treated as non-fatal for the agent loop in the same way as connectivity failures.

#### Scenario: Remote unavailable
- **WHEN** pull or push fails due to connectivity or remote access error
- **THEN** the agent logs a non-fatal warning and retries on a later cycle

#### Scenario: Tick or git timeout during publish
- **WHEN** a publish tick fails because the tick deadline or a git command deadline is exceeded
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

### Requirement: Single-project remote correlation for check
When check mode loads remote machine snapshots (state repository resolved and remote loading enabled), the system SHALL correlate the checked local repository with remote entries using the same project identity rules as the aggregate view (normalized origin, else path basename) and SHALL limit presented situations to that project.

#### Scenario: Same origin on another machine appears in check
- **WHEN** the checked local repo has a normalized origin that matches a remote snapshot entry on another machine
- **THEN** check includes that remote machine’s status in its project summary and may surface cross-machine situations for that identity

#### Scenario: Unrelated remote projects omitted
- **WHEN** remote snapshots contain other projects that do not share the checked repo’s correlation key
- **THEN** check output and situations do not include those unrelated projects
