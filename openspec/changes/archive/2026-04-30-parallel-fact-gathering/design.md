## Context

The executor's `runPlayOnHost` (`internal/executor/executor.go:566-576`) currently calls `facts.Gather(ctx, conn)` synchronously after each host's connector dials. In serial mode (`--forks 1`, the default) this means N sequential round-trips to N hosts. The cost is dominated by SSM, where every `Execute()` call goes through `ssm:SendCommand` → polling `ssm:GetCommandInvocation` → S3 download for large payloads — typically 3–8 seconds per gather even for the small `factsScript`. With four hosts, fact gathering alone is 15–30 seconds before any real work starts.

Parallel host execution (`--forks N`) does parallelize fact gathering today (it's a side effect of running each host in its own goroutine), but users default to serial mode for predictable, ordered output. We want the latency win for the common case without forcing users to opt into apply-phase concurrency.

Existing infrastructure we can reuse:
- `executor.WorkerPool` (`internal/executor/parallel.go`) — already provides bounded concurrency with context cancellation and ordered result collection.
- `facts.Gather` is connector-scoped, context-cancellable, and stateless — safe to call concurrently when given independent connectors.
- `ConnectorFor` / `GetConnector` already constructs a fresh connector per host, so each goroutine has its own connection.

Constraint: per `parallel-execution` spec, output must remain ordered by host in serial mode. Fact-gathering output today is a single per-host `TaskStart`/`TaskResult` pair — small enough to buffer.

## Goals / Non-Goals

**Goals:**
- For multi-host plays, gather facts across all hosts concurrently before the per-host plan/apply loop.
- Apply unconditionally for `--forks 1` and `--forks N` alike; fact gathering is read-only and trivially parallelizable.
- Keep current per-host output for `Gathering Facts` (`✓ Gathering Facts` line) — order it by host in serial mode.
- Preserve per-host failure semantics: a fact-gather error fails only that host (when `--forks > 1`) or fails the play (current serial behavior, since one host's failure already stops the play in serial mode).
- Reuse `WorkerPool` rather than adding a new concurrency primitive.

**Non-Goals:**
- Changing `--forks` semantics or default (still 1 for apply phase).
- Caching facts across runs (out of scope; the existing in-memory cache lives only for the run).
- Parallelizing anything beyond fact gathering (e.g., the plan or apply phases) — that is `parallel-execution`'s domain.
- New CLI flags. The pre-pass should be transparent. (A future flag like `--no-parallel-facts` could be added if needed, but is deferred unless a real use case emerges.)
- Changing `facts.Gather`'s signature or output format.

## Decisions

### Decision 1: Pre-pass before per-host plan/apply loop
Run a fact-gathering pass over all hosts at the start of `runPlay` (after host expansion, before the per-host loop). Store gathered facts in a `map[string]map[string]any` keyed by host, then have `runPlayOnHost` consume from that map instead of calling `facts.Gather` itself.

**Why:** Cleanest separation. Plan/apply still runs serially in serial mode (preserving its output ordering); only the fact-gather phase fans out. No restructuring of the existing loop.

**Alternative considered:** Hook into `runPlayOnHost` to launch fact gathering in a separate goroutine and join later. Rejected — coupling makes per-host buffering harder, and we'd need to thread a future-like primitive through.

### Decision 2: Reuse WorkerPool with a per-play concurrency cap
Use `WorkerPool` with a fork count of `min(len(hosts), factsConcurrency)` where `factsConcurrency` defaults to a sane ceiling. Initial value: **20** (matches typical SSM `MaxConcurrency` defaults and avoids hammering AWS API limits). This is independent of `--forks`.

**Why:** AWS SSM has account-level rate limits (e.g., `SendCommand` TPS); unbounded fan-out across 100+ hosts could trigger throttling. Local/SSH have no such ceiling but Go's runtime handles a few dozen goroutines trivially. A constant ceiling is simpler than per-connector tuning and rarely matters until you have very large fleets.

**Alternative considered:** Per-connector dynamic limit (e.g., 5 for SSM, unlimited for local). Rejected for first iteration — adds complexity; the constant 20 covers typical fleets and SSM rate-limit headroom.

### Decision 3: Per-host buffered output for fact-gather lines
During the pre-pass, each goroutine writes its `TaskStart`/`TaskResult` for "Gathering Facts" to a per-host buffered emitter (same approach as `parallel-execution`'s `FlushBuffers`). After all hosts complete, flush in host order.

**Why:** Maintains today's ordered-by-host output for the serial path. The added latency from buffering before flush is negligible (sub-second per-host text payloads) and only affects the fact-gather phase.

**Alternative considered:** Stream output as it completes (interleaved). Rejected — visually noisy and inconsistent with serial-mode expectations.

### Decision 4: Fail-fast in serial mode, per-host in parallel mode
- If `--forks 1` (serial) and any host's fact gather fails, return the play error immediately (matches today's semantics — first host failure stops the play).
- If `--forks > 1` (parallel), record the per-host failure and let the apply phase fail just that host (matches `parallel-execution`'s error aggregation).

**Why:** Surprising behavior change ("it used to fail-fast on host A; now host B already wasted SSM quota") if we changed serial-mode failure semantics. Keep the user-visible contract intact.

### Decision 5: Skip the pre-pass for single-host or local-connection plays
- If `len(hosts) <= 1`, just gather inline as today (no goroutine overhead, no buffering).
- If `connection: local`, the current code path runs once with `host = "localhost"`; no parallelization needed.

**Why:** Zero benefit, simpler code path, identical output to today. The pre-pass is opt-in only when there's something to parallelize.

### Decision 6: No new connector-side changes
Each goroutine constructs its own connector via the existing `GetConnector(play, host)` helper, calls `Connect`, runs `facts.Gather`, then either keeps the connection alive (reused by `runPlayOnHost`) or closes it.

**Why:** `facts.Gather` already works through the `Connector` interface; SSM/SSH/Docker connectors are independent objects. The simplest design is "reuse the connection that was opened for fact gathering" — pass the open connector along with the gathered facts to `runPlayOnHost`.

**Alternative considered:** Open a connection just for facts, close it, then re-open for apply. Rejected — wastes the SSH handshake / SSM session setup, which is itself expensive.

## Risks / Trade-offs

- **Risk:** Output buffering during fact-gather changes the timing of "✓ Gathering Facts" lines (they appear in a batch instead of one-by-one as each host finishes).
  → **Mitigation:** Total fact-gather wall time is now bounded by the slowest host, so the batch appears sooner overall than the last serial line did. Document in change notes.

- **Risk:** Connection reuse complicates cleanup — if fact gather succeeds but the host is later excluded from apply (e.g., user cancels approval), we still need to close the connector.
  → **Mitigation:** Existing `defer conn.Close()` in `runPlayOnHost` covers the apply path. For pre-pass failures, the failing goroutine closes its own connector before returning.

- **Risk:** SSM rate-limit throttling at very high fan-out.
  → **Mitigation:** `factsConcurrency` ceiling of 20. If a user hits throttling we can lower the default or add a flag.

- **Risk:** A buggy connector implementation that's not goroutine-safe could regress (though all current connectors create independent state per-instance).
  → **Mitigation:** Each goroutine has its own connector instance — no shared state — so connector-internal mutability isn't crossed. Add a unit test that exercises 10 concurrent gathers against the local connector to lock this in.

- **Trade-off:** Slightly more complex executor flow (a pre-pass, then the existing loop). Worth it: the latency win on SSM is the headline value of the change.

- **Trade-off:** If `gather_facts: false` for the play, the pre-pass is skipped and behavior is identical to today.

## Migration Plan

No migration needed — pure performance improvement with no API/CLI/playbook surface changes.

- Default rollout: behavior change is automatic on next release.
- Rollback: revert the executor diff; `facts.Gather` is unchanged.
- Validation: run the SSM integration tests (`tests/integration/ssm_test.go` if present, or the docker-based `tests/integration/`) and confirm fact gathering still produces identical fact maps. Time the four-host SSM example in the user's prompt to verify the speedup.

## Open Questions

- Should `factsConcurrency` be configurable via a flag/env var (e.g., `TACK_FACTS_FORKS`) before shipping? Lean toward "no, constant 20 is fine, add later if asked."
- For SSM specifically, should we batch multiple hosts into a single `SendCommand` invocation (SSM supports targeting multiple instances per command)? That would be a much larger redesign — defer to a separate change. The fan-out approach in this proposal still wins ~N× compared to today and doesn't preclude future batching.
