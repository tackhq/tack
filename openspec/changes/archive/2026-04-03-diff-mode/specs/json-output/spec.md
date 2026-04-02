## MODIFIED Requirements

### Requirement: Plan events in JSON mode
In JSON mode, the plan phase SHALL emit `plan_task` events with task preview information including checksum data and optional content diffs.

#### Scenario: Plan output
- **WHEN** `--output json` is specified and the plan phase runs
- **THEN** each planned task SHALL emit a JSON object with `type: "plan_task"`, `host`, `task`, `module`, `action` (will_change/no_change/will_skip/conditional), and relevant parameters

#### Scenario: Plan output with checksums
- **WHEN** `--output json` is specified and a planned task has checksum data
- **THEN** the `plan_task` event SHALL include `old_checksum` and `new_checksum` fields

#### Scenario: Plan output with diff content
- **WHEN** `--output json` is specified with `--diff` and a planned task has content data
- **THEN** the `plan_task` event SHALL include `old_content` and `new_content` string fields

#### Scenario: Plan output without diff flag
- **WHEN** `--output json` is specified without `--diff` and a planned task has content data
- **THEN** the `plan_task` event SHALL NOT include `old_content` or `new_content` fields
