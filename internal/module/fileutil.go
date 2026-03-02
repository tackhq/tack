package module

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
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

	result, err := conn.Execute(ctx, cmd)
	if err != nil {
		return "", "", "", err
	}
	if result.ExitCode != 0 {
		return "", "", "", fmt.Errorf("stat failed: %s", result.Stderr)
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
func EnsureAttributes(ctx context.Context, conn connector.Connector, path, mode, owner, group string) (bool, error) {
	var changed bool

	currentMode, currentOwner, currentGroup, err := GetFileAttributes(ctx, conn, path)
	if err != nil {
		return false, fmt.Errorf("failed to get file attributes: %w", err)
	}

	if mode != "" && NormalizeMode(currentMode) != NormalizeMode(mode) {
		result, err := conn.Execute(ctx, fmt.Sprintf("chmod %s %s", connector.ShellQuote(mode), connector.ShellQuote(path)))
		if err != nil {
			return false, fmt.Errorf("failed to set mode: %w", err)
		}
		if result.ExitCode != 0 {
			return false, fmt.Errorf("chmod failed: %s", result.Stderr)
		}
		changed = true
	}

	needOwnerChange := owner != "" && currentOwner != owner
	needGroupChange := group != "" && currentGroup != group

	if needOwnerChange || needGroupChange {
		var ownership string
		if owner != "" && group != "" {
			ownership = fmt.Sprintf("%s:%s", owner, group)
		} else if owner != "" {
			ownership = owner
		} else {
			ownership = fmt.Sprintf(":%s", group)
		}

		result, err := conn.Execute(ctx, fmt.Sprintf("chown %s %s", connector.ShellQuote(ownership), connector.ShellQuote(path)))
		if err != nil {
			return false, fmt.Errorf("failed to set ownership: %w", err)
		}
		if result.ExitCode != 0 {
			return false, fmt.Errorf("chown failed: %s", result.Stderr)
		}
		changed = true
	}

	return changed, nil
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

	result, err := conn.Execute(ctx, fmt.Sprintf("cp -p %s %s", connector.ShellQuote(path), connector.ShellQuote(backupPath)))
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("backup failed: %s", result.Stderr)
	}
	return nil
}
