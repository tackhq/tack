## Context

Tag filtering (`--tags`/`--skip-tags`) is implemented in `internal/executor/tags.go` via `effectiveTags(task, playTags, blockTags)` (union of own + inherited play/role/block tags, deduped) and `shouldRunTask(eTags, tags, skipTags)`. The output layer (`internal/output`) renders task lifecycle through the `Emitter` interface: `TaskStart`, `TaskResult`, `DisplayPlan`, `DisplayMultiHostPlan`. Today none of these surface tags. `PlannedTask` (the plan-row struct) has no `Tags` field, and `TaskResult(name, status, changed, message)` does not receive tags. The executor already computes `eTags` at the call sites that decide whether a task runs, so the data exists at the right moments — it just isn't passed to the emitter.

## Goals / Non-Goals

**Goals:**
- Show a task's effective tags inline in both the plan preview and the apply result line.
- Reuse the existing `effectiveTags()` so displayed tags exactly equal what `-t`/`--skip-tags` match.
- Dimmed, bracketed suffix (`[a,b]`), omitted when empty; plain under `--no-color`.
- Include tags in JSON task events.

**Non-Goals:**
- No change to tag definition, inheritance, or filtering behavior.
- No new CLI flags and no option to hide tags (always shown when present).
- No change to `TaskStart` (and therefore no change to spinner mechanics beyond the appended suffix).

## Decisions

### Extend `Emitter.TaskResult` with `tags []string`
New signature: `TaskResult(name, status string, changed bool, message string, tags []string)`. The emitter is the right place to format display; passing the resolved slice keeps formatting out of the executor. All implementers update: text `Output`, `JSONEmitter`, and the test emitters `nullEmitter`, `testOutputCapture`, `countingEmitter`.
- Alternative considered: append `[tags]` to the `name` string at the call site. Rejected — it pollutes the name used by the JSON emitter and couples formatting to the executor.
- Alternative considered: a separate `TaskTags(...)` emitter call. Rejected — extra ordering/coupling for no benefit.

### Add `Tags []string` to `output.PlannedTask`
Populate it wherever planned rows are built in `planTasks` (the `eTags` is already computed there for the skip decision). Plan renderers (`DisplayPlan`, `renderMultiHostPlanLine`) append the same bracketed suffix.

### Thread `eTags` through the executor, do not recompute
`runSingleTask` gains an `eTags []string` parameter forwarded to `TaskResult`; callers (`applyHostPlan`, block execution) already hold `eTags` from their `shouldRunTask` check and pass it in. Skip-path `TaskResult` calls pass their local `eTags` directly. This avoids duplicating the inheritance logic and guarantees display == filter set.

### Formatting lives in `internal/output`
A small `formatTags([]string) string` helper returns `""` for empty input and `[a,b]` otherwise. Rendering wraps it in `o.color(colorGray, ...)`, so `--no-color` yields plain text and buffered/non-TTY output is unaffected beyond the literal suffix.

## Risks / Trade-offs

- **Play-wide tags add noise on every line** → Acceptable and correct: those tags genuinely match `-t`, so showing them is accurate. Dim styling keeps them visually secondary.
- **Interface signature change ripples to all `Emitter` implementers (incl. tests)** → Mechanical; compiler enforces completeness. Covered by updating the three test emitters in the same change.
- **Existing plan/result golden tests will see the new suffix** → Update those assertions as part of the change; untagged tasks are unaffected (no bracket), limiting churn.
- **Long tag lists could widen lines / interact with the spinner redraw** → The suffix is appended after the name in the same single line; the spinner's `\033[K` clear-to-EOL still applies, so redraw stays correct.
