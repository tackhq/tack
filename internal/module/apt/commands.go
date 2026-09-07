package apt

import "fmt"

// This file holds pure builders for the leaf shell command strings that BOTH
// the runtime Run path and the Emit (exported shell) path construct. Sharing
// them keeps the executed and emitted commands byte-for-byte in sync.

// aptUpdateCmd refreshes the package cache.
const aptUpdateCmd = "DEBIAN_FRONTEND=noninteractive apt-get update -qq"

// aptAutoremoveCmd removes unused dependency packages.
const aptAutoremoveCmd = "DEBIAN_FRONTEND=noninteractive apt-get autoremove -y -qq"

// buildUpgradeCmd returns the apt-get upgrade command for the given mode, or ""
// when the mode does not trigger an upgrade.
func buildUpgradeCmd(mode string) string {
	switch mode {
	case "yes", "safe":
		return "DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -qq"
	case "full":
		return "DEBIAN_FRONTEND=noninteractive apt-get full-upgrade -y -qq"
	case "dist":
		return "DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -y -qq"
	default:
		return ""
	}
}

// buildInstallCmd builds an apt-get install invocation from a pre-rendered flags
// string and a pre-rendered (already shell-quoted) package list.
func buildInstallCmd(flags, pkgs string) string {
	return fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y -qq %s %s", flags, pkgs)
}

// buildRemoveCmd builds an apt-get remove/purge invocation from an action
// ("remove" or "purge") and a pre-rendered (already shell-quoted) package list.
func buildRemoveCmd(action, pkgs string) string {
	return fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get %s -y -qq %s", action, pkgs)
}

// buildCurlDownload builds a curl download command from pre-rendered destination
// and source tokens.
func buildCurlDownload(dst, src string) string {
	return fmt.Sprintf("curl -fsSL -o %s %s", dst, src)
}

// buildDpkgInstall builds a dpkg install (with apt-get -f fallback) command from
// a pre-rendered path token.
func buildDpkgInstall(path string) string {
	return fmt.Sprintf("DEBIAN_FRONTEND=noninteractive dpkg -i %s || apt-get install -f -y -qq", path)
}
