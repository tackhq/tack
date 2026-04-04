package executor

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/eugenetaranov/bolt/internal/output"
	"github.com/eugenetaranov/bolt/internal/playbook"
)

// runAssertPlaybook parses and runs a playbook YAML with auto-approve and returns success + output.
func runAssertPlaybook(t *testing.T, yamlStr string) (bool, string) {
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

// pctxForAssert builds a minimal PlayContext for unit-testing executeAssert.
func pctxForAssert(t *testing.T, vars map[string]any) (*PlayContext, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	pctx := &PlayContext{
		Play:       &playbook.Play{},
		Vars:       vars,
		Registered: map[string]any{},
		Output:     output.New(buf),
	}
	return pctx, buf
}

func TestAssertPassSingleCondition(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 1})
	task := &playbook.Task{Assert: &playbook.AssertSpec{That: []string{"x == 1"}}}

	res, err := exec.executeAssert(context.Background(), pctx, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "ok" {
		t.Errorf("expected ok, got %s", res.Status)
	}
	if res.Changed {
		t.Errorf("assert must never report changed")
	}
}

func TestAssertFailSingleCondition(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 1})
	task := &playbook.Task{Assert: &playbook.AssertSpec{That: []string{"x == 2"}}}

	res, err := exec.executeAssert(context.Background(), pctx, task)
	if err == nil {
		t.Fatal("expected error on failing assert")
	}
	if res.Status != "failed" {
		t.Errorf("expected failed, got %s", res.Status)
	}
	if !strings.Contains(err.Error(), "x == 2") {
		t.Errorf("expected default message to contain failing expr, got: %v", err)
	}
}

func TestAssertListConditionsAllPass(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 1, "y": 2})
	task := &playbook.Task{Assert: &playbook.AssertSpec{That: []string{"x == 1", "y == 2"}}}

	res, err := exec.executeAssert(context.Background(), pctx, task)
	if err != nil || res.Status != "ok" {
		t.Fatalf("expected pass, got status=%s err=%v", res.Status, err)
	}
}

func TestAssertListMultipleFailures(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 0, "y": 0})
	task := &playbook.Task{Assert: &playbook.AssertSpec{That: []string{"x == 1", "y == 2"}}}

	_, err := exec.executeAssert(context.Background(), pctx, task)
	if err == nil {
		t.Fatal("expected failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "x == 1") || !strings.Contains(msg, "y == 2") {
		t.Errorf("expected both failing exprs in default msg, got: %s", msg)
	}
}

func TestAssertCustomFailMsg(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 0})
	task := &playbook.Task{Assert: &playbook.AssertSpec{
		That:    []string{"x == 1"},
		FailMsg: "OS must be Linux",
	}}

	_, err := exec.executeAssert(context.Background(), pctx, task)
	if err == nil || err.Error() != "OS must be Linux" {
		t.Errorf("expected custom fail_msg, got: %v", err)
	}
}

func TestAssertSuccessMsg(t *testing.T) {
	exec := New()
	pctx, buf := pctxForAssert(t, map[string]any{"x": 1})
	task := &playbook.Task{Assert: &playbook.AssertSpec{
		That:       []string{"x == 1"},
		SuccessMsg: "preconditions OK",
	}}

	if _, err := exec.executeAssert(context.Background(), pctx, task); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(buf.String(), "preconditions OK") {
		t.Errorf("expected success_msg in output, got:\n%s", buf.String())
	}
}

func TestAssertQuietModeOnSuccess(t *testing.T) {
	exec := New()
	pctx, buf := pctxForAssert(t, map[string]any{"x": 1})
	task := &playbook.Task{Assert: &playbook.AssertSpec{
		That:  []string{"x == 1"},
		Quiet: true,
	}}

	if _, err := exec.executeAssert(context.Background(), pctx, task); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// quiet should suppress per-condition listing
	if strings.Contains(buf.String(), "x == 1 => true") {
		t.Errorf("quiet mode should suppress per-condition output, got:\n%s", buf.String())
	}
}

func TestAssertQuietStillEmitsFailure(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 0})
	task := &playbook.Task{Assert: &playbook.AssertSpec{
		That:  []string{"x == 1"},
		Quiet: true,
	}}

	_, err := exec.executeAssert(context.Background(), pctx, task)
	if err == nil {
		t.Fatal("expected failure even with quiet")
	}
	if !strings.Contains(err.Error(), "x == 1") {
		t.Errorf("expected failing expr in error message, got: %v", err)
	}
}

func TestAssertRegisterShapeOnPass(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 1, "y": 2})
	task := &playbook.Task{
		Register: "my_assert",
		Assert:   &playbook.AssertSpec{That: []string{"x == 1", "y == 2"}},
	}

	if _, err := exec.executeAssert(context.Background(), pctx, task); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	raw, ok := pctx.Registered["my_assert"].(map[string]any)
	if !ok {
		t.Fatalf("expected registered map, got %T", pctx.Registered["my_assert"])
	}
	if raw["failed"] != false {
		t.Errorf("expected failed=false, got %v", raw["failed"])
	}
	if raw["changed"] != false {
		t.Errorf("expected changed=false, got %v", raw["changed"])
	}
	conds, ok := raw["evaluated_conditions"].([]map[string]any)
	if !ok {
		t.Fatalf("expected evaluated_conditions slice, got %T", raw["evaluated_conditions"])
	}
	if len(conds) != 2 {
		t.Fatalf("expected 2 evaluated conditions, got %d", len(conds))
	}
	for i, c := range conds {
		if c["result"] != true {
			t.Errorf("condition[%d] result expected true, got %v", i, c["result"])
		}
	}
}

func TestAssertRegisterShapeOnFail(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{"x": 0, "y": 2})
	task := &playbook.Task{
		Register: "my_assert",
		Assert:   &playbook.AssertSpec{That: []string{"x == 1", "y == 2"}},
	}

	_, _ = exec.executeAssert(context.Background(), pctx, task)
	raw := pctx.Registered["my_assert"].(map[string]any)
	if raw["failed"] != true {
		t.Errorf("expected failed=true, got %v", raw["failed"])
	}
	conds := raw["evaluated_conditions"].([]map[string]any)
	if conds[0]["result"] != false {
		t.Errorf("first condition should be false")
	}
	if conds[1]["result"] != true {
		t.Errorf("second condition should be true")
	}
}

func TestAssertMalformedExpression(t *testing.T) {
	exec := New()
	pctx, _ := pctxForAssert(t, map[string]any{})
	task := &playbook.Task{Assert: &playbook.AssertSpec{That: []string{"x =="}}}

	_, err := exec.executeAssert(context.Background(), pctx, task)
	if err == nil {
		t.Fatal("expected parser error")
	}
}

// Operator parity tests: one scenario per operator class to prove reuse.
func TestAssertOperatorParity(t *testing.T) {
	exec := New()
	vars := map[string]any{
		"os_type":   "Linux",
		"arch":      "x86_64",
		"supported": []any{"Linux", "Darwin"},
		"count":     5,
		"my_var":    "ok",
	}
	cases := []struct {
		name string
		cond string
		want bool
	}{
		{"eq", "os_type == 'Linux'", true},
		{"in", "os_type in ['Linux', 'Darwin']", true},
		{"is_defined", "my_var is defined", true},
		{"is_not_defined", "missing_var is not defined", true},
		{"and", "os_type == 'Linux' and arch == 'x86_64'", true},
		{"gte", "count >= 3", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pctx, _ := pctxForAssert(t, vars)
			task := &playbook.Task{Assert: &playbook.AssertSpec{That: []string{tc.cond}}}
			_, err := exec.executeAssert(context.Background(), pctx, task)
			if tc.want && err != nil {
				t.Errorf("expected pass for %q, got: %v", tc.cond, err)
			}
			if !tc.want && err == nil {
				t.Errorf("expected fail for %q, got pass", tc.cond)
			}
		})
	}
}

// Playbook-level tests (end-to-end via runPlay → runTask → executeAssert).

func TestAssertPlaybookPasses(t *testing.T) {
	yaml := `
name: assert pass
hosts: localhost
connection: local
gather_facts: false
vars:
  deploy_env: prod
tasks:
  - name: preflight
    assert:
      that:
        - "deploy_env == 'prod'"
`
	ok, out := runAssertPlaybook(t, yaml)
	if !ok {
		t.Errorf("expected success, out:\n%s", out)
	}
}

func TestAssertPlaybookFails(t *testing.T) {
	yaml := `
name: assert fail
hosts: localhost
connection: local
gather_facts: false
vars:
  deploy_env: dev
tasks:
  - name: preflight
    assert:
      that:
        - "deploy_env == 'prod'"
      fail_msg: "must run in prod"
`
	ok, out := runAssertPlaybook(t, yaml)
	if ok {
		t.Errorf("expected failure, out:\n%s", out)
	}
	if !strings.Contains(out, "must run in prod") {
		t.Errorf("expected fail_msg in output, got:\n%s", out)
	}
}

func TestAssertInBlockTriggersRescue(t *testing.T) {
	yaml := `
name: assert in block
hosts: localhost
connection: local
gather_facts: false
vars:
  deploy_env: dev
tasks:
  - name: guarded
    block:
      - assert:
          that:
            - "deploy_env == 'prod'"
    rescue:
      - command:
          cmd: echo rescued
`
	ok, out := runAssertPlaybook(t, yaml)
	if !ok {
		t.Errorf("expected rescue to recover, out:\n%s", out)
	}
	if !strings.Contains(out, "rescued") {
		t.Errorf("expected rescue task to run, out:\n%s", out)
	}
}

func TestAssertWhenSkipsAssert(t *testing.T) {
	yaml := `
name: assert skipped by when
hosts: localhost
connection: local
gather_facts: false
vars:
  deploy_env: dev
tasks:
  - name: preflight
    when: "deploy_env == 'prod'"
    assert:
      that:
        - "false"
`
	ok, out := runAssertPlaybook(t, yaml)
	if !ok {
		t.Errorf("expected skipped assert to succeed overall, out:\n%s", out)
	}
}

func TestAssertDryRunFailsOnFalseCondition(t *testing.T) {
	pb, err := playbook.ParseRaw([]byte(`
name: dry run assert
hosts: localhost
connection: local
gather_facts: false
vars:
  deploy_env: dev
tasks:
  - assert:
      that:
        - "deploy_env == 'prod'"
`), "test.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	buf := &bytes.Buffer{}
	exec := New()
	exec.Output = output.New(buf)
	exec.AutoApprove = true
	exec.DryRun = true
	res, err := exec.Run(context.Background(), pb)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Success {
		t.Errorf("expected failure under --dry-run when assert is false. Output:\n%s", buf.String())
	}
}

func TestAssertRegisteredAvailableToLaterTask(t *testing.T) {
	yaml := `
name: register assert
hosts: localhost
connection: local
gather_facts: false
vars:
  x: 1
tasks:
  - register: chk
    assert:
      that:
        - "x == 1"
  - name: use result
    when: "chk.failed == false"
    command:
      cmd: echo downstream
`
	ok, out := runAssertPlaybook(t, yaml)
	if !ok {
		t.Errorf("expected success, out:\n%s", out)
	}
	if !strings.Contains(out, "downstream") {
		t.Errorf("expected downstream task to run, out:\n%s", out)
	}
}
