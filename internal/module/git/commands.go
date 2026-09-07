package git

import (
	"fmt"

	"github.com/tackhq/tack/internal/connector"
)

// buildFetchOriginCmd builds `<env>git -C <dest> fetch origin` with dest
// shell-quoted. env is an optional command prefix (e.g. a GIT_SSH_COMMAND
// assignment) or "". It is the shared leaf-command builder used by the Run path
// (plain fetch) and by Emit. (Depth/unshallow fetch variants and the clone and
// checkout commands intentionally differ between the two paths and are not
// shared here.)
func buildFetchOriginCmd(env, dest string) string {
	return fmt.Sprintf("%sgit -C %s fetch origin", env, connector.ShellQuote(dest))
}

// buildSubmoduleUpdateCmd builds
// `<env>git -C <dest> submodule update --init --recursive` with dest
// shell-quoted. Shared by the Run path (doSubmodules) and Emit.
func buildSubmoduleUpdateCmd(env, dest string) string {
	return fmt.Sprintf("%sgit -C %s submodule update --init --recursive", env, connector.ShellQuote(dest))
}
