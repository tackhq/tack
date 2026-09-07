package apt

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the apt module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	stateStr := module.GetString(params, "state", "present")
	state := State(stateStr)
	updateCache := module.GetBool(params, "update_cache", false)
	upgrade := module.GetString(params, "upgrade", "none")
	installRecommends := module.GetBool(params, "install_recommends", true)
	autoremove := module.GetBool(params, "autoremove", false)
	debFile := module.GetString(params, "deb", "")
	names := module.GetStringSlice(params, "name")

	var lines []string

	// Update cache
	if updateCache {
		lines = append(lines, aptUpdateCmd)
	}

	// Upgrade
	if c := buildUpgradeCmd(upgrade); c != "" {
		lines = append(lines, c)
	}

	// Deb file
	if debFile != "" {
		if strings.HasPrefix(debFile, "http://") || strings.HasPrefix(debFile, "https://") {
			lines = append(lines, buildCurlDownload("/tmp/tack-pkg.deb", connector.ShellQuote(debFile)))
			lines = append(lines, buildDpkgInstall("/tmp/tack-pkg.deb"))
		} else {
			lines = append(lines, buildDpkgInstall(connector.ShellQuote(debFile)))
		}
	}

	// Package management with idempotency check
	if len(names) > 0 {
		quoted := make([]string, len(names))
		for i, name := range names {
			quoted[i] = connector.ShellQuote(name)
		}
		pkgList := strings.Join(quoted, " ")

		recommends := "--no-install-recommends"
		if installRecommends {
			recommends = "--install-recommends"
		}

		switch state {
		case StatePresent, StateLatest:
			// Check if already installed, install only if needed
			for _, name := range names {
				lines = append(lines, fmt.Sprintf("if ! dpkg-query -W -f='${Status}' %s 2>/dev/null | grep -q 'install ok installed'; then", connector.ShellQuote(name)))
				lines = append(lines, "  "+buildInstallCmd(recommends, connector.ShellQuote(name)))
				lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
				lines = append(lines, "fi")
			}
			if state == StateLatest {
				lines = append(lines, buildInstallCmd(recommends, pkgList))
			}
		case StateAbsent:
			for _, name := range names {
				lines = append(lines, fmt.Sprintf("if dpkg-query -W -f='${Status}' %s 2>/dev/null | grep -q 'install ok installed'; then", connector.ShellQuote(name)))
				lines = append(lines, "  "+buildRemoveCmd("remove", connector.ShellQuote(name)))
				lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
				lines = append(lines, "fi")
			}
		case StatePurged:
			for _, name := range names {
				lines = append(lines, fmt.Sprintf("if dpkg-query -W %s 2>/dev/null; then", connector.ShellQuote(name)))
				lines = append(lines, "  "+buildRemoveCmd("purge", connector.ShellQuote(name)))
				lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")
				lines = append(lines, "fi")
			}
		}
	}

	// Autoremove
	if autoremove {
		lines = append(lines, aptAutoremoveCmd)
	}

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

var _ module.Emitter = (*Module)(nil)
