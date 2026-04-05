## ADDED Requirements

### Requirement: HTTP GET inventory fetch
The HTTP plugin SHALL perform a GET request to the configured URL and parse the response body as Tack-native inventory (JSON or YAML).

#### Scenario: Successful fetch
- **WHEN** the HTTP endpoint returns 200 with a valid JSON inventory body
- **THEN** the plugin SHALL parse it and return a populated `*Inventory`

#### Scenario: Non-2xx response
- **WHEN** the HTTP endpoint returns a non-2xx status code
- **THEN** the plugin SHALL return an error that includes the status code and response body (truncated to 1KB)

#### Scenario: Network error
- **WHEN** the HTTP request fails due to DNS resolution, connection refused, or similar
- **THEN** the plugin SHALL return an error wrapping the underlying network error

### Requirement: HTTP plugin configuration
The plugin SHALL be configured via a YAML file with `plugin: http` and the following fields:
- `url` (required): The endpoint URL
- `headers` (optional): Map of header name to value
- `params` (optional): Map of query parameter name to value
- `timeout` (optional): Request timeout in seconds (overrides global default)

#### Scenario: URL with query params
- **WHEN** config specifies `url: https://cmdb/hosts` and `params: {env: prod}`
- **THEN** the request SHALL be sent to `https://cmdb/hosts?env=prod`

#### Scenario: Custom headers
- **WHEN** config specifies `headers: {X-API-Key: secret123}`
- **THEN** the request SHALL include the `X-API-Key: secret123` header

#### Scenario: Missing URL
- **WHEN** the config does not include a `url` field
- **THEN** the plugin SHALL return a validation error before making any request

### Requirement: HTTP authentication
The plugin SHALL support bearer token and basic auth via the `auth` config section. Only one auth method SHALL be active per config.

#### Scenario: Bearer token auth
- **WHEN** config specifies `headers: {Authorization: "Bearer mytoken"}`
- **THEN** the request SHALL include the `Authorization: Bearer mytoken` header

#### Scenario: Basic auth
- **WHEN** config specifies `auth: {basic: {username: user, password: pass}}`
- **THEN** the request SHALL include a Basic auth header with the encoded credentials

### Requirement: TLS configuration
The plugin SHALL support custom TLS settings via a `tls` config section:
- `ca_cert`: Path to CA certificate file
- `client_cert`: Path to client certificate file
- `client_key`: Path to client key file
- `insecure_skip_verify`: Boolean to skip server certificate verification

#### Scenario: Custom CA certificate
- **WHEN** config specifies `tls: {ca_cert: /etc/ssl/custom-ca.pem}`
- **THEN** the HTTP client SHALL use that CA certificate for server verification

#### Scenario: Mutual TLS
- **WHEN** config specifies `tls: {client_cert: cert.pem, client_key: key.pem}`
- **THEN** the HTTP client SHALL present the client certificate during TLS handshake

#### Scenario: Insecure skip verify
- **WHEN** config specifies `tls: {insecure_skip_verify: true}`
- **THEN** the HTTP client SHALL skip server certificate verification

### Requirement: Variable interpolation in config
The plugin SHALL support `{{ env.VAR_NAME }}` interpolation in string config values. This allows credentials and dynamic values to come from environment variables.

#### Scenario: Environment variable in header
- **WHEN** config specifies `headers: {Authorization: "Bearer {{ env.API_TOKEN }}"}` and `API_TOKEN=secret` is set
- **THEN** the request SHALL include `Authorization: Bearer secret`

#### Scenario: Undefined environment variable
- **WHEN** config references `{{ env.MISSING_VAR }}` and the variable is not set
- **THEN** the plugin SHALL return an error indicating the undefined variable
