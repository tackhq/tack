package module_test

import (
	"context"
	"sync"
	"testing"

	"github.com/tackhq/tack/internal/connector/local"
	"github.com/tackhq/tack/internal/module/file"
)

// TestCheckerConcurrentSafety verifies that a Checker implementation can be
// invoked from multiple goroutines simultaneously, each with its own
// connector instance, without data races. The multi-host plan pre-pass
// relies on this property: per-host goroutines all call Check() against
// their own connectors.
//
// We use the file module as the canonical Checker — it touches the
// filesystem read-only via the connector and is portable. Other built-in
// Checkers (apt, brew, copy, template, etc.) all use stateless
// `type Module struct{}` receivers per a static audit, so the safety
// property is structural; this test catches accidental introductions of
// package-level mutable state.
//
// Run with: go test -race ./internal/module/...
func TestCheckerConcurrentSafety(t *testing.T) {
	mod := &file.Module{}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			conn := local.New() // each goroutine gets its own connector
			// Path is absent → Check returns no_change without errors. The
			// point isn't behavior, it's that the Check() call itself is
			// race-free.
			params := map[string]any{
				"path":  "/nonexistent/path/safe-for-tests-concurrent-checker",
				"state": "absent",
			}
			if _, err := mod.Check(context.Background(), conn, params); err != nil {
				t.Errorf("Check returned error: %v", err)
			}
		}()
	}
	wg.Wait()
}
