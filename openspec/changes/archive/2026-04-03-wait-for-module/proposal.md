## Why

Bolt currently has no way to pause playbook execution until an external condition is met. After starting a service, deploying an application, or provisioning infrastructure, subsequent tasks often depend on a port being open, a file appearing, a URL responding, or a command succeeding. Without a `wait_for` module, users resort to fragile `command` + `sleep` loops that are hard to read, non-idempotent, and inconsistent across platforms.

## What Changes

- Add a new `wait_for` module under `internal/module/waitfor/` that polls a condition with configurable timeout and retry interval
- Support four condition types:
  - **port** - Wait for a TCP port to be open (or closed) on a given host
  - **path** - Wait for a file or directory to exist (or be absent)
  - **command** - Wait for a shell command to return exit code 0
  - **url** - Wait for an HTTP(S) URL to return a successful status code
- The module integrates with the existing `Module` interface and registry
- Supports `state: started` (condition met) and `state: stopped` (condition no longer met) for port and path types
- Returns elapsed wait time and attempt count in result data for debugging
- Implements `Checker` interface for check/dry-run mode (reports uncertain since it can't predict future state)

## Capabilities

### New Capabilities
- `wait-for-port`: Wait for a TCP port to become reachable or closed on a target host
- `wait-for-path`: Wait for a filesystem path to exist or be absent
- `wait-for-command`: Wait for a shell command to succeed (exit 0)
- `wait-for-url`: Wait for an HTTP(S) endpoint to return a successful response

### Modified Capabilities

_None — this is a new module with no changes to existing specs._

## Impact

- **New code**: `internal/module/waitfor/` package (~300-400 lines)
- **Module registry**: Auto-registers via `init()` following existing pattern
- **Dependencies**: No new external dependencies — uses Go stdlib `net`, `net/http`, and existing `connector.Execute()` for command checks
- **Connector interface**: No changes — port/URL checks run from the controller; path/command checks execute on the target via the connector
- **Playbook parsing**: No changes — the module uses the standard `module` + `params` task syntax
- **Documentation**: New module needs to be added to module listing and examples
