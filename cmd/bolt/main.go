// Package main is the entrypoint for the bolt CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	// Import modules to register them
	_ "github.com/eugenetaranov/bolt/internal/module/apt"
	_ "github.com/eugenetaranov/bolt/internal/module/brew"
	_ "github.com/eugenetaranov/bolt/internal/module/command"
	_ "github.com/eugenetaranov/bolt/internal/module/copy"
	_ "github.com/eugenetaranov/bolt/internal/module/file"
	_ "github.com/eugenetaranov/bolt/internal/module/template"

	"github.com/eugenetaranov/bolt/internal/executor"
	"github.com/eugenetaranov/bolt/internal/module"
	"github.com/eugenetaranov/bolt/internal/playbook"
	"github.com/eugenetaranov/bolt/internal/source"
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
	dryRun      bool
	noColor     bool
	autoApprove bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show full diffs in plan output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// Add subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(modulesCmd)
}

// runCmd executes a playbook
var runCmd = &cobra.Command{
	Use:   "run <playbook.yaml>",
	Short: "Run a playbook",
	Long: `Execute a playbook against the specified hosts.

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
(useful for CI/scripting), or --dry-run to show the plan without applying.

Examples:
  bolt run setup.yaml
  bolt run setup.yaml --auto-approve
  bolt run setup.yaml --debug
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
	runCmd.Flags().StringP("inventory", "i", "", "Inventory file (not yet implemented)")
	runCmd.Flags().StringSliceP("extra-vars", "e", nil, "Extra variables (key=value)")
	runCmd.Flags().StringSlice("tags", nil, "Only run tasks with these tags")
	runCmd.Flags().StringSlice("skip-tags", nil, "Skip tasks with these tags")
	runCmd.Flags().IntP("forks", "f", 1, "Number of parallel processes (not yet implemented)")

	// Connection override flags
	runCmd.Flags().StringArrayP("connection", "c", nil, "Connection URI (e.g. ssh://user@host:port, docker://container, local://)")
	runCmd.Flags().String("hosts", "", "Comma-separated list of target hosts")
	runCmd.Flags().String("ssh-user", "", "SSH username")
	runCmd.Flags().Int("ssh-port", 0, "SSH port")
	runCmd.Flags().String("ssh-key", "", "Path to SSH private key")
	sshPassFlag := runCmd.Flags().String("ssh-password", "", "SSH password (prompted if flag present with no value)")
	_ = sshPassFlag
	runCmd.Flags().Lookup("ssh-password").NoOptDefVal = ""
	runCmd.Flags().Bool("ssh-insecure", false, "Skip SSH host key verification")
	runCmd.Flags().BoolP("sudo", "s", false, "Enable sudo for all tasks")
	sudoPassFlag := runCmd.Flags().String("sudo-password", "", "Sudo password (prompted if flag present with no value)")
	_ = sudoPassFlag
	runCmd.Flags().Lookup("sudo-password").NoOptDefVal = ""
	runCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip interactive approval prompt (for CI/scripting)")
}

func runPlaybook(cmd *cobra.Command, args []string) error {
	// Resolve playbook source (local, git, s3, http)
	src, err := source.Resolve(args[0])
	if err != nil {
		return fmt.Errorf("invalid playbook source: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	playbookPath, cleanup, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch playbook: %w", err)
	}
	defer cleanup()

	// Parse playbook
	pb, err := playbook.ParseFileRaw(playbookPath)
	if err != nil {
		return fmt.Errorf("failed to parse playbook: %w", err)
	}

	// Build connection overrides from CLI flags and env vars
	overrides, err := buildConnOverrides(cmd)
	if err != nil {
		return err
	}

	// Create executor
	exec := executor.New()
	exec.Debug = debug
	exec.Verbose = verbose
	exec.DryRun = dryRun
	exec.AutoApprove = autoApprove
	exec.Overrides = overrides
	exec.PromptSudoPassword = func() (string, error) {
		fmt.Fprint(os.Stderr, "Sudo password: ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(passBytes), nil
	}
	exec.Output.SetColor(!noColor)
	exec.Output.SetDebug(debug)
	exec.Output.SetVerbose(verbose)

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nInterrupted, cleaning up...")
		cancel()
	}()

	// Run playbook
	result, err := exec.Run(ctx, pb)
	if err != nil {
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

	return o, nil
}
