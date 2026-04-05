## ADDED Requirements

### Requirement: Assert task keyword
The system SHALL recognize `assert` as a built-in task keyword alongside `block`, `include_tasks`, and other control-flow primitives. A task with an `assert` key SHALL be dispatched to the executor's assert handler rather than the module registry.

#### Scenario: Assert task is parsed
- **WHEN** a playbook contains a task with `assert: { that: [...] }`
- **THEN** the parser SHALL accept it as a valid task and the executor SHALL route it to the assert handler

#### Scenario: Assert is not in the module registry
- **WHEN** the module registry is listed
- **THEN** `assert` SHALL NOT appear (it is a built-in keyword, not a registered module)

### Requirement: Conditions evaluated via existing engine
The assert handler SHALL evaluate each expression in `that:` using the same conditional engine used by `when:` (`evaluateConditionExpr` in `internal/executor/conditions.go`). All operators supported by `when:` SHALL be supported: `==`, `!=`, `<`, `>`, `<=`, `>=`, `in`, `not in`, `is defined`, `is not defined`, `and`, `or`, `not`, and parenthesized grouping.

#### Scenario: Simple equality condition passes
- **WHEN** `that: "facts.os_type == 'Linux'"` on a Linux host
- **THEN** the assertion SHALL pass

#### Scenario: Boolean operator in condition
- **WHEN** `that: "facts.os_family == 'Debian' and facts.arch == 'x86_64'"` and both match
- **THEN** the assertion SHALL pass

#### Scenario: Membership operator in condition
- **WHEN** `that: "facts.os_type in ['Linux', 'Darwin']"` on macOS
- **THEN** the assertion SHALL pass

#### Scenario: Is defined check
- **WHEN** `that: "my_var is defined"` and `my_var` was set via `-e my_var=1`
- **THEN** the assertion SHALL pass

### Requirement: `that` accepts string or list
The `that:` parameter SHALL accept either a single condition string or a YAML list of condition strings. Both forms SHALL be treated identically; a single string is equivalent to a list of length one.

#### Scenario: Single condition as string
- **WHEN** task specifies `that: "x == 1"`
- **THEN** the handler SHALL evaluate one condition

#### Scenario: Multiple conditions as list
- **WHEN** task specifies `that: ["x == 1", "y == 2"]`
- **THEN** the handler SHALL evaluate both conditions

### Requirement: Fails task when any condition is false
The assert handler SHALL fail the task if any condition in `that:` evaluates to false. Conditions SHALL be evaluated in order. A failing task SHALL respect `block:`/`rescue:`/`always:` semantics — an assert inside a block SHALL trigger its rescue.

#### Scenario: Single failing condition
- **WHEN** `that: "x == 2"` and `x = 1`
- **THEN** the task SHALL fail

#### Scenario: First of many conditions fails
- **WHEN** `that: ["x == 1", "y == 2"]` and `x = 0`, `y = 2`
- **THEN** the task SHALL fail and report the first failing condition

#### Scenario: Assert inside block triggers rescue
- **WHEN** an `assert` task inside a `block:` fails and the block has a `rescue:`
- **THEN** the rescue tasks SHALL execute

#### Scenario: All conditions pass
- **WHEN** `that: ["x == 1", "y == 2"]` and both match
- **THEN** the task SHALL succeed with Changed=false

### Requirement: Custom failure message
The assert handler SHALL emit `fail_msg` (when provided) as the task failure message. When `fail_msg` is not provided, the handler SHALL emit a default message listing each failing condition's source expression.

#### Scenario: Custom fail_msg
- **WHEN** assert fails and `fail_msg: "OS must be Linux"` is set
- **THEN** the failure message SHALL be `"OS must be Linux"`

#### Scenario: Default fail_msg with single failure
- **WHEN** assert fails with one failing condition `"x == 2"` and no `fail_msg`
- **THEN** the failure message SHALL contain the text `"x == 2"`

#### Scenario: Default fail_msg with multiple failures
- **WHEN** assert fails with conditions `["x == 2", "y == 3"]` both false and no `fail_msg`
- **THEN** the failure message SHALL list both failing condition expressions

### Requirement: Success message and quiet mode
The assert handler SHALL emit `success_msg` (when provided) when all conditions pass. When `quiet: true` is set, the handler SHALL suppress per-condition output on success and emit only a minimal OK indicator.

#### Scenario: Custom success_msg emitted
- **WHEN** assert passes and `success_msg: "preconditions OK"` is set
- **THEN** the task output SHALL contain `"preconditions OK"`

#### Scenario: Quiet mode on success
- **WHEN** assert passes and `quiet: true`
- **THEN** per-condition output SHALL be suppressed

#### Scenario: Quiet mode on failure
- **WHEN** assert fails and `quiet: true`
- **THEN** the failure message SHALL still be emitted

### Requirement: Never changes system state
The assert handler SHALL return a result with `changed: false` on success and SHALL NOT invoke the connector or modify any remote or local state.

#### Scenario: Assert does not invoke connector
- **WHEN** an assert task runs
- **THEN** no command SHALL be dispatched through the connector

#### Scenario: Changed is always false on success
- **WHEN** assert succeeds
- **THEN** the task result SHALL report `changed: false`

### Requirement: Dry-run and check mode
The assert handler SHALL evaluate conditions identically under `--dry-run` / check mode. A failing assert under dry-run SHALL fail the play, matching normal-mode behavior.

#### Scenario: Dry-run evaluates conditions
- **WHEN** tack runs with `--dry-run` and assert conditions are true
- **THEN** the task SHALL succeed

#### Scenario: Dry-run fails on failing assert
- **WHEN** tack runs with `--dry-run` and an assert condition is false
- **THEN** the play SHALL fail

### Requirement: Diff mode is a no-op
The assert handler SHALL be a no-op under `--diff` (no diff output is produced for assert tasks).

#### Scenario: Diff mode produces no assert diff
- **WHEN** tack runs with `--diff` over a playbook with assert tasks
- **THEN** no diff output SHALL be produced for the assert tasks

### Requirement: Integrates with when, tags, register
Assert tasks SHALL support `when:`, `tags:`, and `register:` like any other task.

#### Scenario: `when` false skips assert
- **WHEN** an assert task has `when: false`
- **THEN** the assert SHALL be skipped and conditions SHALL NOT be evaluated

#### Scenario: Tag selection includes/excludes assert
- **WHEN** assert task has `tags: [preflight]` and run with `--tags preflight`
- **THEN** the assert SHALL execute; with `--skip-tags preflight` it SHALL be skipped

#### Scenario: Register captures assert result
- **WHEN** assert task has `register: my_assert` and all conditions pass
- **THEN** `my_assert` SHALL be a map containing `failed: false`, `changed: false`, and `evaluated_conditions` listing each expression and its result

#### Scenario: Registered result on failure (with rescue)
- **WHEN** assert inside a block with rescue fails and has `register: my_assert`
- **THEN** `my_assert` SHALL contain `failed: true` and be accessible to rescue tasks

### Requirement: Parameter validation
The assert handler SHALL return a parse/validation error when `that:` is missing, empty, or not a string/list of strings.

#### Scenario: Missing `that`
- **WHEN** an assert task omits `that:`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Empty `that` list
- **WHEN** `that: []`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Non-string element in list
- **WHEN** `that: ["x == 1", 42]`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Malformed condition expression
- **WHEN** `that: "x =="` (invalid expression)
- **THEN** the task SHALL fail with a condition-parser error naming the expression

### Requirement: Works with every connector
The assert handler SHALL produce identical results regardless of which connector (local, SSH, SSM, Docker) is configured for the play, because no connector interaction occurs.

#### Scenario: Assert on SSH-targeted play
- **WHEN** a play targets an SSH host and contains an assert task
- **THEN** assert SHALL evaluate against the control host's play context and SHALL NOT contact the remote host
