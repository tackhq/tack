## ADDED Requirements

### Requirement: Wait for filesystem path to exist
The `wait_for` module with `type: path` SHALL check for the existence of a file or directory on the target system via the connector, polling until the path exists or the timeout is exceeded.

#### Scenario: File appears before timeout
- **WHEN** `wait_for` is invoked with `type: path`, `path: "/var/run/app.pid"`, `timeout: 60`
- **THEN** the module SHALL execute `test -e '/var/run/app.pid'` on the target via the connector every `interval` seconds
- **THEN** when the path exists, the module SHALL return `Changed: true` with `elapsed` and `attempts` in `Result.Data`

#### Scenario: Path does not appear before timeout
- **WHEN** `wait_for` is invoked with `type: path`, `path: "/tmp/ready"`, `timeout: 10`
- **THEN** if the path does not exist within 10 seconds, the module SHALL return an error containing "timeout waiting for path /tmp/ready to exist"

### Requirement: Wait for filesystem path to be absent
The `wait_for` module with `type: path` and `state: stopped` SHALL poll until the specified path no longer exists on the target.

#### Scenario: File is removed before timeout
- **WHEN** `wait_for` is invoked with `type: path`, `path: "/var/lock/deploy.lock"`, `state: stopped`, `timeout: 120`
- **THEN** the module SHALL poll until `test -e` returns non-zero
- **THEN** when the path is absent, the module SHALL return `Changed: true`

#### Scenario: File remains present past timeout
- **WHEN** `wait_for` is invoked with `type: path`, `path: "/var/lock/deploy.lock"`, `state: stopped`, `timeout: 5`
- **THEN** the module SHALL return an error containing "timeout waiting for path /var/lock/deploy.lock to be absent"

### Requirement: Path parameter validation
The module SHALL validate that the `path` parameter is present.

#### Scenario: Missing path parameter
- **WHEN** `wait_for` is invoked with `type: path` and no `path` parameter
- **THEN** the module SHALL return an error with message "required parameter 'path' is missing"

### Requirement: Path check executes on target
Path existence checks SHALL execute on the remote target via the connector, not on the controller.

#### Scenario: Check file on remote host
- **WHEN** `wait_for` is invoked with `type: path`, `path: "/opt/app/ready"`
- **THEN** the existence check SHALL be executed on the target system using `connector.Execute()`
