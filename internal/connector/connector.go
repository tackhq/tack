// Package connector defines the interface for executing commands on target systems.
package connector

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Result holds the output from command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Connector is the interface for connecting to and executing commands on targets.
type Connector interface {
	// Connect establishes a connection to the target.
	Connect(ctx context.Context) error

	// Execute runs a command on the target and returns the result.
	Execute(ctx context.Context, cmd string) (*Result, error)

	// Upload copies a file from local source to remote destination.
	Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error

	// Download copies a file from remote source to local destination.
	Download(ctx context.Context, src string, dst io.Writer) error

	// SetSudo enables or disables sudo for subsequent commands.
	SetSudo(enabled bool, password string)

	// Close terminates the connection.
	Close() error

	// String returns a human-readable description of the connection.
	String() string
}

// Run executes a command and returns an error if the command fails (non-zero exit code).
// Returns the Result so callers needing stdout can use it.
func Run(ctx context.Context, conn Connector, cmd string) (*Result, error) {
	result, err := conn.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return result, fmt.Errorf("%s", strings.TrimSpace(result.Stderr))
	}
	return result, nil
}

// ShellQuote wraps a string in single quotes for safe shell usage.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// BuildSudoCommand wraps a command with sudo if enabled, skipping when already root.
func BuildSudoCommand(cmd string, sudoEnabled bool, password string, isRoot bool) string {
	if !sudoEnabled || isRoot {
		return cmd
	}

	escaped := strings.ReplaceAll(cmd, "'", "'\"'\"'")
	if password != "" {
		return fmt.Sprintf("printf '%%s\\n' '%s' | sudo -S sh -c '%s'", password, escaped)
	}
	return fmt.Sprintf("sudo sh -c '%s'", escaped)
}

// Config holds common configuration for connectors.
type Config struct {
	// Host is the target hostname or IP address.
	Host string

	// User is the username for authentication.
	User string

	// Timeout is the connection timeout in seconds.
	Timeout int
}
