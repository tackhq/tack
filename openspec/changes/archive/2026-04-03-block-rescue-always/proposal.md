## Why

Bolt has no structured error handling for task groups. When a task fails, execution stops — there's no way to run cleanup/rollback tasks or ensure finalization logic always runs. This is a P0 structural gap that blocks real-world workflows like database migrations (rollback on failure), blue-green deploys (switch back on failure), and service restarts (always restart regardless of outcome).

Ansible's `block`/`rescue`/`always` pattern is the established solution and what DevOps engineers expect.

## What Changes

- Add `block:` directive — groups a list of tasks to attempt as a unit
- Add `rescue:` directive — tasks to run if any task in `block:` fails
- Add `always:` directive — tasks that run regardless of block/rescue outcome
- `block:` supports `when:` conditions (applied to the entire block)
- `block:` supports `name:` for descriptive plan output
- Tasks inside block/rescue/always inherit block-level directives (`sudo:`, `when:`)
- Proper error propagation: if rescue also fails, the error is reported; always still runs
- Integration with plan/check mode: block tasks shown grouped

## Capabilities

### New Capabilities
- `block-rescue-always`: Structured error handling with block/rescue/always task grouping — attempt tasks, handle failures, and ensure cleanup runs

### Modified Capabilities
<!-- None -->

## Impact

- **Playbook structs** (`internal/playbook/playbook.go`): Add `Block`, `Rescue`, `Always` fields to `Task`; add `IsBlock()` helper
- **Parser** (`internal/playbook/parser.go`): Add `block`, `rescue`, `always` to `knownTaskFields`; parse nested task lists
- **Executor** (`internal/executor/executor.go`): Add `runBlock()` method; integrate into main task loop alongside include handling
- **Plan mode**: Display block tasks grouped with rescue/always sections
- **Docs**: README section, example playbooks
