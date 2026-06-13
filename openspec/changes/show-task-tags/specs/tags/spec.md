## ADDED Requirements

### Requirement: Effective tags displayed in apply output
The system SHALL display each task's effective tags after the task's result line during apply. Effective tags are the union of the task's own tags and inherited play, role, and block tags (the same set `--tags`/`--skip-tags` match against). Tags SHALL be rendered as a bracketed, comma-separated suffix (e.g. `[web,deploy]`) using the dimmed/gray style, so they render as plain text when color is disabled. When a task has no effective tags, no bracket SHALL be shown.

#### Scenario: Task with effective tags shows them after its result
- **WHEN** a task with effective tags `["web", "deploy"]` completes during apply
- **THEN** its result line SHALL include the suffix `[web,deploy]` after the task name

#### Scenario: Task with no tags shows no bracket
- **WHEN** a task with no effective tags completes during apply
- **THEN** its result line SHALL NOT include any tag bracket

#### Scenario: Inherited tags are included in the displayed set
- **WHEN** a play declares `tags: [setup]` and a task declares `tags: [config]`
- **THEN** that task's result line SHALL display `[setup,config]`

#### Scenario: Tags are dimmed and plain under no-color
- **WHEN** color output is disabled (`--no-color`)
- **THEN** the displayed tags SHALL appear as plain bracketed text with no color escape codes

### Requirement: Effective tags displayed in plan preview
The system SHALL display each task's effective tags in the plan preview, for both single-host and multi-host (consolidated) plans, using the same bracketed dimmed suffix as the apply output. When a task has no effective tags, no bracket SHALL be shown.

#### Scenario: Tags shown in single-host plan
- **WHEN** a plan is rendered for a single host and a task has effective tags `["deploy"]`
- **THEN** that task's plan line SHALL include the suffix `[deploy]`

#### Scenario: Tags shown in multi-host plan
- **WHEN** a consolidated multi-host plan is rendered and a task has effective tags `["deploy"]`
- **THEN** that task's plan line SHALL include the suffix `[deploy]`

#### Scenario: Untagged task in plan shows no bracket
- **WHEN** a plan line is rendered for a task with no effective tags
- **THEN** the line SHALL NOT include any tag bracket

### Requirement: Effective tags included in JSON output
The system SHALL include each task's effective tags as an array on the corresponding task event when JSON output is selected (`--output json`).

#### Scenario: JSON task event carries tags
- **WHEN** JSON output is enabled and a task with effective tags `["web", "deploy"]` completes
- **THEN** that task's JSON event SHALL include a `tags` field equal to `["web", "deploy"]`

#### Scenario: JSON task event for untagged task
- **WHEN** JSON output is enabled and a task with no effective tags completes
- **THEN** that task's JSON event SHALL include a `tags` field that is an empty array or omitted
