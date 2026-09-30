# Codebase review — 2026-09-29

**Scope:** Go CLI (scan, situations, matrix, `check`), sync/agent/locks/snapshot, config/scheduler/doctor, cd-hook, fix-ownership, CI, and the VS Code extension.  
**Companion:** product backlog in [`developer-review-findings.md`](./developer-review-findings.md).  
**Tests:** `go vet ./...` clean (also `GOOS=windows`/`darwin`). `go test ./...` passes. Extension `npm test`: 51/51.

**Open:** 0 Fix · 14 Maybe + 2 root-cause rows · 3 P3 watch.

---

**Priority:** severity ≠ priority; ranked by damage to the north star (*trustworthy cross-machine awareness*) × how often the path fires.

| Pri | Meaning |
|-----|---------|
| **P0 · Fix now** | Breaks publish/trust on paths you already run. |
| **P1 · Soon** | Wrong answers or false calm/alarms on common daily paths. |
| **P2 · When pain twice** | Real bug, niche trigger or side path. |
| **P3 · Defer** | Smell, hygiene, or documented known risk. |

## Verdict

Publish-boundary and scan false-alarm Fix work has shipped. Remaining items are niche or pain-twice — batch when touching the same file (`gitsync.go` / X-5, discover, extension `check.ts`).

---

## Root causes worth fixing once

| # | Root cause | Fix | Closes |
|---|---|---|---|
| X-1 | Branch/upstream state assembled from 4 separate git calls with exit-code/string heuristics | One `git status --porcelain=v2 --branch -z` per repo | S-10 (and future branch/upstream edge cases) |
| X-5 | Cleanup runs on the ctx that was just cancelled; child processes orphaned on kill | `context.WithoutCancel` + short timeout for cleanup; `signal.NotifyContext` in check mode | Y-2, E-2 |

**Verdict:** X-1 Maybe — optional cleanup now that detached HEAD / gone upstream have targeted fixes. X-5 Maybe (see Y-2, E-2).

---

## Open findings

IDs: **S** = scan/status/situations/check · **Y** = sync/agent/snapshot · **C** = config/scheduler/doctor/hooks · **E** = VS Code extension · **D** = discover / side tools.

### P2 · When pain twice

#### Y-2 · Rebase cleanup never runs once ctx is cancelled

- **Where:** `gitsync.go` → `abortRebaseIfInProgress(ctx, …)`; `internal/gitexec` returns `ctx.Err()` without starting git.
- **Failure:** SIGTERM or tick timeout mid-`pull --rebase` leaves the state clone mid-rebase. The next agent tick self-heals, but if the agent is stopped, viewers only do `pull --ff-only` and read a mid-rebase tree.
- **Fix:** Run cleanup with `context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)`. *(X-5)*
- **Verdict:** Maybe — real, but self-heals on the next agent tick.

#### Y-3 · "Nothing to commit" detection can never match — even in English

- **Where:** `gitsync.go`
- **What's wrong:** `git commit` prints the message to **stdout** (discarded as `_`) with empty stderr; the code searches `stderr + err.Error()` = `"exit status 1"`. The untracked sync lock also changes the message to "nothing added to commit but untracked files present".
- **Failure:** A no-op commit becomes a `SyncWarning` and triggers `restorePublishedSnapshot`. Needs a byte-identical snapshot to fire.
- **Fix:** `git diff --cached --quiet -- <path>` before committing; skip on exit 0. No string matching.
- **Verdict:** Maybe — dead code confirmed, but the trigger is rare.

#### S-5 · Empty repo with an error produces no situation → `check` says ok

- **Where:** `situations.go` (`repo.Error != "" && !repo.IsEmpty`)
- **Failure:** Empty repo whose status probe fails → no situation: `check` exits 0 `ok:true` while the matrix shows "error".
- **Fix:** Drop `!repo.IsEmpty`.
- **Verdict:** Maybe — one-token fix, but needs an empty repo *and* a failing probe.

#### S-7 · `--dirty-only` hides remote-only unfinished work

- **Where:** `situations.go` (`other_machine_work` requires a local row)
- **Failure:** Project dirty/unpushed on desktop but not cloned on laptop → vanishes under `--dirty-only`.
- **Fix:** `remote_only_work` cue when there's no local row.
- **Verdict:** Maybe — the default matrix still shows the row; only the filtered view hides it.

#### S-8 · Remote load errors invisible in `check --json`

- **Where:** `check_json.go`
- **What's wrong:** Corrupt remote snapshots (`LoadError`) are dropped from JSON and situations → exit 0 `ok:true`, stderr warning only. (Usage text now correctly documents exit 2=attention.)
- **Fix:** Add `load_errors` to JSON.
- **Verdict:** Maybe — corrupt snapshots are rare.

#### S-10 · `ls-files --others` without `--directory` walks every untracked file

- **Where:** `main.go`
- **Failure:** Unignored `node_modules`/build output → multi-second listing → 30s timeouts → error.
- **Fix:** Add `--directory --no-empty-directory` (only non-emptiness matters).
- **Verdict:** Maybe — one-flag fix; only bites repos with big unignored trees.

#### S-11 · Summary ignores situation-only cues

- **Where:** `aggregate.go` `printAggregateSummary`
- **Impact:** Counts use `snapshotNeedsAttention`. Tip/branch mismatch with clean trees → `0 need attention` while `--dirty-only` still lists the project.
- **Fix:** Count projects with situations.
- **Verdict:** Maybe — footer false calm on the cross-machine case, but the matrix row above it is correct.

#### E-2 · Timeout/abort SIGKILLs `check` and orphans its `git pull` on the state repo

- **Where:** `vscode-extension/src/check.ts` · Go: `internal/gitexec` (`Setpgid: true`); only agent mode installs `signal.NotifyContext`
- **Failure:** Orphan pull can collide with the agent's next pull (`index.lock`) — one failed tick, then self-heals.
- **Fix:** Extension: SIGTERM, then SIGKILL after ~2s. Go: `signal.NotifyContext` in check mode. *(X-5)*
- **Verdict:** Maybe — real but transient; largely defused by viewer BatchMode + short ConnectTimeout.

#### E-3 · One timeout resets Dismiss → notification comes back

- **Where:** `vscode-extension/src/notification.ts`
- **Failure:** Dismiss → a flaky refresh errors (fingerprint `undefined` clears memory) → next success re-notifies the same episode.
- **Fix:** Reset only on a genuine all-clear (no error outcomes).
- **Verdict:** Maybe — annoying re-spam, few-line fix.

#### E-4 · Hard 30s kill + `~` not expanded (extension)

- **Where:** `vscode-extension/src/check.ts`
- **Impact:** Slow check+pull times out → `FU · error`. `~/bin/...` in `binaryPath` never resolves → perpetual setup state.
- **Fix:** Expand `~`/`$HOME`; make timeout configurable only if timeouts are seen.
- **Verdict:** Maybe — `~` expansion is a two-line fix; the setup message already names the bad path.

#### C-4 · Install/smoke-publish environment ≠ systemd service environment

- **Where:** `main.go` · `scheduler_linux.go` · `config.go` (`os.UserConfigDir`)
- **Failure:** Shell has `XDG_CONFIG_HOME`/`SSH_AUTH_SOCK`, the user systemd manager often doesn't → service finds no config or pushes fail without ssh-agent.
- **Fix:** Bake an absolute `--config` path into the unit; document ssh-agent for systemd.
- **Verdict:** Maybe — real for ssh-agent users; a README note plus `--config` in `ExecStart` is enough.

#### D-1 · `--redact-paths` breaks no-origin correlation

- **Where:** `snapshot.go` `redactPath` · `origin.go` `pathBasenameIdentity`
- **What's wrong:** Paths become `…/basename`; parent is dropped so identity after redact ≠ plain `parent/base`. Two different parents both redacting to `…/app` falsely collide.
- **Fix:** Keep `parent/base` in the redacted path (or hash it); never key solely on post-redact basename.
- **Verdict:** Maybe — only redact-paths + no-origin repos.

#### D-2 · Repos named bin/vendor/obj never discovered

- **Where:** `internal/discover/repos.go` `shouldSkipDir`
- **What's wrong:** `SkipDir` on those basenames means a git root at `…/bin` never reaches `.git`.
- **Fix:** Inspect for `.git` before `SkipDir`.
- **Verdict:** Maybe — silent loss is bad, but such repo names under the scan root are rare.

#### D-3 · fix-ownership writes relative `safe.directory`

- **Where:** `fix-ownership-tool/fix-ownership.go`
- **What's wrong:** Paths are not `Abs`'d. `fix-ownership .` adds `./foo`-style entries that print "Fixed" but do not satisfy git.
- **Fix:** `filepath.Abs` before add; compare Abs forms when checking existing.
- **Verdict:** Maybe — real, one-line fix, but side tool run rarely.

### P3 · Defer

| ID | Finding | Where | Fix |
|---|---|---|---|
| C-7 | zsh doesn't word-split `FIND_UNCOMMITTED_CD_ARGS` → hook fails when the var holds **more than one** arg | `examples/cd-hook/cd-hook.bash` | `${=FIND_UNCOMMITTED_CD_ARGS}` under zsh |
| W-1 | Clock skew: stale labels trust wall clocks; future-dated snapshots never go stale; published-age uses `abs()` | `duration.go`, extension `types.ts` | Clamp future ages / render "clock skew" if seen |
| W-2 | Hidden-dir skip: repos under nested `.*` dirs never found (explicit hidden scan roots OK) | `internal/discover/repos.go` | Opt-in only on measured pain |
