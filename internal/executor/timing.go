package executor

import "time"

// playClock splits one play's wall time into plan, approval-wait and apply
// phases. Methods are nil-safe so paths entered without runPlay (tests) work.
type playClock struct {
	start      time.Time
	planEnd    time.Time // approval prompt shown
	applyStart time.Time // approval given (or auto-approved)
}

// markPlanEnd records that the plan phase ended and the approval prompt is
// about to be shown.
func (c *playClock) markPlanEnd() {
	if c != nil && c.planEnd.IsZero() {
		c.planEnd = time.Now()
	}
}

// markApplyStart records that the apply phase began.
func (c *playClock) markApplyStart() {
	if c == nil || !c.applyStart.IsZero() {
		return
	}
	c.applyStart = time.Now()
	if c.planEnd.IsZero() {
		c.planEnd = c.applyStart // auto-approved: no wait
	}
}

// finish adds this play's phase durations to stats. A play that never
// reached apply (dry run, no changes, declined) counts entirely as plan.
func (c *playClock) finish(stats *Stats) {
	if c == nil || stats == nil {
		return
	}
	end := time.Now()
	if c.applyStart.IsZero() {
		stop := end
		if !c.planEnd.IsZero() {
			stop = c.planEnd // declined at the prompt: the wait isn't planning
			stats.ApprovalWait += end.Sub(c.planEnd)
		}
		stats.PlanTime += stop.Sub(c.start)
		return
	}
	stats.PlanTime += c.planEnd.Sub(c.start)
	stats.ApprovalWait += c.applyStart.Sub(c.planEnd)
	stats.ApplyTime += end.Sub(c.applyStart)
}
