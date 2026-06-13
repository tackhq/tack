## Why

Tasks can be filtered with `-t/--tags` and `--skip-tags`, but the output never shows which tags a task carries. To narrow the next run (e.g. `tack run -t deploy`), a user must open the playbook and roles to discover the tag names. Surfacing each task's effective tags inline closes that loop: read the tag off the line, re-run with `-t`.

## What Changes

- Display each task's **effective tags** (its own tags plus inherited play/role/block tags, deduped — exactly the set `--tags`/`--skip-tags` match against) inline in `tack run` output.
- Tags appear as a dimmed, bracketed suffix after the task name, e.g. `✓ install nginx [web,deploy]`. The bracket is omitted entirely when a task has no effective tags.
- Tags are shown in **both** the plan preview (single-host and multi-host) and the per-task result line during apply.
- The dim styling uses the existing gray color, so under `--no-color` the tags render as plain text.
- The JSON emitter includes a `tags` array on each task event for machine consumption.
- **BREAKING (internal only)**: the `Emitter.TaskResult` method signature gains a `tags []string` parameter. This is an internal Go interface, not a user-facing API; no playbook or CLI behavior is removed.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `tags`: Adds requirements for tag **visibility** in output. The existing tag model (definition, inheritance, `--tags`/`--skip-tags` filtering) is unchanged; this adds that a task's effective tags SHALL be displayed in the plan preview and apply result lines, and included in JSON output.

## Impact

- `internal/output/emitter.go` — `TaskResult` signature gains `tags []string`.
- `internal/output/output.go` — `PlannedTask` gains a `Tags` field; plan and result-line rendering append a dimmed `[..]` suffix; new `formatTags` helper.
- `internal/output/json.go` — task event includes a `tags` array.
- `internal/executor/executor.go` — thread the already-computed effective tags (`effectiveTags()` in `internal/executor/tags.go`) into `runSingleTask`, the `TaskResult` skip-path call sites, and `PlannedTask` construction in `planTasks`.
- All `Emitter` implementers (text, JSON, and the test emitters `nullEmitter`/`testOutputCapture`/`countingEmitter` in `internal/executor`) update to the new `TaskResult` signature.
- No new dependencies. No CLI flags added. Existing plan/result golden tests updated for the new suffix.
