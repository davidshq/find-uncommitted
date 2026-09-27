## Context

The agent already separates **scan ticks** (`AgentConfig.Interval`, sticky `interval`, default 30s) from **liveness commits** (`SyncConfig.Heartbeat`, hard-coded 2m when unset). Unchanged snapshots skip commit unless the heartbeat is due — so idle commit rate is ~720/machine/day from the 2m heartbeat, not from every 30s tick. Sticky TOML already stores `interval` / `stale_ttl` but not heartbeat. An earlier draft of this change also proposed `push_on_change` and new CLI flags; those were cut as overbuild — content changes should stay immediate.

## Goals / Non-Goals

**Goals:**

- Expose heartbeat as a sticky TOML setting (same resolve precedence as other sticky strings)
- Ship calmer built-in defaults so fresh installs are quiet without a config file
- Keep `stale_ttl` coherent with quieter heartbeats
- Prefer config knobs over redesigning the git bus

**Non-Goals:**

- `push_on_change` / deferred content publishes (revisit only if someone asks twice)
- New `--heartbeat` flag or env var for v1 (sticky config + install write is enough; keep existing `--interval`)
- Orphan refs / object-store backends
- Snapshot schema changes
- Adaptive / battery-aware intervals
- Renaming `interval` → `check_interval`

## Decisions

1. **Two sticky durations only**
   - `interval` — check cadence (scan + publish decision). Keep the name; docs call it check cadence.
   - `heartbeat` — liveness commit when content is unchanged. Wire sticky resolve into `SyncConfig.Heartbeat`.
   - Content change → commit/push on that check tick (unchanged from today).
   - Alternatives considered: `push_on_change` bool (rejected — fights the product; default-true dead weight + `*bool` complexity); separate push ticker (rejected); rename `interval` (rejected — sticky churn).

2. **Calmer defaults (fresh installs / unset keys)**
   - `interval`: **2m** (was 30s)
   - `heartbeat`: **15m** (was 2m) — ~96 idle commits/day vs ~720
   - `stale_ttl`: **30m** (was 5m) — roughly `2 × heartbeat`
   - Do not rewrite existing sticky `interval` on upgrade; machines with `interval = "30s"` keep frequent scans but inherit quieter heartbeat when unset.
   - Document restoring `interval = "30s"` + `heartbeat = "2m"` (+ `stale_ttl = "5m"` if desired).

3. **Config surface for v1**
   - Add `heartbeat` to `UserConfig` / resolve / install + agent first-write persistence.
   - Optional: resolve from env if wiring is trivial and matches `FIND_UNCOMMITTED_INTERVAL` pattern; **do not** add a CLI flag in v1.
   - Agent startup log should mention check interval and heartbeat.

4. **No publish-gate changes beyond default**
   - `PublishLocalSnapshot` logic stays: needsCommit when content changed **or** heartbeat due; skip must not rewrite `updated_at`; still `pushIfAhead`.
   - Only the default returned by `SyncConfig.heartbeat()` changes (plus sticky override).

## Risks / Trade-offs

- **[Existing sticky `interval = "30s"`]** → Frequent scans remain until the operator edits config; idle commits drop via new heartbeat default — acceptable silent win.
- **[Default `stale_ttl` bump]** → Healthy quiet agents stop looking falsely stale; operators who want aggressive labeling keep a short `stale_ttl` and matching heartbeat.
- **[No defer-on-change]** → Busy machines may still commit every check while status churns — correct for a cross-machine dirty detector; heartbeat only caps the idle case.

## Migration Plan

- Rebuild/restart agent; no snapshot migration.
- Missing `heartbeat` → `15m`; missing interval/stale → new calm defaults.
- Rollback: previous binary; optional sticky key revert.

## Open Questions

- None blocking.
