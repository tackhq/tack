// Package playbook defines the structure and parsing of Bolt playbooks.
package playbook

import (
	"fmt"
	"sort"
	"strings"
)

// Playbook represents a complete playbook with one or more plays.
type Playbook struct {
	// Path is the file path the playbook was loaded from.
	Path string

	// Plays is the list of plays in the playbook.
	Plays []*Play
}

// SSHConfig holds SSH connection parameters for a play.
type SSHConfig struct {
	// User is the SSH username.
	User string `yaml:"user"`

	// Port is the SSH port (default: 22).
	Port int `yaml:"port"`

	// Key is the path to the SSH private key file.
	Key string `yaml:"key"`

	// Password is the SSH password (if not using key auth).
	Password string `yaml:"password"`

	// HostKeyChecking controls whether the host key is verified.
	// nil means use the default (true / strict). Set to false to disable.
	HostKeyChecking *bool `yaml:"host_key_checking"`
}

// SSMConfig holds AWS Systems Manager connection parameters for a play.
type SSMConfig struct {
	// Region is the AWS region for SSM (e.g. "us-east-1").
	Region string `yaml:"region"`

	// Bucket is the S3 bucket used for large file transfers (optional).
	Bucket string `yaml:"bucket"`

	// Instances is a list of EC2 instance IDs to target.
	// Convenience alias for the play-level hosts field when connection is ssm.
	Instances []string `yaml:"instances"`

	// Tags selects EC2 instances by tag at runtime (mutually exclusive with Instances).
	Tags map[string]string `yaml:"tags"`
}

// Play represents a single play targeting a set of hosts.
type Play struct {
	// Name is an optional description of the play.
	Name string `yaml:"name"`

	// Hosts specifies which hosts to target.
	Hosts []string `yaml:"hosts"`

	// Connection specifies how to connect (local, ssh, ssm).
	Connection string `yaml:"connection"`

	// Vars defines variables available to all tasks in the play.
	Vars map[string]any `yaml:"vars"`

	// Roles is the list of roles to include in the play.
	Roles []string `yaml:"roles"`

	// Tasks is the list of tasks to execute.
	Tasks []*Task `yaml:"tasks"`

	// Handlers are tasks triggered by notify.
	Handlers []*Task `yaml:"handlers"`

	// Sudo enables privilege escalation.
	Sudo bool `yaml:"sudo"`

	// GatherFacts controls whether to gather system facts (default: true).
	GatherFacts *bool `yaml:"gather_facts"`

	// SSH holds SSH connection configuration (used when connection: ssh).
	SSH *SSHConfig `yaml:"ssh"`

	// SSM holds AWS SSM connection configuration (used when connection: ssm).
	SSM *SSMConfig `yaml:"ssm"`

	// SudoPassword is the password for privilege escalation.
	SudoPassword string `yaml:"sudo_password"`

	// VarsFiles is a list of YAML file paths whose variables are loaded
	// and merged into play variables. Paths starting with "?" are optional
	// (no error if missing). Files are relative to the playbook directory.
	VarsFiles []string `yaml:"vars_files"`

	// VaultFile is the path to an encrypted vault file whose variables
	// are merged into play vars at runtime.
	VaultFile string `yaml:"vault_file"`
}

// Task represents a single task in a play.
type Task struct {
	// Name is a description of the task.
	Name string `yaml:"name"`

	// Module is the name of the module to execute.
	Module string `yaml:"-"`

	// Params are the parameters to pass to the module.
	Params map[string]any `yaml:"-"`

	// Include is a path/URL to an external tasks file to include.
	Include string `yaml:"-"`

	// RolePath is the path to the role this task belongs to (empty for play tasks).
	RolePath string `yaml:"-"`

	// When is a conditional expression; task runs only if true.
	When string `yaml:"when"`

	// Register stores the task result in a variable with this name.
	Register string `yaml:"register"`

	// Notify lists handlers to trigger if the task changes something.
	Notify []string `yaml:"-"`

	// Loop iterates the task over a list of items.
	Loop []any `yaml:"-"`

	// LoopExpr is a variable reference (e.g. "{{ windmill_files }}") resolved at runtime.
	LoopExpr string `yaml:"-"`

	// LoopVar is the variable name for the current item (default: "item").
	LoopVar string `yaml:"loop_var"`

	// IgnoreErrors continues execution even if the task fails.
	IgnoreErrors bool `yaml:"ignore_errors"`

	// Retries is the number of times to retry on failure.
	Retries int `yaml:"retries"`

	// Delay is seconds to wait between retries.
	Delay int `yaml:"delay"`

	// Sudo enables privilege escalation for this task.
	Sudo *bool `yaml:"sudo"`

	// Changed controls when the task reports as changed.
	// Can be a boolean or a conditional expression.
	ChangedWhen string `yaml:"changed_when"`

	// Failed controls when the task reports as failed.
	FailedWhen string `yaml:"failed_when"`
}

// Role represents an Ansible-compatible role with tasks, handlers, and variables.
type Role struct {
	// Name is the role name (directory name).
	Name string

	// Path is the absolute path to the role directory.
	Path string

	// Tasks loaded from tasks/main.yaml.
	Tasks []*Task

	// Handlers loaded from handlers/main.yaml.
	Handlers []*Task

	// Vars loaded from vars/main.yaml (higher priority).
	Vars map[string]any

	// Defaults loaded from defaults/main.yaml (lower priority).
	Defaults map[string]any
}

// ShouldGatherFacts returns whether facts should be gathered for this play.
func (p *Play) ShouldGatherFacts() bool {
	if p.GatherFacts == nil {
		return true // default
	}
	return *p.GatherFacts
}

// GetConnection returns the connection type, defaulting to "local".
func (p *Play) GetConnection() string {
	if p.Connection == "" {
		return "local"
	}
	return p.Connection
}

// ShouldSudo returns whether privilege escalation is enabled for this task.
func (t *Task) ShouldSudo(playSudo bool) bool {
	if t.Sudo != nil {
		return *t.Sudo
	}
	return playSudo
}

// GetLoopVar returns the loop variable name, defaulting to "item".
func (t *Task) GetLoopVar() string {
	if t.LoopVar == "" {
		return "item"
	}
	return t.LoopVar
}

// Validate checks the play for common errors.
func (p *Play) Validate() error {
	conn := p.GetConnection()
	switch conn {
	case "local", "docker", "ssh", "ssm":
		// Valid
	default:
		return fmt.Errorf("invalid connection type: %s (must be local, docker, ssh, or ssm)", conn)
	}

	for i, task := range p.Tasks {
		if err := task.Validate(); err != nil {
			taskName := task.Name
			if taskName == "" {
				taskName = fmt.Sprintf("task %d", i+1)
			}
			return fmt.Errorf("%s: %w", taskName, err)
		}
	}

	for i, handler := range p.Handlers {
		if err := handler.Validate(); err != nil {
			handlerName := handler.Name
			if handlerName == "" {
				handlerName = fmt.Sprintf("handler %d", i+1)
			}
			return fmt.Errorf("%s: %w", handlerName, err)
		}
		if handler.Name == "" {
			return fmt.Errorf("handler %d: handlers must have a name for notify to reference", i+1)
		}
	}

	return nil
}

// Validate checks the task for common errors.
func (t *Task) Validate() error {
	if t.Module == "" && t.Include == "" {
		return fmt.Errorf("task has no module specified")
	}

	if t.Retries < 0 {
		return fmt.Errorf("retries cannot be negative")
	}

	if t.Delay < 0 {
		return fmt.Errorf("delay cannot be negative")
	}

	return nil
}

// String returns a human-readable description of the task.
func (t *Task) String() string {
	if t.Name != "" {
		return t.Name
	}
	return fmt.Sprintf("%s: %v", t.Module, summarizeParams(t.Params))
}

// summarizeParams creates a brief summary of task parameters.
func summarizeParams(params map[string]any) string {
	if len(params) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := params[k]
		switch val := v.(type) {
		case string:
			if len(val) > 30 {
				val = val[:27] + "..."
			}
			parts = append(parts, fmt.Sprintf("%s=%q", k, val))
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		if len(parts) >= 3 {
			parts = append(parts, "...")
			break
		}
	}

	return "{" + strings.Join(parts, ", ") + "}"
}
