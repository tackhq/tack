package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("plays: []\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDiscoverDefaultFile_None(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestDiscoverDefaultFile_Single(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "site.yaml")
	t.Chdir(dir)

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "site.yaml" {
		t.Fatalf("expected site.yaml, got %q", got)
	}
}

func TestDiscoverDefaultFile_PrefersYamlOrder(t *testing.T) {
	// Only the .yml variant exists; it should be found.
	dir := t.TempDir()
	writeFile(t, dir, "site.yml")
	t.Chdir(dir)

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "site.yml" {
		t.Fatalf("expected site.yml, got %q", got)
	}
}

func TestDiscoverDefaultFile_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "site.yaml")
	writeFile(t, dir, "site.yml")
	t.Chdir(dir)

	_, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous default playbook") {
		t.Fatalf("expected ambiguity message, got %v", err)
	}
}

func TestDiscoverDefaultFile_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "site.yaml"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(dir)

	got, err := discoverDefaultFile("playbook", defaultPlaybookNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected directory to be ignored, got %q", got)
	}
}

func TestRunCmd_AcceptsZeroArgs(t *testing.T) {
	// The run command should accept zero positional args (default discovery).
	if err := runCmd.Args(runCmd, []string{}); err != nil {
		t.Fatalf("run should accept zero args: %v", err)
	}
	if err := runCmd.Args(runCmd, []string{"a", "b"}); err == nil {
		t.Fatal("run should reject more than one arg")
	}
}

func TestValidateCmd_AcceptsZeroArgs(t *testing.T) {
	// cobra.ArbitraryArgs has a nil Args validator; verify it is not ExactArgs/MinimumNArgs.
	if validateCmd.Args != nil {
		if err := validateCmd.Args(validateCmd, []string{}); err != nil {
			t.Fatalf("validate should accept zero args: %v", err)
		}
	}
}
