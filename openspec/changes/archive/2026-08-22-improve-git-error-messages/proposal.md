## Why

Repository status checks often surface opaque failures such as `exit status 128` without git's stderr detail, and some benign states (empty repositories, missing upstream) are misclassified as "fix local git error" in Attention. That erodes trust: users cannot tell whether a repo is broken, empty, or merely unconfigured. After adding git timeouts and worker caps, clearer per-repo error text is the next step to make Attention actionable.

## What Changes

- Include git stderr (or a trimmed fatal line) in per-repository `Error` text when subprocesses fail, instead of only `exit status N`
- Classify only **known-benign** upstream/git states explicitly (no upstream, empty/no-commits repo); do not map all exit 128 generically
- Treat empty repositories (no commits yet) as informational status, not Attention-worthy local errors
- Keep unknown git fatals as real errors with full detail so real problems are not papered over
- Add tests for error formatting and upstream classification edge cases
- Document error-message behavior briefly in README troubleshooting

## Capabilities

### New Capabilities

- `repo-status-errors`: Per-repository git failure messages include actionable stderr detail; benign repo states are classified narrowly without hiding unknown fatals

### Modified Capabilities

- `cross-machine-state-sync`: Attention and inventory treatment for empty repositories and upstream-check outcomes (untracked upstream vs true errors)
- `git-command-timeouts`: Timeout/cancellation repo errors SHALL remain distinct from generic git fatals and include timeout wording (not conflated with invalid-repo or upstream messages)

## Impact

- Affected code: `main.go` (`checkRepoStatus`, error helpers), possibly small shared helper in `gitexec.go`
- Tests: unit tests for `formatGitError`, upstream classification (no upstream, empty repo, unknown fatal)
- Docs: README troubleshooting — git error detail and empty-repo behavior
- No **BREAKING** CLI changes; CSV `Changes`/`Status` columns may show more descriptive error strings (behavioral improvement only)
