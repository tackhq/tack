## ADDED Requirements

### Requirement: Concurrent host execution
The executor SHALL run plays across up to N hosts concurrently when `--forks N` is specified, where N is greater than 1. Each host SHALL execute its **apply phase** in its own goroutine with an independent PlayContext and Connector. The plan and approval phases SHALL run on the main thread before any apply goroutine is dispatched.

#### Scenario: Forks set to 5 with 10 hosts
- **WHEN** `--forks 5` is specified and the play targets 10 hosts
- **THEN** the executor SHALL render the consolidated plan and prompt for approval once on the main thread, then run up to 5 hosts concurrently in the apply phase, queuing the remaining 5 until a slot becomes available

#### Scenario: Default forks value
- **WHEN** `--forks` is not specified
- **THEN** the executor SHALL default to 1 (serial apply), preserving backward compatibility

#### Scenario: Forks greater than host count
- **WHEN** `--forks 20` is specified and the play targets 5 hosts
- **THEN** the executor SHALL run all 5 hosts concurrently in the apply phase without error

### Requirement: Output buffering
Per-host **apply-phase** output SHALL be buffered during parallel execution and flushed sequentially in host order after all hosts complete, preventing interleaved output. Plan output is rendered once on the main thread and is not buffered per-host.

#### Scenario: Two hosts complete out of order
- **WHEN** host-b completes apply before host-a during parallel execution
- **THEN** apply-phase output SHALL be displayed in order: host-a first, then host-b

#### Scenario: Serial execution output unchanged
- **WHEN** `--forks 1` (default)
- **THEN** apply-phase output SHALL stream in real-time as before, with no buffering

### Requirement: Error aggregation
Host failures SHALL NOT terminate execution of other hosts. The executor SHALL collect results from all hosts and produce a unified summary.

#### Scenario: One host fails out of three
- **WHEN** 3 hosts are targeted and host-b fails on a task
- **THEN** host-a and host-c SHALL continue executing, and the final recap SHALL show host-b as failed

#### Scenario: All hosts fail
- **WHEN** all hosts fail during execution
- **THEN** the executor SHALL report all failures in the recap and exit with non-zero status

### Requirement: Context cancellation
The executor SHALL respect context cancellation (SIGINT) during parallel execution. All in-flight goroutines SHALL be cancelled when the context is done.

#### Scenario: User sends SIGINT during parallel execution
- **WHEN** the user presses Ctrl+C while hosts are executing in parallel
- **THEN** all running host goroutines SHALL be cancelled and cleanup SHALL proceed

### Requirement: TACK_FORKS environment variable
The fork count SHALL be configurable via the `TACK_FORKS` environment variable, with CLI flag taking precedence.

#### Scenario: Environment variable set
- **WHEN** `TACK_FORKS=10` is set and no `--forks` flag is provided
- **THEN** the executor SHALL use 10 as the fork count

#### Scenario: CLI flag overrides environment
- **WHEN** `TACK_FORKS=10` is set and `--forks 5` is provided
- **THEN** the executor SHALL use 5 as the fork count

### Requirement: Plan phase remains serial
The plan phase (preview and approval) SHALL remain serial regardless of fork count. Parallelism SHALL only apply to the apply phase after approval.

#### Scenario: Plan display with forks
- **WHEN** `--forks 5` is specified
- **THEN** plans SHALL be displayed one host at a time, followed by a single approval prompt, then parallel apply
