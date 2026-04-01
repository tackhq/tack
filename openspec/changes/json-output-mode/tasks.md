## 1. Output Abstraction

- [ ] 1.1 Define `Emitter` interface in `internal/output/output.go` with methods: PlaybookStart, PlayStart, PlanTask, TaskStart, TaskResult, HostRecap, PlaybookRecap, Error
- [ ] 1.2 Refactor existing text output functions into a `TextEmitter` struct implementing the interface
- [ ] 1.3 Create `JSONEmitter` struct that emits NDJSON events with type and timestamp fields

## 2. JSON Event Types

- [ ] 2.1 Implement `playbook_start` event: `{type, timestamp, playbook}`
- [ ] 2.2 Implement `play_start` event: `{type, timestamp, play, hosts}`
- [ ] 2.3 Implement `plan_task` event: `{type, timestamp, host, task, module, action, params}`
- [ ] 2.4 Implement `task_result` event: `{type, timestamp, host, task, module, status, changed, message, data}`
- [ ] 2.5 Implement `host_recap` event: `{type, timestamp, host, ok, changed, failed, skipped, duration}`
- [ ] 2.6 Implement `playbook_recap` event: `{type, timestamp, ok, changed, failed, skipped, duration, success}`
- [ ] 2.7 Implement `error` event: `{type, timestamp, message}`

## 3. CLI Integration

- [ ] 3.1 Add `--output` flag to `cmd/bolt/main.go` accepting `text` or `json`
- [ ] 3.2 Auto-enable `--auto-approve` when `--output json` is set
- [ ] 3.3 Pass selected emitter to executor

## 4. Executor Integration

- [ ] 4.1 Update executor to use Emitter interface instead of direct output.Print* calls
- [ ] 4.2 Skip approval prompt when JSON mode is active
- [ ] 4.3 Ensure errors go to stderr in JSON mode

## 5. Testing

- [ ] 5.1 Unit test JSONEmitter: verify each event type produces valid JSON with required fields
- [ ] 5.2 Unit test TextEmitter: verify existing output behavior unchanged
- [ ] 5.3 Integration test: run playbook with `--output json` and parse all output lines as valid JSON
- [ ] 5.4 Unit test: `--output json` implies auto-approve
