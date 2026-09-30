## ADDED Requirements

### Requirement: Run-scoped credential state
The executor SHALL hold escalation credentials for the duration of a run, keyed by (credential source key, become method, become user). Each entry SHALL be in exactly one state: unset, provided (an empty string counts as provided), or not needed. Credentials SHALL be stored as byte slices and zeroed when the run ends, and SHALL NOT be stored on or serialized from `playbook.Play`.

#### Scenario: Empty password is a provided value
- **WHEN** the user presses Enter at the password prompt
- **THEN** the credential is recorded as provided-empty and the user is not prompted again in the same run

#### Scenario: Multiple plays share one credential
- **WHEN** a playbook has three plays that escalate on the same host with the same method and user
- **THEN** the credential is resolved at most once

### Requirement: Password source precedence
For a given host, tack SHALL resolve the escalation password from the first available source in this order:
1. inventory host/group variable `tack_become_password` (or `ansible_become_password`)
2. `--become-password-file` or `TACK_BECOME_PASSWORD_FILE`
3. `TACK_BECOME_PASSWORD` or `TACK_SUDO_PASSWORD`
4. `--sudo-password=VALUE`
5. play `sudo_password:` after variable interpolation
6. interactive prompt

With `-vv`, tack SHALL log which source was used per host and SHALL NOT log the value.

#### Scenario: Host var overrides CLI file
- **WHEN** host `db1` has `tack_become_password` in inventory and `--become-password-file` is also given
- **THEN** `db1` uses the inventory value and other hosts use the file value

#### Scenario: sudo_password references a vault var
- **WHEN** a play declares `sudo_password: "{{ vault_sudo_pw }}"` and the vault defines `vault_sudo_pw`
- **THEN** the interpolated vault value is used as the password

#### Scenario: Password file from stdin
- **WHEN** tack runs with `--become-password-file -` and the password is piped on stdin
- **THEN** the first line of stdin is used as the password

#### Scenario: Group-readable password file
- **WHEN** the file given to `--become-password-file` is readable by group or others
- **THEN** tack prints a warning and still uses the file

### Requirement: Send password only on demand
Before plan checks, for each host whose tag-filtered tasks escalate with the `sudo` or `doas` method on a password-capable connector (local or SSH), tack SHALL probe once per run with `sudo -n true` (or `doas -n true`). When the probe succeeds, the host's credential SHALL be "not needed" and no password SHALL be sent to it. When the probe cannot run or fails, the host SHALL be treated as needing a password. The probe SHALL be skipped for `su`, SSM, Docker and connections already running as the target user, and when `--no-prompt`/`--no-sudo-prompt` is set.

#### Scenario: NOPASSWD host receives no password
- **WHEN** `TACK_BECOME_PASSWORD` is set and host `web1`'s `sudo -n true` succeeds
- **THEN** no escalated command on `web1` is fed the password on stdin

#### Scenario: Mixed fleet
- **WHEN** `web1` is NOPASSWD and `web2` requires a password, and no source is configured, on a TTY
- **THEN** tack prompts once, and only `web2` receives the password

#### Scenario: Timestamp expires mid-run
- **WHEN** a host probed as not needing a password later fails an escalated command because sudo requires a password, and a non-interactive source is configured
- **THEN** the host's credential switches to that source and the command is retried once with it

#### Scenario: Only brew tasks selected
- **WHEN** tack runs with `--tags brew` and none of the selected tasks escalate
- **THEN** no probe runs and no prompt appears

### Requirement: Single prompt at a fixed point
Tack SHALL prompt for an escalation password at most once per credential key per run. The prompt SHALL appear on the main thread after host preparation (connection, facts, probe) and before plan checks and plan output, or before connecting when `-K/--ask-become-pass` is given. Tack SHALL NOT prompt for an escalation password during the apply phase. Concurrent hosts needing the same key SHALL wait for the single prompt.

#### Scenario: Prompt before plan
- **WHEN** a host needs a password and no source is configured, on a TTY
- **THEN** the prompt appears before any plan lines are printed

#### Scenario: Auto-approve still prompts once
- **WHEN** tack runs with `-a` on a TTY and a host needs a password with no source
- **THEN** tack prompts once before the plan, and does not prompt again during apply

#### Scenario: -K prompts upfront
- **WHEN** tack runs with `-K`
- **THEN** the prompt appears before any host connection is made, and the probe does not trigger a second prompt

### Requirement: Non-interactive runs fail fast
When stdin is not a terminal and a host needs a password with no configured source, that host SHALL fail during preparation with an error that names `--become-password-file`, `TACK_BECOME_PASSWORD` and `tack_become_password`. Other hosts SHALL continue. No password prompt SHALL be attempted.

#### Scenario: CI with NOPASSWD
- **WHEN** tack runs in CI without a TTY or password source and every host's probe succeeds
- **THEN** the run proceeds without error or prompt

#### Scenario: CI missing a password
- **WHEN** tack runs without a TTY, no password source, and host `db1`'s probe fails
- **THEN** `db1` fails before plan checks with the actionable error and no task on `db1` is applied

### Requirement: Wrong password handling
When a password from a non-interactive source is rejected by sudo, tack SHALL mark that credential invalid for the rest of the run and SHALL NOT retry it on any host. A prompted password SHALL be verified against one host before it is used for others. On rejection, tack SHALL re-prompt at most twice, then abort the run before apply.

#### Scenario: Bad password from env across a fleet
- **WHEN** `TACK_BECOME_PASSWORD` is wrong and 50 hosts need a password
- **THEN** at most one authentication attempt is made with it, and every host needing it fails with an "incorrect sudo password" error

#### Scenario: Typo at the prompt
- **WHEN** the user mistypes the password once and then enters it correctly
- **THEN** tack re-prompts once, verifies the second entry, and continues without having sent the first entry to more than one host

### Requirement: Credential CLI surface
The `run` command SHALL provide:
- `-K/--ask-become-pass` (alias `--ask-sudo-pass`)
- `--become-password-file PATH`
- `--no-prompt`, which disables all interactive credential prompts

The following SHALL remain working aliases throughout 1.x:
- `--no-sudo-prompt` and `TACK_SUDO_NO_PROMPT`
- `TACK_SUDO_PASSWORD`
- bare `--sudo-password`, as an alias of `-K`

`--sudo-password=VALUE` SHALL keep working and SHALL print a deprecation warning once per run stating that command-line passwords are visible in process listings and shell history.

#### Scenario: Deprecated inline password
- **WHEN** tack runs with `--sudo-password=hunter2`
- **THEN** the password is used and a deprecation warning is printed that does not contain `hunter2`

#### Scenario: Space-separated sudo password
- **WHEN** tack runs `tack run --sudo-password hunter2 site.yml` and `hunter2` is not an existing file
- **THEN** tack exits with an error explaining that `--sudo-password` takes its value with `=` and suggesting `--become-password-file`

#### Scenario: Literal sudo_password in YAML
- **WHEN** a play declares `sudo_password: plaintext` with no interpolation
- **THEN** the value is used and a warning recommends a vault variable reference

### Requirement: Password never disclosed
The resolved escalation password SHALL NOT appear in:
- command arguments;
- log or debug output;
- plan or diff output;
- JSON output events;
- registered variables;
- `tack export` output;
- error messages.

Any occurrence in captured task output SHALL be replaced with `********`.

#### Scenario: Task echoes the password
- **WHEN** a command task's stdout contains the resolved password and the task is registered
- **THEN** the registered stdout and displayed output contain `********` in place of the password

#### Scenario: Export
- **WHEN** `tack export` runs on a playbook with `sudo_password:` set
- **THEN** the exported script does not contain the password value
