package template

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the template module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
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

	// Get template variables (injected by executor / export compiler)
	templateVars := module.GetMap(params, "_template_vars")
	if templateVars == nil {
		templateVars = vars
	}

	templatePath := module.ResolveRolePath(src, params, "templates")

	// Read template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("reading template file %q: %w", templatePath, err)
	}

	// Render template at export time
	rendered, err := renderTemplateForEmit(src, string(templateContent), templateVars)
	if err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}

	qdest := connector.ShellQuote(dest)
	tmpDest := dest + ".tack.tmp"
	qtmp := connector.ShellQuote(tmpDest)

	var lines []string

	// Write rendered content via heredoc
	if !strings.Contains(rendered, "TACK_EOF") {
		lines = append(lines, fmt.Sprintf("cat > %s <<'TACK_EOF'", qtmp))
		lines = append(lines, rendered)
		lines = append(lines, "TACK_EOF")
	} else {
		// Fallback to a different delimiter
		lines = append(lines, fmt.Sprintf("cat > %s <<'TACK_TEMPLATE_EOF'", qtmp))
		lines = append(lines, rendered)
		lines = append(lines, "TACK_TEMPLATE_EOF")
	}

	// Diff-guard
	lines = append(lines, fmt.Sprintf("if ! diff -q %s %s >/dev/null 2>&1; then", qdest, qtmp))
	if backup {
		lines = append(lines, fmt.Sprintf("  [ -f %s ] && cp %s %s.bak", qdest, qdest, qdest))
	}
	lines = append(lines, fmt.Sprintf("  mv %s %s", qtmp, qdest))
	lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
	lines = append(lines, "else")
	lines = append(lines, fmt.Sprintf("  rm -f %s", qtmp))
	lines = append(lines, "fi")

	// Mode
	if mode != "" {
		mode = module.NormalizeMode(mode)
		lines = append(lines, fmt.Sprintf("chmod %s %s", mode, qdest))
	}

	// Owner/group
	if owner != "" || group != "" {
		ownership := owner
		if group != "" {
			ownership += ":" + group
		}
		lines = append(lines, fmt.Sprintf("chown %s %s", connector.ShellQuote(ownership), qdest))
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

// renderTemplateForEmit renders a Go template without SSM param support.
func renderTemplateForEmit(name, content string, vars map[string]any) (string, error) {
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
		"ssm_param": func(path string) (string, error) {
			return "", fmt.Errorf("ssm_param is not available during export (template %s)", name)
		},
	})

	tmpl, err := tmpl.Parse(content)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

var _ module.Emitter = (*Module)(nil)

// suppress unused import warning
var _ = context.Background
