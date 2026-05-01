## 1. Emitter interface and text mode

- [x] 1.1 Add `HostFactsResult(host string, ok bool, errMsg string)` and `PlayHosts(hosts []string)` to the `Emitter` interface in `internal/output/emitter.go`.
- [x] 1.2 Update `Output.HostStart(host, connType)` in `internal/output/output.go` to write `HOST <host> [<conn>]` without a trailing newline (line is closed by the next emitter call).
- [x] 1.3 Implement `Output.HostFactsResult(host, ok, errMsg)`: append ` - gathering facts ✓\n` on success, ` - gathering facts ✗\n` on failure, then `Error(errMsg)` on a follow-up line. Adds an internal flag/state to know whether the prior banner is "open".
- [x] 1.4 Add `Output.HostStartDone(host)` (or equivalent) that closes the banner with a newline when `gather_facts: false` skips the fact step.
- [x] 1.5 Update `Output.PlayStart(play)` to emit nothing when `play.Name` is empty; remove the join-hosts fallback.
- [x] 1.6 Implement `Output.PlayHosts(hosts)`: render `HOSTS <list>` with the ≤5 / >5 truncation rule.

## 2. JSON emitter

- [x] 2.1 Add no-op `HostFactsResult` and `PlayHosts` to `internal/output/json.go`.
- [x] 2.2 Decide and implement: keep or retire the `task_start`/`task_result` events for "Gathering Facts". Default: retire after grepping for consumers; if any test/doc asserts on them, keep them and note the rationale.

## 3. Executor wiring

- [x] 3.1 In `internal/executor/executor.go` `preparePlayContext`, replace the `TaskStart("Gathering Facts", "")` / `TaskResult("Gathering Facts", "ok"|"failed", ...)` pair with `HostFactsResult(host, ok, errMsg)`. Keep the prep-pass branch (where facts are reused) unchanged.
- [x] 3.2 When `play.ShouldGatherFacts()` is false, ensure the executor calls `HostStartDone(host)` (or the chosen line-close API) so the HOST banner is properly newline-terminated.
- [x] 3.3 In the multi-host path (`runMultiHostPlay`), call `e.Output.PlayHosts(play.Hosts)` on the main thread after `PlayStart` (if any) and before `flushPrepBuffers`.
- [x] 3.4 Confirm the parallel pre-pass still produces correct ordering: each host's buffered emitter sees `HostStart` and then `HostFactsResult` (or `HostStartDone`) and the flushed text contains the inlined banner per host.
- [x] 3.5 Audit error paths between `HostStart` and the next emitter call (connection errors, signal handling) and ensure the half-rendered banner is closed with a newline before any error message prints.

## 4. Tests

- [x] 4.1 Unit-test `Output.HostStart` + `HostFactsResult` to confirm the combined output is `HOST <h> [<c>] - gathering facts ✓\n` (and the failure variant).
- [x] 4.2 Unit-test `Output.PlayStart` to confirm anonymous plays produce no output and named plays produce `PLAY <name>\n`.
- [x] 4.3 Unit-test `Output.PlayHosts` for: 2 hosts inline, exactly 5 hosts inline, 6 hosts (one overflow), 12 hosts (`(and 7 more)`).
- [x] 4.4 Unit-test the `gather_facts: false` path renders `HOST <h> [<c>]\n` with no suffix.
- [x] 4.5 Update existing executor and output tests that snapshot the start-of-play output (search for `Gathering Facts` and `PLAY ` in `*_test.go`).
- [x] 4.6 Add a multi-host parallel test that flushes buffered host outputs and confirms each host's banner text is intact (no torn lines, correct fact suffix).
- [x] 4.7 Run `make lint` and `make test` clean.

## 5. Documentation

- [x] 5.1 Update `docs/connectors.md` "Multi-host Plan & Approval" sample output to reflect new banners (`PLAY` only when named, new `HOSTS` line, inlined fact suffix).
- [x] 5.2 If `README.md` or `llms.txt` shows sample output, update them to match.
- [x] 5.3 If any `examples/` README references the old `Gathering Facts` task line, update.
