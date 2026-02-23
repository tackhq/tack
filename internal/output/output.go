// Package output provides formatted output for playbook execution.
package output

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

// Colors for terminal output.
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"
)

// Stats holds execution statistics for output.
type Stats interface {
	GetOK() int
	GetChanged() int
	GetFailed() int
	GetSkipped() int
	GetDuration() time.Duration
}

// Output handles formatted output.
type Output struct {
	w        io.Writer
	useColor bool
	debug    bool
	verbose  bool
}

// New creates a new output handler.
func New(w io.Writer) *Output {
	return &Output{
		w:        w,
		useColor: true,
	}
}

// SetColor enables or disables color output.
func (o *Output) SetColor(enabled bool) {
	o.useColor = enabled
}

// SetDebug enables or disables debug output.
func (o *Output) SetDebug(enabled bool) {
	o.debug = enabled
}

// SetVerbose enables or disables verbose output (full diffs in plan).
func (o *Output) SetVerbose(enabled bool) {
	o.verbose = enabled
}

// color returns the string wrapped in color codes if enabled.
func (o *Output) color(c, s string) string {
	if !o.useColor {
		return s
	}
	return c + s + colorReset
}

// PlaybookStart prints the playbook start banner.
func (o *Output) PlaybookStart(path string) {
	o.printf("\n%s %s\n", o.color(colorBold, "PLAYBOOK"), path)
	if o.debug {
		o.printf("%s\n", strings.Repeat("-", 60))
	}
}

// PlaybookEnd prints the playbook summary.
func (o *Output) PlaybookEnd(stats Stats) {
	o.printf("\n%s ", o.color(colorBold, "RECAP"))

	ok := o.color(colorGreen, fmt.Sprintf("ok=%d", stats.GetOK()))
	changed := o.color(colorYellow, fmt.Sprintf("changed=%d", stats.GetChanged()))
	failed := o.color(colorRed, fmt.Sprintf("failed=%d", stats.GetFailed()))
	skipped := o.color(colorCyan, fmt.Sprintf("skipped=%d", stats.GetSkipped()))

	o.printf("%s %s %s %s", ok, changed, failed, skipped)
	o.printf(" %s\n", o.color(colorGray, fmt.Sprintf("(%.2fs)", stats.GetDuration().Seconds())))
}

// PlayStart prints the play start banner.
func (o *Output) PlayStart(play *playbook.Play) {
	name := play.Name
	if name == "" {
		name = strings.Join(play.Hosts, ", ")
	}
	o.printf("\n%s %s\n", o.color(colorBold, "PLAY"), name)
}

// TaskStart is called when a task begins (no output in compact mode).
func (o *Output) TaskStart(name, moduleName string) {
	// In compact mode, we don't print anything on task start
	// Output is printed in TaskResult
}

// TaskResult prints the task result in a single line.
// Format: [status] module | host | task name
func (o *Output) TaskResult(name, status string, changed bool, message string) {
	// Determine status indicator and color
	var indicator string
	var statusColor string

	switch {
	case strings.HasPrefix(status, "ok"):
		indicator = "✓"
		statusColor = colorGreen
	case strings.HasPrefix(status, "changed"):
		indicator = "✓"
		statusColor = colorYellow
	case strings.HasPrefix(status, "skipped"):
		indicator = "○"
		statusColor = colorCyan
	case strings.HasPrefix(status, "failed"):
		indicator = "✗"
		statusColor = colorRed
	default:
		indicator = "?"
		statusColor = colorGray
	}

	// Print compact single line
	o.printf("  %s %s\n", o.color(statusColor, indicator), name)

	// In debug mode, print additional details
	if o.debug && message != "" {
		o.printf("    %s %s\n", o.color(colorGray, "→"), message)
	}
}

// TaskResultDetailed prints detailed task result (for debug mode).
func (o *Output) TaskResultDetailed(name, module, host, status, message string, data map[string]any) {
	// Determine status indicator and color
	var indicator string
	var statusColor string
	var statusText string

	switch {
	case strings.HasPrefix(status, "ok"):
		indicator = "✓"
		statusColor = colorGreen
		statusText = "ok"
	case strings.HasPrefix(status, "changed"):
		indicator = "✓"
		statusColor = colorYellow
		statusText = "changed"
	case strings.HasPrefix(status, "skipped"):
		indicator = "○"
		statusColor = colorCyan
		statusText = "skipped"
	case strings.HasPrefix(status, "failed"):
		indicator = "✗"
		statusColor = colorRed
		statusText = "FAILED"
	default:
		indicator = "?"
		statusColor = colorGray
		statusText = status
	}

	// Print compact line: [indicator] [module] name (host) - status
	moduleStr := ""
	if module != "" {
		moduleStr = o.color(colorGray, fmt.Sprintf("[%s] ", module))
	}
	hostStr := o.color(colorGray, fmt.Sprintf("(%s)", host))

	o.printf("  %s %s%s %s %s\n",
		o.color(statusColor, indicator),
		moduleStr,
		name,
		hostStr,
		o.color(statusColor, statusText))

	// In debug mode, print additional details
	if o.debug {
		if message != "" {
			o.printf("      %s %s\n", o.color(colorGray, "msg:"), message)
		}
		for k, v := range data {
			if k == "stdout" || k == "stderr" {
				if s, ok := v.(string); ok && s != "" {
					lines := strings.Split(strings.TrimSpace(s), "\n")
					o.printf("      %s\n", o.color(colorGray, k+":"))
					for _, line := range lines {
						o.printf("        %s\n", line)
					}
				}
			}
		}
	}
}

// Section prints a section header.
func (o *Output) Section(name string) {
	o.printf("\n%s\n", o.color(colorBold, name))
}

// Info prints an informational message.
func (o *Output) Info(format string, args ...any) {
	o.printf("%s %s\n", o.color(colorBlue, "INFO"), fmt.Sprintf(format, args...))
}

// Warn prints a warning message.
func (o *Output) Warn(format string, args ...any) {
	o.printf("%s %s\n", o.color(colorYellow, "WARN"), fmt.Sprintf(format, args...))
}

// Error prints an error message.
func (o *Output) Error(format string, args ...any) {
	o.printf("%s %s\n", o.color(colorRed, "ERROR"), fmt.Sprintf(format, args...))
}

// Debug prints a debug message (only in debug mode).
func (o *Output) Debug(format string, args ...any) {
	if o.debug {
		o.printf("%s %s\n", o.color(colorGray, "DEBUG"), fmt.Sprintf(format, args...))
	}
}

func (o *Output) printf(format string, args ...any) {
	fmt.Fprintf(o.w, format, args...)
}

// PlannedTask describes a task as evaluated during the plan phase.
type PlannedTask struct {
	Name      string
	Module    string
	Status    string // "will_run", "will_skip", "conditional", "will_change", "no_change", "always_runs"
	Reason    string // skip reason or condition text
	LoopCount int    // >0 if looped
	Params    map[string]any

	// Content comparison fields (populated from CheckResult).
	OldChecksum string
	NewChecksum string
	OldContent  string
	NewContent  string
}

// DisplayPlan renders the plan table showing what tasks will run.
func (o *Output) DisplayPlan(tasks []PlannedTask, dryRun bool) {
	label := "PLAN"
	if dryRun {
		label = "PLAN (dry run)"
	}
	o.printf("\n%s\n", o.color(colorBold, label))

	var willRun, willSkip, conditional, willChange, noChange, alwaysRuns int
	for _, t := range tasks {
		var indicator, col string
		switch t.Status {
		case "will_change":
			indicator = "+"
			col = colorYellow
			willChange++
		case "no_change":
			indicator = "="
			col = colorGreen
			noChange++
		case "always_runs":
			indicator = "~"
			col = colorYellow
			alwaysRuns++
		case "will_run":
			indicator = "+"
			col = colorGreen
			willRun++
		case "will_skip":
			indicator = "○"
			col = colorCyan
			willSkip++
		case "conditional":
			indicator = "?"
			col = colorYellow
			conditional++
		}

		module := ""
		if t.Module != "" {
			module = fmt.Sprintf("[%s]", t.Module)
		}

		suffix := ""
		if t.Status == "will_skip" && t.Reason != "" {
			suffix = fmt.Sprintf("(%s)", t.Reason)
		} else if t.Status == "conditional" && t.Reason != "" {
			suffix = fmt.Sprintf("(when: %s)", t.Reason)
		} else if t.Status == "no_change" && t.Reason != "" {
			suffix = fmt.Sprintf("(%s)", t.Reason)
		} else if t.Status == "always_runs" && t.Reason != "" {
			suffix = fmt.Sprintf("(%s)", t.Reason)
		} else if t.Status == "will_change" && t.Reason != "" {
			suffix = fmt.Sprintf("(%s)", t.Reason)
		}
		if t.LoopCount > 0 {
			if suffix != "" {
				suffix += " "
			}
			suffix += fmt.Sprintf("(%d items)", t.LoopCount)
		}

		line := fmt.Sprintf("  %s %-30s %-12s %s", indicator, t.Name, module, suffix)
		o.printf("%s\n", o.color(col, strings.TrimRight(line, " ")))

		// Show task parameters
		for _, paramLine := range formatTaskParams(t.Module, t.Params) {
			o.printf("      %s\n", o.color(colorGray, paramLine))
		}

		// Show checksums or diff when content differs
		if t.OldChecksum != "" && t.NewChecksum != "" && t.OldChecksum != t.NewChecksum {
			if o.verbose && t.OldContent != "" && t.NewContent != "" {
				for _, diffLine := range unifiedDiff(t.OldContent, t.NewContent) {
					var diffColor string
					if strings.HasPrefix(diffLine, "+ ") {
						diffColor = colorGreen
					} else if strings.HasPrefix(diffLine, "- ") {
						diffColor = colorRed
					} else {
						diffColor = colorGray
					}
					o.printf("      %s\n", o.color(diffColor, diffLine))
				}
			} else {
				o.printf("      %s\n", o.color(colorRed, "old: "+t.OldChecksum))
				o.printf("      %s\n", o.color(colorGreen, "new: "+t.NewChecksum))
			}
		} else if t.OldChecksum == "" && t.NewChecksum != "" {
			o.printf("      %s\n", o.color(colorYellow, "new: "+t.NewChecksum))
		}
	}

	var summaryParts []string
	if willChange > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d to change", willChange))
	}
	if noChange > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d ok", noChange))
	}
	if alwaysRuns > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d to run", alwaysRuns))
	}
	if willRun > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d to run", willRun))
	}
	if conditional > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d conditional", conditional))
	}
	if willSkip > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d to skip", willSkip))
	}
	if len(summaryParts) == 0 {
		summaryParts = append(summaryParts, "nothing to do")
	}

	o.printf("\n%s %s\n",
		o.color(colorBold, "Plan:"),
		strings.Join(summaryParts, ", ")+".")
}

// formatTaskParams returns display lines for key parameters of a task, based on module type.
func formatTaskParams(module string, params map[string]any) []string {
	if len(params) == 0 {
		return nil
	}

	// Module-specific key lists (order matters for display)
	moduleKeys := map[string][]string{
		"command":  {"cmd"},
		"shell":    {"cmd"},
		"copy":     {"dest", "src", "mode"},
		"file":     {"path", "state", "mode", "owner"},
		"template": {"src", "dest", "mode"},
		"apt":      {"name", "state", "update_cache"},
		"brew":     {"name", "state", "cask"},
	}

	var lines []string

	if keys, ok := moduleKeys[module]; ok {
		for _, k := range keys {
			if v, exists := params[k]; exists {
				lines = append(lines, fmt.Sprintf("%s: %s", k, truncateParamValue(v)))
			}
		}
	} else {
		// Generic fallback: show all params except internal ones
		for k, v := range params {
			if strings.HasPrefix(k, "_") {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: %s", k, truncateParamValue(v)))
		}
	}

	return lines
}

// truncateParamValue formats a parameter value for display, truncating long strings.
func truncateParamValue(v any) string {
	s := fmt.Sprintf("%v", v)
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// unifiedDiff produces a simple unified-diff-style output comparing old and new content.
func unifiedDiff(old, new string) []string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Trim trailing empty line from final newline split
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	var result []string

	// Simple line-by-line diff: walk both slices with a basic LCS approach.
	// For brevity we use a two-pointer approach that handles common prefixes/suffixes
	// and shows removed/added blocks in between.
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			result = append(result, "  "+oldLines[i])
			i++
			j++
		} else {
			// Find next matching line
			matchI, matchJ := findNextMatch(oldLines, newLines, i, j)
			for ; i < matchI; i++ {
				result = append(result, "- "+oldLines[i])
			}
			for ; j < matchJ; j++ {
				result = append(result, "+ "+newLines[j])
			}
		}
	}
	for ; i < len(oldLines); i++ {
		result = append(result, "- "+oldLines[i])
	}
	for ; j < len(newLines); j++ {
		result = append(result, "+ "+newLines[j])
	}

	return result
}

// findNextMatch finds the nearest matching line pair starting from positions oi and nj.
func findNextMatch(old, new []string, oi, nj int) (int, int) {
	// Look ahead up to 50 lines for a match
	limit := 50
	for dist := 1; dist < limit; dist++ {
		// Check if old[oi+dist] matches any new[nj..nj+dist]
		if oi+dist < len(old) {
			for k := nj; k < len(new) && k <= nj+dist; k++ {
				if old[oi+dist] == new[k] {
					return oi + dist, k
				}
			}
		}
		// Check if new[nj+dist] matches any old[oi..oi+dist]
		if nj+dist < len(new) {
			for k := oi; k < len(old) && k <= oi+dist; k++ {
				if old[k] == new[nj+dist] {
					return k, nj + dist
				}
			}
		}
	}
	// No match found within limit — dump everything remaining
	return len(old), len(new)
}

// PromptApproval asks the user to confirm applying changes.
// Returns true only if the user types exactly "yes".
// Responds immediately to SIGINT/SIGTERM.
func (o *Output) PromptApproval() bool {
	o.printf("\n%s ", o.color(colorBold, "Do you want to apply these changes?"))
	fmt.Fprint(o.w, "(yes/no): ")

	// Read input in a goroutine so we can race against signals
	resultCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			resultCh <- strings.TrimSpace(scanner.Text())
		} else {
			resultCh <- ""
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case text := <-resultCh:
		return text == "yes"
	case <-sigCh:
		fmt.Fprintln(o.w)
		return false
	}
}
