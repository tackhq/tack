package output

import (
	"github.com/tackhq/tack/internal/playbook"
)

// Emitter is the interface for output backends (text, JSON, etc.).
type Emitter interface {
	PlaybookStart(path string)
	PlaybookEnd(stats Stats)
	PlayStart(play *playbook.Play)
	HostStart(host, connType string)
	TaskStart(name, moduleName string)
	TaskResult(name, status string, changed bool, message string)
	DisplayPlan(tasks []PlannedTask, dryRun bool)
	PromptApproval() bool
	Section(name string)
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	Debug(format string, args ...any)
	SetColor(enabled bool)
	SetDebug(enabled bool)
	SetVerbose(enabled bool)
	SetDiff(enabled bool)
	DiffEnabled() bool
}

// Verify Output implements Emitter at compile time.
var _ Emitter = (*Output)(nil)
