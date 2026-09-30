package executor

import (
	"testing"
	"time"
)

func TestPlayClock_ExcludesApprovalWait(t *testing.T) {
	base := time.Now()
	c := &playClock{start: base.Add(-10 * time.Second)}
	c.planEnd = base.Add(-8 * time.Second)    // plan took 2s
	c.applyStart = base.Add(-3 * time.Second) // waited 5s at the prompt
	stats := &Stats{}
	c.finish(stats) // apply ~3s

	if stats.PlanTime != 2*time.Second {
		t.Errorf("PlanTime = %v, want 2s", stats.PlanTime)
	}
	if stats.ApprovalWait != 5*time.Second {
		t.Errorf("ApprovalWait = %v, want 5s", stats.ApprovalWait)
	}
	if stats.ApplyTime < 3*time.Second || stats.ApplyTime > 4*time.Second {
		t.Errorf("ApplyTime = %v, want ~3s", stats.ApplyTime)
	}
}

func TestPlayClock_AutoApproveAndNoApply(t *testing.T) {
	c := &playClock{start: time.Now()}
	c.markApplyStart() // -a: plan ends when apply starts
	if c.planEnd != c.applyStart {
		t.Error("auto-approve should record zero approval wait")
	}

	stats := &Stats{}
	(&playClock{start: time.Now().Add(-time.Second)}).finish(stats) // dry run / no changes
	if stats.PlanTime < time.Second || stats.ApplyTime != 0 {
		t.Errorf("no-apply play: plan=%v apply=%v", stats.PlanTime, stats.ApplyTime)
	}

	var nilClock *playClock // paths entered without runPlay
	nilClock.markPlanEnd()
	nilClock.markApplyStart()
	nilClock.finish(stats)
}
