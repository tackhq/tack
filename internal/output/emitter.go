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
	// DisplayPlan renders a single-host plan. Used for plays targeting one
	// host or local connection. Output for the single-host serial flow is
	// byte-identical to historical behavior.
	DisplayPlan(tasks []PlannedTask, dryRun bool)
	// DisplayMultiHostPlan renders a consolidated plan for multi-host plays
	// with per-line host attribution. The hosts slice defines render order;
	// tasks are grouped by their PlannedTask.Host. Hosts whose plan contains
	// no will_run/will_change/always_runs/conditional entries are omitted
	// from the body and only counted in the footer's "unchanged" total.
	DisplayMultiHostPlan(tasks []PlannedTask, hosts []string, dryRun bool)
	// PromptApproval asks the user to confirm applying changes. The target
	// argument is a human-readable description of which host(s) the changes
	// will hit (e.g. "web1.prod (ssh)" or "4 hosts (web1, web2, web3, web4)")
	// and is included in the prompt line so users can identify the target
	// without scrolling above the prompt.
	PromptApproval(target string) bool
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
