## Why

Several small but impactful CLI usability issues create friction for new and experienced users alike. The `--forks` flag silently does nothing, `--check` vs `--dry-run` aliasing is confusing, the approval prompt is unnecessarily strict, and `bolt modules` provides no detail. These are individually minor but collectively degrade the user experience.

## What Changes

- **Remove `--forks` flag** until parallel execution is implemented — prevents false expectations
- **Unify dry-run flags** — keep `--dry-run` as the primary flag, `--check` as a documented alias, both behave identically at all scopes
- **Flexible approval prompt** — accept `y`, `Y`, `yes`, `Yes`, `YES` (case-insensitive match on "y" or "yes")
- **Per-module help** — add `bolt module <name>` subcommand that shows parameters, states, and usage examples from embedded documentation

## Capabilities

### New Capabilities
- `cli-ux-improvements`: Bundle of CLI usability fixes covering flag cleanup, prompt flexibility, and module documentation

### Modified Capabilities

_None._

## Impact

- **Modified code**: `cmd/bolt/main.go` — remove `--forks`, unify dry-run, add `module` subcommand
- **Modified code**: `internal/executor/executor.go` or `cmd/bolt/main.go` — flexible approval prompt
- **Modified code**: `internal/module/module.go` — optional module description/help interface
- **BREAKING**: Removing `--forks` flag will break scripts that use `-f` or `--forks` (acceptable since the flag never worked)
