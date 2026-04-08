package blockinfile

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the blockinfile module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	path, err := module.RequireString(params, "path")
	if err != nil {
		return nil, err
	}

	stateStr := module.GetString(params, "state", "present")
	block := module.GetString(params, "block", "")
	marker := module.GetString(params, "marker", "# {mark} MANAGED BLOCK")
	create := module.GetBool(params, "create", false)
	backup := module.GetBool(params, "backup", false)

	qpath := connector.ShellQuote(path)
	beginMarker := strings.ReplaceAll(marker, "{mark}", "BEGIN")
	endMarker := strings.ReplaceAll(marker, "{mark}", "END")

	var lines []string

	if backup {
		lines = append(lines, fmt.Sprintf("[ -f %s ] && cp -p %s %s.bak", qpath, qpath, qpath))
	}

	if create {
		lines = append(lines, fmt.Sprintf("[ -f %s ] || touch %s", qpath, qpath))
	}

	switch stateStr {
	case "present":
		// Use awk to replace or append the managed block
		// First check if block markers exist
		lines = append(lines, fmt.Sprintf("if grep -qF %s %s 2>/dev/null; then", connector.ShellQuote(beginMarker), qpath))
		// Replace existing block between markers using awk
		lines = append(lines, fmt.Sprintf("  awk -v begin=%s -v end=%s -v block=%s '",
			connector.ShellQuote(beginMarker), connector.ShellQuote(endMarker), connector.ShellQuote(block)))
		lines = append(lines, "    $0 == begin { print; print block; skip=1; next }")
		lines = append(lines, "    $0 == end { skip=0 }")
		lines = append(lines, "    !skip { print }")
		lines = append(lines, fmt.Sprintf("  ' %s > %s.tmp && mv %s.tmp %s", qpath, qpath, qpath, qpath))
		lines = append(lines, "else")
		// Append new block with markers
		lines = append(lines, fmt.Sprintf("  echo %s >> %s", connector.ShellQuote(beginMarker), qpath))
		lines = append(lines, fmt.Sprintf("  echo %s >> %s", connector.ShellQuote(block), qpath))
		lines = append(lines, fmt.Sprintf("  echo %s >> %s", connector.ShellQuote(endMarker), qpath))
		lines = append(lines, "fi")
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")

	case "absent":
		// Remove the block between markers
		lines = append(lines, fmt.Sprintf("if grep -qF %s %s 2>/dev/null; then", connector.ShellQuote(beginMarker), qpath))
		lines = append(lines, fmt.Sprintf("  awk -v begin=%s -v end=%s '",
			connector.ShellQuote(beginMarker), connector.ShellQuote(endMarker)))
		lines = append(lines, "    $0 == begin { skip=1; next }")
		lines = append(lines, "    $0 == end { skip=0; next }")
		lines = append(lines, "    !skip { print }")
		lines = append(lines, fmt.Sprintf("  ' %s > %s.tmp && mv %s.tmp %s", qpath, qpath, qpath, qpath))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
