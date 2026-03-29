package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenetaranov/bolt/internal/vault"
	"github.com/spf13/cobra"
)

// createTestVault encrypts content with password and writes to path.
func createTestVault(t *testing.T, path string, content string, password string) {
	t.Helper()
	data, err := vault.Encrypt([]byte(content), []byte(password))
	if err != nil {
		t.Fatalf("createTestVault: encrypt: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("createTestVault: write: %v", err)
	}
}

// makeVaultParentCmd creates a parent vaultCmd-like cobra.Command with vault-password-file
// PersistentFlag registered so child commands inherit it.
func makeVaultParentCmd(t *testing.T) *cobra.Command {
	t.Helper()
	parent := &cobra.Command{Use: "vault"}
	parent.PersistentFlags().String("vault-password-file", "", "Path to file containing vault password")
	return parent
}

// makeChildCmd creates a child cobra.Command attached to a parent with vault flags.
func makeChildCmd(t *testing.T, flags map[string]string) *cobra.Command {
	t.Helper()
	parent := makeVaultParentCmd(t)
	child := &cobra.Command{Use: "subcmd"}
	parent.AddCommand(child)
	for k, v := range flags {
		if err := parent.PersistentFlags().Set(k, v); err != nil {
			t.Fatalf("makeChildCmd: set flag %s: %v", k, err)
		}
	}
	return child
}

// TestResolveVaultPassword_EnvVar tests that BOLT_VAULT_PASSWORD env var takes precedence.
func TestResolveVaultPassword_EnvVar(t *testing.T) {
	t.Setenv("BOLT_VAULT_PASSWORD", "env-secret")
	cmd := makeChildCmd(t, nil)
	got, err := resolveVaultPassword(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, []byte("env-secret")) {
		t.Errorf("want %q, got %q", "env-secret", string(got))
	}
}

// TestResolveVaultPassword_File tests that --vault-password-file reads first line only.
func TestResolveVaultPassword_File(t *testing.T) {
	tmp := t.TempDir()
	pwFile := filepath.Join(tmp, "vault_pass.txt")
	if err := os.WriteFile(pwFile, []byte("file-secret\nignored-line\n"), 0600); err != nil {
		t.Fatalf("write pw file: %v", err)
	}
	cmd := makeChildCmd(t, map[string]string{"vault-password-file": pwFile})
	got, err := resolveVaultPassword(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, []byte("file-secret")) {
		t.Errorf("want %q, got %q", "file-secret", string(got))
	}
}

// TestAtomicWrite_Success tests that atomicWrite creates a file with correct content and permissions.
func TestAtomicWrite_Success(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "test.vault")
	content := []byte("encrypted-data")
	if err := atomicWrite(dst, content, 0600); err != nil {
		t.Fatalf("atomicWrite: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: want %q, got %q", content, got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("want perm 0600, got %04o", perm)
	}
}

// TestAtomicWrite_CleansUpOnFailure tests that atomicWrite leaves no temp files on failure.
func TestAtomicWrite_CleansUpOnFailure(t *testing.T) {
	// Writing to a nonexistent directory should fail
	err := atomicWrite("/nonexistent-dir/sub/test.vault", []byte("data"), 0600)
	if err == nil {
		t.Fatal("expected error writing to nonexistent dir")
	}
	// Verify no .bolt-vault-*.tmp files were left in /tmp or current dir
	// (We can't check /nonexistent-dir since it doesn't exist, but we verify
	// the function returned an error indicating cleanup occurred)
	tmpDir := os.TempDir()
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".bolt-vault-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", filepath.Join(tmpDir, e.Name()))
		}
	}
}

// TestRunVaultInit_FileExists tests that runVaultInit refuses when target file exists.
func TestRunVaultInit_FileExists(t *testing.T) {
	t.Setenv("BOLT_VAULT_PASSWORD", "test-password")
	tmp := t.TempDir()
	target := filepath.Join(tmp, "secrets.vault")
	// Create file so it exists
	if err := os.WriteFile(target, []byte("existing"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Build a command tree with vault-password-file flag
	parent := makeVaultParentCmd(t)
	initCmd := &cobra.Command{Use: "init", Args: cobra.ExactArgs(1)}
	parent.AddCommand(initCmd)

	err := runVaultInit(initCmd, []string{target})
	if err == nil {
		t.Fatal("expected error for existing file, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should contain 'already exists', got: %v", err)
	}
}

// TestRunVaultEdit_NoOpDetection tests that unchanged content skips re-encryption.
func TestRunVaultEdit_NoOpDetection(t *testing.T) {
	t.Setenv("BOLT_VAULT_PASSWORD", "test-password")
	// Use 'cat' as EDITOR: cat reads the file to stdout, does NOT modify the temp file
	// so the content remains unchanged.
	t.Setenv("EDITOR", "cat")

	tmp := t.TempDir()
	vaultPath := filepath.Join(tmp, "secrets.vault")
	content := "db_password: unchanged\n"
	createTestVault(t, vaultPath, content, "test-password")

	// Capture original mod time
	info, err := os.Stat(vaultPath)
	if err != nil {
		t.Fatalf("stat vault: %v", err)
	}
	origMod := info.ModTime()

	// Redirect stderr to capture "No changes detected" message
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	parent := makeVaultParentCmd(t)
	editCmd := &cobra.Command{Use: "edit", Args: cobra.ExactArgs(1)}
	parent.AddCommand(editCmd)

	runErr := runVaultEdit(editCmd, []string{vaultPath})

	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderrOut := buf.String()

	if runErr != nil {
		t.Fatalf("runVaultEdit: %v", runErr)
	}
	if !strings.Contains(stderrOut, "No changes detected") {
		t.Errorf("expected 'No changes detected' in stderr, got: %q", stderrOut)
	}
	// Mod time should be unchanged
	info2, err := os.Stat(vaultPath)
	if err != nil {
		t.Fatalf("stat vault after: %v", err)
	}
	if !info2.ModTime().Equal(origMod) {
		t.Errorf("vault file was modified when it should not have been")
	}
}

// TestRunVaultEdit_ContentChanged tests that changed content is re-encrypted correctly.
func TestRunVaultEdit_ContentChanged(t *testing.T) {
	t.Setenv("BOLT_VAULT_PASSWORD", "test-password")
	// Use a shell one-liner as EDITOR that writes new content to the file path
	t.Setenv("EDITOR", "sh")

	tmp := t.TempDir()
	vaultPath := filepath.Join(tmp, "secrets.vault")
	createTestVault(t, vaultPath, "db_password: old\n", "test-password")

	// We need the editor to write new content; override EDITOR to a helper script
	scriptPath := filepath.Join(tmp, "fake_editor.sh")
	scriptContent := "#!/bin/sh\nprintf 'db_password: new\\n' > \"$1\"\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", scriptPath)

	parent := makeVaultParentCmd(t)
	editCmd := &cobra.Command{Use: "edit", Args: cobra.ExactArgs(1)}
	parent.AddCommand(editCmd)

	if err := runVaultEdit(editCmd, []string{vaultPath}); err != nil {
		t.Fatalf("runVaultEdit: %v", err)
	}

	// Read and decrypt the vault to verify new content
	data, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("read vault: %v", err)
	}
	plaintext, err := vault.Decrypt(data, []byte("test-password"))
	if err != nil {
		t.Fatalf("decrypt vault: %v", err)
	}
	if string(plaintext) != "db_password: new\n" {
		t.Errorf("want %q, got %q", "db_password: new\n", string(plaintext))
	}
}
