## Context

`find-uncommitted check <path>` already resolves a work-tree root, live-checks local status, correlates optional remote snapshots via sticky config, prints a compact project × machine summary plus Attention nudges, and exits `0` / `1` / `2`. VS Code’s built-in Git UI does not see other machines. Strategic Fork A wants ambient surfaces; the review findings already name an “editor thin client.” This design makes the extension a **subprocess consumer** of the CLI, not a second implementation of sync or discovery.

## Goals / Non-Goals

**Goals:**

- Ship a VS Code-compatible extension (works in Cursor) that shows cross-machine check status for workspace folders
- Add stable JSON output to `check` so the extension never scrapes human text
- Match the product posture: nudge, don’t strong-arm; other-machine divergence is the loud cue; local dirt stays quieter
- Soft-fail when the binary is missing or the folder isn’t a git work tree

**Non-Goals:**

- Reimplement scanning, agent, or state-repo sync in TypeScript
- Full Project × Machine morning matrix (Fork B) inside the editor
- OS notifications, auto git mutations, or auto-installing shell hooks
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

3. **Primary UI = status bar item; detail on demand**
   - One status-bar item (or one per multi-root folder if needed) with a short glyph/text.
   - Hover / “Show Check Details” command shows the summary line + nudges (Output channel or InformationMessage).
   - Two-tier styling: cross-machine situation kinds (`other_machine_work`, `branch_mismatch`, `tip_mismatch`, `stale_evidence` when tied to remote work) elevate warning/emphasis; pure local dirty/unpushed use quieter text.
   - *Alternative:* always-on Webview panel — deferred; too heavy for “ambient.”

4. **Triggers: open + manual + light refresh**
   - Run check when a workspace folder is added/opened (debounced).
   - Command palette: “Find Uncommitted: Check Workspace”, “Refresh”, “Reveal Output”.
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
   - Aggregate status bar: if any folder has cross-machine attention, that wins the glyph; else if any local attention, quieter cue; else clear/hidden.

## Risks / Trade-offs

- **[Latency]** Live check + optional state-repo pull can take hundreds of ms to a few seconds → debounce, show “checking…” briefly, never block the UI thread; cancel prior runs on refresh.
- **[PATH / GUI apps]** Editors launched from a dock may lack shell `PATH` → document setting override; consider expanding `~/.local/bin` on Linux/macOS as a soft fallback later if needed.
- **[Schema drift]** Extension breaks if JSON fields rename → version field in JSON (`schemaVersion: 1`); keep additive-only changes for v1.
- **[False calm]** Stale remote snapshots can under-warn → surface `stale` on machine cells and situation.stale in UI copy (CLI already labels this).
- **[Scope creep]** Matrix / notifications tempt feature expansion → keep non-goals visible; extension only consumes `check`.

## Migration Plan

1. Land `--json` on `check` with tests; human mode unchanged.
2. Scaffold extension; parse JSON; status bar + commands.
3. Dogfood locally via “Install from VSIX” / `code --install-extension` / Cursor equivalent.
4. Rollback: uninstall extension; CLI unchanged if JSON is additive.

## Open Questions

- Exact status-bar glyph set (plain text vs codicons) — pick something boring and readable; no need to mirror Starship until Fork A prompt work exists.
- Whether Cursor needs any extra activation events beyond standard VS Code — assume VS Code API compatibility unless dogfood shows otherwise.
- Optional: expose `findUncommitted.checkOnOpen` boolean (default true) — yes unless it proves noisy.
