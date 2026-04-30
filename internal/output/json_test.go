package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tackhq/tack/internal/playbook"
)

func parseJSONLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid JSON: %v\nline: %s", err, line)
	}
	return m
}

func TestJSONEmitter_PlaybookStart(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.PlaybookStart("/tmp/test.yaml")
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "playbook_start" {
		t.Errorf("expected type=playbook_start, got %v", m["type"])
	}
	if m["playbook"] != "/tmp/test.yaml" {
		t.Errorf("expected playbook path, got %v", m["playbook"])
	}
	if m["timestamp"] == nil {
		t.Error("expected timestamp field")
	}
	if m["version"] == nil {
		t.Error("expected version field")
	}
	if got := int(m["version"].(float64)); got != jsonSchemaVersion {
		t.Errorf("expected version=%d, got %d", jsonSchemaVersion, got)
	}
}

func TestJSONEmitter_HostAttribution(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})

	j.HostStart("web1", "ssh")
	j.TaskStart("install nginx", "apt")
	j.TaskResult("install nginx", "changed", true, "installed")
	j.HostStart("web2", "ssh")
	j.TaskStart("install nginx", "apt")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 events, got %d", len(lines))
	}

	// host_start carries host
	hs := parseJSONLine(t, lines[0])
	if hs["host"] != "web1" {
		t.Errorf("expected host=web1 on host_start, got %v", hs["host"])
	}

	// task_start after first HostStart carries host=web1
	ts := parseJSONLine(t, lines[1])
	if ts["host"] != "web1" {
		t.Errorf("expected task_start host=web1, got %v", ts["host"])
	}

	// task_result inherits host=web1
	tr := parseJSONLine(t, lines[2])
	if tr["host"] != "web1" {
		t.Errorf("expected task_result host=web1, got %v", tr["host"])
	}

	// After second HostStart, subsequent events carry host=web2
	ts2 := parseJSONLine(t, lines[4])
	if ts2["host"] != "web2" {
		t.Errorf("expected task_start host=web2, got %v", ts2["host"])
	}
}

func TestJSONEmitter_PlanTaskHost(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})

	plans := []PlannedTask{
		{Host: "web1", Name: "install nginx", Module: "apt", Status: "will_change"},
		{Host: "web2", Name: "install nginx", Module: "apt", Status: "will_run"},
	}
	j.DisplayMultiHostPlan(plans, []string{"web1", "web2"}, false)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 plan_task events, got %d", len(lines))
	}
	for i, line := range lines {
		m := parseJSONLine(t, line)
		if m["type"] != "plan_task" {
			t.Errorf("line %d: expected type=plan_task, got %v", i, m["type"])
		}
		if m["host"] != plans[i].Host {
			t.Errorf("line %d: expected host=%s, got %v", i, plans[i].Host, m["host"])
		}
	}
}

func TestJSONEmitter_PlanTaskNoHost(t *testing.T) {
	// Single-host plays use DisplayPlan with PlannedTask.Host == "" — the
	// `host` field must be omitted, not emitted as empty string.
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})

	plans := []PlannedTask{
		{Name: "install nginx", Module: "apt", Status: "will_change"},
	}
	j.DisplayPlan(plans, false)

	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if _, ok := m["host"]; ok {
		t.Error("expected no host field when PlannedTask.Host is empty")
	}
}

func TestJSONEmitter_PlaybookRecap(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.PlaybookEnd(&mockStats{ok: 3, changed: 1, failed: 0, skipped: 2, duration: 5 * time.Second})
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "playbook_recap" {
		t.Errorf("expected type=playbook_recap, got %v", m["type"])
	}
	if m["ok"] != float64(3) {
		t.Errorf("expected ok=3, got %v", m["ok"])
	}
	if m["success"] != true {
		t.Errorf("expected success=true, got %v", m["success"])
	}
}

func TestJSONEmitter_PlayStart(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.PlayStart(&playbook.Play{Name: "Setup", Hosts: []string{"web1", "web2"}})
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "play_start" {
		t.Errorf("expected type=play_start, got %v", m["type"])
	}
	if m["play"] != "Setup" {
		t.Errorf("expected play=Setup, got %v", m["play"])
	}
}

func TestJSONEmitter_TaskResult(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.TaskResult("Install nginx", "changed", true, "installed nginx 1.24")
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "task_result" {
		t.Errorf("expected type=task_result, got %v", m["type"])
	}
	if m["changed"] != true {
		t.Errorf("expected changed=true, got %v", m["changed"])
	}
	if m["status"] != "changed" {
		t.Errorf("expected status=changed, got %v", m["status"])
	}
}

func TestJSONEmitter_PlanTask(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.DisplayPlan([]PlannedTask{
		{Name: "Install pkg", Module: "apt", Status: "will_change", Params: map[string]any{"name": "nginx"}},
	}, false)
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "plan_task" {
		t.Errorf("expected type=plan_task, got %v", m["type"])
	}
	if m["action"] != "will_change" {
		t.Errorf("expected action=will_change, got %v", m["action"])
	}
}

func TestJSONEmitter_HostRecap(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.HostStart("web1", "ssh")
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "host_start" {
		t.Errorf("expected type=host_start, got %v", m["type"])
	}
	if m["host"] != "web1" {
		t.Errorf("expected host=web1, got %v", m["host"])
	}
}

func TestJSONEmitter_Error(t *testing.T) {
	var buf, errBuf bytes.Buffer
	j := NewJSONEmitter(&buf, &errBuf)
	j.Error("something went wrong: %s", "details")
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["type"] != "error" {
		t.Errorf("expected type=error, got %v", m["type"])
	}
	if !strings.Contains(errBuf.String(), "something went wrong") {
		t.Errorf("expected error on stderr, got %q", errBuf.String())
	}
}

func TestJSONEmitter_PromptApproval(t *testing.T) {
	j := NewJSONEmitter(&bytes.Buffer{}, &bytes.Buffer{})
	if !j.PromptApproval() {
		t.Error("JSONEmitter.PromptApproval should always return true")
	}
}

func TestTextEmitter_Unchanged(t *testing.T) {
	var buf bytes.Buffer
	o := New(&buf)
	o.PlaybookStart("/tmp/test.yaml")
	if !strings.Contains(buf.String(), "PLAYBOOK") {
		t.Errorf("expected PLAYBOOK in text output, got %q", buf.String())
	}
}

func TestJSONEmitter_PlanTaskChecksums(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.DisplayPlan([]PlannedTask{
		{
			Name:        "Copy config",
			Module:      "copy",
			Status:      "will_change",
			OldChecksum: "abc123",
			NewChecksum: "def456",
		},
	}, false)
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["old_checksum"] != "abc123" {
		t.Errorf("expected old_checksum=abc123, got %v", m["old_checksum"])
	}
	if m["new_checksum"] != "def456" {
		t.Errorf("expected new_checksum=def456, got %v", m["new_checksum"])
	}
}

func TestJSONEmitter_PlanTaskDiffContent(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	j.SetDiff(true)
	j.DisplayPlan([]PlannedTask{
		{
			Name:       "Copy config",
			Module:     "copy",
			Status:     "will_change",
			OldContent: "old line",
			NewContent: "new line",
		},
	}, false)
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if m["old_content"] != "old line" {
		t.Errorf("expected old_content with --diff, got %v", m["old_content"])
	}
	if m["new_content"] != "new line" {
		t.Errorf("expected new_content with --diff, got %v", m["new_content"])
	}
}

func TestJSONEmitter_PlanTaskNoDiffContent(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSONEmitter(&buf, &bytes.Buffer{})
	// diff NOT set
	j.DisplayPlan([]PlannedTask{
		{
			Name:       "Copy config",
			Module:     "copy",
			Status:     "will_change",
			OldContent: "old line",
			NewContent: "new line",
		},
	}, false)
	m := parseJSONLine(t, strings.TrimSpace(buf.String()))
	if _, exists := m["old_content"]; exists {
		t.Error("expected no old_content without --diff flag")
	}
	if _, exists := m["new_content"]; exists {
		t.Error("expected no new_content without --diff flag")
	}
}

func TestNewEmitter_InvalidMode(t *testing.T) {
	_, err := NewEmitter("xml")
	if err == nil {
		t.Fatal("expected error for invalid output mode")
	}
}

func TestNewEmitter_JSON_AutoApprove(t *testing.T) {
	e, err := NewEmitter("json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !e.PromptApproval() {
		t.Error("JSON emitter should auto-approve")
	}
}
