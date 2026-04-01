## Why

Bolt's `when:` conditions currently support only simple equality checks (`==`, `!=`) and truthiness. Users cannot express compound conditions like `when: facts.os_family == 'Debian' and facts.os_version_id >= '22'` or membership tests like `when: facts.os_type in ['Linux', 'Darwin']`. This forces users to split logic across multiple tasks or nest includes, creating playbook bloat and reducing readability.

## What Changes

- Add boolean operators: `and`, `or` with proper precedence (`and` binds tighter than `or`)
- Add comparison operators: `<`, `>`, `<=`, `>=` (string comparison, numeric when both sides are numbers)
- Add membership operators: `in`, `not in` (check if value is in a list)
- Add parenthesized grouping for explicit precedence control
- Replace ad-hoc string matching with a recursive descent expression parser
- Maintain backward compatibility with all existing condition syntax

## Capabilities

### New Capabilities
- `enhanced-conditionals`: Boolean operators, comparison operators, membership tests, and parenthesized grouping in `when:` conditions

### Modified Capabilities

_None — all existing condition syntax remains valid._

## Impact

- **New code**: `internal/executor/conditions.go` — expression lexer and recursive descent parser
- **Modified code**: `internal/executor/executor.go` — replace `evaluateCondition()` with new parser
- **No dependency changes** — pure Go implementation
- **No breaking changes** — existing conditions continue to work identically
