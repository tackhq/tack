package git

import (
	"fmt"
	"strings"

	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/module"
)

// Emit produces shell script text for the git module.
func (m *Module) Emit(params map[string]any, vars map[string]any) (*module.EmitResult, error) {
	repo, err := module.RequireString(params, "repo")
	if err != nil {
		return nil, err
	}

	dest, err := module.RequireString(params, "dest")
	if err != nil {
		return nil, err
	}

	version := module.GetString(params, "version", "")
	force := module.GetBool(params, "force", false)
	depth := module.GetInt(params, "depth", 0)
	acceptHostkey := module.GetBool(params, "accept_hostkey", false)
	keyFile := module.GetString(params, "key_file", "")
	bare := module.GetBool(params, "bare", false)
	singleBranch := module.GetBool(params, "single_branch", false)
	recursive := module.GetBool(params, "recursive", false)

	qrepo := connector.ShellQuote(repo)
	qdest := connector.ShellQuote(dest)

	// Build GIT_SSH_COMMAND if needed
	gitSSH := ""
	if acceptHostkey || keyFile != "" {
		sshOpts := "ssh"
		if acceptHostkey {
			sshOpts += " -o StrictHostKeyChecking=no"
		}
		if keyFile != "" {
			sshOpts += fmt.Sprintf(" -i %s", connector.ShellQuote(keyFile))
		}
		gitSSH = fmt.Sprintf("GIT_SSH_COMMAND=%s ", connector.ShellQuote(sshOpts))
	}

	var lines []string

	// Check if repo already exists
	dotGit := dest + "/.git"
	if bare {
		dotGit = dest + "/HEAD"
	}
	lines = append(lines, fmt.Sprintf("if [ ! -e %s ]; then", connector.ShellQuote(dotGit)))

	// Clone
	cloneCmd := fmt.Sprintf("%sgit clone", gitSSH)
	if bare {
		cloneCmd += " --bare"
	}
	if depth > 0 {
		cloneCmd += fmt.Sprintf(" --depth=%d", depth)
	}
	if singleBranch {
		cloneCmd += " --single-branch"
	}
	if version != "" {
		cloneCmd += fmt.Sprintf(" --branch %s", connector.ShellQuote(version))
	}
	cloneCmd += fmt.Sprintf(" %s %s", qrepo, qdest)

	// Create parent directory
	lines = append(lines, fmt.Sprintf("  mkdir -p %s", connector.ShellQuote(destParent(dest))))
	lines = append(lines, "  "+cloneCmd)

	if recursive {
		lines = append(lines, fmt.Sprintf("  %sgit -C %s submodule update --init --recursive", gitSSH, qdest))
	}
	lines = append(lines, "  TACK_CHANGED=$((TACK_CHANGED+1))")

	lines = append(lines, "else")
	// Already cloned — fetch and checkout if version specified
	if version != "" {
		if force {
			lines = append(lines, fmt.Sprintf("  %sgit -C %s fetch origin", gitSSH, qdest))
			lines = append(lines, fmt.Sprintf("  %sgit -C %s reset --hard origin/%s 2>/dev/null || %sgit -C %s checkout --detach %s",
				gitSSH, qdest, connector.ShellQuote(version), gitSSH, qdest, connector.ShellQuote(version)))
		} else {
			lines = append(lines, fmt.Sprintf("  _tack_head=$(%sgit -C %s rev-parse HEAD)", gitSSH, qdest))
			lines = append(lines, fmt.Sprintf("  _tack_want=$(%sgit -C %s rev-parse %s 2>/dev/null || echo 'unknown')", gitSSH, qdest, connector.ShellQuote(version)))
			lines = append(lines, "  if [ \"$_tack_head\" != \"$_tack_want\" ]; then")
			lines = append(lines, fmt.Sprintf("    %sgit -C %s fetch origin", gitSSH, qdest))
			lines = append(lines, fmt.Sprintf("    %sgit -C %s checkout --detach %s", gitSSH, qdest, connector.ShellQuote(version)))
			lines = append(lines, "    TACK_CHANGED=$((TACK_CHANGED+1))")
			lines = append(lines, "  fi")
		}
		if recursive {
			lines = append(lines, fmt.Sprintf("  %sgit -C %s submodule update --init --recursive", gitSSH, qdest))
		}
	}
	lines = append(lines, "fi")

	return &module.EmitResult{
		Supported: true,
		Shell:     strings.Join(lines, "\n"),
	}, nil
}

func destParent(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

var _ module.Emitter = (*Module)(nil)
