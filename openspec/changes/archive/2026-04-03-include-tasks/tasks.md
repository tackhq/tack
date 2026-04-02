## 1. Parser — Add `include_tasks` keyword and `vars:` parsing

- [x] 1.1 Add `include_tasks` to `knownTaskFields` map in `internal/playbook/parser.go`
- [x] 1.2 Parse `include_tasks:` in `parseRawTask()` — map it to `Task.Include` (same field as `include:`)
- [x] 1.3 Add `IncludeVars map[string]any` field to `Task` struct in `internal/playbook/playbook.go`
- [x] 1.4 Parse `vars:` on include/include_tasks directives in `parseRawTask()` into `Task.IncludeVars`
- [x] 1.5 Write unit tests for parser: `include_tasks:` with and without `vars:`, verify both `include:` and `include_tasks:` produce identical `Task` structs

## 2. Executor — Variable scoping for `vars:`

- [x] 2.1 In `runInclude()`, before executing included tasks, snapshot current `pctx.Vars` keys that will be overridden by `Task.IncludeVars`
- [x] 2.2 Merge `Task.IncludeVars` into `pctx.Vars` before executing included tasks
- [x] 2.3 After included tasks complete, restore overridden vars and remove injected keys from `pctx.Vars`
- [x] 2.4 Ensure registered variables from included tasks persist (written to `pctx.Registered`, not affected by var restore)
- [x] 2.5 Write unit tests: vars scoping (override + restore), vars don't leak, registered vars persist

## 3. Executor — Loop support on includes

- [x] 3.1 In the main task loop or `runInclude()`, detect when `include_tasks`/`include` has `loop:` or `with_items:`
- [x] 3.2 Load and parse the task file once, then iterate: set loop variable (`loop_var` / default `item`) and `loop_index`, execute included tasks per iteration
- [x] 3.3 Combine loop variables with `IncludeVars` — both should be available in each iteration
- [x] 3.4 Write unit tests: loop with include_tasks, loop_var override, loop + vars combined

## 4. Executor — Circular include detection

- [x] 4.1 Add a `visitedPaths []string` parameter (or field on executor/context) threaded through `runInclude()` calls
- [x] 4.2 Resolve include paths to absolute (using `filepath.Abs` + `filepath.EvalSymlinks`) before checking visited set
- [x] 4.3 Check for path in visited set before loading — return error with full chain if circular (e.g., "circular include: a.yml → b.yml → a.yml")
- [x] 4.4 Enforce max depth of 64 — return clear error if exceeded
- [x] 4.5 Write unit tests: direct cycle detection, indirect cycle detection, max depth error, non-cycle reuse of same file from different branches is allowed

## 5. Path resolution improvements

- [x] 5.1 Ensure relative paths resolve relative to the file containing the `include_tasks:` directive (use playbook `Path` field)
- [x] 5.2 When task has `RolePath` set, resolve relative paths against `<RolePath>/tasks/` directory
- [x] 5.3 Absolute paths used as-is (no resolution)
- [x] 5.4 Write unit tests: playbook-relative, role-relative, and absolute path resolution

## 6. Plan mode integration

- [x] 6.1 Update `planTasks()` to display `include_tasks:` entries with module shown as "include_tasks" and path in params
- [x] 6.2 Handle `when:` on include_tasks in plan: show "conditional" if referencing registered vars, evaluate otherwise
- [x] 6.3 Write unit test: plan output for include_tasks with and without conditions

## 7. Documentation

- [x] 7.1 Add `include_tasks` section to README.md with usage examples: basic, vars, when, loop
- [x] 7.2 Add note that `include:` and `include_tasks:` are equivalent, with `include_tasks:` preferred
- [x] 7.3 Create example playbook: `examples/include-tasks/playbook.yml` with a main playbook and shared task files demonstrating vars, when, and loop patterns
- [x] 7.4 Create the shared task files referenced by the example playbook (e.g., `examples/include-tasks/tasks/install.yml`, `examples/include-tasks/tasks/configure.yml`)

## 8. Integration testing

- [x] 8.1 Add integration test: end-to-end playbook with `include_tasks:`, `vars:`, and `when:` — verify correct execution and variable scoping
- [x] 8.2 Add integration test: loop-driven `include_tasks:` — verify each iteration executes with correct variables
- [x] 8.3 Add integration test: nested includes (include within include) — verify correct execution and path resolution
