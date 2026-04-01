## Context

Bolt's executor iterates over hosts in a simple for-loop, calling `runPlayOnHost()` sequentially. Each host gets an independent `PlayContext` with its own variable scope, connector, and registered results. This isolation makes parallelism structurally safe — no shared mutable state between hosts.

The `--forks` flag is already declared in cobra (`cmd/bolt/main.go`) but the value is never read by the executor. Users who set `-f 10` get serial execution with no warning.

## Goals / Non-Goals

**Goals:**
- Execute plays across up to N hosts concurrently via goroutine worker pool
- Buffer output per-host to prevent interleaved terminal output
- Aggregate errors — one host failing does not stop others
- Plan phase remains serial (interactive approval prompt)
- Apply phase runs in parallel after approval
- Respect context cancellation (SIGINT stops all workers)

**Non-Goals:**
- Task-level parallelism within a single host (tasks always run sequentially per host)
- Dynamic fork adjustment based on system resources
- Distributed execution across multiple controllers
- Progress bars or live per-host status (future enhancement)

## Decisions

### 1. Semaphore-based worker pool using `sync.WaitGroup` + buffered channel

A buffered channel of size N acts as a semaphore. Each host launches a goroutine that acquires a slot from the channel before executing. `sync.WaitGroup` tracks completion.

**Alternative considered:** `golang.org/x/sync/errgroup` — rejected because we need to continue on individual host errors (errgroup cancels on first error by default). A manual WaitGroup + channel gives us the collect-all-errors behavior we need.

### 2. Per-host output buffering with sequential flush

Each goroutine writes to a `bytes.Buffer`. After all hosts complete, buffers are flushed to stdout in host order. This preserves deterministic output ordering regardless of completion order.

**Alternative considered:** Real-time streaming with host-prefixed lines (`[web1] ✓ task`). Rejected for v1 because it changes the output format. Can be added later as an opt-in mode.

### 3. Plan phase stays serial, apply phase parallelizes

The plan phase requires interactive approval. Plans are shown per-host sequentially, then a single approval prompt covers all hosts. After approval, the apply phase fans out.

**Alternative considered:** Parallel plan + parallel apply. Rejected because multiple plan outputs interleave, and multiple approval prompts are confusing.

### 4. Error collection with unified recap

Each host returns its own `HostResult` (success/failure + stats). The executor collects all results and produces a unified recap showing per-host status. A single host failure marks the overall run as failed but does not prevent other hosts from completing.

## Risks / Trade-offs

- **[Risk] Resource exhaustion with high fork count** — 100 concurrent SSH connections could overwhelm the controller or target network. → Mitigation: Document recommended limits; default to 1.
- **[Risk] Output buffer memory for long-running plays** — Buffering all output for 50 hosts could use significant memory. → Mitigation: Acceptable for v1; streaming mode can be added later.
- **[Trade-off] Output is delayed until all hosts complete** — Users don't see progress until the slowest host finishes. → Acceptable for correctness; live mode is a future enhancement.
