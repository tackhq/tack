## ADDED Requirements

### Requirement: include_tasks directive
The system SHALL support an `include_tasks:` directive on tasks as an alias for the existing `include:` directive. Both keywords SHALL behave identically — loading and executing an external YAML task file at runtime.

#### Scenario: Basic include_tasks usage
- **WHEN** a play contains a task with `include_tasks: common/install.yml`
- **THEN** the executor SHALL load `common/install.yml` at runtime, parse its task list, and execute those tasks inline
- **AND** the included tasks SHALL have access to the current play context variables

#### Scenario: Existing include directive continues to work
- **WHEN** a play contains a task with `include: common/install.yml`
- **THEN** the executor SHALL behave identically to `include_tasks: common/install.yml`

### Requirement: Variable passing with vars block
The system SHALL support an optional `vars:` block on `include_tasks:` (and `include:`) directives. The specified variables SHALL be merged into the play context for the included tasks only.

#### Scenario: Include with vars block
- **WHEN** a task specifies:
  ```yaml
  - include_tasks: install.yml
    vars:
      package_name: nginx
      version: "1.24"
  ```
- **THEN** the included tasks SHALL have access to `package_name` and `version` variables

#### Scenario: Vars are scoped to the include
- **WHEN** the play has `vars: { package_name: "apache" }` and an `include_tasks` specifies `vars: { package_name: "nginx" }`
- **THEN** within the included tasks, `package_name` SHALL resolve to `"nginx"`
- **AND** after the include completes, `package_name` SHALL revert to `"apache"` in the outer play context

#### Scenario: Vars do not leak to subsequent tasks
- **WHEN** an `include_tasks` directive passes `vars: { temp_var: "value" }` and `temp_var` is not defined in the play
- **THEN** tasks after the include SHALL NOT have access to `temp_var`

### Requirement: Variable interpolation in include paths
The system SHALL support `{{ variable }}` interpolation in `include_tasks:` file paths, resolved at runtime using the current play context.

#### Scenario: Dynamic include path
- **WHEN** a task specifies `include_tasks: "{{ os_family }}/packages.yml"` and `os_family` resolves to `debian`
- **THEN** the executor SHALL load and execute `debian/packages.yml`

#### Scenario: Undefined variable in include path
- **WHEN** a task specifies `include_tasks: "{{ missing_var }}/tasks.yml"` and `missing_var` is not defined
- **THEN** the executor SHALL return an error indicating the variable could not be resolved

### Requirement: Conditional include with when
The system SHALL support `when:` conditions on `include_tasks:` directives. The condition SHALL be evaluated at runtime to determine whether the entire include executes.

#### Scenario: Include with when condition — true
- **WHEN** a task specifies `include_tasks: debian-packages.yml` with `when: facts.os == "debian"` and `facts.os` is `"debian"`
- **THEN** the executor SHALL load and execute the tasks from `debian-packages.yml`

#### Scenario: Include with when condition — false
- **WHEN** a task specifies `include_tasks: debian-packages.yml` with `when: facts.os == "debian"` and `facts.os` is `"centos"`
- **THEN** the executor SHALL skip the include entirely and report it as "skipped"

### Requirement: Loop support on include_tasks
The system SHALL support `loop:` (and `with_items:`) on `include_tasks:` directives, executing the included file's tasks once per loop iteration.

#### Scenario: Include with loop
- **WHEN** a task specifies:
  ```yaml
  - include_tasks: install-package.yml
    loop:
      - nginx
      - redis
    loop_var: pkg
  ```
- **THEN** the executor SHALL execute the tasks from `install-package.yml` twice — once with `pkg: nginx` and once with `pkg: redis`

#### Scenario: Loop variable accessible in included tasks
- **WHEN** an `include_tasks` runs with `loop:` and `loop_var: svc`
- **THEN** included tasks SHALL access the current iteration value via `{{ svc }}` and the index via `{{ loop_index }}`

#### Scenario: Include with loop and vars
- **WHEN** an `include_tasks` has both `loop:` and `vars:` blocks
- **THEN** the `vars:` SHALL be merged alongside the loop variable for each iteration

### Requirement: Path resolution
The system SHALL resolve include paths using the following rules:

1. Absolute paths are used as-is
2. Relative paths are resolved relative to the file containing the `include_tasks:` directive
3. If the task belongs to a role, relative paths are resolved relative to the role's `tasks/` directory

#### Scenario: Relative path from playbook
- **WHEN** a playbook at `/opt/playbooks/deploy.yml` contains `include_tasks: tasks/setup.yml`
- **THEN** the executor SHALL resolve the path as `/opt/playbooks/tasks/setup.yml`

#### Scenario: Relative path within a role
- **WHEN** a role task contains `include_tasks: subtasks.yml` and the role is at `roles/myrole/`
- **THEN** the executor SHALL resolve the path as `roles/myrole/tasks/subtasks.yml`

#### Scenario: Absolute path
- **WHEN** a task specifies `include_tasks: /etc/tack/shared/setup.yml`
- **THEN** the executor SHALL use the path as-is

### Requirement: Circular include detection
The system SHALL detect circular include chains at runtime and report a clear error.

#### Scenario: Direct circular include
- **WHEN** file `a.yml` contains `include_tasks: b.yml` and `b.yml` contains `include_tasks: a.yml`
- **THEN** the executor SHALL return an error indicating a circular include with the chain (e.g., "circular include detected: a.yml → b.yml → a.yml")

#### Scenario: Max recursion depth exceeded
- **WHEN** include nesting exceeds 64 levels (even without a true cycle)
- **THEN** the executor SHALL return an error indicating maximum include depth exceeded

### Requirement: Plan mode display
The system SHALL display `include_tasks:` entries in plan output as a single task entry, since actual tasks are resolved at runtime.

#### Scenario: Plan output for include_tasks
- **WHEN** plan mode processes a task with `include_tasks: setup.yml`
- **THEN** the plan SHALL display a single entry showing the include with status "will_run"

#### Scenario: Plan output for conditional include_tasks
- **WHEN** plan mode processes a task with `include_tasks: setup.yml` and `when:` referencing a registered variable
- **THEN** the plan SHALL display the entry with status "conditional"

### Requirement: Registered variables from included tasks persist
Variables registered by tasks inside an include SHALL persist in the play context after the include completes (unlike `vars:` which are scoped).

#### Scenario: Register in included task
- **WHEN** an included task uses `register: result` and a subsequent task (after the include) references `{{ result.stdout }}`
- **THEN** the variable SHALL be available and resolve correctly
