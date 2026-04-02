## 1. Module Scaffold

- [x] 1.1 Create `internal/module/waitfor/` package with `waitfor.go` containing the `Module` struct, `Name()` returning `"wait_for"`, `init()` registration, and `Run()` skeleton
- [x] 1.2 Implement parameter extraction in `Run()`: `type` (required), `timeout` (default 300), `interval` (default 5), `state` (default "started"), plus type-specific params (`host`, `port`, `path`, `cmd`, `url`)
- [x] 1.3 Implement the core polling loop using `time.NewTicker` + `context.WithTimeout` that calls a condition-checker function and returns `Changed: true` on success or error on timeout

## 2. Port Check Implementation

- [x] 2.1 Implement `checkPort()` function using `net.DialTimeout` with per-attempt timeout of `min(interval, 5s)`, checking from the controller (not via connector)
- [x] 2.2 Handle `state: stopped` — invert the success condition so that connection refused = success
- [x] 2.3 Validate `port` is present and numeric; default `host` to `"localhost"`

## 3. Path Check Implementation

- [x] 3.1 Implement `checkPath()` function that executes `test -e <path>` on the target via `connector.Execute()`
- [x] 3.2 Handle `state: stopped` — invert so `test -e` failure (path absent) = success
- [x] 3.3 Validate `path` parameter is present

## 4. Command Check Implementation

- [x] 4.1 Implement `checkCommand()` function that executes the `cmd` on the target via `connector.Execute()` and treats exit code 0 as success
- [x] 4.2 On transport-level connector errors (not non-zero exit), return error immediately without retrying
- [x] 4.3 Include `stdout` and `stderr` from the final successful command in `Result.Data`
- [x] 4.4 Validate `cmd` parameter is present

## 5. URL Check Implementation

- [x] 5.1 Implement `checkURL()` function using `http.Client` with per-request timeout, treating status 200-399 as success
- [x] 5.2 Handle connection errors and non-success status codes as retry-able failures (not immediate errors)
- [x] 5.3 Validate `url` parameter is present and uses `http` or `https` scheme
- [x] 5.4 Include `status_code` in `Result.Data` on success

## 6. Check Mode and Documentation

- [x] 6.1 Implement `Checker` interface returning `UncertainChange("wait_for cannot predict future state")` for all condition types
- [x] 6.2 Implement `Describer` interface with `Description()` and `Parameters()` for module documentation

## 7. Tests

- [x] 7.1 Unit tests for parameter validation: missing `type`, missing type-specific required params, invalid `url` scheme, default values
- [x] 7.2 Unit tests for port check: mock `net.Dial` success/failure, `state: started` vs `state: stopped`, timeout behavior
- [x] 7.3 Unit tests for path check: mock connector returning success/failure for `test -e`, both states, timeout
- [x] 7.4 Unit tests for command check: mock connector returning exit 0, non-zero, and transport errors
- [x] 7.5 Unit tests for URL check: use `httptest.NewServer` for success, error codes, and connection refusal
- [x] 7.6 Unit test for check mode returning `UncertainChange`
- [x] 7.7 Integration test: start an HTTP server, use `wait_for` with `type: url` to wait for it, verify result data contains elapsed time and attempt count
