// Package executor runs playbooks against target hosts.
package executor

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eugenetaranov/bolt/internal/connector"
	"github.com/eugenetaranov/bolt/internal/connector/docker"
	"github.com/eugenetaranov/bolt/internal/connector/local"
	sshconn "github.com/eugenetaranov/bolt/internal/connector/ssh"
	ssmconn "github.com/eugenetaranov/bolt/internal/connector/ssm"
	"github.com/eugenetaranov/bolt/internal/inventory"
	"github.com/eugenetaranov/bolt/internal/module"
	"github.com/eugenetaranov/bolt/internal/output"
	"github.com/eugenetaranov/bolt/internal/playbook"
	"github.com/eugenetaranov/bolt/internal/source"
	"github.com/eugenetaranov/bolt/pkg/facts"
	"github.com/eugenetaranov/bolt/pkg/ssmparams"
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
	SSMInstances []string
	SSMTags      map[string]string
	SSMRegion    string
	SSMBucket    string
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

	// Inventory holds the loaded inventory (optional). When set, group names
	// in play.Hosts are expanded and per-host vars/SSH config are applied.
	Inventory *inventory.Inventory
}

// New creates a new executor.
func New() *Executor {
	return &Executor{
		Output: output.New(os.Stdout),
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

	// SSMParams is a lazy-init cached SSM Parameter Store client.
	SSMParams *ssmparams.Client
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
		e.ApplyOverrides(play)
		if err := e.runPlay(ctx, play, stats, rolesDir); err != nil {
			if ctx.Err() != nil {
				return result, nil
			}
			result.Success = false
			e.Output.Error("Play failed: %v", err)
			break
		}
	}

	stats.EndTime = time.Now()
	e.Output.PlaybookEnd(stats)

	return result, nil
}

// ApplyOverrides applies CLI/env connection overrides to a play.
func (e *Executor) ApplyOverrides(play *playbook.Play) {
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
	if o.Sudo {
		play.Sudo = true
	}
	if o.SudoPassword != "" {
		play.SudoPassword = o.SudoPassword
	}

	// SSH overrides
	if o.SSHUser != "" || o.SSHPort != 0 || o.SSHKey != "" || o.HasSSHPass || o.SSHInsecure {
		if play.SSH == nil {
			play.SSH = &playbook.SSHConfig{}
		}
		if o.SSHUser != "" {
			play.SSH.User = o.SSHUser
		}
		if o.SSHPort != 0 {
			play.SSH.Port = o.SSHPort
		}
		if o.SSHKey != "" {
			play.SSH.Key = o.SSHKey
		}
		if o.HasSSHPass {
			play.SSH.Password = o.SSHPass
		}
		if o.SSHInsecure {
			f := false
			play.SSH.HostKeyChecking = &f
		}
	}

	// SSM overrides
	if o.SSMRegion != "" || o.SSMBucket != "" || len(o.SSMInstances) > 0 || len(o.SSMTags) > 0 {
		if play.SSM == nil {
			play.SSM = &playbook.SSMConfig{}
		}
		if o.SSMRegion != "" {
			play.SSM.Region = o.SSMRegion
		}
		if o.SSMBucket != "" {
			play.SSM.Bucket = o.SSMBucket
		}
		if play.Connection == "ssm" && len(play.Hosts) == 0 {
			if len(o.SSMInstances) > 0 {
				play.Hosts = o.SSMInstances
			} else if len(o.SSMTags) > 0 {
				play.SSM.Tags = o.SSMTags
			}
		}
	}
}

// runPlay executes a single play.
func (e *Executor) runPlay(ctx context.Context, play *playbook.Play, stats *Stats, rolesDir string) error {
	// Expand inventory group names in play.Hosts and apply group-level config.
	if e.Inventory != nil && len(play.Hosts) > 0 {
		expanded := make([]string, 0, len(play.Hosts))
		for _, h := range play.Hosts {
			hosts, group, ok := e.Inventory.ExpandGroup(h)
			if !ok {
				// Not in inventory — pass through as-is (plain hostname, URI, etc.)
				expanded = append(expanded, h)
				continue
			}
			expanded = append(expanded, hosts...)
			// Apply group-level connection/SSH/SSM as defaults (play values take priority)
			if group != nil {
				if play.Connection == "" && group.Connection != "" {
					play.Connection = group.Connection
				}
				if play.SSH == nil && group.SSH != nil {
					play.SSH = group.SSH
				}
				if play.SSM == nil && group.SSM != nil {
					play.SSM = &playbook.SSMConfig{
						Region: group.SSM.Region,
						Bucket: group.SSM.Bucket,
						Tags:   group.SSM.Tags,
					}
				}
			}
		}
		play.Hosts = expanded
	}

	if play.GetConnection() == "ssm" && play.SSM != nil && len(play.Hosts) == 0 {
		// ssm.instances is a convenience alias for hosts when connection is ssm
		if len(play.SSM.Instances) > 0 {
			play.Hosts = play.SSM.Instances
		} else if len(play.SSM.Tags) > 0 {
			// SSM tag resolution: discover instance IDs at runtime
			ids, err := ssmconn.ResolveInstancesByTags(ctx, play.SSM.Tags, play.SSM.Region)
			if err != nil {
				return fmt.Errorf("failed to resolve SSM instances by tags: %w", err)
			}
			play.Hosts = ids
		}
	}

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

	if ctx.Err() != nil {
		return ctx.Err()
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
	e.Output.HostStart(host, play.GetConnection())

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

	// Inject inventory vars as lower-priority defaults (play vars take precedence).
	// Group vars are lowest, per-host vars are higher (but still below play vars).
	if e.Inventory != nil {
		for _, g := range e.Inventory.GetHostGroups(host) {
			for k, v := range g.Vars {
				if _, exists := pctx.Vars[k]; !exists {
					pctx.Vars[k] = v
				}
			}
		}
		if entry := e.Inventory.GetHost(host); entry != nil {
			for k, v := range entry.Vars {
				if _, exists := pctx.Vars[k]; !exists {
					pctx.Vars[k] = v
				}
			}
		}
	}

	// Add environment variables
	pctx.Vars["env"] = getEnvMap()

	// Get connector for this host
	conn, err := e.GetConnector(play, host)
	if err != nil {
		return fmt.Errorf("failed to create connector for host %s: %w", host, err)
	}
	pctx.Connector = conn

	// Connect
	if err := conn.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", host, err)
	}
	defer conn.Close()

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

	// Create lazy SSM Parameter Store client.
	// Region priority: play.SSM.Region > ec2_region fact > AWS SDK default.
	ssmRegion := ""
	if play.SSM != nil && play.SSM.Region != "" {
		ssmRegion = play.SSM.Region
	} else if r, ok := pctx.Facts["ec2_region"].(string); ok && r != "" {
		ssmRegion = r
	}
	pctx.SSMParams = ssmparams.New(ssmRegion)

	// Expand role tasks and handlers
	allTasks := playbook.ExpandRoleTasks(roles, play.Tasks)
	allHandlers := playbook.ExpandRoleHandlers(roles, play.Handlers)

	// --- Plan phase ---
	planned := e.planTasks(ctx, pctx, allTasks)
	if len(allHandlers) > 0 {
		planned = append(planned, e.planHandlers(allTasks, planned, allHandlers)...)
	}
	e.Output.DisplayPlan(planned, e.DryRun)

	// Dry run stops after showing the plan
	if e.DryRun {
		return nil
	}

	// No drift detected — nothing to apply
	if allNoChange(planned) {
		for _, t := range planned {
			stats.Tasks++
			if t.Status == "will_skip" {
				stats.Skipped++
			} else {
				stats.OK++
			}
		}
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

	// Resolve loop expression (e.g. "{{ windmill_files }}") to a concrete list
	if task.LoopExpr != "" && len(task.Loop) == 0 {
		resolved, err := e.interpolateString(ctx, task.LoopExpr, pctx)
		if err == nil {
			if items, ok := resolved.([]any); ok {
				task.Loop = items
			}
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
		pctx.Connector.SetSudo(taskSudo, pctx.Play.SudoPassword)
		defer pctx.Connector.SetSudo(playSudo, pctx.Play.SudoPassword)
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
	params, err := e.interpolateParams(ctx, task.Params, pctx)
	if err != nil {
		e.Output.TaskResult(taskName, "failed", false, err.Error())
		return nil, fmt.Errorf("failed to interpolate parameters: %w", err)
	}

	// Inject role path for role tasks (allows modules like copy to find role files)
	if task.RolePath != "" {
		params["_role_path"] = task.RolePath
	}

	// Inject template variables and SSM params client for template module
	if task.Module == "template" {
		params["_template_vars"] = pctx.Vars
		if pctx.SSMParams != nil {
			params["_ssm_params"] = pctx.SSMParams
		}
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
	includePath, err := e.interpolateString(ctx, task.Include, pctx)
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

		// Resolve loop expression for plan display
		if task.LoopExpr != "" && len(task.Loop) == 0 {
			resolved, err := e.interpolateString(ctx, task.LoopExpr, pctx)
			if err == nil {
				if items, ok := resolved.([]any); ok {
					task.Loop = items
				}
			}
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

		// For looped tasks, temporarily set the loop variable to the first item
		// so that param interpolation and checks produce meaningful results.
		if len(task.Loop) > 0 {
			loopVar := task.GetLoopVar()
			pctx.Vars[loopVar] = task.Loop[0]
			// Clean up after param interpolation below
		}

		// Resolve params for plan display
		taskCopy := *task
		playbook.ExpandShorthand(&taskCopy)
		resolved, resolveErr := e.interpolateParams(ctx, taskCopy.Params, pctx)
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
				if taskSudo != playSudo {
					pctx.Connector.SetSudo(taskSudo, pctx.Play.SudoPassword)
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
					if pctx.SSMParams != nil {
						checkParams["_ssm_params"] = pctx.SSMParams
					}
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
					pctx.Connector.SetSudo(playSudo, pctx.Play.SudoPassword)
				}
			}
		}

		// Clean up loop variable
		if len(task.Loop) > 0 {
			delete(pctx.Vars, task.GetLoopVar())
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
// It uses task definitions and their plan results to determine whether
// any notifying task would actually produce a change.
func (e *Executor) planHandlers(tasks []*playbook.Task, taskPlan []output.PlannedTask, handlers []*playbook.Task) []output.PlannedTask {
	// Build a set of handler names that could potentially be notified.
	// A handler could be notified if at least one task that lists it in
	// notify has a plan status that implies change (will_change, always_runs,
	// will_run, conditional — anything other than no_change / will_skip).
	maybeNotified := make(map[string]bool)
	for i, task := range tasks {
		if len(task.Notify) == 0 {
			continue
		}
		if i >= len(taskPlan) {
			break
		}
		st := taskPlan[i].Status
		if st != "no_change" && st != "will_skip" {
			for _, name := range task.Notify {
				maybeNotified[name] = true
			}
		}
	}

	var plan []output.PlannedTask
	for _, h := range handlers {
		if !maybeNotified[h.Name] {
			continue
		}
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
	if play.SudoPassword != "" {
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

	play.SudoPassword = pass
	return nil
}

// GetConnector returns a connector for the play targeting a specific host.
func (e *Executor) GetConnector(play *playbook.Play, host string) (connector.Connector, error) {
	connType := play.GetConnection()

	switch connType {
	case "local":
		var opts []local.Option
		if play.Sudo {
			opts = append(opts, local.WithSudo())
			if play.SudoPassword != "" {
				opts = append(opts, local.WithSudoPassword(play.SudoPassword))
			}
		}
		return local.New(opts...), nil

	case "docker":
		return docker.New(host), nil

	case "ssh":
		var sshOpts []sshconn.Option
		if play.Sudo {
			sshOpts = append(sshOpts, sshconn.WithSudo())
			if play.SudoPassword != "" {
				sshOpts = append(sshOpts, sshconn.WithSudoPassword(play.SudoPassword))
			}
		}

		// Resolve effective SSH config: play.SSH > inventory host SSH > inventory group SSH.
		// We build a merged view where play settings always win.
		effectiveSSH := mergeSSHConfig(play.SSH, e.Inventory, host)

		if effectiveSSH.User != "" {
			sshOpts = append(sshOpts, sshconn.WithUser(effectiveSSH.User))
		}
		// Check if the host string embeds a port (e.g. "host:2222" from a URI).
		// Embedded port takes priority over all other port settings.
		sshHost := host
		if h, p, err := net.SplitHostPort(host); err == nil {
			sshHost = h
			if pn, err := strconv.Atoi(p); err == nil {
				sshOpts = append(sshOpts, sshconn.WithPort(pn))
			}
		} else if effectiveSSH.Port != 0 {
			sshOpts = append(sshOpts, sshconn.WithPort(effectiveSSH.Port))
		}
		if effectiveSSH.Key != "" {
			sshOpts = append(sshOpts, sshconn.WithKeyFile(effectiveSSH.Key))
		}
		if effectiveSSH.Password != "" {
			sshOpts = append(sshOpts, sshconn.WithPassword(effectiveSSH.Password))
		}
		if effectiveSSH.HostKeyChecking != nil && !*effectiveSSH.HostKeyChecking {
			sshOpts = append(sshOpts, sshconn.WithInsecureHostKey())
		}
		return sshconn.New(sshHost, sshOpts...), nil

	case "ssm":
		var ssmOpts []ssmconn.Option
		if play.Sudo {
			ssmOpts = append(ssmOpts, ssmconn.WithSudo())
			if play.SudoPassword != "" {
				ssmOpts = append(ssmOpts, ssmconn.WithSudoPassword(play.SudoPassword))
			}
		}
		if play.SSM != nil {
			if play.SSM.Region != "" {
				ssmOpts = append(ssmOpts, ssmconn.WithRegion(play.SSM.Region))
			}
			if play.SSM.Bucket != "" {
				ssmOpts = append(ssmOpts, ssmconn.WithBucket(play.SSM.Bucket))
			}
		}
		return ssmconn.New(host, ssmOpts...), nil

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
	if len(s) >= 2 &&
		((strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
			(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\""))) {
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
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
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

// allNoChange returns true if every planned task has status "no_change" or "will_skip".
func allNoChange(tasks []output.PlannedTask) bool {
	for _, t := range tasks {
		if t.Status != "no_change" && t.Status != "will_skip" {
			return false
		}
	}
	return true
}

// toStringMap converts a value to map[string]string. Supports map[string]string
// directly or map[string]any with string values (from YAML parsing).
func toStringMap(v any) (map[string]string, bool) {
	switch m := v.(type) {
	case map[string]string:
		return m, true
	case map[string]any:
		result := make(map[string]string, len(m))
		for k, val := range m {
			result[k] = fmt.Sprintf("%v", val)
		}
		return result, true
	default:
		return nil, false
	}
}

// mergeSSHConfig returns the effective SSH config for a host by merging
// play-level SSH settings with inventory per-host and group SSH settings.
// Priority: play.SSH > inventory host SSH > inventory group SSH.
// A zero-value field means "not set"; the first non-zero value wins.
func mergeSSHConfig(playCfg *playbook.SSHConfig, inv *inventory.Inventory, host string) playbook.SSHConfig {
	var result playbook.SSHConfig

	// Collect sources from lowest to highest priority so higher-priority
	// values overwrite lower-priority ones.
	var sources []*playbook.SSHConfig

	// Lowest: group SSH
	if inv != nil {
		for _, g := range inv.GetHostGroups(host) {
			if g.SSH != nil {
				sources = append(sources, g.SSH)
			}
		}
	}

	// Middle: per-host inventory SSH
	if inv != nil {
		if entry := inv.GetHost(host); entry != nil && entry.SSH != nil {
			sources = append(sources, entry.SSH)
		}
	}

	// Highest: play-level SSH
	if playCfg != nil {
		sources = append(sources, playCfg)
	}

	for _, src := range sources {
		if src.User != "" {
			result.User = src.User
		}
		if src.Port != 0 {
			result.Port = src.Port
		}
		if src.Key != "" {
			result.Key = src.Key
		}
		if src.Password != "" {
			result.Password = src.Password
		}
		if src.HostKeyChecking != nil {
			result.HostKeyChecking = src.HostKeyChecking
		}
	}

	return result
}
