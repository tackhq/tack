package group

import (
	"fmt"

	"github.com/tackhq/tack/internal/connector"
)

// buildGroupmodCmd builds `groupmod -g <gid> <name>` with the name shell-quoted.
// Shared by the Run path and Emit.
func buildGroupmodCmd(gid int, name string) string {
	return fmt.Sprintf("groupmod -g %d %s", gid, connector.ShellQuote(name))
}

// buildGroupdelCmd builds `groupdel <name>` with the name shell-quoted. Shared
// by the Run path and Emit. (groupadd flag ordering intentionally differs
// between the two paths and is not shared here.)
func buildGroupdelCmd(name string) string {
	return fmt.Sprintf("groupdel %s", connector.ShellQuote(name))
}
