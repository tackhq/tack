package generate

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/eugenetaranov/bolt/internal/connector"
)

const (
	maxFileSize  = 1 << 20 // 1MB
	maxFindDepth = 5
)

// FileCollector captures file, directory, and symlink state.
type FileCollector struct{}

func (c *FileCollector) Collect(ctx context.Context, conn connector.Connector, paths []string, _ map[string]any) ([]TaskDef, error) {
	// Expand glob/recursive paths first
	expanded, err := c.expandPaths(ctx, conn, paths)
	if err != nil {
		return nil, err
	}

	var tasks []TaskDef

	for _, path := range expanded {
		path = strings.TrimRight(path, "/")

		// stat the path: type, mode, owner, group, size
		result, err := conn.Execute(ctx, fmt.Sprintf("stat -L -c '%%F|%%a|%%U|%%G|%%s' %s 2>/dev/null || stat -f '%%HT|%%Lp|%%Su|%%Sg|%%z' %s 2>/dev/null", path, path))
		if err != nil || result.ExitCode != 0 {
			fmt.Fprintf(WarnWriter, "warning: %s not found, skipping\n", path)
			continue
		}

		line := strings.TrimSpace(result.Stdout)
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			fmt.Fprintf(WarnWriter, "warning: could not parse stat output for %s, skipping\n", path)
			continue
		}

		fileType := strings.ToLower(parts[0])
		mode := parts[1]
		owner := parts[2]
		group := parts[3]
		size := parts[4]

		// Check if it's a symlink (before -L dereference, check separately)
		linkResult, err := conn.Execute(ctx, fmt.Sprintf("readlink %s 2>/dev/null", path))
		if err == nil && linkResult.ExitCode == 0 && strings.TrimSpace(linkResult.Stdout) != "" {
			// It's a symlink
			target := strings.TrimSpace(linkResult.Stdout)
			tasks = append(tasks, TaskDef{
				Name:   fmt.Sprintf("Symlink %s", path),
				Module: "file",
				Params: map[string]any{
					"path":  path,
					"src":   target,
					"state": "link",
				},
			})
			continue
		}

		if strings.Contains(fileType, "directory") {
			tasks = append(tasks, TaskDef{
				Name:   fmt.Sprintf("Ensure directory %s", path),
				Module: "file",
				Params: map[string]any{
					"path":  path,
					"state": "directory",
					"mode":  fmt.Sprintf("0%s", mode),
					"owner": owner,
					"group": group,
				},
			})
			continue
		}

		// Regular file
		// Check size
		sizeInt := 0
		_, _ = fmt.Sscanf(size, "%d", &sizeInt)
		if sizeInt > maxFileSize {
			fmt.Fprintf(WarnWriter, "warning: %s is larger than 1MB (%d bytes), skipping\n", path, sizeInt)
			continue
		}

		// Download content
		var buf bytes.Buffer
		if err := conn.Download(ctx, path, &buf); err != nil {
			fmt.Fprintf(WarnWriter, "warning: could not read %s: %v, skipping\n", path, err)
			continue
		}

		tasks = append(tasks, TaskDef{
			Name:   fmt.Sprintf("Configure %s", path),
			Module: "copy",
			Params: map[string]any{
				"dest":    path,
				"content": buf.String(),
				"mode":    fmt.Sprintf("0%s", mode),
				"owner":   owner,
				"group":   group,
			},
		})
	}

	return tasks, nil
}

// expandPaths resolves paths ending in /* or / to their recursive contents,
// capped at maxFindDepth levels. Plain paths are passed through unchanged.
func (c *FileCollector) expandPaths(ctx context.Context, conn connector.Connector, paths []string) ([]string, error) {
	var expanded []string
	for _, p := range paths {
		dir := ""
		if strings.HasSuffix(p, "/*") {
			dir = strings.TrimSuffix(p, "/*")
		} else if strings.HasSuffix(p, "/") {
			dir = strings.TrimSuffix(p, "/")
		}

		if dir == "" {
			expanded = append(expanded, p)
			continue
		}

		// Use find with depth limit to enumerate contents
		result, err := conn.Execute(ctx, fmt.Sprintf("find %s -maxdepth %d 2>/dev/null | sort", dir, maxFindDepth))
		if err != nil || result.ExitCode != 0 {
			fmt.Fprintf(WarnWriter, "warning: could not list %s, skipping\n", p)
			continue
		}

		for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				expanded = append(expanded, line)
			}
		}
	}
	return expanded, nil
}
