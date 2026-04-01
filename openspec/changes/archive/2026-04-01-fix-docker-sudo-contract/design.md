## Context

The Connector interface requires all implementations to support `SetSudo(enabled bool, password string)`. Local, SSH, and SSM connectors implement this by wrapping commands with `sudo -S`. Docker containers don't use the host sudo mechanism — instead, `docker exec -u <user>` controls which user runs the command.

Currently the Docker connector stores a configurable `user` field (set via `WithUser()` option) and passes it to `docker exec -u`. When `SetSudo()` is called, nothing happens.

## Goals / Non-Goals

**Goals:**
- Make `SetSudo(true, _)` switch Docker exec to run as `root`
- Make `SetSudo(false, _)` revert to the originally configured user (or container default)
- Maintain backward compatibility for playbooks not using sudo

**Non-Goals:**
- Support sudo passwords in Docker (not applicable)
- Support non-root privilege escalation (e.g., `sudo -u postgres`) — future work
- Change the Upload/Download methods (they already use docker cp which runs as host docker user)

## Decisions

### 1. Override user to "root" when sudo enabled

When `SetSudo(true, _)` is called, store a flag and override the `-u` argument in `Execute()` to `root`. When `SetSudo(false, _)` is called, revert to the original user. This is simple and matches the semantics of sudo on other connectors.

**Alternative considered:** Actually running `sudo` inside the container. Rejected because most containers don't have sudo installed, and this would require password handling that doesn't apply to containers.

### 2. Preserve original user for toggle support

The executor toggles sudo per-task. Store the original `user` value at construction time so `SetSudo(false, _)` can revert cleanly.

## Risks / Trade-offs

- **[Risk] Container doesn't allow user switching** — Some container runtimes restrict `-u`. → Mitigation: Docker's default behavior allows `-u root`; restricted runtimes would error at the docker exec level with a clear message.
- **[Trade-off] Password parameter ignored** — Consistent with interface but callers can't detect this. → Acceptable since Docker auth is fundamentally different from host sudo.
