## MODIFIED Requirements

### Requirement: Context-deadline git subprocesses
The system SHALL execute every git subprocess used for repository scanning and state-repository sync via a cancellable context with a per-command deadline. When the deadline expires or the parent context is cancelled, the system SHALL terminate the subprocess and return an error rather than blocking indefinitely. Non-interactive git invocations SHALL disable terminal credential prompts so credential waits fail instead of hanging on a TTY. Timeout and cancellation failures SHALL be reported distinctly from generic git fatal errors and MUST NOT be relabeled as invalid-repository or upstream-configuration failures.

#### Scenario: Command exceeds deadline
- **WHEN** a git subprocess does not complete before the configured per-command deadline
- **THEN** the process is cancelled and the caller receives a timeout/cancellation error

#### Scenario: Parent context cancelled
- **WHEN** the parent context for a git invocation is cancelled before the command finishes
- **THEN** the subprocess is terminated and the caller receives a cancellation error

#### Scenario: Scan records timeout as repo error
- **WHEN** a per-repository status check fails because a git command timed out or was cancelled
- **THEN** that repository's status includes a non-empty Error describing timeout or cancellation (not a generic invalid-repository or opaque exit code alone) and other repositories continue to be checked

#### Scenario: Credential prompt does not hang forever
- **WHEN** git would otherwise wait for an interactive terminal credential prompt during a scan or sync
- **THEN** the invocation does not block indefinitely waiting for TTY input
