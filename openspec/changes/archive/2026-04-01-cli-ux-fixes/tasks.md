## 1. Remove Forks Flag

- [x] 1.1 Remove `--forks` / `-f` flag declaration from `cmd/bolt/main.go`
- [x] 1.2 Remove any references to the forks variable in executor options

## 2. Flexible Approval Prompt

- [x] 2.1 Update approval prompt logic to accept case-insensitive "y" or "yes" using `strings.EqualFold`
- [x] 2.2 Ensure empty input and any other string is treated as rejection

## 3. Unify Dry-Run Flags

- [x] 3.1 Make `--check` a PersistentFlag alias for `--dry-run` at the root command level
- [x] 3.2 Remove the command-specific `--check` flag from the `run` command
- [x] 3.3 Verify both flags work identically on all commands

## 4. Per-Module Help

- [x] 4.1 Add optional `Describer` interface to `internal/module/module.go` with `Description() string` and `Parameters() []ParamDoc`
- [x] 4.2 Define `ParamDoc` struct: `{Name, Type, Required, Default, Description}`
- [x] 4.3 Add `bolt module <name>` subcommand to `cmd/bolt/main.go`
- [x] 4.4 Implement Describer for at least `apt`, `yum`, and `file` modules as examples
- [x] 4.5 Fall back to "no documentation available" for modules without Describer

## 5. Testing

- [x] 5.1 Test: `--forks` flag produces error
- [x] 5.2 Test: approval prompt accepts y, Y, yes, YES, Yes
- [x] 5.3 Test: approval prompt rejects empty, no, other strings
- [x] 5.4 Test: `--check` and `--dry-run` produce identical behavior
- [x] 5.5 Test: `bolt module apt` shows parameter documentation
- [x] 5.6 Test: `bolt module nonexistent` shows error with available modules
