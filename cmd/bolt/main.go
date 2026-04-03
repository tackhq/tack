// Package main is the entrypoint for the bolt CLI.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	// Import modules to register them
	_ "github.com/eugenetaranov/bolt/internal/module/apt"
	_ "github.com/eugenetaranov/bolt/internal/module/blockinfile"
	_ "github.com/eugenetaranov/bolt/internal/module/brew"
	_ "github.com/eugenetaranov/bolt/internal/module/command"
	_ "github.com/eugenetaranov/bolt/internal/module/copy"
	_ "github.com/eugenetaranov/bolt/internal/module/file"
	_ "github.com/eugenetaranov/bolt/internal/module/lineinfile"
	_ "github.com/eugenetaranov/bolt/internal/module/systemd"
	_ "github.com/eugenetaranov/bolt/internal/module/template"
	_ "github.com/eugenetaranov/bolt/internal/module/waitfor"
	_ "github.com/eugenetaranov/bolt/internal/module/yum"

	"github.com/eugenetaranov/bolt/internal/executor"
	"github.com/eugenetaranov/bolt/internal/output"
	"github.com/eugenetaranov/bolt/internal/generate"
	"github.com/eugenetaranov/bolt/internal/inventory"
	"github.com/eugenetaranov/bolt/internal/module"
	"github.com/eugenetaranov/bolt/internal/playbook"
	"github.com/eugenetaranov/bolt/internal/source"
	"github.com/eugenetaranov/bolt/internal/testrun"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Global flags
var (
	debug       bool
	verbose     bool
	showDiff    bool
	dryRun      bool
	noColor     bool
	autoApprove bool
	outputMode  string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// signalContext returns a context that is cancelled on the first SIGINT/SIGTERM
// and exits the process on the second signal.
func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\nInterrupted, cleaning up...")
			stop()
			// Wait for second signal to force-exit
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			os.Exit(130)
		case <-ctx.Done():
			// Normal cancellation (defer cancel()), not a signal — stay quiet
		}
	}()
	return ctx, stop
}

// addConnectionFlags registers the common connection flags on a command.
func addConnectionFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayP("connection", "c", nil, "Connection URI (e.g. ssh://user@host:port, docker://container, local://)")
	cmd.Flags().String("hosts", "", "Comma-separated list of target hosts")
	cmd.Flags().String("ssh-user", "", "SSH username")
	cmd.Flags().Int("ssh-port", 0, "SSH port")
	cmd.Flags().String("ssh-key", "", "Path to SSH private key")
	cmd.Flags().String("ssh-password", "", "SSH password (prompted if flag present with no value)")
	cmd.Flags().Lookup("ssh-password").NoOptDefVal = ""
	cmd.Flags().Bool("ssh-insecure", false, "Skip SSH host key verification")
	cmd.Flags().BoolP("sudo", "s", false, "Enable sudo for all tasks")
	cmd.Flags().String("sudo-password", "", "Sudo password (prompted if flag present with no value)")
	cmd.Flags().Lookup("sudo-password").NoOptDefVal = ""
}

var rootCmd = &cobra.Command{
	Use:   "bolt",
	Short: "Bolt - System bootstrapping and configuration management",
	Long: `Bolt is a Go-based configuration management tool inspired by Ansible.
It helps you bootstrap and configure macOS and Linux systems using
simple YAML playbooks.

Supports local execution, SSH, and AWS SSM connectors.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug output with detailed task information")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&showDiff, "diff", false, "Show file content diffs in plan output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "check", false, "Alias for --dry-run")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&outputMode, "output", "text", "Output format: text or json")

	// Add subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(modulesCmd)
	rootCmd.AddCommand(moduleCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(scaffoldCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(vaultCmd)
}

// runCmd executes a playbook
var runCmd = &cobra.Command{
	Use:   "run <playbook.yaml | role-dir>",
	Short: "Run a playbook or role",
	Long: `Execute a playbook or role directory against the specified hosts.

If the argument is a directory, it is treated as a role and wrapped in
a temporary playbook. Connection and host settings come from CLI flags.

Connection flags override playbook values. Environment variables
(BOLT_CONNECTION, BOLT_HOSTS, BOLT_SSH_USER, BOLT_SSH_PORT,
BOLT_SSH_KEY, BOLT_SSH_PASSWORD) fill in when neither CLI flag
nor playbook provides a value.

The -c flag supports URI-style connection strings:
  ssh://host, ssh://user@host, ssh://user@host:port, ssh://user:pass@host:port
  docker://container-name
  local://

Multiple -c flags target multiple hosts:
  bolt run setup.yaml -c ssh://user@web1:2222 -c ssh://user@web2:2222

Explicit flags (--ssh-user, --ssh-port, etc.) override URI-derived values.

By default, bolt shows a plan of what will run and prompts for
confirmation before applying. Use --auto-approve to skip the prompt
(useful for CI/scripting), or --check/--dry-run to show the plan without applying.

Examples:
  bolt run setup.yaml
  bolt run setup.yaml --auto-approve
  bolt run setup.yaml --debug
  bolt run setup.yaml --check
  bolt run setup.yaml --dry-run
  bolt run setup.yaml -c ssh://deploy@web1:2222
  bolt run setup.yaml -c ssh://deploy@web1 -c ssh://deploy@web2
  bolt run setup.yaml -c ssh --hosts=web1,web2
  bolt run setup.yaml --ssh-user=deploy --ssh-key=~/.ssh/deploy_key
  BOLT_SSH_HOSTS=web1,web2 bolt run setup.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runPlaybook,
}

func init() {
	// Run-specific flags can be added here
	runCmd.Flags().StringP("inventory", "i", "", "Inventory file (YAML)")
	runCmd.Flags().StringSliceP("extra-vars", "e", nil, "Extra variables (key=value)")
	runCmd.Flags().StringSlice("tags", nil, "Only run tasks with these tags")
	runCmd.Flags().StringSlice("skip-tags", nil, "Skip tasks with these tags")
	// Connection override flags
	addConnectionFlags(runCmd)
	runCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip interactive approval prompt (for CI/scripting)")
	runCmd.Flags().IntP("forks", "f", 1, "Number of hosts to execute concurrently")

	// Vault flags
	runCmd.Flags().String("vault-password-file", "", "Path to file containing vault password (first line used)")

	// SSM flags
	runCmd.Flags().StringSlice("ssm-instances", nil, "Instance IDs for SSM (comma-separated)")
	runCmd.Flags().StringToString("ssm-tags", nil, "EC2 tags for SSM instance discovery (key=value,...)")
	runCmd.Flags().String("ssm-region", "", "AWS region for SSM")
	runCmd.Flags().String("ssm-bucket", "", "S3 bucket for SSM file transfer")
}

func runPlaybook(cmd *cobra.Command, args []string) error {
	// Resolve playbook source (local, git, s3, http)
	src, err := source.Resolve(args[0])
	if err != nil {
		return fmt.Errorf("invalid playbook source: %w", err)
	}

	ctx, cancel := signalContext(context.Background())
	defer cancel()

	playbookPath, cleanup, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch playbook: %w", err)
	}
	defer cleanup()

	// If the path is a directory, treat it as a role
	var pb *playbook.Playbook
	if info, statErr := os.Stat(playbookPath); statErr == nil && info.IsDir() {
		absRole, err := filepath.Abs(playbookPath)
		if err != nil {
			return fmt.Errorf("failed to resolve role path: %w", err)
		}
		pb = &playbook.Playbook{
			Path: absRole,
			Plays: []*playbook.Play{{
				Name:       fmt.Sprintf("Run role: %s", filepath.Base(absRole)),
				Hosts:      []string{"localhost"},
				Connection: "local",
				Roles:      []playbook.RoleRef{{Name: absRole}},
				Vars:       make(map[string]any),
			}},
		}
	} else {
		var err error
		pb, err = playbook.ParseFileRaw(playbookPath)
		if err != nil {
			return fmt.Errorf("failed to parse playbook: %w", err)
		}
	}

	// Build connection overrides from CLI flags and env vars
	overrides, err := buildConnOverrides(cmd)
	if err != nil {
		return err
	}

	// Load inventory file if provided
	var inv *inventory.Inventory
	if inventoryPath, _ := cmd.Flags().GetString("inventory"); inventoryPath != "" {
		var err error
		inv, err = inventory.Load(inventoryPath)
		if err != nil {
			return fmt.Errorf("failed to load inventory: %w", err)
		}
	}

	// Create output emitter
	emitter, err := output.NewEmitter(outputMode)
	if err != nil {
		return err
	}

	// JSON mode implies auto-approve (no interactive prompts on stdout)
	if outputMode == "json" {
		autoApprove = true
	}

	// Resolve forks: CLI flag > env var > default (1)
	forks, _ := cmd.Flags().GetInt("forks")
	if !cmd.Flags().Changed("forks") {
		if envForks := os.Getenv("BOLT_FORKS"); envForks != "" {
			if n, err := strconv.Atoi(envForks); err == nil {
				forks = n
			} else {
				return fmt.Errorf("invalid BOLT_FORKS value %q: %w", envForks, err)
			}
		}
	}
	if forks < 1 {
		return fmt.Errorf("--forks must be >= 1, got %d", forks)
	}

	// Create executor
	exec := executor.New()
	exec.Output = emitter
	exec.Debug = debug
	exec.Verbose = verbose
	exec.ShowDiff = showDiff
	exec.DryRun = dryRun
	exec.AutoApprove = autoApprove
	exec.Forks = forks
	exec.Overrides = overrides
	exec.Inventory = inv
	exec.PromptSudoPassword = func() (string, error) {
		fmt.Fprint(os.Stderr, "Sudo password: ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(passBytes), nil
	}
	// Vault password resolution: env > file > prompt (D-01)
	vaultPwFile, _ := cmd.Flags().GetString("vault-password-file")
	if envPw := os.Getenv("BOLT_VAULT_PASSWORD"); envPw != "" {
		pw := []byte(envPw)
		exec.ResolveVaultPassword = func() ([]byte, error) { return pw, nil }
	} else if vaultPwFile != "" {
		data, err := os.ReadFile(vaultPwFile)
		if err != nil {
			return fmt.Errorf("--vault-password-file: %w", err)
		}
		// First line only, trim trailing newline (D-03)
		line := strings.SplitN(string(data), "\n", 2)[0]
		pw := []byte(line)
		exec.ResolveVaultPassword = func() ([]byte, error) { return pw, nil }
	} else {
		exec.ResolveVaultPassword = func() ([]byte, error) {
			fmt.Fprint(os.Stderr, "Vault password: ")
			passBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			return passBytes, err
		}
	}
	// Wire tag filters
	if tags, _ := cmd.Flags().GetStringSlice("tags"); len(tags) > 0 {
		exec.Tags = tags
	}
	if skipTags, _ := cmd.Flags().GetStringSlice("skip-tags"); len(skipTags) > 0 {
		exec.SkipTags = skipTags
	}

	exec.Output.SetColor(!noColor)
	exec.Output.SetDebug(debug)
	exec.Output.SetVerbose(verbose)
	exec.Output.SetDiff(showDiff)

	// Run playbook
	result, err := exec.Run(ctx, pb)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	if !result.Success {
		os.Exit(1)
	}

	return nil
}

// validateCmd validates a playbook without running it
var validateCmd = &cobra.Command{
	Use:   "validate <playbook.yaml> [playbook2.yaml ...]",
	Short: "Validate one or more playbooks",
	Long: `Parse and validate playbooks without executing them.

This checks for:
  - Valid YAML syntax
  - Required fields (hosts, tasks)
  - Valid module names
  - Task structure

Examples:
  bolt validate setup.yaml
  bolt validate *.yaml`,
	Args: cobra.MinimumNArgs(1),
	RunE: validatePlaybooks,
}

func validatePlaybooks(cmd *cobra.Command, args []string) error {
	var hasErrors bool

	for _, playbookPath := range args {
		if err := validatePlaybook(playbookPath); err != nil {
			fmt.Printf("FAIL: %s - %v\n", playbookPath, err)
			hasErrors = true
		} else {
			fmt.Printf("OK: %s\n", playbookPath)
		}
	}

	if hasErrors {
		return fmt.Errorf("one or more playbooks failed validation")
	}

	fmt.Printf("\nAll %d playbook(s) valid.\n", len(args))
	return nil
}

func validatePlaybook(ref string) error {
	src, err := source.Resolve(ref)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	playbookPath, cleanup, err := src.Fetch(context.Background())
	if err != nil {
		return err
	}
	defer cleanup()

	// Parse playbook
	pb, err := playbook.ParseFileRaw(playbookPath)
	if err != nil {
		return err
	}

	// Validate modules exist
	var errors []string
	for _, play := range pb.Plays {
		for _, task := range play.Tasks {
			playbook.ExpandShorthand(task)
			if err := playbook.ResolveModule(task); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", task.String(), err))
			}
		}
		for _, handler := range play.Handlers {
			playbook.ExpandShorthand(handler)
			if err := playbook.ResolveModule(handler); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", handler.String(), err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d error(s): %s", len(errors), errors[0])
	}

	return nil
}

// modulesCmd lists available modules
var modulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List available modules",
	Long:  `Display a list of all available modules that can be used in playbooks.`,
	Run: func(cmd *cobra.Command, args []string) {
		modules := module.List()
		if len(modules) == 0 {
			fmt.Println("No modules registered.")
			return
		}

		fmt.Println("Available modules:")
		fmt.Println()
		for _, name := range modules {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
		fmt.Printf("Total: %d modules\n", len(modules))
	},
}

// moduleCmd shows documentation for a specific module.
var moduleCmd = &cobra.Command{
	Use:   "module [name]",
	Short: "Show module documentation",
	Long: `Display detailed documentation for a specific module including
parameters, types, defaults, and descriptions.

If no module name is given, lists all available modules.

Examples:
  bolt module apt
  bolt module file
  bolt module`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			modulesCmd.Run(cmd, args)
			return nil
		}

		name := args[0]
		mod := module.Get(name)
		if mod == nil {
			modules := module.List()
			return fmt.Errorf("unknown module %q\n\nAvailable modules: %s", name, strings.Join(modules, ", "))
		}

		desc, ok := mod.(module.Describer)
		if !ok {
			fmt.Printf("Module: %s\n\nNo documentation available.\n", name)
			return nil
		}

		fmt.Printf("Module: %s\n\n  %s\n\nParameters:\n\n", name, desc.Description())
		for _, p := range desc.Parameters() {
			req := ""
			if p.Required {
				req = " (required)"
			}
			def := ""
			if p.Default != "" {
				def = fmt.Sprintf(" [default: %s]", p.Default)
			}
			fmt.Printf("  %-20s %s%s%s\n", p.Name, p.Description, req, def)
			fmt.Printf("  %-20s type: %s\n\n", "", p.Type)
		}
		return nil
	},
}

// generateCmd captures live system resources and outputs a playbook.
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Capture live system state as a playbook",
	Long: `Connect to a target system, read the current state of specified resources,
and generate a ready-to-use playbook YAML.

Specify which resources to capture using --packages, --files, --services,
and --users flags. At least one resource flag is required.

Examples:
  bolt generate --packages neovim,tmux
  bolt generate -c ssh://root@web1 --packages nginx --services nginx --files /etc/nginx/nginx.conf
  bolt generate --connection local --files /etc/hosts -o setup.yaml
  bolt generate -c ssh://deploy@web1 --users deploy,app --sudo`,
	RunE: runGenerate,
}

func init() {
	// Resource flags
	generateCmd.Flags().StringSlice("packages", nil, "Packages to capture (comma-separated)")
	generateCmd.Flags().StringSlice("files", nil, "Files/directories to capture (comma-separated)")
	generateCmd.Flags().StringSlice("services", nil, "Systemd services to capture (comma-separated)")
	generateCmd.Flags().StringSlice("users", nil, "Users to capture (comma-separated)")
	generateCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")

	// Connection flags (same as run command)
	addConnectionFlags(generateCmd)
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	packages, _ := cmd.Flags().GetStringSlice("packages")
	files, _ := cmd.Flags().GetStringSlice("files")
	services, _ := cmd.Flags().GetStringSlice("services")
	users, _ := cmd.Flags().GetStringSlice("users")
	output, _ := cmd.Flags().GetString("output")

	if len(packages) == 0 && len(files) == 0 && len(services) == 0 && len(users) == 0 {
		return fmt.Errorf("at least one resource flag is required (--packages, --files, --services, --users)")
	}

	// Build connection overrides
	overrides, err := buildConnOverrides(cmd)
	if err != nil {
		return err
	}

	// Build a minimal play to get a connector
	play := &playbook.Play{
		Vars: make(map[string]any),
	}
	if overrides.Connection == "" {
		overrides.Connection = "local"
	}

	exec := executor.New()
	exec.Overrides = overrides
	exec.ApplyOverrides(play)

	host := "localhost"
	if len(play.Hosts) > 0 {
		host = play.Hosts[0]
	}
	conn, err := exec.GetConnector(play, host)
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}

	ctx, cancel := signalContext(context.Background())
	defer cancel()

	opts := generate.Options{
		Packages:   packages,
		Files:      files,
		Services:   services,
		Users:      users,
		Hosts:      play.Hosts,
		Connection: play.Connection,
		Output:     output,
	}

	return generate.Generate(ctx, conn, opts)
}

// scaffoldCmd generates a sample role directory structure.
var scaffoldCmd = &cobra.Command{
	Use:   "scaffold <rolename>",
	Short: "Generate a sample role directory structure",
	Long: `Create a new role with sample files demonstrating all resource types
(packages, files, services, templates).

Examples:
  bolt scaffold myrole
  bolt scaffold myrole --path ./my-roles`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		name := args[0]
		if err := generate.ScaffoldRole(name, path); err != nil {
			return err
		}
		fmt.Printf("Created role %s at %s/%s\n", name, path, name)
		return nil
	},
}

func init() {
	scaffoldCmd.Flags().String("path", "roles", "Base directory for the role")
}

// testCmd tests a role or playbook in an ephemeral Docker container.
var testCmd = &cobra.Command{
	Use:   "test <playbook.yaml | rolename>",
	Short: "Test a role or playbook in an ephemeral Docker container",
	Long: `Run a role or playbook in a Docker container and report results.

Containers are reused by default — the container name is derived from the
target so repeated runs hit the same container, letting you verify
idempotency. Use --new to force a fresh container or --rm to remove
the container after the run.

If the argument ends in .yaml/.yml or is an existing file, it is treated
as a playbook. Otherwise it is treated as a role name (looked up under ./roles/).

The playbook's connection and hosts are overridden to use the Docker container.

Examples:
  bolt test myrole                  # reuse or create, keep after
  bolt test myrole --new            # force fresh container
  bolt test myrole --rm             # remove container after run
  bolt test myrole --new --rm       # fresh + disposable (one-shot)
  bolt test playbook.yaml
  bolt test myrole --image debian:12`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		image, _ := cmd.Flags().GetString("image")
		newFlag, _ := cmd.Flags().GetBool("new")
		rmFlag, _ := cmd.Flags().GetBool("rm")

		ctx, cancel := signalContext(context.Background())
		defer cancel()

		return testrun.Run(ctx, testrun.Options{
			Target:  args[0],
			Image:   image,
			New:     newFlag,
			Remove:  rmFlag,
			Debug:   debug,
			Verbose: verbose,
			DryRun:  dryRun,
			NoColor: noColor,
		})
	},
}

func init() {
	testCmd.Flags().String("image", "ubuntu:24.04", "Docker image to use for the test container")
	testCmd.Flags().Bool("new", false, "Force a fresh container (remove existing first)")
	testCmd.Flags().Bool("rm", false, "Remove the container after the test run")
}

// flagOrEnv returns the flag value if changed, otherwise the environment variable value.
func flagOrEnv(cmd *cobra.Command, flagName, envVar string) string {
	if cmd.Flags().Changed(flagName) {
		val, _ := cmd.Flags().GetString(flagName)
		return val
	}
	return os.Getenv(envVar)
}

// buildConnOverrides constructs connection overrides from CLI flags and env vars.
func buildConnOverrides(cmd *cobra.Command) (*executor.ConnOverrides, error) {
	// Start with URI-derived values from -c flags
	var o *executor.ConnOverrides

	if cmd.Flags().Changed("connection") {
		connVals, _ := cmd.Flags().GetStringArray("connection")
		merged, err := executor.MergeConnectionURIs(connVals)
		if err != nil {
			return nil, fmt.Errorf("invalid connection flag: %w", err)
		}
		o = merged
	} else if envConn := os.Getenv("BOLT_CONNECTION"); envConn != "" {
		o = &executor.ConnOverrides{Connection: envConn}
	} else {
		o = &executor.ConnOverrides{}
	}

	// --hosts overrides any URI-derived hosts
	hostsStr := flagOrEnv(cmd, "hosts", "BOLT_HOSTS")
	if hostsStr != "" {
		o.Hosts = nil
		for _, h := range strings.Split(hostsStr, ",") {
			h = strings.TrimSpace(h)
			if h != "" {
				o.Hosts = append(o.Hosts, h)
			}
		}
	}

	// Explicit flags override URI-derived SSH values
	if cmd.Flags().Changed("ssh-user") {
		o.SSHUser, _ = cmd.Flags().GetString("ssh-user")
	} else if envUser := os.Getenv("BOLT_SSH_USER"); envUser != "" && o.SSHUser == "" {
		o.SSHUser = envUser
	}

	if cmd.Flags().Changed("ssh-key") {
		o.SSHKey, _ = cmd.Flags().GetString("ssh-key")
	} else if envKey := os.Getenv("BOLT_SSH_KEY"); envKey != "" {
		o.SSHKey = envKey
	}

	// SSH port: explicit flag > env > URI-derived > 0
	if cmd.Flags().Changed("ssh-port") {
		o.SSHPort, _ = cmd.Flags().GetInt("ssh-port")
	} else if envPort := os.Getenv("BOLT_SSH_PORT"); envPort != "" && o.SSHPort == 0 {
		port, err := strconv.Atoi(envPort)
		if err != nil {
			return nil, fmt.Errorf("invalid BOLT_SSH_PORT: %w", err)
		}
		o.SSHPort = port
	}

	// SSH password: explicit flag > env > URI-derived
	if cmd.Flags().Changed("ssh-password") {
		o.HasSSHPass = true
		val, _ := cmd.Flags().GetString("ssh-password")
		if val == "" {
			// Prompt interactively
			fmt.Fprint(os.Stderr, "SSH password: ")
			passBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return nil, fmt.Errorf("failed to read password: %w", err)
			}
			o.SSHPass = string(passBytes)
		} else {
			o.SSHPass = val
		}
	} else if envPass := os.Getenv("BOLT_SSH_PASSWORD"); envPass != "" && !o.HasSSHPass {
		o.HasSSHPass = true
		o.SSHPass = envPass
	}

	// SSH insecure: flag > env > false
	if cmd.Flags().Changed("ssh-insecure") {
		o.SSHInsecure, _ = cmd.Flags().GetBool("ssh-insecure")
	} else if envInsecure := os.Getenv("BOLT_SSH_INSECURE"); envInsecure != "" {
		o.SSHInsecure = envInsecure == "1" || envInsecure == "true" || envInsecure == "yes"
	}

	// Sudo: flag > false
	if cmd.Flags().Changed("sudo") {
		o.Sudo, _ = cmd.Flags().GetBool("sudo")
	}

	// Sudo password: explicit flag > env
	if cmd.Flags().Changed("sudo-password") {
		val, _ := cmd.Flags().GetString("sudo-password")
		if val == "" {
			// Prompt interactively
			fmt.Fprint(os.Stderr, "Sudo password: ")
			passBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return nil, fmt.Errorf("failed to read sudo password: %w", err)
			}
			o.SudoPassword = string(passBytes)
		} else {
			o.SudoPassword = val
		}
	} else if envPass := os.Getenv("BOLT_SUDO_PASSWORD"); envPass != "" {
		o.SudoPassword = envPass
	}

	// SSM: instances
	if cmd.Flags().Changed("ssm-instances") {
		o.SSMInstances, _ = cmd.Flags().GetStringSlice("ssm-instances")
	} else if envInst := os.Getenv("BOLT_SSM_INSTANCES"); envInst != "" {
		for _, id := range strings.Split(envInst, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				o.SSMInstances = append(o.SSMInstances, id)
			}
		}
	}

	// SSM: tags
	if cmd.Flags().Changed("ssm-tags") {
		o.SSMTags, _ = cmd.Flags().GetStringToString("ssm-tags")
	} else if envTags := os.Getenv("BOLT_SSM_TAGS"); envTags != "" {
		o.SSMTags = make(map[string]string)
		for _, kv := range strings.Split(envTags, ",") {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				o.SSMTags[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	// SSM: region
	if cmd.Flags().Changed("ssm-region") {
		o.SSMRegion, _ = cmd.Flags().GetString("ssm-region")
	} else if envRegion := os.Getenv("BOLT_SSM_REGION"); envRegion != "" {
		o.SSMRegion = envRegion
	}

	// SSM: bucket
	if cmd.Flags().Changed("ssm-bucket") {
		o.SSMBucket, _ = cmd.Flags().GetString("ssm-bucket")
	} else if envBucket := os.Getenv("BOLT_SSM_BUCKET"); envBucket != "" {
		o.SSMBucket = envBucket
	}

	// Infer connection type from protocol-specific flags
	if o.Connection == "" {
		if o.SSHUser != "" || o.SSHKey != "" || o.SSHPort != 0 || o.HasSSHPass {
			o.Connection = "ssh"
			o.ConnectionInferred = true
		} else if len(o.SSMInstances) > 0 || len(o.SSMTags) > 0 {
			o.Connection = "ssm"
			o.ConnectionInferred = true
		} else if len(o.Hosts) > 0 && !isLocalHost(o.Hosts) {
			o.Connection = "ssh"
			o.ConnectionInferred = true
		}
	}

	return o, nil
}

// isLocalHost returns true when every entry in hosts resolves to localhost.
func isLocalHost(hosts []string) bool {
	for _, h := range hosts {
		// Strip port if present
		if host, _, err := net.SplitHostPort(h); err == nil {
			h = host
		}
		switch strings.ToLower(h) {
		case "localhost", "127.0.0.1", "::1":
		default:
			return false
		}
	}
	return true
}
