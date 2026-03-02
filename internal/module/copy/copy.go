// Package copy provides a module for copying files to target systems.
package copy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
	"github.com/eugenetaranov/bolt/internal/module"
)

func init() {
	module.Register(&Module{})
}

// Module copies files to the target system.
type Module struct{}

// Name returns the module identifier.
func (m *Module) Name() string {
	return "copy"
}

// Run executes the copy module.
//
// Parameters:
//   - dest (string, required): Destination path on the target
//   - src (string): Source file path on the controller (mutually exclusive with content)
//   - content (string): Inline content to write (mutually exclusive with src)
//   - mode (string): File permissions in octal (e.g., "0644")
//   - owner (string): Owner username
//   - group (string): Group name
//   - backup (bool): Create backup before overwriting (default: false)
//   - force (bool): Overwrite even if destination exists (default: true)
//   - create_dirs (bool): Create parent directories if needed (default: false)
//   - validate (string): Command to validate file before finalizing (%s = temp file path)
func (m *Module) Run(ctx context.Context, conn connector.Connector, params map[string]any) (*module.Result, error) {
	// Extract parameters
	dest, err := module.RequireString(params, "dest")
	if err != nil {
		return nil, err
	}

	src := module.GetString(params, "src", "")
	content := module.GetString(params, "content", "")
	mode := module.GetString(params, "mode", "0644")
	owner := module.GetString(params, "owner", "")
	group := module.GetString(params, "group", "")
	backup := module.GetBool(params, "backup", false)
	force := module.GetBool(params, "force", true)
	createDirs := module.GetBool(params, "create_dirs", false)
	validate := module.GetString(params, "validate", "")

	// Validate parameters
	if src == "" && content == "" {
		return nil, fmt.Errorf("either 'src' or 'content' parameter is required")
	}
	if src != "" && content != "" {
		return nil, fmt.Errorf("'src' and 'content' are mutually exclusive")
	}

	// Get source content
	var srcContent []byte
	if src != "" {
		srcPath := module.ResolveRolePath(src, params, "files")

		// Read from local file
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source file '%s': %w", srcPath, err)
		}
		srcContent = data
	} else {
		srcContent = []byte(content)
	}

	// Calculate checksum of source
	srcChecksum := module.Checksum(srcContent)

	// Check if destination exists and compare checksums
	destExists, destChecksum, err := module.GetRemoteChecksum(ctx, conn, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to check destination: %w", err)
	}

	// If destination exists with same content, check if we need to update mode/owner
	if destExists && srcChecksum == destChecksum {
		// File content matches, check attributes
		attrChanged, err := module.EnsureAttributes(ctx, conn, dest, mode, owner, group)
		if err != nil {
			return nil, err
		}
		if attrChanged {
			return module.Changed("attributes updated"), nil
		}
		return module.Unchanged("file already exists with correct content and attributes"), nil
	}

	// If destination exists and force=false, skip
	if destExists && !force {
		return module.Unchanged("destination exists and force=false"), nil
	}

	// Create parent directories if needed
	if createDirs {
		if err := createParentDirs(ctx, conn, dest); err != nil {
			return nil, err
		}
	}

	// Create backup if needed
	if destExists && backup {
		if err := module.CreateBackup(ctx, conn, dest); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Upload to temp file first if validation is needed
	targetPath := dest
	if validate != "" {
		targetPath = fmt.Sprintf("/tmp/bolt-copy-%d", time.Now().UnixNano())
	}

	// Upload the file
	modeInt, err := module.ParseMode(mode)
	if err != nil {
		return nil, fmt.Errorf("invalid mode: %w", err)
	}

	if err := conn.Upload(ctx, bytes.NewReader(srcContent), targetPath, modeInt); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	// Run validation if specified
	if validate != "" {
		validateCmd := strings.ReplaceAll(validate, "%s", connector.ShellQuote(targetPath))
		result, err := conn.Execute(ctx, validateCmd)
		if err != nil {
			// Clean up temp file (ignore error)
			_, _ = conn.Execute(ctx, fmt.Sprintf("rm -f %s", connector.ShellQuote(targetPath)))
			return nil, fmt.Errorf("validation command failed: %w", err)
		}
		if result.ExitCode != 0 {
			// Clean up temp file (ignore error)
			_, _ = conn.Execute(ctx, fmt.Sprintf("rm -f %s", connector.ShellQuote(targetPath)))
			return nil, fmt.Errorf("validation failed: %s", result.Stderr)
		}

		// Move temp file to destination
		result, err = conn.Execute(ctx, fmt.Sprintf("mv %s %s", connector.ShellQuote(targetPath), connector.ShellQuote(dest)))
		if err != nil {
			return nil, fmt.Errorf("failed to move validated file: %w", err)
		}
		if result.ExitCode != 0 {
			return nil, fmt.Errorf("failed to move validated file: %s", result.Stderr)
		}
	}

	// Set attributes
	if _, err := module.EnsureAttributes(ctx, conn, dest, mode, owner, group); err != nil {
		return nil, err
	}

	var msg string
	if destExists {
		msg = "file updated"
	} else {
		msg = "file created"
	}

	return module.ChangedWithData(msg, map[string]any{
		"dest":     dest,
		"checksum": srcChecksum,
	}), nil
}

// createParentDirs creates parent directories for a path.
func createParentDirs(ctx context.Context, conn connector.Connector, path string) error {
	// Extract directory from path
	cmd := fmt.Sprintf("mkdir -p \"$(dirname %s)\"", connector.ShellQuote(path))
	result, err := conn.Execute(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("mkdir failed: %s", result.Stderr)
	}
	return nil
}

// Check determines whether the copy module would make changes without applying them.
func (m *Module) Check(ctx context.Context, conn connector.Connector, params map[string]any) (*module.CheckResult, error) {
	dest, err := module.RequireString(params, "dest")
	if err != nil {
		return nil, err
	}

	src := module.GetString(params, "src", "")
	content := module.GetString(params, "content", "")
	mode := module.GetString(params, "mode", "0644")
	owner := module.GetString(params, "owner", "")
	group := module.GetString(params, "group", "")
	force := module.GetBool(params, "force", true)

	if src == "" && content == "" {
		return nil, fmt.Errorf("either 'src' or 'content' parameter is required")
	}
	if src != "" && content != "" {
		return nil, fmt.Errorf("'src' and 'content' are mutually exclusive")
	}

	var srcContent []byte
	if src != "" {
		srcPath := module.ResolveRolePath(src, params, "files")
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source file '%s': %w", srcPath, err)
		}
		srcContent = data
	} else {
		srcContent = []byte(content)
	}

	srcChecksum := module.Checksum(srcContent)

	destExists, destChecksum, err := module.GetRemoteChecksum(ctx, conn, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to check destination: %w", err)
	}

	if !destExists {
		cr := module.WouldChange("file does not exist")
		cr.NewChecksum = srcChecksum
		cr.NewContent = string(srcContent)
		return cr, nil
	}

	if destExists && !force {
		return module.NoChange("destination exists and force=false"), nil
	}

	if srcChecksum != destChecksum {
		cr := module.WouldChange("content differs")
		cr.OldChecksum = destChecksum
		cr.NewChecksum = srcChecksum
		cr.NewContent = string(srcContent)
		// Fetch old content for diff
		result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", connector.ShellQuote(dest)))
		if err == nil && result.ExitCode == 0 {
			cr.OldContent = result.Stdout
		}
		return cr, nil
	}

	// Content matches, check attributes
	attrDiffer, err := module.CheckAttributes(ctx, conn, dest, mode, owner, group)
	if err != nil {
		return nil, err
	}
	if attrDiffer {
		return module.WouldChange("attributes differ"), nil
	}

	return module.NoChange("file already exists with correct content and attributes"), nil
}

// Ensure Module implements the module.Module interface.
var _ module.Module = (*Module)(nil)

// Ensure Module implements the module.Checker interface.
var _ module.Checker = (*Module)(nil)
