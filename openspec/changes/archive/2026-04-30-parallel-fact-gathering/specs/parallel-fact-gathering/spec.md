## ADDED Requirements

### Requirement: Concurrent fact gathering across hosts
The executor SHALL gather facts for all target hosts in a play concurrently, in a pre-pass that runs after host expansion and before the per-host plan/apply loop. This pre-pass SHALL run regardless of the `--forks` value, including the default `--forks 1` (serial apply).

#### Scenario: Four SSM hosts with default forks
- **WHEN** a play targets four SSM hosts with `gather_facts: true` and `--forks` is not specified
- **THEN** the executor SHALL invoke `facts.Gather` for all four hosts concurrently and complete the fact-gathering phase in roughly the time of the slowest host's gather, not the sum of all four

#### Scenario: Single-host play
- **WHEN** a play targets exactly one host
- **THEN** the executor SHALL gather facts inline without spawning a worker pool, producing identical output to the pre-parallel-fact-gathering behavior

#### Scenario: Local-connection play
- **WHEN** a play uses `connection: local`
- **THEN** the executor SHALL gather facts once for `localhost` without parallel fan-out

### Requirement: Concurrency ceiling
The executor SHALL bound concurrent fact-gather goroutines by an internal `factsConcurrency` limit (default 20) to avoid overwhelming connector backends (e.g., AWS SSM API rate limits) on very large fleets. The limit SHALL be independent of `--forks`.

#### Scenario: 50 hosts with default ceiling
- **WHEN** a play targets 50 SSM hosts and the default fact concurrency ceiling is 20
- **THEN** the executor SHALL run at most 20 fact-gather goroutines simultaneously, queuing the remaining 30 until slots free up

#### Scenario: Hosts under the ceiling
- **WHEN** a play targets 5 hosts and the ceiling is 20
- **THEN** the executor SHALL run all 5 fact gathers concurrently without queuing

### Requirement: Disabled fact gathering bypass
The pre-pass SHALL be skipped entirely when the play disables fact gathering. No goroutines, connections, or output SHALL be produced for fact gathering in that case.

#### Scenario: gather_facts is false
- **WHEN** a play sets `gather_facts: false` (or equivalent default override)
- **THEN** the executor SHALL skip the parallel fact-gather pre-pass and proceed directly to per-host plan/apply with empty fact maps

### Requirement: Fact propagation to play context
Facts gathered in the pre-pass SHALL be made available to each host's `PlayContext` such that `pctx.Facts` and `pctx.Vars["facts"]` are populated before the plan phase runs, identical to today's inline-gather behavior.

#### Scenario: Plan condition references facts
- **WHEN** a play has `gather_facts: true` and a task uses `when: facts.os_family == "Debian"`
- **THEN** the plan phase SHALL evaluate the condition against the pre-gathered facts for that host

#### Scenario: Registered template uses facts
- **WHEN** a task interpolates `{{ facts.hostname }}` into a template
- **THEN** the value SHALL match the hostname returned by the host's pre-pass fact gather

### Requirement: Per-host fact-gather failure isolation in parallel mode
When `--forks > 1`, a failure during fact gathering for one host SHALL be recorded as that host's failure and SHALL NOT abort fact gathering for other hosts. The failed host SHALL be excluded from the apply phase and reported in the run recap as failed.

#### Scenario: One of three hosts fails fact gather under parallel forks
- **WHEN** `--forks 3` is set, three hosts are targeted, and host-b's `facts.Gather` returns an error
- **THEN** host-a and host-c SHALL still gather facts and proceed to plan/apply, while host-b SHALL be marked failed and skipped from apply

### Requirement: Fail-fast fact-gather failure in serial mode
When `--forks 1` (serial apply), the executor SHALL preserve today's fail-fast semantics: if any host's fact gather fails, the play SHALL terminate with that error and remaining hosts SHALL NOT be processed for plan/apply.

#### Scenario: First host fails fact gather under serial mode
- **WHEN** `--forks 1` (default) and host-a's `facts.Gather` returns an error
- **THEN** the executor SHALL return the error and SHALL NOT proceed to host-b or host-c

### Requirement: Output ordering for fact-gather phase
Per-host fact-gather output (the `Gathering Facts` task line) SHALL be buffered during the parallel pre-pass and flushed in host order before the per-host plan/apply loop begins, regardless of completion order.

#### Scenario: Hosts complete fact gather out of order
- **WHEN** host-b's fact gather finishes before host-a's during the pre-pass
- **THEN** the printed output SHALL show host-a's `Gathering Facts` line before host-b's

#### Scenario: Failure line ordering
- **WHEN** host-a fails fact gather and host-b succeeds
- **THEN** host-a's failure output SHALL appear before host-b's success output in the flushed pre-pass output

### Requirement: Connection reuse from pre-pass to apply
The connector opened during the fact-gather pre-pass for a successful host SHALL be reused by the subsequent plan/apply phase rather than reconnecting.

#### Scenario: SSH host gathered then applied
- **WHEN** host-a successfully gathers facts via SSH in the pre-pass
- **THEN** the executor SHALL pass the open SSH connector to `runPlayOnHost` for plan/apply without performing a second `Connect` call

#### Scenario: Failed pre-pass closes connection
- **WHEN** host-a's fact gather fails after the connector has been opened
- **THEN** the executor SHALL close that connector before recording the failure

### Requirement: Context cancellation during pre-pass
The fact-gather pre-pass SHALL respect context cancellation. If the context is cancelled (e.g., user sends SIGINT), all in-flight fact-gather goroutines SHALL terminate and the executor SHALL not begin the plan/apply phase.

#### Scenario: SIGINT during pre-pass
- **WHEN** the user presses Ctrl+C while ten hosts are gathering facts
- **THEN** all running goroutines SHALL observe the cancelled context, return promptly, and the executor SHALL exit without running plan/apply for any host
