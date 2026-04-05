package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
)

// UserCollector captures user account state via getent/id.
type UserCollector struct{}

func (c *UserCollector) Collect(ctx context.Context, conn connector.Connector, names []string, _ map[string]any) ([]TaskDef, error) {
	var tasks []TaskDef

	for _, user := range names {
		// Get passwd entry: name:x:uid:gid:gecos:home:shell
		result, err := conn.Execute(ctx, fmt.Sprintf("getent passwd %s", user))
		if err != nil || result.ExitCode != 0 {
			fmt.Fprintf(WarnWriter, "warning: user %q not found, skipping\n", user)
			continue
		}

		fields := strings.SplitN(strings.TrimSpace(result.Stdout), ":", 7)
		if len(fields) < 7 {
			fmt.Fprintf(WarnWriter, "warning: unexpected passwd format for %q, skipping\n", user)
			continue
		}

		uid := fields[2]
		home := fields[5]
		shell := fields[6]

		// Get supplementary groups
		groupResult, err := conn.Execute(ctx, fmt.Sprintf("id -Gn %s", user))
		var groups []string
		if err == nil && groupResult.ExitCode == 0 {
			for _, g := range strings.Fields(strings.TrimSpace(groupResult.Stdout)) {
				if g != user { // exclude primary group (same as username)
					groups = append(groups, g)
				}
			}
		}

		// Build useradd command
		// No dedicated user module yet, so emit a command task
		cmd := fmt.Sprintf("id %s &>/dev/null || useradd --uid %s --home-dir %s --shell %s", user, uid, home, shell)
		if len(groups) > 0 {
			cmd += fmt.Sprintf(" --groups %s", strings.Join(groups, ","))
		}
		cmd += " " + user

		tasks = append(tasks, TaskDef{
			Name:   fmt.Sprintf("Ensure user %s exists", user),
			Module: "command",
			Params: map[string]any{
				"cmd": cmd,
			},
		})
	}

	return tasks, nil
}
