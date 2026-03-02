// Package module defines the interface for idempotent system operations.
package module

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/eugenetaranov/bolt/internal/connector"
)

// Result holds the outcome of a module execution.
type Result struct {
	// Changed indicates whether the module made any changes to the system.
	Changed bool

	// Message is a human-readable description of what happened.
	Message string

	// Data holds any additional output data from the module.
	Data map[string]any
}

// Module is the interface that all modules must implement.
type Module interface {
	// Name returns the module's unique identifier.
	Name() string

	// Run executes the module with the given parameters.
	// It should be idempotent - running it multiple times with the same
	// parameters should have the same effect as running it once.
	Run(ctx context.Context, conn connector.Connector, params map[string]any) (*Result, error)
}

// registry holds all registered modules.
var (
	registry   = make(map[string]Module)
	registryMu sync.RWMutex
)

// Register adds a module to the registry.
// It panics if a module with the same name is already registered.
func Register(m Module) {
	registryMu.Lock()
	defer registryMu.Unlock()

	name := m.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("module %q is already registered", name))
	}
	registry[name] = m
}

// Get retrieves a module from the registry by name.
// Returns nil if the module is not found.
func Get(name string) Module {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// List returns the names of all registered modules.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CheckResult describes what a module would do without making changes.
type CheckResult struct {
	WouldChange bool
	Uncertain   bool   // true when change status can't be determined (e.g. command)
	Message     string

	// Optional fields for content comparison (populated by copy/template modules).
	OldChecksum string
	NewChecksum string
	OldContent  string
	NewContent  string
}

// Checker is an optional interface for check/dry-run support.
// Check() must be read-only — it queries remote state but MUST NOT modify it.
type Checker interface {
	Check(ctx context.Context, conn connector.Connector, params map[string]any) (*CheckResult, error)
}

// WouldChange creates a CheckResult indicating changes are needed.
func WouldChange(msg string) *CheckResult {
	return &CheckResult{WouldChange: true, Message: msg}
}

// NoChange creates a CheckResult indicating no changes are needed.
func NoChange(msg string) *CheckResult {
	return &CheckResult{WouldChange: false, Message: msg}
}

// UncertainChange creates a CheckResult indicating change status can't be determined.
func UncertainChange(msg string) *CheckResult {
	return &CheckResult{Uncertain: true, Message: msg}
}

// Helper functions for creating results

// Changed creates a Result indicating a change was made.
func Changed(msg string) *Result {
	return &Result{Changed: true, Message: msg}
}

// Unchanged creates a Result indicating no change was needed.
func Unchanged(msg string) *Result {
	return &Result{Changed: false, Message: msg}
}

// ChangedWithData creates a Result with a change and additional data.
func ChangedWithData(msg string, data map[string]any) *Result {
	return &Result{Changed: true, Message: msg, Data: data}
}
