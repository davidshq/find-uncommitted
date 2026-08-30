## ADDED Requirements

### Requirement: Check mode JSON output
Check mode SHALL support a machine-readable JSON output flag (e.g. `--json`) that, when set, prints a single JSON document to stdout describing the check result instead of the human-readable summary and nudge lines. Human-readable output SHALL remain the default when the flag is unset. Exit codes SHALL remain `0` (no situations), `2` (attention), and `1` (error) regardless of output format.

#### Scenario: JSON on successful clear check
- **WHEN** the user runs `find-uncommitted check --json <path>` and the path is a git work tree with no situations
- **THEN** stdout is a JSON object that includes a schema version, the project label, per-machine status entries, an empty or absent situations list, and the process exits `0`

#### Scenario: JSON on attention
- **WHEN** the user runs `find-uncommitted check --json <path>` and one or more situations apply
- **THEN** stdout JSON includes those situations (each with kind and nudge text at minimum) and the process exits `2`

#### Scenario: JSON on error
- **WHEN** the user runs `find-uncommitted check --json <path>` and the path is not a git work tree (or usage/config aborts the run)
- **THEN** the process exits `1` and does not claim a successful clear project status in stdout JSON as if the check succeeded

#### Scenario: Default remains human text
- **WHEN** the user runs `find-uncommitted check <path>` without the JSON flag
- **THEN** output remains the compact human summary and nudge lines (not a JSON document as the primary stdout)

### Requirement: Stable check JSON contract for consumers
JSON check output SHALL include a `schemaVersion` (integer, starting at `1`) and enough structured fields for an editor client to render project × machine status without scraping text: project label, machine entries (machine id, whether local, whether stale, branch, and dirty/unpushed/behind-style status fields already known to snapshots), and situations (`kind`, `nudge`, related machines, stale flag when applicable). Additive fields in later versions MUST NOT require a schemaVersion bump if older clients can ignore them; renaming or removing fields requires a new schemaVersion.

#### Scenario: Consumer can distinguish local vs remote machine
- **WHEN** check JSON includes both the local machine and at least one remote snapshot row for the project
- **THEN** the local entry is marked local and remote entries are not, so a client can emphasize cross-machine situations

#### Scenario: Warnings stay on stderr
- **WHEN** check JSON mode runs and the tool emits a non-fatal warning (e.g. state repo busy, pull failed)
- **THEN** that warning is written to stderr and stdout remains parseable JSON for the check result
