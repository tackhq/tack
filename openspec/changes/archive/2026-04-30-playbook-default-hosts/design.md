## Context

A tack playbook today is a YAML sequence: each element is a play with its own `hosts:`, `connection:`, etc. For multi-play playbooks targeting one group, this duplication is annoying and error-prone — change the target and you must touch every play.

The parser lives in `internal/playbook/parser.go` and produces a `Playbook{Plays []*Play}` value. The root unmarshal currently tries `[]map[string]any` first, then falls back to a single `map[string]any` (a single play). Extending it to also recognize a "playbook mapping with `plays:`" is straightforward and stays compatible with both prior shapes.

Plays already have rich override semantics — CLI overrides win over playbook values (`internal/executor/executor.go:264`), and inventory groups expand at execute time (`internal/executor/executor.go:332`). The cleanest place to apply playbook-level defaults is **at parse time**, so the executor never has to know about the new format.

## Goals / Non-Goals

**Goals:**
- Eliminate hosts/connection/sudo/vars duplication for multi-play playbooks targeting one group.
- Zero breakage for existing sequence-format playbooks, examples, and tests.
- Defaults resolved at parse time so the executor and downstream code work unchanged.
- Clear error when the new mapping format is malformed (missing `plays:`, etc.).

**Non-Goals:**
- A whole new playbook schema or field renames. We add an alternate top-level shape, not a replacement.
- Inheritance for every Play field (SSH config, SSM config, vars_files, vault_file, roles, handlers, etc.). The first cut covers `hosts`, `connection`, `sudo`, `vars` — the fields users actually duplicate. More can be added later if asked.
- Changing CLI override precedence. CLI flags still override anything from the playbook (default or per-play).
- Multi-document YAML (`---` separators) or `import_playbook`-style aggregation. Out of scope.

## Decisions

### Detect format from YAML root node kind

The parser currently does:
```go
var rawPlays []map[string]any
if err := yaml.Unmarshal(data, &rawPlays); err != nil {
    var rawPlay map[string]any
    if err := yaml.Unmarshal(data, &rawPlay); err != nil { ... }
    rawPlays = []map[string]any{rawPlay}
}
```

Change it to first parse into `yaml.Node`, switch on `node.Kind`:
- `SequenceNode` → existing path (each item is a play).
- `MappingNode` with `plays:` key → new path: pull out `hosts`, `connection`, `sudo`, `vars` as defaults, decode `plays:` as `[]map[string]any`, apply defaults to each.
- `MappingNode` without `plays:` → treat as a single-play map (existing fallback) **only if** none of the new playbook-level reserved keys (`plays`) is present. This preserves the "single play as a map at root" behavior the current code allows.

Why YAML node first: it lets us distinguish "mapping that's actually a playbook config" from "mapping that's actually a single play" without ambiguity, and it's a small, well-scoped change to the entry point.

**Alternative considered:** a magic `defaults:` key under the existing sequence root (e.g. first element with `defaults: true`). Rejected — feels hacky, and breaks the invariant that every sequence element is a play.

### Apply defaults at parse time, not execute time

`parseRawPlay` is called once per play. After parsing, if playbook-level defaults exist, fill in unset fields:
- `play.Hosts` — if empty, copy playbook default (already a `[]string` after the same scalar-or-list dance).
- `play.Connection` — if `""`, copy.
- `play.Sudo` — if `false` and the playbook explicitly set `sudo: true`, set true. (We can't distinguish "unset" from "false" with a plain bool, so playbook-level `sudo: false` has no effect — only `sudo: true` propagates. This matches how users actually use the field; non-issue in practice.)
- `play.Vars` — merge playbook vars in first, then play-level vars (so play wins on key conflicts).

Other fields (`SSH`, `SSM`, `VarsFiles`, etc.) are explicitly NOT inherited in this change. If someone needs SSH config inheritance, that's a follow-up.

**Alternative considered:** keep playbook-level defaults on the `Playbook` struct and have the executor merge at run time. Rejected — adds an extra concept for the executor to track, and the executor already has CLI-override merging logic that would interleave awkwardly. Parse-time merge keeps `Play` as the single source of truth.

### Playbook struct changes

Add minimal storage for round-tripping/debugging, but the merge happens during parse:

```go
type Playbook struct {
    Path  string
    Plays []*Play
    // Defaults captures playbook-level inheritance values for tooling/debug;
    // they are already applied to each play in Plays.
    Defaults *PlaybookDefaults
}

type PlaybookDefaults struct {
    Hosts      []string
    Connection string
    Sudo       bool
    Vars       map[string]any
}
```

`Defaults` is informational. Existing readers of `Playbook.Plays` see fully-resolved plays with no behavior change.

### Validation message

`Play.Validate()` today checks `len(p.Hosts) == 0` and errors. Keep that check — defaults are already merged in by the time validation runs, so the existing error path naturally covers "neither playbook nor play declared hosts." We just update the error string to mention the playbook-level option:

> "play has no hosts; declare `hosts:` on the play or at the playbook level"

## Risks / Trade-offs

- **Risk: a single-play map at root could be misread as the new format if the user names a task field `plays`.** → Mitigation: `plays` is reserved at the playbook root; if present, we always parse as the new format. Plays/tasks don't have a `plays:` field today, so the collision is implausible.
- **Risk: users expect `sudo: false` at playbook level to force-disable sudo even if a play sets `sudo: true`.** → Trade-off: we document that play-level always wins. Forcing sudo off from above would require a new field; defer.
- **Risk: docs and examples drift between two formats.** → Mitigation: keep all existing examples in sequence format; add ONE mapping-format example. README's quickstart stays sequence; advanced usage doc shows mapping.
- **Risk: tools that hand-compose playbook YAML (e.g. `tack generate`) might emit ambiguous output.** → Mitigation: `tack generate` continues to emit sequence format. Explicitly out of scope for this change.

## Migration Plan

No migration required. Existing playbooks parse unchanged. Users opt into the mapping format by writing it. Rollback is removing the parser branch — no data is persisted.
