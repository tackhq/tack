package yum

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the yum module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	stateStr := module.GetString(params, "state", "present")
	state := State(stateStr)
	updateCache := module.GetBool(params, "update_cache", false)
	upgrade := module.GetString(params, "upgrade", "none")
	autoremove := module.GetBool(params, "autoremove", false)
	names := module.GetStringSlice(params, "name")

	// Use yum by default; script can override with dnf if available
	pkgMgr := "yum"

	var lines []string

	// Detect dnf vs yum at runtime
	lines = append(lines, "if command -v dnf >/dev/null 2>&1; then _tack_pkg=dnf; else _tack_pkg=yum; fi")

	if updateCache {
		lines = append(lines, "${_tack_pkg} makecache -q")
	}

	if upgrade == "yes" {
		lines = append(lines, "${_tack_pkg} upgrade -y -q")
	}

	for _, name := range names {
		qname := connector.ShellQuote(name)
		switch state {
		case StatePresent:
			lines = append(lines, fmt.Sprintf("if ! rpm -q %s >/dev/null 2>&1; then", qname))
			lines = append(lines, fmt.Sprintf("  ${_tack_pkg} install -y -q %s", qname))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		case StateAbsent:
			lines = append(lines, fmt.Sprintf("if rpm -q %s >/dev/null 2>&1; then", qname))
			lines = append(lines, fmt.Sprintf("  ${_tack_pkg} remove -y -q %s", qname))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		case StateLatest:
			lines = append(lines, fmt.Sprintf("if ! rpm -q %s >/dev/null 2>&1; then", qname))
			lines = append(lines, fmt.Sprintf("  ${_tack_pkg} install -y -q %s", qname))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "else")
			lines = append(lines, fmt.Sprintf("  ${_tack_pkg} update -y -q %s", qname))
			lines = append(lines, "fi")
		}
	}

	if autoremove {
		lines = append(lines, "${_tack_pkg} autoremove -y -q")
	}

	_ = pkgMgr
	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
