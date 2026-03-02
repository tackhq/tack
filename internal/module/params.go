package module

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetString extracts a string parameter with a default value.
func GetString(params map[string]any, key, defaultValue string) string {
	v, ok := params[key]
	if !ok {
		return defaultValue
	}
	s, ok := v.(string)
	if !ok {
		return defaultValue
	}
	return s
}

// GetBool extracts a bool parameter with a default value.
func GetBool(params map[string]any, key string, defaultValue bool) bool {
	v, ok := params[key]
	if !ok {
		return defaultValue
	}
	b, ok := v.(bool)
	if !ok {
		return defaultValue
	}
	return b
}

// GetInt extracts an int parameter with a default value.
func GetInt(params map[string]any, key string, defaultValue int) int {
	v, ok := params[key]
	if !ok {
		return defaultValue
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return defaultValue
}

// RequireString extracts a required string parameter.
func RequireString(params map[string]any, key string) (string, error) {
	v, ok := params[key]
	if !ok {
		return "", fmt.Errorf("required parameter '%s' is missing", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("parameter '%s' must be a string", key)
	}
	if s == "" {
		return "", fmt.Errorf("parameter '%s' cannot be empty", key)
	}
	return s, nil
}

// GetStringSlice extracts a string slice parameter.
// Handles single strings, []any, and []string values.
func GetStringSlice(params map[string]any, key string) []string {
	v, ok := params[key]
	if !ok {
		return nil
	}

	// Single string
	if s, ok := v.(string); ok {
		if s == "" {
			return nil
		}
		return []string{s}
	}

	if slice, ok := v.([]any); ok {
		var result []string
		for _, item := range slice {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	}

	if slice, ok := v.([]string); ok {
		return slice
	}

	return nil
}

// ResolveRolePath resolves a relative source path against a role's subdirectory.
// If src is absolute, it is returned as-is. If a _role_path is set in params and the
// candidate file exists under rolePath/subdir/src, that path is returned. Otherwise src is returned unchanged.
func ResolveRolePath(src string, params map[string]any, subdir string) string {
	if filepath.IsAbs(src) {
		return src
	}
	if rolePath := GetString(params, "_role_path", ""); rolePath != "" {
		candidate := filepath.Join(rolePath, subdir, src)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return src
}

// GetMap extracts a map parameter with a default empty map.
func GetMap(params map[string]any, key string) map[string]any {
	v, ok := params[key]
	if !ok {
		return make(map[string]any)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return make(map[string]any)
	}
	return m
}
