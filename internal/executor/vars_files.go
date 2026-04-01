package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

// loadVarsFile reads a single YAML file and returns its contents as a variable map.
func loadVarsFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var vars map[string]any
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return nil, fmt.Errorf("invalid YAML in %s: %w", path, err)
	}

	return vars, nil
}

// loadVarsFiles loads and merges all vars_files for a play. Files are loaded in
// order; later files override earlier ones. Paths starting with "?" are optional
// (skipped when missing). Variable interpolation in paths uses the provided vars.
func (e *Executor) loadVarsFiles(play *playbook.Play, playbookDir string, vars map[string]any) (map[string]any, error) {
	merged := make(map[string]any)

	for _, rawPath := range play.VarsFiles {
		optional := false
		if strings.HasPrefix(rawPath, "?") {
			optional = true
			rawPath = rawPath[1:]
		}

		// Interpolate variables in the path
		resolved := interpolatePath(rawPath, vars)

		// Resolve relative to playbook directory
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(playbookDir, resolved)
		}

		fileVars, err := loadVarsFile(resolved)
		if err != nil {
			if optional && os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("vars_files %q: %w", rawPath, err)
		}

		// Merge: later files override earlier
		for k, v := range fileVars {
			merged[k] = v
		}
	}

	return merged, nil
}

// interpolatePath replaces {{ var }} patterns in a file path with values from vars.
func interpolatePath(path string, vars map[string]any) string {
	return varPattern.ReplaceAllStringFunc(path, func(match string) string {
		inner := varPattern.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		key := strings.TrimSpace(inner[1])
		if val, ok := vars[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match
	})
}
