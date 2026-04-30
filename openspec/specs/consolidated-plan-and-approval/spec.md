## ADDED Requirements

### Requirement: Per-line host attribution on plan output
The executor SHALL render each task line in a multi-host plan with a host prefix in the form `<host>: <indicator> <module> <name>`. Hostnames SHALL be left-padded for column alignment up to a maximum width of 30 characters; hostnames longer than 30 characters SHALL be truncated with a single-character ellipsis (`…`).

#### Scenario: Three hosts with mixed plans
- **WHEN** a play targets `web1`, `web2`, `web3` and only `web1` and `web3` have changes
- **THEN** the rendered plan SHALL show `web1: + install nginx` and `web3: ~ rotate cert`, padded to a uniform host column

#### Scenario: SSM instance IDs as hostnames
- **WHEN** a play targets `i-0817eea131fa23c39` and `i-0a7b29ada0a9bc187`
- **THEN** each plan line SHALL be prefixed with the full instance ID (≤30 chars), column-aligned

#### Scenario: Hostname longer than 30 characters
- **WHEN** a hostname is 35 characters long
- **THEN** the prefix SHALL be truncated to the first 29 characters followed by `…`, then `:`

### Requirement: No-op hosts contribute no plan lines
A host whose plan contains zero tasks with status `will_run`, `will_change`, or `always_runs` SHALL NOT contribute any task lines to the rendered plan. Such hosts SHALL be counted only in the footer's "unchanged" total.

#### Scenario: 50 hosts, 47 unchanged
- **WHEN** 47 of 50 targeted hosts have only `no_change` and `will_skip` plan entries
- **THEN** the rendered plan body SHALL contain only the 3 changing hosts' lines, and the footer SHALL include "(47 unchanged)"

### Requirement: Consolidated plan footer
The plan footer for a multi-host play SHALL read in the form: `Plan: <X> to change, <Y> to run, <Z> ok across <N> hosts (<M> unchanged).` where `X`, `Y`, `Z` are sums of task statuses across all hosts, `N` is the total number of targeted hosts, and `M` is the count of no-op hosts.

#### Scenario: Three hosts, mixed
- **WHEN** the play has 3 targeted hosts, 2 tasks would change, 1 task would run, 0 tasks no-op, and 1 host has zero changes
- **THEN** the footer SHALL read `Plan: 2 to change, 1 to run, 0 ok across 3 hosts (1 unchanged).`

### Requirement: Single global approval prompt
For multi-host plays, the executor SHALL call `PromptApproval` exactly once after rendering the consolidated plan and before any host begins applying. The prompt SHALL run on the main thread, never inside a per-host goroutine.

#### Scenario: Four-host play with default forks
- **WHEN** a play targets four hosts and `--forks 1`
- **THEN** the executor SHALL render the consolidated plan once and prompt for approval exactly once

#### Scenario: Four-host play with forks > 1
- **WHEN** a play targets four hosts and `--forks 4`
- **THEN** the executor SHALL render the consolidated plan once on the main thread and prompt for approval exactly once before dispatching parallel apply

#### Scenario: Auto-approve flag set
- **WHEN** `--auto-approve` is set
- **THEN** the executor SHALL render the consolidated plan and proceed to apply without prompting

### Requirement: PlannedTask carries host attribution
The `PlannedTask` struct SHALL include a `Host` field populated during plan computation. The field SHALL be the host name as it appears in `play.Hosts` after inventory expansion.

#### Scenario: Plan task field populated
- **WHEN** `planTasks` produces a `PlannedTask` for host `web1`
- **THEN** `PlannedTask.Host` SHALL equal `"web1"`

### Requirement: Single-host fast path preserved
A play targeting exactly one host SHALL render its plan via the existing `DisplayPlan` path (no per-line host prefix, no `(N unchanged)` footer suffix). Output SHALL be byte-identical to current main-branch behavior.

#### Scenario: Single-host serial play
- **WHEN** a play targets one host with `--forks 1`
- **THEN** the rendered output SHALL match a snapshot of the current main-branch output exactly

### Requirement: JSON event host attribution
The JSON output emitter SHALL include a `host` field on `plan_task`, `task_start`, and `task_result` events. The JSON output schema version SHALL be incremented in the same release.

#### Scenario: Plan task in JSON
- **WHEN** the JSON emitter renders a plan task for `web1`
- **THEN** the emitted event SHALL include `"host": "web1"`

#### Scenario: Schema version bumped
- **WHEN** this change is released
- **THEN** the JSON output schema version constant SHALL be one greater than its previous release value

### Requirement: SIGINT during approval aborts cleanly
When the user sends SIGINT during the global approval prompt, the executor SHALL exit without applying any host. No partial-apply state SHALL be possible from approval-time interruption.

#### Scenario: User cancels at approval
- **WHEN** the user presses Ctrl+C while the consolidated approval prompt is awaiting input
- **THEN** the executor SHALL exit non-zero and SHALL NOT have applied any task on any host

### Requirement: Approval prompt identifies the target host(s)
The interactive approval prompt SHALL include a description of the target host(s) and connection type so the user can identify what is about to be modified without scrolling above the prompt. The prompt text SHALL take the form `Apply these changes to <target>? (yes/no): ` where `<target>` is rendered as follows:

- For a play targeting exactly one host, `<target>` SHALL be `<host> (<connection>)`. Example: `web1.prod (ssh)` or `i-0a1b2c3d4e5f (ssm)`.
- For a play targeting two or more hosts, `<target>` SHALL begin with the host count and SHALL list host names in parentheses. When the host count is five or fewer, all names SHALL be listed. When the host count exceeds five, the first five names SHALL be listed followed by a literal `, ...` suffix. Example: `4 hosts (web1, web2, web3, web4)`; `12 hosts (web1, web2, web3, web4, web5, ...)`.

The prompt content rule SHALL apply to both the single-host fast path (after `DisplayPlan`) and the consolidated multi-host path (after `DisplayMultiHostPlan`). The `--auto-approve` flag and the JSON emitter's auto-approval behavior SHALL be unaffected: in those modes no prompt is shown.

#### Scenario: Single-host SSH play
- **WHEN** a play targets exactly one host `web1.prod` over `connection: ssh` and `--auto-approve` is not set
- **THEN** the prompt line SHALL read `Apply these changes to web1.prod (ssh)? (yes/no): `

#### Scenario: Single-host SSM instance
- **WHEN** a play targets exactly one host `i-0a1b2c3d4e5f` over `connection: ssm`
- **THEN** the prompt line SHALL read `Apply these changes to i-0a1b2c3d4e5f (ssm)? (yes/no): `

#### Scenario: Multi-host play within the visible cap
- **WHEN** a play targets four hosts `web1`, `web2`, `web3`, `web4`
- **THEN** the prompt line SHALL read `Apply these changes to 4 hosts (web1, web2, web3, web4)? (yes/no): `

#### Scenario: Multi-host play exceeding the visible cap
- **WHEN** a play targets twelve hosts beginning with `web1`, `web2`, `web3`, `web4`, `web5`
- **THEN** the prompt line SHALL read `Apply these changes to 12 hosts (web1, web2, web3, web4, web5, ...)? (yes/no): `

#### Scenario: Auto-approve suppresses the prompt
- **WHEN** `--auto-approve` is set on any play
- **THEN** no prompt SHALL be shown and the executor SHALL proceed to apply

#### Scenario: JSON emitter does not prompt
- **WHEN** the JSON emitter is active
- **THEN** the emitter SHALL auto-approve regardless of the host target string and SHALL NOT print the prompt to stdout
