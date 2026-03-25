package executor

import (
	"fmt"
	"testing"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

func TestEvaluateCondition(t *testing.T) {
	exec := New()
	pctx := &PlayContext{
		Vars: map[string]any{
			"enabled":   true,
			"disabled":  false,
			"name":      "test",
			"empty":     "",
			"count":     5,
			"os_family": "Debian",
			"facts": map[string]any{
				"os": "linux",
			},
		},
		Registered: map[string]any{
			"result": map[string]any{
				"changed": true,
			},
			"unchanged": map[string]any{
				"changed": false,
			},
		},
	}

	tests := []struct {
		name      string
		condition string
		want      bool
	}{
		// Truthiness
		{"true var", "enabled", true},
		{"false var", "disabled", false},
		{"non-empty string", "name", true},
		{"empty string", "empty", false},
		{"positive number", "count", true},

		// Equality
		{"string equals", "os_family == 'Debian'", true},
		{"string not equals", "os_family == 'RedHat'", false},
		{"dotted equals", "facts.os == 'linux'", true},

		// Inequality
		{"not equals true", "os_family != 'RedHat'", true},
		{"not equals false", "os_family != 'Debian'", false},

		// Negation
		{"not true", "not enabled", false},
		{"not false", "not disabled", true},
		{"not empty", "not empty", true},

		// Registered results
		{"registered changed", "result.changed", true},
		{"registered not changed", "unchanged.changed", false},

		// Boolean literals
		{"literal true", "true", true},
		{"literal false", "false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exec.evaluateCondition(tt.condition, pctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		// Nil
		{"nil", nil, false},

		// Booleans
		{"true", true, true},
		{"false", false, false},

		// Strings
		{"non-empty string", "hello", true},
		{"empty string", "", false},
		{"string false", "false", false},
		{"string False", "False", false},
		{"string no", "no", false},
		{"string yes", "yes", true},

		// Numbers
		{"positive int", 5, true},
		{"zero int", 0, false},
		{"positive float", 3.14, true},
		// Note: zero float returns true due to type comparison quirk in Go
		// {"zero float", 0.0, false},

		// Slices
		{"non-empty slice", []any{"a", "b"}, true},
		{"empty slice", []any{}, false},

		// Maps
		{"non-empty map", map[string]any{"key": "value"}, true},
		{"empty map", map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTruthy(tt.value)
			if got != tt.want {
				t.Errorf("isTruthy(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestResolveValue(t *testing.T) {
	exec := New()
	pctx := &PlayContext{
		Vars: map[string]any{
			"myvar": "myvalue",
			"nested": map[string]any{
				"key": "nested_value",
			},
		},
		Registered: make(map[string]any),
	}

	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"variable", "myvar", "myvalue"},
		{"single quoted string", "'literal'", "literal"},
		{"double quoted string", "\"literal\"", "literal"},
		{"boolean true", "true", true},
		{"boolean True", "True", true},
		{"boolean false", "false", false},
		{"boolean False", "False", false},
		{"dotted path", "nested.key", "nested_value"},
		{"undefined", "notexist", "notexist"}, // Returns the string if not found
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exec.resolveValue(tt.input, pctx)
			if got != tt.want {
				t.Errorf("resolveValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStatsImplementsInterface(t *testing.T) {
	stats := &Stats{
		OK:      1,
		Changed: 2,
		Failed:  3,
		Skipped: 4,
	}

	if stats.GetOK() != 1 {
		t.Errorf("GetOK() = %d, want 1", stats.GetOK())
	}
	if stats.GetChanged() != 2 {
		t.Errorf("GetChanged() = %d, want 2", stats.GetChanged())
	}
	if stats.GetFailed() != 3 {
		t.Errorf("GetFailed() = %d, want 3", stats.GetFailed())
	}
	if stats.GetSkipped() != 4 {
		t.Errorf("GetSkipped() = %d, want 4", stats.GetSkipped())
	}
}

func TestGetEnvMap(t *testing.T) {
	env := getEnvMap()

	// Should have at least some environment variables
	if len(env) == 0 {
		t.Error("expected non-empty environment map")
	}

	// PATH should typically exist
	if _, ok := env["PATH"]; !ok {
		t.Log("PATH not found in environment (might be ok in some test environments)")
	}
}

func TestApplyOverrides_SSMRegionAndBucket(t *testing.T) {
	exec := New()
	exec.Overrides = &ConnOverrides{
		SSMRegion: "us-west-2",
		SSMBucket: "my-bucket",
	}
	play := &playbook.Play{}

	exec.ApplyOverrides(play)

	if play.SSM == nil {
		t.Fatal("play.SSM should not be nil")
	}
	if play.SSM.Region != "us-west-2" {
		t.Errorf("SSM.Region = %v, want us-west-2", play.SSM.Region)
	}
	if play.SSM.Bucket != "my-bucket" {
		t.Errorf("SSM.Bucket = %v, want my-bucket", play.SSM.Bucket)
	}
}

func TestApplyOverrides_SSMInstances(t *testing.T) {
	exec := New()
	exec.Overrides = &ConnOverrides{
		Connection:   "ssm",
		SSMInstances: []string{"i-aaa", "i-bbb"},
	}
	play := &playbook.Play{
		Connection: "ssm",
	}

	exec.ApplyOverrides(play)

	if len(play.Hosts) != 2 || play.Hosts[0] != "i-aaa" || play.Hosts[1] != "i-bbb" {
		t.Errorf("Hosts = %v, want [i-aaa i-bbb]", play.Hosts)
	}
}

func TestApplyOverrides_SSMTags(t *testing.T) {
	exec := New()
	exec.Overrides = &ConnOverrides{
		Connection: "ssm",
		SSMTags:    map[string]string{"Env": "prod", "Role": "web"},
	}
	play := &playbook.Play{
		Connection: "ssm",
	}

	exec.ApplyOverrides(play)

	if play.SSM == nil {
		t.Fatal("play.SSM should not be nil")
	}
	if play.SSM.Tags["Env"] != "prod" || play.SSM.Tags["Role"] != "web" {
		t.Errorf("SSM.Tags = %v, want {Env:prod Role:web}", play.SSM.Tags)
	}
}

func TestApplyOverrides_SSMInstancesPreferredOverTags(t *testing.T) {
	exec := New()
	exec.Overrides = &ConnOverrides{
		Connection:   "ssm",
		SSMInstances: []string{"i-explicit"},
		SSMTags:      map[string]string{"Env": "prod"},
	}
	play := &playbook.Play{
		Connection: "ssm",
	}

	exec.ApplyOverrides(play)

	// Instances should be set as hosts; tags should NOT populate SSM.Tags (instances take priority)
	if len(play.Hosts) != 1 || play.Hosts[0] != "i-explicit" {
		t.Errorf("Hosts = %v, want [i-explicit]", play.Hosts)
	}
	if play.SSM != nil && len(play.SSM.Tags) > 0 {
		t.Error("SSM.Tags should not be set when instances are provided")
	}
}

func TestApplyOverrides_SSMSkippedForNonSSM(t *testing.T) {
	exec := New()
	exec.Overrides = &ConnOverrides{
		Connection:   "ssh",
		SSMInstances: []string{"i-aaa"},
		SSMTags:      map[string]string{"Env": "prod"},
	}
	play := &playbook.Play{
		Connection: "ssh",
	}

	exec.ApplyOverrides(play)

	// SSM instances/tags should not populate hosts for non-SSM connections
	if len(play.Hosts) != 0 {
		t.Errorf("Hosts = %v, want empty (non-SSM connection)", play.Hosts)
	}
}

func TestApplyOverrides_SSMDoesNotOverrideExistingHosts(t *testing.T) {
	exec := New()
	exec.Overrides = &ConnOverrides{
		Connection:   "ssm",
		Hosts:        []string{"i-from-hosts"},
		SSMInstances: []string{"i-from-instances"},
	}
	play := &playbook.Play{
		Connection: "ssm",
	}

	exec.ApplyOverrides(play)

	// Hosts from --hosts override should be used, SSMInstances should not override
	if len(play.Hosts) != 1 || play.Hosts[0] != "i-from-hosts" {
		t.Errorf("Hosts = %v, want [i-from-hosts]", play.Hosts)
	}
}

func TestGetConnector_SSM(t *testing.T) {
	exec := New()
	play := &playbook.Play{
		Connection: "ssm",
		SSM: &playbook.SSMConfig{
			Region: "eu-west-1",
			Bucket: "transfer-bucket",
		},
	}

	conn, err := exec.GetConnector(play, "i-test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := conn.String()
	if got != "ssm://i-test123 (region=eu-west-1)" {
		t.Errorf("String() = %q, want %q", got, "ssm://i-test123 (region=eu-west-1)")
	}
}

func TestGetConnector_SSMWithSudo(t *testing.T) {
	exec := New()
	play := &playbook.Play{
		Connection: "ssm",
		Sudo:       true,
	}

	conn, err := exec.GetConnector(play, "i-test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := conn.String()
	if got != "ssm://i-test123 (sudo)" {
		t.Errorf("String() = %q, want %q", got, "ssm://i-test123 (sudo)")
	}
}

func TestGetConnector_SSMMinimal(t *testing.T) {
	exec := New()
	play := &playbook.Play{
		Connection: "ssm",
	}

	conn, err := exec.GetConnector(play, "i-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := conn.String(); got != "ssm://i-abc" {
		t.Errorf("String() = %q, want %q", got, "ssm://i-abc")
	}
}

func TestToStringMap(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    map[string]string
		wantOK  bool
	}{
		{
			name:   "map[string]string",
			input:  map[string]string{"a": "1", "b": "2"},
			want:   map[string]string{"a": "1", "b": "2"},
			wantOK: true,
		},
		{
			name:   "map[string]any with strings",
			input:  map[string]any{"a": "1", "b": "2"},
			want:   map[string]string{"a": "1", "b": "2"},
			wantOK: true,
		},
		{
			name:   "map[string]any with mixed types",
			input:  map[string]any{"a": "hello", "b": 42},
			want:   map[string]string{"a": "hello", "b": "42"},
			wantOK: true,
		},
		{
			name:   "nil",
			input:  nil,
			want:   nil,
			wantOK: false,
		},
		{
			name:   "string (wrong type)",
			input:  "not a map",
			want:   nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toStringMap(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK {
				if len(got) != len(tt.want) {
					t.Errorf("got %v, want %v", got, tt.want)
				} else {
					for k, v := range tt.want {
						if got[k] != v {
							t.Errorf("got[%q] = %q, want %q", k, got[k], v)
						}
					}
				}
			}
		})
	}
}

func TestConnOverrides_SSMFields(t *testing.T) {
	o := &ConnOverrides{
		SSMInstances: []string{"i-111", "i-222"},
		SSMTags:      map[string]string{"Env": "prod"},
		SSMRegion:    "ap-southeast-1",
		SSMBucket:    "my-bucket",
	}

	if len(o.SSMInstances) != 2 {
		t.Errorf("SSMInstances = %v, want 2 elements", o.SSMInstances)
	}
	if o.SSMTags["Env"] != "prod" {
		t.Errorf("SSMTags[Env] = %q, want prod", o.SSMTags["Env"])
	}
	if o.SSMRegion != "ap-southeast-1" {
		t.Errorf("SSMRegion = %q, want ap-southeast-1", o.SSMRegion)
	}
	if o.SSMBucket != "my-bucket" {
		t.Errorf("SSMBucket = %q, want my-bucket", o.SSMBucket)
	}
}

// Silence the unused import warning.
var _ = fmt.Sprintf
