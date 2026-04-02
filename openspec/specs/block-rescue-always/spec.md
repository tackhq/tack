## ADDED Requirements

### Requirement: Block task grouping
The system SHALL support a `block:` directive on tasks that groups a list of tasks to execute as a unit.

#### Scenario: Basic block execution
- **WHEN** a play contains a task with `block:` containing 3 tasks and all succeed
- **THEN** the executor SHALL execute all 3 block tasks sequentially and report success

#### Scenario: Block with name
- **WHEN** a block has `name: "Deploy application"`
- **THEN** the output SHALL display the block name before executing its tasks

### Requirement: Rescue on block failure
The system SHALL support a `rescue:` directive alongside `block:` that executes when any task in the block fails.

#### Scenario: Block fails and rescue runs
- **WHEN** a block task fails and `rescue:` is defined
- **THEN** the executor SHALL stop remaining block tasks, execute all rescue tasks sequentially, and consider the block recovered if rescue succeeds

#### Scenario: Block fails with no rescue
- **WHEN** a block task fails and no `rescue:` is defined
- **THEN** the executor SHALL stop remaining block tasks and propagate the error (subject to `always:` and `ignore_errors`)

#### Scenario: Rescue also fails
- **WHEN** a block task fails, rescue runs, and a rescue task also fails
- **THEN** the executor SHALL propagate the rescue failure error (subject to `always:` and `ignore_errors`)

### Requirement: Always runs regardless
The system SHALL support an `always:` directive alongside `block:` that executes regardless of whether block or rescue succeeded or failed.

#### Scenario: Always runs after block success
- **WHEN** all block tasks succeed
- **THEN** the executor SHALL execute `always:` tasks after the block completes

#### Scenario: Always runs after block failure
- **WHEN** a block task fails (with or without rescue)
- **THEN** the executor SHALL execute `always:` tasks after rescue completes (or after block failure if no rescue)

#### Scenario: Always runs after rescue failure
- **WHEN** both block and rescue fail
- **THEN** the executor SHALL still execute `always:` tasks

### Requirement: Block-level when condition
The system SHALL support `when:` on block tasks. The condition SHALL gate the entire block — if false, block, rescue, and always are all skipped.

#### Scenario: Block with when — true
- **WHEN** a block has `when: facts.os == "linux"` and the condition is true
- **THEN** the executor SHALL execute the block tasks normally

#### Scenario: Block with when — false
- **WHEN** a block has `when: facts.os == "linux"` and the condition is false
- **THEN** the executor SHALL skip the entire block (including rescue and always) and report "skipped"

### Requirement: Block-level sudo inheritance
The system SHALL support `sudo:` on block tasks. The sudo setting SHALL be inherited by all tasks within block, rescue, and always unless overridden at the individual task level.

#### Scenario: Block sudo inherited
- **WHEN** a block has `sudo: true` and a task within block does not specify `sudo:`
- **THEN** the task SHALL execute with sudo enabled

#### Scenario: Task-level sudo override
- **WHEN** a block has `sudo: true` and a task within block has `sudo: false`
- **THEN** the task SHALL execute without sudo

### Requirement: Nested blocks
The system SHALL support blocks within block, rescue, or always sections.

#### Scenario: Block within rescue
- **WHEN** a rescue section contains a `block:` with its own rescue
- **THEN** the nested block SHALL execute with its own error handling independently

### Requirement: Block validation
The system SHALL validate that a task with `block:` does not also specify a module.

#### Scenario: Block with module is rejected
- **WHEN** a task specifies both `block:` and a module (e.g., `command:`)
- **THEN** the parser SHALL return an error indicating that block tasks cannot have a module

#### Scenario: Rescue without block is rejected
- **WHEN** a task specifies `rescue:` without `block:`
- **THEN** the parser SHALL return an error indicating that rescue requires a block

### Requirement: Plan mode display
The system SHALL display block tasks in plan output with their grouped structure.

#### Scenario: Plan shows block structure
- **WHEN** plan mode processes a block task
- **THEN** the plan SHALL display the block name, followed by its block tasks, and indicate rescue/always sections if present

### Requirement: Documentation and examples
The project SHALL include documentation and example playbooks for block/rescue/always.

#### Scenario: README documents block/rescue/always
- **WHEN** a user reads the README
- **THEN** they SHALL find a section explaining block/rescue/always with YAML examples

#### Scenario: Example playbook exists
- **WHEN** a user looks in the examples directory
- **THEN** they SHALL find a playbook demonstrating block/rescue/always with a realistic rollback scenario
