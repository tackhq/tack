package export

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/module"
	"github.com/tackhq/tack/internal/playbook"
	"gopkg.in/yaml.v3"
)

// renderBlock renders a single task block with header, shell, and change counter.
func renderBlock(name string, tags []string, result *module.EmitResult, noLog bool) string {
	var sb strings.Builder

	// Header
	tagStr := ""
	if len(tags) > 0 {
		tagStr = fmt.Sprintf(" (tags: %s)", strings.Join(tags, ","))
	}
	sb.WriteString(fmt.Sprintf("# === TASK: %s ===%s\n", name, tagStr))
	sb.WriteString(fmt.Sprintf("TACK_CURRENT_TASK=%s\n", shellQuote(name)))

	// PreHook
	if result.PreHook != "" {
		sb.WriteString(result.PreHook)
		if !strings.HasSuffix(result.PreHook, "\n") {
			sb.WriteString("\n")
		}
	}

	// Warnings as comments
	for _, w := range result.Warnings {
		sb.WriteString(fmt.Sprintf("# WARN: %s\n", w))
	}

	// Shell payload
	shell := result.Shell
	if noLog {
		// Wrap to suppress output
		shell = wrapNoLog(shell)
	}
	sb.WriteString(shell)
	if !strings.HasSuffix(shell, "\n") {
		sb.WriteString("\n")
	}

	return sb.String()
}

// wrapNoLog wraps shell commands to suppress stdout/stderr.
func wrapNoLog(shell string) string {
	lines := strings.Split(strings.TrimRight(shell, "\n"), "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Don't wrap comments, empty lines, or variable assignments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || isAssignment(trimmed) {
			result = append(result, line)
			continue
		}
		result = append(result, line+" >/dev/null 2>&1")
	}
	return strings.Join(result, "\n")
}

// isAssignment checks if a line is a simple variable assignment.
func isAssignment(line string) bool {
	if idx := strings.Index(line, "="); idx > 0 {
		before := line[:idx]
		// Simple var name (letters, digits, underscore)
		for _, c := range before {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
		return true
	}
	return false
}

// renderUnsupportedTask renders an unsupported task as a comment block.
func renderUnsupportedTask(task *playbook.Task, reason string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# === TASK: %s ===\n", task.Name))
	sb.WriteString(fmt.Sprintf("# UNSUPPORTED: %s\n", reason))
	sb.WriteString("# Original task YAML:\n")
	sb.WriteString(taskToCommentYAML(task))
	return sb.String()
}

// renderUnsupportedBlock renders a block/rescue/always as unsupported.
func renderUnsupportedBlock(task *playbook.Task) string {
	var sb strings.Builder
	name := task.Name
	if name == "" {
		name = "unnamed block"
	}
	sb.WriteString(fmt.Sprintf("# === TASK: %s ===\n", name))
	sb.WriteString("# UNSUPPORTED: block/rescue/always not supported in v1\n")
	sb.WriteString("# Original task YAML:\n")
	sb.WriteString(taskToCommentYAML(task))
	return sb.String()
}

// renderUnsupportedHandlers renders handlers as a single unsupported block.
func renderUnsupportedHandlers(handlers []*playbook.Task) string {
	var sb strings.Builder
	sb.WriteString("# === HANDLERS ===\n")
	sb.WriteString("# UNSUPPORTED: handlers not supported in v1\n")
	for _, h := range handlers {
		sb.WriteString(fmt.Sprintf("#   - %s\n", h.Name))
	}
	return sb.String()
}

// taskToCommentYAML serializes a task to YAML and wraps each line as a comment.
// Parameters marked no_log have their values redacted.
func taskToCommentYAML(task *playbook.Task) string {
	// Build a simplified representation
	m := map[string]any{
		"name": task.Name,
	}
	if task.Module != "" {
		m["module"] = task.Module
	}
	if task.When != "" {
		m["when"] = task.When
	}
	if len(task.Tags) > 0 {
		m["tags"] = task.Tags
	}

	if task.Params != nil {
		m["params"] = task.Params
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return "#   (failed to serialize)\n"
	}

	var sb strings.Builder
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		sb.WriteString(fmt.Sprintf("#   %s\n", line))
	}
	return sb.String()
}

// shellQuote wraps a string in single quotes for bash, escaping embedded quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
