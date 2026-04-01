package executor

import (
	"testing"
)

func newTestPctx() *PlayContext {
	return &PlayContext{
		Vars: map[string]any{
			"os_type":    "Linux",
			"os_family":  "Debian",
			"arch":       "x86_64",
			"version":    "24",
			"port":       8080,
			"enabled":    true,
			"disabled":   false,
			"empty":      "",
			"supported":  []any{"Linux", "Darwin"},
			"count":      3,
			"zero":       0,
			"facts": map[string]any{
				"os_type":       "Linux",
				"os_family":     "Debian",
				"os_version_id": "24",
			},
		},
		Registered: map[string]any{
			"deploy_result": map[string]any{
				"changed": true,
				"data":    map[string]any{"status": "ok"},
			},
		},
	}
}

func TestConditionsAndOperator(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"os_type == 'Linux' and arch == 'x86_64'", true},
		{"os_type == 'Linux' and arch == 'arm64'", false},
		{"os_type == 'Windows' and arch == 'x86_64'", false},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsOrOperator(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"os_type == 'Linux' or os_type == 'Darwin'", true},
		{"os_type == 'Windows' or os_type == 'Linux'", true},
		{"os_type == 'Windows' or os_type == 'FreeBSD'", false},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsPrecedence(t *testing.T) {
	pctx := newTestPctx()
	// "a or b and c" should parse as "a or (b and c)"
	// disabled=false, enabled=true, os_type=='Linux' => false or (true and true) => true
	got, err := evaluateConditionExpr("disabled or enabled and os_type == 'Linux'", pctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true for 'false or (true and true)'")
	}
}

func TestConditionsParentheses(t *testing.T) {
	pctx := newTestPctx()
	// (disabled or enabled) and os_type == 'Windows'
	// (false or true) and false => true and false => false
	got, err := evaluateConditionExpr("(disabled or enabled) and os_type == 'Windows'", pctx)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false for '(true) and false'")
	}
}

func TestConditionsComparison(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"version >= '22'", true},   // numeric: 24 >= 22
		{"version < '30'", true},    // numeric: 24 < 30
		{"version > '24'", false},   // numeric: 24 > 24
		{"version <= '24'", true},   // numeric: 24 <= 24
		{"count > 1", true},         // numeric: 3 > 1
		{"count < 1", false},        // numeric: 3 < 1
		{"os_family > 'A'", true},   // string: "Debian" > "A"
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsIn(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"os_type in supported", true},
		{"'Windows' in supported", false},
		{"os_type not in supported", false},
		{"'Windows' not in supported", true},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsInlineList(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"os_family in ['Debian', 'RedHat']", true},
		{"os_family in ['RedHat', 'Suse']", false},
		{"os_type not in ['Windows']", true},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsIsDefined(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"os_type is defined", true},
		{"nonexistent is defined", false},
		{"os_type is not defined", false},
		{"nonexistent is not defined", true},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsNotOperator(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"not disabled", true},
		{"not enabled", false},
		{"not os_type == 'Windows'", true},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsBackwardCompat(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		// Truthiness
		{"enabled", true},
		{"disabled", false},
		{"empty", false},
		{"os_type", true},
		// Equality
		{"os_type == 'Linux'", true},
		{"os_type != 'Windows'", true},
		// Dotted
		{"facts.os_type == 'Linux'", true},
		// Registered
		{"deploy_result.changed", true},
		// Not
		{"not disabled", true},
		// Boolean literals
		{"true", true},
		{"false", false},
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestConditionsEdgeCases(t *testing.T) {
	pctx := newTestPctx()
	tests := []struct {
		cond string
		want bool
	}{
		{"", true},              // empty condition
		{"zero", false},         // 0 is falsy
		{"count", true},         // 3 is truthy
	}
	for _, tt := range tests {
		got, err := evaluateConditionExpr(tt.cond, pctx)
		if err != nil {
			t.Errorf("%q: %v", tt.cond, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %v, want %v", tt.cond, got, tt.want)
		}
	}
}
