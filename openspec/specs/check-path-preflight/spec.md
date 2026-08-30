# check-path-preflight Specification

## Purpose

Path-scoped pre-flight for a single local git repository: compact project × machine status, Attention-style nudges for that project only, and scriptable exit codes — without scanning a directory tree.

## Requirements

### Requirement: Path-scoped check command
The CLI SHALL support a `check <path>` mode invoked as `find-uncommitted [flags] check <path>` that evaluates a single local git repository (resolving `<path>` to its work-tree root) without scanning sibling repositories under a directory tree.

#### Scenario: Check a repository root
- **WHEN** the user runs `find-uncommitted check /path/to/repo` and that path is a git work tree
- **THEN** the tool live-checks only that repository and does not discover or status other repos under a parent scan root

#### Scenario: Check from a subdirectory
- **WHEN** the user runs `find-uncommitted check /path/to/repo/src` and `src` is inside a git work tree
- **THEN** the tool resolves the work-tree root and checks that repository

#### Scenario: Path is not a git repository
- **WHEN** the user runs `find-uncommitted check <path>` and `<path>` is not inside a git work tree
- **THEN** the tool prints an error to stderr and exits with code `1` without scanning for nested repos

#### Scenario: Missing path argument
- **WHEN** the user runs `find-uncommitted check` with no path
- **THEN** the tool prints usage guidance to stderr and exits with code `1`

### Requirement: Compact check output with situations
In check mode the CLI SHALL print a compact per-machine status summary for the correlated project and any Attention-style nudges for that project only. It MUST NOT print the full multi-repo inventory table or the long “may take a while” scan preamble.

#### Scenario: Cross-machine attention on check
- **WHEN** a state repository is configured, remote loading is enabled, and another machine’s snapshot for the same project identity needs attention while local is clean
- **THEN** check output includes a summary covering local and remote machine status and at least one nudge describing the other-machine situation

#### Scenario: Local-only check
- **WHEN** check runs with `--no-remote` or without a resolvable state repository path for remotes
- **THEN** output reflects local status (and local situations) without requiring remote snapshots

#### Scenario: Nothing needing attention
- **WHEN** check finds no situations for the project
- **THEN** output indicates the project is clear (or shows clean machine statuses) without Attention bullets claiming problems

### Requirement: Check mode exit codes
Check mode SHALL exit with code `0` when no situations need attention for the project, code `2` when one or more situations need attention, and code `1` for usage errors, invalid configuration that aborts the run, or failure to resolve/check the path as a git repository.

#### Scenario: Clean exit
- **WHEN** check completes and `DetectSituations` yields no situations for the project
- **THEN** the process exits with code `0`

#### Scenario: Attention exit
- **WHEN** check completes and one or more situations apply to the project
- **THEN** the process exits with code `2`

#### Scenario: Error exit
- **WHEN** check cannot proceed (missing path, not a git repo, or invalid state repo when remotes are required)
- **THEN** the process exits with code `1`
