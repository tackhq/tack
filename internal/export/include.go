package export

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tackhq/tack/internal/playbook"
)

const maxIncludeDepth = 64

// compileInclude handles include_tasks at export time.
func (c *Compiler) compileInclude(task *playbook.Task, playTags, blockTags []string, result *CompileResult) (blocks []string, skipped []string, err error) {
	return c.compileIncludeAtDepth(task, playTags, blockTags, result, 0)
}

func (c *Compiler) compileIncludeAtDepth(task *playbook.Task, playTags, blockTags []string, result *CompileResult, depth int) (blocks []string, skipped []string, err error) {
	if depth >= maxIncludeDepth {
		return nil, nil, fmt.Errorf("circular include detected (depth %d): %s", depth, task.Include)
	}

	includePath := task.Include

	// Check for dynamic include (variable interpolation in path)
	if strings.Contains(includePath, "{{") {
		resolved := fmt.Sprintf("%v", c.interpolateString(includePath))
		if strings.Contains(resolved, "{{") {
			// Still has unresolved variables — unsupported
			result.Unsupported = append(result.Unsupported, TaskSummary{
				Name:   task.Name,
				Module: "include_tasks",
				Reason: "dynamic include with unresolvable path",
			})
			blocks = append(blocks, renderUnsupportedTask(task, "dynamic include with unresolvable path"))
			return blocks, skipped, nil
		}
		includePath = resolved
	}

	// Check for loop on include — unsupported
	if len(task.Loop) > 0 || task.LoopExpr != "" {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: "include_tasks",
			Reason: "include_tasks with loop",
		})
		blocks = append(blocks, renderUnsupportedTask(task, "include_tasks with loop"))
		return blocks, skipped, nil
	}

	// Resolve path relative to playbook directory
	if !filepath.IsAbs(includePath) {
		// Check if it's a role-relative path
		if task.RolePath != "" {
			includePath = filepath.Join(task.RolePath, "tasks", includePath)
		} else {
			includePath = filepath.Join(c.PlaybookDir, includePath)
		}
	}

	// Load included tasks
	tasks, err := playbook.LoadTasksFile(includePath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading include %q: %w", includePath, err)
	}

	// Apply include vars
	if len(task.IncludeVars) > 0 {
		for k, v := range task.IncludeVars {
			c.vars[k] = v
		}
	}

	// Compile included tasks
	for _, t := range tasks {
		tb, ts, err := c.compileTask(t, playTags, blockTags, result)
		if err != nil {
			return nil, nil, fmt.Errorf("in include %q: %w", task.Include, err)
		}
		blocks = append(blocks, tb...)
		skipped = append(skipped, ts...)
	}

	return blocks, skipped, nil
}
