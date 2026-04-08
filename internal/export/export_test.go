package export

import (
	"context"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/playbook"

	// Register modules
	_ "github.com/tackhq/tack/internal/module/apt"
	_ "github.com/tackhq/tack/internal/module/brew"
	_ "github.com/tackhq/tack/internal/module/command"
	_ "github.com/tackhq/tack/internal/module/copy"
	_ "github.com/tackhq/tack/internal/module/file"
)

func TestCompile_BasicTasks(t *testing.T) {
	play := &playbook.Play{
		Name:       "test",
		Hosts:      []string{"localhost"},
		Connection: "local",
		Vars:       map[string]any{"mydir": "/tmp/test"},
		Tasks: []*playbook.Task{
			{Name: "Create dir", Module: "file", Params: map[string]any{"path": "{{ mydir }}", "state": "directory"}},
			{Name: "Run cmd", Module: "command", Params: map[string]any{"cmd": "echo hello"}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:    pb,
		Opts:        Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir: "/tmp",
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(result.Supported) != 2 {
		t.Errorf("expected 2 supported tasks, got %d", len(result.Supported))
	}

	if !strings.Contains(result.Script, "mkdir -p '/tmp/test'") {
		t.Error("expected interpolated path in script")
	}
	if !strings.Contains(result.Script, "echo hello") {
		t.Error("expected command in script")
	}
	if !strings.Contains(result.Script, "set -euo pipefail") {
		t.Error("expected bash strict mode")
	}
	if !strings.Contains(result.Script, "trap on_exit EXIT") {
		t.Error("expected trap in script")
	}
}

func TestCompile_WhenFalse(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Vars:  map[string]any{"enabled": "false"},
		Tasks: []*playbook.Task{
			{Name: "Skipped task", Module: "command", Params: map[string]any{"cmd": "echo skip"}, When: "enabled == 'true'"},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:    pb,
		Opts:        Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir: "/tmp",
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(result.Supported) != 0 {
		t.Errorf("expected 0 supported tasks, got %d", len(result.Supported))
	}
	if !strings.Contains(result.Script, "# SKIPPED (when false)") {
		t.Error("expected SKIPPED comment in script")
	}
}

func TestCompile_TagFiltering(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{Name: "Web task", Module: "command", Params: map[string]any{"cmd": "echo web"}, Tags: []string{"web"}},
			{Name: "DB task", Module: "command", Params: map[string]any{"cmd": "echo db"}, Tags: []string{"db"}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:    pb,
		Opts:        Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true, Tags: []string{"web"}},
		PlaybookDir: "/tmp",
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(result.Supported) != 1 {
		t.Errorf("expected 1 supported task, got %d", len(result.Supported))
	}
	if !strings.Contains(result.Script, "echo web") {
		t.Error("expected web task in script")
	}
	if strings.Contains(result.Script, "echo db") {
		t.Error("did not expect db task in script")
	}
}

func TestCompile_Deterministic(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Vars:  map[string]any{"a": "1", "b": "2"},
		Tasks: []*playbook.Task{
			{Name: "Task A", Module: "command", Params: map[string]any{"cmd": "echo a"}},
			{Name: "Task B", Module: "command", Params: map[string]any{"cmd": "echo b"}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	opts := Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true}

	var scripts [2]string
	for i := 0; i < 2; i++ {
		compiler := &Compiler{Playbook: pb, Opts: opts, PlaybookDir: "/tmp"}
		result, err := compiler.Compile(context.Background(), play, "localhost", nil)
		if err != nil {
			t.Fatalf("Compile %d failed: %v", i, err)
		}
		scripts[i] = result.Script
	}

	if scripts[0] != scripts[1] {
		t.Error("expected identical output from two runs")
	}
}

func TestCompile_UnsupportedModule(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{Name: "Wait task", Module: "wait_for", Params: map[string]any{"port": 8080}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:    pb,
		Opts:        Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir: "/tmp",
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(result.Unsupported) != 1 {
		t.Errorf("expected 1 unsupported task, got %d", len(result.Unsupported))
	}
	if !strings.Contains(result.Script, "# UNSUPPORTED") {
		t.Error("expected UNSUPPORTED comment in script")
	}
}

func TestInterpolation_Recursive(t *testing.T) {
	vars := map[string]any{
		"env": map[string]string{"HOME": "/home/user"},
		"mydir": "{{ env.HOME }}/projects",
	}
	result := interpolateWithVars("{{ mydir }}", vars, false)
	if result != "/home/user/projects" {
		t.Errorf("expected /home/user/projects, got %v", result)
	}
}

func TestInterpolation_NoFacts(t *testing.T) {
	vars := map[string]any{}
	result := interpolateWithVars("{{ facts.os_type }}", vars, true)
	if result != factSentinel {
		t.Errorf("expected sentinel, got %v", result)
	}
}

func TestEvalCondition_Simple(t *testing.T) {
	c := &Compiler{vars: map[string]any{"os": "linux"}}
	result, err := c.evalCondition("os == 'linux'")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true")
	}

	result, err = c.evalCondition("os == 'darwin'")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false")
	}
}

func TestEvalCondition_IsDefined(t *testing.T) {
	c := &Compiler{vars: map[string]any{"x": "1"}}
	result, err := c.evalCondition("x is defined")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true for defined var")
	}

	result, err = c.evalCondition("y is defined")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false for undefined var")
	}
}
