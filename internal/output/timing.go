package output

import (
	"fmt"
	"time"
)

// PhaseStats is optionally implemented by Stats to report how long the plan
// and apply phases took (approval wait excluded) and how long the run waited
// at the approval prompt.
type PhaseStats interface {
	GetPlanDuration() time.Duration
	GetApplyDuration() time.Duration
	GetApprovalWait() time.Duration
}

// SetTimings enables grey per-step durations and plan/apply phase totals.
func (o *Output) SetTimings(enabled bool) {
	o.timings = enabled
}

// TimingsEnabled reports whether durations are shown.
func (o *Output) TimingsEnabled() bool {
	return o.timings
}

// FormatStepDuration renders a step duration in whole units: 0s, 5s, 1m15s,
// 2h03m.
func FormatStepDuration(d time.Duration) string {
	s := int(d / time.Second)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	default:
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	}
}

// FormatTotalDuration renders a phase total: one decimal under 10s (0.4s,
// 3.2s), otherwise the step format.
func FormatTotalDuration(d time.Duration) string {
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return FormatStepDuration(d)
}

// elapsedSuffix returns "  <duration>" in grey for a step that started at
// start, or "" when timings are off or the start is unknown.
func (o *Output) elapsedSuffix(start time.Time) string {
	if !o.timings || start.IsZero() {
		return ""
	}
	return "  " + o.color(colorGray, FormatStepDuration(o.now().Sub(start)))
}

// planTotalSuffix returns the grey plan-phase time for the "Plan:" line,
// measured from PlayStart.
func (o *Output) planTotalSuffix() string {
	if !o.timings || o.playStart.IsZero() {
		return ""
	}
	return "  " + o.color(colorGray, "("+FormatTotalDuration(o.now().Sub(o.playStart))+")")
}

// now returns the current time (overridable in tests).
func (o *Output) now() time.Time {
	if o.clock != nil {
		return o.clock()
	}
	return time.Now()
}
