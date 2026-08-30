## ADDED Requirements

### Requirement: Single-project remote correlation for check
When check mode loads remote machine snapshots (state repository resolved and remote loading enabled), the system SHALL correlate the checked local repository with remote entries using the same project identity rules as the aggregate view (normalized origin, else path basename) and SHALL limit presented situations to that project.

#### Scenario: Same origin on another machine appears in check
- **WHEN** the checked local repo has a normalized origin that matches a remote snapshot entry on another machine
- **THEN** check includes that remote machine’s status in its project summary and may surface cross-machine situations for that identity

#### Scenario: Unrelated remote projects omitted
- **WHEN** remote snapshots contain other projects that do not share the checked repo’s correlation key
- **THEN** check output and situations do not include those unrelated projects
