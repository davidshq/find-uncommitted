# Developer review findings

**Date:** 2026-07-24 (updated after sticky config ship)  
**Scope:** Full codebase review (scan CLI, cross-machine sync, agent/schedulers, config, docs, packaging)  
**Participants (personas):** CLI/UX engineer · Systems/sync engineer · Product/platform engineer · Pragmatic engineer

Three developers independently reviewed the project, then reconciled findings. A pragmatic engineer later ranked what actually matters next. Sticky TOML config (`openspec/changes/archive/2026-07-24-add-sticky-toml-config`) is **shipped**.

---

## Shared verdict

`find-uncommitted` already solves a real job: **don’t start conflicting work on machine B while machine A still has dirt.** Local scanning is solid; Git-as-a-sync-bus is a strong, on-brand architecture for a personal tool. Sticky config closed the install→bare-scan gap.

What holds it back is not “more scan dimensions.” It’s **productization and trust**: fix broken first-run docs, make aggregate views correlate the *same* project across machines, harden the shared state-repo clone under concurrent agent+CLI use, and ship a scripting/install story people can leave running.

**North star (agreed):** remain a personal unfinished-Git-work control plane—not a team SaaS, not a full repo manager. Own cross-machine awareness with zero backend.

---

## Roundtable notes

### CLI/UX

The interactive table is good at *triage*, but habit formation is blocked by friction: wrong README build (`go build main.go` fails on the multi-file package), incoherent help vs error paths, nondeterministic local row order, `.` path display that expands then truncates oddly, always-on “may take a while” preamble, emoji/column width issues, and no exit codes / quiet / JSON for scripting. Flag soup mixes scan, agent, and scheduler install—soft subcommands (`scan`, `doctor`, `config show`) can wait, but help and post-install “next steps” should not.

### Systems / sync

Architecture is right for a handful of machines. The reliability gaps that will bite at 24/7 use: **no lock between agent and interactive CLI on the shared clone** (flock only serializes agents), agent ticks ignore context/timeouts (hung git blocks forever), `time.After` in the agent loop, ~720 heartbeat commits/machine/day at a 2m heartbeat, Windows has no crash restart (Linux does), and hostname-only machine IDs collide on cloned VMs. Prefer quieter freshness and serialize git ops on the state clone before adding features.

### Product / platform

The unique wedge is cross-machine awareness—not prettier `git status`. Biggest product miss: aggregate rows keyed by absolute path never correlate laptop vs desktop copies of the same repo. Also missing: LICENSE, CI, releases, `go install`-friendly module path, worktree detection (`.git` *file*), and folding `fix-ownership-tool/` into the main binary. Default 30s agent interval is aggressive for laptops; calmer defaults + `status`/`doctor` make the agent feel operable after install.

### Pragmatic engineer

Personal tool, few machines, one user who already knows git. Optimize for **trust and habit**, not packaging theater. Smallest change that makes you leave the agent running wins.

#### Priority (ruthless)

| When | Do | Why |
|------|-----|-----|
| **This week** | Agent tick timeouts + `Ticker` | Hung git ⇒ process looks healthy and lies forever. |
| **This week** | Calmer default interval (2–5m) + quieter heartbeat | 30s / chatty heartbeats punish laptops and the state repo. Sticky config means one default change sticks. |
| **This week*** | State-repo advisory lock (agent ↔ CLI) | *If the agent is installed 24/7. Only real corruption risk left. Prefer “agent busy → read on-disk snapshots.” |
| **Next** | Path display + sort local rows | Habit friction; not architecture. |
| **Next** | Persist stable `machine_id` in TOML | Hostname collisions are rare and silent. Nasty when they hit. |
| **Later** | Exit codes, JSON/quiet/plain, worktrees, `doctor`, `--print-config`, CI/`go install` | Scripting and ops polish. Add when you actually pipe this or debug install twice. |
| **Later** | Cross-machine identity by `origin` URL | Real product leap—**after** the agent is boring and trustworthy. |
| **Never (now)** | Subcommands, setup wizard, brew/scoop, macOS LaunchAgent, ignore files, notifications, behind-upstream, fold fix-ownership, schema version, machine prune | Feature gravity. Wait for measured pain. |

#### What’s over-scoped

**P0 is a productization wishlist, not a week.** LICENSE + module path + CI are fine hygiene; they do not unblock daily use if you build from a clone. Exit codes as P0 invents a scripting consumer. “UX hygiene” bags a 20-line sort with a help-system redesign—do path + sort, stop.

**P1 buries the lever under flags.** Quieter heartbeat belongs with this week’s interval change, not under `--format json`. `doctor` / `--print-config` / porcelain collapse / Windows restart parity do not increase morning use. Worktrees matter only if you use them.

Identity correlation in “medium-term” is right; elevating it into P1 polish is wrong. Don’t group paths until rows are stable and the agent isn’t lying.

#### Missed / underweighted

- **Fail loud on sticky-config misconfig** — bad `state_repo` silently falling back to local-only trains distrust of aggregate views.
- **One post-install smoke publish** — “your file landed in the state repo” beats any `doctor` command you’ll write this year.
- **Partial/corrupt snapshot JSON** — skip + warn; one bad pull must not blank the aggregate.
- **Treat battery / state-repo history as first-class** — interval + heartbeat are the control knobs; more CLI flags are not.
- **Don’t ship identity correlation on unsorted/ugly paths** — grouping garbage makes a worse morning scan.

**Bottom line:** Fix docs, stop hung ticks, calm the agent, lock the clone. Everything else waits until you miss it twice.

---

## Short-term (1–4 weeks)

Ship trust and habit. Do not expand scan surface area yet.

### P0 — must land next

3. **State-repo advisory lock** — serialize agent publish and CLI pull; prefer “agent busy → use on-disk snapshots” over racing `git` on one index.
4. **Agent tick context + timeouts** — `CommandContext`, cancellable ticks; replace `time.After` with `Ticker`.
5. **UX hygiene** — unified help/errors; fix `.` path display; sort local results by path.
6. **Exit codes** — e.g. `0` clean (under filter), `1` usage/error, `2` attention needed; document them.
7. **Packaging baseline** — LICENSE, module path usable with `go install`, GitHub Actions `go test` + cross-compile.

### P1 — next polish sprint

8. `--plain` / non-TTY / `NO_COLOR` for ASCII-stable output.
9. `--quiet` when scripting or with `--output`.
10. `--format json` (reuse snapshot/aggregate types).
11. Detect git worktrees (`.git` file) + smoke tests.
12. Quieter freshness (heartbeat ≈ `stale-ttl/2` or separate liveness) and calmer default interval (2–5m) with docs for aggressive 30s.
13. Windows task recovery / restart-on-failure parity with systemd.
14. Persist stable `machine_id` in sticky config (don’t rely on hostname alone).
15. Post-install checklist (config path, how to verify agent, sample scan, privacy).
16. `--print-config` / resolved values + sources (already tracked as `ConfigSource`).
17. Collapse per-repo git calls toward `status --porcelain=v2`.

---

## Medium-term (1–3 months)

Make the morning scan answer: **where is unfinished work for this project?**

| Theme | Ideas |
|-------|--------|
| **Cross-machine identity** | Correlate by normalized `origin` URL (+ basename / scan-root-relative fallback); group UI as Project × Machines |
| **Operability** | `doctor` / `status`: lock PID, last publish, config path, systemd/task health, reachability |
| **Noise control** | Ignore file; optional drop of untracked-upstream from attention; sticky `exclude` globs |
| **One binary** | Fold `fix-ownership-tool` into main CLI (`fix-ownership` / prompt on ownership errors) |
| **Platform parity** | macOS LaunchAgent (`scheduler_other.go` is currently a hard decline) |
| **Distribution** | goreleaser, Homebrew / Scoop / winget |
| **Setup** | Guided wizard via `gh` (create private state repo → clone → config → scheduler → smoke publish) |
| **Richer git signal** | Detect *behind* upstream; surface in Changes |
| **CLI shape** | Soft subcommands (`scan`, `agent`, `config`, `doctor`) when flag soup hurts |
| **Power users** | `--paths-only` / `-0` for fzf; wide-terminal columns |
| **Sync hygiene** | Machine retire/prune/rename; snapshot schema `version`; orphan file cleanup |
| **Tests** | Scanner, `checkRepoStatus`, ownership, aggregate CSV/display, agent tick integration (today coverage is strong for snapshot/gitsync/config only) |

---

## Long-term (6+ months)

Stay Git-backed until measured pain (history bloat, push races, scale). Evolution path:

1. **Local truth** — reliable discovery (worktrees, ignores, performance).
2. **Shared awareness** — sticky config done; next is correlated aggregate across machines.
3. **Decision support** — `check <path>` pre-flight (“dirty on laptop, 2h stale”); optional notifications; shell prompt / starship snippet; editor thin client reading JSON + sticky config.
4. **Optional light collaboration** — private state repo already enables couples/small teams; never build accounts.

**Only if Git bus hurts:** squash/shallow/gc policy → object store with conditional PUTs → small HTTP+blob API. Tens of machines × thousands of repos is fine with quiet heartbeats; hundreds of machines wants a non-Git bus.

**Explicitly defer / avoid:** multi-profile config until single-profile hurts; public multi-tenant sync; web SaaS dashboard; rewriting git; deep submodule/stash feature creep before identity + install are done.

---

## Risks & technical debt (watch list)

| Risk | Impact |
|------|--------|
| Concurrent git on shared clone | Index corruption / flaky interactive pulls while agent runs |
| Heartbeat commit volume | State repo becomes the operational bottleneck |
| Hung git / walk | Process “running” but silently stale |
| Hostname `machine_id` | Same ID ⇒ last writer wins across distinct machines |
| Path-only aggregate identity | Cross-machine view is a log dump, not decision support |
| `--redact-paths` is basename-only | Weak privacy for unique folder names |
| Clock skew | Stale labels trust wall clocks |
| `main.go` monolith | Slows testing scan vs sync vs display |
| Hidden-dir skip | Repos under `.*` dirs never found |
| README drift | Build instructions still `main.go`-only; snapshot filename docs outdated |

---

## Suggested sequencing

```
This week   Docs + timeouts/Ticker + calmer interval/heartbeat (+ state-repo lock if agent is 24/7)
             ↓
Next         Path/sort polish + stable machine_id
             ↓
Later        Exit codes / JSON / doctor / CI when you feel the pain
             ↓
Then         Repo identity across machines → aggregate becomes the product
```

**Opinionated bottom line (reconciled):** Sync + sticky config are ahead of packaging theater. Make the agent boring and trustworthy (docs, timeouts, calm cadence, clone lock), tidy the table, then make “same repo on two machines” obvious—and this becomes a tool people leave running.

---

## Appendix: priority cheat sheet

| Pri | Item | Owner lens |
|-----|------|------------|
| This week | Agent tick cancel/timeouts + Ticker | Systems / Pragmatic |
| This week | Calmer default interval + quieter heartbeat | Systems / Pragmatic |
| This week* | State-repo lock (agent ↔ CLI) if agent is 24/7 | Systems / Pragmatic |
| Next | Path display + sort local rows | CLI / Pragmatic |
| Next | Stable `machine_id` in sticky config | Systems / Pragmatic |
| Later | Exit codes, JSON/quiet/plain, worktrees | CLI |
| Later | `doctor` / `--print-config`; CI / LICENSE / `go install` | CLI / Product |
| Later | Correlate repos by remote URL | Product |
| Defer | macOS LaunchAgent; wizards; brew; subcommands; prune; notifications | Product |
