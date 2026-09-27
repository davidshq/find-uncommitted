## MODIFIED Requirements

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
