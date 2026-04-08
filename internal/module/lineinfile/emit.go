package lineinfile

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the lineinfile module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	path, err := module.RequireString(params, "path")
	if err != nil {
		return nil, err
	}

	stateStr := module.GetString(params, "state", "present")
	line := module.GetString(params, "line", "")
	regexp := module.GetString(params, "regexp", "")
	create := module.GetBool(params, "create", false)
	backup := module.GetBool(params, "backup", false)

	qpath := connector.ShellQuote(path)

	var lines []string

	if backup {
		lines = append(lines, fmt.Sprintf("[ -f %s ] && cp -p %s %s.bak", qpath, qpath, qpath))
	}

	switch stateStr {
	case "present":
		if line == "" {
			return nil, fmt.Errorf("'line' parameter is required when state=present")
		}
		qline := connector.ShellQuote(line)

		if create {
			lines = append(lines, fmt.Sprintf("[ -f %s ] || touch %s", qpath, qpath))
		}

		if regexp != "" {
			// Replace matching line, or append if no match
			qregexp := connector.ShellQuote(regexp)
			lines = append(lines, fmt.Sprintf("if grep -qE %s %s 2>/dev/null; then", qregexp, qpath))
			// Use sed to replace — escape sed delimiters in the line
			sedLine := strings.ReplaceAll(line, "/", "\\/")
			sedLine = strings.ReplaceAll(sedLine, "&", "\\&")
			lines = append(lines, fmt.Sprintf("  sed -i'' -E 's/%s.*/%s/' %s", strings.ReplaceAll(regexp, "/", "\\/"), sedLine, qpath))
			lines = append(lines, "else")
			lines = append(lines, fmt.Sprintf("  echo %s >> %s", qline, qpath))
			lines = append(lines, "fi")
		} else {
			// Ensure line is present
			lines = append(lines, fmt.Sprintf("if ! grep -qF %s %s 2>/dev/null; then", qline, qpath))
			lines = append(lines, fmt.Sprintf("  echo %s >> %s", qline, qpath))
			lines = append(lines, "fi")
		}
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")

	case "absent":
		if regexp != "" {
			qregexp := connector.ShellQuote(regexp)
			lines = append(lines, fmt.Sprintf("if grep -qE %s %s 2>/dev/null; then", qregexp, qpath))
			lines = append(lines, fmt.Sprintf("  sed -i'' -E '/%s/d' %s", strings.ReplaceAll(regexp, "/", "\\/"), qpath))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		} else if line != "" {
			qline := connector.ShellQuote(line)
			lines = append(lines, fmt.Sprintf("if grep -qF %s %s 2>/dev/null; then", qline, qpath))
			lines = append(lines, fmt.Sprintf("  grep -vF %s %s > %s.tmp && mv %s.tmp %s", qline, qpath, qpath, qpath, qpath))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		}
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
