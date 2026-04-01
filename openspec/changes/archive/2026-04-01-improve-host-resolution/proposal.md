## Why

When host resolution fails (e.g., SSM tag resolution returns zero instances, or no hosts are specified anywhere), bolt produces a generic error — `play is missing 'hosts' (provide via playbook or -c flag)` — that doesn't mention `--hosts`, doesn't distinguish between "you forgot to specify targets" and "your targets resolved to nothing", and provides no path to target all inventory hosts at once.

## What Changes

- Improve the generic "missing hosts" error message to mention all three ways to provide hosts: `--hosts`, playbook `hosts:` field, and `-c` flag.
- Add a distinct error when SSM tag resolution succeeds but returns zero matching instances, so users know the tags didn't match rather than thinking they forgot to specify hosts.
- Add `--hosts all` support: when an inventory is loaded, `--hosts all` expands every group, deduplicates, and targets all hosts. This is the only way to run against the entire inventory — bolt never implicitly targets all hosts.

## Capabilities

### New Capabilities
- `hosts-all`: Support `--hosts all` to target every host in a loaded inventory file, expanding all groups and deduplicating.

### Modified Capabilities

## Impact

- `internal/executor/executor.go` — error messages at host validation (line ~333), SSM tag resolution (line ~317-329), and new "all" expansion logic in `runPlay`.
- `internal/inventory/inventory.go` — new `AllHosts()` method to expand all groups and deduplicate.
- `cmd/bolt/main.go` — no changes needed; `--hosts all` flows through existing flag handling as the literal string `"all"`.
