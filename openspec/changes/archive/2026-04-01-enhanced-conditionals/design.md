## Context

The current condition evaluator in `executor.go` uses a series of if/else checks with `strings.Contains` and `strings.Split` to match patterns like `X == Y`, `X != Y`, `not X`, and bare truthiness. This approach cannot handle compound expressions or operator precedence.

Ansible uses Jinja2's full expression engine. Bolt doesn't need that complexity but needs a proper parser for compound boolean expressions.

## Goals / Non-Goals

**Goals:**
- Parse and evaluate compound boolean expressions with `and`/`or`
- Support comparison operators `<`, `>`, `<=`, `>=` with smart type coercion
- Support `in` and `not in` for list membership
- Support parenthesized grouping
- Maintain 100% backward compatibility with existing conditions
- Keep the implementation small and self-contained (no external parser libraries)

**Non-Goals:**
- Full Jinja2 expression compatibility
- Arithmetic expressions (`+`, `-`, `*`, `/`)
- Function calls in conditions (e.g., `when: len(list) > 0`)
- Regular expression matching
- Ternary expressions

## Decisions

### 1. Recursive descent parser in a new file `conditions.go`

A hand-written recursive descent parser with a simple lexer. Grammar:

```
expr       -> or_expr
or_expr    -> and_expr ("or" and_expr)*
and_expr   -> not_expr ("and" not_expr)*
not_expr   -> "not" not_expr | comparison
comparison -> primary (("==" | "!=" | "<" | ">" | "<=" | ">=" | "in" | "not in" | "is defined" | "is not defined") primary)?
primary    -> "(" expr ")" | string_literal | number | boolean | variable
```

**Alternative considered:** Using `go/ast` or `govaluate` library. Rejected -- `go/ast` parses Go syntax (wrong grammar), and adding a dependency for a ~200-line parser is overkill.

### 2. Smart type coercion for comparisons

When comparing with `<`, `>`, `<=`, `>=`:
- If both sides parse as numbers (int or float), compare numerically
- Otherwise, compare as strings lexicographically
- `==` and `!=` keep current behavior (string comparison)

### 3. `in` operates on interpolated lists

`value in list_var` checks membership. The right-hand side must resolve to a list (from variables). Inline list syntax `value in ['a', 'b']` is also supported via the parser.

### 4. Variables interpolated before parsing

The existing variable interpolation (`{{ var }}`) runs first, then the condition string is parsed. This means `when: {{ my_condition }}` still works -- it expands to a string that gets parsed.

## Risks / Trade-offs

- **[Risk] Breaking existing conditions** -- Some edge cases in string matching might behave differently with a real parser. Mitigation: Comprehensive test suite against all existing condition patterns.
- **[Risk] Performance** -- Parser runs per-task per-host. Mitigation: Conditions are short strings; parsing is O(n) and negligible vs. command execution.
- **[Trade-off] No arithmetic** -- Users can't do `when: port + 1 == 8081`. Acceptable; arithmetic in conditions is rare and can be added later.
