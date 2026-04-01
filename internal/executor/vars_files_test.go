package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

func TestLoadVarsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vars.yaml")
	if err := os.WriteFile(path, []byte("db_host: localhost\ndb_port: 5432\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vars, err := loadVarsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["db_host"] != "localhost" {
		t.Errorf("expected db_host=localhost, got %v", vars["db_host"])
	}
	if vars["db_port"] != 5432 {
		t.Errorf("expected db_port=5432, got %v", vars["db_port"])
	}
}

func TestLoadVarsFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vars.yaml"), []byte("key1: val1\nkey2: val2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"vars.yaml"},
		Vars:      map[string]any{},
	}

	merged, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged["key1"] != "val1" {
		t.Errorf("expected key1=val1, got %v", merged["key1"])
	}
	if merged["key2"] != "val2" {
		t.Errorf("expected key2=val2, got %v", merged["key2"])
	}
}

func TestLoadVarsFiles_MultipleOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), []byte("key: base\nonly_base: yes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "override.yaml"), []byte("key: override\nonly_override: yes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"base.yaml", "override.yaml"},
		Vars:      map[string]any{},
	}

	merged, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged["key"] != "override" {
		t.Errorf("expected key=override (last wins), got %v", merged["key"])
	}
	if merged["only_base"] != "yes" {
		t.Errorf("expected only_base from first file, got %v", merged["only_base"])
	}
	if merged["only_override"] != "yes" {
		t.Errorf("expected only_override from second file, got %v", merged["only_override"])
	}
}

func TestLoadVarsFiles_RelativePath(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "vars")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "prod.yaml"), []byte("env: production\n"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"vars/prod.yaml"},
		Vars:      map[string]any{},
	}

	merged, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged["env"] != "production" {
		t.Errorf("expected env=production, got %v", merged["env"])
	}
}

func TestLoadVarsFiles_PathInterpolation(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "vars")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "staging.yaml"), []byte("db_host: staging-db\n"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"vars/{{ env }}.yaml"},
		Vars:      map[string]any{"env": "staging"},
	}

	merged, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged["db_host"] != "staging-db" {
		t.Errorf("expected db_host=staging-db, got %v", merged["db_host"])
	}
}

func TestLoadVarsFiles_MissingRequired(t *testing.T) {
	dir := t.TempDir()

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"nonexistent.yaml"},
		Vars:      map[string]any{},
	}

	_, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err == nil {
		t.Fatal("expected error for missing required file")
	}
}

func TestLoadVarsFiles_OptionalMissing(t *testing.T) {
	dir := t.TempDir()

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"?optional.yaml"},
		Vars:      map[string]any{},
	}

	merged, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(merged) != 0 {
		t.Errorf("expected empty map for skipped optional file, got %v", merged)
	}
}

func TestLoadVarsFiles_OptionalPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "local.yaml"), []byte("override: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"?local.yaml"},
		Vars:      map[string]any{},
	}

	merged, err := exec.loadVarsFiles(play, dir, play.Vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged["override"] != true {
		t.Errorf("expected override=true, got %v", merged["override"])
	}
}

func TestLoadVarsFiles_Precedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extra.yaml"), []byte("shared: from_file\nfile_only: yes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := New()
	play := &playbook.Play{
		VarsFiles: []string{"extra.yaml"},
		Vars:      map[string]any{"shared": "from_play", "play_only": "yes"},
	}

	// Simulate the merge: play vars first, then vars_files override
	pctxVars := make(map[string]any)
	for k, v := range play.Vars {
		pctxVars[k] = v
	}

	vfVars, err := exec.loadVarsFiles(play, dir, pctxVars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Apply vars_files on top of play vars (as executor does)
	for k, v := range vfVars {
		pctxVars[k] = v
	}

	if pctxVars["shared"] != "from_file" {
		t.Errorf("expected vars_files to override play vars: shared=%v", pctxVars["shared"])
	}
	if pctxVars["play_only"] != "yes" {
		t.Errorf("expected play-only var preserved: play_only=%v", pctxVars["play_only"])
	}
	if pctxVars["file_only"] != "yes" {
		t.Errorf("expected file-only var added: file_only=%v", pctxVars["file_only"])
	}
}
