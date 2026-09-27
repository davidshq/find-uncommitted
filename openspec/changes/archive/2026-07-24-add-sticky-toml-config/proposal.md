## Why

Installing the cross-machine agent persists `--state-repo` in the OS scheduler, but the interactive CLI still requires that path on every scan. After setup, bare `find-uncommitted DIR` stays local-only, which breaks the mental model that sync is “on.” A small sticky TOML config shared by agent and CLI closes that gap.

## What Changes

- Add a user-scoped TOML config (XDG / `%AppData%`) that stores at least `state_repo`, and optionally `scan_root`, `stale_ttl`, `machine_id`, `redact_paths`, and `interval`
- `--install-scheduler` (and agent when config is missing) writes/updates that file
- Interactive scans load the config when flags are unset and, if `state_repo` is set, pull + show aggregate remotes by default
- Keep `--no-remote` as the local-only escape hatch; flags and env still override config
- Degrade gracefully when the configured clone is missing or offline (warn; still show local results / on-disk snapshots)
- Update README for the new “install once, scan aggregates by default” flow
- No **BREAKING** changes: no config continues to mean local-only

## Capabilities

### New Capabilities
- `cli-user-config`: Persist and resolve sticky user settings (TOML path, precedence, write-on-install) for CLI and agent shared use

### Modified Capabilities
- `cross-machine-state-sync`: Aggregate visibility applies when `state_repo` comes from sticky config (not only an explicit `--state-repo` flag); document opt-out via `--no-remote` and offline degradation

## Impact

- Affected code: `main.go` flag resolution, scheduler install paths (`scheduler_linux.go`, `scheduler_windows.go`), agent startup, possibly new `config.go`
- Dependencies: TOML parsing library (or minimal hand-rolled subset if kept tiny)
- Systems: systemd user unit / Windows task remain launchers; config file becomes the durable source of truth
- Docs: README cross-machine and usage sections
