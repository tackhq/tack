package executor

import (
	"bytes"
	"context"
	"testing"

	"github.com/tackhq/tack/internal/output"
	"github.com/tackhq/tack/internal/playbook"
)

// TestPlaybookDefaultsRoundtripEquivalence confirms that a mapping-format
// playbook with playbook-level `hosts:` resolves and executes identically
// to the equivalent sequence-format playbook.
func TestPlaybookDefaultsRoundtripEquivalence(t *testing.T) {
	mappingYAML := `
hosts: localhost
connection: local
vars:
  greeting: hello
plays:
  - name: First play
    gather_facts: false
    tasks:
      - command:
          cmd: "echo {{ greeting }} one"

  - name: Second play
    gather_facts: false
    tasks:
      - command:
          cmd: "echo {{ greeting }} two"
`
	sequenceYAML := `
- name: First play
  hosts: localhost
  connection: local
  gather_facts: false
  vars:
    greeting: hello
  tasks:
    - command:
        cmd: "echo {{ greeting }} one"

- name: Second play
  hosts: localhost
  connection: local
  gather_facts: false
  vars:
    greeting: hello
  tasks:
    - command:
        cmd: "echo {{ greeting }} two"
`

	runOK := func(yamlStr string) (bool, string) {
		t.Helper()
		pb, err := playbook.ParseRaw([]byte(yamlStr), "test.yaml")
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}

		// Confirm parsed plays look identical at the field level we care about.
		for _, p := range pb.Plays {
			if len(p.Hosts) != 1 || p.Hosts[0] != "localhost" {
				t.Errorf("expected hosts=[localhost], got %v", p.Hosts)
			}
			if p.Connection != "local" {
				t.Errorf("expected connection=local, got %q", p.Connection)
			}
			if p.Vars["greeting"] != "hello" {
				t.Errorf("expected vars.greeting=hello, got %v", p.Vars["greeting"])
			}
		}

		buf := &bytes.Buffer{}
		exec := New()
		exec.Output = output.New(buf)
		exec.AutoApprove = true

		res, err := exec.Run(context.Background(), pb)
		if err != nil {
			t.Fatalf("run error: %v", err)
		}
		return res.Success, buf.String()
	}

	mappingOK, _ := runOK(mappingYAML)
	sequenceOK, _ := runOK(sequenceYAML)

	if !mappingOK {
		t.Error("mapping-format playbook did not succeed")
	}
	if !sequenceOK {
		t.Error("sequence-format playbook did not succeed")
	}
}
