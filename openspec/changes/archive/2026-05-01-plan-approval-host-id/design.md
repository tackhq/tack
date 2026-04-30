## Context

Approval prompts live in two paths today:

- Single-host: `internal/executor/executor.go:runPlayOnHost` calls `emitter.PromptApproval()` after `emitter.DisplayPlan(planned, dryRun)`. The `HostStart(host, connType)` banner runs above the plan, but several lines higher than the prompt line.
- Multi-host: `runMultiHostPlay` calls `e.Output.PromptApproval()` once globally after `DisplayMultiHostPlan(allPlanned, play.Hosts, dryRun)`. The plan body has per-line host attribution; the prompt line itself is generic.

`PromptApproval()` is on the `Emitter` interface (implemented by `output.Output` and `output.JSONEmitter`). Today it takes no parameters and prints a fixed string. The JSON emitter auto-approves and ignores the prompt text.

The proposal asks for the prompt to identify the host(s) directly. The change is small and localized: enrich the prompt text and pass enough context from the executor into the emitter. This change touches only the user-facing prompt line and `Apply cancelled.` info message — plan body rendering is unchanged.

## Goals / Non-Goals

**Goals:**
- Make the approval prompt unambiguous about what host(s) it covers.
- Show connection type for single-host prompts (so SSM/SSH/Docker/local are distinguishable when hostnames collide).
- Keep multi-host prompts readable when host counts are large (cap visible names, append `...`).
- No regression to JSON output, `--auto-approve`, or the plan body rendering.
- Single-host output stays close to current — the additive prompt suffix is the only change above-the-fold.

**Non-Goals:**
- Adding inventory metadata (tags, region, ASG) to the prompt. The host name + connection type is the contract; richer identification is a follow-up if asked.
- Changing the actual approval keystroke (`y`/`yes` is unchanged).
- Per-host approval inside a multi-host play. Approval is still global, matching the current `consolidated-plan-and-approval` capability.
- Reformatting the `HOST <host> [<conn>]` banner or the plan footer.

## Decisions

### Decision 1: Replace `PromptApproval()` with `PromptApproval(target string)`

Add a single string parameter that the executor formats and the emitter prints verbatim. The emitter does not know about Play structs; the executor decides what the human-readable target is. Examples:

- Single-host: `web1.prod (ssh)`
- Multi-host (≤5): `4 hosts (web1, web2, web3, web4)`
- Multi-host (>5): `12 hosts (web1, web2, web3, web4, web5, ...)`

The prompt becomes: `Apply these changes to <target>? (yes/no): `.

**Alternative considered: pass `*playbook.Play`.** Tighter coupling between `output` and `playbook` packages; today the emitter takes only primitives or `[]PlannedTask`. Rejected to keep the layering clean.

**Alternative considered: keep `PromptApproval()` and rely on the upstream `HOST` banner.** Rejected — that banner is several lines above; the failure mode the proposal cites (mis-identifying the target) is real even with a banner present.

### Decision 2: Format helper lives in the executor

Add `formatApprovalTarget(hosts []string, connection string) string` in `internal/executor/`. The executor already owns `play.Hosts` and `play.GetConnection()` and is the natural place to decide truncation. The emitter renders the prompt; it doesn't decide which hosts are interesting.

### Decision 3: Cap visible host names at 5

Multi-host plays with more than 5 hosts truncate to first 5 plus a literal `, ...` and rely on the leading count for full info: `12 hosts (web1, web2, web3, web4, web5, ...)`. Five is enough for typical small clusters and avoids wrapping the prompt across terminal lines for large fleets. This is consistent with how the consolidated plan footer already trades full enumeration for an aggregate count.

**Alternative considered: print all hosts, regardless of count.** Rejected — large fleets (50+) make the prompt unreadable.

### Decision 4: JSON emitter ignores the new arg

`json.PromptApproval(target string)` keeps its current always-`true` auto-approve behavior. The arg is accepted for interface compatibility and discarded. Documented in the JSON emitter's doc comment.

### Decision 5: `Apply cancelled.` stays as-is

The cancel branch already prints a generic `Apply cancelled.` info line. No host needed there because the prompt above already named it; adding it again is redundant.

## Risks / Trade-offs

- **[Risk]** Width: a long hostname plus a long connection label could push the prompt past 80 columns. → **Mitigation:** the prompt is one line and tooling normally wraps acceptably; we don't truncate the single-host name (users explicitly want to see it). If this becomes an issue we can switch to two lines (banner above prompt) without a spec change.
- **[Risk]** Tests that snapshot prompt text will need updates. → **Mitigation:** small, predictable diff; we touch the test files in the same change.
- **[Trade-off]** The `, ...` truncation hides hosts. → **Mitigation:** the leading count and the consolidated plan body still enumerate all hosts. Users wanting full enumeration can scroll up.
- **[Risk]** `Emitter` interface change ripples to any third-party emitter implementations. → **Mitigation:** there are no external implementations in this repo; both internal implementations are updated together. The signature is additive at the call site (we pass one extra string).

## Migration Plan

Single change, no migration. Tests update in lockstep with the new signature. Rollback is reverting the commit; no persisted state.
