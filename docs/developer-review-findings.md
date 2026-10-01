# Developer review findings

**Date:** 2026-07-24 (updated 2026-09-27 — open items only; triaged 2026-09-29)  
**Scope:** Product / habit backlog (not technical bugs)  
**Participants (personas):** CLI/UX engineer · Systems/sync engineer · Product/platform engineer · Pragmatic engineer  
**Companion:** `strategic-directions.md` (product forks; current bet is Fork A · Ambient Signal) · technical findings: [`codebase-review-2026-09-29.md`](./codebase-review-2026-09-29.md)

Three developers independently reviewed the project, then reconciled findings. A pragmatic engineer later ranked what actually matters next. This file keeps **open product** work only — technical bugs live exclusively in the 09-29 review.

## Shared verdict

**North star (agreed):** remain a personal unfinished-Git-work control plane—not a team SaaS, not a full repo manager. Own cross-machine awareness with zero backend. Habit surface next is Fork A (prompt/menubar) once the shipped `cd` hook is boring.

---

## Roundtable notes (open friction)

### CLI/UX

Full-scan path still has habit friction: always-on “may take a while” preamble, emoji/column width quirks, and no `--quiet` / `--plain` / scan-wide JSON.

#### Priority (ruthless)

| When | Do | Why |
|------|-----|-----|
| **Next (product)** | prompt/menubar once the `cd` hook habit is boring | Fork A — see `strategic-directions.md` |
| **Next (when pain twice)** | `--quiet` / `--plain`, scan-wide JSON | Scripting polish |
| **Never (now)** | Setup wizard, brew/scoop, macOS LaunchAgent, ignore files, notifications, fold fix-ownership, schema version, machine prune | Feature gravity. Wait for measured pain. |

**Verdict (prompt/menubar):** Maybe — product bet, owned by `strategic-directions.md`; land only after the signal in the 09-29 review is trustworthy enough.

---

### Scripting / UX polish (when needed)

1. `--plain` / non-TTY / `NO_COLOR` for ASCII-stable output. **Verdict:** Maybe — only if output gets piped somewhere that chokes.
2. `--quiet` when scripting or with `--output` (defer until the `cd` hook summary line is noise). **Verdict:** Maybe — `check` already covers the scripted path.
3. Scan-wide `--format json` (check already has `--json`). **Verdict:** Maybe — no consumer yet.

---

## Long-term (open)

Stay Git-backed until measured pain (history bloat, push races, scale). Evolution path:

1. **Decision support** — shell prompt / starship snippet; deepen editor thin client as needed.
2. **Optional light collaboration** — private state repo already enables couples/small teams; never build accounts.

**Only if Git bus hurts:** squash/shallow/gc policy → object store with conditional PUTs → small HTTP+blob API. Tens of machines × thousands of repos is fine with quiet heartbeats; hundreds of machines wants a non-Git bus.

**Explicitly defer / avoid:** multi-profile config until single-profile hurts; public multi-tenant sync; web SaaS dashboard; rewriting git; deep submodule/stash feature creep; auto-installed hooks; notifications until prompt habit is real.

**Verdict:** Maybe — roadmap, correctly gated on measured pain; nothing to do now.

---

## Suggested sequencing

```
Now          remaining technical Maybes in codebase-review-2026-09-29.md (as they bite)
             ↓
Next         Fork A: prompt/menubar once cd-hook habit is boring
             ↓
When pain    --quiet/--plain, scan-wide JSON
```

**Opinionated bottom line:** Make cross-machine attention show up where you already look (prompt / menubar / editor) — after the signal is trustworthy.
