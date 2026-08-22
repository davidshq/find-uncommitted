## Why

At a 2-minute heartbeat the agent creates ~720 commits/machine/day even when nothing changed — that volume is a knob problem (aggressive defaults + heartbeat not sticky-configurable), not a reason to redesign the git bus. Calmer check cadence and a quieter, config-backed liveness interval make the agent laptop- and state-repo-friendly while real status changes still publish on the next check.

## What Changes

- Expose **heartbeat** (liveness interval) as a sticky TOML setting with the same flag/env/config precedence as existing sticky keys; keep `interval` as check cadence
- **Calmer defaults** when unset: check `2m`, heartbeat `15m`, stale TTL `30m`; document that `interval = "30s"` + `heartbeat = "2m"` restores the old aggressive profile
- Keep existing publish behavior: commit/push immediately when snapshot content changes; when unchanged, commit only when the heartbeat is due (no `updated_at` rewrite on skip)
- Persist `heartbeat` on install / first agent config write alongside existing fields
- Update README example config for the two knobs and new defaults

**Deferred (not in this change):** `push_on_change` defer policy; new `--heartbeat` CLI flag / env (sticky TOML is enough for v1; existing `--interval` remains)

## Capabilities

### New Capabilities

- (none)

### Modified Capabilities

- `cli-user-config`: Persist and resolve sticky `heartbeat`; calmer built-in defaults for unset `interval` / `stale_ttl`
- `cross-machine-state-sync`: Quieter default heartbeat; document check vs liveness; content changes still publish on the check that detects them

## Impact

- Affected code: `config.go` / `config_test.go`, `main.go` (resolve + wire heartbeat from sticky/env if already following interval pattern lightly — prefer config over new flags), `agent.go` (defaults), `gitsync.go` (heartbeat default), install/agent config persistence, README
- No snapshot JSON schema change; `updated_at` remains the freshness clock
- No change to git-as-bus architecture
- Existing sticky `interval` / `stale_ttl` keep winning over new defaults; unset `heartbeat` picks up `15m`
