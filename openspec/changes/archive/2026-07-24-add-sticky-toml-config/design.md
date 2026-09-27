## Context

Cross-machine sync already publishes snapshots via `--agent` / `--install-scheduler`, but the interactive CLI only aggregates remotes when `--state-repo` is passed. Scheduler units embed the path; the CLI does not read them. Users who completed setup still see local-only reports from bare scans. This design adds a small shared TOML config as the durable source of truth for agent and CLI.

Constraints: keep Windows + Linux parity; no silent remotes without an install/config signal; degrade offline without failing local scans; credentials stay in Git (never in config).

## Goals / Non-Goals

**Goals:**
- Persist sticky settings in a user-scoped TOML file
- Have `--install-scheduler` (and agent when missing) write/update that file
- Default interactive scans to aggregate remotes when `state_repo` is configured
- Preserve flag/env overrides and `--no-remote` opt-out
- Keep scheduler units as thin launchers that prefer the same config

**Non-Goals:**
- Subcommand rewrite (`scan` / `status` / `agent`)
- Scraping systemd units or Task Scheduler XML as config
- Auto-discovering state clones on disk
- Multi-profile / multi-user system-wide config
- Storing secrets or tokens in the TOML file

## Decisions

1. **Config location**
   - Linux/macOS: `$XDG_CONFIG_HOME/find-uncommitted/config.toml` (fallback `~/.config/find-uncommitted/config.toml`)
   - Windows: `%AppData%\find-uncommitted\config.toml`
   - Alternatives considered: beside-binary (breaks upgrades), env-only (forgotten across shells), unit scraping (fragile). Rejected.

2. **Schema (small TOML, not one-string file)**
   ```toml
   state_repo = "/path/to/uncommitted-state"
   scan_root = "/path/to/repos"          # optional
   machine_id = ""                       # optional; empty = hostname
   interval = "30s"                      # optional
   stale_ttl = "5m"                      # optional
   redact_paths = false                  # optional
   ```
   - Rationale: enough to keep agent and CLI from drifting without becoming a settings product.

3. **Precedence**
   - Explicit flags > env (`FIND_UNCOMMITTED_STATE_REPO`, and matching vars for other keys if useful) > config file > built-in defaults (local-only).
   - If `state_repo` resolves and `--no-remote` is unset, interactive mode pulls (`--ff-only`) and loads aggregates.

4. **Who writes the file**
   - `--install-scheduler` always writes/updates config from the args used for install, then registers the unit/task.
   - `--agent` writes config if missing (or if `state_repo` was passed and differs—update `state_repo` only when explicitly flagged).
   - No separate `config init` required for v1 (optional later).

5. **UX feedback**
   - When config supplies `state_repo`, print one stderr line: using state repo from config; pass `--no-remote` for local only.
   - Missing/invalid clone or pull failure: warn, continue with local scan and any on-disk snapshots; do not hard-fail interactive use.

6. **TOML dependency**
   - Prefer a small maintained parser (e.g. `github.com/BurntSushi/toml` or `pelletier/go-toml`) over hand-rolled parsing for robustness with comments/optional keys.

7. **Scheduler relationship**
   - Units may still pass `--state-repo` for clarity during migration, but runtime resolution SHOULD load the config file so edits apply without rewriting the unit. Longer-term, ExecStart can shrink to `--agent` (+ optional scan root).

## Risks / Trade-offs

- **[Surprise remotes]** → Only activate when config exists; stderr notice + `--no-remote`.
- **[Config vs unit drift]** → Install rewrites both; document config as source of truth; prefer reading config in agent.
- **[Offline / auth failures]** → Warn and degrade; never block local results on network.
- **[Dependency weight]** → Small TOML lib is acceptable vs fragile custom parse.
- **[Existing installs]** → Users with only a systemd unit and no config stay local-only until they re-run `--install-scheduler` or create the file—document migration.

## Migration Plan

1. Ship binary with config load/write support.
2. Ask existing users to re-run `--install-scheduler` (or copy `state_repo` into the TOML path once).
3. Rollback: delete/rename config file restores prior local-only default; flags still work.

## Open Questions

- Whether optional `scan_root` in config should allow omitting the directory argument on interactive scans (nice-to-have; can defer).
- Exact env var names beyond `FIND_UNCOMMITTED_STATE_REPO` (can add only as needed).
