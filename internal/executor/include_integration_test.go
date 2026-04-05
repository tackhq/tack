package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/connector/local"
	_ "github.com/tackhq/tack/internal/module/command"
	_ "github.com/tackhq/tack/internal/module/file"
	"github.com/tackhq/tack/internal/output"
	"github.com/tackhq/tack/internal/playbook"
)

func writeTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// testOutputCapture captures task results for assertion.
type testOutputCapture struct {
	nullEmitter
	results []capturedResult
}

type capturedResult struct {
	name, status, detail string
	changed              bool
}

func (t *testOutputCapture) TaskStart(name, module string) {}
func (t *testOutputCapture) TaskResult(name, status string, changed bool, detail string) {
	t.results = append(t.results, capturedResult{name, status, detail, changed})
}

// newIncludeTestPctx creates a PlayContext with a local connector for integration tests.
func newIncludeTestPctx(t *testing.T, playbookDir string, vars map[string]any, emitter output.Emitter) *PlayContext {
	t.Helper()
	if vars == nil {
		vars = make(map[string]any)
	}
	conn := local.New()
	if err := conn.Connect(context.Background()); err != nil {
		t.Fatalf("failed to connect local connector: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return &PlayContext{
		Play:             &playbook.Play{},
		Vars:             vars,
		Facts:            make(map[string]any),
		Registered:       make(map[string]any),
		NotifiedHandlers: make(map[string]bool),
		Connector:        conn,
		Output:           emitter,
		PlaybookDir:      playbookDir,
	}
}

func TestIncludeTasksWithVarsScoping(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")

	// Use file module to create a directory (no shell needed)
	tasksContent := `
- name: "Create dir"
  file:
    path: "` + targetDir + `"
    state: directory
`
	writeTestFile(t,filepath.Join(tmpDir, "install.yml"), []byte(tasksContent), 0644)

	exec := New()
	capture := &testOutputCapture{}
	pctx := newIncludeTestPctx(t, tmpDir, map[string]any{"existing_var": "keep_me"}, capture)

	task := &playbook.Task{
		Name:    "Install nginx",
		Include: "install.yml",
		IncludeVars: map[string]any{
			"package_name": "nginx",
		},
	}

	stats := &Stats{}
	err := exec.runInclude(context.Background(), pctx, task, stats, nil)
	if err != nil {
		t.Fatalf("runInclude failed: %v", err)
	}

	// Verify vars don't leak
	if _, exists := pctx.Vars["package_name"]; exists {
		t.Error("include vars 'package_name' leaked into outer context")
	}
	if pctx.Vars["existing_var"] != "keep_me" {
		t.Errorf("existing var was modified: %v", pctx.Vars["existing_var"])
	}

	// Verify the included task actually ran
	if stats.Tasks < 1 {
		t.Errorf("expected at least 1 task executed, got %d", stats.Tasks)
	}
}

func TestIncludeTasksWithVarsOverride(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")

	tasksContent := `
- name: "Create dir"
  file:
    path: "` + targetDir + `"
    state: directory
`
	writeTestFile(t,filepath.Join(tmpDir, "install.yml"), []byte(tasksContent), 0644)

	exec := New()
	pctx := newIncludeTestPctx(t, tmpDir, map[string]any{"pkg": "apache"}, &nullEmitter{})

	task := &playbook.Task{
		Name:    "Install override",
		Include: "install.yml",
		IncludeVars: map[string]any{
			"pkg": "nginx",
		},
	}

	stats := &Stats{}
	err := exec.runInclude(context.Background(), pctx, task, stats, nil)
	if err != nil {
		t.Fatalf("runInclude failed: %v", err)
	}

	// After include, pkg should be restored to "apache"
	if pctx.Vars["pkg"] != "apache" {
		t.Errorf("expected pkg to be restored to 'apache', got %v", pctx.Vars["pkg"])
	}
}

func TestIncludeTasksWithLoop(t *testing.T) {
	tmpDir := t.TempDir()

	tasksContent := `
- name: "Create dir"
  file:
    path: "` + tmpDir + `/loop_target"
    state: directory
`
	writeTestFile(t,filepath.Join(tmpDir, "process.yml"), []byte(tasksContent), 0644)

	exec := New()
	capture := &testOutputCapture{}
	pctx := newIncludeTestPctx(t, tmpDir, nil, capture)

	task := &playbook.Task{
		Name:    "Process services",
		Include: "process.yml",
		Loop:    []any{"nginx", "redis", "postgres"},
		LoopVar: "svc",
	}

	stats := &Stats{}
	err := exec.runInclude(context.Background(), pctx, task, stats, nil)
	if err != nil {
		t.Fatalf("runInclude failed: %v", err)
	}

	// Should have processed 3 iterations x 1 task each = 3 task executions
	if stats.Tasks != 3 {
		t.Errorf("expected 3 tasks from loop, got %d", stats.Tasks)
	}

	// Loop vars should be cleaned up
	if _, exists := pctx.Vars["svc"]; exists {
		t.Error("loop var 'svc' leaked into outer context")
	}
	if _, exists := pctx.Vars["loop_index"]; exists {
		t.Error("loop var 'loop_index' leaked into outer context")
	}
}

func TestNestedIncludes(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "nested_target")

	writeTestFile(t,filepath.Join(tmpDir, "outer.yml"), []byte(`
- name: From outer
  include: inner.yml
`), 0644)
	writeTestFile(t,filepath.Join(tmpDir, "inner.yml"), []byte(`
- name: From inner
  file:
    path: "`+targetDir+`"
    state: directory
`), 0644)

	exec := New()
	capture := &testOutputCapture{}
	pctx := newIncludeTestPctx(t, tmpDir, nil, capture)

	task := &playbook.Task{
		Name:    "Include outer",
		Include: "outer.yml",
	}

	stats := &Stats{}
	err := exec.runInclude(context.Background(), pctx, task, stats, nil)
	if err != nil {
		t.Fatalf("nested include failed: %v", err)
	}

	if stats.Tasks < 1 {
		t.Errorf("expected at least 1 task from nested include, got %d", stats.Tasks)
	}

	// Verify the nested task actually ran
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Error("nested include did not create the target directory")
	}
}

func TestCircularIncludeRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestFile(t,filepath.Join(tmpDir, "a.yml"), []byte(`
- name: From A
  include: b.yml
`), 0644)
	writeTestFile(t,filepath.Join(tmpDir, "b.yml"), []byte(`
- name: From B
  include: a.yml
`), 0644)

	exec := New()
	pctx := newIncludeTestPctx(t, tmpDir, nil, &nullEmitter{})

	task := &playbook.Task{
		Name:    "Start chain",
		Include: "a.yml",
	}

	stats := &Stats{}
	err := exec.runInclude(context.Background(), pctx, task, stats, nil)
	if err == nil {
		t.Fatal("expected circular include error, got nil")
	}
	if !strings.Contains(err.Error(), "circular include") {
		t.Errorf("expected 'circular include' in error, got: %v", err)
	}
}

func TestIncludeTasksWhenConditionSkip(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestFile(t,filepath.Join(tmpDir, "tasks.yml"), []byte(`
- name: Should not run
  file:
    path: /tmp/should-not-exist
    state: directory
`), 0644)

	exec := New()
	capture := &testOutputCapture{}
	pctx := newIncludeTestPctx(t, tmpDir, map[string]any{
		"facts": map[string]any{"os": "darwin"},
	}, capture)

	task := &playbook.Task{
		Name:    "Linux only",
		Include: "tasks.yml",
		When:    "facts.os == 'linux'",
	}

	stats := &Stats{}
	err := exec.runInclude(context.Background(), pctx, task, stats, nil)
	if err != nil {
		t.Fatalf("runInclude failed: %v", err)
	}

	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped task, got %d", stats.Skipped)
	}
}

// Ensure emitters satisfy output.Emitter at compile time.
var _ output.Emitter = (*nullEmitter)(nil)
var _ output.Emitter = (*testOutputCapture)(nil)
