package playbook

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/eugenetaranov/bolt/internal/module"
)

// knownTaskFields are fields that are task directives, not module names.
var knownTaskFields = map[string]bool{
	"name":         true,
	"when":         true,
	"register":     true,
	"notify":       true,
	"loop":         true,
	"with_items":   true,
	"loop_var":     true,
	"ignore_errors": true,
	"retries":      true,
	"delay":        true,
	"become":       true,
	"become_user":  true,
	"changed_when": true,
	"failed_when":  true,
	"include":      true,
}

// ParseFileRaw parses a playbook with proper module detection.
func ParseFileRaw(path string) (*Playbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook: %w", err)
	}

	return ParseRaw(data, path)
}

// ParseRaw parses a playbook with proper module detection.
func ParseRaw(data []byte, path string) (*Playbook, error) {
	// First, try to unmarshal as a list of raw play maps
	var rawPlays []map[string]any
	if err := yaml.Unmarshal(data, &rawPlays); err != nil {
		// Try as single play
		var rawPlay map[string]any
		if err := yaml.Unmarshal(data, &rawPlay); err != nil {
			return nil, fmt.Errorf("invalid playbook format: %w", err)
		}
		rawPlays = []map[string]any{rawPlay}
	}

	playbook := &Playbook{Path: path}

	for i, rawPlay := range rawPlays {
		play, err := parseRawPlay(rawPlay)
		if err != nil {
			return nil, fmt.Errorf("play %d: %w", i+1, err)
		}
		if err := play.Validate(); err != nil {
			return nil, fmt.Errorf("play %d: %w", i+1, err)
		}
		playbook.Plays = append(playbook.Plays, play)
	}

	return playbook, nil
}

// parseRawPlay parses a single play from a raw map.
func parseRawPlay(raw map[string]any) (*Play, error) {
	play := &Play{
		Vars: make(map[string]any),
	}

	// Parse simple fields
	if v, ok := raw["name"].(string); ok {
		play.Name = v
	}
	switch v := raw["hosts"].(type) {
	case string:
		play.Hosts = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				play.Hosts = append(play.Hosts, s)
			}
		}
	}
	if v, ok := raw["connection"].(string); ok {
		play.Connection = v
	}
	if v, ok := raw["become"].(bool); ok {
		play.Become = v
	}
	if v, ok := raw["become_user"].(string); ok {
		play.BecomeUser = v
	}
	if v, ok := raw["gather_facts"].(bool); ok {
		play.GatherFacts = &v
	}

	// Parse vars
	if vars, ok := raw["vars"].(map[string]any); ok {
		play.Vars = vars
	}

	// Parse roles
	if roles, ok := raw["roles"].([]any); ok {
		for _, role := range roles {
			if roleName, ok := role.(string); ok {
				play.Roles = append(play.Roles, roleName)
			}
		}
	}

	// Parse tasks
	if items, ok := raw["tasks"].([]any); ok {
		tasks, err := parseTaskList(items, "task")
		if err != nil {
			return nil, err
		}
		play.Tasks = tasks
	}

	// Parse handlers
	if items, ok := raw["handlers"].([]any); ok {
		handlers, err := parseTaskList(items, "handler")
		if err != nil {
			return nil, err
		}
		play.Handlers = handlers
	}

	return play, nil
}

// parseTaskList parses a list of raw task/handler maps into Task structs.
func parseTaskList(items []any, itemType string) ([]*Task, error) {
	var tasks []*Task
	for i, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s %d: invalid %s format", itemType, i+1, itemType)
		}
		task, err := parseRawTask(m)
		if err != nil {
			return nil, fmt.Errorf("%s %d: %w", itemType, i+1, err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// parseRawTask parses a single task from a raw map.
func parseRawTask(raw map[string]any) (*Task, error) {
	task := &Task{
		Params: make(map[string]any),
	}

	// Parse known task fields
	if v, ok := raw["name"].(string); ok {
		task.Name = v
	}
	if v, ok := raw["when"].(string); ok {
		task.When = v
	}
	if v, ok := raw["register"].(string); ok {
		task.Register = v
	}
	if v, ok := raw["loop_var"].(string); ok {
		task.LoopVar = v
	}
	if v, ok := raw["ignore_errors"].(bool); ok {
		task.IgnoreErrors = v
	}
	if v, ok := raw["retries"].(int); ok {
		task.Retries = v
	}
	if v, ok := raw["delay"].(int); ok {
		task.Delay = v
	}
	if v, ok := raw["become"].(bool); ok {
		task.Become = &v
	}
	if v, ok := raw["become_user"].(string); ok {
		task.BecomeUser = v
	}
	if v, ok := raw["changed_when"].(string); ok {
		task.ChangedWhen = v
	}
	if v, ok := raw["failed_when"].(string); ok {
		task.FailedWhen = v
	}
	if v, ok := raw["include"].(string); ok {
		task.Include = v
	}

	// Parse notify (can be string or list)
	if notify, ok := raw["notify"]; ok {
		switch n := notify.(type) {
		case string:
			task.Notify = []string{n}
		case []any:
			for _, item := range n {
				if s, ok := item.(string); ok {
					task.Notify = append(task.Notify, s)
				}
			}
		}
	}

	// Parse loop (can be "loop" or "with_items")
	if loop, ok := raw["loop"]; ok {
		if items, ok := loop.([]any); ok {
			task.Loop = items
		}
	} else if loop, ok := raw["with_items"]; ok {
		if items, ok := loop.([]any); ok {
			task.Loop = items
		}
	}

	// Find the module - it's a key that's not a known task field
	for key, value := range raw {
		if knownTaskFields[key] {
			continue
		}

		// This must be the module name
		if task.Module != "" {
			return nil, fmt.Errorf("multiple modules specified: %s and %s", task.Module, key)
		}

		task.Module = key

		// Parse module parameters
		switch params := value.(type) {
		case map[string]any:
			task.Params = params
		case string:
			// Short form: module: "arg"
			// Convert to params with special key
			task.Params = map[string]any{"_raw": params}
		case nil:
			// Module with no parameters
			task.Params = make(map[string]any)
		default:
			task.Params = map[string]any{"_raw": value}
		}
	}

	return task, nil
}

// ExpandShorthand expands shorthand module syntax.
// For example, "apt: name=nginx state=present" becomes proper params.
func ExpandShorthand(task *Task) {
	raw, ok := task.Params["_raw"].(string)
	if !ok {
		return
	}

	// Check if it's key=value format
	if !strings.Contains(raw, "=") {
		// Single argument - module-specific handling
		switch task.Module {
		case "command", "shell":
			task.Params = map[string]any{"cmd": raw}
		case "file":
			task.Params = map[string]any{"path": raw}
		case "copy":
			task.Params = map[string]any{"dest": raw}
		default:
			task.Params = map[string]any{"name": raw}
		}
		return
	}

	// Parse key=value pairs
	newParams := make(map[string]any)
	parts := strings.Fields(raw)
	for _, part := range parts {
		if idx := strings.Index(part, "="); idx > 0 {
			key := part[:idx]
			value := part[idx+1:]
			// Handle quoted values
			value = strings.Trim(value, "\"'")
			newParams[key] = value
		}
	}

	task.Params = newParams
}

// ResolveModule checks if the task's module exists in the registry.
func ResolveModule(task *Task) error {
	if task.Module == "" {
		return fmt.Errorf("no module specified")
	}

	m := module.Get(task.Module)
	if m == nil {
		available := module.List()
		return fmt.Errorf("unknown module '%s' (available: %s)",
			task.Module, strings.Join(available, ", "))
	}

	return nil
}
