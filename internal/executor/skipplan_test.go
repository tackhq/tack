package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tackhq/tack/internal/playbook"
)

func TestSkipPlanPhase(t *testing.T) {
	tests := []struct {
		name     string
		skipPlan bool
		dryRun   bool
		want     bool
	}{
		{"neither", false, false, false},
		{"skip-plan only", true, false, true},
		{"dry-run wins over skip-plan", true, true, false},
		{"dry-run only", false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Executor{SkipPlan: tt.skipPlan, DryRun: tt.dryRun}
			if got := e.skipPlanPhase(); got != tt.want {
				t.Fatalf("skipPlanPhase = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMultiHostPlay_SkipPlanBypassesPrompt(t *testing.T) {
	// --no-plan must skip the approval prompt and apply directly, even though
	// the emitter would decline (approveAnswer=false) if it were ever asked.
	hosts := []string{"web1", "web2"}
	e, emitter := newMultiHostExecutor(t, hosts, false, 1)
	e.SkipPlan = true

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
		"--no-plan should bypass the approval prompt entirely")
}
