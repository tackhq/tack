package lineinfile

import (
	"testing"
)

func TestEnsurePresent_ExactMatch(t *testing.T) {
	content := "line1\nline2\nline3\n"
	result, err := ensurePresent(content, "line2", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result != content {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestEnsurePresent_AppendMissing(t *testing.T) {
	content := "line1\nline2\n"
	result, err := ensurePresent(content, "line3", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "line1\nline2\nline3\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_RegexpReplace(t *testing.T) {
	content := "port=8080\nhost=localhost\n"
	result, err := ensurePresent(content, "port=9090", "^port=", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "port=9090\nhost=localhost\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_RegexpReplaceLast(t *testing.T) {
	content := "allow=10.0.0.1\nallow=10.0.0.2\nallow=10.0.0.3\n"
	result, err := ensurePresent(content, "allow=192.168.1.1", "^allow=", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "allow=10.0.0.1\nallow=10.0.0.2\nallow=192.168.1.1\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_RegexpAlreadyCorrect(t *testing.T) {
	content := "port=9090\nhost=localhost\n"
	result, err := ensurePresent(content, "port=9090", "^port=", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result != content {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestEnsurePresent_RegexpNoMatch(t *testing.T) {
	content := "host=localhost\n"
	result, err := ensurePresent(content, "port=8080", "^port=", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "host=localhost\nport=8080\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_InsertAfterRegex(t *testing.T) {
	content := "[section1]\nkey1=val1\n[section2]\nkey2=val2\n"
	result, err := ensurePresent(content, "key3=val3", "", `^\[section1\]`, "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "[section1]\nkey3=val3\nkey1=val1\n[section2]\nkey2=val2\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_InsertBeforeRegex(t *testing.T) {
	content := "line1\nline2\nline3\n"
	result, err := ensurePresent(content, "inserted", "", "", "^line2$")
	if err != nil {
		t.Fatal(err)
	}
	expected := "line1\ninserted\nline2\nline3\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_InsertBeforeBOF(t *testing.T) {
	content := "line1\nline2\n"
	result, err := ensurePresent(content, "first", "", "", "BOF")
	if err != nil {
		t.Fatal(err)
	}
	expected := "first\nline1\nline2\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsurePresent_InsertAfterNoMatch(t *testing.T) {
	content := "line1\nline2\n"
	result, err := ensurePresent(content, "new", "", "^nonexistent$", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "line1\nline2\nnew\n"
	if result != expected {
		t.Errorf("expected append to end:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureAbsent_ExactMatch(t *testing.T) {
	content := "line1\nline2\nline3\n"
	result, err := ensureAbsent(content, "line2", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "line1\nline3\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureAbsent_Regexp(t *testing.T) {
	content := "allow=10.0.0.1\nhost=localhost\nallow=10.0.0.2\n"
	result, err := ensureAbsent(content, "", "^allow=")
	if err != nil {
		t.Fatal(err)
	}
	expected := "host=localhost\n"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestEnsureAbsent_NoMatch(t *testing.T) {
	content := "line1\nline2\n"
	result, err := ensureAbsent(content, "nonexistent", "")
	if err != nil {
		t.Fatal(err)
	}
	if result != content {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestEnsurePresent_EmptyFile(t *testing.T) {
	result, err := ensurePresent("", "newline", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := "newline\n"
	if result != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, result)
	}
}

func TestEnsureAbsent_EmptyContent(t *testing.T) {
	result, err := ensureAbsent("", "anything", "")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty, got:\n%q", result)
	}
}

func TestSplitJoinRoundTrip(t *testing.T) {
	content := "line1\nline2\nline3\n"
	lines := splitLines(content)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	rejoined := joinLines(lines)
	if rejoined != content {
		t.Errorf("round trip failed:\noriginal: %q\nrejoined: %q", content, rejoined)
	}
}
