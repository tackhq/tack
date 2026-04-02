package blockinfile

import (
	"testing"
)

const (
	begin = "# BEGIN BOLT MANAGED BLOCK"
	end   = "# END BOLT MANAGED BLOCK"
)

func TestEnsureBlockPresent_NewBlock(t *testing.T) {
	content := "line1\nline2\n"
	result := ensureBlockPresent(content, "managed content", begin, end, "", "")
	expected := "line1\nline2\n# BEGIN BOLT MANAGED BLOCK\nmanaged content\n# END BOLT MANAGED BLOCK\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureBlockPresent_ReplaceExisting(t *testing.T) {
	content := "before\n# BEGIN BOLT MANAGED BLOCK\nold content\n# END BOLT MANAGED BLOCK\nafter\n"
	result := ensureBlockPresent(content, "new content", begin, end, "", "")
	expected := "before\n# BEGIN BOLT MANAGED BLOCK\nnew content\n# END BOLT MANAGED BLOCK\nafter\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureBlockPresent_AlreadyCorrect(t *testing.T) {
	content := "before\n# BEGIN BOLT MANAGED BLOCK\nmanaged\n# END BOLT MANAGED BLOCK\nafter\n"
	result := ensureBlockPresent(content, "managed", begin, end, "", "")
	if result != content {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestEnsureBlockPresent_EmptyBlock(t *testing.T) {
	content := "before\n# BEGIN BOLT MANAGED BLOCK\nold content\n# END BOLT MANAGED BLOCK\nafter\n"
	result := ensureBlockPresent(content, "", begin, end, "", "")
	expected := "before\n# BEGIN BOLT MANAGED BLOCK\n# END BOLT MANAGED BLOCK\nafter\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureBlockPresent_EmptyBlockAlreadyEmpty(t *testing.T) {
	content := "before\n# BEGIN BOLT MANAGED BLOCK\n# END BOLT MANAGED BLOCK\nafter\n"
	result := ensureBlockPresent(content, "", begin, end, "", "")
	if result != content {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestEnsureBlockAbsent_MarkersPresent(t *testing.T) {
	content := "before\n# BEGIN BOLT MANAGED BLOCK\nmanaged\n# END BOLT MANAGED BLOCK\nafter\n"
	result := ensureBlockAbsent(content, begin, end)
	expected := "before\nafter\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureBlockAbsent_NoMarkers(t *testing.T) {
	content := "line1\nline2\n"
	result := ensureBlockAbsent(content, begin, end)
	if result != content {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestCustomMarkers(t *testing.T) {
	params := map[string]any{
		"marker": "<!-- {mark} CUSTOM -->",
	}
	b, e := resolveMarkers(params)
	if b != "<!-- BEGIN CUSTOM -->" {
		t.Errorf("begin marker: expected '<!-- BEGIN CUSTOM -->', got %q", b)
	}
	if e != "<!-- END CUSTOM -->" {
		t.Errorf("end marker: expected '<!-- END CUSTOM -->', got %q", e)
	}
}

func TestCustomMarkerBeginEnd(t *testing.T) {
	params := map[string]any{
		"marker":       "# {mark} BLOCK",
		"marker_begin": "START",
		"marker_end":   "STOP",
	}
	b, e := resolveMarkers(params)
	if b != "# START BLOCK" {
		t.Errorf("begin: expected '# START BLOCK', got %q", b)
	}
	if e != "# STOP BLOCK" {
		t.Errorf("end: expected '# STOP BLOCK', got %q", e)
	}
}

func TestInsertBlockAfterRegex(t *testing.T) {
	content := "[section1]\nkey1=val1\n[section2]\nkey2=val2\n"
	result := ensureBlockPresent(content, "managed", begin, end, `^\[section1\]`, "")
	expected := "[section1]\n# BEGIN BOLT MANAGED BLOCK\nmanaged\n# END BOLT MANAGED BLOCK\nkey1=val1\n[section2]\nkey2=val2\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestInsertBlockBeforeBOF(t *testing.T) {
	content := "line1\nline2\n"
	result := ensureBlockPresent(content, "managed", begin, end, "", "BOF")
	expected := "# BEGIN BOLT MANAGED BLOCK\nmanaged\n# END BOLT MANAGED BLOCK\nline1\nline2\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureBlockPresent_EmptyFile(t *testing.T) {
	result := ensureBlockPresent("", "content", begin, end, "", "")
	expected := "# BEGIN BOLT MANAGED BLOCK\ncontent\n# END BOLT MANAGED BLOCK\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureBlockAbsent_EmptyFile(t *testing.T) {
	result := ensureBlockAbsent("", begin, end)
	if result != "" {
		t.Errorf("expected empty, got:\n%s", result)
	}
}
