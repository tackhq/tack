## Context

Bolt currently has a single `include:` directive on tasks that loads external task files at runtime. The infrastructure already exists:

- `Task.Include` field in `internal/playbook/playbook.go`
- `LoadTasksFile()` in `internal/playbook/roles.go` parses external YAML task files
- `runInclude()` in `internal/executor/executor.go` handles runtime inclusion with variable interpolation
- The parser recognizes `include` as a known task field
- `when:` conditions already gate include execution

What's missing: `vars:` support, `loop:` on includes, circular detection, and the `include_tasks:` keyword alias.

**Stakeholders:** DevOps engineers authoring playbooks (primary users), Python developers familiar with Ansible patterns (expect `include_tasks:` keyword), IT support maintaining documentation.

## Goals / Non-Goals

**Goals:**
- Add `include_tasks:` as the standard keyword (alias for `include:`)
- Add `vars:` block support for scoped variable passing
- Add `loop:` support for iterating includes
- Support `{{ variable }}` interpolation in include paths (already partially works — formalize)
- Circular include detection with clear errors
- Consistent path resolution (playbook-relative and role-relative)
- Documentation and examples

**Non-Goals:**
- Static/parse-time inclusion (`import_tasks`) — unnecessary complexity for this tool
- Play-level inclusion (`include_playbook`) — different scope, future work
- `include_role` / `import_role` — separate feature
- Task file search paths or plugin-style include resolvers
- Jinja2-style templating in included files

## Decisions

### 1. Single directive, not two

**Decision:** Provide only `include_tasks:` (dynamic, runtime). No `import_tasks`.

**Rationale:** The practical benefit of parse-time expansion (better plan output) doesn't justify the complexity of maintaining two code paths with different semantics. One directive, consistently runtime, is easier to understand and maintain. `include:` stays as an alias.

### 2. Enhance existing `runInclude()` rather than rewrite

**Decision:** Extend the existing `runInclude()` in the executor with `vars:` merging, loop support, and circular detection. Don't create a new code path.

**Rationale:** The current implementation already handles path interpolation, task loading, and inline execution. Adding features incrementally avoids regressions and keeps the diff small.

### 3. Variable scoping for `vars:`

**Decision:** Variables passed via `vars:` are merged into a copy of the play context vars for the duration of the included tasks, then discarded. They do NOT persist after the include completes.

**Rationale:** Scoped variables prevent accidental pollution of the outer play context. This matches Ansible behavior. Users who want persistent variables should use `register:` or play-level `vars:`.

**Implementation:** Before executing included tasks, snapshot `pctx.Vars`, merge include vars on top, execute, then restore the snapshot.

### 4. Loop support

**Decision:** When `loop:` is present on an `include_tasks:` directive, the executor loads the task file once and executes its tasks once per loop iteration, with the loop variable set.

**Rationale:** This is a natural extension of Bolt's existing loop support on regular tasks. The include file is loaded once (cached for the loop), and each iteration gets its own variable scope.

### 5. Circular include detection

**Decision:** Track file paths in a `visitedPaths` set passed through the include chain. Error immediately if a path is re-encountered. Max depth of 64.

**Rationale:** Simple and effective. The set is stack-scoped (not global), so the same file can be included from two independent branches — only true cycles are detected.

### 6. Path resolution

**Decision:** Paths resolved relative to the file containing the directive. If the task is inside a role, resolve relative to the role's `tasks/` directory. Absolute paths used as-is.

**Rationale:** Consistent with existing `LoadTasksFile()` behavior and Ansible conventions.

### 7. `include:` as alias

**Decision:** Keep `include:` as a backward-compatible alias for `include_tasks:`. No deprecation warning.

**Rationale:** Both keywords map to the same `Task.Include` field. No reason to break existing playbooks or add warning noise. If we want to deprecate `include:` later, that's a separate decision.

## Risks / Trade-offs

**[Risk] Variable snapshot/restore could miss edge cases with registered vars** → Mitigation: Only snapshot/restore the `vars:` overlay keys, not the entire vars map. Registered variables from included tasks persist (they're written to `pctx.Registered`, not `pctx.Vars`).

**[Risk] Loop + include could be slow for large task files** → Mitigation: Load and parse the task file once, execute the parsed tasks N times. Document that very large loops with complex includes may be slow.

**[Risk] Circular detection false positives with symlinks** → Mitigation: Resolve paths to absolute before checking the visited set. Use `filepath.EvalSymlinks()`.

**[Trade-off] Tasks in includes are invisible to the planner** → Inherent to runtime inclusion. Plan output shows `include_tasks` as a single entry. This is acceptable and documented.
