package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
)

// ServiceCollector captures systemd service state.
type ServiceCollector struct{}

func (c *ServiceCollector) Collect(ctx context.Context, conn connector.Connector, names []string, _ map[string]any) ([]TaskDef, error) {
	var tasks []TaskDef

	for _, svc := range names {
		// Check if active
		activeResult, err := conn.Execute(ctx, fmt.Sprintf("systemctl is-active %s 2>/dev/null", svc))
		if err != nil {
			fmt.Fprintf(WarnWriter, "warning: could not check service %q, skipping\n", svc)
			continue
		}
		active := strings.TrimSpace(activeResult.Stdout)

		// Check if enabled
		enabledResult, err := conn.Execute(ctx, fmt.Sprintf("systemctl is-enabled %s 2>/dev/null", svc))
		if err != nil {
			fmt.Fprintf(WarnWriter, "warning: could not check service %q enabled state, skipping\n", svc)
			continue
		}
		enabled := strings.TrimSpace(enabledResult.Stdout)

		state := "stopped"
		if active == "active" {
			state = "started"
		}

		isEnabled := enabled == "enabled"

		name := fmt.Sprintf("Ensure %s is %s", svc, state)
		if isEnabled {
			name = fmt.Sprintf("Ensure %s is %s and enabled", svc, state)
		}

		tasks = append(tasks, TaskDef{
			Name:   name,
			Module: "systemd",
			Params: map[string]any{
				"name":    svc,
				"state":   state,
				"enabled": isEnabled,
			},
		})
	}

	return tasks, nil
}
