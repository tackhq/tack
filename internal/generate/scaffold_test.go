package generate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldRole(t *testing.T) {
	tmpDir := t.TempDir()

	err := ScaffoldRole("myrole", tmpDir)
	if err != nil {
		t.Fatalf("ScaffoldRole failed: %v", err)
	}

	roleDir := filepath.Join(tmpDir, "myrole")

	// Verify all expected directories exist
	dirs := []string{"tasks", "handlers", "defaults", "vars", "files", "templates"}
	for _, d := range dirs {
		info, err := os.Stat(filepath.Join(roleDir, d))
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}

	// Verify all expected files exist and are non-empty
	files := []string{
		"tasks/main.yaml",
		"handlers/main.yaml",
		"defaults/main.yaml",
		"vars/main.yaml",
		"files/config.txt",
		"templates/app.conf.j2",
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(roleDir, f))
		if err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("expected file %s to be non-empty", f)
		}
	}

	// Verify tasks reference the role name
	tasksData, _ := os.ReadFile(filepath.Join(roleDir, "tasks/main.yaml"))
	if got := string(tasksData); !contains(got, "myrole") {
		t.Error("tasks/main.yaml should reference the role name")
	}

	// Verify defaults reference the role name
	defaultsData, _ := os.ReadFile(filepath.Join(roleDir, "defaults/main.yaml"))
	if got := string(defaultsData); !contains(got, "myrole") {
		t.Error("defaults/main.yaml should reference the role name")
	}
}

func TestScaffoldRoleAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the role directory first
	if err := os.MkdirAll(filepath.Join(tmpDir, "myrole"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := ScaffoldRole("myrole", tmpDir)
	if err == nil {
		t.Fatal("expected error when role directory already exists")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
