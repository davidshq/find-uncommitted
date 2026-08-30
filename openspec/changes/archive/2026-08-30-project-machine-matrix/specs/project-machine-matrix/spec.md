## ADDED Requirements

### Requirement: Default Project × Machine matrix
For interactive tree scans (not `check` mode), when presenting human-readable results the CLI SHALL print a Project × Machine matrix as the primary section: one row per project identity, one column per known machine. Local machine column headers SHALL be marked (for example with `*`). Cell text SHALL be compact status suitable for columns (at minimum distinguishing clean, dirty, unpushed with count when known, behind/diverged when applicable, empty, error, and stale remote evidence). Branch SHALL appear in a cell only when it differs across machines for that project or otherwise materially aids the glance. The CLI MUST NOT require a separate mode flag (such as `--morning` or `--matrix`) to obtain this primary view.

#### Scenario: Matrix is default tree-scan output
- **WHEN** the user runs a normal tree scan with human output (state repo present or local-only)
- **THEN** stdout’s primary status section is a Project × Machine matrix, not a path-centric Full inventory table

#### Scenario: Cross-machine dirty collision visible in cells
- **WHEN** the same project is dirty on the local machine and dirty on another machine
- **THEN** the matrix row for that project shows dirty (or equivalent) in both machine columns

#### Scenario: No morning mode flag
- **WHEN** the user runs the default tree scan without opt-in inventory flags
- **THEN** the matrix appears without requiring `--morning`, `--matrix`, or similar

### Requirement: Opt-in path inventory
The CLI SHALL provide an opt-in flag (at least `--inventory`) that prints the path-centric inventory table (Machine, Repository path, Branch, Status, Changes), grouped by project identity as today. Default tree-scan output MUST NOT include that path table. `--verbose` MAY enable the same path inventory. When inventory mode is active, the CLI MAY also print Attention nudge prose for situations; default matrix-only output MUST NOT stack Attention + matrix + full path inventory as three primary sections.

#### Scenario: Inventory flag restores path table
- **WHEN** the user passes `--inventory` on a tree scan
- **THEN** output includes the path-centric inventory table with per-clone rows

#### Scenario: Default omits path table
- **WHEN** the user runs a tree scan without `--inventory` (and without verbose-as-inventory)
- **THEN** output does not include the Full inventory path table as a primary section

#### Scenario: No three-hero default
- **WHEN** the user runs a default tree scan (no inventory/verbose inventory)
- **THEN** stdout does not present Attention list, matrix, and full path inventory together as co-equal primary sections

### Requirement: Dirty-only filters matrix projects
When `--dirty-only` is set, the matrix SHALL include only projects that produced at least one situation (same predicate as today’s situation-aware dirty-only), while still scanning clean locals when remotes are enabled so cross-machine cues can fire. Load-error snapshot rows SHALL remain representable (footer or dedicated row/section).

#### Scenario: Dirty-only hides clear projects
- **WHEN** `--dirty-only` is set and a project has no situations
- **THEN** that project does not appear as a matrix row

#### Scenario: Dirty-only still surfaces other-machine work
- **WHEN** `--dirty-only` is set, local is clean for a project, and another machine has dirty or unpushed work for that project
- **THEN** the project appears in the matrix with the remote cell reflecting that work
