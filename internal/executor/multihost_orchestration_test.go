package executor

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/output"
	"github.com/tackhq/tack/internal/playbook"
)

// countingEmitter wraps a nullEmitter and counts PromptApproval calls. Used
// to verify the consolidated approval prompt is invoked exactly once for a
// multi-host play, regardless of host count or fork value.
type countingEmitter struct {
	nullEmitter
	approvalCalls int64
	approveAnswer bool
}

func (c *countingEmitter) PromptApproval(_ string) bool {
	atomic.AddInt64(&c.approvalCalls, 1)
	return c.approveAnswer
}

func (c *countingEmitter) calls() int64 {
	return atomic.LoadInt64(&c.approvalCalls)
}

func newMultiHostExecutor(t *testing.T, hosts []string, autoApprove bool, forks int) (*Executor, *countingEmitter) {
	t.Helper()
	emitter := &countingEmitter{approveAnswer: false}
	e := New()
	e.Output = emitter
	e.AutoApprove = autoApprove
	e.Forks = forks
	e.connectorFactory = func(play *playbook.Play, host string) (connector.Connector, error) {
		return &fakeConnector{host: host}, nil
	}
	return e, emitter
}

func TestMultiHostPlay_ApprovalRunsOnce_Serial(t *testing.T) {
	hosts := []string{"web1", "web2", "web3"}
	e, emitter := newMultiHostExecutor(t, hosts, false, 1)

	// All hosts have a will_run task → not allNoChange → approval required.
	play := &playbook.Play{
		Hosts:      hosts,
		Connection: "ssh",
		Tasks: []*playbook.Task{
			{Module: "command", Params: map[string]any{"cmd": "true"}},
		},
	}

	stats := &Stats{}
	err := e.runMultiHostPlay(context.Background(), play, stats, nil, "")
	// User declined → no error, just early return.
	require.NoError(t, err)

	assert.Equal(t, int64(1), emitter.calls(),
		"expected exactly one approval prompt for %d-host serial play", len(hosts))
}

func TestMultiHostPlay_ApprovalRunsOnce_Forks(t *testing.T) {
	hosts := []string{"web1", "web2", "web3", "web4"}
	e, emitter := newMultiHostExecutor(t, hosts, false, 4)

	play := &playbook.Play{
		Hosts:      hosts,
		Connection: "ssh",
		Tasks: []*playbook.Task{
			{Module: "command", Params: map[string]any{"cmd": "true"}},
		},
	}

	stats := &Stats{}
	err := e.runMultiHostPlay(context.Background(), play, stats, nil, "")
	require.NoError(t, err)

	assert.Equal(t, int64(1), emitter.calls(),
		"expected exactly one approval prompt for %d-host parallel play", len(hosts))
}

func TestMultiHostPlay_AutoApproveSkipsPrompt(t *testing.T) {
	hosts := []string{"web1", "web2"}
	e, emitter := newMultiHostExecutor(t, hosts, true, 1)
	emitter.approveAnswer = true // doesn't matter; should not be called

	play := &playbook.Play{
		Hosts:      hosts,
		Connection: "ssh",
		Tasks: []*playbook.Task{
			{Module: "command", Params: map[string]any{"cmd": "true"}},
		},
	}

	stats := &Stats{}
	_ = e.runMultiHostPlay(context.Background(), play, stats, nil, "")

	assert.Equal(t, int64(0), emitter.calls(),
		"--auto-approve should bypass the prompt entirely")
}

func TestMultiHostPlay_NoChangeShortcutSkipsPrompt(t *testing.T) {
	// All hosts no-op → allNoChange shortcut → no prompt.
	hosts := []string{"web1", "web2"}
	e, emitter := newMultiHostExecutor(t, hosts, false, 1)

	// No tasks → planTasks returns empty plan → allNoChange returns true.
	play := &playbook.Play{
		Hosts:      hosts,
		Connection: "ssh",
		Tasks:      nil,
	}

	stats := &Stats{}
	err := e.runMultiHostPlay(context.Background(), play, stats, nil, "")
	require.NoError(t, err)

	assert.Equal(t, int64(0), emitter.calls(),
		"empty plan should not prompt for approval")
}

func TestMultiHostPlay_DryRunSkipsPrompt(t *testing.T) {
	hosts := []string{"web1", "web2"}
	e, emitter := newMultiHostExecutor(t, hosts, false, 1)
	e.DryRun = true

	play := &playbook.Play{
		Hosts:      hosts,
		Connection: "ssh",
		Tasks: []*playbook.Task{
			{Module: "command", Params: map[string]any{"cmd": "true"}},
		},
	}

	stats := &Stats{}
	err := e.runMultiHostPlay(context.Background(), play, stats, nil, "")
	require.NoError(t, err)

	assert.Equal(t, int64(0), emitter.calls(),
		"--dry-run should not prompt for approval")
}

func TestMultiHostPlay_ContextCancelDuringApproval(t *testing.T) {
	// SIGINT during approval (modeled here as context cancel before prompt
	// resolution) must result in zero applied state. This is the headline
	// correctness improvement of consolidated-plan-and-approval.
	hosts := []string{"web1", "web2"}
	e, _ := newMultiHostExecutor(t, hosts, false, 1)

	play := &playbook.Play{
		Hosts:      hosts,
		Connection: "ssh",
		Tasks: []*playbook.Task{
			{Module: "command", Params: map[string]any{"cmd": "true"}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before run starts

	stats := &Stats{}
	_ = e.runMultiHostPlay(ctx, play, stats, nil, "")

	// Either pre-pass cancelled or returned ctx.Err. In all paths, no host
	// applied.
	assert.Equal(t, 0, stats.OK)
	assert.Equal(t, 0, stats.Changed)
	assert.Equal(t, 0, stats.Failed)
}

func TestPlannedTask_HostPopulatedByPlanTasks(t *testing.T) {
	exec := New()
	pctx := &PlayContext{
		Host:       "web1",
		Vars:       make(map[string]any),
		Registered: make(map[string]any),
	}

	tasks := []*playbook.Task{
		{Module: "command", Name: "do thing", Params: map[string]any{"cmd": "true"}},
	}

	plan := exec.planTasks(context.Background(), pctx, tasks, &nullEmitter{})
	require.Len(t, plan, 1)
	assert.Equal(t, "web1", plan[0].Host,
		"PlannedTask should carry the host from PlayContext")
}

// Compile-time check that countingEmitter satisfies output.Emitter.
var _ output.Emitter = (*countingEmitter)(nil)
