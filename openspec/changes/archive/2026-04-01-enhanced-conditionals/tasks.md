## 1. Lexer

- [x] 1.1 Create `internal/executor/conditions.go` with token types: AND, OR, NOT, EQ, NEQ, LT, GT, LTE, GTE, IN, LPAREN, RPAREN, LBRACKET, RBRACKET, COMMA, STRING, NUMBER, BOOL, IDENT, IS_DEFINED, IS_NOT_DEFINED, EOF
- [x] 1.2 Implement lexer that tokenizes condition strings, handling quoted strings, numbers, identifiers, and multi-char operators (`<=`, `>=`, `!=`, `not in`, `is defined`, `is not defined`)

## 2. Parser

- [x] 2.1 Implement recursive descent parser with grammar: expr -> or_expr -> and_expr -> not_expr -> comparison -> primary
- [x] 2.2 Implement primary expression parsing: parenthesized groups, string literals, numbers, booleans, variable references, inline list literals
- [x] 2.3 Implement comparison operators: `==`, `!=`, `<`, `>`, `<=`, `>=`, `in`, `not in`, `is defined`, `is not defined`

## 3. Evaluator

- [x] 3.1 Implement AST evaluator that walks parsed expressions and produces boolean results
- [x] 3.2 Implement smart type coercion for comparison operators (numeric when both sides parse as numbers, string otherwise)
- [x] 3.3 Implement `in` / `not in` evaluation against list values (variable lists and inline list literals)
- [x] 3.4 Implement variable resolution integration -- connect to PlayContext for variable lookup

## 4. Integration

- [x] 4.1 Replace `evaluateCondition()` in `executor.go` with new parser-based evaluation
- [x] 4.2 Ensure variable interpolation (`{{ }}`) still runs before condition parsing
- [x] 4.3 Ensure plan phase condition evaluation works with new parser (handles registered variable detection)

## 5. Testing

- [x] 5.1 Unit test all operators individually: and, or, not, ==, !=, <, >, <=, >=, in, not in
- [x] 5.2 Unit test precedence: `a or b and c` -> `a or (b and c)`
- [x] 5.3 Unit test parenthesized grouping: `(a or b) and c`
- [x] 5.4 Unit test type coercion: numeric vs string comparison
- [x] 5.5 Unit test backward compatibility: all existing condition patterns
- [x] 5.6 Unit test inline list literals: `x in ['a', 'b']`
- [x] 5.7 Unit test edge cases: empty strings, nil values, undefined variables
