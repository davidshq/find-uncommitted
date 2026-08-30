## Why

The default tree scan already correlates projects across machines, but the primary unit of display is still each clone path. Morning habit needs “where is unfinished work on *this* project?” — one row per project, columns per machine — without stacking Attention, a matrix, and a full path inventory as three competing heroes.

## What Changes

- **BREAKING (default CLI output):** Replace the default interactive primary view (Attention list + Full inventory path table) with a **Project × Machine matrix** when a state repo is in play (and a sensible local-only fallback when it is not).
- Compact matrix cells reuse check-style brevity: `clean` / `dirty` / `↑N unpushed` / behind/diverged/empty/error as needed / `stale`; show branch only when it differs or matters.
- Keep `--dirty-only` as the filter for projects that need care (same situation predicate as today).
- Demote today’s path-centric inventory behind an opt-in flag (`--inventory` and/or `--verbose`) — audit trail, not morning primary.
- Do **not** add a `--morning` mode switch; the matrix *is* the default tree-scan output.
- Do **not** print Attention + matrix + full inventory together by default. Situation detection remains; nudge copy is secondary or omitted from the default hero (available via inventory/verbose or `check`).
- `check <path>` stays the single-project preflight; unchanged in role.
- Update README / help text to match; mockup at `mockup/fork-b-correlated-view.html` is the visual reference.

## Capabilities

### New Capabilities
- `project-machine-matrix`: Default tree-scan display as Project × Machine matrix, opt-in path inventory, and rules for what must not appear as co-equal primary sections.

### Modified Capabilities
- `cross-machine-state-sync`: Aggregate visibility and Attention requirements currently mandate “Attention then Full inventory”; update so the default aggregate primary is the matrix, with path inventory opt-in and Attention no longer a required leading hero on tree scans.

## Impact

- Display path in `main.go` / `aggregate.go` / `situations.go` (print order and table layout); reuse `GroupRowsByProject`, `DetectSituations`, and check cell formatting ideas from `check.go`.
- CLI flags / help: add `--inventory` (and possibly wire `--verbose`); no new `--morning`.
- README output examples and any tests that assert Attention-then-inventory stdout shape.
- Does not change snapshot schema, agent publish loop, `check` exit codes, or VS Code extension (still consumes `check --json`).
- Visual reference: `mockup/fork-b-correlated-view.html`; product framing: `strategic-directions.md` Fork B.
