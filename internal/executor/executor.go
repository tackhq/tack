// Package executor runs playbooks against target hosts.
package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
	"github.com/eugenetaranov/bolt/internal/connector/docker"
	"github.com/eugenetaranov/bolt/internal/connector/local"
	sshconn "github.com/eugenetaranov/bolt/internal/connector/ssh"
	"github.com/eugenetaranov/bolt/internal/module"
	"github.com/eugenetaranov/bolt/internal/output"
	"github.com/eugenetaranov/bolt/internal/playbook"
	"github.com/eugenetaranov/bolt/internal/source"
	"github.com/eugenetaranov/bolt/pkg/facts"
)

// ConnOverrides holds CLI/env overrides for connection settings.
type ConnOverrides struct {
	Connection   string
	Hosts        []string
	SSHUser      string
	SSHPort      int
	SSHKey       string
	SSHPass      string
	HasSSHPass   bool // true when --ssh-password flag was explicitly provided
	SSHInsecure  bool
	Sudo         bool
	SudoPassword string
}

// Executor runs playbooks.
type Executor struct {
	// Output handles formatted output.
	Output *output.Output

	// DryRun only shows what would be done without making changes.
	DryRun bool

	// AutoApprove skips the interactive approval prompt.
	AutoApprove bool

	// Debug enables detailed output.
	Debug bool

	// Verbose enables full diffs in plan output.
	Verbose bool

	// Overrides holds CLI/env connection overrides applied to each play.
	Overrides *ConnOverrides

	// PromptSudoPassword is called to prompt the user for a sudo password
	// when tasks require sudo but no password was provided.
	PromptSudoPassword func() (string, error)

	// connectors caches connectors by host.
	connectors map[string]connector.Connector
}

// New creates a new executor.
func New() *Executor {
	return &Executor{
		Output:     output.New(os.Stdout),
		connectors: make(map[string]connector.Connector),
	}
}

// RunResult holds the result of a playbook run.
type RunResult struct {
	// Success is true if all plays completed successfully.
	Success bool

	// Stats holds execution statistics.
	Stats *Stats
}

// Stats holds execution statistics.
type Stats struct {
	Plays     int
	Tasks     int
	OK        int
	Changed   int
	Failed    int
	Skipped   int
	StartTime time.Time
	EndTime   time.Time
}

// Duration returns the total execution time.
func (s *Stats) Duration() time.Duration {
	return s.EndTime.Sub(s.StartTime)
}

// RecordResult increments the appropriate counter based on task status.
func (s *Stats) RecordResult(status string) {
	switch status {
	case "ok":
		s.OK++
	case "changed":
		s.Changed++
	case "skipped":
		s.Skipped++
	}
}

// GetOK returns the OK count (implements output.Stats).
func (s *Stats) GetOK() int { return s.OK }

// GetChanged returns the Changed count (implements output.Stats).
func (s *Stats) GetChanged() int { return s.Changed }

// GetFailed returns the Failed count (implements output.Stats).
func (s *Stats) GetFailed() int { return s.Failed }

// GetSkipped returns the Skipped count (implements output.Stats).
func (s *Stats) GetSkipped() int { return s.Skipped }

// GetDuration returns the duration (implements output.Stats).
func (s *Stats) GetDuration() time.Duration { return s.Duration() }

// PlayContext holds state for a play execution.
type PlayContext struct {
	// Play is the current play.
	Play *playbook.Play

	// Vars holds all variables (play vars + facts + registered).
	Vars map[string]any

	// Facts holds gathered system facts.
	Facts map[string]any

	// Registered holds task results stored via register.
	Registered map[string]any

	// NotifiedHandlers tracks which handlers should run.
	NotifiedHandlers map[string]bool

	// Connector is the connection to the target.
	Connector connector.Connector
}

// Run executes a playbook.
func (e *Executor) Run(ctx context.Context, pb *playbook.Playbook) (*RunResult, error) {
	stats := &Stats{
		StartTime: time.Now(),
		Plays:     len(pb.Plays),
	}

	result := &RunResult{
		Success: true,
		Stats:   stats,
	}

	e.Output.PlaybookStart(pb.Path)

	// Determine roles directory (relative to playbook)
	rolesDir := filepath.Join(filepath.Dir(pb.Path), "roles")

	for _, play := range pb.Plays {
		e.applyOverrides(play)
		if err := e.runPlay(ctx, play, stats, rolesDir); err != nil {
			result.Success = false
			e.Output.Error("Play failed: %v", err)
			break
		}
	}

	stats.EndTime = time.Now()
	e.Output.PlaybookEnd(stats)

	return result, nil
}

// applyOverrides applies CLI/env connection overrides to a play.
func (e *Executor) applyOverrides(play *playbook.Play) {
	if e.Overrides == nil {
		return
	}
	o := e.Overrides

	if o.Connection != "" {
		play.Connection = o.Connection
	}
	if len(o.Hosts) > 0 {
		play.Hosts = o.Hosts
	}

	if play.Vars == nil {
		play.Vars = make(map[string]any)
	}
	if o.SSHUser != "" {
		play.Vars["bolt_ssh_user"] = o.SSHUser
	}
	if o.SSHPort != 0 {
		play.Vars["bolt_ssh_port"] = o.SSHPort
	}
	if o.SSHKey != "" {
		play.Vars["bolt_ssh_key"] = o.SSHKey
	}
	if o.HasSSHPass {
		play.Vars["bolt_ssh_password"] = o.SSHPass
	}
	if o.SSHInsecure {
		play.Vars["bolt_ssh_host_key_checking"] = false
	}
	if o.Sudo {
		play.Sudo = true
	}
	if o.SudoPassword != "" {
		play.Vars["bolt_sudo_password"] = o.SudoPassword
	}
}

// runPlay executes a single play.
func (e *Executor) runPlay(ctx context.Context, play *playbook.Play, stats *Stats, rolesDir string) error {
	// Validate hosts after overrides have been applied (non-local connections need hosts)
	if play.GetConnection() != "local" && len(play.Hosts) == 0 {
		return fmt.Errorf("play is missing 'hosts' (provide via playbook or -c flag)")
	}

	// Load roles if specified
	var roles []*playbook.Role
	if len(play.Roles) > 0 {
		var err error
		roles, err = playbook.LoadRoles(play.Roles, rolesDir)
		if err != nil {
			return fmt.Errorf("failed to load roles: %w", err)
		}
	}

	// Prompt for sudo password before any per-host output
	allTasks := playbook.ExpandRoleTasks(roles, play.Tasks)
	allHandlers := playbook.ExpandRoleHandlers(roles, play.Handlers)
	if err := e.needsSudoPassword(play, allTasks, allHandlers); err != nil {
		return err
	}

	e.Output.PlayStart(play)

	// For local connection, run once regardless of hosts list
	if play.GetConnection() == "local" {
		return e.runPlayOnHost(ctx, play, stats, roles, "localhost")
	}

	// For remote connections, iterate over each host
	for _, host := range play.Hosts {
		if err := e.runPlayOnHost(ctx, play, stats, roles, host); err != nil {
			return err
		}
	}

	return nil
}

// runPlayOnHost executes a play against a single host.
func (e *Executor) runPlayOnHost(ctx context.Context, play *playbook.Play, stats *Stats, roles []*playbook.Role, host string) error {
	// Create play context
	pctx := &PlayContext{
		Play:             play,
		Vars:             make(map[string]any),
		Facts:            make(map[string]any),
		Registered:       make(map[string]any),
		NotifiedHandlers: make(map[string]bool),
	}

	// Merge variables with correct precedence: role defaults < role vars < play vars
	pctx.Vars = playbook.MergeRoleVars(roles, play.Vars)

	// Add environment variables
	pctx.Vars["env"] = getEnvMap()

	// Get connector for this host
	conn, err := e.getConnector(play, host)
	if err != nil {
		return fmt.Errorf("failed to create connector for host %s: %w", host, err)
	}
	pctx.Connector = conn

	// Connect
	if err := conn.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", host, err)
	}

	// Gather facts if enabled
	if play.ShouldGatherFacts() {
		e.Output.TaskStart("Gathering Facts", "")
		f, err := facts.Gather(ctx, conn)
		if err != nil {
			e.Output.TaskResult("Gathering Facts", "failed", false, err.Error())
			return fmt.Errorf("failed to gather facts: %w", err)
		}
		pctx.Facts = f
		pctx.Vars["facts"] = f
		e.Output.TaskResult("Gathering Facts", "ok", false, "")
	}

	// Expand role tasks and handlers
	allTasks := playbook.ExpandRoleTasks(roles, play.Tasks)
	allHandlers := playbook.ExpandRoleHandlers(roles, play.Handlers)

	// --- Plan phase ---
	planned := e.planTasks(ctx, pctx, allTasks)
	if len(allHandlers) > 0 {
		planned = append(planned, e.planHandlers(allHandlers)...)
	}
	e.Output.DisplayPlan(planned, e.DryRun)

	// Dry run stops after showing the plan
	if e.DryRun {
		return nil
	}

	// Prompt for approval unless auto-approved
	if !e.AutoApprove {
		if !e.Output.PromptApproval() {
			e.Output.Info("Apply cancelled.")
			return nil
		}
	}

	// --- Apply phase ---
	for _, task := range allTasks {
		// Handle include directive
		if task.Include != "" {
			if err := e.runInclude(ctx, pctx, task, stats); err != nil {
				if !task.IgnoreErrors {
					return err
				}
				e.Output.TaskResult(task.String(), "failed (ignored)", false, err.Error())
			}
			continue
		}

		stats.Tasks++

		taskResult, err := e.runTask(ctx, pctx, task)
		if err != nil {
			stats.Failed++
			if !task.IgnoreErrors {
				return err
			}
			e.Output.TaskResult(task.String(), "failed (ignored)", false, err.Error())
			continue
		}

		stats.RecordResult(taskResult.Status)
	}

	// Run notified handlers (using expanded handlers)
	if err := e.runHandlersExpanded(ctx, pctx, stats, allHandlers); err != nil {
		return err
	}

	return nil
}

// TaskResult holds the result of a task execution.
type TaskResult struct {
	Status  string // ok, changed, skipped, failed
	Changed bool
	Data    map[string]any
	Error   error
}

// runTask executes a single task.
func (e *Executor) runTask(ctx context.Context, pctx *PlayContext, task *playbook.Task) (*TaskResult, error) {
	taskName := task.String()

	// Check 'when' condition
	if task.When != "" {
		shouldRun, err := e.evaluateCondition(task.When, pctx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate 'when' condition: %w", err)
		}
		if !shouldRun {
			e.Output.TaskResult(taskName, "skipped", false, "when condition not met")
			return &TaskResult{Status: "skipped"}, nil
		}
	}

	// Handle loops
	if len(task.Loop) > 0 {
		return e.runTaskLoop(ctx, pctx, task)
	}

	// Run single task
	return e.runSingleTask(ctx, pctx, task)
}

// runSingleTask executes a task once.
func (e *Executor) runSingleTask(ctx context.Context, pctx *PlayContext, task *playbook.Task) (*TaskResult, error) {
	taskName := task.String()
	e.Output.TaskStart(taskName, task.Module)

	// Handle task-level sudo override
	playSudo := pctx.Play.Sudo
	taskSudo := task.ShouldSudo(playSudo)
	if taskSudo != playSudo {
		sudoPass, _ := pctx.Play.Vars["bolt_sudo_password"].(string)
		pctx.Connector.SetSudo(taskSudo, sudoPass)
		defer pctx.Connector.SetSudo(playSudo, sudoPass)
	}

	// Expand shorthand syntax
	playbook.ExpandShorthand(task)

	// Resolve module
	mod := module.Get(task.Module)
	if mod == nil {
		err := fmt.Errorf("unknown module: %s", task.Module)
		e.Output.TaskResult(taskName, "failed", false, err.Error())
		return nil, err
	}

	// Interpolate variables in params
	params, err := e.interpolateParams(task.Params, pctx)
	if err != nil {
		e.Output.TaskResult(taskName, "failed", false, err.Error())
		return nil, fmt.Errorf("failed to interpolate parameters: %w", err)
	}

	// Inject role path for role tasks (allows modules like copy to find role files)
	if task.RolePath != "" {
		params["_role_path"] = task.RolePath
	}

	// Inject template variables for template module
	if task.Module == "template" {
		params["_template_vars"] = pctx.Vars
	}

	// Handle dry run
	if e.DryRun {
		e.Output.TaskResult(taskName, "skipped (dry run)", false, "")
		return &TaskResult{Status: "skipped"}, nil
	}

	// Execute with retries
	var result *module.Result
	var lastErr error
	maxAttempts := task.Retries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			e.Output.Info("Retry %d/%d for task: %s", attempt, maxAttempts, taskName)
			time.Sleep(time.Duration(task.Delay) * time.Second)
		}

		result, lastErr = mod.Run(ctx, pctx.Connector, params)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		e.Output.TaskResult(taskName, "failed", false, lastErr.Error())
		return &TaskResult{Status: "failed", Error: lastErr}, lastErr
	}

	// Store registered result
	if task.Register != "" {
		pctx.Registered[task.Register] = map[string]any{
			"changed": result.Changed,
			"message": result.Message,
			"data":    result.Data,
		}
		pctx.Vars[task.Register] = pctx.Registered[task.Register]
	}

	// Handle notify
	if result.Changed && len(task.Notify) > 0 {
		for _, handler := range task.Notify {
			pctx.NotifiedHandlers[handler] = true
		}
	}

	// Determine status
	status := "ok"
	if result.Changed {
		status = "changed"
	}

	e.Output.TaskResult(taskName, status, result.Changed, result.Message)

	return &TaskResult{
		Status:  status,
		Changed: result.Changed,
		Data:    result.Data,
	}, nil
}

// runTaskLoop executes a task for each item in a loop.
func (e *Executor) runTaskLoop(ctx context.Context, pctx *PlayContext, task *playbook.Task) (*TaskResult, error) {
	loopVar := task.GetLoopVar()
	var anyChanged bool

	for i, item := range task.Loop {
		// Set loop variable
		pctx.Vars[loopVar] = item
		pctx.Vars["loop_index"] = i

		result, err := e.runSingleTask(ctx, pctx, task)
		if err != nil {
			return result, err
		}

		if result.Changed {
			anyChanged = true
		}
	}

	// Clean up loop variables
	delete(pctx.Vars, loopVar)
	delete(pctx.Vars, "loop_index")

	status := "ok"
	if anyChanged {
		status = "changed"
	}

	return &TaskResult{Status: status, Changed: anyChanged}, nil
}

// runHandlersExpanded executes notified handlers from the expanded handlers list.
func (e *Executor) runHandlersExpanded(ctx context.Context, pctx *PlayContext, stats *Stats, handlers []*playbook.Task) error {
	if len(pctx.NotifiedHandlers) == 0 {
		return nil
	}

	e.Output.Section("RUNNING HANDLERS")

	for _, handler := range handlers {
		if !pctx.NotifiedHandlers[handler.Name] {
			continue
		}

		stats.Tasks++

		result, err := e.runSingleTask(ctx, pctx, handler)
		if err != nil {
			stats.Failed++
			return fmt.Errorf("handler '%s' failed: %w", handler.Name, err)
		}

		stats.RecordResult(result.Status)
	}

	return nil
}

// runInclude handles an include directive during the apply phase.
func (e *Executor) runInclude(ctx context.Context, pctx *PlayContext, task *playbook.Task, stats *Stats) error {
	taskName := task.String()

	// Check 'when' condition
	if task.When != "" {
		shouldRun, err := e.evaluateCondition(task.When, pctx)
		if err != nil {
			return fmt.Errorf("failed to evaluate 'when' condition: %w", err)
		}
		if !shouldRun {
			e.Output.TaskResult(taskName, "skipped", false, "when condition not met")
			stats.Tasks++
			stats.Skipped++
			return nil
		}
	}

	// Interpolate variables in the include path
	includePath, err := e.interpolateString(task.Include, pctx)
	if err != nil {
		return fmt.Errorf("failed to interpolate include path: %w", err)
	}
	includeStr, ok := includePath.(string)
	if !ok {
		return fmt.Errorf("include path must be a string, got %T", includePath)
	}

	e.Output.TaskStart(taskName, "include")

	// Resolve and fetch the source
	src, err := source.Resolve(includeStr)
	if err != nil {
		return fmt.Errorf("failed to resolve include source %q: %w", includeStr, err)
	}

	localPath, cleanup, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch include source %q: %w", includeStr, err)
	}
	defer cleanup()

	// Parse the included tasks file
	includedTasks, err := playbook.LoadTasksFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to parse included tasks from %q: %w", includeStr, err)
	}

	e.Output.TaskResult(taskName, "ok", false, fmt.Sprintf("included %d tasks from %s", len(includedTasks), includeStr))

	// Execute each included task inline
	for _, inclTask := range includedTasks {
		stats.Tasks++

		taskResult, err := e.runTask(ctx, pctx, inclTask)
		if err != nil {
			stats.Failed++
			if !inclTask.IgnoreErrors {
				return err
			}
			e.Output.TaskResult(inclTask.String(), "failed (ignored)", false, err.Error())
			continue
		}

		stats.RecordResult(taskResult.Status)
	}

	return nil
}

// planTasks evaluates tasks without executing them and returns a plan.
func (e *Executor) planTasks(ctx context.Context, pctx *PlayContext, tasks []*playbook.Task) []output.PlannedTask {
	var plan []output.PlannedTask

	// Track which variable names are registered by preceding tasks,
	// so we can detect conditions that depend on runtime results.
	registeredNames := make(map[string]bool)
	for k := range pctx.Registered {
		registeredNames[k] = true
	}

	for _, task := range tasks {
		// Handle include tasks in plan phase
		if task.Include != "" {
			pt := output.PlannedTask{
				Name:   task.String(),
				Module: "include",
				Status: "will_run",
				Params: map[string]any{"source": task.Include},
			}
			if task.When != "" {
				if e.conditionReferencesRegistered(task.When, registeredNames) {
					pt.Status = "conditional"
					pt.Reason = task.When
				} else {
					shouldRun, err := e.evaluateCondition(task.When, pctx)
					if err != nil || !shouldRun {
						pt.Status = "will_skip"
						pt.Reason = "when: " + task.When
					}
				}
			}
			plan = append(plan, pt)
			continue
		}

		pt := output.PlannedTask{
			Name:   task.String(),
			Module: task.Module,
		}

		if len(task.Loop) > 0 {
			pt.LoopCount = len(task.Loop)
		}

		if task.When != "" {
			// Check if the condition references a registered variable
			if e.conditionReferencesRegistered(task.When, registeredNames) {
				pt.Status = "conditional"
				pt.Reason = task.When
			} else {
				shouldRun, err := e.evaluateCondition(task.When, pctx)
				if err != nil || !shouldRun {
					pt.Status = "will_skip"
					pt.Reason = "when: " + task.When
				} else {
					pt.Status = "will_run"
				}
			}
		} else {
			pt.Status = "will_run"
		}

		// Resolve params for plan display
		taskCopy := *task
		playbook.ExpandShorthand(&taskCopy)
		resolved, resolveErr := e.interpolateParams(taskCopy.Params, pctx)
		if resolveErr == nil {
			// Filter out internal params for display
			displayParams := make(map[string]any, len(resolved))
			for k, v := range resolved {
				if !strings.HasPrefix(k, "_") {
					displayParams[k] = v
				}
			}
			pt.Params = displayParams
		}

		// Attempt check for tasks that will run
		if pt.Status == "will_run" && resolveErr == nil && pctx.Connector != nil {
			mod := module.Get(task.Module)
			if checker, ok := mod.(module.Checker); ok {
				// Apply task-level sudo for the check
				playSudo := pctx.Play.Sudo
				taskSudo := task.ShouldSudo(playSudo)
				sudoPass, _ := pctx.Play.Vars["bolt_sudo_password"].(string)
				if taskSudo != playSudo {
					pctx.Connector.SetSudo(taskSudo, sudoPass)
				}

				// Inject internal params needed by template/copy
				checkParams := make(map[string]any, len(resolved))
				for k, v := range resolved {
					checkParams[k] = v
				}
				if task.RolePath != "" {
					checkParams["_role_path"] = task.RolePath
				}
				if task.Module == "template" {
					checkParams["_template_vars"] = pctx.Vars
				}

				cr, err := checker.Check(ctx, pctx.Connector, checkParams)
				if err == nil && cr != nil {
					if cr.Uncertain {
						pt.Status = "always_runs"
						pt.Reason = cr.Message
					} else if cr.WouldChange {
						pt.Status = "will_change"
						pt.Reason = cr.Message
					} else {
						pt.Status = "no_change"
						pt.Reason = cr.Message
					}
					pt.OldChecksum = cr.OldChecksum
					pt.NewChecksum = cr.NewChecksum
					pt.OldContent = cr.OldContent
					pt.NewContent = cr.NewContent
				}
				// On error: silently fall back to "will_run"

				// Restore play-level sudo
				if taskSudo != playSudo {
					pctx.Connector.SetSudo(playSudo, sudoPass)
				}
			}
		}

		// Track registered variable for subsequent tasks
		if task.Register != "" {
			registeredNames[task.Register] = true
		}

		plan = append(plan, pt)
	}

	return plan
}

// conditionReferencesRegistered checks whether a when condition references
// any variable name that was (or will be) populated by a register directive.
func (e *Executor) conditionReferencesRegistered(condition string, registered map[string]bool) bool {
	for name := range registered {
		if strings.Contains(condition, name) {
			return true
		}
	}
	return false
}

// planHandlers produces plan entries for notifiable handlers.
func (e *Executor) planHandlers(handlers []*playbook.Task) []output.PlannedTask {
	var plan []output.PlannedTask
	for _, h := range handlers {
		plan = append(plan, output.PlannedTask{
			Name:   h.String(),
			Module: h.Module,
			Status: "conditional",
			Reason: "notified",
		})
	}
	return plan
}

// needsSudoPassword checks whether any task in the play requires sudo and
// no password has been provided yet. If so, it prompts the user. This runs
// before any host output so the prompt appears before "PLAY <host>".
func (e *Executor) needsSudoPassword(play *playbook.Play, tasks, handlers []*playbook.Task) error {
	// Already have a sudo password
	if _, ok := play.Vars["bolt_sudo_password"].(string); ok {
		return nil
	}

	// Check if any task or the play itself needs sudo
	needsSudo := play.Sudo
	if !needsSudo {
		for _, t := range tasks {
			if t.ShouldSudo(play.Sudo) {
				needsSudo = true
				break
			}
		}
	}
	if !needsSudo {
		for _, h := range handlers {
			if h.ShouldSudo(play.Sudo) {
				needsSudo = true
				break
			}
		}
	}
	if !needsSudo {
		return nil
	}

	// Need a password — prompt for it
	if e.PromptSudoPassword == nil {
		return fmt.Errorf("sudo requires a password; use --sudo-password or configure passwordless sudo")
	}

	pass, err := e.PromptSudoPassword()
	if err != nil {
		return fmt.Errorf("failed to read sudo password: %w", err)
	}

	play.Vars["bolt_sudo_password"] = pass
	return nil
}

// getConnector returns a connector for the play targeting a specific host.
func (e *Executor) getConnector(play *playbook.Play, host string) (connector.Connector, error) {
	connType := play.GetConnection()

	sudoPass, _ := play.Vars["bolt_sudo_password"].(string)

	switch connType {
	case "local":
		var opts []local.Option
		if play.Sudo {
			opts = append(opts, local.WithSudo())
			if sudoPass != "" {
				opts = append(opts, local.WithSudoPassword(sudoPass))
			}
		}
		return local.New(opts...), nil

	case "docker":
		return docker.New(host), nil

	case "ssh":
		var sshOpts []sshconn.Option
		if play.Sudo {
			sshOpts = append(sshOpts, sshconn.WithSudo())
			if sudoPass != "" {
				sshOpts = append(sshOpts, sshconn.WithSudoPassword(sudoPass))
			}
		}
		if u, ok := play.Vars["bolt_ssh_user"].(string); ok {
			sshOpts = append(sshOpts, sshconn.WithUser(u))
		}
		if port, ok := play.Vars["bolt_ssh_port"].(int); ok {
			sshOpts = append(sshOpts, sshconn.WithPort(port))
		}
		if keyFile, ok := play.Vars["bolt_ssh_key"].(string); ok {
			sshOpts = append(sshOpts, sshconn.WithKeyFile(keyFile))
		}
		if pass, ok := play.Vars["bolt_ssh_password"].(string); ok {
			sshOpts = append(sshOpts, sshconn.WithPassword(pass))
		}
		if hostKeyChecking, ok := play.Vars["bolt_ssh_host_key_checking"].(bool); ok && !hostKeyChecking {
			sshOpts = append(sshOpts, sshconn.WithInsecureHostKey())
		}
		return sshconn.New(host, sshOpts...), nil

	case "ssm":
		return nil, fmt.Errorf("SSM connector not yet implemented")

	default:
		return nil, fmt.Errorf("unknown connection type: %s", connType)
	}
}

// evaluateCondition evaluates a when condition.
func (e *Executor) evaluateCondition(condition string, pctx *PlayContext) (bool, error) {
	// Simple condition evaluation
	// Supports: variable truthiness, comparisons, and registered results

	condition = strings.TrimSpace(condition)

	// Check for negation
	if strings.HasPrefix(condition, "not ") {
		result, err := e.evaluateCondition(condition[4:], pctx)
		return !result, err
	}

	// Check for registered variable .changed
	if strings.HasSuffix(condition, ".changed") {
		varName := strings.TrimSuffix(condition, ".changed")
		if reg, ok := pctx.Registered[varName]; ok {
			if regMap, ok := reg.(map[string]any); ok {
				if changed, ok := regMap["changed"].(bool); ok {
					return changed, nil
				}
			}
		}
		return false, nil
	}

	// Check for == comparison
	if strings.Contains(condition, "==") {
		parts := strings.SplitN(condition, "==", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])

		leftVal := e.resolveValue(left, pctx)
		rightVal := e.resolveValue(right, pctx)

		return fmt.Sprintf("%v", leftVal) == fmt.Sprintf("%v", rightVal), nil
	}

	// Check for != comparison
	if strings.Contains(condition, "!=") {
		parts := strings.SplitN(condition, "!=", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])

		leftVal := e.resolveValue(left, pctx)
		rightVal := e.resolveValue(right, pctx)

		return fmt.Sprintf("%v", leftVal) != fmt.Sprintf("%v", rightVal), nil
	}

	// Simple variable truthiness
	val := e.resolveValue(condition, pctx)
	return isTruthy(val), nil
}

// resolveValue resolves a value that might be a variable reference.
func (e *Executor) resolveValue(s string, pctx *PlayContext) any {
	s = strings.TrimSpace(s)

	// String literal
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		return s[1 : len(s)-1]
	}

	// Boolean literals
	if s == "true" || s == "True" {
		return true
	}
	if s == "false" || s == "False" {
		return false
	}

	// Variable lookup
	if val, ok := pctx.Vars[s]; ok {
		return val
	}

	// Dotted variable lookup (e.g., facts.os)
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		var current any = pctx.Vars
		for _, part := range parts {
			if m, ok := current.(map[string]any); ok {
				current = m[part]
			} else {
				return nil
			}
		}
		return current
	}

	return s
}

// isTruthy returns whether a value is considered truthy.
func isTruthy(v any) bool {
	if v == nil {
		return false
	}

	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != "" && val != "false" && val != "False" && val != "no"
	case int, int64, float64:
		return val != 0
	case []any:
		return len(val) > 0
	case map[string]any:
		return len(val) > 0
	default:
		return true
	}
}

// getEnvMap returns environment variables as a map.
func getEnvMap() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if idx := strings.Index(e, "="); idx > 0 {
			env[e[:idx]] = e[idx+1:]
		}
	}
	return env
}
