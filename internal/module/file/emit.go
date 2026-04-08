package file

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the file module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	path, err := module.RequireString(params, "path")
	if err != nil {
		return nil, err
	}

	stateStr := module.GetString(params, "state", "file")
	state := State(stateStr)
	mode := module.GetString(params, "mode", "")
	owner := module.GetString(params, "owner", "")
	group := module.GetString(params, "group", "")
	src := module.GetString(params, "src", "")
	recurse := module.GetBool(params, "recurse", false)
	force := module.GetBool(params, "force", false)

	qpath := connector.ShellQuote(path)
	var lines []string

	switch state {
	case StateDirectory:
		lines = append(lines, fmt.Sprintf("if [ ! -d %s ]; then", qpath))
		lines = append(lines, fmt.Sprintf("  mkdir -p %s", qpath))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")

	case StateTouch:
		lines = append(lines, fmt.Sprintf("touch %s", qpath))
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")

	case StateAbsent:
		lines = append(lines, fmt.Sprintf("if [ -e %s ]; then", qpath))
		lines = append(lines, fmt.Sprintf("  rm -rf %s", qpath))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")

	case StateLink:
		qsrc := connector.ShellQuote(src)
		if force {
			lines = append(lines, fmt.Sprintf("if [ ! -L %s ] || [ \"$(readlink %s)\" != %s ]; then", qpath, qpath, qsrc))
			lines = append(lines, fmt.Sprintf("  ln -sf %s %s", qsrc, qpath))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		} else {
			lines = append(lines, fmt.Sprintf("if [ ! -L %s ]; then", qpath))
			lines = append(lines, fmt.Sprintf("  ln -s %s %s", qsrc, qpath))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		}

	case StateFile:
		// Just ensure attributes
		lines = append(lines, fmt.Sprintf("test -e %s", qpath))
	}

	// Mode
	if mode != "" {
		mode = module.NormalizeMode(mode)
		flag := ""
		if recurse {
			flag = "-R "
		}
		lines = append(lines, fmt.Sprintf("chmod %s%s %s", flag, mode, qpath))
	}

	// Owner/group
	if owner != "" || group != "" {
		ownership := owner
		if group != "" {
			ownership += ":" + group
		}
		flag := ""
		if recurse {
			flag = "-R "
		}
		lines = append(lines, fmt.Sprintf("chown %s%s %s", flag, connector.ShellQuote(ownership), qpath))
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
