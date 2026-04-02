## Why

Bolt already has a basic `include:` directive on tasks that dynamically loads external task files at runtime. However, it lacks key capabilities that make task inclusion truly useful:

- No `vars:` support — cannot pass scoped variables to included files
- No `loop:` support — cannot include the same file multiple times with different parameters
- No circular include detection — recursive includes can hang
- Path resolution inconsistencies between playbook-relative and role-relative contexts
- The `include:` keyword diverges from the Ansible-standard `include_tasks:` that DevOps engineers expect

This is a P0 structural feature that unlocks DRY playbook patterns — reusable task libraries, OS-specific task files, and parameterized includes.

## What Changes

- Add `include_tasks:` as the primary directive (rename of `include:` with enhanced capabilities)
- Add `vars:` block support for scoped variable passing into included task files
- Add `loop:` support for iterating over a list and including the file once per item
- Support `{{ variable }}` interpolation in include paths for dynamic file selection
- Relative path resolution: relative to the including playbook file, or to the role's `tasks/` directory when inside a role
- Circular include detection with clear error messages and a max depth limit
- Keep `include:` as a backward-compatible alias for `include_tasks:` (no deprecation warning needed — same behavior)
- Integration with existing `when:` conditions (already works, now documented)
- Update documentation and add example playbooks

## Capabilities

### New Capabilities
- `include-tasks`: Enhanced runtime task file inclusion — `vars:` support, `loop:` support, variable-interpolated paths, circular detection, and improved path resolution
- `include-docs`: Documentation updates with usage examples, comparison guide, and example playbooks

### Modified Capabilities
<!-- No existing spec-level capabilities are changing -->

## Impact

- **Parser** (`internal/playbook/parser.go`): Add `include_tasks` to `knownTaskFields`, parse `vars:` on include directives
- **Playbook structs** (`internal/playbook/playbook.go`): Add `IncludeVars map[string]any` field to `Task`; `include_tasks` maps to existing `Include` field
- **Executor** (`internal/executor/executor.go`): Enhance `runInclude()` with `vars:` merging, `loop:` support, circular detection via visited-path set
- **Plan phase**: `include_tasks` shown as a single entry with "will_run" or "conditional" status
- **Existing `include:` directive**: Remains functional, treated as alias for `include_tasks:`
- **Docs**: README, example playbooks
