# Developer review findings

**Date:** 2026-07-24 (updated 2026-08-22 — open items only)  
**Scope:** Full codebase review (scan CLI, cross-machine sync, agent/schedulers, config, docs, packaging)  
**Participants (personas):** CLI/UX engineer · Systems/sync engineer · Product/platform engineer · Pragmatic engineer

Three developers independently reviewed the project, then reconciled findings. A pragmatic engineer later ranked what actually matters next.

**Landed 2026-08-22 (pragmatic “do now”):** state-repo sync lock (agent ↔ CLI), local row sort by path, `.` path display fix, stable `machine_id` persisted on install / first agent when unset.

---

## Shared verdict

`find-uncommitted` already solves a real job: **don’t start conflicting work on machine B while machine A still has dirt.** Local scanning is solid; Git-as-a-sync-bus is a strong, on-brand architecture for a personal tool. Sticky config, origin correlation, git timeouts, calmer agent cadence, state-repo locking, stable machine identity, and a sorted local table are landed.

What holds it back now is not “more scan dimensions.” It’s **productization**: exit codes / JSON for scripting, packaging hygiene, and making aggregate views answer “where’s my unfinished work on *this* project?”

**North star (agreed):** remain a personal unfinished-Git-work control plane—not a team SaaS, not a full repo manager. Own cross-machine awareness with zero backend.

---

## Roundtable notes

### CLI/UX

The interactive table is good at *triage*, but habit formation still has friction: incoherent help vs error paths, always-on “may take a while” preamble, emoji/column width issues, and no exit codes / quiet / JSON for scripting. Flag soup mixes scan, agent, and scheduler install—soft subcommands (`scan`, `doctor`, `config show`) can wait, but help and post-install “next steps” should not.

### Systems / sync

Architecture is right for a handful of machines. Remaining reliability gaps for 24/7 use: Windows has no crash restart (Linux systemd does). State-repo git ops are serialized via flock; hostname collisions are mitigated by auto-generated `machine_id` on install.

### Product / platform

The unique wedge is cross-machine awareness—not prettier `git status`. Origin correlation is in place; the next product leap is making aggregate views answer “where’s my unfinished work on *this* project?” (Project × Machine matrix, `check <path>`). Also missing: LICENSE, CI, releases, `go install`-friendly module path, worktree detection (`.git` *file*), and folding `fix-ownership-tool/` into the main binary. `status`/`doctor` would make the agent feel operable after install.

### Pragmatic engineer

Personal tool, few machines, one user who already knows git. Optimize for **trust and habit**, not packaging theater. Next wins are scripting polish when you actually pipe output, then cashing origin work into the primary aggregate view.

#### Priority (ruthless)

| When | Do | Why |
|------|-----|-----|
| **Next** | Exit codes, JSON/quiet/plain, worktrees, `doctor`, `--print-config`, CI/`go install` | Scripting and ops polish. Add when you actually pipe this or debug install twice. |
| **Next** | Project × Machine matrix + `check <path>` | Cash origin correlation into the morning-scan answer. |
| **Later** | Windows task recovery / restart-on-failure parity with systemd | Only if agent runs 24/7 on Windows |
| **Never (now)** | Subcommands, setup wizard, brew/scoop, macOS LaunchAgent, ignore files, notifications, fold fix-ownership, schema version, machine prune | Feature gravity. Wait for measured pain. |

#### What’s over-scoped

**Packaging baseline is hygiene, not habit.** LICENSE + module path + CI do not unblock daily use if you build from a clone. Exit codes invent a scripting consumer—ship when a script exists.

**P1 buries the lever under flags.** `doctor` / `--print-config` / porcelain collapse do not increase morning use. Worktrees matter only if you use them.

#### Missed / underweighted (landed or still relevant)

- **One post-install smoke publish** — landed; “your file landed in the state repo” beats any `doctor` you’ll write this year.
- **Partial/corrupt snapshot JSON** — landed; skip + warn.
- **Treat battery / state-repo history as first-class** — interval + heartbeat are the control knobs; more CLI flags are not.
- **Don’t ship identity correlation on unsorted/ugly paths** — local sort + path display landed; matrix/`check` can proceed.

**Bottom line:** Cash origin work into Project × Machine. Add scripting/packaging when you feel the pain twice.

---

## Short-term (1–4 weeks)

Ship scripting polish and the next aggregate UX leap. Do not expand scan surface area yet.

### P0 — must land next

1. **Exit codes** — e.g. `0` clean (under filter), `1` usage/error, `2` attention needed; document them.
2. **Packaging baseline** — LICENSE, module path usable with `go install`, GitHub Actions `go test` + cross-compile.

### P1 — next polish sprint

3. `--plain` / non-TTY / `NO_COLOR` for ASCII-stable output.
4. `--quiet` when scripting or with `--output`.
5. `--format json` (reuse snapshot/aggregate types).
6. Detect git worktrees (`.git` file) + smoke tests.
7. Windows task recovery / restart-on-failure parity with systemd.
8. Post-install checklist (config path, how to verify agent, sample scan, privacy).
9. `--print-config` / resolved values + sources (already tracked as `ConfigSource`).
10. Collapse per-repo git calls toward `status --porcelain=v2`.
11. Unified help/errors (remaining UX hygiene beyond path/sort).

---

## Medium-term (1–3 months)

Make the morning scan answer: **where is unfinished work for this project?**

| Theme | Ideas |
|-------|--------|
| **Cross-machine identity (UI)** | Project × Machine matrix; `check <path>` pre-flight; origin grouping exists — cash it into the primary view |
| **Operability** | `doctor` / `status`: lock PID, last publish, config path, systemd/task health, reachability |
| **Noise control** | Ignore file; optional drop of untracked-upstream from attention; sticky `exclude` globs |
| **One binary** | Fold `fix-ownership-tool` into main CLI (`fix-ownership` / prompt on ownership errors) |
| **Platform parity** | macOS LaunchAgent (`scheduler_other.go` is currently a hard decline) |
| **Distribution** | goreleaser, Homebrew / Scoop / winget |
| **Setup** | Guided wizard via `gh` (create private state repo → clone → config → scheduler → smoke publish) |
| **CLI shape** | Soft subcommands (`scan`, `agent`, `config`, `doctor`) when flag soup hurts |
| **Power users** | `--paths-only` / `-0` for fzf; wide-terminal columns |
| **Sync hygiene** | Machine retire/prune/rename; snapshot schema `version`; orphan file cleanup |
| **Tests** | Scanner, `checkRepoStatus`, ownership, aggregate CSV/display, agent tick integration (today coverage is strong for snapshot/gitsync/config only) |

---

## Long-term (6+ months)

Stay Git-backed until measured pain (history bloat, push races, scale). Evolution path:

1. **Local truth** — reliable discovery (worktrees, ignores, performance).
2. **Shared awareness** — correlated aggregate across machines (origin plumbing done; matrix + `check` next).
3. **Decision support** — `check <path>` pre-flight (“dirty on laptop, 2h stale”); optional notifications; shell prompt / starship snippet; editor thin client reading JSON + sticky config.
4. **Optional light collaboration** — private state repo already enables couples/small teams; never build accounts.

**Only if Git bus hurts:** squash/shallow/gc policy → object store with conditional PUTs → small HTTP+blob API. Tens of machines × thousands of repos is fine with quiet heartbeats; hundreds of machines wants a non-Git bus.

**Explicitly defer / avoid:** multi-profile config until single-profile hurts; public multi-tenant sync; web SaaS dashboard; rewriting git; deep submodule/stash feature creep before identity + install are done.

---

## Risks & technical debt (watch list)

| Risk | Impact |
|------|--------|
| `--redact-paths` is basename-only | Weak privacy for unique folder names |
| Clock skew | Stale labels trust wall clocks |
| `main.go` monolith | Slows testing scan vs sync vs display |
| Hidden-dir skip | Repos under `.*` dirs never found |
| Windows agent without restart-on-failure | Agent stays down after crash until next logon |

---

## Suggested sequencing

```
Done           State-repo lock, path/sort, stable machine_id
             ↓
Next         Exit codes / JSON / doctor / CI when you feel the pain
             ↓
Then         Project × Machine matrix + check <path> → aggregate becomes the product
```

**Opinionated bottom line (reconciled):** Sync plumbing and morning-scan table hygiene are in place. Make “same repo on two machines” obvious in the primary view, and add scripting/packaging when a real consumer appears.

---

## Appendix: priority cheat sheet

| Pri | Item | Owner lens |
|-----|------|------------|
| Next | Exit codes, JSON/quiet/plain, worktrees | CLI |
| Next | `doctor` / `--print-config`; CI / LICENSE / `go install` | CLI / Product |
| Next | Project × Machine matrix + `check <path>` | Product |
| Later | Windows restart parity | Systems |
| Defer | macOS LaunchAgent; wizards; brew; subcommands; prune; notifications | Product |
