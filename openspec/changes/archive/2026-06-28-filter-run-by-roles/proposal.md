## Why

When developing or debugging a playbook that pulls in many roles, the user often only cares about one or two of them. Today the only way to narrow execution is by tags, which requires every role's tasks to be tagged and forces the user to know those tag names. A direct "only run these roles" filter makes iterating on a single role fast without touching the playbook.

## What Changes

- Add a `-r`/`--roles` CLI flag to the `run` command that accepts a comma-separated list of role names.
- When `--roles` is set, only tasks loaded from the named roles are executed; all other role tasks and all play-level (non-role) tasks are skipped.
- Tasks skipped by the roles filter are reported as skipped (consistent with existing tag-filter skip reporting).
- The roles filter composes with the existing `--tags`/`--skip-tags` filters (a task must pass both to run).
- Document the new flag in CLI help and usage docs.

## Capabilities

### New Capabilities
- `role-filtering`: CLI-driven filtering that restricts a run to a chosen subset of roles by name, for faster iterative development.

### Modified Capabilities
<!-- No existing capability's requirements change; tag filtering behavior is unchanged. -->

## Impact

- `cmd/tack/main.go`: register the `-r`/`--roles` flag on `runCmd` and plumb it into the executor (alongside the existing `--tags`/`--skip-tags` wiring).
- `internal/executor/executor.go`: new `Roles []string` field on `Executor`; apply the role filter during role-task expansion/execution.
- `internal/executor/tags.go` (or a sibling file): helper to decide whether a task belongs to a selected role.
- `internal/playbook`: tasks need their originating role name available for filtering (the `Task` struct already tracks `RolePath`; expose/compare role name).
- Tests: executor filtering tests mirroring the existing tag-filter tests; CLI flag wiring.
- Docs: CLI reference / usage examples for `--roles`.
