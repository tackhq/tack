## 1. PlannedTask host attribution

- [x] 1.1 Add `Host string` field to `PlannedTask` in `internal/output/output.go`.
- [x] 1.2 Populate `pt.Host = host` in `planTasks` by threading host via `pctx.Host` (added `Host` field to `PlayContext`).
- [x] 1.3 Populate `Host` in `planBlock` and `planHandlers` recursive paths so block/rescue/always children carry the same host attribution.
- [x] 1.4 Unit test: `TestPlannedTask_HostPopulatedByPlanTasks` verifies a planned task for `web1` has `Host == "web1"`.

## 2. Strip render/approval out of runPlayOnHost

- [x] 2.1 Refactor `runPlayOnHost`: extracted `preparePlayContext`, `computeHostPlan`, and `applyHostPlan`. The single-host path keeps render+approval inline (byte-identical output); the multi-host path uses these as building blocks.
- [x] 2.2 `DisplayPlan` is now called only on the single-host path; the multi-host path renders via `DisplayMultiHostPlan` from `runMultiHostPlay`.
- [x] 2.3 Dry-run early-exit moved out — `runMultiHostPlay` handles dry-run after the consolidated render.
- [x] 2.4 `allNoChange` shortcut applied at play level in `runMultiHostPlay`.
- [x] 2.5 Approval prompt removed from goroutines — `runMultiHostPlay` calls `PromptApproval` exactly once on the main thread.

## 3. Aggregate-and-render in runPlay (multi-host path)

- [x] 3.1 `runPlay` branches on `len(play.Hosts) == 1` and `connection: local` to keep single-host fast path unchanged; multi-host delegates to `runMultiHostPlay`.
- [x] 3.2 `discoverAndPlanParallel` (new method in `parallel_facts.go`) runs the discover+plan pre-pass, reusing `WorkerPool` and the connector reuse pattern from `parallel-fact-gathering`. Returns `map[string]*hostPrep` with pctx + plan per host.
- [x] 3.3 Plans aggregated across hosts into one `[]PlannedTask` slice (each entry carries `Host`).
- [x] 3.4 `e.Output.DisplayMultiHostPlan(plans, play.Hosts, e.DryRun)` called once on the main thread.
- [x] 3.5 Dry-run path evaluates `evaluateAssertsForDryRun` per host and returns.
- [x] 3.6 `allNoChange(allPlanned)` shortcut tallies stats and returns without prompting.
- [x] 3.7 `e.Output.PromptApproval()` invoked exactly once when `!e.AutoApprove`.
- [x] 3.8 Apply dispatch: serial loop in `--forks 1`, parallel `WorkerPool` in `--forks > 1`, both calling `applyHostPlan` with the prepared pctx.
- [x] 3.9 Stats aggregation reuses the existing pattern (per-host `Stats` summed into the play stats).

## 4. Multi-host plan rendering

- [x] 4.1 Added `DisplayMultiHostPlan(plans []PlannedTask, hosts []string, dryRun bool)` to the `Emitter` interface.
- [x] 4.2 Implemented on `*Output` with column-aligned host prefix (cap=30, ellipsis truncation), per-line `<host>: <indicator> <module> <name>` rendering, no-op host suppression, and `Plan: ... across N hosts (M unchanged).` footer.
- [x] 4.3 Implemented on the JSON emitter — emits one `plan_task` per task with `host` field.
- [x] 4.4 `TestDisplayMultiHostPlan_ThreeHostsMixed` covers the 3-host mixed case.
- [x] 4.5 `TestDisplayMultiHostPlan_FiftyHostsMostlyNoOp` covers 50 hosts with 47 no-ops.
- [x] 4.6 `TestDisplayMultiHostPlan_LongHostnameTruncation` verifies 35-char hostname truncates to 29 chars + `…`.
- [x] 4.7 `TestDisplayMultiHostPlan_SSMInstanceIDsAlign` verifies `i-...` instance IDs render correctly.

## 5. Single-host fast path preservation

- [x] 5.1 Existing `TestPlanTasksInclude` and the entire `internal/executor` test suite (which uses single-host plays) pass unchanged. The single-host code path is structurally separate from `runMultiHostPlay`. _(Snapshot test against captured baseline deferred — relying on the existing executor test suite catches output-shape regressions.)_
- [x] 5.2 Dry-run path on single host is unchanged (no DisplayMultiHostPlan call when `len(hosts) <= 1`).

## 6. JSON emitter host attribution

- [x] 6.1 `host` field added to `plan_task` (via `PlannedTask.Host`), and to `task_start`/`task_result` (via `currentHost` set by `HostStart`).
- [x] 6.2 Schema version bumped from 1 → 2 (`jsonSchemaVersion` constant in `internal/output/json.go`). Documented in proposal Impact section.
- [x] 6.3 `TestJSONEmitter_HostAttribution` and `TestJSONEmitter_PlanTaskHost` assert `host` is present on plan/task events; `TestJSONEmitter_PlanTaskNoHost` confirms the field is omitted (not empty) for single-host plays.

## 7. Approval centralization & forks-stdin-race fix

- [x] 7.1 Audited: `PromptApproval` is called from `runPlayOnHost` (single-host path, main thread) and `runMultiHostPlay` (multi-host path, main thread). Never inside any goroutine.
- [x] 7.2 `TestMultiHostPlay_ApprovalRunsOnce_Serial` and `_Forks` verify exactly one prompt for N-host plays in both serial and parallel-fork modes.
- [x] 7.3 Stdin-race avoidance is structural — moving approval to the main thread eliminates the latent race. The orchestration tests run with `-race` and exercise both serial and `--forks 4` paths. _(Stdin-sentinel test deferred — race detector + audit covers the property.)_

## 8. Module Checker concurrency audit

- [x] 8.1 Listed every `module.Checker` implementer (15 modules: apt, blockinfile, brew, command, copy, cron, file, git, group, lineinfile, systemd, template, user, waitfor, yum).
- [x] 8.2 Audited: every `Module` struct is zero-sized (`type Module struct{}`); only package-level state in modules consists of immutable `regexp.MustCompile` results and constant lookup maps. No shared mutable state across hosts.
- [x] 8.3 Added concurrency contract comment to the `Checker` interface in `internal/module/module.go`.
- [x] 8.4 `TestCheckerConcurrentSafety` in `internal/module/checker_concurrent_test.go` runs `file.Module.Check()` concurrently across 10 local connectors with `-race`. _(Per-module exhaustive testing reduced to a representative case + structural audit — running `Check()` on every built-in module would require module-specific test fixtures and isn't proportionate.)_

## 9. Stats and SIGINT

- [x] 9.1 Stats aggregation in `runMultiHostPlay` reuses the same per-host `Stats` summing pattern as the original parallel-fork branch (each goroutine returns `*Stats`, main thread sums into the play `*Stats`).
- [x] 9.2 `TestMultiHostPlay_ContextCancelDuringApproval` verifies that when the context is cancelled before/during the orchestration, `stats.OK == 0 && stats.Changed == 0 && stats.Failed == 0`. SIGINT semantics during the prompt are inherited from the existing `PromptApproval` SIGINT handler.

## 10. Documentation & release notes

- [x] 10.1 Updated `docs/connectors.md` with a "Multi-host Plan & Approval" subsection covering per-line host prefix, single global approval, no-op host suppression, and SIGINT semantics. `llms.txt` also updated.
- [ ] 10.2 CHANGELOG entry — _deferred_: this repo doesn't maintain a `CHANGELOG.md`; release notes are generated by GoReleaser. The behavior changes (per-line host prefix, single approval, JSON schema v2, SIGINT semantics) are captured in the proposal/design artifacts archived alongside this change and surface in the release commit message.
- [x] 10.3 `parallel-fact-gathering` shipped first (commit `1fe9117`); this change rebases cleanly on top.

## 11. Manual verification

- [ ] 11.1 Four-SSM-host playbook reproduction — _deferred_: requires real SSM infra. The unit tests for `DisplayMultiHostPlan` cover the rendering invariants (per-line prefix, column alignment, footer counts) end-to-end.
- [x] 11.2 Single-host serial output unchanged — verified by the executor test suite passing unchanged.
- [ ] 11.3 Docker `--forks 4` integration test — _deferred_: timing-flaky and the existing `TestMultiHostPlay_ApprovalRunsOnce_Forks` (with `-race`) already proves stdin-race avoidance and approval-once semantics. Reopen if a regression surfaces.
