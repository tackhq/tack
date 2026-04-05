## ADDED Requirements

### Requirement: Wait for URL to respond successfully
The `wait_for` module with `type: url` SHALL make HTTP GET requests to the specified URL, polling until a successful response (status 200-399) is received or the timeout is exceeded.

#### Scenario: URL responds before timeout
- **WHEN** `wait_for` is invoked with `type: url`, `url: "http://localhost:8080/health"`, `timeout: 60`, `interval: 5`
- **THEN** the module SHALL make an HTTP GET request every 5 seconds
- **THEN** when a response with status code 200-399 is received, the module SHALL return `Changed: true` with `elapsed`, `attempts`, and `status_code` in `Result.Data`

#### Scenario: URL does not respond before timeout
- **WHEN** `wait_for` is invoked with `type: url`, `url: "https://api.example.com/ready"`, `timeout: 10`
- **THEN** if no successful response is received within 10 seconds, the module SHALL return an error containing "timeout waiting for url https://api.example.com/ready"

#### Scenario: URL returns error status codes
- **WHEN** `wait_for` is invoked with `type: url` and the URL returns status 500
- **THEN** the module SHALL treat it as a failed attempt and continue polling

#### Scenario: Connection refused or DNS failure
- **WHEN** `wait_for` is invoked with `type: url` and the HTTP request fails with a connection error
- **THEN** the module SHALL treat it as a failed attempt and continue polling (not an immediate error)

### Requirement: URL parameter validation
The module SHALL validate that the `url` parameter is present and well-formed.

#### Scenario: Missing url parameter
- **WHEN** `wait_for` is invoked with `type: url` and no `url` parameter
- **THEN** the module SHALL return an error with message "required parameter 'url' is missing"

#### Scenario: Invalid URL scheme
- **WHEN** `wait_for` is invoked with `type: url`, `url: "ftp://example.com"`
- **THEN** the module SHALL return an error with message "url must use http or https scheme"

### Requirement: URL check executes from controller
HTTP requests for URL checks SHALL originate from the machine running Tack (the controller), not from the target.

#### Scenario: URL check from controller
- **WHEN** `wait_for` is invoked with `type: url`, `url: "http://10.0.1.5:8080/health"`
- **THEN** the HTTP request SHALL originate from the controller

### Requirement: URL check follows redirects
The module SHALL follow HTTP redirects (up to Go's default limit) and evaluate the final response status.

#### Scenario: URL redirects to success
- **WHEN** `wait_for` is invoked with `type: url` and the URL returns a 301 redirect to a page that returns 200
- **THEN** the module SHALL treat it as a successful response
