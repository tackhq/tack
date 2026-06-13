package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestTaskResult_NonInteractive verifies the legacy plain output is unchanged
// when the writer is not a terminal (the common case: pipes, CI, buffers).
func TestTaskResult_NonInteractive(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf) // bytes.Buffer is not a *os.File → interactive=false
	o.SetColor(false)

	o.TaskStart("install nginx", "apt") // no-op
	o.TaskResult("install nginx", "changed", true, "", nil)

	got := buf.String()
	if want := "  ✓ install nginx\n"; got != want {
		t.Fatalf("non-interactive output = %q, want %q", got, want)
	}
	if strings.Contains(got, "\r") || strings.Contains(got, "\033[K") {
		t.Fatalf("non-interactive output must not contain spinner control codes: %q", got)
	}
}

// TestTaskResult_Interactive verifies the spinner path: the line is redrawn in
// place (carriage return + clear-to-EOL) and ends with the final glyph + name.
func TestTaskResult_Interactive(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.interactive = true // force interactive without a real TTY

	done := make(chan struct{})
	go func() {
		o.TaskStart("install nginx", "apt")
		// Let the spinner draw at least one frame.
		time.Sleep(120 * time.Millisecond)
		o.TaskResult("install nginx", "changed", true, "", nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("TaskStart/TaskResult did not complete; spinner goroutine likely not joined")
	}

	got := buf.String()
	for _, want := range []string{"\r", "\033[K", "✓", "install nginx"} {
		if !strings.Contains(got, want) {
			t.Fatalf("interactive output missing %q; got %q", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("interactive result line should end in newline; got %q", got)
	}
	// At least one animation frame should have been drawn.
	if !strings.ContainsAny(got, strings.Join(spinnerFrames, "")) {
		t.Fatalf("expected at least one spinner frame; got %q", got)
	}
}

// TestPrintfStopsSpinner ensures a stray printf (Info/Warn/etc.) halts an active
// spinner instead of racing with it.
func TestPrintfStopsSpinner(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.interactive = true

	o.startSpinner("working")
	o.Info("heads up")

	if o.spin != nil {
		t.Fatal("printf should have stopped the active spinner")
	}
}
