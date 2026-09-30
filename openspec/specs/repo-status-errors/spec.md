# repo-status-errors Specification

## Purpose

Surface actionable git stderr detail in per-repository scan errors and classify only known-benign outcomes (empty repository, missing upstream) without hiding unknown git failures.

## Requirements

### Requirement: Git stderr in repository error messages
When a per-repository git subprocess fails during status checking, the system SHALL include actionable git output in that repository's `Error` field. The message SHALL prefer trimmed stderr (at minimum the first `fatal:` line when present) and MAY fall back to the Go execution error only when stderr is empty. The system MUST NOT replace unknown git failures with a generic message that omits stderr detail.

#### Scenario: Upstream check includes stderr detail
- **WHEN** `git rev-parse @{u}` fails with stderr `fatal: refusing to merge unrelated histories` and exit status 128
- **THEN** the repository error includes that fatal detail (not only `exit status 128`)

#### Scenario: Empty stderr falls back to execution error
- **WHEN** a git subprocess fails with no stderr output
- **THEN** the repository error includes the execution error text

### Requirement: Narrow classification of benign upstream outcomes
During upstream tracking checks, the system SHALL classify only known-benign outcomes explicitly. `no upstream configured` SHALL set untracked-upstream status without a repository error, as SHALL a configured upstream whose tracking ref no longer exists ("gone" after merge + prune). A detached HEAD SHALL be labeled `detached HEAD (<sha>)` and SHALL skip upstream checks rather than report an error. Empty repositories (no commits yet) SHALL NOT be reported as local git errors or Attention-worthy fix-local-error situations. Any other upstream fatal SHALL remain a repository error with stderr detail preserved.

#### Scenario: No upstream configured
- **WHEN** upstream resolution fails with `fatal: no upstream configured`
- **THEN** the repository has untracked upstream set and no `Error` field

#### Scenario: Empty repository
- **WHEN** a repository has no commits yet (verified via unborn HEAD / `rev-parse HEAD` failure with empty-repo messages such as `does not have any commits yet` or `needed a single revision`)
- **THEN** the repository is marked as empty (not an error) and Attention does not emit a fix-local-git-error nudge for it

#### Scenario: Gone upstream
- **WHEN** the branch has a configured upstream whose remote-tracking ref was deleted (merged and pruned)
- **THEN** the repository has untracked upstream set and no `Error` field, and MUST NOT be marked empty

#### Scenario: Detached HEAD
- **WHEN** the repository is on a detached HEAD (submodule, mid-rebase or bisect), including when `git branch --show-current` exits 0 with empty output
- **THEN** the branch is `detached HEAD (<sha>)`, no upstream error is recorded, and dirty state is still reported

#### Scenario: Unknown upstream fatal stays an error
- **WHEN** upstream resolution fails with a fatal message that is neither no-upstream, gone-upstream, nor empty-repo
- **THEN** the repository `Error` includes the stderr detail and Attention MAY include a fix-local-git-error nudge; the repository MUST NOT be marked empty

### Requirement: Invalid repository errors include detail when available
When initial repository validation (`git rev-parse --git-dir`) fails for reasons other than timeout/cancellation or dubious ownership, the system SHALL include git stderr detail when available instead of only a generic invalid-repository label.

#### Scenario: Rev-parse fatal with stderr
- **WHEN** `git rev-parse --git-dir` fails with stderr explaining the failure
- **THEN** the repository error includes that detail (alongside or instead of a generic invalid-repository summary)

#### Scenario: Dubious ownership unchanged
- **WHEN** git reports dubious ownership
- **THEN** the existing safe.directory guidance is shown (this requirement does not change that behavior)
