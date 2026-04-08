package group

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the group module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	name, err := module.RequireString(params, "name")
	if err != nil {
		return nil, err
	}

	state := module.GetString(params, "state", "present")
	gid := module.GetInt(params, "gid", 0)
	system := module.GetBool(params, "system", false)

	qname := connector.ShellQuote(name)
	var lines []string

	switch state {
	case "present":
		lines = append(lines, fmt.Sprintf("if ! getent group %s >/dev/null 2>&1; then", qname))
		addCmd := "groupadd"
		if system {
			addCmd += " -r"
		}
		if gid != 0 {
			addCmd += fmt.Sprintf(" -g %d", gid)
		}
		addCmd += " " + qname
		lines = append(lines, "  "+addCmd)
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "else")
		if gid != 0 {
			lines = append(lines, fmt.Sprintf("  _tack_gid=$(getent group %s | cut -d: -f3)", qname))
			lines = append(lines, fmt.Sprintf("  if [ \"$_tack_gid\" != \"%d\" ]; then", gid))
			lines = append(lines, fmt.Sprintf("    groupmod -g %d %s", gid, qname))
			lines = append(lines, "    TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "  fi")
		}
		lines = append(lines, "fi")

	case "absent":
		lines = append(lines, fmt.Sprintf("if getent group %s >/dev/null 2>&1; then", qname))
		lines = append(lines, fmt.Sprintf("  groupdel %s", qname))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
