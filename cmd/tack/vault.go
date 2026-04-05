package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/tackhq/tack/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// vaultCmd is the parent command for vault operations.
var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage encrypted vault files",
}

// vaultInitCmd creates a new encrypted vault file.
var vaultInitCmd = &cobra.Command{
	Use:   "init <file>",
	Short: "Create a new encrypted vault file",
	Args:  cobra.ExactArgs(1),
	RunE:  runVaultInit,
}

// vaultEditCmd edits an existing encrypted vault file.
var vaultEditCmd = &cobra.Command{
	Use:   "edit <file>",
	Short: "Edit an encrypted vault file",
	Args:  cobra.ExactArgs(1),
	RunE:  runVaultEdit,
}

func init() {
	vaultCmd.AddCommand(vaultInitCmd)
	vaultCmd.AddCommand(vaultEditCmd)
	// PersistentFlags so both subcommands inherit via cmd.Flags()
	vaultCmd.PersistentFlags().String("vault-password-file", "", "Path to file containing vault password")
}

// resolveVaultPassword returns a []byte password using the resolution chain:
// TACK_VAULT_PASSWORD env > --vault-password-file flag > interactive prompt.
// confirmPrompt=true prompts twice and verifies match (for vault init).
// When env or file source is used, confirmation is always skipped.
func resolveVaultPassword(cmd *cobra.Command, confirmPrompt bool) ([]byte, error) {
	// 1. Environment variable (highest priority; skip confirmation even if confirmPrompt)
	if envPw := os.Getenv("TACK_VAULT_PASSWORD"); envPw != "" {
		return []byte(envPw), nil
	}

	// 2. Password file flag (inherited via PersistentFlags on parent).
	// Try cmd.Flags() first (works during cobra Execute()), then InheritedFlags()
	// as fallback (works when RunE is called directly in tests).
	vaultPwFile, _ := cmd.Flags().GetString("vault-password-file")
	if vaultPwFile == "" {
		vaultPwFile, _ = cmd.InheritedFlags().GetString("vault-password-file")
	}
	if vaultPwFile != "" {
		data, err := os.ReadFile(vaultPwFile)
		if err != nil {
			return nil, fmt.Errorf("--vault-password-file: %w", err)
		}
		// First line only
		line := strings.SplitN(string(data), "\n", 2)[0]
		return []byte(line), nil
	}

	// 3. Interactive prompt
	fmt.Fprint(os.Stderr, "Enter vault password: ")
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}

	if confirmPrompt {
		fmt.Fprint(os.Stderr, "Confirm vault password: ")
		pw2, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, fmt.Errorf("reading confirmation password: %w", err)
		}
		if !bytes.Equal(pw, pw2) {
			return nil, fmt.Errorf("passwords do not match")
		}
	}

	return pw, nil
}

// launchEditor opens the file at path in $EDITOR (or vi as fallback).
// The editor is attached to the process's stdin/stdout/stderr.
func launchEditor(ctx context.Context, path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// atomicWrite writes data to a temp file in the same directory as dst,
// then renames it atomically to dst. On failure the temp file is removed.
// This ensures the vault file is never left in a partially-written state.
func atomicWrite(dst string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tack-vault-*.tmp")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	tmpName := tmp.Name()
	wrote := false
	defer func() {
		if !wrote {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("atomic write: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomic write: close: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("atomic write: chmod: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	wrote = true
	return nil
}

// runVaultInit implements `tack vault init <file>`.
// It checks that the file does not exist, prompts for a password with confirmation,
// opens $EDITOR with scaffold content, then encrypts and writes the result.
func runVaultInit(cmd *cobra.Command, args []string) error {
	vaultPath := args[0]

	// D-08: Refuse if file already exists
	if _, err := os.Stat(vaultPath); err == nil {
		return fmt.Errorf("file already exists: %s. Use 'tack vault edit' to modify it", vaultPath)
	}

	// D-09/D-06: Resolve password with confirmation for interactive prompts
	pw, err := resolveVaultPassword(cmd, true)
	if err != nil {
		return err
	}
	defer func() {
		for i := range pw {
			pw[i] = 0
		}
	}()

	// D-07: Write scaffold content to temp file in os.TempDir()
	scaffoldContent := "# Add your secrets as YAML key-value pairs below\ndb_password: changeme\n"
	tmpFile, err := os.CreateTemp("", "tack-vault-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// D-11/CLI-04: Wire cleanup to signal context + defer
	ctx, cancel := signalContext(context.Background())
	defer cancel()

	go func() {
		<-ctx.Done()
		os.Remove(tmpPath)
	}()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write([]byte(scaffoldContent)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing scaffold to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Launch editor
	if err := launchEditor(ctx, tmpPath); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Read edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading edited file: %w", err)
	}

	// Encrypt the content
	encrypted, err := vault.Encrypt(edited, pw)
	if err != nil {
		return fmt.Errorf("encrypting vault: %w", err)
	}

	// D-13/CLI-03: Atomic write to target path
	if err := atomicWrite(vaultPath, encrypted, 0600); err != nil {
		return fmt.Errorf("writing vault file: %w", err)
	}

	// D-05: Print success message
	fmt.Fprintln(os.Stderr, "Vault file encrypted successfully.")
	return nil
}

// runVaultEdit implements `tack vault edit <file>`.
// It decrypts the vault, opens $EDITOR, detects no-op changes, and re-encrypts if modified.
func runVaultEdit(cmd *cobra.Command, args []string) error {
	vaultPath := args[0]

	// Read existing vault file
	data, err := os.ReadFile(vaultPath)
	if err != nil {
		return fmt.Errorf("reading vault: %w", err)
	}

	// Resolve password (no confirmation for edit)
	pw, err := resolveVaultPassword(cmd, false)
	if err != nil {
		return err
	}

	// Decrypt vault content
	plaintext, err := vault.Decrypt(data, pw)
	if err != nil {
		// Zero password on decryption failure
		for i := range pw {
			pw[i] = 0
		}
		return fmt.Errorf("decrypting vault: %w", err)
	}

	// D-10/D-03: Write plaintext to temp file in os.TempDir()
	tmpFile, err := os.CreateTemp("", "tack-vault-*.yaml")
	if err != nil {
		for i := range pw {
			pw[i] = 0
		}
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// D-11/CLI-04: Wire cleanup to signal context + defer
	ctx, cancel := signalContext(context.Background())
	defer cancel()

	go func() {
		<-ctx.Done()
		os.Remove(tmpPath)
	}()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(plaintext); err != nil {
		tmpFile.Close()
		for i := range pw {
			pw[i] = 0
		}
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		for i := range pw {
			pw[i] = 0
		}
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Launch editor
	if err := launchEditor(ctx, tmpPath); err != nil {
		// D-02: Non-zero editor exit → abort, keep original vault unchanged
		for i := range pw {
			pw[i] = 0
		}
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Read edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		for i := range pw {
			pw[i] = 0
		}
		return fmt.Errorf("reading edited file: %w", err)
	}

	// D-04/CLI-05: No-op detection — skip re-encryption if content unchanged
	if subtle.ConstantTimeCompare(plaintext, edited) == 1 {
		for i := range pw {
			pw[i] = 0
		}
		fmt.Fprintln(os.Stderr, "No changes detected, vault unchanged.")
		return nil
	}

	// Re-encrypt with fresh salt/nonce
	encrypted, err := vault.Encrypt(edited, pw)
	// Zero password immediately after encrypt
	for i := range pw {
		pw[i] = 0
	}
	if err != nil {
		return fmt.Errorf("encrypting vault: %w", err)
	}

	// D-13/CLI-03: Atomic write to vault path
	if err := atomicWrite(vaultPath, encrypted, 0600); err != nil {
		return fmt.Errorf("writing vault file: %w", err)
	}

	// D-05: Print success message
	fmt.Fprintln(os.Stderr, "Vault file encrypted successfully.")
	return nil
}
