## Context

Bolt resolves play targets through a pipeline: CLI overrides → inventory group expansion → SSM tag resolution → validation. When resolution fails, the error message is generic and doesn't help users diagnose the problem. Additionally, there's no way to target all inventory hosts without listing every group name.

Current host validation in `internal/executor/executor.go:332-333`:
```go
if play.GetConnection() != "local" && len(play.Hosts) == 0 {
    return fmt.Errorf("play is missing 'hosts' (provide via playbook or -c flag)")
}
```

## Goals / Non-Goals

**Goals:**
- Actionable error messages that tell users exactly what went wrong and how to fix it
- `--hosts all` as an explicit opt-in to target entire inventory
- Zero breaking changes to existing behavior

**Non-Goals:**
- Changing host resolution priority or override semantics
- Auto-discovery of inventory files (e.g., looking for `inventory.yaml` in playbook dir)
- Implicit "run on all hosts" behavior

## Decisions

### 1. "all" is a reserved keyword in --hosts, not a new flag

`--hosts all` is handled as a special case during inventory expansion, not as a separate CLI flag. This keeps the CLI surface small and is consistent with Ansible's `all` pattern.

**Alternative considered:** A `--all-hosts` boolean flag. Rejected because it adds flag sprawl and `--hosts all` is more intuitive.

### 2. AllHosts() lives on the Inventory type

A new `AllHosts() []string` method on `*Inventory` expands every group, merges top-level hosts, and deduplicates. This keeps the logic testable and co-located with other inventory operations.

### 3. Three distinct error paths instead of one

| Condition | Error message |
|-----------|--------------|
| No hosts specified anywhere | `play has no target hosts (provide via --hosts, playbook hosts: field, or -c flag)` |
| SSM tags resolved to zero instances | `SSM tag resolution matched zero instances for tags: {env:prod, role:web}` |
| `--hosts all` without inventory | `--hosts all requires an inventory file (-i flag)` |

### 4. "all" expansion happens in runPlay alongside group expansion

The `"all"` keyword is detected in the inventory expansion loop (line ~287-315). When encountered, it calls `AllHosts()` and replaces the entry. This keeps all host resolution in one place.

## Risks / Trade-offs

- **[Risk] "all" collides with a real hostname or group named "all"** → Unlikely in practice. If needed, users can rename. Ansible has the same convention.
- **[Risk] Large inventories with --hosts all could be surprising** → The explicit opt-in (`--hosts all`) is the mitigation — users know what they asked for.
