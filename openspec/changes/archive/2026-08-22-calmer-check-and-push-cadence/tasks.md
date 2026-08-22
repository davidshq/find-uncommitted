## 1. Sticky heartbeat + calmer defaults

- [x] 1.1 Extend `UserConfig` / `FlagOverrides` / `ResolvedSettings` with `heartbeat` (string); resolve via existing precedence (config required; env optional if trivial; no new CLI flag)
- [x] 1.2 Update `EnsureConfigFromAgent` / install sticky writes to persist `heartbeat` alongside `interval` and `stale_ttl`
- [x] 1.3 Add/adjust unit tests for load/save/resolve of `heartbeat`
- [x] 1.4 Change built-in defaults: check interval `2m`, heartbeat `15m`, stale TTL `30m` (`agent.go` / `gitsync.go`); apply only when unset after resolution

## 2. Wire and document

- [x] 2.1 Wire resolved heartbeat into `SyncConfig.Heartbeat`; log check interval + heartbeat on agent start
- [x] 2.2 Confirm existing `gitsync_test.go` still covers skip-within-heartbeat and heartbeat-due; adjust defaults in tests only if they assumed 2m implicitly
- [x] 2.3 Update README example config and defaults docs for `interval` (check), `heartbeat`, and `stale_ttl`; note how to restore the old 30s/2m/5m profile
- [x] 2.4 Run unit tests (`go test ./...`)
