package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tackhq/tack/internal/playbook"
)

func TestFormatStepDuration(t *testing.T) {
	tests := map[time.Duration]string{
		200 * time.Millisecond:                       "0s",
		5 * time.Second:                              "5s",
		59*time.Second + 999e6:                       "59s",
		75 * time.Second:                             "1m15s",
		2 * time.Minute:                              "2m00s",
		2*time.Hour + 3*time.Minute + 40*time.Second: "2h03m",
	}
	for d, want := range tests {
		if got := FormatStepDuration(d); got != want {
			t.Errorf("FormatStepDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestFormatTotalDuration(t *testing.T) {
	tests := map[time.Duration]string{
		400 * time.Millisecond:  "0.4s",
		3200 * time.Millisecond: "3.2s",
		12 * time.Second:        "12s",
		188 * time.Second:       "3m08s",
	}
	for d, want := range tests {
		if got := FormatTotalDuration(d); got != want {
			t.Errorf("FormatTotalDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

// fakeClock returns a clock func and an advance func.
func fakeClock() (func() time.Time, func(time.Duration)) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time { return now }, func(d time.Duration) { now = now.Add(d) }
}

type phaseStats struct{ plan, apply time.Duration }

func (phaseStats) GetOK() int                        { return 2 }
func (phaseStats) GetChanged() int                   { return 1 }
func (phaseStats) GetFailed() int                    { return 0 }
func (phaseStats) GetSkipped() int                   { return 0 }
func (phaseStats) GetDuration() time.Duration        { return time.Hour }
func (p phaseStats) GetPlanDuration() time.Duration  { return p.plan }
func (p phaseStats) GetApplyDuration() time.Duration { return p.apply }
func (phaseStats) GetApprovalWait() time.Duration    { return time.Minute }

func TestTimings_TextOutput(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.SetColor(false)
	o.SetTimings(true)
	clock, advance := fakeClock()
	o.clock = clock

	o.PlayStart(&playbook.Play{Name: "demo"})
	o.HostStart("web1", "ssh")
	o.HostFactsStart("web1")
	advance(1500 * time.Millisecond)
	o.HostFactsResult("web1", true, "")
	advance(200 * time.Millisecond)
	o.DisplayPlan([]PlannedTask{{Name: "Install", Module: "apt", Status: "will_change"}}, false)

	o.TaskStart("Install", "apt")
	advance(75 * time.Second)
	o.TaskResult("Install", "changed", true, "", nil)
	o.PlaybookEnd(phaseStats{plan: 1700 * time.Millisecond, apply: 75 * time.Second})

	out := buf.String()
	for _, want := range []string{
		"gathering facts ✓  1s\n",
		"Plan: 1 to change.  (1.7s)",
		"✓ Install  1m15s\n",
		"RECAP ok=2 changed=1 failed=0 skipped=0 (plan 1.7s · apply 1m15s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestTimings_OffByDefault(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.SetColor(false)
	o.TaskStart("Install", "apt")
	o.TaskResult("Install", "ok", false, "", nil)
	o.PlaybookEnd(phaseStats{plan: time.Second, apply: time.Second})

	out := buf.String()
	if strings.Contains(out, "0s\n") || strings.Contains(out, "plan ") {
		t.Errorf("timings shown without SetTimings(true):\n%s", out)
	}
	if !strings.Contains(out, "(3600.00s)") {
		t.Errorf("legacy total missing:\n%s", out)
	}
}

func TestSpinLabel_ElapsedAfterTwoSeconds(t *testing.T) {
	o := New(&bytes.Buffer{})
	o.SetColor(false)
	o.SetTimings(true)
	clock, advance := fakeClock()
	o.clock = clock
	o.spinStart = clock()

	advance(1900 * time.Millisecond)
	if got := o.spinLabel("Install", 4); got != "Install" {
		t.Errorf("before 2s: %q", got)
	}
	advance(100 * time.Second)
	o.setDetail("SSM InProgress (101s)")
	if got := o.spinLabel("Install", 4); got != "Install  1m41s · SSM InProgress (101s)" {
		t.Errorf("after 2s: %q", got)
	}
}
