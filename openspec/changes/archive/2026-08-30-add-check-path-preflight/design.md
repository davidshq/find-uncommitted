## Context

Origin correlation, `BuildAggregateRows`, and `DetectSituations` already answer cross-machine questions for a full scan. The missing habit surface is a path-scoped pre-flight (`check <path>`) that reuses that pipeline for one repo. Sticky config / `--state-repo` / `--no-remote` behavior should match interactive scans.

## Goals / Non-Goals

**Goals:**
- Resolve `<path>` to a single local git work tree root (accept repo root or a subdirectory inside it).
- Live-status that one repo; correlate with other machines via existing snapshot load + situation detection filtered to that project key.
- Compact stdout: one status line for local + remotes on that project, then Attention-style nudges (or a single quiet “ok” when clean).
- Exit codes: `0` clean, `2` attention needed, `1` usage/error.
- Stay scriptable for a future `cd` hook without shipping the hook.

**Non-Goals:**
- Project × Machine matrix as the morning primary view.
- Soft subcommand framework (`scan`, `doctor`, …) beyond the literal `check` verb.
- Prompt/menubar integration, notifications, auto-commit helpers.
- Changing agent publish cadence or snapshot schema.

## Decisions

1. **Invocation: positional `check <path>` after flags**  
   `find-uncommitted [flags] check <path>`. Detected after `flag.Parse()` when `args[0] == "check"`. Avoids a new flag soup entry and matches the strategic mockups.  
   *Alternative considered:* `--check <path>` — rejected; harder to compose with scan-root positional and reads less like a verb.

2. **Git root resolution via `git rev-parse --show-toplevel`**  
   Accepts files/dirs inside the repo. Fail with exit `1` if not a git work tree.  
   *Alternative:* only accept directories containing `.git` — rejects `check ./src` which is the natural pre-flight path.

3. **Reuse aggregate + situations; filter to one project key**  
   Build rows from one local `RepoStatus` + loaded remotes via `BuildAggregateRows`; run `DetectSituations`; keep only rows/situations for `repoCorrelationKey(local)` (reuse `FilterRowsByProjectKeys` / `ProjectKeysWithSituations`). Do not invent a parallel nudge engine.

4. **Compact display, not Full inventory**  
   One summary line, e.g. `work-project  laptop*: Dirty (unstaged) · desktop: Clean on main`, then nudge bullets (reuse or thin `DisplayAttention`). No “Scanning…” preamble / “may take a while”.

5. **Exit codes only in check mode (for now)**  
   Full scan keeps today’s always-`0`-on-success behavior to avoid surprising existing wrappers. Check is the first consumer that needs codes.  
   *Alternative:* retrofit exit codes on full scan immediately — deferred until someone scripts the morning scan.

6. **`--no-remote` / missing state repo**  
   Same rules as scan: sticky/`--state-repo` loads remotes unless `--no-remote`; invalid state repo is hard error when remotes requested; local-only check still reports local situations.

## Risks / Trade-offs

- **[Risk] Users type `find-uncommitted check` without a path** → Clear usage on stderr, exit `1`.
- **[Risk] Path is a scan root with many repos, not one repo** → `show-toplevel` from that directory only checks the enclosing repo of that path (if any); if the path itself is not inside a git repo, exit `1` with a clear message (do not fall back to scanning children).
- **[Risk] Local clone has no origin; remote published under origin** → Existing basename vs origin correlation limits apply; document that `check` uses the same identity rules as aggregate.
- **[Trade-off] Exit codes only for check** → Morning scan still always exits 0; acceptable until measured scripting pain.

## Migration Plan

No config migration. Document `check` in README and `printUsage`. Rollback = ignore the verb (users keep using full scan).

## Open Questions

None blocking — `cd` hook install is explicitly follow-up.
