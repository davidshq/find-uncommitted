# vscode-extension Specification

## Purpose

VS Code / Cursor thin client that shells out to `find-uncommitted check --json` for workspace folders and surfaces cross-machine Attention via status bar and optional stock editor warning notifications — without reimplementing sync, discovery, or git mutations.

## Requirements

### Requirement: Extension invokes CLI check for workspace folders
The VS Code-compatible extension SHALL invoke the installed `find-uncommitted` binary in `check` mode with machine-readable JSON against each eligible workspace folder (git work tree), using the user’s environment so sticky config and env vars apply. The extension MUST NOT reimplement repository scanning, state-repo sync, or situation detection in the extension host.

#### Scenario: Check on workspace open
- **WHEN** the user opens a workspace folder that is inside a git work tree and the binary is available
- **THEN** the extension runs check for that folder and updates its UI from the JSON result and exit code

#### Scenario: Multi-root workspace
- **WHEN** the workspace has multiple folders that are git work trees
- **THEN** the extension checks each such folder (not sibling repos outside those folders)

#### Scenario: Non-git folder
- **WHEN** a workspace folder is not inside a git work tree
- **THEN** the extension does not treat that as a hard failure of the whole extension (no repeated error spam); it skips or quietly records that folder

### Requirement: Status bar ambient signal
The extension SHALL expose a status bar indicator reflecting the latest check outcome. Cross-machine attention (situations such as other-machine work, branch mismatch, or tip mismatch) SHALL be visually more prominent than local-only dirty/unpushed/behind cues. When all checked folders are clear (exit `0`, no situations), the indicator SHALL show a quiet clear state or hide according to extension settings. The status bar SHALL remain available regardless of `findUncommitted.attentionDisplay`.

#### Scenario: Other machine needs attention
- **WHEN** check JSON reports a cross-machine situation while local may be clean
- **THEN** the status bar uses the elevated (attention) presentation and the detail view includes the nudge text

#### Scenario: Local dirty only
- **WHEN** check reports only local dirty/unpushed/behind-style situations
- **THEN** the status bar uses the quieter local presentation (not the cross-machine elevated style)

#### Scenario: Manual refresh
- **WHEN** the user runs the extension’s refresh or check command
- **THEN** the extension re-runs check and updates the status bar from the new result

### Requirement: Dismissible attention notification
The extension SHALL support `findUncommitted.attentionDisplay` with values `notification` and `statusBar`, defaulting to `notification`. When the setting is `notification` and check reports cross-machine attention, the extension SHALL show a non-modal VS Code warning notification (`window.showWarningMessage`) that includes nudge text and action buttons for Show Details, Open Settings, and Dismiss — the usual editor notification UI, not a custom banner. When the setting is `statusBar`, the extension MUST NOT show that warning notification for attention outcomes and SHALL rely on the status bar (and detail commands) only. Local-only attention MUST NOT trigger the notification. After Dismiss, the extension MUST NOT re-show the same notification for the same attention episode on subsequent refreshes that keep equivalent cross-machine attention; a clear-then-attention transition or a materially different attention set MAY show the notification again. Open Settings SHALL open the Find Uncommitted configuration focused on `attentionDisplay` (or the extension’s settings section that includes it).

#### Scenario: Notification on cross-machine attention (default)
- **WHEN** `attentionDisplay` is `notification` (or unset) and check reports cross-machine attention
- **THEN** the extension shows a dismissible warning notification with Show Details, Open Settings, and Dismiss actions

#### Scenario: Status-bar-only mode
- **WHEN** the user sets `findUncommitted.attentionDisplay` to `statusBar` and check reports cross-machine attention
- **THEN** the extension does not show the warning notification and still updates the elevated status bar

#### Scenario: Local dirty does not notify
- **WHEN** check reports only local dirty/unpushed/behind-style situations
- **THEN** the extension does not show the attention notification

#### Scenario: Dismiss suppresses re-spam
- **WHEN** the user dismisses the notification and a later refresh reports the same cross-machine attention episode
- **THEN** the extension does not show the notification again until attention clears or the attention set changes materially

#### Scenario: Open Settings from notification
- **WHEN** the user chooses Open Settings on the notification
- **THEN** the editor opens Find Uncommitted settings so the user can switch `attentionDisplay` to `statusBar`

### Requirement: Nudge-only posture
The extension MUST NOT automatically commit, push, pull, stash, or otherwise mutate git repositories. It MUST NOT enable OS-level notifications by default. Detail surfaces (hover, command, output channel, or notification actions) SHALL present nudge text from the CLI situations and MUST NOT execute suggested git commands on the user’s behalf.

#### Scenario: Showing details does not run git mutations
- **WHEN** the user opens check details from the extension
- **THEN** the extension only displays status/nudge information from the last check (or re-runs read-only check) and does not run commit/push/pull

### Requirement: Binary discovery and soft failure
The extension SHALL resolve the CLI via a user setting for binary path when set, otherwise via `PATH` lookup of the platform binary name. When the binary is missing or not executable, the extension SHALL show a clear setup cue and MUST NOT crash the extension host or retry in a tight loop.

#### Scenario: Missing binary
- **WHEN** the configured or PATH binary cannot be found
- **THEN** the status UI indicates setup is required and offers guidance to install or configure the path

#### Scenario: Custom binary path
- **WHEN** the user sets an absolute path to the `find-uncommitted` binary in extension settings
- **THEN** the extension invokes that path for check commands

### Requirement: Debounced non-blocking checks
Check invocations from the extension SHALL run asynchronously without blocking the UI thread. Overlapping refresh requests for the same folder SHALL cancel or supersede in-flight work so stale results do not overwrite newer ones. The extension MUST NOT run check on every editor save by default.

#### Scenario: Rapid refresh
- **WHEN** the user triggers refresh twice in quick succession
- **THEN** the UI converges on the latest completed check result without applying an older superseded result afterward
