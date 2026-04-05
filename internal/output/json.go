package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tackhq/tack/internal/playbook"
)

// JSONEmitter emits newline-delimited JSON events to stdout.
// Errors are written to stderr. The approval prompt is auto-approved.
type JSONEmitter struct {
	w      io.Writer
	errW   io.Writer
	debug  bool
	diff   bool
}

// NewJSONEmitter creates a JSONEmitter writing events to w and errors to errW.
func NewJSONEmitter(w io.Writer, errW io.Writer) *JSONEmitter {
	return &JSONEmitter{w: w, errW: errW}
}

// Verify JSONEmitter implements Emitter at compile time.
var _ Emitter = (*JSONEmitter)(nil)

func (j *JSONEmitter) emit(event map[string]any) {
	event["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(j.errW, "json marshal error: %v\n", err)
		return
	}
	fmt.Fprintf(j.w, "%s\n", data)
}

// PlaybookStart emits a playbook_start event.
func (j *JSONEmitter) PlaybookStart(path string) {
	j.emit(map[string]any{
		"type":     "playbook_start",
		"playbook": path,
		"version":  1,
	})
}

// PlaybookEnd emits a playbook_recap event.
func (j *JSONEmitter) PlaybookEnd(stats Stats) {
	j.emit(map[string]any{
		"type":     "playbook_recap",
		"ok":       stats.GetOK(),
		"changed":  stats.GetChanged(),
		"failed":   stats.GetFailed(),
		"skipped":  stats.GetSkipped(),
		"duration": stats.GetDuration().Seconds(),
		"success":  stats.GetFailed() == 0,
	})
}

// PlayStart emits a play_start event.
func (j *JSONEmitter) PlayStart(play *playbook.Play) {
	name := play.Name
	if name == "" {
		name = strings.Join(play.Hosts, ", ")
	}
	j.emit(map[string]any{
		"type":  "play_start",
		"play":  name,
		"hosts": play.Hosts,
	})
}

// HostStart emits a host_start event.
func (j *JSONEmitter) HostStart(host, connType string) {
	j.emit(map[string]any{
		"type":       "host_start",
		"host":       host,
		"connection": connType,
	})
}

// TaskStart emits a task_start event.
func (j *JSONEmitter) TaskStart(name, moduleName string) {
	j.emit(map[string]any{
		"type":   "task_start",
		"task":   name,
		"module": moduleName,
	})
}

// TaskResult emits a task_result event.
func (j *JSONEmitter) TaskResult(name, status string, changed bool, message string) {
	j.emit(map[string]any{
		"type":    "task_result",
		"task":    name,
		"status":  status,
		"changed": changed,
		"message": message,
	})
}

// DisplayPlan emits plan_task events for each planned task.
func (j *JSONEmitter) DisplayPlan(tasks []PlannedTask, dryRun bool) {
	for _, t := range tasks {
		event := map[string]any{
			"type":   "plan_task",
			"task":   t.Name,
			"module": t.Module,
			"action": t.Status,
		}
		if len(t.Params) > 0 {
			event["params"] = t.Params
		}
		if t.Reason != "" {
			event["reason"] = t.Reason
		}
		if t.OldChecksum != "" {
			event["old_checksum"] = t.OldChecksum
		}
		if t.NewChecksum != "" {
			event["new_checksum"] = t.NewChecksum
		}
		if j.diff {
			if t.OldContent != "" {
				event["old_content"] = t.OldContent
			}
			if t.NewContent != "" {
				event["new_content"] = t.NewContent
			}
		}
		j.emit(event)
	}
}

// PromptApproval always returns true in JSON mode (auto-approve).
func (j *JSONEmitter) PromptApproval() bool {
	return true
}

// Section is a no-op in JSON mode.
func (j *JSONEmitter) Section(_ string) {}

// Info emits an info event.
func (j *JSONEmitter) Info(format string, args ...any) {
	j.emit(map[string]any{
		"type":    "info",
		"message": fmt.Sprintf(format, args...),
	})
}

// Warn emits a warning event.
func (j *JSONEmitter) Warn(format string, args ...any) {
	j.emit(map[string]any{
		"type":    "warning",
		"message": fmt.Sprintf(format, args...),
	})
}

// Error emits an error event to stdout and writes the message to stderr.
func (j *JSONEmitter) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	j.emit(map[string]any{
		"type":    "error",
		"message": msg,
	})
	fmt.Fprintf(j.errW, "error: %s\n", msg)
}

// Debug emits a debug event (only when debug mode is enabled).
func (j *JSONEmitter) Debug(format string, args ...any) {
	if j.debug {
		j.emit(map[string]any{
			"type":    "debug",
			"message": fmt.Sprintf(format, args...),
		})
	}
}

// SetColor is a no-op for JSON output.
func (j *JSONEmitter) SetColor(_ bool) {}

// SetDebug enables or disables debug events.
func (j *JSONEmitter) SetDebug(enabled bool) {
	j.debug = enabled
}

// SetVerbose is a no-op for JSON output.
func (j *JSONEmitter) SetVerbose(_ bool) {}

// SetDiff enables diff data in JSON plan events.
func (j *JSONEmitter) SetDiff(enabled bool) {
	j.diff = enabled
}

// DiffEnabled returns whether diff mode is active.
func (j *JSONEmitter) DiffEnabled() bool {
	return j.diff
}

// NewEmitter creates the appropriate Emitter based on the output mode.
func NewEmitter(mode string) (Emitter, error) {
	switch mode {
	case "text", "":
		return New(os.Stdout), nil
	case "json":
		return NewJSONEmitter(os.Stdout, os.Stderr), nil
	default:
		return nil, fmt.Errorf("invalid output mode %q (valid: text, json)", mode)
	}
}
