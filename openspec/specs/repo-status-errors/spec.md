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
During upstream tracking checks, the system SHALL classify only known-benign outcomes explicitly. `no upstream configured` SHALL set untracked-upstream status without a repository error. Empty repositories (no commits yet) SHALL NOT be reported as local git errors or Attention-worthy fix-local-error situations. Any other upstream fatal SHALL remain a repository error with stderr detail preserved.

#### Scenario: No upstream configured
- **WHEN** upstream resolution fails with `fatal: no upstream configured`
- **THEN** the repository has untracked upstream set and no `Error` field

#### Scenario: Empty repository
- **WHEN** a repository has no commits yet and upstream resolution fails with messages such as `does not have any commits yet` or `no such branch` consistent with an unborn branch
- **THEN** the repository is marked as empty (not an error) and Attention does not emit a fix-local-git-error nudge for it

#### Scenario: Unknown upstream fatal stays an error
- **WHEN** upstream resolution fails with a fatal message that is neither no-upstream nor empty-repo
- **THEN** the repository `Error` includes the stderr detail and Attention MAY include a fix-local-git-error nudge

### Requirement: Invalid repository errors include detail when available
When initial repository validation (`git rev-parse --git-dir`) fails for reasons other than timeout/cancellation or dubious ownership, the system SHALL include git stderr detail when available instead of only a generic invalid-repository label.

#### Scenario: Rev-parse fatal with stderr
- **WHEN** `git rev-parse --git-dir` fails with stderr explaining the failure
- **THEN** the repository error includes that detail (alongside or instead of a generic invalid-repository summary)

#### Scenario: Dubious ownership unchanged
- **WHEN** git reports dubious ownership
- **THEN** the existing safe.directory guidance is shown (this requirement does not change that behavior)
