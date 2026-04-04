package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

// evaluateAssertsForDryRun walks the task tree and evaluates any assert
// tasks, returning an error if any fail. This preserves fail-fast semantics
// under --dry-run where the apply phase never runs.
func (e *Executor) evaluateAssertsForDryRun(pctx *PlayContext, tasks []*playbook.Task) error {
	for _, task := range tasks {
		// Honor when: gate the same way the normal dispatcher does.
		if task.When != "" {
			ok, err := e.evaluateCondition(task.When, pctx)
			if err != nil || !ok {
				continue
			}
		}
		if task.IsBlock() {
			if err := e.evaluateAssertsForDryRun(pctx, task.Block); err != nil {
				// Block failure — if there's a rescue, recover.
				if len(task.Rescue) > 0 {
					continue
				}
				return err
			}
			if err := e.evaluateAssertsForDryRun(pctx, task.Always); err != nil {
				return err
			}
			continue
		}
		if !task.IsAssert() {
			continue
		}
		spec := task.Assert
		if spec == nil || len(spec.That) == 0 {
			return fmt.Errorf("assert: 'that' is required and must contain at least one condition")
		}
		outcome := evaluateAssertSpec(spec, pctx)
		if task.Register != "" {
			pctx.Registered[task.Register] = map[string]any{
				"changed":              false,
				"failed":               outcome.failed,
				"msg":                  outcome.msg,
				"evaluated_conditions": outcome.conditionsPayload(),
			}
			pctx.Vars[task.Register] = pctx.Registered[task.Register]
		}
		if outcome.failed && !task.IgnoreErrors {
			pctx.Output.Error("Assertion failed: %s", outcome.msg)
			return fmt.Errorf("%s", outcome.msg)
		}
	}
	return nil
}

// assertTaskName returns the display name for an assert task.
func assertTaskName(task *playbook.Task) string {
	if task.Name != "" {
		return task.Name
	}
	return "assert"
}

// assertEval holds one evaluated expression and its boolean result.
type assertEval struct {
	expr   string
	result bool
}

// assertOutcome captures the result of evaluating an assert spec.
type assertOutcome struct {
	failed     bool
	msg        string
	conditions []assertEval
}

// evaluateAssertSpec evaluates every condition in spec against pctx and
// returns a structured outcome (no side effects other than reading pctx).
func evaluateAssertSpec(spec *playbook.AssertSpec, pctx *PlayContext) assertOutcome {
	outcome := assertOutcome{conditions: make([]assertEval, 0, len(spec.That))}
	var failedExprs []string
	var firstParseErr error

	for _, expr := range spec.That {
		ok, err := evaluateConditionExpr(expr, pctx)
		if err != nil {
			if firstParseErr == nil {
				firstParseErr = err
			}
			outcome.conditions = append(outcome.conditions, assertEval{expr: expr, result: false})
			failedExprs = append(failedExprs, expr)
			continue
		}
		outcome.conditions = append(outcome.conditions, assertEval{expr: expr, result: ok})
		if !ok {
			failedExprs = append(failedExprs, expr)
		}
	}

	outcome.failed = len(failedExprs) > 0 || firstParseErr != nil
	if outcome.failed {
		if firstParseErr != nil {
			outcome.msg = fmt.Sprintf("Assertion failed: %v", firstParseErr)
		} else if spec.FailMsg != "" {
			outcome.msg = spec.FailMsg
		} else {
			outcome.msg = "Assertion failed:\n" + strings.Join(failedExprs, "\n")
		}
	} else {
		if spec.SuccessMsg != "" {
			outcome.msg = spec.SuccessMsg
		} else if spec.Quiet {
			outcome.msg = "OK"
		} else {
			lines := make([]string, 0, len(outcome.conditions))
			for _, r := range outcome.conditions {
				lines = append(lines, r.expr+" => true")
			}
			outcome.msg = "All assertions passed:\n" + strings.Join(lines, "\n")
		}
	}
	return outcome
}

// conditionsPayload converts evaluated conditions into the slice-of-maps
// shape used for the registered result.
func (o assertOutcome) conditionsPayload() []map[string]any {
	payload := make([]map[string]any, 0, len(o.conditions))
	for _, r := range o.conditions {
		payload = append(payload, map[string]any{
			"expr":   r.expr,
			"result": r.result,
		})
	}
	return payload
}

// executeAssert evaluates an assert task's conditions against the play
// context. It never invokes the connector. A single false condition fails
// the task, which lets block/rescue/always semantics apply normally.
func (e *Executor) executeAssert(ctx context.Context, pctx *PlayContext, task *playbook.Task) (*TaskResult, error) {
	_ = ctx
	taskName := assertTaskName(task)
	pctx.Output.TaskStart(taskName, "assert")

	spec := task.Assert
	if spec == nil || len(spec.That) == 0 {
		err := fmt.Errorf("assert: 'that' is required and must contain at least one condition")
		pctx.Output.TaskResult(taskName, "failed", false, err.Error())
		return &TaskResult{Status: "failed", Error: err}, err
	}

	outcome := evaluateAssertSpec(spec, pctx)

	// Register result map if requested.
	if task.Register != "" {
		registered := map[string]any{
			"changed":              false,
			"failed":               outcome.failed,
			"msg":                  outcome.msg,
			"evaluated_conditions": outcome.conditionsPayload(),
		}
		pctx.Registered[task.Register] = registered
		pctx.Vars[task.Register] = registered
	}

	if outcome.failed {
		pctx.Output.TaskResult(taskName, "failed", false, outcome.msg)
		// Also print message so users can see why it failed.
		pctx.Output.Info("%s", outcome.msg)
		err := fmt.Errorf("%s", outcome.msg)
		return &TaskResult{Status: "failed", Error: err}, err
	}

	pctx.Output.TaskResult(taskName, "ok", false, outcome.msg)
	if !spec.Quiet {
		pctx.Output.Info("%s", outcome.msg)
	} else if spec.SuccessMsg != "" {
		pctx.Output.Info("%s", outcome.msg)
	}
	return &TaskResult{Status: "ok", Changed: false}, nil
}
