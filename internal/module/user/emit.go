package user

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the user module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	name, err := module.RequireString(params, "name")
	if err != nil {
		return nil, err
	}

	state := module.GetString(params, "state", "present")
	shell := module.GetString(params, "shell", "")
	home := module.GetString(params, "home", "")
	uid := module.GetInt(params, "uid", 0)
	groups := module.GetStringSlice(params, "groups")
	system := module.GetBool(params, "system", false)
	password := module.GetString(params, "password", "")
	remove := module.GetBool(params, "remove", false)

	qname := connector.ShellQuote(name)
	var lines []string

	switch state {
	case "present":
		// Check if user exists
		lines = append(lines, fmt.Sprintf("if ! getent passwd %s >/dev/null 2>&1; then", qname))

		// Build useradd command
		addCmd := "useradd"
		if system {
			addCmd += " -r"
		}
		if shell != "" {
			addCmd += fmt.Sprintf(" -s %s", connector.ShellQuote(shell))
		}
		if home != "" {
			addCmd += fmt.Sprintf(" -d %s -m", connector.ShellQuote(home))
		}
		if uid != 0 {
			addCmd += fmt.Sprintf(" -u %d", uid)
		}
		if len(groups) > 0 {
			addCmd += fmt.Sprintf(" -G %s", connector.ShellQuote(strings.Join(groups, ",")))
		}
		if password != "" {
			addCmd += fmt.Sprintf(" -p %s", connector.ShellQuote(password))
		}
		addCmd += " " + qname
		lines = append(lines, "  "+addCmd)
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")

		lines = append(lines, "else")
		// Build usermod command for existing user
		var modParts []string
		if shell != "" {
			modParts = append(modParts, fmt.Sprintf("-s %s", connector.ShellQuote(shell)))
		}
		if home != "" {
			modParts = append(modParts, fmt.Sprintf("-d %s", connector.ShellQuote(home)))
		}
		if len(groups) > 0 {
			modParts = append(modParts, fmt.Sprintf("-aG %s", connector.ShellQuote(strings.Join(groups, ","))))
		}
		if password != "" {
			modParts = append(modParts, fmt.Sprintf("-p %s", connector.ShellQuote(password)))
		}
		if len(modParts) > 0 {
			lines = append(lines, fmt.Sprintf("  usermod %s %s", strings.Join(modParts, " "), qname))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		}
		lines = append(lines, "fi")

	case "absent":
		lines = append(lines, fmt.Sprintf("if getent passwd %s >/dev/null 2>&1; then", qname))
		delCmd := "userdel"
		if remove {
			delCmd += " -r"
		}
		lines = append(lines, fmt.Sprintf("  %s %s", delCmd, qname))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
