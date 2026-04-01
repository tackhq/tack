## ADDED Requirements

### Requirement: JSON output flag
Bolt SHALL support an `--output` flag accepting `text` (default) or `json`. When `json` is specified, all execution output SHALL be emitted as newline-delimited JSON (NDJSON).

#### Scenario: Default output
- **WHEN** `--output` is not specified
- **THEN** output SHALL be human-readable text (current behavior)

#### Scenario: JSON output
- **WHEN** `--output json` is specified
- **THEN** output SHALL be newline-delimited JSON objects

#### Scenario: Invalid output mode
- **WHEN** `--output xml` is specified
- **THEN** bolt SHALL return an error listing valid modes

### Requirement: JSON event schema
Each JSON line SHALL be a self-contained object with at minimum: `type` (string) and `timestamp` (ISO 8601 string).

#### Scenario: Task result event
- **WHEN** a task completes during JSON output
- **THEN** the emitted JSON object SHALL contain: `type: "task_result"`, `timestamp`, `host`, `task` (name), `module`, `status` (ok/changed/failed/skipped), `changed` (bool), `message`, and `data` (if registered output exists)

#### Scenario: Recap event
- **WHEN** a play completes during JSON output
- **THEN** the emitted JSON object SHALL contain: `type: "host_recap"`, `host`, `ok`, `changed`, `failed`, `skipped`, and `duration` (seconds)

### Requirement: Auto-approve in JSON mode
JSON output mode SHALL automatically enable `--auto-approve` behavior, skipping interactive approval prompts.

#### Scenario: JSON mode skips prompt
- **WHEN** `--output json` is specified without `--auto-approve`
- **THEN** the executor SHALL skip the approval prompt and proceed directly to apply

### Requirement: Error output separation
In JSON mode, structured events SHALL go to stdout and error messages SHALL go to stderr.

#### Scenario: Execution error
- **WHEN** a fatal error occurs during JSON output
- **THEN** the error message SHALL be written to stderr, and a final JSON event with `type: "error"` and the error message SHALL be written to stdout

### Requirement: Plan events in JSON mode
In JSON mode, the plan phase SHALL emit `plan_task` events with task preview information.

#### Scenario: Plan output
- **WHEN** `--output json` is specified and the plan phase runs
- **THEN** each planned task SHALL emit a JSON object with `type: "plan_task"`, `host`, `task`, `module`, `action` (will_change/no_change/will_skip/conditional), and relevant parameters
