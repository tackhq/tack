package export

import (
	"fmt"
	"strconv"
	"strings"
)

// evalCondition evaluates a when-condition expression against the compiler's variable context.
// This is a simplified version of the executor's condition evaluator that works with
// the export-time variable context (no connector needed).
func (c *Compiler) evalCondition(condition string) (bool, error) {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return true, nil
	}

	return evalExpr(condition, c.vars)
}

// evalExpr evaluates a boolean expression.
func evalExpr(expr string, vars map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)

	// Handle "and" / "or" (lowest precedence)
	// Split on " and " / " or " respecting parentheses
	if parts, ok := splitLogical(expr, " and "); ok {
		for _, part := range parts {
			result, err := evalExpr(part, vars)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}

	if parts, ok := splitLogical(expr, " or "); ok {
		for _, part := range parts {
			result, err := evalExpr(part, vars)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}

	// Handle "not"
	if strings.HasPrefix(expr, "not ") {
		result, err := evalExpr(expr[4:], vars)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Handle parentheses
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return evalExpr(expr[1:len(expr)-1], vars)
	}

	// Handle "is defined" / "is not defined"
	if strings.HasSuffix(expr, " is defined") {
		varName := strings.TrimSuffix(expr, " is defined")
		varName = strings.TrimSpace(varName)
		val := resolveVarPath(varName, vars)
		return val != nil, nil
	}
	if strings.HasSuffix(expr, " is not defined") {
		varName := strings.TrimSuffix(expr, " is not defined")
		varName = strings.TrimSpace(varName)
		val := resolveVarPath(varName, vars)
		return val == nil, nil
	}

	// Handle comparison operators
	for _, op := range []string{"==", "!=", "<=", ">=", "<", ">", " in ", " not in "} {
		if idx := strings.Index(expr, op); idx >= 0 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])
			return evalComparison(left, op, right, vars)
		}
	}

	// Truthy check: bare variable name
	val := resolveValue(expr, vars)
	return isTruthy(val), nil
}

// evalComparison evaluates a comparison expression.
func evalComparison(left, op, right string, vars map[string]any) (bool, error) {
	op = strings.TrimSpace(op)
	lVal := resolveValue(left, vars)
	rVal := resolveValue(right, vars)

	switch op {
	case "==":
		return fmt.Sprintf("%v", lVal) == fmt.Sprintf("%v", rVal), nil
	case "!=":
		return fmt.Sprintf("%v", lVal) != fmt.Sprintf("%v", rVal), nil
	case "<", ">", "<=", ">=":
		lNum, lOk := toFloat(lVal)
		rNum, rOk := toFloat(rVal)
		if !lOk || !rOk {
			return false, fmt.Errorf("cannot compare non-numeric values: %v %s %v", lVal, op, rVal)
		}
		switch op {
		case "<":
			return lNum < rNum, nil
		case ">":
			return lNum > rNum, nil
		case "<=":
			return lNum <= rNum, nil
		case ">=":
			return lNum >= rNum, nil
		}
	case "in":
		return evalIn(lVal, rVal), nil
	case "not in":
		return !evalIn(lVal, rVal), nil
	}

	return false, fmt.Errorf("unknown operator: %s", op)
}

// resolveValue resolves a string to a typed value.
func resolveValue(s string, vars map[string]any) any {
	s = strings.TrimSpace(s)

	// String literal
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}

	// Boolean literal
	if s == "true" || s == "True" {
		return true
	}
	if s == "false" || s == "False" {
		return false
	}

	// Number literal
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n
	}

	// List literal
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return parseListLiteral(s)
	}

	// Variable lookup
	val := resolveVarPath(s, vars)
	if val != nil {
		return val
	}

	return s // return as-is
}

func parseListLiteral(s string) []any {
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil
	}
	parts := strings.Split(inner, ",")
	result := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && ((p[0] == '\'' && p[len(p)-1] == '\'') || (p[0] == '"' && p[len(p)-1] == '"')) {
			p = p[1 : len(p)-1]
		}
		result = append(result, p)
	}
	return result
}

func isTruthy(val any) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v != "" && v != "false" && v != "False" && v != "0"
	case int:
		return v != 0
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return true
}

func evalIn(needle, haystack any) bool {
	switch h := haystack.(type) {
	case []any:
		needleStr := fmt.Sprintf("%v", needle)
		for _, item := range h {
			if fmt.Sprintf("%v", item) == needleStr {
				return true
			}
		}
	case string:
		if s, ok := needle.(string); ok {
			return strings.Contains(h, s)
		}
	}
	return false
}

func toFloat(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	return 0, false
}

// splitLogical splits an expression on a logical operator, respecting parentheses.
func splitLogical(expr, op string) ([]string, bool) {
	depth := 0
	idx := 0
	var parts []string

	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && i+len(op) <= len(expr) && expr[i:i+len(op)] == op {
			parts = append(parts, expr[idx:i])
			idx = i + len(op)
			i += len(op) - 1
		}
	}

	if len(parts) == 0 {
		return nil, false
	}
	parts = append(parts, expr[idx:])
	return parts, true
}
