package command

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the command module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	cmd, err := module.RequireString(params, "cmd")
	if err != nil {
		return nil, err
	}

	chdir := module.GetString(params, "chdir", "")
	creates := module.GetString(params, "creates", "")
	removes := module.GetString(params, "removes", "")
	changedWhen := module.GetString(params, "changed_when", "")

	var lines []string

	// creates/removes guards
	if creates != "" {
		lines = append(lines, fmt.Sprintf("if [ ! -e %s ]; then", connector.ShellQuote(creates)))
	}
	if removes != "" {
		lines = append(lines, fmt.Sprintf("if [ -e %s ]; then", connector.ShellQuote(removes)))
	}

	// Build the command
	fullCmd := cmd
	if chdir != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", connector.ShellQuote(chdir), cmd)
	}

	// Change detection
	if changedWhen != "" {
		// Capture output and check changed_when expression
		lines = append(lines, fmt.Sprintf("_tack_out=$(%s)", fullCmd))
		lines = append(lines, fmt.Sprintf("if %s; then", changedWhen))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	} else {
		lines = append(lines, fullCmd)
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")
	}

	// Close guards
	if creates != "" || removes != "" {
		lines = append(lines, "fi")
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
