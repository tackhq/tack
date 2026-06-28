## ADDED Requirements

### Requirement: CLI --roles flag
The system SHALL support a `--roles` CLI flag (short form `-r`) on the `run` command that accepts a comma-separated list of role names. When `--roles` is specified, only tasks that originate from one of the named roles SHALL execute; every other task SHALL be skipped.

#### Scenario: Run only a single named role
- **WHEN** the user runs with `-r web` and the playbook includes roles `web`, `db`, and `cache`
- **THEN** only tasks loaded from the `web` role SHALL execute; tasks from `db` and `cache` SHALL be skipped

#### Scenario: Multiple roles filter with OR logic
- **WHEN** the user runs with `--roles web,db`
- **THEN** tasks loaded from the `web` role OR the `db` role SHALL execute; tasks from all other roles SHALL be skipped

#### Scenario: No --roles flag
- **WHEN** the user runs without `--roles`
- **THEN** no role filtering SHALL be applied and all roles SHALL execute normally

### Requirement: Play-level tasks excluded under roles filter
When `--roles` is specified, tasks defined directly on the play (not loaded from any role) SHALL be skipped, so that the run is restricted to the selected role(s) only.

#### Scenario: Non-role play tasks skipped
- **WHEN** the user runs with `-r web` and the play has both a `web` role and play-level tasks defined under `tasks:`
- **THEN** only the `web` role's tasks SHALL execute and the play-level tasks SHALL be skipped

### Requirement: Roles filter reports skipped tasks
Tasks excluded by the `--roles` filter SHALL be reported as skipped in the run output, consistent with how tag-filtered tasks are reported.

#### Scenario: Skipped task is reported
- **WHEN** a task is excluded because its role is not in the `--roles` list
- **THEN** the task SHALL appear in the output with a skipped status indicating it was filtered by role

### Requirement: Roles filter composes with tag filters
The `--roles` filter SHALL compose with the existing `--tags` and `--skip-tags` filters using AND logic: a task SHALL execute only if it passes the roles filter AND the tag filters.

#### Scenario: Roles and tags combined
- **WHEN** the user runs with `--roles web --tags deploy`
- **THEN** only tasks that belong to the `web` role AND whose effective tags include `deploy` SHALL execute

#### Scenario: Roles filter with skip-tags
- **WHEN** the user runs with `--roles web --skip-tags slow`
- **THEN** tasks belonging to the `web` role SHALL execute unless their effective tags include `slow`

### Requirement: Unknown role names match nothing
A role name passed to `--roles` that does not correspond to any role in the playbook SHALL simply match no tasks rather than causing an error.

#### Scenario: Unknown role name
- **WHEN** the user runs with `-r nope` and the playbook contains only roles `web` and `db`
- **THEN** no tasks SHALL execute due to the roles filter and the run SHALL NOT fail with an error
