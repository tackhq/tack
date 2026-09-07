package command

import (
	"fmt"

	"github.com/tackhq/tack/internal/connector"
)

// buildFullCmd wraps cmd with a `cd <chdir> &&` prefix when chdir is set,
// otherwise returns cmd unchanged. It is the shared leaf-command builder used by
// both the Run path and Emit so the executed and emitted command strings stay
// identical. (The creates/removes idempotency guards differ between the two
// paths and are not shared here.)
func buildFullCmd(cmd, chdir string) string {
	if chdir == "" {
		return cmd
	}
	return fmt.Sprintf("cd %s && %s", connector.ShellQuote(chdir), cmd)
}
