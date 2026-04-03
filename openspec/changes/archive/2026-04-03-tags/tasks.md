## 1. Playbook Structs and Parsing

- [x] 1.1 Add `Tags []string` field to `Task` struct in `internal/playbook/playbook.go`
- [x] 1.2 Add `Tags []string` field to `Play` struct in `internal/playbook/playbook.go`
- [x] 1.3 Add `tags` to `knownTaskFields` in `internal/playbook/parser.go` and parse it (string or list of strings) into `Task.Tags`
- [x] 1.4 Parse `tags:` on play definitions in `internal/playbook/parser.go`
- [x] 1.5 Support tags on role references — extend role parsing to handle `{role: name, tags: [...]}` map syntax alongside the existing string syntax
- [x] 1.6 Parse `tags:` on block definitions so block-level tags are stored on the block task

## 2. Tag Filtering Logic

- [x] 2.1 Add `Tags` and `SkipTags` fields (`[]string`) to the `Executor` struct in `internal/executor/executor.go`
- [x] 2.2 Implement `shouldRunTask(effectiveTags []string, tags []string, skipTags []string) bool` helper function in executor that applies tag/skip-tag matching with `always`/`never` semantics
- [x] 2.3 Implement `effectiveTags(task, playTags, roleTags, blockTags []string) []string` helper that computes the union of inherited and own tags
- [x] 2.4 Unit tests for `shouldRunTask` covering: basic matching, OR logic, skip-tags, combined tags+skip-tags, `always` tag, `never` tag, no filters

## 3. Executor Integration

- [x] 3.1 Wire tag filtering into the main task execution loop — call `shouldRunTask` before executing each task and skip with "skipped (tag)" status if filtered out
- [x] 3.2 Propagate play-level tags through the execution context
- [x] 3.3 Propagate role-level tags to role tasks during role loading/expansion
- [x] 3.4 Propagate block-level tags to block/rescue/always tasks during block execution
- [x] 3.5 Ensure handlers execute when notified regardless of `--tags` filter, but respect `--skip-tags`
- [x] 3.6 Ensure tag filtering applies identically in plan/check mode

## 4. CLI Plumbing

- [x] 4.1 Wire existing `--tags` and `--skip-tags` flags from `cmd/bolt/main.go` to the executor (read flag values and set `Executor.Tags` / `Executor.SkipTags`)
- [x] 4.2 Add `--tags` and `--skip-tags` support to the validate command if applicable

## 5. Tests and Validation

- [x] 5.1 Add parser tests for `tags:` field on tasks (string and list forms)
- [x] 5.2 Add parser tests for `tags:` on plays, blocks, and role references
- [x] 5.3 Add executor integration tests: run with `--tags` filters and verify only matching tasks execute
- [x] 5.4 Add executor integration tests: `always` and `never` special tag behavior
- [x] 5.5 Add executor integration tests: tag inheritance through blocks and roles
- [x] 5.6 Add executor integration test: handler execution ignores `--tags` but respects `--skip-tags`
- [x] 5.7 Add example playbook demonstrating tags usage
