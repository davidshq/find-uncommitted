## Context

Scan and sync code invokes git via bare `exec.Command` (`checkRepoStatus`, `repoOriginURL`, `ExecGitRunner`). The agent loop calls `runAgentTick` without a context and waits with `time.After`. Under 24/7 use, a credential prompt or stuck network git leaves the agent process “alive” while snapshots stop updating — the exact trust failure called out in the developer review and strategic reframe.

## Goals / Non-Goals

**Goals:**

- Every git subprocess used by this tool is cancellable via context and subject to a deadline
- Each agent tick has its own deadline; timeout aborts the tick, logs a warning, and the loop continues
- Agent wait uses `time.Ticker` instead of recreating `time.After` each cycle
- Timeout errors are visible (per-repo `Error` for scans; non-fatal warning for agent ticks / sync)

**Non-Goals:**

- Calmer default interval / quieter heartbeat (separate reframe item)
- State-repo advisory lock between agent and CLI
- Collapsing multiple git calls into `status --porcelain=v2`
- Changing snapshot schema or freshness clock semantics
- Optional CLI flags for timeout overrides (constants + docs are enough for v1)

## Decisions

1. **Shared context-aware git runner**
   - Extend `GitRunner` to `Run(ctx, dir, args...)` and implement `ExecGitRunner` with `exec.CommandContext`.
   - Scan helpers (`checkRepoStatus`, `revListCount`, `shortHeadSHA`, `repoOriginURL`) take `context.Context` and use the same runner or a thin `runGit(ctx, ...)` helper so interactive and agent paths share one timeout path.
   - Alternatives considered: only timeout the agent path (rejected — interactive scans hang the same way); wrap only `ExecGitRunner` and leave scan bare (rejected — most hangs are in per-repo status).

2. **Two deadline layers**
   - **Per-command** default (`DefaultGitCommandTimeout`, 30s): bounds a single git invocation.
   - **Per-tick** default (`DefaultAgentTickTimeout`, 2m): parent context for the whole publish tick (pull + scan + publish). Child commands derive from the tick context so cancel propagates.
   - Interactive scans use a root context with the per-command timeout (or a scan-wide deadline derived the same way) so a single hung repo cannot block forever; concurrent workers still respect cancel.
   - Alternatives considered: only per-tick timeout (rejected — a single hung command inside a long tick still burns the whole budget without clear attribution); only per-command (rejected — many sequential git calls can still stall a tick for minutes).

3. **Disable interactive credential prompts + kill process groups**
   - Set `GIT_TERMINAL_PROMPT=0` (and leave stdin nil / `/dev/null`) on spawned git processes so hung waits for TTY credentials fail fast instead of waiting for the context alone.
   - On Unix, start git in its own process group and `Cancel` with `kill(-pid, SIGKILL)` so credential helpers / pager children cannot keep stdout pipes open after the parent dies (plain `CommandContext` only kills the direct child).
   - Alternatives considered: rely only on timeouts (works but wastes the full deadline on every credential hang; without process-group kill, `Wait` can hang until orphaned children exit).

4. **Agent loop: `Ticker` + tick context**
   - Replace `time.After(cfg.Interval)` with `time.NewTicker(cfg.Interval)`; stop on exit.
   - Each iteration: `tickCtx, cancel := context.WithTimeout(parentCtx, tickTimeout)`; defer cancel; pass `tickCtx` through pull/scan/publish.
   - On `tickCtx` deadline exceeded: log warning, do not treat as fatal; next ticker fire proceeds.
   - Parent `signal.NotifyContext` cancel still stops the loop cleanly (including mid-tick via context).

5. **Defaults without new required flags**
   - Ship sensible defaults; optional `--git-timeout` / `--tick-timeout` (or sticky config) only if wiring is cheap — otherwise constants + docs are enough for v1 of this change.
   - Prefer constants first to keep the change focused; expose config in a follow-up if needed.

## Risks / Trade-offs

- **[Large repos / slow disks exceed 30s command timeout]** → Mitigation: timeout is generous for status ops; document override path or raise slightly if real fleets need it; failed repo gets an `Error` string rather than crashing the agent.
- **[Tick timeout mid-publish leaves dirty index in state clone]** → Mitigation: existing sync retry paths; next tick pulls/rebases; same class of failure as kill -9 today, now bounded.
- **[CommandContext leaves orphaned git children holding pipes]** → Mitigation: Unix process-group kill via `Cmd.Cancel`; `WaitDelay` helps release copy goroutines.
- **[Tests that shell out to real git need longer budgets]** → Mitigation: unit-test the runner with a fake slow command / cancelled context; keep scripted `GitRunner` in gitsync tests.

## Migration Plan

- Pure behavior hardening; no snapshot format change; deploy by rebuilding/restarting the agent.
- Rollback: revert to previous binary; no data migration.

## Open Questions

- None blocking: optional CLI/config knobs for timeouts can wait until defaults prove wrong.
