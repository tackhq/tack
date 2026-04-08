package export

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// varRe matches {{ variable }} syntax, same as executor.
var varRe = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

const factSentinel = "__TACK_FACT_NOT_GATHERED__"

// interpolateWithVars resolves {{ var }} references against the given variable map.
// It resolves recursively up to maxDepth to handle variables that reference other variables.
func interpolateWithVars(s string, vars map[string]any, noFacts bool) any {
	return interpolateRecursive(s, vars, noFacts, 0)
}

const maxInterpolateDepth = 10

func interpolateRecursive(s string, vars map[string]any, noFacts bool, depth int) any {
	if depth >= maxInterpolateDepth {
		return s
	}

	trimmed := strings.TrimSpace(s)

	// Full-string variable reference (preserve type)
	if varRe.MatchString(trimmed) {
		match := varRe.FindStringSubmatch(trimmed)
		if match != nil && strings.TrimSpace(match[0]) == trimmed {
			expr := strings.TrimSpace(match[1])
			val := resolveExpr(expr, vars, noFacts)
			if val != nil {
				// Recurse if result is a string that still has {{ }}
				if strVal, ok := val.(string); ok && varRe.MatchString(strVal) {
					return interpolateRecursive(strVal, vars, noFacts, depth+1)
				}
				return val
			}
		}
	}

	// Inline interpolation (always returns string)
	result := varRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := varRe.FindStringSubmatch(match)
		if inner == nil {
			return match
		}
		expr := strings.TrimSpace(inner[1])
		val := resolveExpr(expr, vars, noFacts)
		if val == nil {
			return match // leave unresolved
		}
		return fmt.Sprintf("%v", val)
	})

	// Recurse if result still contains {{ }}
	if varRe.MatchString(result) && result != s {
		return interpolateRecursive(result, vars, noFacts, depth+1)
	}

	return result
}

// resolveExpr resolves a variable expression like "facts.os_type" or "var | default('x')".
func resolveExpr(expr string, vars map[string]any, noFacts bool) any {
	// Handle filters (e.g., "var | default('x')")
	parts := strings.SplitN(expr, "|", 2)
	varName := strings.TrimSpace(parts[0])

	val := resolveVarPath(varName, vars)

	// Apply filters
	if len(parts) > 1 {
		filter := strings.TrimSpace(parts[1])
		val = applyFilter(val, filter, vars)
	}

	// Handle no-facts sentinel
	if val == nil && noFacts && strings.HasPrefix(varName, "facts.") {
		return factSentinel
	}

	return val
}

// resolveVarPath resolves dotted paths like "facts.os_type".
func resolveVarPath(path string, vars map[string]any) any {
	parts := strings.Split(path, ".")
	var current any = vars

	for _, part := range parts {
		switch m := current.(type) {
		case map[string]any:
			val, ok := m[part]
			if !ok {
				return nil
			}
			current = val
		case map[string]string:
			val, ok := m[part]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil
		}
	}

	return current
}

// applyFilter applies a Jinja-style filter to a value.
func applyFilter(val any, filter string, vars map[string]any) any {
	filter = strings.TrimSpace(filter)

	// default('value') or default("value") or default(varname)
	if strings.HasPrefix(filter, "default(") && strings.HasSuffix(filter, ")") {
		if val != nil {
			return val
		}
		inner := filter[8 : len(filter)-1]
		inner = strings.TrimSpace(inner)
		// String literal
		if (strings.HasPrefix(inner, "'") && strings.HasSuffix(inner, "'")) ||
			(strings.HasPrefix(inner, "\"") && strings.HasSuffix(inner, "\"")) {
			return inner[1 : len(inner)-1]
		}
		// Variable reference
		return resolveVarPath(inner, vars)
	}

	if val == nil {
		return nil
	}

	switch filter {
	case "lower":
		if s, ok := val.(string); ok {
			return strings.ToLower(s)
		}
	case "upper":
		if s, ok := val.(string); ok {
			return strings.ToUpper(s)
		}
	case "string":
		return fmt.Sprintf("%v", val)
	case "length":
		switch v := val.(type) {
		case string:
			return len(v)
		case []any:
			return len(v)
		case map[string]any:
			return len(v)
		}
	}

	return val
}

func envEntries() []string {
	return os.Environ()
}
