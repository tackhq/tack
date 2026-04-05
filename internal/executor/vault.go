package executor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tackhq/tack/internal/playbook"
	"github.com/tackhq/tack/internal/vault"
	"gopkg.in/yaml.v3"
)

// loadVaultVars loads and decrypts a vault file, returning its variables.
// Results are cached by resolved file path to avoid repeated Argon2id runs.
// The password is resolved lazily on first call via e.ResolveVaultPassword
// and cached on the Executor for the run duration (D-10, D-11).
func (e *Executor) loadVaultVars(play *playbook.Play, playbookDir string) (map[string]any, error) {
	// Resolve path relative to playbook directory
	vaultPath := play.VaultFile
	if !filepath.IsAbs(vaultPath) {
		vaultPath = filepath.Join(playbookDir, vaultPath)
	}

	// Check var cache first (D-10)
	if cached, ok := e.vaultVarCache[vaultPath]; ok {
		return cached, nil
	}

	// Acquire password lazily (D-11 — prompted once per run)
	if e.vaultPassword == nil {
		if e.ResolveVaultPassword == nil {
			return nil, fmt.Errorf("play references vault_file but no vault password source configured")
		}
		pw, err := e.ResolveVaultPassword()
		if err != nil {
			return nil, fmt.Errorf("acquire vault password: %w", err)
		}
		e.vaultPassword = pw
	}

	// Read encrypted file (D-07 — missing file is fatal)
	data, err := os.ReadFile(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("read vault file %q: %w", vaultPath, err)
	}

	// Decrypt (D-08 — wrong password is fatal)
	plaintext, err := vault.Decrypt(data, e.vaultPassword)
	if err != nil {
		return nil, fmt.Errorf("decrypt vault %q: %w", vaultPath, err)
	}
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	// Parse YAML (D-09 — bad YAML distinguished from wrong password)
	var vars map[string]any
	if err := yaml.Unmarshal(plaintext, &vars); err != nil {
		return nil, fmt.Errorf("vault file %q contains invalid YAML (check password or file integrity): %w", vaultPath, err)
	}
	if vars == nil {
		vars = make(map[string]any)
	}

	// Cache for subsequent plays (D-10)
	e.vaultVarCache[vaultPath] = vars

	return vars, nil
}
