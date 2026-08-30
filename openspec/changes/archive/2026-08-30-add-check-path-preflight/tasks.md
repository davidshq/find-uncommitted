## 1. Path resolution and CLI wiring

- [x] 1.1 Add `resolveGitToplevel(path)` using `git rev-parse --show-toplevel` (accept nested paths; clear error if not a work tree)
- [x] 1.2 Detect positional `check <path>` after flag parse; do not require sticky `scan_root` in check mode
- [x] 1.3 Wire check mode to reuse sticky/`--state-repo`/`--no-remote`/`--stale-ttl`/`--machine-id` like interactive scan
- [x] 1.4 Update usage/help with `check` example and exit codes

## 2. Check pipeline and display

- [x] 2.1 Live-status the single resolved repo; load remotes when configured; build aggregate rows and filter to that project correlation key
- [x] 2.2 Print compact project × machine summary + Attention nudges for that project only (no full inventory / long scan preamble)
- [x] 2.3 Exit `0` when no situations, `2` when situations exist, `1` on usage/path/config errors

## 3. Tests and docs

- [x] 3.1 Unit tests: toplevel resolution, filter-to-project behavior, exit-code helpers / check runner with fixtures
- [x] 3.2 README: document `check <path>`, compact output, and exit codes
- [x] 3.3 Run `go test ./...` and fix regressions
