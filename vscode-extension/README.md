# Find Uncommitted — VS Code / Cursor extension

Thin client for the [find-uncommitted](https://github.com/davidshq/find-uncommitted) CLI. It runs `find-uncommitted --json check` on workspace folders and shows status-bar + optional VS Code warning notifications. It does **not** scan, sync, or run git mutations.

## Prerequisites

1. Build and install the Go CLI so `find-uncommitted` is on your **shell** `PATH` (or note its absolute path).
2. Sticky config / state repo as usual if you want cross-machine cues.

Editors launched from a dock/menu often inherit a minimal `PATH`. If the status bar says **FU · setup**, set:

```json
"findUncommitted.binaryPath": "/absolute/path/to/find-uncommitted"
```

## Develop / install locally

```bash
cd vscode-extension
npm install
npm run compile
npm test
```

**Run Extension** from VS Code/Cursor (`F5` with this folder open), or package a VSIX:

```bash
npm run package
# then: Install from VSIX… → find-uncommitted-0.1.0.vsix
# Cursor: same “Install from VSIX” flow
```

### Dogfood checklist

1. Install the VSIX (or F5) in VS Code or Cursor.
2. Open a git workspace that has sticky state-repo remotes configured.
3. Confirm **FU · ok** or quiet **FU · dirty** for local-only dirt (no notification).
4. With unfinished work published from another machine, confirm elevated **⚠ FU · \<machine\>** and a **dismissible warning notification** (default) with Show Details / Open Settings / Dismiss.
5. Set `findUncommitted.attentionDisplay` to `statusBar` → refresh → notification gone, status bar still elevated.
6. Unset `PATH` / wrong `binaryPath` → **FU · setup** without a crash loop (configured vs PATH messages differ).

## Commands

| Command | Action |
|---------|--------|
| Find Uncommitted: Check Workspace | Run check and open the Output channel |
| Find Uncommitted: Refresh | Re-run check quietly (status bar only) |
| Find Uncommitted: Show Details | Open Output channel (nudges only) |

Each `check` subprocess is killed after **30s** so a stuck state-repo pull cannot hang the UI forever.

## Settings

| Setting | Default | Meaning |
|---------|---------|---------|
| `findUncommitted.binaryPath` | `""` | Override CLI path |
| `findUncommitted.checkOnOpen` | `true` | Debounced check when folders open |
| `findUncommitted.refreshIntervalMinutes` | `0` | Periodic refresh; `0` = off |
| `findUncommitted.hideWhenClear` | `false` | Hide status item when clear |
| `findUncommitted.attentionDisplay` | `"notification"` | `"notification"` = usual VS Code warning notification for cross-machine attention (plus status bar); `"statusBar"` = subtle footer only |

## Attention surfaces

By default, **cross-machine** attention uses the usual VS Code warning notification with:

- **Show Details** — Output channel with nudges  
- **Open Settings** — jump to `attentionDisplay` to prefer the status bar only  
- **Dismiss** — hide for this attention episode (refreshes with the same cue won’t re-spam)

Local-only dirty stays on the status bar. Switch to `"statusBar"` if you want the quieter footer for everything.

## Status tiers

- **FU · ok** — quiet clear  
- **FU · dirty** — local attention only  
- **⚠ FU · \<machine\>** — elevated cross-machine attention  
- **FU · setup** — binary missing  

No OS notifications; no commit/push/pull from the extension.
