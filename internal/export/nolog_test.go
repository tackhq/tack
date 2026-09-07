package export

import (
	"context"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/playbook"
)

// TestCompile_NoLogWrapsOutput verifies that a task marked no_log has its
// emitted shell commands wrapped to suppress stdout/stderr.
func TestCompile_NoLogWrapsOutput(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{Name: "Secret cmd", Module: "command", Params: map[string]any{"cmd": "echo hunter2"}, NoLog: true},
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

	if !strings.Contains(result.Script, ">/dev/null 2>&1") {
		t.Errorf("expected no_log task to redirect output; script:\n%s", result.Script)
	}
}

// TestCompile_NoLogNotWrappedWhenFalse verifies that ordinary tasks are not
// output-suppressed.
func TestCompile_NoLogNotWrappedWhenFalse(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{Name: "Plain cmd", Module: "command", Params: map[string]any{"cmd": "echo hello"}},
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

	// The command payload should not be redirected.
	for _, line := range strings.Split(result.Script, "\n") {
		if strings.Contains(line, "echo hello") && strings.Contains(line, ">/dev/null 2>&1") {
			t.Errorf("did not expect plain task to redirect output; line: %q", line)
		}
	}
}

// TestCompile_NoLogWrapsMultilineBlock verifies that a no_log task whose shell
// spans multiple lines (heredoc + if/else, as the copy module emits) is wrapped
// in a single brace group rather than having a redirect appended per line, which
// would corrupt the heredoc terminator and if/else keywords.
func TestCompile_NoLogWrapsMultilineBlock(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{
				Name:   "Write secret file",
				Module: "copy",
				Params: map[string]any{"dest": "/etc/app.conf", "content": "password=s3cret\n", "mode": "0600"},
				NoLog:  true,
			},
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

	// The heredoc terminator must stand alone (no trailing redirect appended).
	if strings.Contains(result.Script, "TACK_EOF >/dev/null") {
		t.Errorf("heredoc terminator was corrupted by per-line redirect; script:\n%s", result.Script)
	}
	// if/else keywords must not carry a trailing redirect.
	if strings.Contains(result.Script, "then >/dev/null") || strings.Contains(result.Script, "else >/dev/null") {
		t.Errorf("if/else keyword corrupted by per-line redirect; script:\n%s", result.Script)
	}
	// The block should be wrapped once as a brace group with a single redirect.
	if !strings.Contains(result.Script, "} >/dev/null 2>&1") {
		t.Errorf("expected brace-group redirect for no_log block; script:\n%s", result.Script)
	}
}

// TestUnsupportedTask_NoLogRedaction verifies that an UNSUPPORTED task marked
// no_log has its parameter values redacted in the embedded YAML comment.
func TestUnsupportedTask_NoLogRedaction(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{
				Name:   "Secret wait",
				Module: "wait_for",
				Params: map[string]any{"host": "db.internal", "password": "topsecret"},
				NoLog:  true,
			},
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

	if strings.Contains(result.Script, "topsecret") {
		t.Errorf("expected no_log param value to be redacted; script:\n%s", result.Script)
	}
	if strings.Contains(result.Script, "db.internal") {
		t.Errorf("expected all no_log param values redacted; script:\n%s", result.Script)
	}
	if !strings.Contains(result.Script, "<redacted: no_log>") {
		t.Errorf("expected redaction marker in embedded YAML; script:\n%s", result.Script)
	}
}

// TestUnsupportedTask_NoRedactionWhenLoggable verifies that params are shown for
// ordinary unsupported tasks.
func TestUnsupportedTask_NoRedactionWhenLoggable(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{
				Name:   "Plain wait",
				Module: "wait_for",
				Params: map[string]any{"host": "db.internal"},
			},
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

	if !strings.Contains(result.Script, "db.internal") {
		t.Errorf("expected non-no_log param value to be visible; script:\n%s", result.Script)
	}
}
