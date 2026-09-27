## MODIFIED Requirements

### Requirement: Attention nudges for project situations
Situation detection SHALL continue to cover local error, local dirty, local unpushed, local behind, local untracked upstream, cross-machine branch mismatch, same-branch tip mismatch (via published short HEAD SHAs), other-machine dirty/unpushed (including when local is also dirty), and stale remote evidence when a remote attention-worthy row is stale and no stronger cue already qualifies staleness. Empty local repositories (no commits yet) SHALL NOT produce a local-error Attention nudge. Attention entries SHALL suggest advisory verbs only and MUST NOT execute git mutations on user repositories.

For default interactive tree-scan human output, the CLI SHALL NOT require a leading Attention section before the primary Project × Machine matrix. `--dirty-only` SHALL continue to limit presented projects to those with at least one situation (clean locals retained in the scan when remotes are enabled so cross-machine cues can fire); load-error rows remain visible. When the user opts into path inventory (`--inventory` and/or verbose-as-inventory), the CLI MAY print an Attention list of soft suggestions together with the path-centric inventory. `check` mode SHALL continue to surface nudges for the single project per check-path-preflight.

Aggregate path inventory (when requested) SHALL group rows by project identity.

#### Scenario: Default tree scan uses matrix without leading Attention hero
- **WHEN** the CLI finishes a default tree scan (local-only or aggregate) without inventory flags
- **THEN** output’s primary status section is the Project × Machine matrix and does not start with a mandatory Attention list followed by Full inventory

#### Scenario: Inventory may include Attention
- **WHEN** the user passes `--inventory` (or verbose-as-inventory) on a tree scan that has situations
- **THEN** output MAY include Attention nudge prose along with the path-centric inventory

#### Scenario: Local untracked upstream still detected
- **WHEN** a local repository has no upstream tracking configured
- **THEN** situation detection includes an untracked-upstream situation (surfaced via dirty-only / check / inventory Attention as applicable)

#### Scenario: Empty repository no error nudge
- **WHEN** a local repository has no commits yet
- **THEN** situation detection does not include a fix-local-git-error nudge for that project

#### Scenario: Cross-machine branch mismatch still detected
- **WHEN** the local clone and another machine's snapshot for the same origin report different branch names
- **THEN** situation detection includes a branch-mismatch situation naming the other branch and machine

#### Scenario: Same-branch tip mismatch still detected
- **WHEN** local and remote snapshots share a branch name but different short HEAD SHAs
- **THEN** situation detection includes a tip-mismatch situation

#### Scenario: Other machine has work still detected
- **WHEN** another machine's snapshot shows dirty or unpushed work for the same origin
- **THEN** situation detection includes a situation that distinguishes uncommitted work (pull will not help) from unpushed commits

#### Scenario: Dirty-only is situation-aware
- **WHEN** `--dirty-only` is set and remote loading is enabled
- **THEN** presented projects are limited to those that produced at least one situation (clean locals are retained in the scan so cross-machine cues can still fire); load-error rows remain visible

#### Scenario: Nudges never mutate user repos
- **WHEN** Attention or check suggests pull, push, commit, stash, or branch switching
- **THEN** the tool prints text only and does not run those git commands against user repositories
