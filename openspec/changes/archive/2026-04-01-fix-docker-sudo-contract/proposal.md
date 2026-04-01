## Why

The Docker connector's `SetSudo()` method is a silent no-op, violating the Connector interface contract. When playbooks specify `sudo: true` and target Docker containers, tasks execute without privilege escalation. This causes silent failures — tasks that need root permissions fail with confusing permission errors rather than a clear message about sudo not being supported.

## What Changes

- Implement `SetSudo()` in the Docker connector to switch execution user to `root` via `docker exec -u root`
- When sudo is disabled, revert to the container's default user or the configured user
- Password parameter is accepted but ignored (Docker doesn't use sudo passwords)

## Capabilities

### New Capabilities
- `docker-sudo`: Privilege escalation support for the Docker connector via user switching

### Modified Capabilities

_None._

## Impact

- **Modified code**: `internal/connector/docker/docker.go` — implement `SetSudo()` and update `Execute()` to use root user
- **No dependency changes**
- **No breaking changes** — currently sudo is silently ignored, now it works
