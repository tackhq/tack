package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatTags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, ""},
		{"single", []string{"deploy"}, "[deploy]"},
		{"multiple", []string{"web", "deploy"}, "[web,deploy]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTags(tt.in); got != tt.want {
				t.Fatalf("formatTags(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTaskResult_TagSuffix(t *testing.T) {
	// With tags, the suffix is appended; without, nothing extra.
	var buf bytes.Buffer
	o := New(&buf)
	o.SetColor(false)

	o.TaskResult("install nginx", "changed", true, "", []string{"web", "deploy"})
	o.TaskResult("plain task", "ok", false, "", nil)

	out := buf.String()
	if !strings.Contains(out, "install nginx [web,deploy]") {
		t.Fatalf("expected tagged task to show [web,deploy]; got %q", out)
	}
	if strings.Contains(out, "plain task [") {
		t.Fatalf("untagged task should have no bracket; got %q", out)
	}
}

func TestTaskResult_TagSuffixNoColorCodes(t *testing.T) {
	// Under --no-color the suffix must contain no ANSI escape codes.
	var buf bytes.Buffer
	o := New(&buf)
	o.SetColor(false)

	o.TaskResult("t", "ok", false, "", []string{"deploy"})

	if strings.Contains(buf.String(), "\033[") {
		t.Fatalf("no-color output must not contain escape codes; got %q", buf.String())
	}
}

func TestDisplayPlan_TagSuffix(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.SetColor(false)

	o.DisplayPlan([]PlannedTask{
		{Name: "tagged", Module: "apt", Status: "will_change", Tags: []string{"web", "deploy"}},
		{Name: "untagged", Module: "apt", Status: "will_change"},
	}, false)

	out := buf.String()
	if !strings.Contains(out, "[web,deploy]") {
		t.Fatalf("expected single-host plan to show [web,deploy]; got %q", out)
	}
	if strings.Contains(out, "untagged [") {
		t.Fatalf("untagged plan task should have no bracket; got %q", out)
	}
}

func TestDisplayMultiHostPlan_TagSuffix(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.SetColor(false)

	o.DisplayMultiHostPlan([]PlannedTask{
		{Host: "web1", Name: "tagged", Module: "apt", Status: "will_change", Tags: []string{"deploy"}},
	}, []string{"web1"}, false)

	if !strings.Contains(buf.String(), "[deploy]") {
		t.Fatalf("expected multi-host plan to show [deploy]; got %q", buf.String())
	}
}
