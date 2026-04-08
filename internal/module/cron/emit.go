package cron

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the cron module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	name, err := module.RequireString(params, "name")
	if err != nil {
		return nil, err
	}

	job := module.GetString(params, "job", "")
	state := module.GetString(params, "state", "present")
	minute := module.GetString(params, "minute", "*")
	hour := module.GetString(params, "hour", "*")
	day := module.GetString(params, "day", "*")
	month := module.GetString(params, "month", "*")
	weekday := module.GetString(params, "weekday", "*")
	specialTime := module.GetString(params, "special_time", "")
	user := module.GetString(params, "user", "")
	cronFile := module.GetString(params, "cron_file", "")
	disabled := module.GetBool(params, "disabled", false)

	marker := fmt.Sprintf("# Tack: %s", name)

	var lines []string

	if cronFile != "" {
		// Drop-in file mode
		return emitCronFile(cronFile, name, job, state, minute, hour, day, month, weekday, specialTime, user, disabled, marker)
	}

	// User crontab mode
	crontabCmd := "crontab"
	if user != "" {
		crontabCmd = fmt.Sprintf("crontab -u %s", connector.ShellQuote(user))
	}

	// Build the cron entry
	var entry string
	if specialTime != "" {
		entry = fmt.Sprintf("%s %s", specialTime, job)
	} else {
		entry = fmt.Sprintf("%s %s %s %s %s %s", minute, hour, day, month, weekday, job)
	}
	if disabled {
		entry = "# " + entry
	}

	switch state {
	case "present":
		if job == "" {
			return nil, fmt.Errorf("'job' parameter is required when state=present")
		}
		// Read current crontab, update or append, write back
		lines = append(lines, fmt.Sprintf("_tack_cron=$(%s -l 2>/dev/null || true)", crontabCmd))
		lines = append(lines, fmt.Sprintf("if echo \"$_tack_cron\" | grep -qF %s; then", connector.ShellQuote(marker)))
		// Replace existing entry (marker line + next line)
		lines = append(lines, fmt.Sprintf("  echo \"$_tack_cron\" | awk -v marker=%s -v entry=%s '", connector.ShellQuote(marker), connector.ShellQuote(entry)))
		lines = append(lines, "    $0 == marker { print; getline; print entry; next }")
		lines = append(lines, "    { print }")
		lines = append(lines, fmt.Sprintf("  ' | %s -", crontabCmd))
		lines = append(lines, "else")
		// Append new entry
		lines = append(lines, fmt.Sprintf("  { echo \"$_tack_cron\"; echo %s; echo %s; } | %s -", connector.ShellQuote(marker), connector.ShellQuote(entry), crontabCmd))
		lines = append(lines, "fi")
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")

	case "absent":
		// Remove entry by marker
		lines = append(lines, fmt.Sprintf("_tack_cron=$(%s -l 2>/dev/null || true)", crontabCmd))
		lines = append(lines, fmt.Sprintf("if echo \"$_tack_cron\" | grep -qF %s; then", connector.ShellQuote(marker)))
		lines = append(lines, fmt.Sprintf("  echo \"$_tack_cron\" | awk -v marker=%s '", connector.ShellQuote(marker)))
		lines = append(lines, "    $0 == marker { getline; next }")
		lines = append(lines, "    { print }")
		lines = append(lines, fmt.Sprintf("  ' | %s -", crontabCmd))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

func emitCronFile(cronFile, name, job, state, minute, hour, day, month, weekday, specialTime, user string, disabled bool, marker string) (*module.EmitResult, error) {
	qpath := connector.ShellQuote(cronFile)
	var lines []string

	var entry string
	if specialTime != "" {
		if user != "" {
			entry = fmt.Sprintf("%s %s %s", specialTime, user, job)
		} else {
			entry = fmt.Sprintf("%s %s", specialTime, job)
		}
	} else {
		if user != "" {
			entry = fmt.Sprintf("%s %s %s %s %s %s %s", minute, hour, day, month, weekday, user, job)
		} else {
			entry = fmt.Sprintf("%s %s %s %s %s %s", minute, hour, day, month, weekday, job)
		}
	}
	if disabled {
		entry = "# " + entry
	}

	switch state {
	case "present":
		if job == "" {
			return nil, fmt.Errorf("'job' parameter is required when state=present")
		}
		lines = append(lines, fmt.Sprintf("cat > %s <<'TACK_EOF'", qpath))
		lines = append(lines, marker)
		lines = append(lines, entry)
		lines = append(lines, "TACK_EOF")
		lines = append(lines, "TACK_CHANGED=$((TACK_CHANGED+1))")

	case "absent":
		lines = append(lines, fmt.Sprintf("if [ -f %s ]; then", qpath))
		lines = append(lines, fmt.Sprintf("  rm -f %s", qpath))
		lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
		lines = append(lines, "fi")
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
