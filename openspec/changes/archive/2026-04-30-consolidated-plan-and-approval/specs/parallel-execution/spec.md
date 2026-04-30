## MODIFIED Requirements

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
