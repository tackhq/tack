## 1. Output layer: data + formatting

- [x] 1.1 Add `formatTags(tags []string) string` helper in `internal/output` returning `""` for empty input and `[a,b]` (comma-joined) otherwise
- [x] 1.2 Add `Tags []string` field to `output.PlannedTask` in `internal/output/output.go`
- [x] 1.3 Extend the `Emitter.TaskResult` signature in `internal/output/emitter.go` to `TaskResult(name, status string, changed bool, message string, tags []string)`

## 2. Output layer: rendering

- [x] 2.1 In `Output.TaskResult` (output.go), append a gray `formatTags(tags)` suffix after the name in both the spinner-redraw branch and the plain branch; omit when empty
- [x] 2.2 In `DisplayPlan` (single-host) append the gray tag suffix from `PlannedTask.Tags`
- [x] 2.3 In `renderMultiHostPlanLine` (multi-host) append the gray tag suffix from `PlannedTask.Tags`
- [x] 2.4 In `JSONEmitter.TaskResult` (json.go) include a `tags` array on the task event

## 3. Executor: thread effective tags

- [x] 3.1 Add an `eTags []string` parameter to `runSingleTask` and forward it to the success/fail `TaskResult` calls
- [x] 3.2 Update `applyHostPlan` and block-execution call sites to pass the already-computed `eTags` into `runSingleTask`
- [x] 3.3 Pass `eTags` to the skip-path `TaskResult` calls (tag-skip, ignored-failure, etc.) in `applyHostPlan` and block execution
- [x] 3.4 Populate `PlannedTask.Tags` with `eTags` at the construction sites in `planTasks`

## 4. Update remaining Emitter implementers

- [x] 4.1 Update `nullEmitter.TaskResult` (internal/executor) to the new signature
- [x] 4.2 Update `testOutputCapture.TaskResult` (internal/executor) to the new signature, capturing tags if useful for assertions
- [x] 4.3 Update `countingEmitter` / any other test emitter to the new signature

## 5. Tests

- [x] 5.1 Unit test `formatTags` (empty → "", single, multiple)
- [x] 5.2 Output test: `TaskResult` with tags appends `[a,b]`; with no tags appends nothing; gray code absent under `--no-color`
- [x] 5.3 Plan test: single-host and multi-host plan lines include the tag suffix for tagged tasks and none for untagged
- [x] 5.4 JSON test: task event includes the `tags` array
- [x] 5.5 Update any existing plan/result golden-output assertions affected by the new suffix

## 6. Verify

- [x] 6.1 `make build`
- [x] 6.2 Manual: playbook with a play-level tag + task-level tag; `tack run --check` and `tack run -y` show `[playtag,tasktag]`; `tack run -t tasktag` narrows to the displayed tasks
- [x] 6.3 Confirm piped/`--no-color` output is plain (no color codes) and JSON output carries `tags`
- [x] 6.4 `make test` and `make lint`
