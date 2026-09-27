## Context

`checkRepoStatus` in `main.go` runs a sequence of git subprocesses per repository. Failures today often propagate only Go's `exec` error (`exit status 128`) while git's stderr (e.g. `fatal: no such branch: 'main'`) is discarded. Upstream detection treats only the substring `no upstream configured` as benign; empty repositories (`git init`, no commits) hit a different fatal and surface as Attention-worthy local errors.

Recent work added per-command timeouts and worker caps; timeout errors are already classified separately via `isGitContextErr`. This change improves **message fidelity** and **narrow classification** without weakening reporting of unknown git failures.

## Goals / Non-Goals

**Goals:**

- Surface git stderr (trimmed) in repo `Error` strings when subprocesses fail
- Explicitly recognize empty/no-commits repositories and missing upstream as non-error states where appropriate
- Preserve distinct timeout/cancellation wording (do not conflate with invalid repo or upstream fatals)
- Keep Attention "fix local git error" for genuinely broken or unknown git states only

**Non-Goals:**

- Parsing every possible git exit code into bespoke UX
- Auto-fixing repos or running mutating git commands
- Changing scan concurrency, timeout defaults, or CSV column layout
- Hiding unknown `fatal:` messages behind generic friendly text

## Decisions

1. **Shared `formatGitError(stderr, err)` helper**
   - Prefer trimmed stderr when non-empty (strip trailing newline; optionally strip leading `fatal: ` for readability)
   - Fall back to `err.Error()` when stderr is empty
   - Used by `appendRepoCheckError`, upstream block, and branch/rev-parse failures where stderr adds value
   - *Alternative:* format at `ExecGitRunner` only — rejected; classification logic stays in `checkRepoStatus`

2. **Upstream check classification (narrow, not generic 128)**
   - `no upstream configured` → `HasUntrackedUpstream` (existing)
   - `does not have any commits yet` OR (`no such branch` AND zero commits verifiable via `rev-parse HEAD` failure with known empty-repo message) → set a new informational flag or `Error` empty with a dedicated snapshot field / status text like `Empty repository (no commits yet)`; **not** Attention `SituationLocalError`
   - All other upstream fatals → `Error` with `formatGitError` detail
   - *Alternative:* treat all exit 128 as untracked upstream — rejected (papers over real failures)

3. **Empty repo detection timing**
   - After successful `rev-parse --git-dir`, probe commits once: `git rev-parse --verify HEAD` or reuse branch step signals (`no commits yet` in stderr)
   - Short-circuit upstream/ahead/behind checks when empty; mark repo as clean/empty rather than error
   - *Alternative:* skip repo entirely in discovery — rejected (user may want to see empty inits in inventory)

4. **Attention behavior**
   - `SituationLocalError` only when `repo.Error != ""` after classification
   - Empty repos may appear in inventory with a distinct status (e.g. `Empty` or `Clean` with note); no "fix local git error" nudge
   - Untracked upstream keeps existing nudge

5. **Timeout errors unchanged in classification**
   - `isGitContextErr` / `setGitCancelled` path remains authoritative for timeouts
   - `formatGitError` MUST NOT rewrite timeout errors into generic git fatals

## Risks / Trade-offs

- **[stderr verbosity]** → Trim to first fatal line; cap length if needed (e.g. 200 chars) in helper
- **[false empty-repo match]** → Require both upstream fatal pattern and HEAD verification, covered by tests with fixtures/mocked git
- **[snapshot compatibility]** → New empty-repo representation should serialize cleanly in `RepoSnapshot` (optional `IsEmpty` or derive from branch + error-free state); older agents ignore unknown JSON fields
- **[Over-classification]** → Unknown fatals always keep full detail and remain errors

## Migration Plan

- Ship in next binary build; no config migration
- Users with empty `git init` folders see fewer spurious Attention errors immediately
- Re-run agent or restart scheduled task to refresh published snapshots with corrected status

## Open Questions

- Whether to add explicit `IsEmpty` on `RepoSnapshot` vs deriving from branch + flags (recommend `IsEmpty` or `EmptyRepo` bool for clarity in aggregate UI)
