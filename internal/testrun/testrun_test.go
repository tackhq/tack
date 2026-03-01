package testrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsPlaybook(t *testing.T) {
	tests := []struct {
		target string
		want   bool
	}{
		{"setup.yaml", true},
		{"setup.yml", true},
		{"SETUP.YAML", true},
		{"myrole", false},
		{"nginx", false},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := isPlaybook(tt.target)
			if got != tt.want {
				t.Errorf("isPlaybook(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestIsPlaybookExistingFile(t *testing.T) {
	// A file without .yaml/.yml extension that exists should be treated as a playbook
	f, err := os.CreateTemp("", "bolt-test-playbook")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if !isPlaybook(f.Name()) {
		t.Errorf("isPlaybook should return true for existing file without yaml extension")
	}
}

func TestStableContainerName(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"webserver", "bolt-test-webserver"},
		{"setup.yaml", "bolt-test-setup"},
		{"deploy.yml", "bolt-test-deploy"},
		{"/Users/e/projects/bolt/roles/webserver", "bolt-test-webserver"},
		{"my role", "bolt-test-my-role"},
		{"my@role!name", "bolt-test-my-role-name"},
		{"Role_v1.2", "bolt-test-Role_v1.2"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := stableContainerName(tt.target)
			if got != tt.want {
				t.Errorf("stableContainerName(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestStableContainerNameDeterministic(t *testing.T) {
	name1 := stableContainerName("webserver")
	name2 := stableContainerName("webserver")
	if name1 != name2 {
		t.Errorf("stableContainerName should be deterministic: %q != %q", name1, name2)
	}
}

func TestStableContainerNamePrefix(t *testing.T) {
	name := stableContainerName("anything")
	if !strings.HasPrefix(name, "bolt-test-") {
		t.Errorf("container name %q should have prefix bolt-test-", name)
	}
}

func TestGenerateTempPlaybook(t *testing.T) {
	path, cleanup, err := generateTempPlaybook("bolt-test-abc12345", "myrole")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	if !strings.Contains(content, "myrole") {
		t.Error("temp playbook should reference the role name")
	}
	if !strings.Contains(content, "bolt-test-abc12345") {
		t.Error("temp playbook should reference the container name")
	}
	if !strings.Contains(content, "connection: docker") {
		t.Error("temp playbook should use docker connection")
	}
	if !strings.Contains(content, "gather_facts: false") {
		t.Error("temp playbook should disable gather_facts")
	}

	// Verify it has .yaml extension
	if filepath.Ext(path) != ".yaml" {
		t.Errorf("temp playbook should have .yaml extension, got %s", filepath.Ext(path))
	}
}

func TestGenerateTempPlaybookCleanup(t *testing.T) {
	path, cleanup, err := generateTempPlaybook("bolt-test-abc12345", "myrole")
	if err != nil {
		t.Fatal(err)
	}

	// File should exist before cleanup
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("temp playbook should exist: %v", err)
	}

	cleanup()

	// File should be gone after cleanup
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("temp playbook should be removed after cleanup")
	}
}

func TestResolveRoleBareName(t *testing.T) {
	// A bare name that doesn't exist on disk should be returned as-is
	role, err := resolveRole("webserver")
	if err != nil {
		t.Fatal(err)
	}
	if role != "webserver" {
		t.Errorf("resolveRole(bare name) = %q, want %q", role, "webserver")
	}
}

func TestResolveRoleDirectory(t *testing.T) {
	// A directory that exists should be resolved to an absolute path
	dir := t.TempDir()
	role, err := resolveRole(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(role) {
		t.Errorf("resolveRole(directory) = %q, want absolute path", role)
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{
		Target: "myrole",
	}

	if opts.Image != "" {
		t.Error("default Image should be empty (Run fills in ubuntu:24.04)")
	}
	if opts.New {
		t.Error("default New should be false")
	}
	if opts.Remove {
		t.Error("default Remove should be false")
	}
}
