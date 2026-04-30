## Why

Today, multi-host runs make it impossible to tell which host a plan line, diff, or approval prompt belongs to. The user sees:
```
Plan: 3 to change, 44 ok, 1 to run.
Do you want to apply these changes? (yes/no): yes
```
…with zero host attribution, repeated per host. In `--forks 1` (the default), the executor runs `Gathering Facts → plan → DisplayPlan → PromptApproval → apply` serially per host, so the operator approves N times for N hosts and can't see the consolidated impact before deciding.

A latent **correctness bug** compounds it: in `--forks > 1`, `PromptApproval()` runs inside per-host goroutines (`internal/executor/executor.go:622-628`) writing to per-host `bytes.Buffer`s while still reading stdin (`internal/output/output.go:494-520`). Multi-goroutine stdin reads are undefined; in practice forks mode is only safe with `--auto-approve`. This change fixes that as a deliberate goal, not a side effect.

## What Changes

- Add `Host string` to `PlannedTask`; populate it during plan computation so each task carries its origin host.
- Strip `DisplayPlan`, `PromptApproval`, dry-run-exit, and the no-op-shortcut path out of `runPlayOnHost`. The function returns `[]PlannedTask` instead of rendering or prompting inline.
- Aggregate plans across hosts in `runPlay`, render once with **per-line host prefixes** (`web1: + install nginx`), prompt **once globally**, then run apply (serial or via existing `WorkerPool`).
- Plan render: column-align hostnames; no-op hosts contribute zero plan lines (only counted in footer); footer reads `Plan: X to change, Y to run, Z ok across N hosts (M unchanged).`
- Single-host plays remain on the existing `DisplayPlan`/`PromptApproval` fast path — output is byte-identical to today.
- JSON emitter: add `host` field to `plan_task` / `task_start` / `task_result` events; bump schema version. **Potentially BREAKING** for JSON consumers that didn't expect the new field — documented in CHANGELOG.
- **BREAKING (subtle)**: SIGINT during the (now single) approval prompt aborts the play with zero hosts applied, instead of "first N-1 already applied, host-N's approval was interrupted." This is strictly safer; documented in CHANGELOG.

## Capabilities

### New Capabilities
- `consolidated-plan-and-approval`: Cross-host plan aggregation, per-line host attribution in plan output, and a single global approval prompt for multi-host plays.

### Modified Capabilities
- `parallel-execution`: Approval is no longer per-host; it runs once on the main thread before parallel apply dispatch. Closes the stdin-race bug.

## Impact

- **Affected code**:
  - `internal/executor/executor.go` — restructure `runPlay` collect-then-render-then-apply; return-only `runPlayOnHost`.
  - `internal/executor/parallel.go` — reuse `WorkerPool` for discover+plan goroutines; no new helper.
  - `internal/output/output.go` — `PlannedTask.Host`; new `DisplayMultiHostPlan`; per-line prefix renderer; updated footer math.
  - `internal/output/emitter.go` — extend interface with `DisplayMultiHostPlan`.
  - `internal/output/json.go` — `host` field on plan/task events; schema version bump.
- **APIs**: `PlannedTask` gains a field (additive); `Emitter` interface gains a method (every implementer must update — affects `Output` and JSON emitter).
- **Dependencies**: None added.
- **Composition**: Builds on `parallel-fact-gathering` (separate change). Order: that ships first, this on top.
- **UX**: Multi-host plays now render with per-line host prefix and one approval; single-host plays unchanged. CHANGELOG entry mandatory.
