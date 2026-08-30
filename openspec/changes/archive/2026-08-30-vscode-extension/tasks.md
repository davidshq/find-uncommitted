## 1. CLI JSON check contract

- [x] 1.1 Add `--json` flag wiring for `check` mode (default remains human text)
- [x] 1.2 Define `schemaVersion: 1` JSON structs (project label, machines[], situations[], attention/ok fields) and encode from existing check aggregate + situations
- [x] 1.3 Keep exit codes `0`/`1`/`2`; put non-fatal warnings on stderr; ensure error paths do not emit a false “clear” success JSON
- [x] 1.4 Unit tests for JSON clear, attention, and error cases; human-mode smoke still passes
- [x] 1.5 Document `--json` in README check section

## 2. Extension scaffold

- [x] 2.1 Create `vscode-extension/` package (`package.json`, tsconfig, `.vscodeignore`, activation on workspace)
- [x] 2.2 Add settings: `findUncommitted.binaryPath`, `findUncommitted.checkOnOpen` (default true), optional refresh interval (default off/long)
- [x] 2.3 Add commands: Check Workspace, Refresh, Show Details
- [x] 2.4 README in extension folder: install CLI first, binary path for GUI launches, local VSIX install steps

## 3. Check runner + status UI

- [x] 3.1 Resolve binary (setting then PATH); soft-fail UI when missing
- [x] 3.2 Async `check --json` per workspace folder; cancel/supersede in-flight runs; skip non-git folders quietly
- [x] 3.3 Status bar: quiet clear / quiet local attention / elevated cross-machine attention from situation kinds
- [x] 3.4 Details view (hover and/or Output channel) shows summary + nudge text only — no git mutations
- [x] 3.5 Wire check-on-open (debounced) and manual refresh; do not check on every save

## 4. Docs + dogfood

- [x] 4.1 Short “Editor extension” section in root README linking to `vscode-extension/`
- [ ] 4.2 Manual dogfood: install extension in VS Code or Cursor against a multi-machine project; verify elevated cue for other-machine work, quiet local dirty, default VS Code warning notification on cross-machine attention, and that `attentionDisplay: statusBar` restores footer-only

## 5. Attention notification

- [x] 5.1 Add setting `findUncommitted.attentionDisplay` and wire stock `showWarningMessage` for cross-machine attention (initially shipped as enum `banner`; renamed to `notification` in 5.5)
- [x] 5.2 On cross-machine attention when display is notification mode, show dismissible warning notification with Show Details, Open Settings (focus `attentionDisplay`), and Dismiss; fingerprint the episode so timer/manual refresh does not re-spam
- [x] 5.3 Unit tests for display-mode gating (no notification for local-only / `statusBar` mode) and dismiss/episode suppression
- [x] 5.4 Document notification vs status-bar preference in the extension README
- [x] 5.5 Rename setting enum/default from `banner` → `notification` in `package.json`, code, tests, and READMEs; treat legacy `banner` as `notification` if present; keep behavior (usual VS Code warning notification, not custom banner UI)
