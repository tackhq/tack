package executor

import (
	"bytes"
	"context"
	"testing"

	"github.com/tackhq/tack/internal/output"
	"github.com/tackhq/tack/internal/playbook"
)

// runBlockPlaybook parses and runs a playbook YAML with auto-approve and returns success + output.
func runBlockPlaybook(t *testing.T, yamlStr string) (bool, string) {
	t.Helper()

	pb, err := playbook.ParseRaw([]byte(yamlStr), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	buf := &bytes.Buffer{}
	exec := New()
	exec.Output = output.New(buf)
	exec.AutoApprove = true

	result, err := exec.Run(context.Background(), pb)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	return result.Success, buf.String()
}

func TestBlockSucceeds(t *testing.T) {
	yaml := `
name: Test block success
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy block
    block:
      - name: Step 1
        command:
          cmd: echo step1
      - name: Step 2
        command:
          cmd: echo step2
    always:
      - name: Cleanup
        command:
          cmd: echo cleanup
`
	success, out := runBlockPlaybook(t, yaml)
	if !success {
		t.Errorf("expected success, got failure. Output:\n%s", out)
	}
}

func TestBlockFailsRescueRuns(t *testing.T) {
	yaml := `
name: Test block fail with rescue
hosts: localhost
gather_facts: false
tasks:
  - name: Safe deploy
    block:
      - name: Will fail
        command:
          cmd: /bin/sh -c "exit 1"
    rescue:
      - name: Rollback
        command:
          cmd: echo rolling_back
    always:
      - name: Notify
        command:
          cmd: echo notifying
`
	success, out := runBlockPlaybook(t, yaml)
	if !success {
		t.Errorf("expected success (rescue recovered), got failure. Output:\n%s", out)
	}
}

func TestBlockFailsNoRescue(t *testing.T) {
	yaml := `
name: Test block fail without rescue
hosts: localhost
gather_facts: false
tasks:
  - name: Risky block
    block:
      - name: Will fail
        command:
          cmd: /bin/sh -c "exit 1"
    always:
      - name: Always runs
        command:
          cmd: echo always
`
	success, out := runBlockPlaybook(t, yaml)
	if success {
		t.Errorf("expected failure (no rescue), got success. Output:\n%s", out)
	}
}

func TestBlockAndRescueBothFail(t *testing.T) {
	yaml := `
name: Test both fail
hosts: localhost
gather_facts: false
tasks:
  - name: Double fail
    block:
      - name: Block fails
        command:
          cmd: /bin/sh -c "exit 1"
    rescue:
      - name: Rescue also fails
        command:
          cmd: /bin/sh -c "exit 2"
    always:
      - name: Still runs
        command:
          cmd: echo still_running
`
	success, out := runBlockPlaybook(t, yaml)
	if success {
		t.Errorf("expected failure (rescue also failed), got success. Output:\n%s", out)
	}
}

func TestBlockWhenFalseSkips(t *testing.T) {
	yaml := `
name: Test block when false
hosts: localhost
gather_facts: false
vars:
  skip_block: true
tasks:
  - name: Skipped block
    block:
      - name: Should not run
        command:
          cmd: /bin/sh -c "exit 1"
    rescue:
      - name: Should not run either
        command:
          cmd: echo rescue
    always:
      - name: Should not run too
        command:
          cmd: echo always
    when: not skip_block
`
	success, out := runBlockPlaybook(t, yaml)
	if !success {
		t.Errorf("expected success (block skipped), got failure. Output:\n%s", out)
	}
}

func TestNestedBlockWithinRescue(t *testing.T) {
	yaml := `
name: Test nested block
hosts: localhost
gather_facts: false
tasks:
  - name: Outer block
    block:
      - name: Outer fails
        command:
          cmd: /bin/sh -c "exit 1"
    rescue:
      - name: Inner block
        block:
          - name: Inner step
            command:
              cmd: echo inner_recovery
`
	success, out := runBlockPlaybook(t, yaml)
	if !success {
		t.Errorf("expected success (nested block in rescue succeeded), got failure. Output:\n%s", out)
	}
}

func TestPlanBlockOutput(t *testing.T) {
	yaml := `
name: Test plan block
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy block
    block:
      - name: Deploy step
        command:
          cmd: echo deploy
    rescue:
      - name: Rollback step
        command:
          cmd: echo rollback
    always:
      - name: Notify step
        command:
          cmd: echo notify
`
	pb, err := playbook.ParseRaw([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	buf := &bytes.Buffer{}
	exec := New()
	exec.Output = output.New(buf)
	exec.DryRun = true

	result, err := exec.Run(context.Background(), pb)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !result.Success {
		t.Error("expected dry run success")
	}

	out := buf.String()
	// Verify block structure appears in plan
	if !bytes.Contains([]byte(out), []byte("BLOCK:")) {
		t.Errorf("expected BLOCK: in plan output, got:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("RESCUE:")) {
		t.Errorf("expected RESCUE: in plan output, got:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("ALWAYS:")) {
		t.Errorf("expected ALWAYS: in plan output, got:\n%s", out)
	}
}
