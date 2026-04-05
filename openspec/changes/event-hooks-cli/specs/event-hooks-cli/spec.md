## ADDED Requirements

### Requirement: Hook CLI flags
The tack CLI SHALL accept three repeatable flags on the run/execute command: `--on-failure <cmd>`, `--on-success <cmd>`, and `--on-complete <cmd>`. Each flag MAY be specified multiple times. The flags SHALL register commands to invoke at end-of-run.

#### Scenario: Single hook registered
- **WHEN** tack is invoked with `--on-failure "slack-notify fail"`
- **THEN** the command SHALL be registered and executed if the run fails

#### Scenario: Multiple hooks registered
- **WHEN** tack is invoked with `--on-failure "A" --on-failure "B" --on-complete "C"`
- **THEN** all three commands SHALL be registered

#### Scenario: No hooks registered
- **WHEN** tack is invoked without any hook flags
- **THEN** no hook commands SHALL be executed at end-of-run

### Requirement: Hook timeout flag
The tack CLI SHALL accept `--hook-timeout <duration>` (default `30s`) controlling the maximum wall-clock time for each hook. The value SHALL accept Go duration format (e.g. `10s`, `2m`).

#### Scenario: Default timeout
- **WHEN** `--hook-timeout` is not specified
- **THEN** each hook SHALL be limited to 30 seconds

#### Scenario: Custom timeout
- **WHEN** `--hook-timeout 5m` is set
- **THEN** each hook SHALL be limited to 5 minutes

#### Scenario: Invalid duration
- **WHEN** `--hook-timeout notaduration` is set
- **THEN** tack SHALL fail with a parse error before running the playbook

### Requirement: Environment variable equivalents
The tack CLI SHALL support `TACK_ON_FAILURE`, `TACK_ON_SUCCESS`, `TACK_ON_COMPLETE`, and `TACK_HOOK_TIMEOUT` environment variables as equivalents to the CLI flags. When an env var is set, its value SHALL be interpreted as a comma-separated list of commands (literal `,` can be escaped as `\,`). CLI flags SHALL take precedence over environment variables for the same hook type — when a flag is set, the env var for that type SHALL be ignored.

#### Scenario: Env var only
- **WHEN** `TACK_ON_FAILURE="cmd1,cmd2"` is set and no `--on-failure` flag
- **THEN** both `cmd1` and `cmd2` SHALL be registered as failure hooks

#### Scenario: Escaped comma in env
- **WHEN** `TACK_ON_COMPLETE="curl -d a\,b https://x"` is set
- **THEN** one hook SHALL be registered with the comma preserved

#### Scenario: Flag overrides env
- **WHEN** `TACK_ON_FAILURE="env-cmd"` is set and `--on-failure "flag-cmd"` is passed
- **THEN** only `flag-cmd` SHALL be registered for the failure hook type

#### Scenario: Env-only timeout
- **WHEN** `TACK_HOOK_TIMEOUT=10s` is set and no flag is passed
- **THEN** each hook SHALL be limited to 10 seconds

### Requirement: Execution conditions
The system SHALL execute `--on-failure` hooks only when the run status is `failed`, `--on-success` hooks only when the run status is `success`, and `--on-complete` hooks regardless of run status. `--on-complete` hooks SHALL run AFTER the conditional hooks for the same run.

#### Scenario: Run succeeds
- **WHEN** all hosts succeed and `--on-success A --on-failure B --on-complete C` is set
- **THEN** `A` SHALL run, then `C`; `B` SHALL NOT run

#### Scenario: Run fails
- **WHEN** any host fails and `--on-success A --on-failure B --on-complete C` is set
- **THEN** `B` SHALL run, then `C`; `A` SHALL NOT run

### Requirement: Subprocess invocation
Each registered hook SHALL be executed as a subprocess via `/bin/sh -c "<cmd>"` on the control host. The subprocess SHALL inherit tack's environment plus `TACK_RUN_ID`, `TACK_RUN_STATUS`, and `TACK_PLAYBOOK` environment variables.

#### Scenario: Shell features available
- **WHEN** hook command is `echo $TACK_RUN_STATUS | tee /tmp/out`
- **THEN** shell pipes and variable expansion SHALL work

#### Scenario: Convenience env vars
- **WHEN** a hook runs
- **THEN** the subprocess env SHALL contain `TACK_RUN_ID`, `TACK_RUN_STATUS` (`success`/`failed`), and `TACK_PLAYBOOK` set to the playbook path

### Requirement: JSON payload on stdin
The system SHALL write a JSON payload to each hook's stdin and close stdin after writing. The payload SHALL contain `schema_version` (integer, starting at 1), `tack_version` (string, the running tack binary version), `run_id`, `status`, `playbook`, `started_at` (RFC3339), `ended_at` (RFC3339), `duration_ms`, `failed_task_count`, `changed_task_count`, `ok_task_count`, and `hosts` (array of per-host summaries including `name`, `status`, `ok_task_count`, `changed_task_count`, `failed_tasks`, `duration_ms`).

#### Scenario: Schema version present
- **WHEN** any hook runs
- **THEN** the JSON payload SHALL contain `"schema_version": 1`

#### Scenario: Tack version present
- **WHEN** any hook runs
- **THEN** the JSON payload SHALL contain `"tack_version"` set to the running binary's version string

#### Scenario: Run-level fields
- **WHEN** a hook runs after a 2-host playbook
- **THEN** the payload SHALL contain `run_id`, `status`, `playbook`, `started_at`, `ended_at`, `duration_ms`, `failed_task_count`, `changed_task_count`, `ok_task_count`

#### Scenario: Per-host array
- **WHEN** a hook runs
- **THEN** `hosts` SHALL be an array with one entry per host with `name`, `status`, `ok_task_count`, `changed_task_count`, `failed_tasks`, `duration_ms`

#### Scenario: failed_tasks captures failures
- **WHEN** a host failed on task "apt install nginx" with message "E: Unable to locate"
- **THEN** the host's `failed_tasks` SHALL contain `{"task": "apt install nginx", "msg": "E: Unable to locate"}`

#### Scenario: Stdin is closed after write
- **WHEN** the payload has been written
- **THEN** the hook's stdin SHALL be closed

### Requirement: Run status determination
The run-level `status` SHALL be `failed` if any host has `status: failed` or `status: unreachable`. Otherwise it SHALL be `success`. This SHALL also determine which conditional hooks fire.

#### Scenario: All hosts succeeded
- **WHEN** every host completed successfully
- **THEN** run status SHALL be `success`

#### Scenario: One unreachable host
- **WHEN** one host is unreachable
- **THEN** run status SHALL be `failed` and `--on-failure` hooks SHALL fire

#### Scenario: One failed host among many
- **WHEN** 4 hosts succeed and 1 fails
- **THEN** run status SHALL be `failed`

### Requirement: Sequential execution in registration order
When multiple hooks match the run outcome, they SHALL be executed sequentially in the order they were registered on the command line. `--on-complete` hooks SHALL run after the conditional hooks complete.

#### Scenario: Multiple failure hooks
- **WHEN** `--on-failure X --on-failure Y` is set and the run fails
- **THEN** `X` SHALL complete before `Y` starts

#### Scenario: Conditional then complete
- **WHEN** `--on-failure X --on-complete Y` is set and the run fails
- **THEN** `Y` SHALL start only after `X` completes

### Requirement: Timeout with graceful termination
Each hook subprocess SHALL be terminated if it runs longer than the configured timeout. Termination SHALL send SIGTERM, wait 2 seconds, then SIGKILL if the process is still running. A terminated hook SHALL be recorded with a warning naming the command and the timeout duration.

#### Scenario: Hook completes within timeout
- **WHEN** hook finishes in 1 second with a 30s timeout
- **THEN** it SHALL complete normally

#### Scenario: Hook exceeds timeout
- **WHEN** hook runs longer than the timeout
- **THEN** it SHALL receive SIGTERM, and if still running after 2 seconds, SIGKILL

#### Scenario: Timeout warning emitted
- **WHEN** a hook is terminated by timeout
- **THEN** tack SHALL emit a warning to stderr naming the hook command and the timeout

### Requirement: Hook output capture
Hook stdout and stderr SHALL be captured into a combined buffer up to 64KB per hook. In verbose mode (`-v` or higher), the captured output SHALL be printed. When not verbose, captured output SHALL be printed only when the hook exits non-zero. Output exceeding 64KB SHALL be truncated and suffixed with `... [truncated, exceeded 64KB]`.

#### Scenario: Success, non-verbose
- **WHEN** hook exits 0 and tack runs without `-v`
- **THEN** hook output SHALL NOT be printed

#### Scenario: Success, verbose
- **WHEN** hook exits 0 and tack runs with `-v`
- **THEN** hook output SHALL be printed

#### Scenario: Failure, non-verbose
- **WHEN** hook exits non-zero and tack runs without `-v`
- **THEN** hook output SHALL be printed as part of the warning

#### Scenario: Output truncation
- **WHEN** hook prints 100KB to stdout
- **THEN** only the first 64KB SHALL be captured and the truncation suffix SHALL be appended

### Requirement: Hook failures do not change tack exit code
When a hook exits non-zero or times out, tack SHALL emit a warning to stderr but SHALL NOT alter the exit code of the tack process. The tack exit code SHALL always reflect the playbook run's outcome.

#### Scenario: Successful run with failing hook
- **WHEN** the playbook succeeds but `--on-success` hook exits 1
- **THEN** tack SHALL exit 0 and print a warning

#### Scenario: Failed run with failing hook
- **WHEN** the playbook fails (exit code N) and a hook also fails
- **THEN** tack SHALL exit with code N

#### Scenario: Hook warning format
- **WHEN** a hook fails
- **THEN** the warning SHALL include the command and the failure reason (exit code or timeout)

### Requirement: Hooks run after summary flush
Hooks SHALL execute after the normal end-of-run summary has been written to stdout/stderr, so that users see the summary immediately without waiting for hook completion.

#### Scenario: Summary before hook output
- **WHEN** tack runs a play with `--on-complete sleep 1`
- **THEN** the run summary SHALL appear in output before the hook starts

### Requirement: Hooks are control-host only
Hooks SHALL run on the control host (where tack is invoked), NOT on target hosts. The hook command SHALL have no access to target connectors.

#### Scenario: Hook has local env
- **WHEN** a hook reads `$HOME`
- **THEN** it SHALL see the control host's HOME, not any target's HOME

### Requirement: Run ID generation
The system SHALL generate a fresh UUIDv4 per tack invocation, used as `run_id` in the payload and `TACK_RUN_ID` env var.

#### Scenario: run_id is UUIDv4
- **WHEN** a hook runs
- **THEN** `run_id` SHALL match the UUIDv4 format

#### Scenario: run_id is stable within an invocation
- **WHEN** multiple hooks fire in the same run
- **THEN** all hooks SHALL receive the same `run_id` value

### Requirement: Redaction honored in payload
The `failed_tasks[].msg` field SHALL respect the redaction already applied by the output layer (no_log, vault values). Secrets SHALL NOT appear in payloads unredacted.

#### Scenario: no_log task failure
- **WHEN** a failing task has `no_log: true` and `msg` contains "secret=XYZ"
- **THEN** the payload's `failed_tasks[].msg` SHALL be redacted the same way the normal output is
