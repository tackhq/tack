package executor

import (
	"testing"

	"github.com/tackhq/tack/internal/playbook"
)

func TestShouldGatherFacts(t *testing.T) {
	no := false

	tests := []struct {
		name      string
		skipFacts bool
		play      *playbook.Play
		want      bool
	}{
		{"default gathers", false, &playbook.Play{}, true},
		{"no-facts overrides default", true, &playbook.Play{}, false},
		{"gather_facts false disables", false, &playbook.Play{GatherFacts: &no}, false},
		{"no-facts with gather_facts false", true, &playbook.Play{GatherFacts: &no}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Executor{SkipFacts: tt.skipFacts}
			if got := e.shouldGatherFacts(tt.play); got != tt.want {
				t.Fatalf("shouldGatherFacts = %v, want %v", got, tt.want)
			}
		})
	}
}
