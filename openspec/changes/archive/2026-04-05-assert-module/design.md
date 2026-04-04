## Context

Bolt has a mature conditional expression engine in `internal/executor/conditions.go` that parses and evaluates `when:` expressions against a `PlayContext` (facts, vars, registered results, extra vars). This engine supports `==`, `!=`, `<`, `>`, `<=`, `>=`, `in`, `not in`, `is defined`, `is not defined`, `and`, `or`, `not`, and parenthesized grouping.

The entry point is `evaluateConditionExpr(condition string, pctx *PlayContext) (bool, error)`, currently package-private to `executor`.

All existing modules follow a uniform pattern: they implement `Module.Run(ctx, connector, params)` and execute remotely via the connector. Assert is conceptually different — it evaluates expressions locally against the play context. It never touches the target host and doesn't need the connector.

## Goals / Non-Goals

**Goals:**
- Ship a user-visible `assert` task that matches the Ansible mental model: list of conditions, fail fast with clear message.
- Reuse the existing condition parser/evaluator — zero duplication, full operator parity with `when:`.
- Respect `block:`/`rescue:`/`always:`, `tags:`, `--dry-run`, `register:`, `when:`.
- Consistent output formatting (human + JSON output modes).

**Non-Goals:**
- No new expression operators (those would go into `enhanced-conditionals` scope).
- No remote execution. Assert is local-only to the control host.
- No `ansible.builtin.assert` 1:1 compatibility — just the subset that matches Bolt's existing conditional semantics.

## Decisions

### Decision 1: Executor-level task type, not a remote Module

**Chosen:** Handle `assert:` as a special task type in the executor, similar to how `block:`, `include_tasks:`, and `debug:` are handled. The task dispatcher inspects the task shape and routes to an `executeAssert` function that has direct access to `PlayContext`.

**Alternatives considered:**
- **(A) Implement as a normal `Module`:** Would require passing the full `PlayContext` (or at least the evaluator + vars snapshot) through the `Module.Run` interface. That leaks executor internals into every module and forces a bigger interface change. Rejected.
- **(B) Pre-evaluate conditions in executor, pass booleans to module:** The executor would resolve `that:` into booleans before invoking the module. Assert just reads booleans. Works, but then the module has no useful logic — and we lose the ability to report *which* condition failed by its source expression. Rejected.

**Rationale:** Assert is a control-flow primitive, not a system operation. The executor already owns control-flow primitives. Keeping assert at the executor layer matches existing patterns (block/rescue, include_tasks) and avoids bending the Module interface.

### Decision 2: Keep `evaluateConditionExpr` package-private

The assert handler lives in `internal/executor/assert.go` (same package as conditions.go), so it calls `evaluateConditionExpr` directly with no visibility change required.

### Decision 3: `that:` accepts string or list of strings

YAML ergonomics: a single condition is common enough that writing `that: "facts.os_type == 'Linux'"` should work alongside the list form. The parser normalizes both to `[]string`.

### Decision 4: Default failure message format

When `fail_msg` is not provided, emit: `Assertion failed: <first-failing-condition-source>`. When multiple conditions fail, the default lists all of them, one per line. Users who want a custom message set `fail_msg`.

### Decision 5: `register:` result shape

Registered result contains:
- `changed: false` (assert never changes state)
- `failed: <bool>`
- `msg: <fail_msg or success_msg or default>`
- `evaluated_conditions: [{expr: "...", result: true/false}, ...]`

This lets downstream tasks inspect which conditions were evaluated, matching Ansible's registered output for assert.

### Decision 6: Dry-run and check-mode behavior

Assert evaluates identically in `--dry-run`: conditions are read-only by construction. In check mode, a failing assert still fails the play — this matches Ansible and is the user's intent (fail fast regardless of mode).

### Decision 7: Output module name as `assert`

For shorthand and documentation purposes, the task is referenced as `assert:` in YAML. Since it's not a `Module` registry entry, the playbook parser needs to recognize `assert` as a built-in task keyword alongside `block`, `include_tasks`, `debug`, etc.

## Risks / Trade-offs

- **[Risk]** Users expect assert to be a pluggable `Module` and write tooling that discovers it via the module registry. → **Mitigation:** Document clearly that assert is a built-in task keyword, list it in the module docs alongside `block`/`include_tasks`, and consider exposing it through a thin registry facade if demand materializes.
- **[Risk]** Error messages for parse errors in `that:` expressions reference the existing condition parser, which may not give great positional info. → **Mitigation:** Out of scope for this change; file a follow-up if assert adoption surfaces pain.
- **[Trade-off]** No access to connector means assert cannot validate remote state (e.g. "is file X present on target?"). → **Mitigation:** Users can combine `command:` + `register:` + `assert:` for that pattern. A future `remote_assert` could be considered.
- **[Trade-off]** Treating assert as executor-special means it doesn't appear in the `Module` registry and can't be documented via the `Describer` interface. → **Mitigation:** Extend the docs generator to include built-in task keywords.
