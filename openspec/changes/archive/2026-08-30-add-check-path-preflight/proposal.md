## Why

Cross-machine origin correlation and Attention situations already exist, but the habit moment is missing: before digging into one repo, there is no path-scoped pre-flight. Users must remember to run a full scan and hunt for the project. Shipping `check <path>` cashs that work into a rare, actionable nudge (scriptable for a later `cd` hook).

## What Changes

- Add a `check <path>` CLI mode that resolves a single local git repository, loads remote snapshots when a state repo is configured, and prints a compact project × machine summary plus situation nudges for that project only.
- Exit codes for check mode: `0` when nothing needs attention, `2` when one or more situations apply, `1` for usage/errors (invalid path, not a git repo, bad config).
- No full inventory dump, no scan of sibling repos under a root.
- Docs: usage example and exit-code note for `check`.
- Not in scope: Project × Machine matrix as primary morning view, Starship/menubar, `cd` hook install helpers, soft subcommand taxonomy beyond this one verb.

## Capabilities

### New Capabilities
- `check-path-preflight`: Path-scoped pre-flight that correlates one local repo with other machines' snapshots and reports compact status + nudges with meaningful exit codes.

### Modified Capabilities
- `cross-machine-state-sync`: Extend aggregate/Attention behavior to support a single-project check mode that reuses snapshot loading and situation detection without requiring a full tree scan.

## Impact

- CLI entrypoint (`main.go`) argument parsing and a new check code path
- Likely small helpers for “path → git root” and check display/exit
- Tests for check resolution, display, and exit codes
- README usage section
