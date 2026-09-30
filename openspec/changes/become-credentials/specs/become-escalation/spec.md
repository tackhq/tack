## ADDED Requirements

### Requirement: `become` is an alias of `sudo`
The playbook parser SHALL accept `become:` as an exact alias of `sudo:` at play and task level. When both keys are present on the same play or task with different boolean values, parsing SHALL fail with an error naming both keys. Both spellings SHALL remain supported for the lifetime of the 1.x series.

#### Scenario: Task-level become enables escalation
- **WHEN** a task declares `become: true` and its play does not enable sudo
- **THEN** the task runs with escalation, exactly as if it declared `sudo: true`

#### Scenario: Play-level become
- **WHEN** a play declares `become: true`
- **THEN** the play's effective `Sudo` is `true`

#### Scenario: Conflicting sudo and become
- **WHEN** a task declares `sudo: true` and `become: false`
- **THEN** parsing fails with an error mentioning both `sudo` and `become`

#### Scenario: Agreeing sudo and become
- **WHEN** a play declares `sudo: true` and `become: true`
- **THEN** parsing succeeds and escalation is enabled

### Requirement: Escalation precedence
The effective escalation for a task SHALL be determined by the first set value in this order: task `sudo`/`become`, play `sudo`/`become`, playbook-level default, CLI `-s`/`-b`. A task-level `sudo: false` SHALL disable escalation for that task even when `-s` is passed.

#### Scenario: Task opts out under -s
- **WHEN** tack runs with `-s` and a task declares `sudo: false`
- **THEN** that task runs without escalation and the other tasks run with escalation

### Requirement: `-b/--become` flag
The `run` command SHALL accept `-b/--become` as an alias of `-s/--sudo`, with identical semantics.

#### Scenario: -b escalates all tasks
- **WHEN** tack runs with `-b` against a playbook with no `sudo`/`become` keys
- **THEN** every task runs with escalation

### Requirement: Escalation flags do not trigger a password prompt
`-s/--sudo` and `-b/--become` SHALL only enable escalation. Whether tack prompts for a password SHALL be decided solely by the credential rules (`become-credentials`), never by the presence of these flags.

#### Scenario: -s with passwordless sudo
- **WHEN** tack runs interactively with `-s` against a host where `sudo -n true` succeeds
- **THEN** no password prompt is shown

### Requirement: su and doas are passwordless-only
Tack SHALL NOT send a password for `become_method: su` or `become_method: doas`. When a password source is configured and the method is `su` or `doas`, tack SHALL ignore it for that task and log a warning once per run at `-v`.

#### Scenario: doas with a configured password
- **WHEN** `TACK_BECOME_PASSWORD` is set and a task uses `become_method: doas`
- **THEN** the task's command is not fed any password on stdin
