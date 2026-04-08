package systemd

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the systemd module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	name, err := module.RequireString(params, "name")
	if err != nil {
		return nil, err
	}

	state := module.GetString(params, "state", "")
	daemonReload := module.GetBool(params, "daemon_reload", false)

	// Normalize unit name
	unit := name
	if !strings.Contains(unit, ".") {
		unit += ".service"
	}
	qunit := connector.ShellQuote(unit)

	var lines []string

	// Daemon reload
	if daemonReload {
		lines = append(lines, "systemctl daemon-reload")
	}

	// State management
	switch state {
	case "started":
		lines = append(lines, fmt.Sprintf("if ! systemctl is-active --quiet %s; then", qunit))
		lines = append(lines, fmt.Sprintf("  systemctl start %s", qunit))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	case "stopped":
		lines = append(lines, fmt.Sprintf("if systemctl is-active --quiet %s; then", qunit))
		lines = append(lines, fmt.Sprintf("  systemctl stop %s", qunit))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	case "restarted":
		lines = append(lines, fmt.Sprintf("systemctl restart %s", qunit))
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")
	case "reloaded":
		lines = append(lines, fmt.Sprintf("systemctl reload %s", qunit))
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")
	}

	// Enabled/disabled
	if enabledRaw, ok := params["enabled"]; ok {
		enabled, _ := enabledRaw.(bool)
		if enabled {
			lines = append(lines, fmt.Sprintf("if ! systemctl is-enabled --quiet %s; then", qunit))
			lines = append(lines, fmt.Sprintf("  systemctl enable %s", qunit))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		} else {
			lines = append(lines, fmt.Sprintf("if systemctl is-enabled --quiet %s; then", qunit))
			lines = append(lines, fmt.Sprintf("  systemctl disable %s", qunit))
			lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "fi")
		}
	}

	// Masked/unmasked
	if maskedRaw, ok := params["masked"]; ok {
		masked, _ := maskedRaw.(bool)
		if masked {
			lines = append(lines, fmt.Sprintf("systemctl mask %s", qunit))
			lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")
		} else {
			lines = append(lines, fmt.Sprintf("systemctl unmask %s", qunit))
			lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")
		}
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
