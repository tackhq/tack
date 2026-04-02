## ADDED Requirements

### Requirement: README documents include_tasks
The project README SHALL include a section documenting the `include_tasks:` directive with usage examples covering basic inclusion, `vars:` passing, conditional includes, and loop-driven includes.

#### Scenario: README includes directive documentation
- **WHEN** a user reads the README
- **THEN** they SHALL find a section explaining `include_tasks:` with YAML examples for basic usage, vars, when conditions, and loops

### Requirement: Example playbooks for task inclusion
The project SHALL include example playbooks demonstrating `include_tasks:` usage patterns.

#### Scenario: Example for include_tasks with vars
- **WHEN** a user looks in the examples directory
- **THEN** they SHALL find a playbook demonstrating `include_tasks:` with a `vars:` block and a separate task file being included

#### Scenario: Example for conditional include_tasks
- **WHEN** a user looks in the examples directory
- **THEN** they SHALL find a playbook demonstrating `include_tasks:` with a `when:` condition

#### Scenario: Example for loop-driven include_tasks
- **WHEN** a user looks in the examples directory
- **THEN** they SHALL find a playbook demonstrating `include_tasks:` with `loop:` to include the same file multiple times with different variables

### Requirement: Document include_tasks vs include
The documentation SHALL note that `include:` and `include_tasks:` are equivalent, and that `include_tasks:` is the preferred keyword for consistency with Ansible conventions.

#### Scenario: User understands keyword equivalence
- **WHEN** a user reads the include documentation
- **THEN** they SHALL find a note explaining that `include:` and `include_tasks:` behave identically, with `include_tasks:` being the recommended form
