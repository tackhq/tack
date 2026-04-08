package copy

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the copy module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	dest, err := module.RequireString(params, "dest")
	if err != nil {
		return nil, err
	}

	src := module.GetString(params, "src", "")
	content := module.GetString(params, "content", "")
	mode := module.GetString(params, "mode", "0644")
	owner := module.GetString(params, "owner", "")
	group := module.GetString(params, "group", "")
	backup := module.GetBool(params, "backup", false)
	createDirs := module.GetBool(params, "create_dirs", false)

	if src == "" && content == "" {
		return nil, fmt.Errorf("either 'src' or 'content' parameter is required")
	}

	// Resolve content
	var fileContent []byte
	var warnings []string
	if src != "" {
		srcPath := module.ResolveRolePath(src, params, "files")
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("reading source file %q: %w", srcPath, err)
		}
		fileContent = data
	} else {
		fileContent = []byte(content)
	}

	qdest := connector.ShellQuote(dest)
	tmpDest := dest + ".tack.tmp"
	qtmp := connector.ShellQuote(tmpDest)

	var lines []string

	// Create parent directories
	if createDirs {
		lines = append(lines, fmt.Sprintf("mkdir -p %s", connector.ShellQuote(destDir(dest))))
	}

	// Write content via heredoc or base64
	if utf8.Valid(fileContent) && !containsHeredocDelim(string(fileContent)) {
		lines = append(lines, fmt.Sprintf("cat > %s <<'TACK_EOF'", qtmp))
		lines = append(lines, string(fileContent))
		lines = append(lines, "TACK_EOF")
	} else {
		// Binary or content with heredoc delimiter — use base64
		encoded := base64.StdEncoding.EncodeToString(fileContent)
		lines = append(lines, fmt.Sprintf("echo %s | base64 -d > %s", connector.ShellQuote(encoded), qtmp))
		if utf8.Valid(fileContent) {
			warnings = append(warnings, "content contains heredoc delimiter, using base64 encoding")
		}
	}

	// Diff-guard: only replace if content differs
	lines = append(lines, fmt.Sprintf("if ! diff -q %s %s >/dev/null 2>&1; then", qdest, qtmp))
	if backup {
		lines = append(lines, fmt.Sprintf("  [ -f %s ] && cp %s %s.bak", qdest, qdest, qdest))
	}
	lines = append(lines, fmt.Sprintf("  mv %s %s", qtmp, qdest))
	lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
	lines = append(lines, "else")
	lines = append(lines, fmt.Sprintf("  rm -f %s", qtmp))
	lines = append(lines, "fi")

	// Mode
	if mode != "" {
		mode = module.NormalizeMode(mode)
		lines = append(lines, fmt.Sprintf("chmod %s %s", mode, qdest))
	}

	// Owner/group
	if owner != "" || group != "" {
		ownership := owner
		if group != "" {
			ownership += ":" + group
		}
		lines = append(lines, fmt.Sprintf("chown %s %s", connector.ShellQuote(ownership), qdest))
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
		Warnings:  warnings,
	}, nil
}

func destDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func containsHeredocDelim(s string) bool {
	return strings.Contains(s, "TACK_EOF")
}

var _ module.Emitter = (*Module)(nil)
