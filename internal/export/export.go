// Package export compiles playbooks into standalone bash scripts.
package export

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
	"github.com/tackhq/tack/internal/playbook"
	"github.com/tackhq/tack/pkg/facts"
)

// Options controls export behavior.
type Options struct {
	// Host is the target host name (single-host mode).
	Host string

	// AllHosts emits one script per host in the inventory.
	AllHosts bool

	// Output is the file path (single-host) or directory (all-hosts).
	// Empty means stdout (single-host only).
	Output string

	// NoFacts skips fact gathering; fact references become sentinels.
	NoFacts bool

	// CheckOnly compiles but does not write output; prints a summary.
	CheckOnly bool

	// NoBannerTimestamp omits the timestamp line for reproducible output.
	NoBannerTimestamp bool

	// Tags filters to only include tasks matching these tags.
	Tags []string

	// SkipTags excludes tasks matching these tags.
	SkipTags []string

	// ExtraVars are additional variables (highest precedence).
	ExtraVars map[string]string

	// Version is the tack version string for the banner.
	Version string

	// PlaybookPath is the original path for the banner.
	PlaybookPath string

	// Connection type for fact gathering (local, ssh, etc).
	Connection string

	// Timestamp is the frozen export time (for determinism).
	Timestamp string
}

// Compiler compiles a playbook into bash scripts.
type Compiler struct {
	Playbook    *playbook.Playbook
	Opts        Options
	Roles       []*playbook.Role
	PlaybookDir string

	// VaultUsed tracks whether any vault-decrypted values were resolved.
	VaultUsed bool

	// vars holds the merged variable context for the current host.
	vars map[string]any

	// facts holds frozen facts for the current host.
	facts map[string]any

	// registered tracks registered variable names (runtime dependencies).
	registered map[string]bool
}

// CompileResult holds the result for a single host.
type CompileResult struct {
	Host       string
	Script     string
	Supported  []TaskSummary
	Unsupported []TaskSummary
	Warnings   []string
}

// TaskSummary describes a task for the check-only report.
type TaskSummary struct {
	Name   string
	Module string
	Reason string // non-empty for unsupported
	Tags   []string
}

// Compile compiles the playbook for a single host and returns the script.
func (c *Compiler) Compile(ctx context.Context, play *playbook.Play, host string, conn connector.Connector) (*CompileResult, error) {
	result := &CompileResult{Host: host}

	// Initialize state
	c.registered = make(map[string]bool)
	c.VaultUsed = false

	// Build variable context
	if err := c.buildVars(play, host); err != nil {
		return nil, fmt.Errorf("building variables: %w", err)
	}

	// Gather or skip facts
	if err := c.gatherFacts(ctx, play, conn); err != nil {
		return nil, fmt.Errorf("gathering facts: %w", err)
	}

	// Expand role tasks
	allTasks := playbook.ExpandRoleTasks(c.Roles, play.Tasks)

	// Compile tasks
	var blocks []string
	var skipped []string

	for _, task := range allTasks {
		taskBlocks, taskSkipped, err := c.compileTask(task, play.Tags, nil, result)
		if err != nil {
			return nil, fmt.Errorf("task %q: %w", task.Name, err)
		}
		blocks = append(blocks, taskBlocks...)
		skipped = append(skipped, taskSkipped...)
	}

	// Handle handlers as unsupported
	allHandlers := playbook.ExpandRoleHandlers(c.Roles, play.Handlers)
	if len(allHandlers) > 0 {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   "handlers",
			Module: "handler",
			Reason: "handlers not supported in v1",
		})
		blocks = append(blocks, renderUnsupportedHandlers(allHandlers))
	}

	// Build script
	banner := c.renderBanner(host)
	script := banner + "\n"
	for _, s := range skipped {
		script += s + "\n"
	}
	for _, b := range blocks {
		script += "\n" + b + "\n"
	}

	result.Script = script
	return result, nil
}

// compileTask compiles a single task (handling loops, when, tags, blocks).
func (c *Compiler) compileTask(task *playbook.Task, playTags, blockTags []string, result *CompileResult) (blocks []string, skipped []string, err error) {
	// Handle block/rescue/always — unsupported in v1
	if len(task.Block) > 0 {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: "block",
			Reason: "block/rescue/always not supported in v1",
		})
		blocks = append(blocks, renderUnsupportedBlock(task))
		return blocks, skipped, nil
	}

	// Handle include_tasks
	if task.Include != "" {
		return c.compileInclude(task, playTags, blockTags, result)
	}

	// Expand shorthand
	playbook.ExpandShorthand(task)

	// Tag filtering
	eTags := effectiveTags(task, playTags, blockTags)
	if !shouldRunTask(eTags, c.Opts.Tags, c.Opts.SkipTags) {
		return nil, nil, nil
	}

	// Sort tags for deterministic output
	sortedTags := make([]string, len(eTags))
	copy(sortedTags, eTags)
	sort.Strings(sortedTags)

	// Evaluate when condition
	if task.When != "" {
		skip, warn, err := c.evaluateWhen(task.When)
		if err != nil {
			return nil, nil, fmt.Errorf("when condition: %w", err)
		}
		if skip {
			skipped = append(skipped, fmt.Sprintf("# SKIPPED (when false): %s", task.When))
			return nil, skipped, nil
		}
		if warn != "" {
			// Runtime variable reference — emit unconditionally with warning
			block := c.emitTaskBlock(task, sortedTags, result)
			block = fmt.Sprintf("# WARN: %s\n%s", warn, block)
			blocks = append(blocks, block)
			return blocks, skipped, nil
		}
	}

	// Handle loops
	if len(task.Loop) > 0 || task.LoopExpr != "" {
		return c.compileLoop(task, sortedTags, result)
	}

	// Track registered variables
	if task.Register != "" {
		c.registered[task.Register] = true
	}

	// Emit the task
	block := c.emitTaskBlock(task, sortedTags, result)
	blocks = append(blocks, block)
	return blocks, skipped, nil
}

// emitTaskBlock emits a single task's bash block.
func (c *Compiler) emitTaskBlock(task *playbook.Task, sortedTags []string, result *CompileResult) string {
	// Resolve module
	mod := module.Get(task.Module)
	if mod == nil {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: task.Module,
			Reason: fmt.Sprintf("unknown module %q", task.Module),
		})
		return renderUnsupportedTask(task, fmt.Sprintf("unknown module %q", task.Module))
	}

	// Check if module implements Emitter
	emitter, ok := mod.(module.Emitter)
	if !ok {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: task.Module,
			Reason: "module does not support export",
		})
		return renderUnsupportedTask(task, "module does not support export")
	}

	// Interpolate params
	params := c.interpolateParams(task.Params)

	// Call Emit
	emitResult, err := emitter.Emit(params, c.vars)
	if err != nil {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: task.Module,
			Reason: fmt.Sprintf("emit error: %v", err),
		})
		return renderUnsupportedTask(task, fmt.Sprintf("emit error: %v", err))
	}

	if !emitResult.Supported {
		result.Unsupported = append(result.Unsupported, TaskSummary{
			Name:   task.Name,
			Module: task.Module,
			Reason: emitResult.Reason,
		})
		return renderUnsupportedTask(task, emitResult.Reason)
	}

	result.Supported = append(result.Supported, TaskSummary{
		Name:   task.Name,
		Module: task.Module,
		Tags:   sortedTags,
	})

	for _, w := range emitResult.Warnings {
		result.Warnings = append(result.Warnings, fmt.Sprintf("task %q: %s", task.Name, w))
	}

	return renderBlock(task.Name, sortedTags, emitResult, false)
}

// buildVars builds the variable context for a host.
func (c *Compiler) buildVars(play *playbook.Play, host string) error {
	c.vars = make(map[string]any)

	// Role defaults < role vars < play vars
	c.vars = playbook.MergeRoleVars(c.Roles, play.Vars)

	// Extra vars (highest precedence)
	for k, v := range c.Opts.ExtraVars {
		c.vars[k] = v
	}

	// Add env
	c.vars["env"] = getEnvMap()

	return nil
}

// gatherFacts gathers facts from the target or uses sentinels.
func (c *Compiler) gatherFacts(ctx context.Context, play *playbook.Play, conn connector.Connector) error {
	c.facts = make(map[string]any)

	if c.Opts.NoFacts || !play.ShouldGatherFacts() {
		// No facts — sentinel values will be substituted during interpolation
		c.vars["facts"] = c.facts
		return nil
	}

	if conn == nil {
		c.vars["facts"] = c.facts
		return nil
	}

	// Connect and gather
	if err := conn.Connect(ctx); err != nil {
		return fmt.Errorf("connecting for fact gathering: %w", err)
	}
	defer conn.Close()

	f, err := facts.Gather(ctx, conn)
	if err != nil {
		return fmt.Errorf("gathering facts: %w", err)
	}
	c.facts = f
	c.vars["facts"] = f
	return nil
}

// evaluateWhen evaluates a when condition.
// Returns (skip bool, warning string, error).
// skip=true means the condition resolved to false.
// A non-empty warning means a runtime variable reference was found.
func (c *Compiler) evaluateWhen(condition string) (bool, string, error) {
	// Check for registered variable references
	if c.referencesRegistered(condition) {
		return false, fmt.Sprintf("when references runtime variable, included unconditionally: %s", condition), nil
	}

	// Try to evaluate
	result, err := c.evalCondition(condition)
	if err != nil {
		// If the error is due to undefined variable, it might be a runtime dep
		if strings.Contains(err.Error(), "undefined") {
			return false, fmt.Sprintf("when references undefined variable, included unconditionally: %s", condition), nil
		}
		return false, "", fmt.Errorf("evaluating %q: %w", condition, err)
	}

	if !result {
		return true, "", nil
	}
	return false, "", nil
}

// referencesRegistered checks if a condition string references any registered variable.
func (c *Compiler) referencesRegistered(condition string) bool {
	for name := range c.registered {
		if strings.Contains(condition, name) {
			return true
		}
	}
	return false
}

// interpolateParams interpolates variables in task parameters.
func (c *Compiler) interpolateParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range params {
		result[k] = c.interpolateValue(v)
	}
	return result
}

// interpolateValue interpolates a single value.
func (c *Compiler) interpolateValue(v any) any {
	switch val := v.(type) {
	case string:
		return c.interpolateString(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = c.interpolateValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any)
		for k, item := range val {
			result[k] = c.interpolateValue(item)
		}
		return result
	default:
		return v
	}
}

func (c *Compiler) interpolateString(s string) any {
	return interpolateWithVars(s, c.vars, c.Opts.NoFacts)
}

func getEnvMap() map[string]string {
	env := make(map[string]string)
	for _, e := range sortedEnv() {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}
	return env
}

func sortedEnv() []string {
	env := append([]string{}, envEntries()...)
	sort.Strings(env)
	return env
}
