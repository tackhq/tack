## Context

`runPlayOnHost` (`internal/executor/executor.go:489-692`) couples four phases per host: facts gather, plan, plan-render + approval, apply. In `--forks 1` (default) the loop in `runPlay` (`:416-424`) runs the whole pipeline per host, sequentially. Operators see no consolidated view and approve N times for N hosts.

The parallel-fork branch (`:426-484`) pushes the same pipeline into per-host goroutines writing to per-host `bytes.Buffer`s. `PromptApproval` runs inside those goroutines (`:622-628`), invoking `bufio.Scanner.Scan()` on `os.Stdin` (`internal/output/output.go:494-520`). Multiple goroutines reading stdin simultaneously is undefined behavior — in practice today, `--forks > 1` only works under `--auto-approve`.

A separate change in flight, `parallel-fact-gathering`, parallelizes step 1 (fact gather) with a discover pre-pass that opens connectors, gathers facts, and reuses the open connector for the apply phase. This change builds on that foundation: extend the pre-pass to also produce a plan per host, then collect-render-approve on the main thread.

Constraints to respect:
- **Connector state is per-host, not goroutine-safe within a host.** `pctx.Connector.SetSudo(...)` (`:1479`) mutates the connector around `Check()` calls. Cross-host parallelism is fine because each host has its own connector instance; intra-host parallelism is forbidden.
- **`Check()` must be safe for concurrent calls across hosts.** Each `module.Checker` runs against a host-local connector, but we have not yet audited whether any checker uses package-level state. Audit is a task in this change.
- **Single-host fast path** must remain byte-identical so common-case users see no change.
- **`PlannedTask` carries `OldContent`/`NewContent`** (`internal/output/output.go:216-217`) — full file bodies. Aggregating across many hosts can grow RSS; we explicitly defer mitigation to a follow-up change unless reproduced.

## Goals / Non-Goals

**Goals:**
- A multi-host run renders one consolidated plan grouped by per-line host prefix, prompts once, then applies.
- Approval no longer runs inside any goroutine; the stdin race in `--forks > 1` mode is closed.
- Single-host serial flow is byte-identical to today.
- Adds a `host` field to JSON `plan_task` / `task_start` / `task_result` events (with schema version bump).
- No new CLI flags.

**Non-Goals:**
- Selective host opt-out at the approval prompt (`e`/`except`) — deferred to a follow-up change.
- Memory-streaming `OldContent`/`NewContent` for very large fleets — deferred unless reproduced.
- Restructuring stats aggregation — current `Stats` math (`:460-471`) carries over.
- Changing `--forks` semantics or default.
- Changing `gather_facts`/discovery semantics beyond what `parallel-fact-gathering` already covers.

## Decisions

### Decision 1: Per-line host prefix, no plan-time `HOST … [conn]` banners
Each plan line is rendered as `<host>: <indicator> <module> <name>`, with hostnames left-padded to a column width. No per-host section banner during plan render. The apply phase keeps its existing host headers/streaming behavior.

**Why:** User explicitly chose this format. Compact, greppable, and trivially line-oriented; consistent with terraform-style "+/~/-" prefixes that many ops users recognize. Avoids adding parallel rendering modes.

**Alternative considered:** Grouped sections per host. Rejected by user.

### Decision 2: Column-pad hostnames to `min(longestHostname, 30)`
If hostnames are uniformly short (`web1`, `db2`), pad to longest. SSM instance IDs (`i-0817eea131fa23c39` ≈ 19 chars) push width up. Cap at 30 chars; truncate beyond with ellipsis (`i-0817eea131fa23…:`). Test covers the cap and truncation rules.

**Why:** Visual alignment matters for scanning a multi-host plan. A hard cap prevents pathological hostnames from blowing past terminal width.

### Decision 3: Single-host plays use the existing `DisplayPlan` / `PromptApproval` path
`runPlay` branches on `len(play.Hosts)`:
- `== 1`: today's path — call `runPlayOnHost` exactly as before with `DisplayPlan` and `PromptApproval` inline.
- `> 1`: new aggregate path — collect plans, render via `DisplayMultiHostPlan`, prompt once, then run apply (serial or parallel).

**Why:** Maximally protects the 95% case from regression. A snapshot test enforces byte-identity on the single-host path.

**Alternative considered:** Always use the multi-host path with N=1. Rejected because it changes output framing (no `HOST` banner) for users who don't have multi-host plays.

### Decision 4: Discover and plan share one goroutine per host
Reuse the `parallel-fact-gathering` pre-pass goroutine to also call `planTasks`/`planBlock`/`planHandlers` after `facts.Gather` returns. The connector is hot, sudo state is host-local, and plan latency is negligible compared to fact-gather; a separate plan pool would add complexity without meaningful concurrency benefit.

**Why:** Plan is microseconds for most modules once facts are gathered; `Check()` calls dominate only for I/O-heavy modules like `template`/`copy` against remote files. We'd rather pay one connector round-trip pattern (already shaped by discover) than fan out a second time.

**Alternative considered:** Separate plan stage with its own pool. Rejected — adds code and a second connector handoff for negligible win.

### Decision 5: Approval moves to `runPlay` main thread
After plans are aggregated, the executor calls `e.Output.DisplayMultiHostPlan(...)` and `e.Output.PromptApproval()` on the main emitter, never from a goroutine. The parallel-fork branch only runs apply concurrently.

**Why:** This is the correctness fix. It deletes the stdin race and makes SIGINT semantics during approval predictable (zero hosts applied, instead of partial state).

### Decision 6: Footer math counts every host, including no-ops
Footer reads `Plan: X to change, Y to run, Z ok across N hosts (M unchanged).` `M` counts hosts with zero `will_change` / `will_run` / `always_runs` tasks. `N` is `len(play.Hosts)`. `X`/`Y`/`Z` are sums of task statuses across all hosts.

**Why:** Operators want a single line that summarizes both per-task and per-host impact. The "(M unchanged)" parenthetical answers "how many hosts am I touching?" without forcing a count of plan lines.

### Decision 7: JSON `host` field with schema version bump
`plan_task`, `task_start`, `task_result` events gain a `host` field. Bump the schema version constant in `internal/output/json.go`. CHANGELOG calls out the addition.

**Why:** JSON consumers shouldn't have to infer host from preceding `host_start` events; it's brittle. A version bump signals the additive change to disciplined consumers.

## Risks / Trade-offs

- **Risk:** Some `module.Checker` implementer carries hidden global state (cache map, package-level temp dir).
  → **Mitigation:** Tasks include an audit pass over `module.Checker` implementers and a race test that runs `Check()` concurrently against 10 in-process local connectors for every built-in module.

- **Risk:** Single-host fast path drifts from current output during refactor.
  → **Mitigation:** Snapshot test on a representative single-host playbook locks the output. Multi-host renderer gated behind `len(hosts) > 1` so the path is dead code for single-host plays.

- **Risk:** Long hostnames break alignment or terminal width.
  → **Mitigation:** Width cap at 30 + ellipsis truncation. Snapshot test for SSM instance IDs.

- **Risk:** Aggregating plans across 100+ hosts with file-bearing diffs grows RSS unboundedly.
  → **Mitigation:** Acknowledged and deferred to a separate change. Add a `// TODO(memory)` comment at the aggregate site so it's findable.

- **Trade-off:** SIGINT semantics during approval change. Today: partial state possible. After: zero state. This is strictly better but observable; CHANGELOG entry required.

- **Trade-off:** JSON `host` field on every plan/task event slightly enlarges output. Negligible for fleets under 1000 events; consumers who pre-bucketed by `host_start` see redundancy. Schema version bump signals the change.

## Migration Plan

- Land `parallel-fact-gathering` first.
- Land this change as a follow-up. No flag-gating; the multi-host path activates whenever `len(hosts) > 1`.
- Rollback: revert the executor + output diff. `PlannedTask.Host` and JSON `host` fields are additive and harmless if downstream code ignores them.
- Validation:
  - `make test -race` clean.
  - Re-run the user's four-SSM-host playbook; confirm per-line prefixes and single approval.
  - Re-run a single-host serial playbook; confirm byte-identical output via snapshot.

## Open Questions

- **CHANGELOG location**: this repo doesn't appear to have a `CHANGELOG.md` yet (need to confirm). If absent, the JSON schema bump should still be called out in release notes via the existing release flow (`.goreleaser.yaml`).
- **Per-line prefix color**: should hostnames be colorized (e.g., a stable hash → ANSI color) for visual scanning at 50-host scale? Defaults: bold prefix, no color rotation. Add later if asked.
- **Apply phase output during `--forks > 1`**: today, `parallel.go:FlushBuffers` writes per-host buffered output in host order after all hosts complete. This change does not alter that behavior; users can still adopt per-line prefixes in apply output via a future enhancement if requested.
