// Package script implements an inventory plugin that runs an executable
// with --list and parses its stdout as JSON or YAML inventory data.
package script

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/eugenetaranov/bolt/internal/inventory"
)

func init() {
	inventory.RegisterPlugin(&Plugin{})
}

// Plugin implements the inventory.Plugin interface for script-based inventory.
type Plugin struct{}

func (p *Plugin) Name() string { return "script" }

func (p *Plugin) Load(ctx context.Context, config map[string]any) (*inventory.Inventory, error) {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("script plugin: missing required 'path' config")
	}

	cmd := exec.CommandContext(ctx, path, "--list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("script plugin: timed out executing %s", path)
		}
		stderrStr := stderr.String()
		if stderrStr != "" {
			return nil, fmt.Errorf("script plugin: %s failed: %w\nstderr: %s", path, err, stderrStr)
		}
		return nil, fmt.Errorf("script plugin: %s failed: %w", path, err)
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("script plugin: %s produced no output", path)
	}

	inv, err := inventory.ParseInventoryData(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("script plugin: failed to parse output from %s: %w", path, err)
	}

	return inv, nil
}
