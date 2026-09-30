package connector

import (
	"context"
	"fmt"
	"strings"
)

// ProgressKind distinguishes command echoes (which may carry task content
// and are suppressed for no_log tasks) from plain status updates.
type ProgressKind int

const (
	// ProgressStatus is a connector status update ("connecting", "SSM InProgress (12s)").
	ProgressStatus ProgressKind = iota
	// ProgressCommand reports the command about to run.
	ProgressCommand
)

// ProgressFunc receives live progress from a connector.
type ProgressFunc func(kind ProgressKind, msg string)

type progressKey struct{}

// WithProgress returns a context whose connector operations report progress
// to fn. Progress is display-only; connectors never block on it.
func WithProgress(ctx context.Context, fn ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

// ReportProgress sends a status update to the context's progress sink, if any.
func ReportProgress(ctx context.Context, format string, args ...any) {
	if fn, ok := ctx.Value(progressKey{}).(ProgressFunc); ok && fn != nil {
		fn(ProgressStatus, fmt.Sprintf(format, args...))
	}
}

// ReportCommand reports the command about to run, condensed to its first
// line and at most 80 characters.
func ReportCommand(ctx context.Context, cmd string) {
	fn, ok := ctx.Value(progressKey{}).(ProgressFunc)
	if !ok || fn == nil {
		return
	}
	fn(ProgressCommand, "running: "+SummarizeCommand(cmd, 80))
}

// SummarizeCommand condenses cmd to its first non-empty line with whitespace
// collapsed, truncated to max runes with an ellipsis.
func SummarizeCommand(cmd string, max int) string {
	line := ""
	for _, l := range strings.Split(cmd, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			line = l
			break
		}
	}
	line = strings.Join(strings.Fields(line), " ")
	if r := []rune(line); len(r) > max {
		line = string(r[:max-1]) + "…"
	}
	return line
}

// SuppressCommands returns a context that forwards status updates but drops
// command echoes. Connectors use it when a command embeds transferred file
// content (e.g. base64 uploads) that must not be displayed.
func SuppressCommands(ctx context.Context) context.Context {
	fn, ok := ctx.Value(progressKey{}).(ProgressFunc)
	if !ok || fn == nil {
		return ctx
	}
	return WithProgress(ctx, func(kind ProgressKind, msg string) {
		if kind != ProgressCommand {
			fn(kind, msg)
		}
	})
}
