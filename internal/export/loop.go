package export

import (
	"fmt"

	"github.com/tackhq/tack/internal/playbook"
)

// compileLoop handles loop expansion at export time.
func (c *Compiler) compileLoop(task *playbook.Task, sortedTags []string, result *CompileResult) (blocks []string, skipped []string, err error) {
	loopVar := task.GetLoopVar()
	items, err := c.resolveLoopItems(task)
	if err != nil {
		// Runtime dependency — unsupported
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: task.Module,
			Reason: fmt.Sprintf("loop with runtime dependency: %v", err),
		})
		blocks = append(blocks, renderUnsupportedTask(task, fmt.Sprintf("loop with runtime dependency: %v", err)))
		return blocks, skipped, nil
	}

	// Unroll loop — one block per item
	for i, item := range items {
		// Set loop variable in context
		oldVal, hadOld := c.vars[loopVar]
		oldIdx, hadIdx := c.vars["loop_index"]
		c.vars[loopVar] = item
		c.vars["loop_index"] = i

		block := c.emitTaskBlock(task, sortedTags, result)
		blocks = append(blocks, block)

		// Restore
		if hadOld {
			c.vars[loopVar] = oldVal
		} else {
			delete(c.vars, loopVar)
		}
		if hadIdx {
			c.vars["loop_index"] = oldIdx
		} else {
			delete(c.vars, "loop_index")
		}
	}

	return blocks, skipped, nil
}

// resolveLoopItems resolves the loop items, either from a static list or a variable.
func (c *Compiler) resolveLoopItems(task *playbook.Task) ([]any, error) {
	// Static list
	if len(task.Loop) > 0 {
		return task.Loop, nil
	}

	// Expression-based loop (e.g., "{{ packages }}")
	if task.LoopExpr != "" {
		resolved := c.interpolateString(task.LoopExpr)
		switch v := resolved.(type) {
		case []any:
			return v, nil
		case string:
			// If still a string template, it wasn't resolved
			if v == task.LoopExpr {
				return nil, fmt.Errorf("cannot resolve loop expression %q", task.LoopExpr)
			}
			// Single value
			return []any{v}, nil
		default:
			return nil, fmt.Errorf("loop expression %q resolved to non-list type %T", task.LoopExpr, resolved)
		}
	}

	return nil, fmt.Errorf("no loop items found")
}
