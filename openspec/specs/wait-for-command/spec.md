## ADDED Requirements

### Requirement: Wait for command to succeed
The `wait_for` module with `type: command` SHALL execute a shell command on the target via the connector, polling until it returns exit code 0 or the timeout is exceeded.

#### Scenario: Command succeeds before timeout
- **WHEN** `wait_for` is invoked with `type: command`, `cmd: "pg_isready -h localhost"`, `timeout: 60`, `interval: 5`
- **THEN** the module SHALL execute the command on the target every 5 seconds
- **THEN** when the command returns exit code 0, the module SHALL return `Changed: true` with `elapsed`, `attempts`, `stdout`, and `stderr` in `Result.Data`

#### Scenario: Command never succeeds before timeout
- **WHEN** `wait_for` is invoked with `type: command`, `cmd: "systemctl is-active myapp"`, `timeout: 10`
- **THEN** if the command never returns exit code 0 within 10 seconds, the module SHALL return an error containing "timeout waiting for command to succeed"

#### Scenario: Command execution error
- **WHEN** `wait_for` is invoked with `type: command` and the connector returns a transport-level error (not a non-zero exit code)
- **THEN** the module SHALL return an error immediately without retrying

### Requirement: Command parameter validation
The module SHALL validate that the `cmd` parameter is present.

#### Scenario: Missing cmd parameter
- **WHEN** `wait_for` is invoked with `type: command` and no `cmd` parameter
- **THEN** the module SHALL return an error with message "required parameter 'cmd' is missing"

### Requirement: Command check executes on target
Command checks SHALL execute on the remote target via the connector.

#### Scenario: Command runs on remote host
- **WHEN** `wait_for` is invoked with `type: command`, `cmd: "curl -sf http://localhost:8080/health"`
- **THEN** the command SHALL be executed on the target system using `connector.Execute()`
