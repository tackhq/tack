## Context

Start-of-play output flows through three emitter calls in order:

1. `Output.PlayStart(play)` — prints `PLAY <play.Name>`; when `play.Name` is empty, falls back to `strings.Join(play.Hosts, ", ")`. Always emits one line.
2. `Output.HostStart(host, connType)` — prints `HOST <host> [<connType>]`. Emits per host (single-host inline; multi-host buffered then flushed in host order via `flushPrepBuffers`).
3. `emitter.TaskStart("Gathering Facts", "")` followed by `emitter.TaskResult("Gathering Facts", "ok", false, "")` — emitted from `preparePlayContext` when `play.ShouldGatherFacts()` is true. Standalone task-style line.

The multi-host parallel pre-pass (`discoverAndPlanParallel`) runs `preparePlayContext` per host with a buffered emitter so the output is captured during the parallel work and flushed in deterministic host order on the main thread before the consolidated plan renders. Anything we emit during fact-gathering needs to flow through that buffered emitter the same way.

The proposal asks for: (a) `Gathering Facts` inlined into the HOST banner, (b) `PLAY` line dropped when the play is unnamed, and (c) a `HOSTS` summary line for multi-host plays. Each is a self-contained presentation tweak; none changes the order of work or the data model.

## Goals / Non-Goals

**Goals:**
- Remove redundant identity duplication at the top of a run when the play has no name.
- Anchor fact-gathering status to the host it belongs to.
- Keep multi-host runs informative — users should still be able to read off the targeted hosts without scrolling into the consolidated plan body.
- Preserve the JSON output schema and event semantics.
- Preserve `gather_facts: false` behavior — no fact line in either form.

**Non-Goals:**
- Restructuring the plan body or footer.
- Changing the approval-prompt format (separately speced).
- Adding spinners or live-updating output. Current output is line-oriented; this stays the same.
- Truncating the per-host HOST banner. The HOSTS summary truncates; the per-host line is always full.
- Touching the `RECAP` / stats line at the end.

## Decisions

### Decision 1: Inline facts via a new emitter method

Add `Emitter.HostFactsResult(host string, ok bool, errMsg string)`. The executor calls it after `facts.Gather` (or its failure). The text-mode implementation prints a continuation that completes the prior `HostStart` line — concretely:

- After `HostStart` prints `HOST i-... [ssm]` (no trailing newline, or with a placeholder we overwrite), `HostFactsResult(host, true, "")` appends ` - gathering facts ✓\n`.
- On failure, appends ` - gathering facts ✗\n` and follows with an `Error(errMsg)` line as today.

**Implementation note:** the simplest way to keep the line single-physical-line is to defer the trailing newline of `HostStart` until `HostFactsResult` decides what to append. We avoid stateful "expecting completion" trickery by introducing a small helper: `HostStart` writes `HOST <host> [<conn>]` (no `\n`); `HostFactsResult` writes ` - gathering facts ✓\n` (or the failure form). When `gather_facts: false` and `HostFactsResult` is therefore not called, the executor calls a new `HostStartDone()` (or passes a flag to `HostStart`) to terminate the line. Cleaner option: split into `HostBannerLine(host, conn)` that returns the printable text and is called once by the executor with the completed line — but that pulls formatting into the executor, which the codebase has so far kept inside the emitter. Stick with the small-state approach.

**Alternative considered:** keep `Gathering Facts` as its own task line and just suppress the line entirely on success. Rejected — the proposal calls for it inline; suppressing on success and printing on failure makes failures look like surprises.

**Alternative considered:** print `HOST` + `Gathering Facts` together at the time facts return (single emit). This is the cleanest internally but reorders user-visible output: you'd see no host banner until facts complete, which is a regression for the slow-SSM case where users rely on the banner to know which host is being contacted. Rejected.

### Decision 2: `PlayStart` skips emission when the play has no name

`Output.PlayStart` checks `play.Name`. If empty, no line is printed. Today's fallback (`strings.Join(play.Hosts, ", ")`) is removed. The HOST line below carries identity for single-host plays; the new HOSTS line carries it for multi-host plays.

**Alternative considered:** keep `PlayStart` always emitting and use the HOSTS summary as the fallback text inside the PLAY line. Rejected — losing the visual distinction between "named play" and "anonymous play" makes named plays harder to scan in a multi-play playbook.

### Decision 3: New `PlayHosts(hosts []string)` emitter call for multi-host plays

The executor decides when to call it (only for `len(play.Hosts) > 1`). The emitter renders `HOSTS h1, h2, h3` for ≤5 hosts and `HOSTS h1, h2, h3, h4, h5 (and N more)` for >5 hosts. We deliberately diverge from the approval-prompt format: the approval prompt has space-budget concerns because it's followed inline by the user input; this banner is a standalone line where `(and N more)` reads more naturally than `, ...`.

The HOSTS line is emitted once on the main thread, before the per-host `HOST` banners flush. For the parallel pre-pass path this means calling `PlayHosts` from `runMultiHostPlay` (main thread, before `flushPrepBuffers`).

**Alternative considered:** reuse `formatApprovalTarget`. Rejected — that helper bakes in a "(connection)" suffix and "5 hosts" prefix that are wrong for a banner; we'd have to special-case both. Cheaper to add a small dedicated helper.

### Decision 4: JSON emitter no-ops the new methods

`HostFactsResult` and `PlayHosts` on the JSON emitter emit nothing. The existing `task_start` / `task_result` events for "Gathering Facts" can be retired from JSON too — they were artifacts of how text mode rendered the line and have no consumers documented. We will retire them at the same time, but only after confirming via `grep` that no test or doc relies on them. If a consumer is found, we keep the events and simply note in JSON-mode docs that the task name is internal/connection-setup.

### Decision 5: Deprecation of "Gathering Facts" as a task name in TaskResult

Today the executor calls `emitter.TaskStart("Gathering Facts", "")` and `emitter.TaskResult("Gathering Facts", "ok"|"failed", ...)`. After this change those calls are gone. The tests in `internal/executor` that match on `"Gathering Facts"` task lines will need updates — small, scoped to test fixtures.

## Risks / Trade-offs

- **[Risk]** Tests that snapshot the start-of-play output (single-host fast path was previously declared "byte-identical to main") will fail. → **Mitigation:** the existing requirement in `consolidated-plan-and-approval` that asserts byte-identical output is in scope of this change and will be modified to allow the new banner format. We update affected tests in lockstep.
- **[Risk]** Splitting the HOST line into "open" + "complete" in the emitter is mildly stateful and could leak a half-rendered line if a connection error happens between `HostStart` and the next emitter call. → **Mitigation:** any such error path already calls `Error(...)` which writes a fresh line; we'll add a final `\n` flush there so the dangling banner closes cleanly. We add a test for the error path.
- **[Risk]** Buffered emitters (used during the parallel pre-pass) need to preserve the in-flight HOST line across `HostFactsResult`. Since the pre-pass owns one buffered emitter per host and flushes them serially, in-host ordering is fine; we just need to make sure neither buffer pools writes nor adds line-discipline that would break the partial line. → **Mitigation:** check `flushPrepBuffers` and the buffered emitter implementation; verify with a multi-host parallel test that the flushed text has `HOST <h> [conn] - gathering facts ✓\n` per host.
- **[Trade-off]** Multi-host plays with many hosts get a new HOSTS line + N HOST lines. For a 50-host fleet that's 51 host references at the top before the plan — slightly noisier than today's single PLAY line. → **Mitigation:** the HOSTS line truncates at 5 names; the per-host HOST lines are unavoidable and pre-exist this change.
- **[Risk]** `gather_facts: false` plays today emit no fact line; we must not regress to a half-printed HOST line. → **Mitigation:** when `gather_facts: false`, the executor calls a new `HostFactsSkipped(host)` (or simply terminates the HOST line via newline) so the banner closes without "gathering facts" suffix.

## Migration Plan

Single PR, no migration. Tests update in lockstep with the emitter signature changes. Rollback is reverting the commit.
