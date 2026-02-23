// Package template provides a module for rendering templates to target systems.
package template

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
	"github.com/eugenetaranov/bolt/internal/module"
)

func init() {
	module.Register(&Module{})
}

// Module renders templates to the target system.
type Module struct{}

// Name returns the module identifier.
func (m *Module) Name() string {
	return "template"
}

// Run executes the template module.
//
// Parameters:
//   - src (string, required): Template file path (relative paths resolve to role's templates/ dir)
//   - dest (string, required): Destination path on the target
//   - mode (string): File permissions in octal (e.g., "0644")
//   - owner (string): Owner username
//   - group (string): Group name
//   - backup (bool): Create backup before overwriting (default: false)
func (m *Module) Run(ctx context.Context, conn connector.Connector, params map[string]any) (*module.Result, error) {
	// Extract parameters
	src, err := module.RequireString(params, "src")
	if err != nil {
		return nil, err
	}

	dest, err := module.RequireString(params, "dest")
	if err != nil {
		return nil, err
	}

	mode := module.GetString(params, "mode", "0644")
	owner := module.GetString(params, "owner", "")
	group := module.GetString(params, "group", "")
	backup := module.GetBool(params, "backup", false)

	// Get template variables (injected by executor)
	templateVars := module.GetMap(params, "_template_vars")

	// Resolve template path - check if it's relative and we have a role path
	templatePath := src
	if !filepath.IsAbs(src) {
		// Check for role path (injected by executor for role tasks)
		if rolePath := module.GetString(params, "_role_path", ""); rolePath != "" {
			// Look in role's templates directory
			roleTemplatePath := filepath.Join(rolePath, "templates", src)
			if _, err := os.Stat(roleTemplatePath); err == nil {
				templatePath = roleTemplatePath
			}
		}
	}

	// Read template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file '%s': %w", templatePath, err)
	}

	// Render template
	renderedContent, err := renderTemplate(src, string(templateContent), templateVars)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Calculate checksum of rendered content
	srcChecksum := module.Checksum(renderedContent)

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
		return module.Unchanged("template already rendered with correct content and attributes"), nil
	}

	// Create backup if needed
	if destExists && backup {
		if err := createBackup(ctx, conn, dest); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Upload the rendered content
	modeInt, err := module.ParseMode(mode)
	if err != nil {
		return nil, fmt.Errorf("invalid mode: %w", err)
	}

	if err := conn.Upload(ctx, bytes.NewReader(renderedContent), dest, modeInt); err != nil {
		return nil, fmt.Errorf("failed to upload rendered template: %w", err)
	}

	// Set attributes
	if _, err := module.EnsureAttributes(ctx, conn, dest, mode, owner, group); err != nil {
		return nil, err
	}

	var msg string
	if destExists {
		msg = "template updated"
	} else {
		msg = "template rendered"
	}

	return module.ChangedWithData(msg, map[string]any{
		"dest":     dest,
		"checksum": srcChecksum,
	}), nil
}

// renderTemplate renders a Go template with the given variables.
func renderTemplate(name, content string, vars map[string]any) ([]byte, error) {
	// Create template with useful functions
	tmpl := template.New(name).Funcs(template.FuncMap{
		"default": func(def, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"trim":  strings.TrimSpace,
		"join": func(sep string, items []any) string {
			strs := make([]string, len(items))
			for i, item := range items {
				strs[i] = fmt.Sprintf("%v", item)
			}
			return strings.Join(strs, sep)
		},
	})

	// Parse the template
	tmpl, err := tmpl.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// createBackup creates a timestamped backup of a file.
func createBackup(ctx context.Context, conn connector.Connector, path string) error {
	timestamp := time.Now().Format("20060102150405")
	backupPath := fmt.Sprintf("%s.%s.bak", path, timestamp)

	result, err := conn.Execute(ctx, fmt.Sprintf("cp -p %s %s", module.ShellQuote(path), module.ShellQuote(backupPath)))
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("backup failed: %s", result.Stderr)
	}
	return nil
}

// Check determines whether the template module would make changes without applying them.
func (m *Module) Check(ctx context.Context, conn connector.Connector, params map[string]any) (*module.CheckResult, error) {
	src, err := module.RequireString(params, "src")
	if err != nil {
		return nil, err
	}

	dest, err := module.RequireString(params, "dest")
	if err != nil {
		return nil, err
	}

	mode := module.GetString(params, "mode", "0644")
	owner := module.GetString(params, "owner", "")
	group := module.GetString(params, "group", "")
	templateVars := module.GetMap(params, "_template_vars")

	templatePath := src
	if !filepath.IsAbs(src) {
		if rolePath := module.GetString(params, "_role_path", ""); rolePath != "" {
			roleTemplatePath := filepath.Join(rolePath, "templates", src)
			if _, err := os.Stat(roleTemplatePath); err == nil {
				templatePath = roleTemplatePath
			}
		}
	}

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file '%s': %w", templatePath, err)
	}

	renderedContent, err := renderTemplate(src, string(templateContent), templateVars)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	srcChecksum := module.Checksum(renderedContent)

	destExists, destChecksum, err := module.GetRemoteChecksum(ctx, conn, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to check destination: %w", err)
	}

	if !destExists {
		cr := module.WouldChange("file does not exist")
		cr.NewChecksum = srcChecksum
		cr.NewContent = string(renderedContent)
		return cr, nil
	}

	if srcChecksum != destChecksum {
		cr := module.WouldChange("content differs")
		cr.OldChecksum = destChecksum
		cr.NewChecksum = srcChecksum
		cr.NewContent = string(renderedContent)
		// Fetch old content for diff
		result, err := conn.Execute(ctx, fmt.Sprintf("cat %s", module.ShellQuote(dest)))
		if err == nil && result.ExitCode == 0 {
			cr.OldContent = result.Stdout
		}
		return cr, nil
	}

	attrDiffer, err := module.CheckAttributes(ctx, conn, dest, mode, owner, group)
	if err != nil {
		return nil, err
	}
	if attrDiffer {
		return module.WouldChange("attributes differ"), nil
	}

	return module.NoChange("template already rendered with correct content and attributes"), nil
}

// Ensure Module implements the module.Module interface.
var _ module.Module = (*Module)(nil)

// Ensure Module implements the module.Checker interface.
var _ module.Checker = (*Module)(nil)
