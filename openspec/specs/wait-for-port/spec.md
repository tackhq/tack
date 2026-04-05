## ADDED Requirements

### Requirement: Wait for TCP port to become reachable
The `wait_for` module with `type: port` SHALL attempt a TCP connection to the specified `host` and `port` repeatedly until the connection succeeds or the `timeout` is exceeded.

#### Scenario: Port becomes available before timeout
- **WHEN** `wait_for` is invoked with `type: port`, `host: "localhost"`, `port: 8080`, `timeout: 30`, `interval: 2`
- **THEN** the module SHALL attempt a TCP connection to `localhost:8080` every 2 seconds
- **THEN** when the connection succeeds, the module SHALL return `Changed: true` with `elapsed` and `attempts` in `Result.Data`

#### Scenario: Port does not become available before timeout
- **WHEN** `wait_for` is invoked with `type: port`, `host: "localhost"`, `port: 8080`, `timeout: 10`
- **THEN** if no successful TCP connection is made within 10 seconds, the module SHALL return an error with message containing "timeout waiting for port 8080 on localhost"

#### Scenario: Default host
- **WHEN** `wait_for` is invoked with `type: port`, `port: 3000` and no `host` parameter
- **THEN** the module SHALL default `host` to `"localhost"`

### Requirement: Wait for TCP port to become unreachable
The `wait_for` module with `type: port` and `state: stopped` SHALL poll until a TCP connection to the specified `host` and `port` is refused.

#### Scenario: Port closes before timeout
- **WHEN** `wait_for` is invoked with `type: port`, `host: "localhost"`, `port: 8080`, `state: stopped`, `timeout: 30`
- **THEN** the module SHALL attempt TCP connections until the connection is refused
- **THEN** when the connection is refused, the module SHALL return `Changed: true`

#### Scenario: Port remains open past timeout
- **WHEN** `wait_for` is invoked with `type: port`, `host: "localhost"`, `port: 8080`, `state: stopped`, `timeout: 5`
- **THEN** if the port is still accepting connections after 5 seconds, the module SHALL return an error containing "timeout waiting for port 8080 on localhost to close"

### Requirement: Port parameter validation
The module SHALL validate that required parameters are present and well-formed.

#### Scenario: Missing port parameter
- **WHEN** `wait_for` is invoked with `type: port` and no `port` parameter
- **THEN** the module SHALL return an error with message "required parameter 'port' is missing"

#### Scenario: Default timeout and interval
- **WHEN** `wait_for` is invoked with `type: port` and `port: 8080` with no `timeout` or `interval`
- **THEN** the module SHALL use a default `timeout` of 300 seconds and default `interval` of 5 seconds

### Requirement: Port check executes from controller
TCP port checks SHALL execute from the machine running Tack (the controller), not on the remote target via the connector.

#### Scenario: Port check against remote host
- **WHEN** `wait_for` is invoked with `type: port`, `host: "10.0.1.5"`, `port: 443`
- **THEN** the TCP connection attempt SHALL originate from the controller, not from the target host defined in the play
