package export

import (
	"context"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/playbook"
)

// determinismPlay is a representative play exercising loops, tags, when, and
// several supported modules.
func determinismPlay() (*playbook.Playbook, *playbook.Play) {
	play := &playbook.Play{
		Name:  "provision",
		Hosts: []string{"web01"},
		Vars: map[string]any{
			"app_user": "appsvc",
			"doc_root": "/var/www/app",
			"packages": []any{"nginx", "curl", "vim"},
		},
		Tasks: []*playbook.Task{
			{Name: "Make dir", Module: "file", Params: map[string]any{"path": "{{ doc_root }}", "state": "directory"}, Tags: []string{"files"}},
			{Name: "Install pkg", Module: "command", Params: map[string]any{"cmd": "echo install {{ item }}"}, LoopExpr: "{{ packages }}", Tags: []string{"packages"}},
			{Name: "Cache-only", Module: "command", Params: map[string]any{"cmd": "echo prune"}, When: "role == 'cache'"},
			{Name: "Landing", Module: "command", Params: map[string]any{"cmd": "echo hello > {{ doc_root }}/index.html"}, Tags: []string{"files"}},
		},
	}
	return &playbook.Playbook{Path: "provision.yaml", Plays: []*playbook.Play{play}}, play
}

// TestDeterminism_ByteIdentical verifies that two exports of the same input with
// --no-banner-timestamp produce byte-identical output even when the invocation
// timestamps differ (14.2).
func TestDeterminism_ByteIdentical(t *testing.T) {
	pb, play := determinismPlay()

	compile := func(ts string) string {
		c := &Compiler{
			Playbook: pb,
			Opts: Options{
				Version:           "test",
				PlaybookPath:      "provision.yaml",
				NoFacts:           true,
				NoBannerTimestamp: true,
				Timestamp:         ts, // differs between runs; must not affect output
			},
			PlaybookDir: "/tmp",
		}
		res, err := c.Compile(context.Background(), play, "web01", nil)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		return res.Script
	}

	a := compile("2026-01-01T00:00:00Z")
	b := compile("2099-12-31T23:59:59Z")

	if a != b {
		t.Errorf("expected byte-identical output with --no-banner-timestamp despite different timestamps\n--- run A ---\n%s\n--- run B ---\n%s", a, b)
	}

	// Sanity: loop unrolled in input order, when-false pruned, tags sorted.
	if strings.Index(a, "install nginx") > strings.Index(a, "install curl") {
		t.Error("loop items not in input order")
	}
	if !strings.Contains(a, "# SKIPPED (when false): role == 'cache'") {
		t.Error("expected when-false task to be pruned with a comment")
	}
}

// TestDeterminism_OnlyTimestampVaries verifies that, without
// --no-banner-timestamp, two runs differ *only* in the banner timestamp line.
func TestDeterminism_OnlyTimestampVaries(t *testing.T) {
	pb, play := determinismPlay()

	compile := func(ts string) string {
		c := &Compiler{
			Playbook: pb,
			Opts: Options{
				Version:      "test",
				PlaybookPath: "provision.yaml",
				NoFacts:      true,
				Timestamp:    ts,
			},
			PlaybookDir: "/tmp",
		}
		res, err := c.Compile(context.Background(), play, "web01", nil)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		return res.Script
	}

	stripTimestamp := func(s string) string {
		var kept []string
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "# Exported:") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}

	a := compile("2026-01-01T00:00:00Z")
	b := compile("2099-12-31T23:59:59Z")

	if a == b {
		t.Error("expected the banner timestamp line to differ between runs")
	}
	if stripTimestamp(a) != stripTimestamp(b) {
		t.Error("expected only the timestamp line to differ between runs")
	}
}
