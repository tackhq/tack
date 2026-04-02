## ADDED Requirements

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
