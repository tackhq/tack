## Why

Playbooks currently discover misconfiguration late — a missing variable, wrong OS family, or out-of-range value only surfaces when a downstream task fails with a cryptic error. Users need a way to validate preconditions at the top of a play and fail fast with a clear message, so that runs abort before touching the system when expectations aren't met.

## What Changes

- Add an `assert` task type that evaluates a list of boolean expressions using the existing `when:` conditional engine and fails the task if any evaluate to false.
- Params: `that` (required, list of condition strings or a single string), `fail_msg` (optional custom failure message), `success_msg` (optional success message shown when all conditions pass), `quiet` (bool, suppress per-condition output on success).
- On failure, the task fails with `fail_msg` (or a default message naming the first failing condition). Failures respect `block:`/`rescue:`/`always:` semantics — an assert inside a block can trigger its rescue.
- Integrates with existing task machinery: `when:`, `tags:`, `--dry-run` (evaluates conditions normally; never mutates system), `--diff` (no-op), `register:` (records pass/fail result).
- Works with every connector (local, SSH, SSM, Docker) because no remote command is dispatched — the executor evaluates conditions locally against the play context.

## Capabilities

### New Capabilities
- `assert-module`: Precondition validation that evaluates boolean expressions against play vars, facts, registered results, and extra vars; fails the task with a descriptive message when expectations are not met.

### Modified Capabilities

None.

## Impact

- New package: `internal/module/assert/` (for documentation/registry surface) OR executor-level handling — see design.md.
- Minor refactor: the condition evaluator in `internal/executor/conditions.go` (`evaluateConditionExpr`) may need a narrow public entry point so the assert implementation can reuse it without duplicating the parser.
- No new external dependencies.
- Documentation: add `assert` to `README.md`, `docs/modules/`, and `llms.txt`.
- Example playbook showing common validation patterns (OS check, required vars, version ranges).
