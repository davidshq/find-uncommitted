## Context

`find-uncommitted check <path>` already resolves a work-tree root, live-checks local status, correlates optional remote snapshots via sticky config, prints a compact project × machine summary plus Attention nudges, and exits `0` / `1` / `2`. VS Code’s built-in Git UI does not see other machines. Strategic Fork A wants ambient surfaces; the review findings already name an “editor thin client.” This design makes the extension a **subprocess consumer** of the CLI, not a second implementation of sync or discovery.

## Goals / Non-Goals

**Goals:**

- Ship a VS Code-compatible extension (works in Cursor) that shows cross-machine check status for workspace folders
- Add stable JSON output to `check` so the extension never scrapes human text
- Match the product posture: nudge, don’t strong-arm; other-machine divergence is the loud cue; local dirt stays quieter
- Make cross-machine attention hard to miss by default (stock VS Code warning notification), with a setting to prefer status-bar-only
- Soft-fail when the binary is missing or the folder isn’t a git work tree

**Non-Goals:**

- Reimplement scanning, agent, or state-repo sync in TypeScript
- Full Project × Machine morning matrix (Fork B) inside the editor
- OS notifications, auto git mutations, or auto-installing shell hooks
- Custom Webview / editor “banner” chrome or persistent decorations as a first surface
- Marketplace publishing / branding polish as a prerequisite for local use
- Bundling or downloading the Go binary inside the VSIX (user installs CLI separately)

## Decisions

1. **Thin client over subprocess, not a port**
   - Extension runs `find-uncommitted check --json <folder>` (exact flag name finalized in CLI work) with the user’s environment so sticky TOML / env vars apply.
   - *Alternative:* reimplement correlation in TS against snapshot files — rejected; duplicates logic and drifts from Attention.
   - *Alternative:* embed WASM/Go — rejected; packaging complexity for a personal utility.

2. **CLI owns the contract; `--json` is additive**
   - Default stdout stays human text. `--json` prints one JSON document to stdout (stderr still used for warnings).
   - Schema includes: `ok` / attention outcome, `project` label, `machines[]` (id, local, stale, branch, status fields), `situations[]` (kind, nudge, machines, stale).
   - Exit codes remain authoritative; extension maps `2` → attention UI, `1` → error/setup, `0` → clear.
   - *Alternative:* NDJSON or separate files — rejected; one shot per folder is enough.

3. **Dual-surface attention UI: VS Code notification default, status bar always ambient**
   - Use the usual editor notification surface: `window.showWarningMessage` (non-modal) for **cross-machine** attention, with action buttons: **Show Details**, **Open Settings**, **Dismiss**. Do not invent a custom “banner” UI — same pattern as other VS Code / Cursor extensions.
   - Status bar remains always-on ambient: short glyph/text; hover / “Show Check Details” shows summary + nudges (Output channel).
   - Setting `findUncommitted.attentionDisplay`: `"notification"` | `"statusBar"` — **default `"notification"`**. When `"statusBar"`, skip the warning notification and rely on the footer only (quiet posture for users who prefer it).
   - **Open Settings** opens the Find Uncommitted settings filtered to `attentionDisplay` so choosing the subtle footer is one click from the notification.
   - **Dismiss** (or closing the notification) suppresses re-showing the same notification for the current attention episode (same aggregated cross-machine fingerprint / until outcome clears). A later clear→attention transition, or a materially different nudge set, may show it again. Timer refreshes that keep the same attention MUST NOT re-spam.
   - Two-tier styling unchanged: cross-machine kinds (`other_machine_work`, `branch_mismatch`, `tip_mismatch`, `stale_evidence` when tied to remote work) drive notification + elevated status bar; pure local dirty/unpushed stay status-bar-quiet (no notification).
   - Setup / missing binary: elevated status bar only (no recurring notification) — one clear footer cue is enough.
   - *Alternative:* status-bar-only as default — rejected; too easy to miss for the Fork A cross-machine habit.
   - *Alternative:* always-on Webview panel or custom banner chrome — deferred / rejected; stock notifications are enough.
   - *Alternative:* OS-level notifications — rejected (non-goal); editor-local only.

4. **Triggers: open + manual + light refresh**
   - Run check when a workspace folder is added/opened (debounced).
   - Command palette: “Find Uncommitted: Check Workspace”, “Refresh”, “Show Details”.
   - Optional timer refresh (default off or long interval, e.g. 5–15m) so we don’t burn CPU on every keystroke.
   - *Not* on every save — that fights the nudge posture and races the agent.

5. **Binary discovery**
   - Setting `findUncommitted.binaryPath` (empty = look up `find-uncommitted` / `find-uncommitted.exe` on `PATH`).
   - On missing binary: status bar shows setup cue once; command opens README snippet — no crash loops.

6. **Repo layout**
   - Place sources under `vscode-extension/` at repo root (package name e.g. `find-uncommitted`), independent of the Go module. Document in root README under a short “Editor” section.
   - Shared development: implement `--json` in Go first (or in the same change before extension parsing), with unit tests on the encoder.

7. **Multi-root workspaces**
   - Check each `workspace.workspaceFolders` entry that resolves as a git work tree; skip non-git folders silently (CLI exit `1` for “not a repo” → no status item noise, or a single debug log).
   - Aggregate surfaces: if any folder has cross-machine attention, that wins the notification (when enabled) and status-bar glyph; else if any local attention, quieter status-bar cue only; else clear/hidden.

## Risks / Trade-offs

- **[Latency]** Live check + optional state-repo pull can take hundreds of ms to a few seconds → debounce, show “checking…” briefly, never block the UI thread; cancel prior runs on refresh.
- **[PATH / GUI apps]** Editors launched from a dock may lack shell `PATH` → document setting override; consider expanding `~/.local/bin` on Linux/macOS as a soft fallback later if needed.
- **[Schema drift]** Extension breaks if JSON fields rename → version field in JSON (`schemaVersion: 1`); keep additive-only changes for v1.
- **[False calm]** Stale remote snapshots can under-warn → surface `stale` on machine cells and situation.stale in UI copy (CLI already labels this).
- **[Scope creep]** Matrix / OS notifications tempt feature expansion → keep non-goals visible; extension only consumes `check` and uses stock editor notifications.
- **[Notification fatigue]** Default warning notification can annoy if re-shown on every refresh → episode fingerprint + dismiss; setting to switch to `"statusBar"`; Open Settings action on the notification itself.

## Migration Plan

1. Land `--json` on `check` with tests; human mode unchanged.
2. Scaffold extension; parse JSON; status bar + commands.
3. Add `attentionDisplay` (default `notification`) and dismissible cross-machine warning notification with Open Settings / Show Details / Dismiss. (Rename any earlier `"banner"` enum value to `"notification"`.)
4. Dogfood locally via “Install from VSIX” / `code --install-extension` / Cursor equivalent — verify notification on other-machine work and that `"statusBar"` restores footer-only.
5. Rollback: uninstall extension; CLI unchanged if JSON is additive.

## Open Questions

- Exact status-bar glyph set (plain text vs codicons) — pick something boring and readable; notification body uses CLI nudge text. No need to mirror Starship until Fork A prompt work exists.
- Whether Cursor needs any extra activation events beyond standard VS Code — assume VS Code API compatibility unless dogfood shows otherwise.
- `findUncommitted.checkOnOpen` boolean (default true) — already decided yes unless dogfood proves noisy.
