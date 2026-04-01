## Context

Bolt's variable system merges variables from multiple sources in a defined precedence order. Currently there's no way to reference external variable files. Ansible supports `vars_files:` as a play-level directive that loads YAML files and merges them into the play's variable scope.

The playbook parser already handles `vars:` as a `map[string]any`. Adding `vars_files:` requires parsing a `[]string` field and loading each file during executor setup.

## Goals / Non-Goals

**Goals:**
- Parse `vars_files:` as a list of file paths in play YAML
- Load YAML files and merge into play variables in order
- Support relative paths (resolved against playbook directory)
- Support variable interpolation in paths for dynamic file selection
- Error on missing files by default; support optional files
- Integrate into variable precedence chain between play vars and inventory vars

**Non-Goals:**
- Directory-based vars loading (e.g., `vars_files: vars/` loading all files in a directory)
- Encrypted vars files (use `vault_file:` for that)
- Glob patterns in paths
- Watching files for changes

## Decisions

### 1. vars_files loaded after play vars, before inventory vars

Precedence order: role defaults < role vars < play vars < vars_files < inventory vars < vault < facts < registered < env < extra-vars.

This means vars_files can reference play-level vars (for path interpolation) but inventory and CLI vars can still override them.

**Alternative considered:** Loading vars_files at the same level as play vars. Rejected because vars_files should be able to override play defaults while still being overridable by inventory-specific settings.

### 2. Optional files via `?` prefix

Support an optional file marker: if a path starts with `?`, it's optional (no error if missing). Example: `?vars/local.yaml`.

**Alternative considered:** A separate `vars_files_optional:` directive. Rejected as too verbose for a simple concept.

### 3. Path interpolation runs with play-level vars only

When interpolating `vars/{{ env }}.yaml`, only play-level vars and extra-vars are available (inventory and facts haven't been loaded yet). This is documented so users know which variables are available for path construction.

## Risks / Trade-offs

- **[Risk] Circular variable references** — A vars file could reference a variable defined in another vars file. → Mitigation: Files are loaded sequentially; a file can only reference vars from previously loaded files and play vars.
- **[Risk] Path traversal** — `vars_files: [../../etc/shadow]` could read sensitive files. → Mitigation: Bolt runs as the invoking user; file system permissions apply. No additional restriction needed (same as `src:` in copy module).
- **[Trade-off] No directory loading** — Users must list each file explicitly. → Acceptable for v1; directory support can be added later.
