package executor

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/output"
	"github.com/tackhq/tack/internal/playbook"
)

// runTagPlaybook parses and runs a playbook YAML with tag filters and returns success + output.
func runTagPlaybook(t *testing.T, yamlStr string, tags, skipTags []string) (bool, string) {
	t.Helper()

	pb, err := playbook.ParseRaw([]byte(yamlStr), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	buf := &bytes.Buffer{}
	exec := New()
	exec.Output = output.New(buf)
	exec.AutoApprove = true
	exec.Tags = tags
	exec.SkipTags = skipTags

	result, err := exec.Run(context.Background(), pb)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	return result.Success, buf.String()
}

func TestTagsFilterOnlyMatchingTasks(t *testing.T) {
	yaml := `
name: Tag filter test
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy task
    command:
      cmd: echo deploying
    tags: deploy

  - name: Config task
    command:
      cmd: echo configuring
    tags: config

  - name: Untagged task
    command:
      cmd: echo untagged
`
	success, out := runTagPlaybook(t, yaml, []string{"deploy"}, nil)
	if !success {
		t.Errorf("expected success, got failure. Output:\n%s", out)
	}

	if !strings.Contains(out, "Deploy task") {
		t.Error("expected 'Deploy task' to run")
	}
	// Config and untagged should be skipped
	if !strings.Contains(out, "skipped") {
		t.Error("expected some tasks to be skipped")
	}
}

func TestSkipTagsFilterTasks(t *testing.T) {
	yaml := `
name: Skip tags test
hosts: localhost
gather_facts: false
tasks:
  - name: Normal task
    command:
      cmd: echo normal

  - name: Debug task
    command:
      cmd: echo debug
    tags: debug
`
	success, out := runTagPlaybook(t, yaml, nil, []string{"debug"})
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if !strings.Contains(out, "Normal task") {
		t.Error("expected 'Normal task' to run")
	}
}

func TestAlwaysTagRunsDespiteFilter(t *testing.T) {
	yaml := `
name: Always tag test
hosts: localhost
gather_facts: false
tasks:
  - name: Setup always
    command:
      cmd: echo setup
    tags: [always, setup]

  - name: Deploy only
    command:
      cmd: echo deploy
    tags: deploy

  - name: Config only
    command:
      cmd: echo config
    tags: config
`
	success, out := runTagPlaybook(t, yaml, []string{"deploy"}, nil)
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	// "Setup always" should run because of 'always' tag
	if !strings.Contains(out, "Setup always") {
		t.Error("expected 'Setup always' to run (always tag)")
	}
	if !strings.Contains(out, "Deploy only") {
		t.Error("expected 'Deploy only' to run")
	}
}

func TestAlwaysTagSkippedBySkipTags(t *testing.T) {
	yaml := `
name: Always skip test
hosts: localhost
gather_facts: false
tasks:
  - name: Always task
    command:
      cmd: echo always
    tags: [always]

  - name: Deploy task
    command:
      cmd: echo deploy
    tags: deploy
`
	success, out := runTagPlaybook(t, yaml, []string{"deploy"}, []string{"always"})
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if !strings.Contains(out, "Deploy task") {
		t.Error("expected 'Deploy task' to run")
	}
}

func TestNeverTagSkippedByDefault(t *testing.T) {
	yaml := `
name: Never tag test
hosts: localhost
gather_facts: false
tasks:
  - name: Normal task
    command:
      cmd: echo normal

  - name: Debug task
    command:
      cmd: echo debugging
    tags: [never, debug]
`
	success, out := runTagPlaybook(t, yaml, nil, nil)
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if !strings.Contains(out, "Normal task") {
		t.Error("expected 'Normal task' to run")
	}
}

func TestNeverTagRunsWhenExplicitlyTagged(t *testing.T) {
	yaml := `
name: Never override test
hosts: localhost
gather_facts: false
tasks:
  - name: Debug task
    command:
      cmd: echo debugging
    tags: [never, debug]
`
	success, out := runTagPlaybook(t, yaml, []string{"debug"}, nil)
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if !strings.Contains(out, "Debug task") {
		t.Error("expected 'Debug task' to run when explicitly tagged")
	}
}

func TestTagInheritanceThroughBlocks(t *testing.T) {
	yaml := `
name: Block tag inheritance
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy block
    tags: deploy
    block:
      - name: Pull code
        command:
          cmd: echo pulling

      - name: Restart
        command:
          cmd: echo restarting
        tags: restart

  - name: Unrelated task
    command:
      cmd: echo unrelated
    tags: config
`
	success, out := runTagPlaybook(t, yaml, []string{"deploy"}, nil)
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	// Both block child tasks should run (inherited deploy tag)
	if !strings.Contains(out, "Pull code") {
		t.Error("expected 'Pull code' to run (inherits block's deploy tag)")
	}
	if !strings.Contains(out, "Restart") {
		t.Error("expected 'Restart' to run (inherits block's deploy tag)")
	}
}

func TestHandlerIgnoresTagsButRespectsSkipTags(t *testing.T) {
	yaml := `
name: Handler tag test
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy
    command:
      cmd: echo deploying
    tags: deploy
    notify: restart service

handlers:
  - name: restart service
    command:
      cmd: echo restarting
    tags: restart
`
	// Handler should run even though its tags don't match --tags deploy
	success, out := runTagPlaybook(t, yaml, []string{"deploy"}, nil)
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if !strings.Contains(out, "restart service") {
		t.Error("expected handler to run when notified (ignores --tags)")
	}
}

func TestHandlerSkippedBySkipTags(t *testing.T) {
	yaml := `
name: Handler skip-tags test
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy
    command:
      cmd: echo deploying
    notify: slow handler

handlers:
  - name: slow handler
    command:
      cmd: echo slow
    tags: slow
`
	success, out := runTagPlaybook(t, yaml, nil, []string{"slow"})
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if strings.Contains(out, "slow handler") && !strings.Contains(out, "skipped") {
		t.Error("expected handler to be skipped by --skip-tags")
	}
}

func TestPlayLevelTagInheritance(t *testing.T) {
	yaml := `
name: Play tag test
hosts: localhost
gather_facts: false
tags: [infra]
tasks:
  - name: Setup task
    command:
      cmd: echo setting up

  - name: Config task
    command:
      cmd: echo configuring
    tags: config
`
	// Both tasks should run with --tags infra since play tag is inherited
	success, out := runTagPlaybook(t, yaml, []string{"infra"}, nil)
	if !success {
		t.Errorf("expected success. Output:\n%s", out)
	}

	if !strings.Contains(out, "Setup task") {
		t.Error("expected 'Setup task' to run (inherits play's infra tag)")
	}
	if !strings.Contains(out, "Config task") {
		t.Error("expected 'Config task' to run (inherits play's infra tag)")
	}
}

func TestTagFilterInPlanMode(t *testing.T) {
	yamlStr := `
name: Plan mode tag test
hosts: localhost
gather_facts: false
tasks:
  - name: Deploy task
    command:
      cmd: echo deploy
    tags: deploy

  - name: Config task
    command:
      cmd: echo config
    tags: config
`
	pb, err := playbook.ParseRaw([]byte(yamlStr), "test.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	buf := &bytes.Buffer{}
	exec := New()
	exec.Output = output.New(buf)
	exec.DryRun = true
	exec.Tags = []string{"deploy"}

	_, err = exec.Run(context.Background(), pb)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	out := buf.String()
	// In plan mode, non-matching tasks should show as skipped
	if !strings.Contains(out, "Deploy task") {
		t.Error("expected 'Deploy task' to appear in plan")
	}
}
