// Package testrun orchestrates testing roles/playbooks in Docker containers.
package testrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/eugenetaranov/bolt/internal/executor"
	"github.com/eugenetaranov/bolt/internal/playbook"
)

// Options configures a test run.
type Options struct {
	Target  string // role name or playbook path
	Image   string // Docker image (default: ubuntu:24.04)
	New     bool   // force fresh container
	Remove  bool   // remove container after run
	Debug   bool
	Verbose bool
	DryRun  bool
	NoColor bool
}

// Run starts a Docker container (reusing an existing one when possible),
// applies the target playbook or role, prints results, and optionally cleans up.
func Run(ctx context.Context, opts Options) error {
	if opts.Image == "" {
		opts.Image = "ubuntu:24.04"
	}

	name := stableContainerName(opts.Target)

	if err := ensureContainer(ctx, name, opts.Image, opts.New); err != nil {
		return err
	}

	// Cleanup container only if --rm
	defer func() {
		if opts.Remove {
			rmCmd := exec.Command("docker", "rm", "-f", name)
			_ = rmCmd.Run()
			return
		}
		fmt.Fprintf(os.Stderr, "Container kept: %s\n", name)
		fmt.Fprintf(os.Stderr, "  docker exec -it %s /bin/bash\n", name)
		fmt.Fprintf(os.Stderr, "  docker rm -f %s\n", name)
	}()

	// Determine playbook path
	var pbPath string
	var cleanup func()

	if isPlaybook(opts.Target) {
		pbPath = opts.Target
		cleanup = func() {}
	} else {
		role, err := resolveRole(opts.Target)
		if err != nil {
			return fmt.Errorf("resolving role: %w", err)
		}
		pbPath, cleanup, err = generateTempPlaybook(name, role)
		if err != nil {
			return fmt.Errorf("generating temp playbook: %w", err)
		}
	}
	defer cleanup()

	// Parse playbook
	pb, err := playbook.ParseFileRaw(pbPath)
	if err != nil {
		return fmt.Errorf("parsing playbook: %w", err)
	}

	// Create executor with docker overrides
	exec := executor.New()
	exec.Debug = opts.Debug
	exec.Verbose = opts.Verbose
	exec.DryRun = opts.DryRun
	exec.AutoApprove = true
	exec.Overrides = &executor.ConnOverrides{
		Connection: "docker",
		Hosts:      []string{name},
	}
	exec.Output.SetColor(!opts.NoColor)
	exec.Output.SetDebug(opts.Debug)
	exec.Output.SetVerbose(opts.Verbose)

	result, err := exec.Run(ctx, pb)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("test failed")
	}

	return nil
}

// isPlaybook returns true if the target looks like a playbook file.
func isPlaybook(target string) bool {
	lower := strings.ToLower(target)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return true
	}
	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		return true
	}
	return false
}

// resolveRole turns the target into a role reference that LoadRole can find
// from a temp playbook in /tmp. If the target is a directory on disk, we
// convert it to an absolute path (LoadRole handles absolute paths directly).
// Otherwise we return it unchanged (bare name like "webserver" resolved via
// the roles/ symlink).
func resolveRole(target string) (role string, err error) {
	info, statErr := os.Stat(target)
	if statErr == nil && info.IsDir() {
		return filepath.Abs(target)
	}
	return target, nil
}

// generateTempPlaybook writes a minimal playbook that applies the given role
// inside the given container, and returns the path plus a cleanup function.
func generateTempPlaybook(container, role string) (string, func(), error) {
	content := fmt.Sprintf(`- name: "Test role: %s"
  hosts:
    - %s
  connection: docker
  gather_facts: false
  roles:
    - %s
`, role, container, role)

	f, err := os.CreateTemp("", "bolt-test-*.yaml")
	if err != nil {
		return "", nil, err
	}

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()

	// Create a symlink for the roles directory so the executor can find
	// bare role names (e.g. "webserver") relative to the temp playbook.
	var extraCleanup []string
	cwd, _ := os.Getwd()
	if cwd != "" {
		rolesDir := filepath.Join(cwd, "roles")
		if _, err := os.Stat(rolesDir); err == nil {
			linkPath := filepath.Join(filepath.Dir(f.Name()), "roles")
			if os.Symlink(rolesDir, linkPath) == nil {
				extraCleanup = append(extraCleanup, linkPath)
			}
		}
	}

	path := f.Name()
	cleanup := func() {
		os.Remove(path)
		for _, p := range extraCleanup {
			os.Remove(p)
		}
	}

	return path, cleanup, nil
}

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// stableContainerName derives a deterministic container name from the target.
func stableContainerName(target string) string {
	base := filepath.Base(target)
	base = strings.TrimSuffix(base, ".yaml")
	base = strings.TrimSuffix(base, ".yml")
	sanitized := sanitizeRe.ReplaceAllString(base, "-")
	return "bolt-test-" + sanitized
}

// ensureContainer makes sure a container with the given name is running.
// If forceNew is true, any existing container is removed first.
func ensureContainer(ctx context.Context, name, image string, forceNew bool) error {
	if forceNew {
		rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
		_ = rmCmd.Run() // ignore errors (container may not exist)
		return createContainer(ctx, name, image)
	}

	// Check if container already exists
	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", name)
	out, err := inspectCmd.Output()
	if err != nil {
		// Container doesn't exist, create it
		return createContainer(ctx, name, image)
	}

	status := strings.TrimSpace(string(out))
	switch status {
	case "running":
		fmt.Fprintf(os.Stderr, "Reusing running container: %s\n", name)
		return nil
	case "exited", "created":
		fmt.Fprintf(os.Stderr, "Starting stopped container: %s\n", name)
		startCmd := exec.CommandContext(ctx, "docker", "start", name)
		if out, err := startCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("starting container: %s: %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	default:
		// Unknown state, remove and recreate
		rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
		_ = rmCmd.Run()
		return createContainer(ctx, name, image)
	}
}

func createContainer(ctx context.Context, name, image string) error {
	fmt.Fprintf(os.Stderr, "Creating container: %s\n", name)
	runCmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", name, image, "sleep", "900")
	if out, err := runCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("starting container: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
