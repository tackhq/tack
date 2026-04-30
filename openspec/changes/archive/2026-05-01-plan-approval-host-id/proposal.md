## Why

When the plan is shown and tack prompts `Do you want to apply these changes? (yes/no):`, nothing in the prompt line names the host(s) those changes will hit. Users running against unfamiliar inventories or SSM/EC2 instances (where hosts are opaque IDs like `i-0a1b2c3d4e5f`) can't tell from the prompt alone whether they're about to touch the right machine. The host banner is several lines above and easy to miss when the plan is long, and parallel runs across multiple terminals make the gap dangerous. We want the approval prompt itself to carry an unambiguous host identifier.

## What Changes

- Update the approval prompt to include the target host(s) directly in the question, e.g. `Apply these changes to web1.prod (ssh)? (yes/no):` for single-host plays and `Apply these changes to 4 hosts (web1, web2, web3, web4)? (yes/no):` for multi-host plays.
- For multi-host plays with more than 5 hosts, abbreviate the list to the first 5 plus a `...` suffix and the total count.
- Show the connection type alongside single-host targets so users can tell `i-0abc (ssm)` apart from a same-named SSH host.
- No change to `--auto-approve` behavior, JSON output (which auto-approves), or the actual plan body rendering — only the prompt line and the surrounding `Apply cancelled.` message.
- Single-host fast path keeps its existing `HOST <host> [<conn>]` banner above the plan; the prompt suffix is additive.

## Capabilities

### New Capabilities
<!-- None — this extends the existing approval-prompt behavior. -->

### Modified Capabilities
- `consolidated-plan-and-approval`: extends the "Single global approval prompt" requirement so the prompt SHALL identify the targeted host(s) and connection type, with abbreviation for >5 hosts.

## Impact

- `internal/output/output.go` — `PromptApproval()` signature gains hosts/connection context (or a new `PromptApprovalForHosts` is added) and renders the new prompt text.
- `internal/output/json.go` — JSON emitter's `PromptApproval` stays auto-approve; if signature changes, update its implementation accordingly.
- `internal/executor/executor.go` — both call sites (single-host at `runPlayOnHost`, multi-host at `runMultiHostPlay`) pass `play.Hosts` and `play.GetConnection()` (single-host: just the one host) into the prompt.
- Tests: extend output unit tests for the prompt text; existing executor tests that check approval flow may need updates if signatures shift.
- Docs: no user-facing doc changes required; the prompt is self-explanatory.
- No breaking changes to the playbook YAML, CLI flags, or JSON output schema.
