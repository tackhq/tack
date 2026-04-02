## ADDED Requirements

### Requirement: Fetch inventory from HTTP endpoint
The system SHALL fetch inventory data from an HTTP or HTTPS URL when the `-i` flag value starts with `http://` or `https://`. The response body MUST be valid JSON conforming to the standard inventory schema. The system SHALL use a GET request.

#### Scenario: Fetch from HTTPS endpoint
- **WHEN** the `-i` flag is `https://inventory.example.com/api/hosts`
- **THEN** the system SHALL send a GET request to the URL and parse the JSON response as inventory data

#### Scenario: HTTP endpoint returns non-200 status
- **WHEN** the HTTP endpoint returns a non-2xx status code
- **THEN** the system SHALL return an error indicating the HTTP status code and the URL that was requested

#### Scenario: HTTP endpoint returns invalid JSON
- **WHEN** the HTTP endpoint returns a 200 response with a body that is not valid inventory JSON
- **THEN** the system SHALL return a parse error indicating the response could not be decoded

### Requirement: HTTP request respects context cancellation
The system SHALL create the HTTP request with the provided context, so that timeouts and cancellation signals abort the request.

#### Scenario: HTTP request cancelled by timeout
- **WHEN** an HTTP inventory request is in progress and the context is cancelled
- **THEN** the system SHALL cancel the HTTP request and return a context cancellation error

### Requirement: HTTP request uses reasonable defaults
The system SHALL use a default HTTP client timeout of 30 seconds. The system SHALL set the `Accept: application/json` header on the request. The system SHALL not follow more than 10 redirects.

#### Scenario: Slow HTTP endpoint
- **WHEN** the HTTP endpoint takes longer than 30 seconds to respond and no other timeout is set
- **THEN** the system SHALL timeout and return an error indicating the request timed out

### Requirement: Support bearer token authentication
The system SHALL support bearer token authentication via the `BOLT_INVENTORY_TOKEN` environment variable. When set, the system SHALL include an `Authorization: Bearer <token>` header in the HTTP request.

#### Scenario: Token authentication
- **WHEN** `BOLT_INVENTORY_TOKEN` is set to `secret123` and the `-i` flag is an HTTP URL
- **THEN** the HTTP request SHALL include the header `Authorization: Bearer secret123`

#### Scenario: No token set
- **WHEN** `BOLT_INVENTORY_TOKEN` is not set and the `-i` flag is an HTTP URL
- **THEN** the HTTP request SHALL not include an Authorization header
