package yum

import "fmt"

// buildMakecacheCmd builds a makecache command for the given package manager.
// It is shared between the runtime Run path and the Emit (exported shell) path
// to keep the executed and emitted command in sync. Run passes the resolved
// package manager ("dnf" or "yum"); Emit passes the shell variable
// "${_tack_pkg}" that it resolves at script runtime.
func buildMakecacheCmd(pkgMgr string) string {
	return fmt.Sprintf("%s makecache -q", pkgMgr)
}
