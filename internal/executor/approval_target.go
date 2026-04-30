package executor

import (
	"fmt"
	"strings"
)

// approvalTargetVisibleHostsCap caps how many host names are listed inline in
// the approval-prompt target string for multi-host plays. See design.md:
// large fleets push the prompt past one terminal line; the leading count
// already conveys scope, so we truncate the visible names with `, ...`.
const approvalTargetVisibleHostsCap = 5

// formatApprovalTarget renders the human-readable host description shown in
// the approval prompt. The shape depends on how many hosts the play targets:
//
//   - 1 host:        "<host> (<connection>)"
//   - 2..5 hosts:    "<N> hosts (h1, h2, ..., hN)"
//   - 6+ hosts:      "<N> hosts (h1, h2, h3, h4, h5, ...)"
//
// connection is included only in the single-host form because it identifies
// which target (e.g. an SSM instance ID vs a same-named SSH host) the user
// is approving. For multi-host plays the connection is uniform across the
// listed hosts and is conveyed by the surrounding plan output.
func formatApprovalTarget(hosts []string, connection string) string {
	if connection == "" {
		connection = "local"
	}
	switch len(hosts) {
	case 0:
		// Defensive: an empty hosts slice shouldn't reach the prompt
		// (validation rejects it earlier), but render something sane.
		return fmt.Sprintf("(no hosts) (%s)", connection)
	case 1:
		return fmt.Sprintf("%s (%s)", hosts[0], connection)
	}

	visible := hosts
	truncated := false
	if len(hosts) > approvalTargetVisibleHostsCap {
		visible = hosts[:approvalTargetVisibleHostsCap]
		truncated = true
	}
	list := strings.Join(visible, ", ")
	if truncated {
		list += ", ..."
	}
	return fmt.Sprintf("%d hosts (%s)", len(hosts), list)
}
