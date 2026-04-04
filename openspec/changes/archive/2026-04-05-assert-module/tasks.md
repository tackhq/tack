## 1. Parser & Task Model

- [x] 1.1 Add `assert` as a recognized task keyword in `internal/playbook/` (alongside `block`, `include_tasks`); add an `Assert` field to the task struct carrying `that`, `fail_msg`, `success_msg`, `quiet`
- [x] 1.2 Normalize `that:` — accept both string and list-of-strings in YAML; emit validation error on missing/empty/non-string elements
- [x] 1.3 Ensure shorthand expansion leaves `assert` tasks unchanged
- [x] 1.4 Update the playbook parser tests with valid + invalid assert task fixtures

## 2. Executor Handler

- [x] 2.1 Create `internal/executor/assert.go` with `executeAssert(ctx, pctx, task)` that evaluates each condition via `evaluateConditionExpr`
- [x] 2.2 Route `assert:` tasks in the executor dispatcher before the module-registry path
- [x] 2.3 Build the registered-result map: `{changed:false, failed:bool, msg:string, evaluated_conditions:[{expr,result}]}`
- [x] 2.4 Default failure message: join failing expressions one per line under `Assertion failed:` prefix
- [x] 2.5 Emit `success_msg` on pass when provided; honor `quiet:true` to suppress per-condition output
- [x] 2.6 Return task-level failure on any false condition so block/rescue/always semantics apply
- [x] 2.7 Bypass connector entirely — assert never calls `conn.Execute`

## 3. Integration With Existing Machinery

- [x] 3.1 `when:` is evaluated before assert runs (skip assert when `when` is false) — verify existing dispatcher path covers this
- [x] 3.2 `tags:` inclusion/exclusion works on assert tasks — add tag-filter tests
- [x] 3.3 `register:` captures the result map — add test with downstream task reading registered value
- [x] 3.4 `--dry-run` / check mode evaluates normally and still fails on false conditions
- [x] 3.5 `--diff` is a no-op for assert tasks

## 4. Output & Logging

- [x] 4.1 Human output: PASS shows task name + success_msg (or default OK); FAIL shows fail_msg + listed failing expressions
- [x] 4.2 JSON output: include `evaluated_conditions` array in the task result payload
- [x] 4.3 Redaction: assert output MUST NOT print raw var values that came from vault or `no_log:true` tasks — follow existing redaction helpers

## 5. Tests

- [x] 5.1 Unit tests in `internal/executor/assert_test.go` covering: pass/fail, single vs list `that`, custom + default messages, quiet mode, register shape, dry-run behavior, validation errors
- [x] 5.2 Integration test fixture `tests/integration/assert_playbook.yaml` that exercises assert inside a block/rescue and registers results used downstream
- [x] 5.3 Operator-parity tests: one scenario per operator class (`==`, `in`, `is defined`, `and`, comparison) to prove evaluator reuse
- [x] 5.4 Ensure `go test -race ./...` passes

## 6. Documentation

- [x] 6.1 Add `docs/modules/assert.md` (or equivalent location) describing params, examples, and the "built-in keyword, not registered module" note
- [x] 6.2 Update `README.md` feature list and module table to include `assert`
- [x] 6.3 Update `llms.txt` with assert syntax and example
- [x] 6.4 Add example playbook `examples/assert-preflight.yaml` showing OS check + required-var patterns
- [x] 6.5 Update `ROADMAP.md` — mark `assert` module as implemented

## 7. Release

- [x] 7.1 Run `make lint` and `make test`
- [x] 7.2 Validate example playbooks with `bolt validate`
- [x] 7.3 Manual smoke test: run `examples/assert-preflight.yaml` against local connector
