## Why

Today, every play in a playbook must repeat its `hosts:` (and often `connection:`, `sudo:`, etc.) even when all plays target the same group. For multi-play playbooks aimed at a single group like `webservers`, this is boilerplate that obscures the playbook's intent and creates drift risk when the target group changes (every play must be edited in lockstep). Users want to declare the target group once at the top of the playbook and have all plays inherit it unless they opt out.

## What Changes

- Support a new playbook-level format: a YAML mapping with `hosts:` (and optional `connection:`, `sudo:`, `vars:`) at the top, plus a `plays:` array underneath. The existing array-of-plays format continues to work unchanged.
- When the playbook-level format is used, each play inherits `hosts:`, `connection:`, and `sudo:` from the playbook unless the play sets its own value (play-level overrides win).
- Playbook-level `vars:` are merged into each play's vars; play-level keys override playbook-level keys on conflict.
- `tack validate` accepts both formats; the existing "hosts required" check moves to "either the playbook or the play must declare hosts."
- Update playbook parser to detect the format from the top-level YAML node type (sequence → legacy, mapping → new).
- Update `examples/` and docs to demonstrate the new format alongside the old.

## Capabilities

### New Capabilities
- `playbook-defaults`: Playbook-level defaults (hosts, connection, sudo, vars) inherited by plays that don't override them, plus the mapping-style playbook format that carries them.

### Modified Capabilities
<!-- None — playbook structure isn't owned by an existing spec. -->

## Impact

- `internal/playbook/parser.go` — detect mapping vs. sequence at the root, parse playbook-level fields, merge into plays.
- `internal/playbook/playbook.go` — add playbook-level fields to `Playbook` struct (or a new `Defaults` substruct) without breaking existing consumers.
- `internal/playbook/parser_test.go` — coverage for both formats and override precedence.
- `internal/executor/executor.go` — no change expected; merging happens at parse time so the executor sees fully-resolved plays.
- `cmd/tack/main.go` — `validate` reuses the parser; help text and examples updated.
- `examples/` — add a mapping-format example; existing array-format examples remain valid.
- `docs/getting-started.md`, `README.md`, `llms.txt` — document the new format.
- No breaking changes; existing playbooks parse identically.
