## 1. Config module and dependency

- [x] 1.1 Add a TOML dependency and create `config.go` with platform config path helpers (XDG / `%AppData%`)
- [x] 1.2 Define the config struct (`state_repo`, optional `scan_root`, `machine_id`, `interval`, `stale_ttl`, `redact_paths`) with load/save helpers
- [x] 1.3 Implement precedence resolution: flags > env (`FIND_UNCOMMITTED_STATE_REPO` at minimum) > file > defaults
- [x] 1.4 Add unit tests for path resolution, load/save, and precedence

## 2. Wire CLI and agent

- [x] 2.1 Apply resolved sticky config in `main.go` before agent / scan / scheduler flows when flags are unset
- [x] 2.2 When interactive mode uses config-backed `state_repo`, print stderr notice and keep `--no-remote` as opt-out
- [x] 2.3 On missing/invalid state repo or pull failure, warn and continue with local (+ on-disk snapshots) per specs
- [x] 2.4 Have `--agent` create the config file when missing and `--state-repo` was provided

## 3. Scheduler write path

- [x] 3.1 Update `--install-scheduler` to write/update the TOML config from install args (Linux + Windows)
- [x] 3.2 Prefer config-backed resolution in agent launch so unit/task and CLI cannot silently diverge
- [x] 3.3 Document or implement a clear migration note for existing unit-only installs (re-run install)

## 4. Docs and verification

- [x] 4.1 Update README: sticky config location, precedence, default aggregate after install, `--no-remote`, migration
- [x] 4.2 Run unit tests and a manual smoke: install writes config; bare scan aggregates; `--no-remote` stays local
