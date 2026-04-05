// Package cron provides a module for managing cron entries idempotently.
package cron

import (
	"fmt"
	"regexp"
	"strings"
)

// markerPrefix is the comment placed on the line preceding each managed entry.
const markerPrefix = "# TACK: "

// managed represents a located managed block in a crontab's line slice.
type managed struct {
	markerIdx int // index of the marker line
	entryIdx  int // index of the entry (schedule or env) line, or -1 if no following line
}

// locateManaged scans lines for a managed block identified by the given name.
// Returns the located block or (managed{-1,-1}, false) if not present.
func locateManaged(lines []string, name string) (managed, bool) {
	want := markerPrefix + name
	for i, l := range lines {
		if strings.TrimRight(l, " \t") == want {
			entryIdx := -1
			if i+1 < len(lines) {
				entryIdx = i + 1
			}
			return managed{markerIdx: i, entryIdx: entryIdx}, true
		}
	}
	return managed{markerIdx: -1, entryIdx: -1}, false
}

// renderSchedulePrefix builds the schedule portion of a cron line:
// either "@shortcut " or "min hour day mon weekday ".
// If specialTime is non-empty it is used; otherwise the time fields are rendered.
func renderSchedulePrefix(specialTime, minute, hour, day, month, weekday string) string {
	if specialTime != "" {
		return "@" + specialTime + " "
	}
	return fmt.Sprintf("%s %s %s %s %s ", minute, hour, day, month, weekday)
}

// renderEntryLine builds the full cron entry line:
//   - For env mode: just the KEY=VALUE job string
//   - For drop-in mode: "<schedule> <user> <job>"
//   - For user-crontab mode: "<schedule> <job>"
func renderEntryLine(env bool, schedulePrefix, user, job string, dropIn bool) string {
	if env {
		return job
	}
	if dropIn {
		return schedulePrefix + user + " " + job
	}
	return schedulePrefix + job
}

// renderManagedBlock returns the two lines: marker + (possibly commented) entry.
func renderManagedBlock(name, entry string, disabled bool) (marker, line string) {
	marker = markerPrefix + name
	if disabled {
		line = "# " + entry
	} else {
		line = entry
	}
	return
}

// applyPresent inserts or replaces a managed block in the given line slice.
// Returns the new slice.
func applyPresent(lines []string, name, marker, entry string) []string {
	m, found := locateManaged(lines, name)
	if !found {
		// Append. Ensure a blank line separates from any existing content.
		out := make([]string, 0, len(lines)+3)
		out = append(out, lines...)
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, marker, entry)
		return out
	}
	// Replace marker + entry line in place.
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:m.markerIdx]...)
	out = append(out, marker)
	if m.entryIdx >= 0 {
		// Replace existing entry line.
		out = append(out, entry)
		out = append(out, lines[m.entryIdx+1:]...)
	} else {
		// Marker at EOF with no following line; add one.
		out = append(out, entry)
	}
	return out
}

// applyAbsent removes the marker and the entry line following it.
// Returns the new slice and true if anything was removed.
func applyAbsent(lines []string, name string) ([]string, bool) {
	m, found := locateManaged(lines, name)
	if !found {
		return lines, false
	}
	out := make([]string, 0, len(lines))
	out = append(out, lines[:m.markerIdx]...)
	if m.entryIdx >= 0 {
		out = append(out, lines[m.entryIdx+1:]...)
	}
	// Collapse a potential duplicate blank line caused by removal.
	out = collapseTrailingBlanks(out)
	return out, true
}

// collapseTrailingBlanks trims trailing empty lines from a slice.
func collapseTrailingBlanks(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// splitLines splits content into lines, dropping the trailing newline if present
// so the slice does not end with a spurious empty entry.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

// joinLines joins a line slice back to content with a trailing newline.
// Returns "" for an empty slice (so empty files are truly empty).
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// specialTimeSet lists the allowed `@` shortcuts.
var specialTimeSet = map[string]struct{}{
	"reboot":   {},
	"yearly":   {},
	"annually": {},
	"monthly":  {},
	"weekly":   {},
	"daily":    {},
	"hourly":   {},
}

// envLinePattern validates environment-mode job strings.
var envLinePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=.*$`)

// dropInNamePattern enforces /etc/cron.d/ filename restrictions.
var dropInNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// namePattern allows any printable single-line value that doesn't clash with marker parsing.
// Disallows newline, '#', non-printable.
func isValidName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) > 200 {
		return fmt.Errorf("name exceeds 200 characters")
	}
	if strings.ContainsAny(name, "\n\r") {
		return fmt.Errorf("name cannot contain newlines")
	}
	if strings.Contains(name, "#") {
		return fmt.Errorf("name cannot contain '#'")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("name contains non-printable character")
		}
	}
	return nil
}
