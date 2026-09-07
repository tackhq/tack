package export

import (
	"context"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/playbook"
)

// fakeHostVars is a test HostVars implementation.
type fakeHostVars map[string]map[string]any

func (f fakeHostVars) HostVars(host string) map[string]any { return f[host] }

// TestCompile_InventoryHostVars verifies that per-host inventory variables are
// injected and interpolated, and that play vars still win over inventory vars.
func TestCompile_InventoryHostVars(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"web1"},
		Vars:  map[string]any{"app_env": "staging"}, // play var must win over inventory
		Tasks: []*playbook.Task{
			{Name: "Show", Module: "command", Params: map[string]any{"cmd": "echo port={{ listen_port }} env={{ app_env }}"}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:    pb,
		Opts:        Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir: "/tmp",
		Inventory: fakeHostVars{
			"web1": {"listen_port": 8081, "app_env": "production"},
		},
	}

	result, err := compiler.Compile(context.Background(), play, "web1", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result.Script, "echo port=8081 env=staging") {
		t.Errorf("expected host var interpolated and play var to win; script:\n%s", result.Script)
	}
}
