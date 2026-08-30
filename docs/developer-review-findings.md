# Developer review findings

**Date:** 2026-07-24 (updated 2026-08-30 — open items only)  
**Scope:** Full codebase review (scan CLI, cross-machine sync, agent/schedulers, config, docs, packaging)  
**Participants (personas):** CLI/UX engineer · Systems/sync engineer · Product/platform engineer · Pragmatic engineer  
**Companion:** `strategic-directions.md` (product forks; current bet is Fork A · Ambient Signal)

Three developers independently reviewed the project, then reconciled findings. A pragmatic engineer later ranked what actually matters next. This file keeps **open** work only.

---

## Shared verdict

`find-uncommitted` already solves a real job: **don’t start conflicting work on machine B while machine A still has dirt.** Local scanning, Git-as-a-sync-bus, sticky config, origin correlation, Project × Machine matrix, `check <path>` (exit codes + `--json`), worktree discovery, and the VS Code thin client are in place.

What remains is mostly **ops/scripting polish when pain shows twice**, plus habit surfaces outside the morning table (see Fork A). Do not expand scan surface area.

**North star (agreed):** remain a personal unfinished-Git-work control plane—not a team SaaS, not a full repo manager. Own cross-machine awareness with zero backend.

---

## Roundtable notes (open friction)

### CLI/UX

Habit formation still has friction: incoherent help vs error paths, always-on “may take a while” preamble, emoji/column width issues, and no `--quiet` / `--plain` / scan-wide JSON. Flag soup mixes scan, agent, and scheduler install—soft subcommands can wait; help and post-install “next steps” should not.

### Systems / sync

Architecture is right for a handful of machines. Remaining reliability gap for 24/7 use: Windows has no crash restart (Linux systemd does).

### Product / platform

Still missing for packaging/ops: LICENSE, CI, releases, `go install`-friendly module path, folding `fix-ownership-tool/` into the main binary, and `status`/`doctor` after install.

### Pragmatic engineer

Personal tool, few machines, one user who already knows git. Optimize for **trust and habit**, not packaging theater. Ship scripting/packaging when you feel the pain twice; product next is ambient `check` (Fork A), not more morning-table flags.

#### Priority (ruthless)

| When | Do | Why |
|------|-----|-----|
| **Next (product)** | Hand-rolled `cd` hook around `check`; prompt/menubar only after that is boring | Fork A — see `strategic-directions.md` |
| **Next (when pain twice)** | `--quiet` / `--plain`, `doctor`, `--print-config`, CI / LICENSE / `go install` | Scripting and ops polish |
| **Later** | Windows task recovery / restart-on-failure parity with systemd | Only if agent runs 24/7 on Windows |
| **Never (now)** | Subcommands, setup wizard, brew/scoop, macOS LaunchAgent, ignore files, notifications, fold fix-ownership, schema version, machine prune | Feature gravity. Wait for measured pain. |

#### What’s over-scoped

**Packaging baseline is hygiene, not habit.** LICENSE + module path + CI do not unblock daily use if you build from a clone.

**Ops flags bury the lever.** `doctor` / `--print-config` / porcelain collapse do not increase morning use. Defer `--quiet` until a `cd` hook exists and the summary line is noise.

**Bottom line:** Wire `check` into the shell (Fork A). Add scripting/packaging when you feel the pain twice.

---

## Short-term (open)

Do not expand scan surface area. Prefer Fork A habit work over this list unless scripting/install pain shows up twice.

### Packaging (when needed)

1. **Packaging baseline** — LICENSE, module path usable with `go install`, GitHub Actions `go test` + cross-compile.

### Scripting / UX polish (when needed)

2. `--plain` / non-TTY / `NO_COLOR` for ASCII-stable output.
3. `--quiet` when scripting or with `--output` (defer until `cd` hook makes the summary line noise).
4. Scan-wide `--format json` (check already has `--json`).
5. Windows task recovery / restart-on-failure parity with systemd.
6. Post-install checklist (config path, how to verify agent, sample scan, privacy).
7. `--print-config` / resolved values + sources (already tracked as `ConfigSource`).
8. Collapse per-repo git calls toward `status --porcelain=v2`.
9. Unified help/errors (remaining UX hygiene).

---

## Medium-term (open)

| Theme | Ideas |
|-------|--------|
| **Operability** | `doctor` / `status`: lock PID, last publish, config path, systemd/task health, reachability |
| **Noise control** | Ignore file; optional drop of untracked-upstream from attention; sticky `exclude` globs |
| **One binary** | Fold `fix-ownership-tool` into main CLI (`fix-ownership` / prompt on ownership errors) |
| **Platform parity** | macOS LaunchAgent (`scheduler_other.go` is currently a hard decline) |
| **Distribution** | goreleaser, Homebrew / Scoop / winget |
| **Setup** | Guided wizard via `gh` (create private state repo → clone → config → scheduler → smoke publish) |
| **CLI shape** | Soft subcommands (`scan`, `agent`, `config`, `doctor`) when flag soup hurts |
| **Power users** | `--paths-only` / `-0` for fzf; wide-terminal columns |
| **Sync hygiene** | Machine retire/prune/rename; snapshot schema `version`; orphan file cleanup |
| **Tests** | Broader coverage beyond snapshot/gitsync/config (scanner, ownership, aggregate display, agent tick) |

---

## Long-term (open)

Stay Git-backed until measured pain (history bloat, push races, scale). Evolution path:

1. **Decision support** — shell prompt / starship snippet; optional notifications; deepen editor thin client as needed (`check` + VS Code client already exist).
2. **Optional light collaboration** — private state repo already enables couples/small teams; never build accounts.

**Only if Git bus hurts:** squash/shallow/gc policy → object store with conditional PUTs → small HTTP+blob API. Tens of machines × thousands of repos is fine with quiet heartbeats; hundreds of machines wants a non-Git bus.

**Explicitly defer / avoid:** multi-profile config until single-profile hurts; public multi-tenant sync; web SaaS dashboard; rewriting git; deep submodule/stash feature creep.

---

## Risks & technical debt (watch list)

| Risk | Impact |
|------|--------|
| `--redact-paths` is basename-only | Weak privacy for unique folder names |
| Clock skew | Stale labels trust wall clocks |
| `main.go` monolith | Slows testing scan vs sync vs display |
| Hidden-dir skip | Repos under nested `.*` dirs never found (explicit hidden scan roots OK) |
| Windows agent without restart-on-failure | Agent stays down after crash until next logon |

---

## Suggested sequencing

```
Done           Sync lock, matrix, check (+ JSON/exit codes), worktrees, VS Code client
             ↓
Next         Fork A: cd hook around check → prompt/menubar when boring
             ↓
When pain    --quiet/--plain, doctor, CI / LICENSE / go install
```

**Opinionated bottom line:** Aggregate identity and scriptable pre-flight are shipped. Make cross-machine attention show up where you already look (shell / editor), and add packaging when a real consumer appears.

---

## Appendix: priority cheat sheet

| Pri | Item | Owner lens |
|-----|------|------------|
| Next | Fork A: `cd` hook → prompt/menubar | Product |
| When pain | `--quiet` / `--plain`; scan JSON; `doctor` / `--print-config` | CLI |
| When pain | CI / LICENSE / `go install` | Product |
| Later | Windows restart parity | Systems |
| Defer | macOS LaunchAgent; wizards; brew; subcommands; prune; notifications; fold fix-ownership | Product |
