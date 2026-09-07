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

// wrapNoLog suppresses stdout/stderr for an entire task's shell by wrapping it
// in a brace group with a single redirect. A brace group (unlike a subshell)
// runs in the current shell, so variable mutations such as the TACK_CHANGED
// counter still take effect, and multi-line constructs (heredocs, if/else)
// stay syntactically valid — a per-line redirect would corrupt them.
func wrapNoLog(shell string) string {
	return "{\n" + strings.TrimRight(shell, "\n") + "\n} >/dev/null 2>&1"
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
		if task.NoLog {
			// Redact all parameter values for no_log tasks; keep keys so the
			// structure remains visible to auditors without leaking secrets.
			redacted := make(map[string]any, len(task.Params))
			for k := range task.Params {
				redacted[k] = "<redacted: no_log>"
			}
			m["params"] = redacted
		} else {
			m["params"] = task.Params
		}
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
