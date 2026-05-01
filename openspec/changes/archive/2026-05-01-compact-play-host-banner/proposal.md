## Why

Today's start-of-play output repeats information across three lines for a typical run:

```
PLAY i-0637cf7dfdd354125

HOST i-0637cf7dfdd354125 [ssm]
✓ Gathering Facts
```

Two problems:

1. The `PLAY` banner only adds value when the play has an explicit `name:`. When it's empty, today's parser falls back to joining `play.Hosts` — which is exactly what the `HOST` banner immediately below already shows. For single-host plays this is pure noise.
2. `Gathering Facts` is its own task line for what is effectively connection setup. It's vertically separated from the `HOST` banner it belongs to, so when you scroll a multi-host run you see "✓ Gathering Facts" lines that aren't anchored to a host without scrolling.

This change tightens the start-of-play output so the host identity appears once, fact-gathering status is anchored to it, and the `PLAY` line earns its space (or doesn't appear).

## What Changes

- **Inline fact-gathering into the HOST banner.** `HOST <host> [<conn>]` becomes `HOST <host> [<conn>] - gathering facts ✓` once facts return successfully (or `✗` on failure with the error continuing on a follow-up line). No standalone `✓ Gathering Facts` task line.
- **Drop the PLAY banner when the play has no `name:`.** Today's fallback (`PLAY <joined hosts>`) is removed; the host identity already lives on the HOST line below.
- **Add a `HOSTS` summary line for multi-host plays.** When the play targets two or more hosts, emit a single `HOSTS h1, h2, h3` line right after `PLAYBOOK <path>` (and after the named `PLAY` line, if present). For more than five hosts, abbreviate as `HOSTS h1, h2, h3, h4, h5 (and 7 more)` so the line stays short on large fleets.
- **No change to**: per-line host attribution in the consolidated multi-host plan body, the approval prompt format, JSON output events, `PLAYBOOK` / `RECAP` lines, or `gather_facts: false` plays (no fact line is emitted in either case).
- **No change to** the `gather_facts: false` flow other than dropping the standalone task line that was never shown.

## Capabilities

### New Capabilities
<!-- None — this is a presentation-only refinement of existing banners. -->

### Modified Capabilities
- `consolidated-plan-and-approval`: refine the start-of-play banner rules (PLAY shown only when named, new HOSTS summary, fact-gathering inlined into the HOST line). The plan body, footer, and approval prompt requirements are unchanged.

## Impact

- `internal/output/output.go` — `HostStart` gains a fact-gathering completion update (or a new `HostFactsResult` method); `PlayStart` skips emission when the play has no name; new `PlayHosts` (or similar) method renders the `HOSTS` summary.
- `internal/output/json.go` — JSON emitter is unaffected by visual changes; the new methods become no-ops on JSON.
- `internal/output/emitter.go` — interface picks up the small set of new methods.
- `internal/executor/executor.go` — replace the `TaskStart("Gathering Facts", "")` / `TaskResult("Gathering Facts", "ok", false, "")` pair with the inlined HOST-banner update; gate `PlayStart` on play name presence and add the `HOSTS` summary call site for multi-host plays. Both the inline path (`runPlayOnHost`) and the parallel pre-pass (`flushPrepBuffers`) need the same treatment.
- Tests: extend output unit tests; update any executor tests that snapshot banner output (single-host fast path is no longer "byte-identical to current main", but the `consolidated-plan-and-approval` requirement that asserts that will be modified accordingly).
- Docs: `docs/connectors.md` and any quickstart sample output snippets need updating to match.
- No CLI flag changes, no breaking changes to playbook YAML or JSON schema.
