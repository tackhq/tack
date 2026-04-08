package brew

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the brew module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	stateStr := module.GetString(params, "state", "present")
	state := State(stateStr)
	cask := module.GetBool(params, "cask", false)
	upgradeAll := module.GetBool(params, "upgrade_all", false)
	updateHomebrew := module.GetBool(params, "update_homebrew", false)
	options := module.GetStringSlice(params, "options")
	names := module.GetStringSlice(params, "name")

	brewCmd := "brew"
	installCmd := "install"
	uninstallCmd := "uninstall"
	listCmd := "list --formula"
	if cask {
		installCmd = "install --cask"
		uninstallCmd = "uninstall --cask"
		listCmd = "list --cask"
	}

	var lines []string

	if updateHomebrew {
		lines = append(lines, "brew update")
	}

	if upgradeAll {
		lines = append(lines, "brew upgrade")
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")
	}

	optStr := ""
	if len(options) > 0 {
		optStr = " " + strings.Join(options, " ")
	}

	for _, name := range names {
		qname := connector.ShellQuote(name)
		switch state {
		case StatePresent:
			lines = append(lines, fmt.Sprintf("if ! %s %s 2>/dev/null | grep -q %s; then", brewCmd, listCmd, qname))
			lines = append(lines, fmt.Sprintf("  %s %s %s%s", brewCmd, installCmd, qname, optStr))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		case StateAbsent:
			lines = append(lines, fmt.Sprintf("if %s %s 2>/dev/null | grep -q %s; then", brewCmd, listCmd, qname))
			lines = append(lines, fmt.Sprintf("  %s %s %s", brewCmd, uninstallCmd, qname))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		case StateLatest:
			lines = append(lines, fmt.Sprintf("if ! %s %s 2>/dev/null | grep -q %s; then", brewCmd, listCmd, qname))
			lines = append(lines, fmt.Sprintf("  %s %s %s%s", brewCmd, installCmd, qname, optStr))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "else")
			lines = append(lines, fmt.Sprintf("  %s upgrade %s 2>/dev/null || true", brewCmd, qname))
			lines = append(lines, "fi")
		}
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
