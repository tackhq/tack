package export

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tackhq/tack/internal/playbook"
	"github.com/tackhq/tack/internal/vault"
)

// writeTestVault encrypts YAML content with password and writes it to dir.
func writeTestVault(t *testing.T, dir, content, password string) string {
	t.Helper()
	enc, err := vault.Encrypt([]byte(content), []byte(password))
	if err != nil {
		t.Fatalf("encrypt vault: %v", err)
	}
	path := filepath.Join(dir, "secrets.yaml")
	if err := os.WriteFile(path, enc, 0600); err != nil {
		t.Fatalf("write vault: %v", err)
	}
	return path
}

// TestCompile_VaultBannerPresent verifies the secret-handling warning appears
// when a vault-sourced value is resolved into the output.
func TestCompile_VaultBannerPresent(t *testing.T) {
	dir := t.TempDir()
	writeTestVault(t, dir, "db_password: hunter2\n", "test-pw")

	play := &playbook.Play{
		Name:      "test",
		Hosts:     []string{"localhost"},
		VaultFile: "secrets.yaml",
		Tasks: []*playbook.Task{
			{Name: "Use secret", Module: "command", Params: map[string]any{"cmd": "echo {{ db_password }}"}, NoLog: true},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:             pb,
		Opts:                 Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir:          dir,
		ResolveVaultPassword: func() ([]byte, error) { return []byte("test-pw"), nil },
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !compiler.VaultUsed {
		t.Error("expected VaultUsed to be true")
	}
	if !strings.Contains(result.Script, "decrypted from vault") {
		t.Errorf("expected vault warning banner; script:\n%s", result.Script)
	}
	if !strings.Contains(result.Script, "do not commit") {
		t.Error("expected 'do not commit' warning in banner")
	}
}

// TestCompile_VaultBannerAbsentWhenUnused verifies no warning when a vault file
// exists but none of its values flow into the output.
func TestCompile_VaultBannerAbsentWhenUnused(t *testing.T) {
	dir := t.TempDir()
	writeTestVault(t, dir, "db_password: hunter2\n", "test-pw")

	play := &playbook.Play{
		Name:      "test",
		Hosts:     []string{"localhost"},
		VaultFile: "secrets.yaml",
		Tasks: []*playbook.Task{
			{Name: "No secret", Module: "command", Params: map[string]any{"cmd": "echo hello"}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:             pb,
		Opts:                 Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir:          dir,
		ResolveVaultPassword: func() ([]byte, error) { return []byte("test-pw"), nil },
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if compiler.VaultUsed {
		t.Error("expected VaultUsed to be false when no vault value is referenced")
	}
	if strings.Contains(result.Script, "decrypted from vault") {
		t.Errorf("did not expect vault warning banner; script:\n%s", result.Script)
	}
}

// TestCompile_VaultBannerAbsentNoVaultFile verifies no warning for plays with no
// vault file at all.
func TestCompile_VaultBannerAbsentNoVaultFile(t *testing.T) {
	play := &playbook.Play{
		Name:  "test",
		Hosts: []string{"localhost"},
		Tasks: []*playbook.Task{
			{Name: "Plain", Module: "command", Params: map[string]any{"cmd": "echo hi"}},
		},
	}
	pb := &playbook.Playbook{Path: "test.yaml", Plays: []*playbook.Play{play}}

	compiler := &Compiler{
		Playbook:    pb,
		Opts:        Options{Version: "test", PlaybookPath: "test.yaml", NoFacts: true, NoBannerTimestamp: true},
		PlaybookDir: "/tmp",
	}

	result, err := compiler.Compile(context.Background(), play, "localhost", nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if strings.Contains(result.Script, "decrypted from vault") {
		t.Error("did not expect vault warning banner when no vault file")
	}
}
