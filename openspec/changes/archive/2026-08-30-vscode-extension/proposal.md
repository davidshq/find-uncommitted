## Why

`check <path>` already delivers the cross-machine pre-flight nudge that strategic Fork A cares about, but it only lives in the terminal. Most coding time is spent inside VS Code / Cursor, where built-in Git status is **local-only** — so “dirty on the other machine” stays invisible until you remember to run the CLI. An editor thin client is the natural ambient surface for the habit we already want to prove, and the review findings already list “editor thin client reading JSON + sticky config” under decision support.

## What Changes

- Add a **VS Code / Cursor extension** that shells out to the installed `find-uncommitted` binary (sticky config intact) — no reimplementation of scan/sync logic in TypeScript
- On workspace open, folder change, and on demand: run `check` for each workspace folder that is a git work tree and surface outcomes via a **status-bar glyph** (always ambient) plus, by default, a **usual VS Code warning notification** for cross-machine attention
- Setting `findUncommitted.attentionDisplay` (`notification` | `statusBar`, default `notification`) so users who prefer the subtle footer can opt out of the notification; notification actions include Open Settings, Show Details, and Dismiss
- Two-tier signal matching Fork A: local dirty/unpushed stays quiet (status bar only); **other-machine divergence** is the prominent cue (notification when enabled)
- **Never** auto-commit, auto-push, auto-pull, or fire OS notifications by default; nudges use editor-local status bar + stock VS Code notifications only (same posture as Attention)
- Extend CLI `check` with a **machine-readable JSON** output mode so the extension does not scrape human text
- Document binary discovery (`PATH`, setting override) and a minimal “install CLI first” path in the extension README

## Capabilities

### New Capabilities
- `vscode-extension`: Editor thin client — status bar / optional attention notification / commands / settings that invoke `find-uncommitted check` against workspace folders and display project × machine status plus Attention nudges without owning sync or discovery

### Modified Capabilities
- `check-path-preflight`: Add stable JSON (or equivalent structured) output for check mode so non-CLI consumers can parse project label, per-machine cells, situations, and attention outcome without scraping stdout text

## Impact

- **CLI:** small additive surface on `check` (`--json` or similar); exit codes `0`/`1`/`2` stay authoritative; human text mode remains default
- **Repo layout:** new `vscode-extension/` (or `extensions/vscode/`) package — TypeScript, `package.json` contributes, independent of Go module build
- **Runtime dependency:** user must have `find-uncommitted` on `PATH` (or configured path); extension fails soft with setup guidance if missing
- **Not in scope:** full multi-repo morning matrix UI, menubar/Starship, auto-installed hooks, marketplace publishing theater, reimplementing agent/sync in the extension
