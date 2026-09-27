## Context

Interactive tree scans already build `AggregateRow`s, group by project identity (`GroupRowsByProject`), and run `DetectSituations`. Display today is `DisplayAttention` then `displayAggregateTable` (path rows under project headers). `check` already prints a compact per-machine summary for one project via `formatCheckMachineCell`.

Fork B (`strategic-directions.md`) and the mockup `mockup/fork-b-correlated-view.html` redefine the **default** tree-scan hero as a Project × Machine matrix. Path inventory becomes opt-in. No `--morning` flag.

## Goals / Non-Goals

**Goals:**
- Default tree-scan output = Project × Machine matrix (reuse existing aggregation + situation predicates).
- Opt-in path inventory (`--inventory`; `--verbose` may alias or imply it).
- Single primary section by default — no Attention + matrix + inventory stack.
- Preserve `--dirty-only`, CSV export behavior (path-oriented is fine for CSV), `check`, agent, and snapshot schema.

**Non-Goals:**
- Shell prompt / menubar (Fork A), Obsidian-specific product work (Fork C).
- Changing situation detection rules or nudge wording semantics (only where/how they appear).
- VS Code matrix UI; extension stays on `check --json`.
- New snapshot fields or sync protocol changes.
- Auto-installed hooks or packaging theater.

## Decisions

1. **Default is matrix, not a mode flag**  
   Changing default output matches “swap morning primary view.” Alternatives considered: `--matrix` / `--morning` opt-in — rejected; habit should not require remembering a flag.

2. **Flag name `--inventory` for path table**  
   Clear audit-trail intent. `--verbose` may enable the same path table (or inventory + extra detail); prefer one implementation path with optional alias. Do not invent a third display mode.

3. **Attention is not a co-equal default hero**  
   Situation detection still drives `--dirty-only` and `check`. Default tree scan: matrix only (footer summary OK). Nudge text: available under `--inventory`/`--verbose` and always via `check` — not a leading Attention block plus matrix. Alternative: keep Attention above matrix — rejected as two heroes restating the same projects.

4. **Build matrix from `GroupRowsByProject` + machine column set**  
   Collect machine IDs from rows (local first, marked `*`), one cell per (project, machine). Reuse or share formatting with `formatCheckMachineCell`, shortened for column width (`dirty`, `↑3`, `stale`). Multiple clones of one project on one machine: collapse to worst status in the cell; full paths remain on `--inventory`.

5. **Local-only / no state repo**  
   Matrix still works with a single local machine column (or degenerate one-column table). Same code path as aggregate.

6. **CSV unchanged**  
   CSV stays path/machine oriented for scripting; only human default terminal layout changes.

## Risks / Trade-offs

- **[Info loss on glance]** Paths and unstaged/staged/untracked detail leave the default view → Mitigation: `--inventory`; cells still encode dirty vs unpushed vs behind.
- **[Users expect Attention first]** Habit break → Mitigation: README + help; matrix rows *are* the attention set under `--dirty-only`.
- **[Wide terminals / many machines]** Column explosion → Mitigation: truncate machine ids; document that inventory is for deep audit; optional later wrap is out of scope.
- **[Test churn]** Stdout golden tests assume Attention-then-inventory → Mitigation: update tests in the same change.
- **[Both dirty collision less verbal]** Matrix shows two dirty cells without “commit or stash…” → Mitigation: collision remains visible as dual dirty cells; `check` / inventory retain nudge prose.

## Migration Plan

1. Implement matrix printer + `--inventory`; flip default in `main` display path.
2. Update README examples and flag help; point at mockup as intent reference.
3. Update/adjust display and any CLI stdout tests.
4. No config migration; no snapshot version bump. Users who scripted human stdout may need `--inventory` — call out as **BREAKING** in release notes.

## Open Questions

- Exact flag surface: `--inventory` only vs also `--verbose` as alias (prefer both mapping to path table for discoverability).
- Whether `--inventory` reprints Attention nudges above the path table (lean yes — inventory is the “old full view”) or path table alone.
