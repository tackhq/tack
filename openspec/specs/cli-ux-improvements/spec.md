## ADDED Requirements

### Requirement: Remove non-functional forks flag
The `--forks` / `-f` flag SHALL be removed from the CLI until parallel execution is implemented.

#### Scenario: User passes --forks
- **WHEN** user runs `bolt run playbook.yaml --forks 5`
- **THEN** bolt SHALL return an "unknown flag" error

### Requirement: Flexible approval prompt
The approval prompt SHALL accept case-insensitive "y" or "yes" as affirmative responses.

#### Scenario: User types "y"
- **WHEN** the approval prompt is shown and user types "y"
- **THEN** bolt SHALL proceed with apply

#### Scenario: User types "YES"
- **WHEN** the approval prompt is shown and user types "YES"
- **THEN** bolt SHALL proceed with apply

#### Scenario: User types "no"
- **WHEN** the approval prompt is shown and user types "no"
- **THEN** bolt SHALL abort without applying

#### Scenario: User types empty string
- **WHEN** the approval prompt is shown and user presses Enter without input
- **THEN** bolt SHALL abort without applying (no default-yes)

### Requirement: Unified dry-run flags
Both `--dry-run` and `--check` SHALL be persistent flags available to all commands, and SHALL behave identically.

#### Scenario: --check on run command
- **WHEN** user runs `bolt run playbook.yaml --check`
- **THEN** bolt SHALL show the plan without applying, identical to `--dry-run`

### Requirement: Per-module help command
A `bolt module <name>` subcommand SHALL display module documentation including parameters and descriptions.

#### Scenario: Module exists
- **WHEN** user runs `bolt module apt`
- **THEN** bolt SHALL display the apt module's parameters, types, defaults, and descriptions

#### Scenario: Module not found
- **WHEN** user runs `bolt module nonexistent`
- **THEN** bolt SHALL return an error listing available modules

#### Scenario: No module name given
- **WHEN** user runs `bolt module` with no arguments
- **THEN** bolt SHALL list all available modules (same as `bolt modules`)

### Requirement: Dedicated diff flag
Bolt SHALL support a `--diff` persistent flag that enables file content diff display in plan output, independent of `--verbose`.

#### Scenario: --diff flag accepted
- **WHEN** user runs `bolt run playbook.yaml --diff`
- **THEN** bolt SHALL show file content diffs in the plan output for tasks that change files

#### Scenario: --diff combined with --dry-run
- **WHEN** user runs `bolt run playbook.yaml --diff --dry-run`
- **THEN** bolt SHALL show file content diffs in the plan and stop without applying

#### Scenario: --diff combined with --auto-approve
- **WHEN** user runs `bolt run playbook.yaml --diff --auto-approve`
- **THEN** bolt SHALL show file content diffs in the plan and proceed to apply without prompting

#### Scenario: --diff with --verbose
- **WHEN** user runs `bolt run playbook.yaml --diff --verbose`
- **THEN** bolt SHALL show file content diffs (both flags enable diff display, no conflict)
