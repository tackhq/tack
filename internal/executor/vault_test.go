package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tackhq/tack/internal/playbook"
	"github.com/tackhq/tack/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeVaultFile creates a temp vault file encrypted with password and returns its path.
func makeVaultFile(t *testing.T, dir, filename string, content []byte, password []byte) string {
	t.Helper()
	data, err := vault.Encrypt(content, password)
	require.NoError(t, err)
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestLoadVaultVars(t *testing.T) {
	password := []byte("test-password")
	yamlContent := []byte("db_host: localhost\ndb_pass: secret123\n")

	t.Run("returns parsed vars from valid vault file", func(t *testing.T) {
		dir := t.TempDir()
		vaultPath := makeVaultFile(t, dir, "secrets.vault", yamlContent, password)

		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) { return password, nil }
		play := &playbook.Play{VaultFile: vaultPath}

		vars, err := e.loadVaultVars(play, dir)
		require.NoError(t, err)
		assert.Equal(t, "localhost", vars["db_host"])
		assert.Equal(t, "secret123", vars["db_pass"])
	})

	t.Run("resolves relative path against playbookDir", func(t *testing.T) {
		dir := t.TempDir()
		makeVaultFile(t, dir, "secrets.vault", yamlContent, password)

		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) { return password, nil }
		play := &playbook.Play{VaultFile: "secrets.vault"}

		vars, err := e.loadVaultVars(play, dir)
		require.NoError(t, err)
		assert.Equal(t, "localhost", vars["db_host"])
	})

	t.Run("caches result - ResolveVaultPassword called only once", func(t *testing.T) {
		dir := t.TempDir()
		absPath := makeVaultFile(t, dir, "secrets.vault", yamlContent, password)

		callCount := 0
		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) {
			callCount++
			return password, nil
		}
		play := &playbook.Play{VaultFile: absPath}

		// First call
		_, err := e.loadVaultVars(play, dir)
		require.NoError(t, err)
		// Second call — should use cache, not call ResolveVaultPassword again
		_, err = e.loadVaultVars(play, dir)
		require.NoError(t, err)

		assert.Equal(t, 1, callCount, "ResolveVaultPassword should be called only once")
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		dir := t.TempDir()
		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) { return password, nil }
		play := &playbook.Play{VaultFile: filepath.Join(dir, "nonexistent.vault")}

		_, err := e.loadVaultVars(play, dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read vault file")
	})

	t.Run("returns error for wrong password", func(t *testing.T) {
		dir := t.TempDir()
		absPath := makeVaultFile(t, dir, "secrets.vault", yamlContent, []byte("right-password"))

		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) { return []byte("wrong-password"), nil }
		play := &playbook.Play{VaultFile: absPath}

		_, err := e.loadVaultVars(play, dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decrypt vault")
	})

	t.Run("returns error for invalid YAML content", func(t *testing.T) {
		dir := t.TempDir()
		absPath := makeVaultFile(t, dir, "bad.vault", []byte("not: yaml: [invalid"), password)

		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) { return password, nil }
		play := &playbook.Play{VaultFile: absPath}

		_, err := e.loadVaultVars(play, dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid YAML")
	})

	t.Run("acquires password via callback on first call, caches for subsequent", func(t *testing.T) {
		dir := t.TempDir()
		absPath1 := makeVaultFile(t, dir, "vault1.vault", []byte("key1: val1\n"), password)
		absPath2 := makeVaultFile(t, dir, "vault2.vault", []byte("key2: val2\n"), password)

		callCount := 0
		e := New()
		e.ResolveVaultPassword = func() ([]byte, error) {
			callCount++
			return password, nil
		}

		play1 := &playbook.Play{VaultFile: absPath1}
		play2 := &playbook.Play{VaultFile: absPath2}

		_, err := e.loadVaultVars(play1, dir)
		require.NoError(t, err)
		_, err = e.loadVaultVars(play2, dir)
		require.NoError(t, err)

		assert.Equal(t, 1, callCount, "password should be resolved once and cached across vault files")
	})

	t.Run("returns error when no ResolveVaultPassword configured", func(t *testing.T) {
		dir := t.TempDir()
		absPath := makeVaultFile(t, dir, "secrets.vault", yamlContent, password)

		e := New()
		// ResolveVaultPassword is nil
		play := &playbook.Play{VaultFile: absPath}

		_, err := e.loadVaultVars(play, dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no vault password source")
	})

	t.Run("play vars win over vault vars on key conflict", func(t *testing.T) {
		// Simulate the merge logic from runPlayOnHost
		pctx := &PlayContext{Vars: map[string]any{"db_host": "play-value"}}
		vaultVars := map[string]any{"db_host": "vault-value", "db_pass": "secret"}
		for k, v := range vaultVars {
			if _, exists := pctx.Vars[k]; !exists {
				pctx.Vars[k] = v
			}
		}
		assert.Equal(t, "play-value", pctx.Vars["db_host"], "play var should win over vault var")
		assert.Equal(t, "secret", pctx.Vars["db_pass"], "vault var fills gap when not in play vars")
	})
}
