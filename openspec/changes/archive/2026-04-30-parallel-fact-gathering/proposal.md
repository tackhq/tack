## Why

Fact gathering is the slowest step on high-latency connectors (especially SSM, where each `Execute()` round-trip costs several seconds for command submission, polling, and result retrieval). Today, in serial execution mode (the default `--forks 1`), the executor calls `facts.Gather()` per host one after another inside `runPlayOnHost`. With four SSM hosts that's ~4× the latency of a single gather — even though fact gathering is read-only, hosts don't depend on each other, and the work is embarrassingly parallel. Users who don't want to opt into full parallel apply (`--forks N`) still pay this serial cost on every run, including dry-runs and plan previews.

## What Changes

- Add a fact-gathering pre-pass in the executor: when a play has multiple target hosts and `gather_facts` is enabled, gather facts for all hosts concurrently before the per-host plan/apply loop runs.
- Apply this pre-pass regardless of `--forks` value — fact gathering is always safe to parallelize since it makes no state changes.
- Reuse the existing `WorkerPool` concurrency-limit primitive so users can cap concurrent fact gathers via a knob (default: gather all hosts at once up to a sane ceiling like the connector-specific recommended limit).
- Cache gathered facts on the play context so `runPlayOnHost` consumes the pre-gathered result instead of calling `facts.Gather()` again.
- Surface a single consolidated "Gathering Facts" output line per host (preserving today's UX) but emit them as they complete, then proceed into per-host plan/apply.
- Handle partial failures: a host whose fact gather fails SHALL fail just that host (with the same error path as today), not the whole play.

No breaking changes to playbook syntax, CLI flags, or output format for the serial single-host case.

## Capabilities

### New Capabilities
- `parallel-fact-gathering`: Concurrent fact gathering across all target hosts in a play, decoupled from the `--forks` apply-phase concurrency setting.

### Modified Capabilities
<!-- None — parallel-execution still describes apply-phase concurrency; this is a separate capability that runs unconditionally for multi-host plays. -->

## Impact

- **Affected code**:
  - `internal/executor/executor.go` — add fact-gathering pre-pass in `runPlay`; remove (or guard) the inline `facts.Gather()` call in `runPlayOnHost`.
  - `internal/executor/parallel.go` — `WorkerPool` is reused as-is; may add a small helper for fact-only fan-out if needed.
  - `pkg/facts/facts.go` — no API change; `Gather` is already context-cancellable and connector-scoped.
- **APIs**: No public API changes. `PlayContext.Facts` is populated before plan/apply as it is today.
- **Dependencies**: None added.
- **Performance**: For an N-host SSM play, fact-gathering wall time drops from N×t to ~t (bounded by the slowest host) modulo the concurrency ceiling. Local/SSH plays see smaller but still meaningful improvements.
- **UX**: Output ordering for fact-gather lines may interleave during the pre-pass (mitigated by per-host buffering, same approach as `parallel-execution`). The plan/apply phase output is unchanged.
- **Testing**: New unit tests for the pre-pass orchestration; integration tests already cover SSM and Docker fact gathering and will exercise the parallel path automatically once enabled.
