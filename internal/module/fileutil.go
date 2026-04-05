package module

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/tackhq/tack/internal/connector"
)

// Checksum calculates SHA256 checksum of data.
func Checksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// GetRemoteChecksum gets the SHA256 checksum of a remote file.
func GetRemoteChecksum(ctx context.Context, conn connector.Connector, path string) (exists bool, sum string, err error) {
	cmd := fmt.Sprintf(`if [ -f %[1]s ]; then
		if command -v sha256sum >/dev/null 2>&1; then
			sha256sum %[1]s | cut -d' ' -f1
		elif command -v shasum >/dev/null 2>&1; then
			shasum -a 256 %[1]s | cut -d' ' -f1
		else
			echo "NO_SHA"
		fi
	else
		echo "NO_FILE"
	fi`, connector.ShellQuote(path))

	result, err := conn.Execute(ctx, cmd)
	if err != nil {
		return false, "", err
	}

	output := strings.TrimSpace(result.Stdout)
	switch output {
	case "NO_FILE":
		return false, "", nil
	case "NO_SHA":
		return true, "", nil
	default:
		return true, output, nil
	}
}

// ParseMode converts an octal mode string to uint32.
func ParseMode(mode string) (uint32, error) {
	mode = strings.TrimLeft(mode, "0")
	if mode == "" {
		mode = "0"
	}

	var m uint32
	_, err := fmt.Sscanf("0"+mode, "%o", &m)
	if err != nil {
		return 0, err
	}
	return m, nil
}

// GetFileAttributes returns the mode, owner, and group of a file.
func GetFileAttributes(ctx context.Context, conn connector.Connector, path string) (mode, owner, group string, err error) {
	cmd := fmt.Sprintf(`stat -c '%%a %%U %%G' %[1]s 2>/dev/null || stat -f '%%Lp %%Su %%Sg' %[1]s`, connector.ShellQuote(path))

	result, err := connector.Run(ctx, conn, cmd)
	if err != nil {
		return "", "", "", fmt.Errorf("stat failed: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(result.Stdout))
	if len(parts) >= 3 {
		mode = parts[0]
		if len(mode) < 4 {
			mode = strings.Repeat("0", 4-len(mode)) + mode
		}
		owner = parts[1]
		group = parts[2]
	}

	return mode, owner, group, nil
}

// EnsureAttributes sets mode and ownership on a file, only if they differ from desired.
// When recurse is true, uses -R flags and always applies (skips per-file idempotency check).
func EnsureAttributes(ctx context.Context, conn connector.Connector, path, mode, owner, group string, recurse bool) (bool, error) {
	var changed bool

	if !recurse {
		currentMode, currentOwner, currentGroup, err := GetFileAttributes(ctx, conn, path)
		if err != nil {
			return false, fmt.Errorf("failed to get file attributes: %w", err)
		}

		if mode != "" && NormalizeMode(currentMode) != NormalizeMode(mode) {
			if _, err := connector.Run(ctx, conn, fmt.Sprintf("chmod %s %s", connector.ShellQuote(mode), connector.ShellQuote(path))); err != nil {
				return false, fmt.Errorf("failed to set mode: %w", err)
			}
			changed = true
		}

		needOwnerChange := owner != "" && currentOwner != owner
		needGroupChange := group != "" && currentGroup != group

		if needOwnerChange || needGroupChange {
			ownership := buildOwnership(owner, group)
			if _, err := connector.Run(ctx, conn, fmt.Sprintf("chown %s %s", connector.ShellQuote(ownership), connector.ShellQuote(path))); err != nil {
				return false, fmt.Errorf("failed to set ownership: %w", err)
			}
			changed = true
		}

		return changed, nil
	}

	// Recursive mode: always apply, skip idempotency check
	if mode != "" {
		if _, err := connector.Run(ctx, conn, fmt.Sprintf("chmod -R %s %s", connector.ShellQuote(mode), connector.ShellQuote(path))); err != nil {
			return false, fmt.Errorf("failed to set mode: %w", err)
		}
		changed = true
	}

	if owner != "" || group != "" {
		ownership := buildOwnership(owner, group)
		if _, err := connector.Run(ctx, conn, fmt.Sprintf("chown -R %s %s", connector.ShellQuote(ownership), connector.ShellQuote(path))); err != nil {
			return false, fmt.Errorf("failed to set ownership: %w", err)
		}
		changed = true
	}

	return changed, nil
}

// buildOwnership formats an owner:group string for chown.
func buildOwnership(owner, group string) string {
	if owner != "" && group != "" {
		return fmt.Sprintf("%s:%s", owner, group)
	} else if owner != "" {
		return owner
	}
	return fmt.Sprintf(":%s", group)
}

// CheckAttributes checks whether file attributes differ from desired values without modifying them.
func CheckAttributes(ctx context.Context, conn connector.Connector, path, mode, owner, group string) (bool, error) {
	currentMode, currentOwner, currentGroup, err := GetFileAttributes(ctx, conn, path)
	if err != nil {
		return false, fmt.Errorf("failed to get file attributes: %w", err)
	}

	if mode != "" && NormalizeMode(currentMode) != NormalizeMode(mode) {
		return true, nil
	}
	if owner != "" && currentOwner != owner {
		return true, nil
	}
	if group != "" && currentGroup != group {
		return true, nil
	}
	return false, nil
}

// CreateBackup creates a timestamped backup of a file.
func CreateBackup(ctx context.Context, conn connector.Connector, path string) error {
	timestamp := time.Now().Format("20060102150405")
	backupPath := fmt.Sprintf("%s.%s.bak", path, timestamp)

	if _, err := connector.Run(ctx, conn, fmt.Sprintf("cp -p %s %s", connector.ShellQuote(path), connector.ShellQuote(backupPath))); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	return nil
}

// DeployOpts configures a file deployment operation.
type DeployOpts struct {
	Content []byte
	Dest    string
	Mode    string
	Owner   string
	Group   string
	Backup  bool
	Label   string // "file" or "template" for messages
}

// DeployFile handles the common checksum-compare, backup, upload, ensure-attributes
// logic shared by copy and template modules.
func DeployFile(ctx context.Context, conn connector.Connector, opts DeployOpts) (*Result, error) {
	srcChecksum := Checksum(opts.Content)

	destExists, destChecksum, err := GetRemoteChecksum(ctx, conn, opts.Dest)
	if err != nil {
		return nil, fmt.Errorf("failed to check destination: %w", err)
	}

	// If destination exists with same content, check if we need to update attributes
	if destExists && srcChecksum == destChecksum {
		attrChanged, err := EnsureAttributes(ctx, conn, opts.Dest, opts.Mode, opts.Owner, opts.Group, false)
		if err != nil {
			return nil, err
		}
		if attrChanged {
			return Changed("attributes updated"), nil
		}
		return Unchanged(opts.Label + " already exists with correct content and attributes"), nil
	}

	// Create backup if needed
	if destExists && opts.Backup {
		if err := CreateBackup(ctx, conn, opts.Dest); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Upload the file
	modeInt, err := ParseMode(opts.Mode)
	if err != nil {
		return nil, fmt.Errorf("invalid mode: %w", err)
	}

	if err := conn.Upload(ctx, bytes.NewReader(opts.Content), opts.Dest, modeInt); err != nil {
		return nil, fmt.Errorf("failed to upload %s: %w", opts.Label, err)
	}

	// Set attributes
	if _, err := EnsureAttributes(ctx, conn, opts.Dest, opts.Mode, opts.Owner, opts.Group, false); err != nil {
		return nil, err
	}

	msg := opts.Label + " created"
	if destExists {
		msg = opts.Label + " updated"
	}

	return ChangedWithData(msg, map[string]any{
		"dest":     opts.Dest,
		"checksum": srcChecksum,
	}), nil
}

// CheckOptions configures behavior of the check phase.
type CheckOptions struct {
	// DiffEnabled controls whether remote file content is fetched for diff display.
	// When false, only checksums are compared (faster, no cat over the wire).
	DiffEnabled bool
}

// CheckDeployFile checks whether a file deployment would make changes without applying them.
// If opts is nil, defaults to fetching content (backward compatible).
func CheckDeployFile(ctx context.Context, conn connector.Connector, content []byte, dest, mode, owner, group string, opts ...CheckOptions) (*CheckResult, error) {
	fetchContent := true
	if len(opts) > 0 {
		fetchContent = opts[0].DiffEnabled
	}
	srcChecksum := Checksum(content)

	destExists, destChecksum, err := GetRemoteChecksum(ctx, conn, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to check destination: %w", err)
	}

	if !destExists {
		cr := WouldChange("file does not exist")
		cr.NewChecksum = srcChecksum
		cr.NewContent = string(content)
		return cr, nil
	}

	if srcChecksum != destChecksum {
		cr := WouldChange("content differs")
		cr.OldChecksum = destChecksum
		cr.NewChecksum = srcChecksum
		cr.NewContent = string(content)
		// Fetch old content for diff only when diff/verbose mode is active
		if fetchContent {
			result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", connector.ShellQuote(dest)))
			if err == nil && result.ExitCode == 0 {
				cr.OldContent = result.Stdout
			}
		}
		return cr, nil
	}

	// Content matches, check attributes
	attrDiffer, err := CheckAttributes(ctx, conn, dest, mode, owner, group)
	if err != nil {
		return nil, err
	}
	if attrDiffer {
		return WouldChange("attributes differ"), nil
	}

	return NoChange("file already exists with correct content and attributes"), nil
}
