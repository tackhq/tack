package systemd

import (
	"fmt"

	"github.com/tackhq/tack/internal/connector"
)

// systemctlDaemonReloadCmd is the leaf command that reloads the systemd manager
// configuration. Shared by the Run path (runDaemonReload) and Emit so both
// produce the exact same string.
const systemctlDaemonReloadCmd = "systemctl daemon-reload"

// buildSystemctlCmd builds `systemctl <action> <unit>` with the unit
// shell-quoted. It is the shared leaf-command builder used by the Run path
// (runSystemctl) and by Emit for the start/stop/restart/reload, enable/disable,
// and mask/unmask actions.
func buildSystemctlCmd(action, unit string) string {
	return fmt.Sprintf("systemctl %s %s", action, connector.ShellQuote(unit))
}
