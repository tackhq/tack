## ADDED Requirements

### Requirement: Boolean AND operator
The `when:` condition evaluator SHALL support the `and` operator to combine two conditions, returning true only when both sides are true. `and` SHALL bind tighter than `or`.

#### Scenario: Both conditions true
- **WHEN** condition is `facts.os_family == 'Debian' and facts.arch == 'x86_64'` and both facts match
- **THEN** the condition SHALL evaluate to true

#### Scenario: One condition false
- **WHEN** condition is `facts.os_family == 'Debian' and facts.arch == 'arm64'` and arch is x86_64
- **THEN** the condition SHALL evaluate to false

### Requirement: Boolean OR operator
The `when:` condition evaluator SHALL support the `or` operator to combine two conditions, returning true when at least one side is true. `or` SHALL have lower precedence than `and`.

#### Scenario: One condition true
- **WHEN** condition is `facts.os_type == 'Linux' or facts.os_type == 'Darwin'` and os_type is Darwin
- **THEN** the condition SHALL evaluate to true

#### Scenario: Precedence with and/or
- **WHEN** condition is `a or b and c` where a=false, b=true, c=true
- **THEN** the condition SHALL evaluate to true (parsed as `a or (b and c)`)

### Requirement: Comparison operators
The `when:` condition evaluator SHALL support `<`, `>`, `<=`, `>=` operators. When both operands are numeric, comparison SHALL be numeric. Otherwise, comparison SHALL be lexicographic string comparison.

#### Scenario: Numeric comparison
- **WHEN** condition is `facts.os_version_id >= '22'` and os_version_id is "24"
- **THEN** the condition SHALL evaluate to true (numeric: 24 >= 22)

#### Scenario: String comparison fallback
- **WHEN** condition is `facts.os_name > 'A'` and os_name is "Ubuntu"
- **THEN** the condition SHALL evaluate to true (lexicographic: "Ubuntu" > "A")

### Requirement: Membership operator (in)
The `when:` condition evaluator SHALL support `in` and `not in` operators for list membership testing.

#### Scenario: Value in list variable
- **WHEN** condition is `facts.os_type in supported_os` and supported_os is ['Linux', 'Darwin'] and os_type is 'Linux'
- **THEN** the condition SHALL evaluate to true

#### Scenario: Value not in list
- **WHEN** condition is `facts.os_type not in ['Windows']` and os_type is 'Linux'
- **THEN** the condition SHALL evaluate to true

#### Scenario: Inline list syntax
- **WHEN** condition is `facts.os_family in ['Debian', 'RedHat']`
- **THEN** the parser SHALL support inline list literals

### Requirement: Parenthesized grouping
The `when:` condition evaluator SHALL support parentheses for explicit precedence control.

#### Scenario: Parentheses override precedence
- **WHEN** condition is `(a or b) and c` where a=true, b=false, c=false
- **THEN** the condition SHALL evaluate to false (without parens: `a or (b and c)` = true)

### Requirement: Backward compatibility
All existing condition syntax SHALL continue to work identically, including: bare truthiness, `not`, `==`, `!=`, `is defined`, `is not defined`, and registered variable access.

#### Scenario: Existing equality check
- **WHEN** condition is `facts.os_type == 'Linux'`
- **THEN** the condition SHALL evaluate identically to current behavior

#### Scenario: Existing truthiness check
- **WHEN** condition is `some_var`
- **THEN** the condition SHALL evaluate to true if some_var is truthy
