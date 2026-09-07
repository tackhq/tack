package user

import (
	"github.com/tackhq/tack/internal/connector"
)

// buildUserdelCmd builds `userdel [-r] <name>` with the name shell-quoted. It is
// the shared leaf-command builder used by both the Run path and Emit so the two
// produce byte-identical delete commands. (useradd/usermod construction
// intentionally differs between the two paths and is not shared here.)
func buildUserdelCmd(name string, remove bool) string {
	cmd := "userdel"
	if remove {
		cmd += " -r"
	}
	return cmd + " " + connector.ShellQuote(name)
}
