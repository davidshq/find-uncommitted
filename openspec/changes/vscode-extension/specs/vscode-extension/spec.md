## ADDED Requirements

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
The extension SHALL expose a status bar indicator reflecting the latest check outcome. Cross-machine attention (situations such as other-machine work, branch mismatch, or tip mismatch) SHALL be visually more prominent than local-only dirty/unpushed/behind cues. When all checked folders are clear (exit `0`, no situations), the indicator SHALL show a quiet clear state or hide according to extension settings.

#### Scenario: Other machine needs attention
- **WHEN** check JSON reports a cross-machine situation while local may be clean
- **THEN** the status bar uses the elevated (attention) presentation and the detail view includes the nudge text

#### Scenario: Local dirty only
- **WHEN** check reports only local dirty/unpushed/behind-style situations
- **THEN** the status bar uses the quieter local presentation (not the cross-machine elevated style)

#### Scenario: Manual refresh
- **WHEN** the user runs the extension’s refresh or check command
- **THEN** the extension re-runs check and updates the status bar from the new result

### Requirement: Nudge-only posture
The extension MUST NOT automatically commit, push, pull, stash, or otherwise mutate git repositories. It MUST NOT enable OS-level notifications by default. Detail surfaces (hover, command, or output channel) SHALL present nudge text from the CLI situations and MUST NOT execute suggested git commands on the user’s behalf.

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
