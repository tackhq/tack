## 1. Carry role name on tasks

- [x] 1.1 Add a `RoleName string` field to the `Task` struct in `internal/playbook/playbook.go`
- [x] 1.2 Populate `RoleName` during role expansion in `internal/playbook/roles.go` wherever `RolePath` is set (so every role-derived task knows its role; play-level tasks keep an empty name)

## 2. Executor role filter

- [x] 2.1 Add `Roles []string` field to the `Executor` struct in `internal/executor/executor.go`
- [x] 2.2 Add `shouldRunRole(taskRoleName string, roles []string) bool` in `internal/executor/tags.go` (or a sibling `roles.go`): returns true when `roles` is empty, otherwise true only if `taskRoleName` is in `roles`
- [x] 2.3 At each `shouldRunTask(...)` gate in `executor.go`, also require `shouldRunRole(task.RoleName, e.Roles)`, emitting a skipped result (reason: role filter) when excluded — mirroring the existing tag-skip emit

## 3. CLI flag wiring

- [x] 3.1 Register `-r`/`--roles` (StringSlice) on `runCmd` in `cmd/tack/main.go`, with help text "Only run tasks from these roles"
- [x] 3.2 Plumb the flag into `exec.Roles` alongside the existing `--tags`/`--skip-tags` wiring
- [x] 3.3 Confirm `-r` short flag does not collide with another flag on `runCmd`

## 4. Tests

- [x] 4.1 Executor test: `--roles` runs only the named role's tasks and skips other roles
- [x] 4.2 Executor test: multiple roles use OR logic; play-level tasks are skipped when `--roles` is set
- [x] 4.3 Executor test: `--roles` composes with `--tags` and `--skip-tags` (AND logic)
- [x] 4.4 Executor test: unknown role name matches nothing and does not error
- [x] 4.5 Test that excluded tasks are reported as skipped in the output

## 5. Docs

- [x] 5.1 Document the `--roles` flag in CLI help/usage docs with an example
- [x] 5.2 Run `make build` and `make test`; verify the new flag appears in `tack run --help`
