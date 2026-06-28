## Context

The executor already supports tag-based filtering via `Executor.Tags` / `Executor.SkipTags`, wired from the `--tags` / `--skip-tags` CLI flags in `cmd/tack/main.go` and gated by `shouldRunTask()` in `internal/executor/tags.go`. Roles are loaded from `play.Roles` (`[]RoleRef`) and expanded into a flat task list via `playbook.ExpandRoleTasks`. Each role-derived `Task` tracks its origin through `Task.RolePath` (absolute role directory), but not its plain role *name*. We want a parallel "only these roles" filter so a developer can iterate on one role without editing the playbook or relying on tags.

## Goals / Non-Goals

**Goals:**
- A `-r`/`--roles` flag on `run` that restricts execution to the named role(s).
- Skipped tasks are reported, not silently dropped — same UX as tag filtering.
- Composes cleanly (AND) with `--tags`/`--skip-tags`.
- Minimal, localized change that follows the established tag-filter pattern.

**Non-Goals:**
- No `--skip-roles` inverse flag in this change (can be added later if needed).
- No new playbook YAML syntax — `--roles` is purely a CLI runtime filter.
- No change to how roles are loaded, ordered, or how their tags are inherited.

## Decisions

### Decision: Filter by role name carried on the task
Add a `RoleName string` field to `Task`, populated during role expansion (where `RolePath` is already set in `internal/playbook/roles.go`). Filtering on a stable name is clearer than comparing absolute `RolePath` values against resolved role directories, and it matches what the user types in `--roles`.

*Alternative considered:* reuse `RolePath` and resolve each `--roles` name to a directory for comparison. Rejected — couples the filter to path resolution and role-search-dir logic, and is harder to test.

### Decision: Gate at the same point as tag filtering
Add `Executor.Roles []string`. Introduce `shouldRunRole(taskRoleName string, roles []string) bool` in `internal/executor/tags.go` (or a sibling `roles.go`). At each existing `shouldRunTask(...)` call site in `executor.go`, also require `shouldRunRole(task.RoleName, e.Roles)`. When `e.Roles` is empty the helper returns `true` (no filtering). This keeps the AND-composition with tags automatic and reuses the existing "skipped" emit path.

*Alternative considered:* drop non-matching roles before `ExpandRoleTasks` / filter the `play.Roles` list. Rejected — it would hide skipped tasks from the output (the spec requires reporting them) and wouldn't cover play-level tasks.

### Decision: Play-level (non-role) tasks have empty `RoleName` and are skipped when filtering is active
`shouldRunRole("", ["web"])` returns `false`. This satisfies the requirement that `--roles` restricts the run to the selected roles only. When `--roles` is unset, empty-name tasks run as today.

### Decision: Unknown role names match nothing, no error
The filter is a pure membership test; a name with no corresponding role simply never matches. This avoids brittle validation against dynamically loaded roles and keeps the flag forgiving during development.

## Risks / Trade-offs

- **Silently running zero tasks when a role name is mistyped** → Mitigate with a clear skipped-task report and, optionally, a one-line notice when `--roles` matched no tasks. Acceptable for a developer-convenience flag.
- **`-r` short flag collisions** → Confirm `-r` is not already bound on `runCmd`; the executor's flag set currently uses long forms for tag filters, so `-r` should be free.
- **Handlers triggered by role tasks** → Handlers run only when notified by a task that executed; since filtered-out tasks don't run, they won't notify. No special handling needed, but worth a test.

## Migration Plan

Purely additive CLI flag with a new optional struct field; no migration. Default behavior (no `--roles`) is unchanged. Rollback is removal of the flag and field.

## Open Questions

- Should a future `--skip-roles` be added for symmetry with `--skip-tags`? Deferred.
- Should the run print an explicit warning when `--roles` matches zero tasks? Recommended but optional; left to implementation discretion.
