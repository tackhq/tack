package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
)

// PackageCollector captures installed package state.
type PackageCollector struct{}

func (c *PackageCollector) Collect(ctx context.Context, conn connector.Connector, names []string, facts map[string]any) ([]TaskDef, error) {
	pkgMgr, _ := facts["pkg_manager"].(string)
	switch pkgMgr {
	case "apt":
		return c.collectApt(ctx, conn, names)
	case "brew":
		return c.collectBrew(ctx, conn, names)
	case "dnf", "yum":
		return c.collectDnf(ctx, conn, names, pkgMgr)
	default:
		return nil, fmt.Errorf("unsupported package manager: %q", pkgMgr)
	}
}

func (c *PackageCollector) collectApt(ctx context.Context, conn connector.Connector, names []string) ([]TaskDef, error) {
	var installed []string
	for _, pkg := range names {
		result, err := conn.Execute(ctx, fmt.Sprintf("dpkg-query -W -f='${Status}\\n' %s 2>/dev/null", pkg))
		if err != nil || result.ExitCode != 0 {
			fmt.Fprintf(WarnWriter, "warning: package %q not found on system, skipping\n", pkg)
			continue
		}
		if strings.Contains(result.Stdout, "install ok installed") {
			installed = append(installed, pkg)
		} else {
			fmt.Fprintf(WarnWriter, "warning: package %q not installed, skipping\n", pkg)
		}
	}

	if len(installed) == 0 {
		return nil, nil
	}

	if len(installed) == 1 {
		return []TaskDef{{
			Name:   fmt.Sprintf("Install %s", installed[0]),
			Module: "apt",
			Params: map[string]any{"name": installed[0], "state": "present"},
		}}, nil
	}

	return []TaskDef{{
		Name:   "Install packages",
		Module: "apt",
		Params: map[string]any{"name": "{{ item }}", "state": "present"},
		Loop:   installed,
	}}, nil
}

func (c *PackageCollector) collectBrew(ctx context.Context, conn connector.Connector, names []string) ([]TaskDef, error) {
	var formulae, casks []string

	for _, pkg := range names {
		// Check formulae
		result, err := conn.Execute(ctx, fmt.Sprintf("brew list --formula 2>/dev/null | grep -x %s", pkg))
		if err == nil && result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == pkg {
			formulae = append(formulae, pkg)
			continue
		}

		// Check casks
		result, err = conn.Execute(ctx, fmt.Sprintf("brew list --cask 2>/dev/null | grep -x %s", pkg))
		if err == nil && result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == pkg {
			casks = append(casks, pkg)
			continue
		}

		fmt.Fprintf(WarnWriter, "warning: package %q not found on system, skipping\n", pkg)
	}

	var tasks []TaskDef

	if len(formulae) == 1 {
		tasks = append(tasks, TaskDef{
			Name:   fmt.Sprintf("Install %s", formulae[0]),
			Module: "brew",
			Params: map[string]any{"name": formulae[0], "state": "present"},
		})
	} else if len(formulae) > 1 {
		tasks = append(tasks, TaskDef{
			Name:   "Install packages",
			Module: "brew",
			Params: map[string]any{"name": "{{ item }}", "state": "present"},
			Loop:   formulae,
		})
	}

	if len(casks) == 1 {
		tasks = append(tasks, TaskDef{
			Name:   fmt.Sprintf("Install %s (cask)", casks[0]),
			Module: "brew",
			Params: map[string]any{"name": casks[0], "state": "present", "cask": true},
		})
	} else if len(casks) > 1 {
		tasks = append(tasks, TaskDef{
			Name:   "Install casks",
			Module: "brew",
			Params: map[string]any{"name": "{{ item }}", "state": "present", "cask": true},
			Loop:   casks,
		})
	}

	return tasks, nil
}

func (c *PackageCollector) collectDnf(ctx context.Context, conn connector.Connector, names []string, mgr string) ([]TaskDef, error) {
	var installed []string
	for _, pkg := range names {
		result, err := conn.Execute(ctx, fmt.Sprintf("rpm -q %s 2>/dev/null", pkg))
		if err != nil || result.ExitCode != 0 {
			fmt.Fprintf(WarnWriter, "warning: package %q not found on system, skipping\n", pkg)
			continue
		}
		installed = append(installed, pkg)
	}

	if len(installed) == 0 {
		return nil, nil
	}

	// Emit command tasks since there's no dedicated dnf/yum module yet
	if len(installed) == 1 {
		return []TaskDef{{
			Name:   fmt.Sprintf("Install %s", installed[0]),
			Module: "command",
			Params: map[string]any{"cmd": fmt.Sprintf("%s install -y %s", mgr, installed[0])},
		}}, nil
	}

	return []TaskDef{{
		Name:   "Install packages",
		Module: "command",
		Params: map[string]any{"cmd": fmt.Sprintf("%s install -y {{ item }}", mgr)},
		Loop:   installed,
	}}, nil
}
