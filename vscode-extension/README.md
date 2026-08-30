# Find Uncommitted — VS Code / Cursor extension

Thin client for the [find-uncommitted](https://github.com/davidshq/find-uncommitted) CLI. It runs `find-uncommitted --json check` on workspace folders and shows a status-bar nudge. It does **not** scan, sync, or run git mutations.

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
3. Confirm **FU · ok** or quiet **FU · dirty** for local-only dirt.
4. With unfinished work published from another machine, confirm elevated **⚠ FU · \<machine\>** and Show Details nudges.
5. Unset `PATH` / wrong `binaryPath` → **FU · setup** without a crash loop (configured vs PATH messages differ).

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

## Status tiers

- **FU · ok** — quiet clear  
- **FU · dirty** — local attention only  
- **⚠ FU · \<machine\>** — elevated cross-machine attention  
- **FU · setup** — binary missing  

No OS notifications; no commit/push/pull from the extension.
