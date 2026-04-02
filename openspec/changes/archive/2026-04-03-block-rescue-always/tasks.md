## 1. Playbook structs — Add block/rescue/always fields

- [x] 1.1 Add `Block []*Task`, `Rescue []*Task`, `Always []*Task` fields to `Task` struct in `internal/playbook/playbook.go`
- [x] 1.2 Add `IsBlock() bool` helper method on `Task` (returns `len(t.Block) > 0`)
- [x] 1.3 Update `Task.Validate()` to reject tasks with both `Block` and `Module` set
- [x] 1.4 Update `Task.Validate()` to reject `Rescue` or `Always` without `Block`
- [x] 1.5 Update `Task.String()` to return block name or "block" for unnamed blocks

## 2. Parser — Parse block/rescue/always directives

- [x] 2.1 Add `block`, `rescue`, `always` to `knownTaskFields` map in `internal/playbook/parser.go`
- [x] 2.2 In `parseRawTask()`, parse `block:` as a nested task list using `parseTaskList()`
- [x] 2.3 Parse `rescue:` and `always:` similarly as nested task lists
- [x] 2.4 Write unit tests: basic block parsing, block with rescue/always, block with name/when/sudo, reject block+module, reject rescue without block

## 3. Executor — Implement runBlock()

- [x] 3.1 Add `runBlock(ctx, pctx, task, stats, visitedPaths)` method to Executor
- [x] 3.2 Implement block execution: run block tasks sequentially, stop on first failure
- [x] 3.3 Implement rescue execution: if block failed and rescue exists, run rescue tasks sequentially
- [x] 3.4 Implement always execution: run always tasks regardless of block/rescue outcome
- [x] 3.5 Implement error propagation: recovered if rescue succeeded; error if block failed without rescue or rescue also failed
- [x] 3.6 Handle block-level `when:` — skip entire block (including rescue/always) if false
- [x] 3.7 Handle block-level `sudo:` inheritance — apply to child tasks that don't override
- [x] 3.8 Integrate `runBlock()` into the main task loop in `runPlayOnHost()` (alongside include handling)
- [x] 3.9 Support nested blocks — `runBlock()` calls back into the task loop which handles blocks recursively
- [x] 3.10 Support includes within block/rescue/always sections (pass visitedPaths through)

## 4. Plan mode

- [x] 4.1 Update `planTasks()` to detect block tasks and display them with grouped structure
- [x] 4.2 Show rescue/always sections in plan output if present
- [x] 4.3 Handle block-level `when:` in plan (evaluate condition, show skip/run status for entire block)
- [x] 4.4 Write unit test: plan output for block with rescue/always

## 5. Unit and integration tests

- [x] 5.1 Write executor test: block succeeds — all block tasks run, rescue skipped, always runs
- [x] 5.2 Write executor test: block fails — rescue runs, always runs, error recovered
- [x] 5.3 Write executor test: block fails, no rescue — always runs, error propagated
- [x] 5.4 Write executor test: block and rescue both fail — always runs, error propagated
- [x] 5.5 Write executor test: block-level when=false skips entire block
- [x] 5.6 Write executor test: nested block within rescue

## 6. Documentation

- [x] 6.1 Add block/rescue/always section to README.md with YAML examples
- [x] 6.2 Add feature bullet to README Features list
- [x] 6.3 Create example playbook: `examples/block-rescue/playbook.yml` demonstrating a deploy-with-rollback pattern
- [x] 6.4 Update ROADMAP.md to mark block/rescue/always as complete
