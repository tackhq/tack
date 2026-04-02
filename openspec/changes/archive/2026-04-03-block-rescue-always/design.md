## Context

Bolt executes tasks sequentially. On failure, execution stops (unless `ignore_errors: true`). There is no way to group tasks and handle failures structurally. The executor's main loop in `runPlayOnHost()` iterates `allTasks` and calls `runTask()` for each. Includes are handled as a special case before the regular task path.

Key existing infrastructure:
- `Task` struct has `When`, `IgnoreErrors`, `Sudo` fields
- `runTask()` returns `(TaskResult, error)` — error means task failed
- `Stats` tracks ok/changed/failed/skipped counts
- Plan mode via `planTasks()` shows each task with predicted status

## Goals / Non-Goals

**Goals:**
- `block:` groups tasks; if any fails, `rescue:` runs; `always:` runs regardless
- Block-level `when:` applies to the entire block (skip all if false)
- Block-level `name:` for descriptive output
- Nested blocks supported (block within block/rescue/always)
- Plan mode shows block structure
- Documentation and examples

**Non-Goals:**
- Block-level `loop:` (iterate the entire block) — complex, future work
- Block-level `register:` (register the block outcome) — can be added later
- `rescue` variable injection (Ansible's `ansible_failed_task` etc.) — unnecessary complexity for now

## Decisions

### 1. Block as a special Task type

**Decision:** A block is a `Task` with `Block []*Task` populated and `Module` empty. Similar pattern to how includes work (`Task.Include` populated, `Module` empty).

**Rationale:** Keeps the data model flat — blocks are tasks in the task list, parsed and validated like other tasks. The executor detects them via `task.IsBlock()`.

### 2. Execution flow

**Decision:**
1. Execute `block:` tasks sequentially. If all succeed → skip `rescue:`, run `always:`.
2. If any `block:` task fails → stop remaining block tasks, run `rescue:` tasks sequentially.
3. Run `always:` tasks regardless of block/rescue outcome.
4. Final error: if `rescue` succeeded, block is considered recovered (no error). If `rescue` failed or was absent and `block` failed, propagate the error — unless `always` ran cleanly and the block has `ignore_errors`.

**Rationale:** Matches Ansible semantics exactly. DevOps engineers expect this behavior.

### 3. Block-level directive inheritance

**Decision:** `when:` on a block applies to the whole block (skip entirely if false). `sudo:` on a block is inherited by all tasks within block/rescue/always unless overridden at task level.

**Rationale:** Consistent with Ansible. Reduces YAML repetition.

### 4. Parser changes

**Decision:** Add `block`, `rescue`, `always` to `knownTaskFields`. In `parseRawTask()`, if `block:` key is present, parse it as a nested task list. Same for `rescue:` and `always:`. A task with `block:` must not have a module.

### 5. Plan mode

**Decision:** Show block tasks indented/grouped under the block name. Rescue and always shown as sub-sections. Status of individual tasks within the block shown normally.

## Risks / Trade-offs

**[Risk] Deeply nested blocks** → Mitigation: No explicit depth limit needed (unlike includes, blocks don't load external files). Stack depth is naturally limited.

**[Risk] Error state complexity** → Mitigation: Clear state machine: block_failed → rescue runs → always runs → determine final outcome. Each phase is independent.

**[Trade-off] No block-level loop** → Keeps implementation simple. Users can loop individual tasks within a block.
