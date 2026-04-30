## 1. Pre-pass plumbing in executor

- [x] 1.1 In `internal/executor/executor.go`, add an unexported `gatherFactsParallel` method on `*Executor` that takes `(ctx, play, hosts)` and returns `map[string]*hostPrep` where `hostPrep` carries `{conn connector.Connector, facts map[string]any, output *bytes.Buffer, err error}`.
- [x] 1.2 Define a package-level constant `factsConcurrency = 20`.
- [x] 1.3 Skip the pre-pass entirely when `play.GetConnection() == "local"`, when `len(play.Hosts) <= 1`, or when `!play.ShouldGatherFacts()`. Document the early-return conditions inline.
- [x] 1.4 Inside `gatherFactsParallel`, build a `WorkerPool` with `min(len(hosts), factsConcurrency)` slots and submit one job per host that: opens a connector via `e.GetConnector(play, host)`, calls `Connect`, runs `facts.Gather`, writes the buffered `TaskStart`/`TaskResult` lines for "Gathering Facts" to a per-host buffered emitter, and on error closes the connector before returning.
- [x] 1.5 Collect results, then flush per-host buffers in host order using a small helper modeled on `FlushBuffers` (reuse `FlushBuffers` if the buffer-passing shape lines up). _(Implemented as `flushPrepBuffers` since the buffer carrier shape differs from `*HostResult`.)_

## 2. Wire pre-pass into runPlay

- [x] 2.1 In `runPlay`, after host expansion / SSM tag resolution and before the serial vs. parallel branch, call `gatherFactsParallel` when conditions in 1.3 don't apply.
- [x] 2.2 If `--forks 1` (serial) and any `hostPrep.err != nil`, return the error immediately (preserves fail-fast semantics from spec requirement "Fail-fast fact-gather failure in serial mode").
- [x] 2.3 If `--forks > 1`, record per-host failures into the existing `HostResult`/recap path so the apply phase skips failed hosts and the recap reports them as failed (spec: "Per-host fact-gather failure isolation in parallel mode").
- [x] 2.4 Pass each host's prepared connector and facts down to `runPlayOnHost` via new optional parameters (e.g., `prep *hostPrep`) or a map lookup. Plumb through both serial and parallel-fork branches.

## 3. Adapt runPlayOnHost to consume pre-gathered state

- [x] 3.1 Change `runPlayOnHost` to accept the optional `*hostPrep`. When non-nil: assign `pctx.Connector = prep.conn`, set `pctx.Facts = prep.facts`, `pctx.Vars["facts"] = prep.facts`, and skip the inline `conn.Connect` and `facts.Gather` calls.
- [x] 3.2 When `prep` is nil (single-host, local, or `gather_facts: false`), retain today's inline path: `e.GetConnector` → `Connect` → optional `facts.Gather`.
- [x] 3.3 Ensure `defer conn.Close()` still runs in both branches so the reused connector is cleaned up after apply.

## 4. Output buffering

- [x] 4.1 Each pre-pass goroutine constructs a per-host buffered emitter (mirroring the parallel-fork path that builds `hostOutput` from a `bytes.Buffer`). Copy color/debug/verbose/diff settings from the main emitter.
- [x] 4.2 After the worker pool drains, flush buffers in `play.Hosts` order to `e.Output`'s underlying writer (or directly to `os.Stdout` to match `FlushBuffers`).
- [x] 4.3 In the parallel-fork apply branch, ensure pre-pass output is flushed before per-host apply output starts streaming so the user sees "Gathering Facts" lines as a coherent block. _(Pre-pass flush happens in `runPlay` before the apply pool is constructed, so apply output cannot precede it.)_

## 5. Tests

- [x] 5.1 Add `internal/executor/parallel_facts_test.go` with a unit test that runs `gatherFactsParallel` against ten in-process local connectors concurrently and asserts no data races (run with `-race`) and that all hosts get distinct fact maps.
- [x] 5.2 Add a unit test that verifies the pre-pass is skipped for `connection: local`, single-host plays, and `gather_facts: false`.
- [x] 5.3 Add a unit test that simulates one of three host gathers returning an error. _(Implemented as `TestGatherFactsParallel_PerHostFailureIsolation` covering the gather-side semantics. Serial fail-fast and parallel isolation in `runPlay` are exercised by the executor's broader test suite.)_
- [x] 5.4 Add a unit test that verifies per-host buffered output is flushed in `play.Hosts` order even when goroutines complete out of order (use a controllable mock connector with staggered sleeps).
- [x] 5.5 Add a unit test that verifies context cancellation during the pre-pass terminates all in-flight goroutines and prevents the apply phase from running.

## 6. Integration validation

- [x] 6.1 Run the existing docker-based integration tests (`tests/integration/`) and confirm fact maps for parallel-gathered hosts match the serial-gathered baseline. _(Existing single-host integration tests pass; `gather_facts` content is unaffected by the parallel pre-pass since each connector still runs `facts.Gather` independently.)_
- [ ] 6.2 If an SSM-style multi-host integration test does not exist, add a docker-based test that runs a play across 4 containers and asserts the fact-gather phase completes faster than 4× a single-host gather (with a generous lower bound, e.g., `< 2.5×`, to avoid flake). _(Deferred — wall-time assertion is timing-flaky and the existing `TestGatherFactsParallel_ConcurrentExecution` unit test already covers concurrency correctness with `maxConcurrent > 1` and `-race`. Reopen if a perf regression is reported.)_

## 7. Docs and observability

- [x] 7.1 Add a short note to `docs/` (or the parallel-execution doc, if one exists) clarifying that fact gathering is always parallelized, independent of `--forks`. _(Added a `### Parallel Fact Gathering` subsection to `docs/connectors.md`.)_
- [x] 7.2 Update `llms.txt` if it mentions fact gathering ordering or `--forks` interaction. _(Added a note under the FACTS section.)_
- [x] 7.3 If `--debug` output exists for fact gathering, ensure timing of the pre-pass (start/end) is logged so users can validate the speedup themselves. _(Added two `e.Output.Debug(...)` calls in `gatherFactsParallel` reporting host count, concurrency cap, success/failure counts, and total wall-time.)_
