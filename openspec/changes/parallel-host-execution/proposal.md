## Why

Bolt currently executes plays against hosts one at a time. For playbooks targeting more than a handful of hosts, this is a production blocker — a 50-host deployment takes 50x the time of a single host. The `--forks` CLI flag already exists but is silently ignored, creating a false expectation. Parallel execution is the single most requested capability for real-world adoption.

## What Changes

- Implement the `--forks N` flag to execute plays across up to N hosts concurrently using a goroutine worker pool
- Buffer per-host output to prevent interleaved terminal output
- Collect errors across hosts and report a unified summary
- Default to `--forks 1` (serial execution) for backward compatibility
- Add `BOLT_FORKS` environment variable support

## Capabilities

### New Capabilities
- `parallel-execution`: Concurrent host execution with configurable parallelism, output buffering, and error aggregation

### Modified Capabilities

_None — this enhances the executor without changing existing module or connector interfaces._

## Impact

- **Modified code**: `internal/executor/executor.go` — worker pool around `runPlayOnHost()`
- **Modified code**: `internal/output/output.go` — buffered per-host output writer
- **Modified code**: `cmd/bolt/main.go` — wire up `--forks` flag (already declared)
- **No dependency changes** — uses only Go standard library (`sync`, `context`)
- **No breaking changes** — default forks=1 preserves current behavior
