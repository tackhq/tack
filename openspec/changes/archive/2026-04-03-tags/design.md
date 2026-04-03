## Context

Bolt currently defines `--tags` and `--skip-tags` CLI flags (in `cmd/bolt/main.go` lines 178-179) but they are not wired to the executor or the playbook structs. There is no `Tags` field on `Task`, `Play`, or role references. The feature needs to be built end-to-end: YAML parsing, struct fields, CLI flag plumbing, and executor filtering logic.

The block/rescue/always feature (recently shipped) establishes the pattern for task grouping with inherited properties, which tags will follow for inheritance through blocks.

## Goals / Non-Goals

**Goals:**
- Enable selective task execution via `--tags` and `--skip-tags` CLI flags
- Support `tags:` field on tasks, blocks, plays, and role references in YAML
- Implement tag inheritance: block-level tags apply to child tasks, role-level tags apply to all role tasks, play-level tags apply to all play tasks
- Support special tags `always` (runs unless explicitly skip-tagged) and `never` (skipped unless explicitly tagged)
- Tags work correctly with plan/check mode (filtered tasks shown as skipped or omitted)
- Tags work correctly with handlers (handlers run if notified, regardless of tags — matching Ansible behavior)

**Non-Goals:**
- Tag expressions or boolean logic (e.g., `--tags "deploy AND staging"`) — simple list matching only
- Tag-based variable scoping — tags are purely a filtering mechanism
- Persistent tag profiles or saved tag sets
- Regex or wildcard tag matching

## Decisions

### 1. Tags field type: `[]string` on Task, Play, and role references

Tags are stored as `[]string` on the `Task` struct and `Play` struct. In YAML, tags can be specified as a single string (converted to a one-element slice) or a list. Role references already use string names; tags for roles will be specified via an expanded role syntax (`role: name, tags: [...]`).

**Alternative considered**: Using a `map[string]bool` or set type. Rejected because tags are simple labels with no values, and slices are simpler to serialize/deserialize in YAML. A set can be built at runtime for O(1) lookups during filtering.

### 2. Filtering happens at task execution time, not parse time

Tag filtering is applied in the executor's task loop, not during YAML parsing. Tasks are still parsed and present in the playbook struct; they are simply skipped at runtime based on the active tag/skip-tag sets.

**Rationale**: This preserves the full playbook structure for plan mode display, validation, and future introspection. It also avoids complicating the parser with runtime concerns.

**Alternative considered**: Pruning tasks during parsing. Rejected because it makes plan mode display harder and couples parsing with execution configuration.

### 3. Tag inheritance model

- Play-level tags: apply to all tasks in the play (including role tasks and handlers)
- Block-level tags: apply to all tasks within the block (including rescue and always sections)
- Role-level tags: apply to all tasks loaded from the role
- Task-level tags: additive — a task's effective tags are its own tags plus all inherited tags
- Tags are accumulated, never subtracted by inheritance

**Implementation**: When executing a task, compute effective tags by merging: play tags + role tags (if in a role) + block tags (if in a block) + task's own tags. Store inherited tags in a context/accumulator passed through the execution chain.

### 4. Special tag semantics

- `always`: Task runs even when `--tags` is specified and the task's other tags don't match. Exception: if `always` is in `--skip-tags`, the task is skipped.
- `never`: Task is skipped even when no `--tags` filter is active. Exception: if one of the task's tags is in `--tags`, the task runs.

These match Ansible's semantics exactly, which is what users expect.

### 5. Handler behavior with tags

Handlers execute when notified, regardless of tag filtering. If a tagged task runs and notifies a handler, the handler runs — even if the handler's own tags don't match `--tags`. This matches Ansible behavior and prevents broken state from partial runs.

### 6. Tag data flows through executor, not a new package

No new package is needed. Tag filtering logic lives in `internal/executor/` as a helper function. The executor receives tag/skip-tag slices from CLI flags (via `ConnOverrides` or a new `RunOptions` struct).

**Decision**: Add `Tags` and `SkipTags` fields to the `Executor` struct directly, since they are run-level configuration like `DryRun` and `Debug`.

## Risks / Trade-offs

- **[Risk] Tag typos silently skip tasks** → Mitigation: In plan mode, display which tasks are skipped due to tag filtering so users can verify. Consider a `--list-tags` flag in the future.
- **[Risk] Inherited tags may surprise users** → Mitigation: Document inheritance clearly. Plan mode output shows effective tags per task.
- **[Risk] `always`/`never` semantics are non-obvious** → Mitigation: Follow Ansible's documented behavior exactly; users already familiar with Ansible expect this.
- **[Trade-off] Handlers ignore tag filtering** → This matches Ansible but could surprise users who expect handlers to also be filtered. Document this behavior explicitly.
