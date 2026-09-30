package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/output"
)

// taskDetailer is implemented by emitters that can show a live sub-status
// next to the running task's spinner (the interactive text emitter).
type taskDetailer interface {
	TaskDetail(detail string)
}

// withTaskProgress wires connector progress for one task to the UI: the
// emitter's live task detail (serial runs) and pctx.OnStatus (the parallel
// progress line). Command echoes are dropped for no_log tasks.
func withTaskProgress(ctx context.Context, pctx *PlayContext, taskName string, noLog bool) context.Context {
	detailer, _ := pctx.Output.(taskDetailer)
	hook := pctx.OnStatus
	if detailer == nil && hook == nil {
		return ctx
	}
	if hook != nil {
		hook(taskName, "")
	}
	return connector.WithProgress(ctx, func(kind connector.ProgressKind, msg string) {
		if kind == connector.ProgressCommand && noLog {
			return
		}
		if detailer != nil {
			detailer.TaskDetail(msg)
		}
		if hook != nil {
			hook(taskName, msg)
		}
	})
}

// hostActivity tracks what each host is doing while hosts run concurrently,
// for the single live progress line on the main terminal.
type hostActivity struct {
	mu     sync.Mutex
	hosts  []string
	status map[string]string
	done   map[string]bool
}

func newHostActivity(hosts []string) *hostActivity {
	return &hostActivity{hosts: hosts, status: make(map[string]string), done: make(map[string]bool)}
}

// set records host's current task and connector detail.
func (a *hostActivity) set(host, task, detail string) {
	s := task
	switch {
	case s == "":
		s = detail
	case detail != "":
		s += " · " + detail
	}
	a.mu.Lock()
	a.status[host] = s
	a.mu.Unlock()
}

// finish marks host as done.
func (a *hostActivity) finish(host string) {
	a.mu.Lock()
	a.done[host] = true
	delete(a.status, host)
	a.mu.Unlock()
}

// progressFor returns a connector progress sink that records host's
// connection-level activity (no task).
func (a *hostActivity) progressFor(host string) connector.ProgressFunc {
	return func(_ connector.ProgressKind, msg string) { a.set(host, "", msg) }
}

// statusHook returns an OnStatus hook recording host's task activity.
func (a *hostActivity) statusHook(host string) func(task, detail string) {
	return func(task, detail string) { a.set(host, task, detail) }
}

// label renders e.g. "applying 1/3 hosts · web2: Install nginx · SSM InProgress (12s) (+1 more)".
func (a *hostActivity) label(verb string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d/%d hosts", verb, len(a.done), len(a.hosts))
	var active []string
	for _, h := range a.hosts {
		if s, ok := a.status[h]; ok && !a.done[h] {
			active = append(active, h+": "+s)
		}
	}
	if len(active) > 0 {
		b.WriteString(" · ")
		b.WriteString(active[0])
		if len(active) > 1 {
			fmt.Fprintf(&b, " (+%d more)", len(active)-1)
		}
	}
	return b.String()
}

// startProgress shows a live progress line on the run's terminal emitter
// (no-op for other emitters or non-interactive output).
func (e *Executor) startProgress(labelFn func() string) (stop func()) {
	if o, ok := e.Output.(*output.Output); ok {
		return o.StartProgress(labelFn)
	}
	return func() {}
}
