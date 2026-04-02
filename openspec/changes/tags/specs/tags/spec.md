## ADDED Requirements

### Requirement: Tags field on tasks
The system SHALL support a `tags:` field on task definitions. The field SHALL accept either a single string or a list of strings. Tags are case-sensitive labels used for filtering task execution.

#### Scenario: Task with single tag string
- **WHEN** a task specifies `tags: deploy`
- **THEN** the task's tags list SHALL contain `["deploy"]`

#### Scenario: Task with tag list
- **WHEN** a task specifies `tags: [deploy, config]`
- **THEN** the task's tags list SHALL contain `["deploy", "config"]`

#### Scenario: Task without tags
- **WHEN** a task does not specify `tags:`
- **THEN** the task's tags list SHALL be empty

### Requirement: Tags field on plays
The system SHALL support a `tags:` field on play definitions. Play-level tags are inherited by all tasks within the play.

#### Scenario: Play with tags
- **WHEN** a play specifies `tags: [setup]` and contains a task with `tags: [config]`
- **THEN** the task's effective tags SHALL be `["setup", "config"]`

#### Scenario: Play tags inherited by role tasks
- **WHEN** a play specifies `tags: [infra]` and includes a role
- **THEN** all tasks within that role SHALL inherit the `infra` tag

### Requirement: Tags field on blocks
The system SHALL support a `tags:` field on block definitions. Block-level tags are inherited by all tasks within the block, including rescue and always sections.

#### Scenario: Block with tags inherited by block tasks
- **WHEN** a block specifies `tags: [database]` and contains a task with no tags
- **THEN** the task's effective tags SHALL include `["database"]`

#### Scenario: Block tags inherited by rescue tasks
- **WHEN** a block specifies `tags: [deploy]` and has a rescue task with `tags: [rollback]`
- **THEN** the rescue task's effective tags SHALL be `["deploy", "rollback"]`

#### Scenario: Block tags inherited by always tasks
- **WHEN** a block specifies `tags: [deploy]` and has an always task with no tags
- **THEN** the always task's effective tags SHALL include `["deploy"]`

### Requirement: Tags on role references
The system SHALL support tags on role references in play definitions. Role-level tags are inherited by all tasks loaded from the role.

#### Scenario: Role reference with tags
- **WHEN** a play includes a role with `tags: [webserver]`
- **THEN** all tasks in that role SHALL inherit the `webserver` tag

#### Scenario: Role task with own tags plus role tags
- **WHEN** a role is included with `tags: [webserver]` and a role task has `tags: [nginx]`
- **THEN** that task's effective tags SHALL be `["webserver", "nginx"]`

### Requirement: CLI --tags flag
The system SHALL support a `--tags` CLI flag that accepts a comma-separated list of tag names. When `--tags` is specified, only tasks whose effective tags contain at least one of the specified tags SHALL execute.

#### Scenario: Run only tagged tasks
- **WHEN** the user runs with `--tags deploy` and the playbook has tasks tagged `deploy`, `config`, and untagged tasks
- **THEN** only tasks with `deploy` in their effective tags SHALL execute; all others SHALL be skipped

#### Scenario: Multiple tags filter with OR logic
- **WHEN** the user runs with `--tags deploy,config`
- **THEN** tasks with `deploy` OR `config` (or both) in their effective tags SHALL execute

#### Scenario: No --tags flag
- **WHEN** the user runs without `--tags`
- **THEN** all tasks SHALL execute (no tag filtering applied), except those tagged `never`

### Requirement: CLI --skip-tags flag
The system SHALL support a `--skip-tags` CLI flag that accepts a comma-separated list of tag names. Tasks whose effective tags contain any of the specified skip-tags SHALL be skipped.

#### Scenario: Skip tagged tasks
- **WHEN** the user runs with `--skip-tags debug` and the playbook has tasks tagged `debug` and untagged tasks
- **THEN** tasks with `debug` in their effective tags SHALL be skipped; all others SHALL execute

#### Scenario: Combined --tags and --skip-tags
- **WHEN** the user runs with `--tags deploy --skip-tags slow`
- **THEN** tasks matching `deploy` SHALL execute, UNLESS they also match `slow`

### Requirement: Special tag always
Tasks tagged with `always` SHALL execute even when `--tags` filtering is active and the task's other tags do not match the filter. The `always` tag SHALL be overridden only by explicit inclusion in `--skip-tags`.

#### Scenario: Always tag runs despite --tags filter
- **WHEN** the user runs with `--tags deploy` and a task has `tags: [always, setup]`
- **THEN** the task SHALL execute because it is tagged `always`

#### Scenario: Always tag skipped by explicit --skip-tags
- **WHEN** the user runs with `--tags deploy --skip-tags always`
- **THEN** tasks tagged `always` that do not also match `deploy` SHALL be skipped

### Requirement: Special tag never
Tasks tagged with `never` SHALL be skipped during normal execution and when `--tags` filtering does not explicitly include `never` or one of the task's other tags.

#### Scenario: Never tag skipped by default
- **WHEN** the user runs without `--tags` and a task has `tags: [never, debug]`
- **THEN** the task SHALL be skipped

#### Scenario: Never tag runs when explicitly tagged
- **WHEN** the user runs with `--tags debug` and a task has `tags: [never, debug]`
- **THEN** the task SHALL execute because `debug` is explicitly requested

### Requirement: Handlers ignore tag filtering
Handlers SHALL execute when notified by a task, regardless of whether the handler's tags match the active `--tags` filter. Handlers SHALL still respect `--skip-tags`.

#### Scenario: Handler runs when notified despite tag mismatch
- **WHEN** a task tagged `deploy` notifies a handler tagged `restart`, and the user runs with `--tags deploy`
- **THEN** the handler SHALL execute because it was notified

#### Scenario: Handler skipped by --skip-tags
- **WHEN** a handler is tagged `slow` and the user runs with `--skip-tags slow`
- **THEN** the handler SHALL NOT execute even if notified

### Requirement: Tag filtering in plan mode
When running in plan/check mode, tag filtering SHALL apply identically to normal execution. Skipped tasks SHALL be visually indicated in the plan output.

#### Scenario: Plan mode shows tag-filtered tasks
- **WHEN** the user runs in plan mode with `--tags deploy`
- **THEN** only tasks matching `deploy` SHALL appear in the plan, and non-matching tasks SHALL either be omitted or shown as skipped

### Requirement: Tag inheritance accumulation
A task's effective tags SHALL be the union of its own tags and all inherited tags from its play, role, and enclosing block(s). Tags are additive and never subtracted through inheritance.

#### Scenario: Multi-level tag inheritance
- **WHEN** a play has `tags: [infra]`, a role has `tags: [web]`, a block has `tags: [deploy]`, and a task has `tags: [nginx]`
- **THEN** the task's effective tags SHALL be `["infra", "web", "deploy", "nginx"]`
